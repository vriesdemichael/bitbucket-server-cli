package openapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func NewClientWithResponsesFromConfig(cfg config.AppConfig) (*openapigenerated.ClientWithResponses, error) {
	serverURL := strings.TrimRight(cfg.BitbucketURL, "/") + "/rest"

	return openapigenerated.NewClientWithResponses(
		serverURL,
		openapigenerated.WithHTTPClient(&http.Client{Timeout: 20 * time.Second}),
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
