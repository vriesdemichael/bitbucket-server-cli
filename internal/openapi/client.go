package openapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/diagnostics"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/network"
)

func NewClientWithResponsesFromConfig(cfg config.AppConfig) (*openapigenerated.ClientWithResponses, error) {
	serverURL := strings.TrimRight(cfg.BitbucketURL, "/") + "/rest"

	transport, err := network.NewSafeTransport(network.TLSOptions{
		CAFile:             cfg.CAFile,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &retryTransport{
			base:        transport,
			retries:     cfg.RetryCount,
			baseBackoff: cfg.RetryBackoff,
			logger: diagnostics.NewLogger(diagnostics.Config{
				Level:  diagnostics.Level(cfg.LogLevel),
				Format: diagnostics.Format(cfg.LogFormat),
			}, diagnostics.EnabledWriter(cfg.DiagnosticsEnabled, diagnostics.OutputWriter())),
		},
	}

	return openapigenerated.NewClientWithResponses(
		serverURL,
		openapigenerated.WithHTTPClient(httpClient),
		openapigenerated.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			if cfg.BitbucketToken != "" {
				request.Header.Set("Authorization", "Bearer "+cfg.BitbucketToken)
				return nil
			}
			if cfg.BitbucketUsername != "" && cfg.BitbucketPassword != "" {
				request.SetBasicAuth(cfg.BitbucketUsername, cfg.BitbucketPassword)
			}
			return nil
		}),
	)
}

type retryTransport struct {
	base        http.RoundTripper
	retries     int
	baseBackoff time.Duration
	logger      *diagnostics.Logger
}

func (transport *retryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}

	var lastResponse *http.Response
	var lastError error

	for attempt := 0; attempt <= transport.retries; attempt++ {
		started := time.Now()
		activeRequest := request
		if attempt > 0 {
			if request.GetBody == nil && request.Body != nil {
				break
			}

			clone := request.Clone(request.Context())
			if request.GetBody != nil {
				body, err := request.GetBody()
				if err != nil {
					break
				}
				clone.Body = body
			}
			activeRequest = clone
		}

		response, err := base.RoundTrip(activeRequest)
		if err != nil {
			lastError = err
			fields := map[string]any{
				"method":      request.Method,
				"endpoint":    request.URL.Path,
				"attempt":     attempt + 1,
				"retry_count": transport.retries,
				"duration_ms": time.Since(started).Milliseconds(),
				"error":       err.Error(),
			}
			if attempt < transport.retries {
				transport.logger.Warn("http request failed", fields)
				time.Sleep(time.Duration(attempt+1) * transport.baseBackoff)
				continue
			}
			transport.logger.Error("http request failed", fields)
			return nil, err
		}

		transport.logger.Debug("http request completed", map[string]any{
			"method":      request.Method,
			"endpoint":    request.URL.Path,
			"status":      response.StatusCode,
			"attempt":     attempt + 1,
			"retry_count": transport.retries,
			"duration_ms": time.Since(started).Milliseconds(),
		})

		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			lastResponse = response
			fields := map[string]any{
				"method":      request.Method,
				"endpoint":    request.URL.Path,
				"status":      response.StatusCode,
				"attempt":     attempt + 1,
				"retry_count": transport.retries,
				"duration_ms": time.Since(started).Milliseconds(),
			}
			if attempt < transport.retries {
				transport.logger.Warn("http retriable response", fields)
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
				time.Sleep(time.Duration(attempt+1) * transport.baseBackoff)
				continue
			}
			transport.logger.Error("http retriable response", fields)
		}

		return response, nil
	}

	if lastResponse != nil {
		return lastResponse, nil
	}

	return nil, lastError
}
