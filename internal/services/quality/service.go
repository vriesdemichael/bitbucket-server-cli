package quality

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type RepositoryRef struct {
	ProjectKey string
	Slug       string
}

type BuildStatusSetInput struct {
	Key         string
	State       string
	URL         string
	Name        string
	Description string
	Ref         string
	Parent      string
	BuildNumber string
	DurationMS  int64
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) SetBuildStatus(ctx context.Context, commitID string, input BuildStatusSetInput) error {
	trimmedCommitID := strings.TrimSpace(commitID)
	if trimmedCommitID == "" {
		return apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}

	trimmedKey := strings.TrimSpace(input.Key)
	trimmedState := strings.TrimSpace(input.State)
	trimmedURL := strings.TrimSpace(input.URL)
	if trimmedKey == "" {
		return apperrors.New(apperrors.KindValidation, "build status key is required", nil)
	}
	if trimmedState == "" {
		return apperrors.New(apperrors.KindValidation, "build status state is required", nil)
	}
	if trimmedURL == "" {
		return apperrors.New(apperrors.KindValidation, "build status url is required", nil)
	}

	state := openapigenerated.RestBuildStatusState(strings.ToUpper(trimmedState))
	body := openapigenerated.AddBuildStatusJSONRequestBody{
		Key:   &trimmedKey,
		State: &state,
		Url:   &trimmedURL,
	}

	if strings.TrimSpace(input.Name) != "" {
		trimmedName := strings.TrimSpace(input.Name)
		body.Name = &trimmedName
	}
	if strings.TrimSpace(input.Description) != "" {
		trimmedDescription := strings.TrimSpace(input.Description)
		body.Description = &trimmedDescription
	}
	if strings.TrimSpace(input.Ref) != "" {
		trimmedRef := strings.TrimSpace(input.Ref)
		body.Ref = &trimmedRef
	}
	if strings.TrimSpace(input.Parent) != "" {
		trimmedParent := strings.TrimSpace(input.Parent)
		body.Parent = &trimmedParent
	}
	if strings.TrimSpace(input.BuildNumber) != "" {
		trimmedBuildNumber := strings.TrimSpace(input.BuildNumber)
		body.BuildNumber = &trimmedBuildNumber
	}
	if input.DurationMS > 0 {
		duration := input.DurationMS
		body.Duration = &duration
	}

	response, err := service.client.AddBuildStatusWithResponse(ctx, trimmedCommitID, body)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to set build status", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) GetBuildStatuses(ctx context.Context, commitID string, limit int, orderBy string) ([]openapigenerated.RestBuildStatus, error) {
	trimmedCommitID := strings.TrimSpace(commitID)
	if trimmedCommitID == "" {
		return nil, apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if limit <= 0 {
		limit = 25
	}

	start := float32(0)
	pageLimit := float32(limit)
	statuses := make([]openapigenerated.RestBuildStatus, 0)

	for {
		params := &openapigenerated.GetBuildStatusParams{Start: &start, Limit: &pageLimit}
		if strings.TrimSpace(orderBy) != "" {
			resolvedOrderBy := strings.TrimSpace(orderBy)
			params.OrderBy = &resolvedOrderBy
		}

		response, err := service.client.GetBuildStatusWithResponse(ctx, trimmedCommitID, params)
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to get build statuses", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return nil, err
		}
		if response.JSON200 == nil || response.JSON200.Values == nil {
			break
		}

		statuses = append(statuses, (*response.JSON200.Values)...)

		if response.JSON200.IsLastPage != nil && *response.JSON200.IsLastPage {
			break
		}
		if response.JSON200.NextPageStart == nil {
			break
		}

		start = float32(*response.JSON200.NextPageStart)
	}

	return statuses, nil
}

