package comment

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type RepositoryRef struct {
	ProjectKey string
	Slug       string
}

type Context struct {
	Type           string `json:"type"`
	ProjectKey     string `json:"project_key"`
	RepositorySlug string `json:"repository_slug"`
	CommitID       string `json:"commit_id,omitempty"`
	PullRequestID  string `json:"pull_request_id,omitempty"`
}

type Target struct {
	Repository    RepositoryRef
	CommitID      string
	PullRequestID string
}

func (target Target) Context() Context {
	ctx := Context{
		ProjectKey:     target.Repository.ProjectKey,
		RepositorySlug: target.Repository.Slug,
	}

	if strings.TrimSpace(target.CommitID) != "" {
		ctx.Type = "commit"
		ctx.CommitID = target.CommitID
		return ctx
	}

	ctx.Type = "pull_request"
	ctx.PullRequestID = target.PullRequestID
	return ctx
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) List(ctx context.Context, target Target, path string, limit int) ([]openapigenerated.RestComment, error) {
	if err := validateTarget(target); err != nil {
		return nil, err
	}
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, apperrors.New(apperrors.KindValidation, "comment path is required for list operations", nil)
	}
	if limit <= 0 {
		limit = 25
	}

	start := float32(0)
	pageLimit := float32(limit)
	results := make([]openapigenerated.RestComment, 0)

	for {
		if strings.TrimSpace(target.CommitID) != "" {
			response, err := service.client.GetCommentsWithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.CommitID, &openapigenerated.GetCommentsParams{Path: &trimmedPath, Start: &start, Limit: &pageLimit})
			if err != nil {
				return nil, apperrors.New(apperrors.KindTransient, "failed to list commit comments", err)
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
			continue
		}

		response, err := service.client.GetComments2WithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.PullRequestID, &openapigenerated.GetComments2Params{Path: trimmedPath, Start: &start, Limit: &pageLimit})
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list pull request comments", err)
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

func (service *Service) Create(ctx context.Context, target Target, text string) (openapigenerated.RestComment, error) {
	if err := validateTarget(target); err != nil {
		return openapigenerated.RestComment{}, err
	}

	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return openapigenerated.RestComment{}, apperrors.New(apperrors.KindValidation, "comment text is required", nil)
	}

	body := openapigenerated.RestComment{Text: &trimmedText}

	if strings.TrimSpace(target.CommitID) != "" {
		response, err := service.client.CreateCommentWithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.CommitID, nil, body)
		if err != nil {
			return openapigenerated.RestComment{}, apperrors.New(apperrors.KindTransient, "failed to create commit comment", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return openapigenerated.RestComment{}, err
		}
		if response.ApplicationjsonCharsetUTF8201 != nil {
			return *response.ApplicationjsonCharsetUTF8201, nil
		}
		return body, nil
	}

	response, err := service.client.CreateComment2WithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.PullRequestID, body)
	if err != nil {
		return openapigenerated.RestComment{}, apperrors.New(apperrors.KindTransient, "failed to create pull request comment", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestComment{}, err
	}
	if response.ApplicationjsonCharsetUTF8201 != nil {
		return *response.ApplicationjsonCharsetUTF8201, nil
	}

	return body, nil
}

func (service *Service) Update(ctx context.Context, target Target, commentID string, text string, version *int32) (openapigenerated.RestComment, error) {
	if err := validateTarget(target); err != nil {
		return openapigenerated.RestComment{}, err
	}

	trimmedCommentID := strings.TrimSpace(commentID)
	if trimmedCommentID == "" {
		return openapigenerated.RestComment{}, apperrors.New(apperrors.KindValidation, "comment id is required", nil)
	}

	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return openapigenerated.RestComment{}, apperrors.New(apperrors.KindValidation, "comment text is required", nil)
	}

	resolvedVersion := version
	if resolvedVersion == nil {
		current, err := service.Get(ctx, target, trimmedCommentID)
		if err != nil {
			return openapigenerated.RestComment{}, err
		}
		resolvedVersion = current.Version
	}

	body := openapigenerated.RestComment{Text: &trimmedText, Version: resolvedVersion}

	if strings.TrimSpace(target.CommitID) != "" {
		response, err := service.client.UpdateCommentWithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.CommitID, trimmedCommentID, body)
		if err != nil {
			return openapigenerated.RestComment{}, apperrors.New(apperrors.KindTransient, "failed to update commit comment", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return openapigenerated.RestComment{}, err
		}
		if response.ApplicationjsonCharsetUTF8200 != nil {
			return *response.ApplicationjsonCharsetUTF8200, nil
		}
		return body, nil
	}

	response, err := service.client.UpdateComment2WithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.PullRequestID, trimmedCommentID, body)
	if err != nil {
		return openapigenerated.RestComment{}, apperrors.New(apperrors.KindTransient, "failed to update pull request comment", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestComment{}, err
	}
	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return body, nil
}

