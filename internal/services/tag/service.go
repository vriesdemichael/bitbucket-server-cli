package tag

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

type ListOptions struct {
	Limit      int
	OrderBy    string
	FilterText string
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) List(ctx context.Context, repo RepositoryRef, options ListOptions) ([]openapigenerated.RestTag, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	if options.Limit <= 0 {
		options.Limit = 25
	}

	start := float32(0)
	pageLimit := float32(options.Limit)
	results := make([]openapigenerated.RestTag, 0)

	for {
		params := &openapigenerated.GetTagsParams{Start: &start, Limit: &pageLimit}
		if strings.TrimSpace(options.OrderBy) != "" {
			orderBy := strings.TrimSpace(options.OrderBy)
			params.OrderBy = &orderBy
		}
		if strings.TrimSpace(options.FilterText) != "" {
			filterText := strings.TrimSpace(options.FilterText)
			params.FilterText = &filterText
		}

		response, err := service.client.GetTagsWithResponse(ctx, repo.ProjectKey, repo.Slug, params)
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list repository tags", err)
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

func (service *Service) Create(ctx context.Context, repo RepositoryRef, name string, startPoint string, message string) (openapigenerated.RestTag, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return openapigenerated.RestTag{}, err
	}

	trimmedName := strings.TrimSpace(name)
	trimmedStartPoint := strings.TrimSpace(startPoint)
	if trimmedName == "" {
		return openapigenerated.RestTag{}, apperrors.New(apperrors.KindValidation, "tag name is required", nil)
	}
	if trimmedStartPoint == "" {
		return openapigenerated.RestTag{}, apperrors.New(apperrors.KindValidation, "tag start-point is required", nil)
	}

	body := openapigenerated.CreateTagForRepositoryJSONRequestBody{
		Name:       &trimmedName,
		StartPoint: &trimmedStartPoint,
	}
	if strings.TrimSpace(message) != "" {
		trimmedMessage := strings.TrimSpace(message)
		body.Message = &trimmedMessage
	}

	response, err := service.client.CreateTagForRepositoryWithResponse(ctx, repo.ProjectKey, repo.Slug, body)
	if err != nil {
		return openapigenerated.RestTag{}, apperrors.New(apperrors.KindTransient, "failed to create repository tag", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestTag{}, err
	}

	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return openapigenerated.RestTag{}, nil
}

func (service *Service) Get(ctx context.Context, repo RepositoryRef, name string) (openapigenerated.RestTag, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return openapigenerated.RestTag{}, err
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return openapigenerated.RestTag{}, apperrors.New(apperrors.KindValidation, "tag name is required", nil)
	}

	response, err := service.client.GetTagWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedName)
	if err != nil {
		return openapigenerated.RestTag{}, apperrors.New(apperrors.KindTransient, "failed to get repository tag", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestTag{}, err
	}

	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return openapigenerated.RestTag{}, nil
}

func (service *Service) Delete(ctx context.Context, repo RepositoryRef, name string) error {
	if err := validateRepositoryRef(repo); err != nil {
		return err
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return apperrors.New(apperrors.KindValidation, "tag name is required", nil)
	}

	response, err := service.client.DeleteTagWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedName)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to delete repository tag", err)
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
