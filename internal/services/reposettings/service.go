package reposettings

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	"io"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type RepositoryRef struct {
	ProjectKey string
	Slug       string
}

type PermissionUser struct {
	Name       string `json:"name"`
	Display    string `json:"display_name,omitempty"`
	Permission string `json:"permission,omitempty"`
}

type PermissionGroup struct {
	Name       string `json:"name"`
	Permission string `json:"permission,omitempty"`
}

type WebhookList struct {
	Count   int `json:"count"`
	Payload any `json:"payload"`
}

type WebhookCreateInput struct {
	Name   string
	URL    string
	Events []string
	Active bool
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) ListRepositoryPermissionUsers(ctx context.Context, repo RepositoryRef, limit int) ([]PermissionUser, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}

	start := float32(0)
	pageLimit := float32(limit)
	results := make([]PermissionUser, 0)

	for {
		response, err := service.client.GetUsersWithAnyPermission2WithResponse(ctx, repo.ProjectKey, repo.Slug, &openapigenerated.GetUsersWithAnyPermission2Params{
			Start: &start,
			Limit: &pageLimit,
		})
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list repository permissions", err)
		}
		if err := openapi.MapStatusError(response.StatusCode(), response.Body); err != nil {
			return nil, err
		}
		if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Values == nil {
			break
		}

		for _, value := range *response.ApplicationjsonCharsetUTF8200.Values {
			entry := PermissionUser{}
			if value.User != nil {
				if value.User.Name != nil {
					entry.Name = *value.User.Name
				}
				if value.User.DisplayName != nil {
					entry.Display = *value.User.DisplayName
				}
			}
			if value.Permission != nil {
				entry.Permission = string(*value.Permission)
			}
			results = append(results, entry)
		}

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

func (service *Service) ListRepositoryPermissionGroups(ctx context.Context, repo RepositoryRef, limit int) ([]PermissionGroup, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}

	start := float32(0)
	pageLimit := float32(limit)
	results := make([]PermissionGroup, 0)

	for {
		response, err := service.client.GetGroupsWithAnyPermission2WithResponse(ctx, repo.ProjectKey, repo.Slug, &openapigenerated.GetGroupsWithAnyPermission2Params{
			Start: &start,
			Limit: &pageLimit,
		})
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list repository group permissions", err)
		}
		if err := openapi.MapStatusError(response.StatusCode(), response.Body); err != nil {
			return nil, err
		}
		if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Values == nil {
			break
		}

		for _, value := range *response.ApplicationjsonCharsetUTF8200.Values {
			entry := PermissionGroup{}
			if value.Group != nil {
				if value.Group.Name != nil {
					entry.Name = *value.Group.Name
				}
			}
			if value.Permission != nil {
				entry.Permission = string(*value.Permission)
			}
			results = append(results, entry)
		}

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

