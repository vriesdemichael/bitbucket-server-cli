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
}

type HealthStatus struct {
	Healthy       bool   `json:"healthy"`
	StatusCode    int    `json:"status_code"`
	Authenticated bool   `json:"authenticated"`
	Message       string `json:"message"`
}

func NewFromConfig(cfg config.AppConfig) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BitbucketURL, "/"),
		http: &http.Client{
			Timeout:   20 * time.Second,
			Transport: &network.SafeTransport{},
		},
		token:    cfg.BitbucketToken,
		username: cfg.BitbucketUsername,
		password: cfg.BitbucketPassword,
		retries:  2,
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
			lastErr = apperrors.New(apperrors.KindTransient, "request failed", err)
			if attempt < client.retries {
				time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
				continue
			}
			return lastErr
		}

		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			return apperrors.New(apperrors.KindTransient, "failed to read response", readErr)
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if out == nil || len(bytes.TrimSpace(body)) == 0 {
				return nil
			}

			if err := json.Unmarshal(body, out); err != nil {
				return apperrors.New(apperrors.KindPermanent, "failed to decode API response", err)
			}
			return nil
		}

		mappedErr := openapi.MapStatusError(response.StatusCode, body)
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			lastErr = mappedErr
			if attempt < client.retries {
				time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
				continue
			}
			return lastErr
		}

		return mappedErr
	}

	if lastErr != nil {
		return lastErr
	}

	return apperrors.New(apperrors.KindTransient, "request failed after retries", nil)
}

func (client *Client) Health(ctx context.Context) (HealthStatus, error) {
	requestURL, err := url.Parse(client.baseURL + "/rest/api/1.0/projects?limit=1")
	if err != nil {
		return HealthStatus{}, apperrors.New(apperrors.KindValidation, "invalid health probe URL", err)
	}

	var lastErr error
	for attempt := 0; attempt <= client.retries; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return HealthStatus{}, apperrors.New(apperrors.KindInternal, "failed to build health request", err)
		}

		request.Header.Set("Accept", "application/json")
		client.applyAuth(request)

		response, err := client.http.Do(request)
		if err != nil {
			lastErr = apperrors.New(apperrors.KindTransient, "health probe failed", err)
			if attempt < client.retries {
				time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
				continue
			}
			return HealthStatus{}, lastErr
		}

		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()

		switch {
		case response.StatusCode >= 200 && response.StatusCode < 300:
			return HealthStatus{
				Healthy:       true,
				StatusCode:    response.StatusCode,
				Authenticated: true,
				Message:       "Bitbucket API reachable and authenticated",
			}, nil
		case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden || response.StatusCode >= 300 && response.StatusCode < 400:
			return HealthStatus{
				Healthy:       true,
				StatusCode:    response.StatusCode,
				Authenticated: false,
				Message:       "Bitbucket reachable but credentials are missing or insufficient",
			}, nil
		case response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests:
			lastErr = openapi.MapStatusError(response.StatusCode, nil)
			if attempt < client.retries {
				time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
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
