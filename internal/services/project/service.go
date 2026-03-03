package project

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type ListOptions struct {
	Limit int
	Name  string
}

type CreateInput struct {
	Key         string
	Name        string
	Description string
}

type UpdateInput struct {
	Name        string
	Description string
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) List(ctx context.Context, options ListOptions) ([]openapigenerated.RestProject, error) {
	if options.Limit <= 0 {
		options.Limit = 25
	}

	start := float32(0)
	pageLimit := float32(options.Limit)
	results := make([]openapigenerated.RestProject, 0)

	for {
		params := &openapigenerated.GetProjectsParams{Start: &start, Limit: &pageLimit}
		if strings.TrimSpace(options.Name) != "" {
			name := strings.TrimSpace(options.Name)
			params.Name = &name
		}

		response, err := service.client.GetProjectsWithResponse(ctx, params)
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list projects", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return nil, err
		}
		if response.ApplicationjsonCharsetUTF8200 == nil || response.ApplicationjsonCharsetUTF8200.Values == nil {
			break
		}

		results = append(results, (*response.ApplicationjsonCharsetUTF8200.Values)...)

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

func (service *Service) Get(ctx context.Context, key string) (openapigenerated.RestProject, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return openapigenerated.RestProject{}, apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}

	response, err := service.client.GetProjectWithResponse(ctx, trimmedKey)
	if err != nil {
		return openapigenerated.RestProject{}, apperrors.New(apperrors.KindTransient, "failed to get project", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestProject{}, err
	}

	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return openapigenerated.RestProject{}, nil
}

func (service *Service) Create(ctx context.Context, input CreateInput) (openapigenerated.RestProject, error) {
	trimmedKey := strings.TrimSpace(input.Key)
	if trimmedKey == "" {
		return openapigenerated.RestProject{}, apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}

	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" {
		return openapigenerated.RestProject{}, apperrors.New(apperrors.KindValidation, "project name is required", nil)
	}

	body := openapigenerated.CreateProjectJSONRequestBody{
		Key:  &trimmedKey,
		Name: &trimmedName,
	}
	if trimmedDesc := strings.TrimSpace(input.Description); trimmedDesc != "" {
		body.Description = &trimmedDesc
	}

	response, err := service.client.CreateProjectWithResponse(ctx, body)
	if err != nil {
		return openapigenerated.RestProject{}, apperrors.New(apperrors.KindTransient, "failed to create project", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestProject{}, err
	}

	if response.ApplicationjsonCharsetUTF8201 != nil {
		return *response.ApplicationjsonCharsetUTF8201, nil
	}

	return openapigenerated.RestProject{}, nil
}

func (service *Service) Update(ctx context.Context, key string, input UpdateInput) (openapigenerated.RestProject, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return openapigenerated.RestProject{}, apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}

	body := openapigenerated.UpdateProjectJSONRequestBody{}
	if trimmedName := strings.TrimSpace(input.Name); trimmedName != "" {
		body.Name = &trimmedName
	}
	if trimmedDesc := strings.TrimSpace(input.Description); trimmedDesc != "" {
		body.Description = &trimmedDesc
	}

	response, err := service.client.UpdateProjectWithResponse(ctx, trimmedKey, body)
	if err != nil {
		return openapigenerated.RestProject{}, apperrors.New(apperrors.KindTransient, "failed to update project", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestProject{}, err
	}

	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return openapigenerated.RestProject{}, nil
}

func (service *Service) Delete(ctx context.Context, key string) error {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return apperrors.New(apperrors.KindValidation, "project key is required", nil)
	}

	response, err := service.client.DeleteProjectWithResponse(ctx, trimmedKey)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete project", err)
	}

	return mapStatusError(response.StatusCode(), response.Body)
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
