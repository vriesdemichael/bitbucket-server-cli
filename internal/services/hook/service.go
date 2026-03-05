package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) ListProjectHooks(ctx context.Context, projectKey string, limit int) ([]openapigenerated.RestRepositoryHook, error) {
	if strings.TrimSpace(projectKey) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}
	if limit <= 0 {
		limit = 100
	}

	start := float32(0)
	pageLimit := float32(limit)
	results := make([]openapigenerated.RestRepositoryHook, 0)

	for {
		response, err := service.client.GetRepositoryHooksWithResponse(ctx, projectKey, &openapigenerated.GetRepositoryHooksParams{
			Start: &start,
			Limit: &pageLimit,
		})
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list project hooks", err)
		}
		if response.StatusCode() != 200 {
			return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
		}

		if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Values == nil {
			break
		}

		results = append(results, *response.ApplicationjsonCharsetUTF8200.Values...)

		if response.ApplicationjsonCharsetUTF8200.IsLastPage != nil && *response.ApplicationjsonCharsetUTF8200.IsLastPage {
			break
		}
		if response.ApplicationjsonCharsetUTF8200.NextPageStart == nil {
			break
		}
		start = float32(*response.ApplicationjsonCharsetUTF8200.NextPageStart)
	}

	return results, nil
}

func (service *Service) ListRepositoryHooks(ctx context.Context, projectKey, repositorySlug string, limit int) ([]openapigenerated.RestRepositoryHook, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key and repository slug are required", nil)
	}
	if limit <= 0 {
		limit = 100
	}

	start := float32(0)
	pageLimit := float32(limit)
	results := make([]openapigenerated.RestRepositoryHook, 0)

	for {
		response, err := service.client.GetRepositoryHooks1WithResponse(ctx, projectKey, repositorySlug, &openapigenerated.GetRepositoryHooks1Params{
			Start: &start,
			Limit: &pageLimit,
		})
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list repository hooks", err)
		}
		if response.StatusCode() != 200 {
			return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
		}

		if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Values == nil {
			break
		}

		results = append(results, *response.ApplicationjsonCharsetUTF8200.Values...)

		if response.ApplicationjsonCharsetUTF8200.IsLastPage != nil && *response.ApplicationjsonCharsetUTF8200.IsLastPage {
			break
		}
		if response.ApplicationjsonCharsetUTF8200.NextPageStart == nil {
			break
		}
		start = float32(*response.ApplicationjsonCharsetUTF8200.NextPageStart)
	}

	return results, nil
}

func (service *Service) EnableProjectHook(ctx context.Context, projectKey, hookKey string) (openapigenerated.RestRepositoryHook, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(hookKey) == "" {
		return openapigenerated.RestRepositoryHook{}, apperrors.New(apperrors.KindValidation, "project key and hook key are required", nil)
	}
	response, err := service.client.EnableHookWithResponse(ctx, projectKey, hookKey, nil)
	if err != nil {
		return openapigenerated.RestRepositoryHook{}, apperrors.New(apperrors.KindTransient, "failed to enable project hook", err)
	}
	if response.StatusCode() != 200 {
		return openapigenerated.RestRepositoryHook{}, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}
	if response.ApplicationjsonCharsetUTF8200 == nil {
		return openapigenerated.RestRepositoryHook{}, nil
	}
	return *response.ApplicationjsonCharsetUTF8200, nil
}

func (service *Service) DisableProjectHook(ctx context.Context, projectKey, hookKey string) error {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(hookKey) == "" {
		return apperrors.New(apperrors.KindValidation, "project key and hook key are required", nil)
	}
	response, err := service.client.DisableHookWithResponse(ctx, projectKey, hookKey)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to disable project hook", err)
	}
	if response.StatusCode() != 204 {
		return apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}
	return nil
}

func (service *Service) EnableRepositoryHook(ctx context.Context, projectKey, repositorySlug, hookKey string) (openapigenerated.RestRepositoryHook, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" || strings.TrimSpace(hookKey) == "" {
		return openapigenerated.RestRepositoryHook{}, apperrors.New(apperrors.KindValidation, "project key, repository slug, and hook key are required", nil)
	}
	response, err := service.client.EnableHook1WithResponse(ctx, projectKey, repositorySlug, hookKey, nil)
	if err != nil {
		return openapigenerated.RestRepositoryHook{}, apperrors.New(apperrors.KindTransient, "failed to enable repository hook", err)
	}
	if response.StatusCode() != 200 {
		return openapigenerated.RestRepositoryHook{}, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}
	if response.ApplicationjsonCharsetUTF8200 == nil {
		return openapigenerated.RestRepositoryHook{}, nil
	}
	return *response.ApplicationjsonCharsetUTF8200, nil
}

func (service *Service) DisableRepositoryHook(ctx context.Context, projectKey, repositorySlug, hookKey string) error {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" || strings.TrimSpace(hookKey) == "" {
		return apperrors.New(apperrors.KindValidation, "project key, repository slug, and hook key are required", nil)
	}
	response, err := service.client.DisableHook1WithResponse(ctx, projectKey, repositorySlug, hookKey)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to disable repository hook", err)
	}
	if response.StatusCode() != 204 {
		return apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}
	return nil
}

func (service *Service) GetProjectHookSettings(ctx context.Context, projectKey, hookKey string) (any, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(hookKey) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key and hook key are required", nil)
	}
	response, err := service.client.GetSettingsWithResponse(ctx, projectKey, hookKey)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to get project hook settings", err)
	}
	if response.StatusCode() != 200 {
		return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	var settings any
	if err := json.Unmarshal(response.Body, &settings); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode hook settings", err)
	}
	return settings, nil
}

func (service *Service) GetRepositoryHookSettings(ctx context.Context, projectKey, repositorySlug, hookKey string) (any, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" || strings.TrimSpace(hookKey) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key, repository slug, and hook key are required", nil)
	}
	response, err := service.client.GetSettings1WithResponse(ctx, projectKey, repositorySlug, hookKey)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to get repository hook settings", err)
	}
	if response.StatusCode() != 200 {
		return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	var settings any
	if err := json.Unmarshal(response.Body, &settings); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode hook settings", err)
	}
	return settings, nil
}

func (service *Service) SetProjectHookSettings(ctx context.Context, projectKey, hookKey string, settings map[string]any) (any, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(hookKey) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key and hook key are required", nil)
	}
	body, err := json.Marshal(settings)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to encode hook settings", err)
	}

	response, err := service.client.SetSettingsWithBodyWithResponse(ctx, projectKey, hookKey, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to set project hook settings", err)
	}
	if response.StatusCode() != 200 {
		return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	var result any
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode hook settings response", err)
	}
	return result, nil
}

func (service *Service) SetRepositoryHookSettings(ctx context.Context, projectKey, repositorySlug, hookKey string, settings map[string]any) (any, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" || strings.TrimSpace(hookKey) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key, repository slug, and hook key are required", nil)
	}
	body, err := json.Marshal(settings)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to encode hook settings", err)
	}

	response, err := service.client.SetSettings1WithBodyWithResponse(ctx, projectKey, repositorySlug, hookKey, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to set repository hook settings", err)
	}
	if response.StatusCode() != 200 {
		return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	var result any
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode hook settings response", err)
	}
	return result, nil
}