func (service *Service) GetBuildStatusStats(ctx context.Context, commitID string, includeUnique bool) (openapigenerated.RestBuildStats, error) {
	trimmedCommitID := strings.TrimSpace(commitID)
	if trimmedCommitID == "" {
		return openapigenerated.RestBuildStats{}, apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}

	params := &openapigenerated.GetBuildStatusStatsParams{IncludeUnique: &includeUnique}
	response, err := service.client.GetBuildStatusStatsWithResponse(ctx, trimmedCommitID, params)
	if err != nil {
		return openapigenerated.RestBuildStats{}, apperrors.New(apperrors.KindTransient, "failed to get build status stats", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestBuildStats{}, err
	}

	if response.JSON200 != nil {
		return *response.JSON200, nil
	}

	return openapigenerated.RestBuildStats{}, nil
}

func (service *Service) ListRequiredBuildChecks(ctx context.Context, repo RepositoryRef, limit int) ([]openapigenerated.RestRequiredBuildCondition, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 25
	}

	start := float32(0)
	pageLimit := float32(limit)
	checks := make([]openapigenerated.RestRequiredBuildCondition, 0)

	for {
		response, err := service.client.GetPageOfRequiredBuildsMergeChecksWithResponse(
			ctx,
			repo.ProjectKey,
			repo.Slug,
			&openapigenerated.GetPageOfRequiredBuildsMergeChecksParams{Start: &start, Limit: &pageLimit},
		)
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list required build merge checks", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return nil, err
		}
		if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Values == nil {
			break
		}

		checks = append(checks, (*response.ApplicationjsonCharsetUTF8200.Values)...)

		if response.ApplicationjsonCharsetUTF8200.IsLastPage != nil && *response.ApplicationjsonCharsetUTF8200.IsLastPage {
			break
		}
		if response.ApplicationjsonCharsetUTF8200.NextPageStart == nil {
			break
		}

		start = float32(*response.ApplicationjsonCharsetUTF8200.NextPageStart)
	}

	return checks, nil
}

func (service *Service) CreateRequiredBuildCheck(ctx context.Context, repo RepositoryRef, payload map[string]any) (map[string]any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.New(apperrors.KindValidation, "invalid required build check payload", err)
	}

	response, err := service.client.CreateRequiredBuildsMergeCheckWithBodyWithResponse(
		ctx,
		repo.ProjectKey,
		repo.Slug,
		"application/json",
		bytes.NewReader(rawPayload),
	)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to create required build merge check", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return nil, err
	}

	if response.ApplicationjsonCharsetUTF8200 == nil {
		return map[string]any{}, nil
	}

	encoded, err := json.Marshal(response.ApplicationjsonCharsetUTF8200)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to encode required build merge check response", err)
	}

	parsed := map[string]any{}
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode required build merge check response", err)
	}

	return parsed, nil
}

func (service *Service) UpdateRequiredBuildCheck(ctx context.Context, repo RepositoryRef, id int64, payload map[string]any) (map[string]any, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, apperrors.New(apperrors.KindValidation, "required build merge check id must be > 0", nil)
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.New(apperrors.KindValidation, "invalid required build merge check payload", err)
	}

	response, err := service.client.UpdateRequiredBuildsMergeCheckWithBodyWithResponse(
		ctx,
		repo.ProjectKey,
		repo.Slug,
		id,
		"application/json",
		bytes.NewReader(rawPayload),
	)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to update required build merge check", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return nil, err
	}

	if response.ApplicationjsonCharsetUTF8200 == nil {
		return map[string]any{}, nil
	}

	encoded, err := json.Marshal(response.ApplicationjsonCharsetUTF8200)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to encode required build merge check response", err)
	}

	parsed := map[string]any{}
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return nil, apperrors.New(apperrors.KindPermanent, "failed to decode required build merge check response", err)
	}

	return parsed, nil
}

func (service *Service) DeleteRequiredBuildCheck(ctx context.Context, repo RepositoryRef, id int64) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}
	if id <= 0 {
		return apperrors.New(apperrors.KindValidation, "required build merge check id must be > 0", nil)
	}

	response, err := service.client.DeleteRequiredBuildsMergeCheckWithResponse(ctx, repo.ProjectKey, repo.Slug, id)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete required build merge check", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) ListReports(ctx context.Context, repo RepositoryRef, commitID string, limit int) ([]openapigenerated.RestInsightReport, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}
	trimmedCommitID := strings.TrimSpace(commitID)
	if trimmedCommitID == "" {
		return nil, apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if limit <= 0 {
		limit = 25
	}

	start := float32(0)
	pageLimit := float32(limit)
	reports := make([]openapigenerated.RestInsightReport, 0)

	for {
		response, err := service.client.GetReportsWithResponse(
			ctx,
			repo.ProjectKey,
			repo.Slug,
			trimmedCommitID,
			&openapigenerated.GetReportsParams{Start: &start, Limit: &pageLimit},
		)
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list code insights reports", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return nil, err
		}
		if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Values == nil {
			break
		}

		reports = append(reports, (*response.ApplicationjsonCharsetUTF8200.Values)...)

		if response.ApplicationjsonCharsetUTF8200.IsLastPage != nil && *response.ApplicationjsonCharsetUTF8200.IsLastPage {
			break
		}
		if response.ApplicationjsonCharsetUTF8200.NextPageStart == nil {
			break
		}

		start = float32(*response.ApplicationjsonCharsetUTF8200.NextPageStart)
	}

	return reports, nil
}