func (service *Service) GrantRepositoryUserPermission(ctx context.Context, repo RepositoryRef, username string, permission string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}
	trimmedUser := strings.TrimSpace(username)
	if trimmedUser == "" {
		return apperrors.New(apperrors.KindValidation, "username is required", nil)
	}

	normalizedPermission, err := normalizeRepositoryPermission(permission)
	if err != nil {
		return err
	}

	response, err := service.client.SetPermissionForUserWithResponse(ctx, repo.ProjectKey, repo.Slug, &openapigenerated.SetPermissionForUserParams{
		Name:       []string{trimmedUser},
		Permission: openapigenerated.SetPermissionForUserParamsPermission(normalizedPermission),
	})
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to grant repository permission", err)
	}

	return openapi.MapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) RevokeRepositoryUserPermission(ctx context.Context, repo RepositoryRef, username string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}
	trimmedUser := strings.TrimSpace(username)
	if trimmedUser == "" {
		return apperrors.New(apperrors.KindValidation, "username is required", nil)
	}

	response, err := service.client.RevokePermissionsForUser2WithResponse(ctx, repo.ProjectKey, repo.Slug, &openapigenerated.RevokePermissionsForUser2Params{
		Name: trimmedUser,
	})
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to revoke repository user permission", err)
	}

	return openapi.MapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) GrantRepositoryGroupPermission(ctx context.Context, repo RepositoryRef, group string, permission string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return apperrors.New(apperrors.KindValidation, "group name is required", nil)
	}

	normalizedPermission, err := normalizeRepositoryPermission(permission)
	if err != nil {
		return err
	}

	response, err := service.client.SetPermissionForGroupWithResponse(ctx, repo.ProjectKey, repo.Slug, &openapigenerated.SetPermissionForGroupParams{
		Name:       []string{trimmedGroup},
		Permission: openapigenerated.SetPermissionForGroupParamsPermission(normalizedPermission),
	})
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to grant repository group permission", err)
	}

	return openapi.MapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) RevokeRepositoryGroupPermission(ctx context.Context, repo RepositoryRef, group string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return apperrors.New(apperrors.KindValidation, "group name is required", nil)
	}

	response, err := service.client.RevokePermissionsForGroup2WithResponse(ctx, repo.ProjectKey, repo.Slug, &openapigenerated.RevokePermissionsForGroup2Params{
		Name: trimmedGroup,
	})
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to revoke repository group permission", err)
	}

	return openapi.MapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) ListRepositoryWebhooks(ctx context.Context, repo RepositoryRef) (WebhookList, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return WebhookList{}, err
	}

	response, err := service.client.FindWebhooks1WithResponse(ctx, repo.ProjectKey, repo.Slug, nil)
	if err != nil {
		return WebhookList{}, apperrors.New(apperrors.KindTransient, "failed to list repository webhooks", err)
	}
	if err := openapi.MapStatusError(response.StatusCode(), response.Body); err != nil {
		return WebhookList{}, err
	}

	if !json.Valid(response.Body) {
		return WebhookList{}, apperrors.New(apperrors.KindPermanent, "invalid JSON payload from webhooks endpoint", nil)
	}

	var payload any
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return WebhookList{}, apperrors.New(apperrors.KindPermanent, "failed to decode webhooks payload", err)
	}

	count := 0
	switch typed := payload.(type) {
	case []any:
		count = len(typed)
	case map[string]any:
		if values, ok := typed["values"].([]any); ok {
			count = len(values)
		}
	}

	return WebhookList{Count: count, Payload: payload}, nil
}

func (service *Service) CreateRepositoryWebhook(ctx context.Context, repo RepositoryRef, input WebhookCreateInput) (any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	trimmedName := strings.TrimSpace(input.Name)
	trimmedURL := strings.TrimSpace(input.URL)
	if trimmedName == "" {
		return nil, apperrors.New(apperrors.KindValidation, "webhook name is required", nil)
	}
	if trimmedURL == "" {
		return nil, apperrors.New(apperrors.KindValidation, "webhook url is required", nil)
	}

	events := make([]string, 0, len(input.Events))
	for _, event := range input.Events {
		if trimmedEvent := strings.TrimSpace(event); trimmedEvent != "" {
			events = append(events, trimmedEvent)
		}
	}
	if len(events) == 0 {
		events = []string{"repo:refs_changed"}
	}

	body := openapigenerated.CreateWebhook1JSONRequestBody{
		Name:   &trimmedName,
		Url:    &trimmedURL,
		Events: &events,
		Active: &input.Active,
	}

	response, err := service.client.CreateWebhook1WithResponse(ctx, repo.ProjectKey, repo.Slug, body)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to create repository webhook", err)
	}
	if err := openapi.MapStatusError(response.StatusCode(), response.Body); err != nil {
		return nil, err
	}

	if !json.Valid(response.Body) {
		return nil, nil
	}

	var payload any
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode created webhook payload", err)
	}

	return payload, nil
}

