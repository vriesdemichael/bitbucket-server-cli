package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/diagnostics"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/network"
)

type Client struct {
	baseURL  string
	http     *http.Client
	token    string
	username string
	password string
	retries  int
	backoff  time.Duration
	logger   *diagnostics.Logger
	initErr  error
}

type HealthStatus struct {
	Healthy       bool   `json:"healthy"`
	StatusCode    int    `json:"status_code"`
	Authenticated bool   `json:"authenticated"`
	Message       string `json:"message"`
}

func NewFromConfig(cfg config.AppConfig) *Client {
	transport, err := network.NewSafeTransport(network.TLSOptions{
		CAFile:             cfg.CAFile,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	if err != nil {
		transport = &network.SafeTransport{}
	}

	return &Client{
		baseURL: strings.TrimRight(cfg.BitbucketURL, "/"),
		http: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: transport,
		},
		token:    cfg.BitbucketToken,
		username: cfg.BitbucketUsername,
		password: cfg.BitbucketPassword,
		retries:  cfg.RetryCount,
		backoff:  cfg.RetryBackoff,
		logger: diagnostics.NewLogger(diagnostics.Config{
			Level:  diagnostics.Level(cfg.LogLevel),
			Format: diagnostics.Format(cfg.LogFormat),
		}, diagnosticsWriter(cfg.DiagnosticsEnabled, diagnostics.OutputWriter())),
		initErr: err,
	}
}

func (client *Client) GetJSON(ctx context.Context, path string, query map[string]string, out any) error {
	return client.doJSON(ctx, http.MethodGet, path, query, nil, out)
}

func (client *Client) PostJSON(ctx context.Context, path string, query map[string]string, in any, out any) error {
	return client.doJSON(ctx, http.MethodPost, path, query, in, out)
}

func (client *Client) PutJSON(ctx context.Context, path string, query map[string]string, in any, out any) error {
	return client.doJSON(ctx, http.MethodPut, path, query, in, out)
}

func (client *Client) DeleteJSON(ctx context.Context, path string, query map[string]string, in any, out any) error {
	return client.doJSON(ctx, http.MethodDelete, path, query, in, out)
}

func (client *Client) doJSON(ctx context.Context, method string, path string, query map[string]string, in any, out any) error {
	if client.initErr != nil {
		return apperrors.New(apperrors.KindValidation, "failed to initialize HTTP transport", client.initErr)
	}

	requestURL, err := url.Parse(client.baseURL + path)
	if err != nil {
		return apperrors.New(apperrors.KindValidation, "invalid request URL", err)
	}

	values := requestURL.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	requestURL.RawQuery = values.Encode()

	var payload []byte
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return apperrors.New(apperrors.KindValidation, "failed to encode request body", err)
		}
		payload = encoded
	}

	var lastErr error
	for attempt := 0; attempt <= client.retries; attempt++ {
		started := time.Now()
		var bodyReader io.Reader
		if payload != nil {
			bodyReader = bytes.NewReader(payload)
		}

		request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), bodyReader)
		if err != nil {
			return apperrors.New(apperrors.KindInternal, "failed to build request", err)
		}

		request.Header.Set("Accept", "application/json")
		if payload != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		client.applyAuth(request)

		response, err := client.http.Do(request)
		if err != nil {
			fields := map[string]any{
				"method":      method,
				"endpoint":    requestURL.Path,
				"attempt":     attempt + 1,
				"retry_count": client.retries,
				"duration_ms": time.Since(started).Milliseconds(),
				"error":       err.Error(),
			}
			lastErr = apperrors.New(apperrors.KindTransient, "request failed", err)
			if attempt < client.retries {
				client.logger.Warn("http request failed", fields)
				time.Sleep(time.Duration(attempt+1) * client.backoff)
				continue
			}
			client.logger.Error("http request failed", fields)
			return lastErr
		}

		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			return apperrors.New(apperrors.KindTransient, "failed to read response", readErr)
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			client.logger.Debug("http request completed", map[string]any{
				"method":      method,
				"endpoint":    requestURL.Path,
				"status":      response.StatusCode,
				"attempt":     attempt + 1,
				"retry_count": client.retries,
				"duration_ms": time.Since(started).Milliseconds(),
			})
			if out == nil || len(bytes.TrimSpace(body)) == 0 {
				return nil
			}

			if err := json.Unmarshal(body, out); err != nil {
				return apperrors.New(apperrors.KindPermanent, "failed to decode API response", err)
			}
			return nil
		}

		mappedErr := openapi.MapStatusError(response.StatusCode, body)
		fields := map[string]any{
			"method":      method,
			"endpoint":    requestURL.Path,
			"status":      response.StatusCode,
			"attempt":     attempt + 1,
			"retry_count": client.retries,
			"duration_ms": time.Since(started).Milliseconds(),
			"error":       mappedErr.Error(),
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			lastErr = mappedErr
			if attempt < client.retries {
				client.logger.Warn("http request returned error status", fields)
				time.Sleep(time.Duration(attempt+1) * client.backoff)
				continue
			}
			client.logger.Error("http request returned error status", fields)
			return lastErr
		}

		client.logger.Error("http request returned error status", fields)

		return mappedErr
	}

	if lastErr != nil {
		return lastErr
	}

	return apperrors.New(apperrors.KindTransient, "request failed after retries", nil)
}