func (service *Service) SetReport(ctx context.Context, repo RepositoryRef, commitID string, key string, request openapigenerated.SetACodeInsightsReportJSONRequestBody) (openapigenerated.RestInsightReport, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return openapigenerated.RestInsightReport{}, err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedKey := strings.TrimSpace(key)
	if trimmedCommitID == "" {
		return openapigenerated.RestInsightReport{}, apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if trimmedKey == "" {
		return openapigenerated.RestInsightReport{}, apperrors.New(apperrors.KindValidation, "report key is required", nil)
	}

	response, err := service.client.SetACodeInsightsReportWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedCommitID, trimmedKey, request)
	if err != nil {
		return openapigenerated.RestInsightReport{}, apperrors.New(apperrors.KindTransient, "failed to set code insights report", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestInsightReport{}, err
	}

	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return openapigenerated.RestInsightReport{}, nil
}

func (service *Service) GetReport(ctx context.Context, repo RepositoryRef, commitID string, key string) (openapigenerated.RestInsightReport, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return openapigenerated.RestInsightReport{}, err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedKey := strings.TrimSpace(key)
	if trimmedCommitID == "" {
		return openapigenerated.RestInsightReport{}, apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if trimmedKey == "" {
		return openapigenerated.RestInsightReport{}, apperrors.New(apperrors.KindValidation, "report key is required", nil)
	}

	response, err := service.client.GetACodeInsightsReportWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedCommitID, trimmedKey)
	if err != nil {
		return openapigenerated.RestInsightReport{}, apperrors.New(apperrors.KindTransient, "failed to get code insights report", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestInsightReport{}, err
	}

	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return openapigenerated.RestInsightReport{}, nil
}

func (service *Service) DeleteReport(ctx context.Context, repo RepositoryRef, commitID string, key string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedKey := strings.TrimSpace(key)
	if trimmedCommitID == "" {
		return apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if trimmedKey == "" {
		return apperrors.New(apperrors.KindValidation, "report key is required", nil)
	}

	response, err := service.client.DeleteACodeInsightsReportWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedCommitID, trimmedKey)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete code insights report", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) AddAnnotations(ctx context.Context, repo RepositoryRef, commitID string, key string, annotations []openapigenerated.RestSingleAddInsightAnnotationRequest) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedKey := strings.TrimSpace(key)
	if trimmedCommitID == "" {
		return apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if trimmedKey == "" {
		return apperrors.New(apperrors.KindValidation, "report key is required", nil)
	}
	if len(annotations) == 0 {
		return apperrors.New(apperrors.KindValidation, "at least one annotation is required", nil)
	}

	body := openapigenerated.AddAnnotationsJSONRequestBody{Annotations: &annotations}
	response, err := service.client.AddAnnotationsWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedCommitID, trimmedKey, body)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to add code insights annotations", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
}

func (service *Service) ListAnnotations(ctx context.Context, repo RepositoryRef, commitID string, key string) ([]openapigenerated.RestInsightAnnotation, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedKey := strings.TrimSpace(key)
	if trimmedCommitID == "" {
		return nil, apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if trimmedKey == "" {
		return nil, apperrors.New(apperrors.KindValidation, "report key is required", nil)
	}

	response, err := service.client.GetAnnotationsWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedCommitID, trimmedKey)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to list code insights annotations", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return nil, err
	}
	if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Annotations == nil {
		return []openapigenerated.RestInsightAnnotation{}, nil
	}

	return *response.ApplicationjsonCharsetUTF8200.Annotations, nil
}

func (service *Service) DeleteAnnotations(ctx context.Context, repo RepositoryRef, commitID string, key string, externalID string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedKey := strings.TrimSpace(key)
	trimmedExternalID := strings.TrimSpace(externalID)
	if trimmedCommitID == "" {
		return apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}
	if trimmedKey == "" {
		return apperrors.New(apperrors.KindValidation, "report key is required", nil)
	}
	if trimmedExternalID == "" {
		return apperrors.New(apperrors.KindValidation, "external annotation id is required", nil)
	}

	response, err := service.client.DeleteAnnotationsWithResponse(
		ctx,
		repo.ProjectKey,
		repo.Slug,
		trimmedCommitID,
		trimmedKey,
		&openapigenerated.DeleteAnnotationsParams{ExternalId: &trimmedExternalID},
	)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete code insights annotations", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
}

func validateRepositoryRef(repo RepositoryRef) error {
	if strings.TrimSpace(repo.ProjectKey) == "" || strings.TrimSpace(repo.Slug) == "" {
		return apperrors.New(apperrors.KindValidation, "repository must be specified as project/repo", nil)
	}

	return nil
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
