package openapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
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
}

func (transport *retryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}

	var lastResponse *http.Response
	var lastError error

	for attempt := 0; attempt <= transport.retries; attempt++ {
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
			if attempt < transport.retries {
				time.Sleep(time.Duration(attempt+1) * transport.baseBackoff)
				continue
			}
			return nil, err
		}

		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			lastResponse = response
			if attempt < transport.retries {
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
				time.Sleep(time.Duration(attempt+1) * transport.baseBackoff)
				continue
			}
		}

		return response, nil
	}

	if lastResponse != nil {
		return lastResponse, nil
	}

	return nil, lastError
}