func (client *Client) Health(ctx context.Context) (HealthStatus, error) {
	if client.initErr != nil {
		return HealthStatus{}, apperrors.New(apperrors.KindValidation, "failed to initialize HTTP transport", client.initErr)
	}

	requestURL, err := url.Parse(client.baseURL + "/rest/api/1.0/projects?limit=1")
	if err != nil {
		return HealthStatus{}, apperrors.New(apperrors.KindValidation, "invalid health probe URL", err)
	}

	var lastErr error
	for attempt := 0; attempt <= client.retries; attempt++ {
		started := time.Now()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return HealthStatus{}, apperrors.New(apperrors.KindInternal, "failed to build health request", err)
		}

		request.Header.Set("Accept", "application/json")
		client.applyAuth(request)

		response, err := client.http.Do(request)
		if err != nil {
			fields := map[string]any{
				"method":      http.MethodGet,
				"endpoint":    requestURL.Path,
				"attempt":     attempt + 1,
				"retry_count": client.retries,
				"duration_ms": time.Since(started).Milliseconds(),
				"error":       err.Error(),
			}
			lastErr = apperrors.New(apperrors.KindTransient, "health probe failed", err)
			if attempt < client.retries {
				client.logger.Warn("health probe failed", fields)
				time.Sleep(time.Duration(attempt+1) * client.backoff)
				continue
			}
			client.logger.Error("health probe failed", fields)
			return HealthStatus{}, lastErr
		}

		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()

		switch {
		case response.StatusCode >= 200 && response.StatusCode < 300:
			client.logger.Debug("health probe completed", map[string]any{
				"status":      response.StatusCode,
				"endpoint":    requestURL.Path,
				"attempt":     attempt + 1,
				"retry_count": client.retries,
				"duration_ms": time.Since(started).Milliseconds(),
			})
			return HealthStatus{
				Healthy:       true,
				StatusCode:    response.StatusCode,
				Authenticated: true,
				Message:       "Bitbucket API reachable and authenticated",
			}, nil
		case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden || response.StatusCode >= 300 && response.StatusCode < 400:
			client.logger.Info("health probe unauthenticated", map[string]any{
				"status":      response.StatusCode,
				"endpoint":    requestURL.Path,
				"attempt":     attempt + 1,
				"retry_count": client.retries,
				"duration_ms": time.Since(started).Milliseconds(),
			})
			return HealthStatus{
				Healthy:       true,
				StatusCode:    response.StatusCode,
				Authenticated: false,
				Message:       "Bitbucket reachable but credentials are missing or insufficient",
			}, nil
		case response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests:
			lastErr = openapi.MapStatusError(response.StatusCode, nil)
			if attempt < client.retries {
				time.Sleep(time.Duration(attempt+1) * client.backoff)
				continue
			}
			return HealthStatus{}, lastErr
		default:
			return HealthStatus{}, openapi.MapStatusError(response.StatusCode, nil)
		}
	}

	if lastErr != nil {
		return HealthStatus{}, lastErr
	}

	return HealthStatus{}, apperrors.New(apperrors.KindTransient, "health probe failed after retries", nil)
}

func (client *Client) applyAuth(request *http.Request) {
	if client.token != "" {
		request.Header.Set("Authorization", "Bearer "+client.token)
		return
	}

	if client.username != "" && client.password != "" {
		request.SetBasicAuth(client.username, client.password)
	}
}

func diagnosticsWriter(enabled bool, writer io.Writer) io.Writer {
	if enabled {
		return writer
	}

	return io.Discard
}
