package pullrequestactivity

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type RepositoryRef struct {
	ProjectKey string `json:"project_key"`
	Slug       string `json:"slug"`
}

type ListOptions struct {
	Limit int `json:"limit"`
	Start int `json:"start"`
}

type Activity struct {
	ID          int64                         `json:"id,omitempty"`
	Action      string                        `json:"action,omitempty"`
	CreatedDate int64                         `json:"created_date,omitempty"`
	Comment     *openapigenerated.RestComment `json:"comment,omitempty"`
	Raw         map[string]any                `json:"raw"`
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) List(ctx context.Context, repository RepositoryRef, pullRequestID string, options ListOptions) ([]Activity, error) {
	if err := validateRepositoryRef(repository); err != nil {
		return nil, err
	}

	resolvedID, err := normalizePullRequestID(pullRequestID)
	if err != nil {
		return nil, err
	}

	if options.Limit <= 0 {
		options.Limit = 25
	}
	if options.Start < 0 {
		return nil, apperrors.New(apperrors.KindValidation, "start must be greater than or equal to 0", nil)
	}

	start := float32(options.Start)
	limit := float32(options.Limit)
	results := make([]Activity, 0)

	for {
		response, err := service.client.GetActivitiesWithResponse(ctx, repository.ProjectKey, repository.Slug, resolvedID, &openapigenerated.GetActivitiesParams{Start: &start, Limit: &limit})
		if err != nil {
			return nil, apperrors.New(apperrors.KindTransient, "failed to list pull request activities", err)
		}
		if response.StatusCode() >= 400 {
			return nil, mapActivityStatusError(response.StatusCode(), response.Body)
		}

		page, err := decodeActivityPage(response.Body)
		if err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "failed to decode pull request activities", err)
		}

		results = append(results, page.Values...)
		if page.IsLastPage || page.NextPageStart == nil {
			break
		}
		if int(*page.NextPageStart) == int(start) {
			break
		}
		start = float32(*page.NextPageStart)
	}

	return results, nil
}

func ExtractComments(activities []Activity) []openapigenerated.RestComment {
	comments := make([]openapigenerated.RestComment, 0)
	seen := map[int64]struct{}{}

	for _, activity := range activities {
		if activity.Comment == nil {
			continue
		}

		comment := *activity.Comment
		if comment.Id != nil {
			if _, ok := seen[*comment.Id]; ok {
				continue
			}
			seen[*comment.Id] = struct{}{}
		}

		comments = append(comments, comment)
	}

	return comments
}

type activityPage struct {
	IsLastPage    bool       `json:"isLastPage"`
	NextPageStart *int       `json:"nextPageStart,omitempty"`
	Values        []Activity `json:"values"`
}

type rawActivity struct {
	ID          *int64                     `json:"id,omitempty"`
	Action      *string                    `json:"action,omitempty"`
	CreatedDate *int64                     `json:"createdDate,omitempty"`
	Comment     *json.RawMessage           `json:"comment,omitempty"`
	Raw         map[string]json.RawMessage `json:"-"`
}

func (activity *rawActivity) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	activity.Raw = raw

	type alias rawActivity
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*activity = rawActivity(decoded)
	activity.Raw = raw
	return nil
}

type rawActivityPage struct {
	IsLastPage    bool          `json:"isLastPage"`
	NextPageStart *int          `json:"nextPageStart,omitempty"`
	Values        []rawActivity `json:"values"`
}

func decodeActivityPage(body []byte) (activityPage, error) {
	page := rawActivityPage{}
	if err := json.Unmarshal(body, &page); err != nil {
		return activityPage{}, err
	}

	results := make([]Activity, 0, len(page.Values))
	for _, item := range page.Values {
		mapped, err := mapActivity(item)
		if err != nil {
			return activityPage{}, err
		}
		results = append(results, mapped)
	}

	return activityPage{IsLastPage: page.IsLastPage, NextPageStart: page.NextPageStart, Values: results}, nil
}

func mapActivity(item rawActivity) (Activity, error) {
	mapped := Activity{
		ID:          safeInt64(item.ID),
		Action:      safeString(item.Action),
		CreatedDate: safeInt64(item.CreatedDate),
		Raw:         map[string]any{},
	}

	for key, value := range item.Raw {
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			return Activity{}, err
		}
		mapped.Raw[key] = decoded
	}

	if item.Comment != nil {
		comment := openapigenerated.RestComment{}
		if err := json.Unmarshal(*item.Comment, &comment); err != nil {
			return Activity{}, err
		}
		mapped.Comment = &comment
	}

	return mapped, nil
}

func validateRepositoryRef(repository RepositoryRef) error {
	if strings.TrimSpace(repository.ProjectKey) == "" || strings.TrimSpace(repository.Slug) == "" {
		return apperrors.New(apperrors.KindValidation, "repository must be specified as project/repo", nil)
	}

	return nil
}

func normalizePullRequestID(pullRequestID string) (string, error) {
	trimmed := strings.TrimSpace(pullRequestID)
	if trimmed == "" {
		return "", apperrors.New(apperrors.KindValidation, "pull request id is required", nil)
	}
	if _, err := strconv.Atoi(trimmed); err != nil {
		return "", apperrors.New(apperrors.KindValidation, "pull request id must be a number", err)
	}

	return trimmed, nil
}

func mapActivityStatusError(statusCode int, body []byte) error {
	if statusCode == 404 {
		return apperrors.New(apperrors.KindNotFound, "pull request activity not found", nil)
	}
	if statusCode == 400 {
		return apperrors.New(apperrors.KindValidation, fmt.Sprintf("pull request activity request failed with status %d", statusCode), nil)
	}
	if statusCode >= 500 {
		return apperrors.New(apperrors.KindInternal, fmt.Sprintf("pull request activity request failed with status %d", statusCode), nil)
	}
	if len(body) > 0 {
		return apperrors.New(apperrors.KindInternal, fmt.Sprintf("pull request activity request failed with status %d", statusCode), nil)
	}

	return apperrors.New(apperrors.KindInternal, fmt.Sprintf("pull request activity request failed with status %d", statusCode), nil)
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func safeInt64(value *int64) int64 {
	if value == nil {
		return 0
	}

	return *value
}
