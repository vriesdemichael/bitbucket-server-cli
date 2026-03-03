package repository

import (
	"context"
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

type CreateInput struct {
	Name          string
	Description   string
	Forkable      bool
	DefaultBranch string
}

type ForkInput struct {
	Name    string
	Project string
}

type UpdateInput struct {
	Name          string
	Description   string
	DefaultBranch string
}

type AdminService struct {
	client *openapigenerated.ClientWithResponses
}

func NewAdminService(client *openapigenerated.ClientWithResponses) *AdminService {
	return &AdminService{client: client}
}

func (service *AdminService) Create(ctx context.Context, projectKey string, input CreateInput) (openapigenerated.RestRepository, error) {
	trimmedProject := strings.TrimSpace(projectKey)
	if trimmedProject == "" {
		return openapigenerated.RestRepository{}, apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}

	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" {
		return openapigenerated.RestRepository{}, apperrors.New(apperrors.KindValidation, "repository name is required", nil)
	}

	scmId := "git"
	body := openapigenerated.CreateRepositoryJSONRequestBody{
		Name:     &trimmedName,
		ScmId:    &scmId,
		Forkable: &input.Forkable,
	}

	if trimmedDesc := strings.TrimSpace(input.Description); trimmedDesc != "" {
		body.Description = &trimmedDesc
	}
	if trimmedBranch := strings.TrimSpace(input.DefaultBranch); trimmedBranch != "" {
		body.DefaultBranch = &trimmedBranch
	}

	response, err := service.client.CreateRepositoryWithResponse(ctx, trimmedProject, body)
	if err != nil {
		return openapigenerated.RestRepository{}, apperrors.New(apperrors.KindTransient, "failed to create repository", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestRepository{}, err
	}

	if response.ApplicationjsonCharsetUTF8201 != nil {
		return *response.ApplicationjsonCharsetUTF8201, nil
	}

	return openapigenerated.RestRepository{}, nil
}

func (service *AdminService) Fork(ctx context.Context, repo RepositoryRef, input ForkInput) (openapigenerated.RestRepository, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return openapigenerated.RestRepository{}, err
	}

	body := openapigenerated.ForkRepositoryJSONRequestBody{}
	if trimmedName := strings.TrimSpace(input.Name); trimmedName != "" {
		body.Name = &trimmedName
	}

	if trimmedProject := strings.TrimSpace(input.Project); trimmedProject != "" {
		body.Project = &struct {
			Avatar      *string                                     `json:"avatar,omitempty"`
			AvatarUrl   *string                                     `json:"avatarUrl,omitempty"`
			Description *string                                     `json:"description,omitempty"`
			Id          *int32                                      `json:"id,omitempty"`
			Key         string                                      `json:"key"`
			Links       *map[string]interface{}                     `json:"links,omitempty"`
			Name        *string                                     `json:"name,omitempty"`
			Public      *bool                                       `json:"public,omitempty"`
			Scope       *string                                     `json:"scope,omitempty"`
			Type        *openapigenerated.RestRepositoryProjectType `json:"type,omitempty"`
		}{Key: trimmedProject}
	}

	response, err := service.client.ForkRepositoryWithResponse(ctx, repo.ProjectKey, repo.Slug, body)
	if err != nil {
		return openapigenerated.RestRepository{}, apperrors.New(apperrors.KindTransient, "failed to fork repository", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestRepository{}, err
	}

	if response.ApplicationjsonCharsetUTF8201 != nil {
		return *response.ApplicationjsonCharsetUTF8201, nil
	}

	return openapigenerated.RestRepository{}, nil
}

func (service *AdminService) Delete(ctx context.Context, repo RepositoryRef) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}

	response, err := service.client.DeleteRepositoryWithResponse(ctx, repo.ProjectKey, repo.Slug)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete repository", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
}

func (service *AdminService) Update(ctx context.Context, repo RepositoryRef, input UpdateInput) (openapigenerated.RestRepository, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return openapigenerated.RestRepository{}, err
	}

	body := openapigenerated.UpdateRepositoryJSONRequestBody{}
	if trimmedName := strings.TrimSpace(input.Name); trimmedName != "" {
		body.Name = &trimmedName
	}
	if trimmedDesc := strings.TrimSpace(input.Description); trimmedDesc != "" {
		body.Description = &trimmedDesc
	}
	if trimmedBranch := strings.TrimSpace(input.DefaultBranch); trimmedBranch != "" {
		body.DefaultBranch = &trimmedBranch
	}

	response, err := service.client.UpdateRepositoryWithResponse(ctx, repo.ProjectKey, repo.Slug, body)
	if err != nil {
		return openapigenerated.RestRepository{}, apperrors.New(apperrors.KindTransient, "failed to update repository", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestRepository{}, err
	}

	if response.ApplicationjsonCharsetUTF8201 != nil {
		return *response.ApplicationjsonCharsetUTF8201, nil
	}

	return openapigenerated.RestRepository{}, nil
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
