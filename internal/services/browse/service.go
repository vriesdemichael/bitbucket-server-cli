package browse

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

type TreeOptions struct {
	At    string
	Limit int
}

type FileOptions struct {
	At    string
	Blame bool
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) Tree(ctx context.Context, repo RepositoryRef, path string, options TreeOptions) ([]string, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	trimmedPath := strings.TrimSpace(path)

	if options.Limit <= 0 {
		options.Limit = 1000
	}

	start := float32(0)
	pageLimit := float32(options.Limit)
	var at *string
	if strings.TrimSpace(options.At) != "" {
		a := strings.TrimSpace(options.At)
		at = &a
	}

	results := make([]string, 0)

	for {
		var responseStatus int
		var responseBody []byte
		var responseValues *[]openapigenerated.FileListResource
		var responseIsLastPage *bool
		var responseNextPageStart *int32

		if trimmedPath == "" {
			params := &openapigenerated.StreamFilesParams{Start: &start, Limit: &pageLimit, At: at}
			resp, err := service.client.StreamFilesWithResponse(ctx, repo.ProjectKey, repo.Slug, params)
			if err != nil {
				return nil, apperrors.New(apperrors.KindTransient, "failed to stream repository files", err)
			}
			responseStatus = resp.StatusCode()
			responseBody = resp.Body
			if resp.ApplicationjsonCharsetUTF8200 != nil {
				responseValues = resp.ApplicationjsonCharsetUTF8200.Values
				responseIsLastPage = resp.ApplicationjsonCharsetUTF8200.IsLastPage
				responseNextPageStart = resp.ApplicationjsonCharsetUTF8200.NextPageStart
			}
		} else {
			params := &openapigenerated.StreamFiles1Params{Start: &start, Limit: &pageLimit, At: at}
			resp, err := service.client.StreamFiles1WithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedPath, params)
			if err != nil {
				return nil, apperrors.New(apperrors.KindTransient, "failed to stream repository files", err)
			}
			responseStatus = resp.StatusCode()
			responseBody = resp.Body
			if resp.ApplicationjsonCharsetUTF8200 != nil {
				responseValues = resp.ApplicationjsonCharsetUTF8200.Values
				responseIsLastPage = resp.ApplicationjsonCharsetUTF8200.IsLastPage
				responseNextPageStart = resp.ApplicationjsonCharsetUTF8200.NextPageStart
			}
		}

		if err := mapStatusError(responseStatus, responseBody); err != nil {
			return nil, err
		}

		if responseValues == nil {
			break
		}

		for _, val := range *responseValues {
			if strVal, ok := val.(string); ok {
				results = append(results, strVal)
			}
		}

		if responseIsLastPage != nil && *responseIsLastPage {
			break
		}
		if responseNextPageStart == nil {
			break
		}

		start = float32(*responseNextPageStart)
	}

	return results, nil
}

func (service *Service) Raw(ctx context.Context, repo RepositoryRef, path string, at string) ([]byte, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, apperrors.New(apperrors.KindValidation, "path is required", nil)
	}

	var atParam *string
	if strings.TrimSpace(at) != "" {
		a := strings.TrimSpace(at)
		atParam = &a
	}

	params := &openapigenerated.StreamRawParams{At: atParam}
	resp, err := service.client.StreamRawWithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedPath, params)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to get raw content", err)
	}

	if err := mapStatusError(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (service *Service) File(ctx context.Context, repo RepositoryRef, path string, options FileOptions) ([]byte, error) {
	if err := validateRepositoryRef(repo); err != nil {
		return nil, err
	}

	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, apperrors.New(apperrors.KindValidation, "path is required", nil)
	}

	var atParam *string
	if strings.TrimSpace(options.At) != "" {
		a := strings.TrimSpace(options.At)
		atParam = &a
	}

	var blameParam *string
	if options.Blame {
		b := "true"
		blameParam = &b
	}

	params := &openapigenerated.GetContent1Params{At: atParam, Blame: blameParam}
	resp, err := service.client.GetContent1WithResponse(ctx, repo.ProjectKey, repo.Slug, trimmedPath, params)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to get file content", err)
	}

	if err := mapStatusError(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}

	return resp.Body, nil
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
