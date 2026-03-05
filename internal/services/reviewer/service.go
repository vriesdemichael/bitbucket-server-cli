package reviewer

import (
	"context"
	"encoding/json"
	"strconv"
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

func (service *Service) ListProjectConditions(ctx context.Context, projectKey string) ([]openapigenerated.RestPullRequestCondition, error) {
	if strings.TrimSpace(projectKey) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}

	response, err := service.client.GetPullRequestConditionsWithResponse(ctx, projectKey)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to list project reviewer conditions", err)
	}
	if response.StatusCode() != 200 {
		return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	if response.JSON200 == nil {
		return []openapigenerated.RestPullRequestCondition{}, nil
	}

	return *response.JSON200, nil
}

func (service *Service) ListRepositoryConditions(ctx context.Context, projectKey, repositorySlug string) ([]openapigenerated.RestPullRequestCondition, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "project key and repository slug are required", nil)
	}

	response, err := service.client.GetPullRequestConditions1WithResponse(ctx, projectKey, repositorySlug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to list repository reviewer conditions", err)
	}
	if response.StatusCode() != 200 {
		return nil, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	if response.JSON200 == nil {
		return []openapigenerated.RestPullRequestCondition{}, nil
	}

	return *response.JSON200, nil
}

func (service *Service) DeleteProjectCondition(ctx context.Context, projectKey string, conditionID string) error {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(conditionID) == "" {
		return apperrors.New(apperrors.KindValidation, "project key and condition ID are required", nil)
	}

	response, err := service.client.DeletePullRequestConditionWithResponse(ctx, projectKey, conditionID)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete project reviewer condition", err)
	}
	if response.StatusCode() >= 300 {
		return apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	return nil
}

func (service *Service) DeleteRepositoryCondition(ctx context.Context, projectKey, repositorySlug string, conditionID string) error {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" || strings.TrimSpace(conditionID) == "" {
		return apperrors.New(apperrors.KindValidation, "project key, repository slug, and condition ID are required", nil)
	}

	id, err := strconv.ParseInt(conditionID, 10, 32)
	if err != nil {
		return apperrors.New(apperrors.KindValidation, "condition ID must be an integer", err)
	}

	response, err := service.client.DeletePullRequestCondition1WithResponse(ctx, projectKey, repositorySlug, int32(id))
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete repository reviewer condition", err)
	}
	if response.StatusCode() >= 300 {
		return apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	return nil
}

func (service *Service) CreateProjectCondition(ctx context.Context, projectKey string, condition openapigenerated.RestDefaultReviewersRequest) (openapigenerated.RestPullRequestCondition, error) {
	if strings.TrimSpace(projectKey) == "" {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}

	response, err := service.client.CreatePullRequestConditionWithResponse(ctx, projectKey, condition)
	if err != nil {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindTransient, "failed to create project reviewer condition", err)
	}
	if response.StatusCode() >= 300 {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	if response.JSON200 != nil {
		return *response.JSON200, nil
	}

	if response.StatusCode() == 201 {
		var created openapigenerated.RestPullRequestCondition
		if err := json.Unmarshal(response.Body, &created); err == nil {
			return created, nil
		}
	}

	return openapigenerated.RestPullRequestCondition{}, nil
}

func (service *Service) CreateRepositoryCondition(ctx context.Context, projectKey, repositorySlug string, condition openapigenerated.RestDefaultReviewersRequest) (openapigenerated.RestPullRequestCondition, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindValidation, "project key and repository slug are required", nil)
	}

	response, err := service.client.CreatePullRequestCondition1WithResponse(ctx, projectKey, repositorySlug, condition)
	if err != nil {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindTransient, "failed to create repository reviewer condition", err)
	}
	if response.StatusCode() >= 300 {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	if response.JSON200 != nil {
		return *response.JSON200, nil
	}

	if response.StatusCode() == 201 {
		var created openapigenerated.RestPullRequestCondition
		if err := json.Unmarshal(response.Body, &created); err == nil {
			return created, nil
		}
	}

	return openapigenerated.RestPullRequestCondition{}, nil
}

func (service *Service) UpdateProjectCondition(ctx context.Context, projectKey string, conditionID string, condition openapigenerated.UpdatePullRequestConditionJSONRequestBody) (openapigenerated.RestPullRequestCondition, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(conditionID) == "" {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindValidation, "project key and condition ID are required", nil)
	}

	response, err := service.client.UpdatePullRequestConditionWithResponse(ctx, projectKey, conditionID, condition)
	if err != nil {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindTransient, "failed to update project reviewer condition", err)
	}
	if response.StatusCode() >= 300 {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	if response.JSON200 != nil {
		return *response.JSON200, nil
	}

	return openapigenerated.RestPullRequestCondition{}, nil
}

func (service *Service) UpdateRepositoryCondition(ctx context.Context, projectKey, repositorySlug string, conditionID string, condition openapigenerated.UpdatePullRequestCondition1JSONRequestBody) (openapigenerated.RestPullRequestCondition, error) {
	if strings.TrimSpace(projectKey) == "" || strings.TrimSpace(repositorySlug) == "" || strings.TrimSpace(conditionID) == "" {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindValidation, "project key, repository slug, and condition ID are required", nil)
	}

	response, err := service.client.UpdatePullRequestCondition1WithResponse(ctx, projectKey, repositorySlug, conditionID, condition)
	if err != nil {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindTransient, "failed to update repository reviewer condition", err)
	}
	if response.StatusCode() >= 300 {
		return openapigenerated.RestPullRequestCondition{}, apperrors.New(apperrors.KindPermanent, "bitbucket API returned error: "+string(response.Body), nil)
	}

	if response.JSON200 != nil {
		return *response.JSON200, nil
	}

	return openapigenerated.RestPullRequestCondition{}, nil
}