func (service *Service) Delete(ctx context.Context, target Target, commentID string, version *int32) (*int32, error) {
	if err := validateTarget(target); err != nil {
		return nil, err
	}

	trimmedCommentID := strings.TrimSpace(commentID)
	if trimmedCommentID == "" {
		return nil, apperrors.New(apperrors.KindValidation, "comment id is required", nil)
	}

	resolvedVersion := version
	if resolvedVersion == nil {
		current, err := service.Get(ctx, target, trimmedCommentID)
		if err != nil {
			return nil, err
		}
		resolvedVersion = current.Version
	}

	var versionParam *string
	if resolvedVersion != nil {
		value := strconv.Itoa(int(*resolvedVersion))
		versionParam = &value
	}

	if strings.TrimSpace(target.CommitID) != "" {
		response, err := service.client.DeleteCommentWithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.CommitID, trimmedCommentID, &openapigenerated.DeleteCommentParams{Version: versionParam})
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to delete commit comment", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return nil, err
		}
		return resolvedVersion, nil
	}

	response, err := service.client.DeleteComment2WithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.PullRequestID, trimmedCommentID, &openapigenerated.DeleteComment2Params{Version: versionParam})
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to delete pull request comment", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return nil, err
	}

	return resolvedVersion, nil
}

func (service *Service) Get(ctx context.Context, target Target, commentID string) (openapigenerated.RestComment, error) {
	if err := validateTarget(target); err != nil {
		return openapigenerated.RestComment{}, err
	}

	trimmedCommentID := strings.TrimSpace(commentID)
	if trimmedCommentID == "" {
		return openapigenerated.RestComment{}, apperrors.New(apperrors.KindValidation, "comment id is required", nil)
	}

	if strings.TrimSpace(target.CommitID) != "" {
		response, err := service.client.GetCommentWithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.CommitID, trimmedCommentID)
		if err != nil {
			return openapigenerated.RestComment{}, apperrors.New(apperrors.KindTransient, "failed to get commit comment", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return openapigenerated.RestComment{}, err
		}
		if response.ApplicationjsonCharsetUTF8200 != nil {
			return *response.ApplicationjsonCharsetUTF8200, nil
		}
		return openapigenerated.RestComment{}, nil
	}

	response, err := service.client.GetComment2WithResponse(ctx, target.Repository.ProjectKey, target.Repository.Slug, target.PullRequestID, trimmedCommentID)
	if err != nil {
		return openapigenerated.RestComment{}, apperrors.New(apperrors.KindTransient, "failed to get pull request comment", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return openapigenerated.RestComment{}, err
	}
	if response.ApplicationjsonCharsetUTF8200 != nil {
		return *response.ApplicationjsonCharsetUTF8200, nil
	}

	return openapigenerated.RestComment{}, nil
}

func validateTarget(target Target) error {
	if strings.TrimSpace(target.Repository.ProjectKey) == "" || strings.TrimSpace(target.Repository.Slug) == "" {
		return apperrors.New(apperrors.KindValidation, "repository must be specified as project/repo", nil)
	}

	hasCommit := strings.TrimSpace(target.CommitID) != ""
	hasPullRequest := strings.TrimSpace(target.PullRequestID) != ""

	if hasCommit == hasPullRequest {
		return apperrors.New(apperrors.KindValidation, "exactly one of commit or pull request id is required", nil)
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