func (service *Service) DeleteRepositoryWebhook(ctx context.Context, repo RepositoryRef, webhookID string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}
	trimmedWebhookID := strings.TrimSpace(webhookID)
	if trimmedWebhookID == "" {
		return apperrors.New(apperrors.KindValidation, "webhook id is required", nil)
	}

	response, err := service.client.DeleteWebhook1WithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedWebhookID)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete repository webhook", err)
	}

	return openapi.MapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) GetRepositoryPullRequestSettings(ctx context.Context, repo RepositoryRef) (map[string]any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	response, err := service.client.GetPullRequestSettings1(ctx, repo.ProjectKey, repo.Slug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to get pull request settings", err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to read pull request settings response", readErr)
	}

	if err := openapi.MapStatusError(response.StatusCode, body); err != nil {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, apperrors.New(apperrors.KindPermanent, "invalid JSON payload from pull request settings endpoint", nil)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode pull request settings payload", err)
	}

	return payload, nil
}

func (service *Service) UpdateRepositoryPullRequestSettings(ctx context.Context, repo RepositoryRef, settings map[string]any) (map[string]any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	rawPayload, err := json.Marshal(settings)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to encode pull request settings update", err)
	}

	response, err := service.client.UpdatePullRequestSettings1WithBody(ctx, repo.ProjectKey, repo.Slug, "application/json", bytes.NewReader(rawPayload))
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to update pull request settings", err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to read pull request settings update response", readErr)
	}
	if err := openapi.MapStatusError(response.StatusCode, body); err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(body)) == 0 || !json.Valid(body) {
		// Fallback for non-JSON response from some Bitbucket versions/endpoints
		return settings, nil
	}

	updated := map[string]any{}
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode pull request settings update response", err)
	}

	return updated, nil
}

func (service *Service) UpdateRepositoryPullRequestRequiredAllTasks(ctx context.Context, repo RepositoryRef, required bool) (map[string]any, error) {
	return service.UpdateRepositoryPullRequestSettings(ctx, repo, map[string]any{"requiredAllTasksComplete": required})
}

func (service *Service) UpdateRepositoryPullRequestRequiredApproversCount(ctx context.Context, repo RepositoryRef, count int) (map[string]any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, apperrors.New(apperrors.KindValidation, "required approvers count must be >= 0", nil)
	}

	// Try object structure first (modern Bitbucket)
	settings := map[string]any{
		"requiredApprovers": map[string]any{
			"enabled": count > 0,
			"count":   count,
		},
	}
	if count == 0 {
		settings["requiredApprovers"] = map[string]any{
			"enabled": false,
		}
	}

	result, err := service.UpdateRepositoryPullRequestSettings(ctx, repo, settings)
	if err != nil {
		// Only fallback if it's a validation error AND it likely relates to the payload structure.
		// Our MapStatusError includes the body in the message.
		if apperrors.IsKind(err, apperrors.KindValidation) &&
			(strings.Contains(strings.ToLower(err.Error()), "invalid") || strings.Contains(strings.ToLower(err.Error()), "payload")) {
			return service.UpdateRepositoryPullRequestSettings(ctx, repo, map[string]any{"requiredApprovers": count})
		}
		return nil, err
	}

	return result, nil
}

func (service *Service) ListRequiredBuildsMergeChecks(ctx context.Context, repo RepositoryRef) (any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	response, err := service.client.GetPageOfRequiredBuildsMergeChecksWithResponse(ctx, repo.ProjectKey, repo.Slug, nil)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to list required builds merge checks", err)
	}
	if err := openapi.MapStatusError(response.StatusCode(), response.Body); err != nil {
		return nil, err
	}

	var payload any
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode merge checks payload", err)
	}

	return payload, nil
}

func validateRepositoryRef(repo RepositoryRef) error {
	if strings.TrimSpace(repo.ProjectKey) == "" || strings.TrimSpace(repo.Slug) == "" {
		return apperrors.New(apperrors.KindValidation, "repository must be specified as project/repo", nil)
	}

	return nil
}

func normalizeRepositoryPermission(permission string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(permission)) {
	case "REPO_READ":
		return "REPO_READ", nil
	case "REPO_WRITE":
		return "REPO_WRITE", nil
	case "REPO_ADMIN":
		return "REPO_ADMIN", nil
	default:
		return "", apperrors.New(apperrors.KindValidation, "permission must be one of REPO_READ, REPO_WRITE, REPO_ADMIN", nil)
	}
}
