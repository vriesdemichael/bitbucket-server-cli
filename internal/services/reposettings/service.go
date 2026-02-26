package reposettings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
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

func (service *Service) ListRepositoryWebhooks(ctx context.Context, repo RepositoryRef) (WebhookList, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return WebhookList{}, err
	}

	response, err := service.client.FindWebhooks1WithResponse(ctx, repo.ProjectKey, repo.Slug, nil)
	if err != nil {
		return WebhookList{}, apperrors.New(apperrors.KindTransient, "failed to list repository webhooks", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
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
		Permission: normalizedPermission,
	})
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to grant repository permission", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
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
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
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

	return mapStatusError(response.StatusCode(), response.Body)
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

	if err := mapStatusError(response.StatusCode, body); err != nil {
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

func (service *Service) UpdateRepositoryPullRequestRequiredAllTasks(ctx context.Context, repo RepositoryRef, required bool) (map[string]any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	rawPayload, err := json.Marshal(map[string]any{"requiredAllTasksComplete": required})
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
	if err := mapStatusError(response.StatusCode, body); err != nil {
		return nil, err
	}

	if !json.Valid(body) {
		updated := map[string]any{}
		updated["requiredAllTasksComplete"] = required
		return updated, nil
	}

	updated := map[string]any{}
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode pull request settings update payload", err)
	}

	return updated, nil
}

func (service *Service) UpdateRepositoryPullRequestRequiredApproversCount(ctx context.Context, repo RepositoryRef, count int) (map[string]any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, apperrors.New(apperrors.KindValidation, "required approvers count must be >= 0", nil)
	}

	payload := map[string]any{}
	payload["requiredApprovers"] = count

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to encode pull request approvers update", err)
	}

	response, err := service.client.UpdatePullRequestSettings1WithBody(ctx, repo.ProjectKey, repo.Slug, "application/json", bytes.NewReader(rawPayload))
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to update pull request required approvers", err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to read pull request approvers update response", readErr)
	}
	if err := mapStatusError(response.StatusCode, body); err != nil {
		return nil, err
	}

	if !json.Valid(body) {
		return payload, nil
	}

	updated := map[string]any{}
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode pull request approvers update payload", err)
	}

	return updated, nil
}

func validateRepositoryRef(repo RepositoryRef) error {
	if strings.TrimSpace(repo.ProjectKey) == "" || strings.TrimSpace(repo.Slug) == "" {
		return apperrors.New(apperrors.KindValidation, "repository must be specified as project/repo", nil)
	}

	return nil
}

func normalizeRepositoryPermission(permission string) (openapigenerated.SetPermissionForUserParamsPermission, error) {
	switch strings.ToUpper(strings.TrimSpace(permission)) {
	case "REPO_READ":
		return openapigenerated.SetPermissionForUserParamsPermission("REPO_READ"), nil
	case "REPO_WRITE":
		return openapigenerated.SetPermissionForUserParamsPermission("REPO_WRITE"), nil
	case "REPO_ADMIN":
		return openapigenerated.SetPermissionForUserParamsPermission("REPO_ADMIN"), nil
	default:
		return "", apperrors.New(apperrors.KindValidation, "permission must be one of REPO_READ, REPO_WRITE, REPO_ADMIN", nil)
	}
}

func mapStatusError(status int, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(status)
	}

	baseMessage := fmt.Sprintf("bitbucket API returned %d: %s", status, message)

	switch status {
	case http.StatusBadRequest:
		return apperrors.New(apperrors.KindValidation, baseMessage, nil)
	case http.StatusUnauthorized:
		return apperrors.New(apperrors.KindAuthentication, baseMessage, nil)
	case http.StatusForbidden:
		return apperrors.New(apperrors.KindAuthorization, baseMessage, nil)
	case http.StatusNotFound:
		return apperrors.New(apperrors.KindNotFound, baseMessage, nil)
	case http.StatusConflict:
		return apperrors.New(apperrors.KindConflict, baseMessage, nil)
	case http.StatusTooManyRequests:
		return apperrors.New(apperrors.KindTransient, baseMessage, nil)
	default:
		if status >= 500 {
			return apperrors.New(apperrors.KindTransient, baseMessage, nil)
		}
		return apperrors.New(apperrors.KindPermanent, baseMessage, nil)
	}
}
