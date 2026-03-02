package pullrequest

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

type RepositoryRef struct {
	ProjectKey string `json:"project_key"`
	Slug       string `json:"slug"`
}

type ListOptions struct {
	State        string `json:"state"`
	Limit        int    `json:"limit"`
	Start        int    `json:"start"`
	SourceBranch string `json:"source_branch,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
}

type PullRequest struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	State        string `json:"state"`
	Open         bool   `json:"open"`
	Closed       bool   `json:"closed"`
	Author       string `json:"author,omitempty"`
	SourceBranch string `json:"source_branch,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
	CreatedDate  int64  `json:"created_date,omitempty"`
	UpdatedDate  int64  `json:"updated_date,omitempty"`
}

type Service struct {
	client *httpclient.Client
}

func NewService(client *httpclient.Client) *Service {
	return &Service{client: client}
}

func (service *Service) List(ctx context.Context, repository RepositoryRef, options ListOptions) ([]PullRequest, error) {
	if strings.TrimSpace(repository.ProjectKey) == "" || strings.TrimSpace(repository.Slug) == "" {
		return nil, apperrors.New(apperrors.KindValidation, "repository must be specified as project/repo", nil)
	}

	normalizedState, err := normalizeState(options.State)
	if err != nil {
		return nil, err
	}

	if options.Limit <= 0 {
		options.Limit = 25
	}
	if options.Start < 0 {
		return nil, apperrors.New(apperrors.KindValidation, "start must be greater than or equal to 0", nil)
	}

	path := fmt.Sprintf("/rest/api/latest/projects/%s/repos/%s/pull-requests", repository.ProjectKey, repository.Slug)
	results := make([]PullRequest, 0)
	start := options.Start

	for {
		query := map[string]string{
			"limit": strconv.Itoa(options.Limit),
			"start": strconv.Itoa(start),
		}
		if normalizedState == "open" {
			query["state"] = "OPEN"
		} else {
			query["state"] = "ALL"
		}

		var response pagedPullRequestResponse
		if err := service.client.GetJSON(ctx, path, query, &response); err != nil {
			return nil, err
		}

		for _, value := range response.Values {
			mapped := mapPullRequest(value)
			if matchesFilters(mapped, normalizedState, options.SourceBranch, options.TargetBranch) {
				results = append(results, mapped)
			}
		}

		if response.IsLastPage {
			break
		}

		if response.NextPageStart == start {
			break
		}

		start = response.NextPageStart
	}

	return results, nil
}

func normalizeState(state string) (string, error) {
	resolved := strings.ToLower(strings.TrimSpace(state))
	if resolved == "" {
		return "open", nil
	}

	switch resolved {
	case "open", "closed", "all":
		return resolved, nil
	default:
		return "", apperrors.New(apperrors.KindValidation, "--state must be one of: open, closed, all", nil)
	}
}

func matchesFilters(pullRequest PullRequest, state string, sourceBranch string, targetBranch string) bool {
	switch state {
	case "open":
		if !pullRequest.Open {
			return false
		}
	case "closed":
		if pullRequest.Open && !pullRequest.Closed {
			return false
		}
	}

	if !branchMatches(sourceBranch, pullRequest.SourceBranch) {
		return false
	}

	if !branchMatches(targetBranch, pullRequest.TargetBranch) {
		return false
	}

	return true
}

func branchMatches(filter string, actual string) bool {
	trimmedFilter := strings.TrimSpace(filter)
	if trimmedFilter == "" {
		return true
	}

	return normalizeBranch(trimmedFilter) == normalizeBranch(actual)
}

func normalizeBranch(branch string) string {
	trimmed := strings.TrimSpace(branch)
	trimmed = strings.TrimPrefix(trimmed, "refs/heads/")
	return strings.ToLower(trimmed)
}

func mapPullRequest(raw pullRequestValue) PullRequest {
	author := ""
	if raw.Author != nil && raw.Author.User != nil {
		author = strings.TrimSpace(raw.Author.User.DisplayName)
		if author == "" {
			author = strings.TrimSpace(raw.Author.User.Name)
		}
	}

	return PullRequest{
		ID:           raw.ID,
		Title:        raw.Title,
		State:        strings.TrimSpace(raw.State),
		Open:         raw.Open,
		Closed:       raw.Closed,
		Author:       author,
		SourceBranch: branchDisplayName(raw.FromRef),
		TargetBranch: branchDisplayName(raw.ToRef),
		CreatedDate:  raw.CreatedDate,
		UpdatedDate:  raw.UpdatedDate,
	}
}

func branchDisplayName(reference *pullRequestRef) string {
	if reference == nil {
		return ""
	}

	display := strings.TrimSpace(reference.DisplayID)
	if display != "" {
		return display
	}

	return strings.TrimSpace(reference.ID)
}

type pagedPullRequestResponse struct {
	Values        []pullRequestValue `json:"values"`
	IsLastPage    bool               `json:"isLastPage"`
	NextPageStart int                `json:"nextPageStart"`
}

type pullRequestValue struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	State       string           `json:"state"`
	Open        bool             `json:"open"`
	Closed      bool             `json:"closed"`
	CreatedDate int64            `json:"createdDate"`
	UpdatedDate int64            `json:"updatedDate"`
	Author      *pullRequestUser `json:"author"`
	FromRef     *pullRequestRef  `json:"fromRef"`
	ToRef       *pullRequestRef  `json:"toRef"`
}

type pullRequestUser struct {
	User *struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"user"`
}

type pullRequestRef struct {
	ID        string `json:"id"`
	DisplayID string `json:"displayId"`
}
