package cli

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func newPRCommand(options *rootOptions) *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Pull request commands",
	}

	var repositorySelector string
	var state string
	var role string
	var order string
	var user string
	var start int
	var limit int

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			params := &openapigenerated.GetPullRequests1Params{}
			if strings.TrimSpace(state) != "" {
				upper := strings.ToUpper(strings.TrimSpace(state))
				params.State = &upper
			}
			if strings.TrimSpace(role) != "" {
				upper := strings.ToUpper(strings.TrimSpace(role))
				params.Role = &upper
			}
			if strings.TrimSpace(order) != "" {
				upper := strings.ToUpper(strings.TrimSpace(order))
				params.Order = &upper
			}
			if strings.TrimSpace(user) != "" {
				trimmedUser := strings.TrimSpace(user)
				params.User = &trimmedUser
			}
			if start > 0 {
				startValue := float32(start)
				params.Start = &startValue
			}
			if limit <= 0 {
				limit = 25
			}
			limitValue := float32(limit)
			params.Limit = &limitValue

			response, err := client.GetPullRequests1WithResponse(cmd.Context(), params)
			if err != nil {
				return apperrors.New(apperrors.KindTransient, "failed to list pull requests", err)
			}
			if err := mapStatusErrorFromBody(response.StatusCode(), response.Body); err != nil {
				return err
			}

			pullRequests := make([]openapigenerated.RestPullRequest, 0)
			isLastPage := true
			nextPageStart := 0
			if response.ApplicationjsonCharsetUTF8200 != nil {
				if response.ApplicationjsonCharsetUTF8200.Values != nil {
					pullRequests = append(pullRequests, (*response.ApplicationjsonCharsetUTF8200.Values)...)
				}
				if response.ApplicationjsonCharsetUTF8200.IsLastPage != nil {
					isLastPage = *response.ApplicationjsonCharsetUTF8200.IsLastPage
				}
				if response.ApplicationjsonCharsetUTF8200.NextPageStart != nil {
					nextPageStart = int(*response.ApplicationjsonCharsetUTF8200.NextPageStart)
				}
			}

			if strings.TrimSpace(repositorySelector) != "" {
				repo, err := resolveRepositorySelector(repositorySelector, cfg)
				if err != nil {
					return err
				}

				filtered := make([]openapigenerated.RestPullRequest, 0, len(pullRequests))
				for _, pullRequest := range pullRequests {
					if pullRequestMatchesRepository(pullRequest, repo.ProjectKey, repo.Slug) {
						filtered = append(filtered, pullRequest)
					}
				}
				pullRequests = filtered
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"pull_requests":   pullRequests,
					"count":           len(pullRequests),
					"is_last_page":    isLastPage,
					"next_page_start": nextPageStart,
				})
			}

			if len(pullRequests) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No pull requests found")
				return nil
			}

			for _, pullRequest := range pullRequests {
				fromDisplayID := safePullRequestFromRefDisplayID(pullRequest.FromRef)
				toDisplayID := safePullRequestToRefDisplayID(pullRequest.ToRef)
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%s\t%s -> %s\t%s\n",
					pullRequestIDString(pullRequest),
					safeStringFromPullRequestState(pullRequest.State),
					fromDisplayID,
					toDisplayID,
					safeString(pullRequest.Title),
				)
			}

			return nil
		},
	}
	listCmd.Flags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (filters dashboard pull requests)")
	listCmd.Flags().StringVar(&state, "state", "", "Filter by state: OPEN, DECLINED, MERGED")
	listCmd.Flags().StringVar(&role, "role", "", "Filter by role: REVIEWER, AUTHOR, PARTICIPANT")
	listCmd.Flags().StringVar(&order, "order", "", "Order: NEWEST, OLDEST, DRAFT_STATUS, PARTICIPANT_STATUS, CLOSED_DATE")
	listCmd.Flags().StringVar(&user, "user", "", "User to list pull requests for (defaults to current user)")
	listCmd.Flags().IntVar(&start, "start", 0, "Start index for pagination")
	listCmd.Flags().IntVar(&limit, "limit", 25, "Page size for pull request list operations")
	prCmd.AddCommand(listCmd)

	return prCmd
}

func newIssueCommand(options *rootOptions) *cobra.Command {
	issueCmd := &cobra.Command{
		Use:   "issue",
		Short: "Issue commands",
	}

	var repositorySelector string
	var pullRequestID string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List issue keys linked to a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySelector(repositorySelector, cfg)
			if err != nil {
				return err
			}

			trimmedPullRequestID := strings.TrimSpace(pullRequestID)
			if trimmedPullRequestID == "" {
				return apperrors.New(apperrors.KindValidation, "--pr is required", nil)
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			response, err := client.GetIssueKeysForPullRequestWithResponse(
				cmd.Context(),
				repo.ProjectKey,
				repo.Slug,
				trimmedPullRequestID,
			)
			if err != nil {
				return apperrors.New(apperrors.KindTransient, "failed to list issue keys for pull request", err)
			}
			if err := mapStatusErrorFromBody(response.StatusCode(), response.Body); err != nil {
				return err
			}

			issues := make([]openapigenerated.RestJiraIssue, 0)
			if response.ApplicationjsonCharsetUTF8200 != nil {
				issues = append(issues, (*response.ApplicationjsonCharsetUTF8200)...)
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"issues": issues, "count": len(issues)})
			}

			if len(issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No issues found")
				return nil
			}

			for _, issue := range issues {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", safeString(issue.Key), safeString(issue.Url))
			}

			return nil
		},
	}
	listCmd.Flags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	listCmd.Flags().StringVar(&pullRequestID, "pr", "", "Pull request ID context")
	_ = listCmd.MarkFlagRequired("pr")
	issueCmd.AddCommand(listCmd)

	return issueCmd
}

func pullRequestMatchesRepository(pullRequest openapigenerated.RestPullRequest, projectKey string, slug string) bool {
	targetProjectKey := strings.TrimSpace(projectKey)
	targetSlug := strings.TrimSpace(slug)
	if targetProjectKey == "" || targetSlug == "" {
		return false
	}

	if pullRequest.ToRef != nil && pullRequest.ToRef.Repository != nil && pullRequest.ToRef.Repository.Project != nil {
		if strings.EqualFold(strings.TrimSpace(pullRequest.ToRef.Repository.Project.Key), targetProjectKey) && strings.EqualFold(strings.TrimSpace(safeString(pullRequest.ToRef.Repository.Slug)), targetSlug) {
			return true
		}
	}

	if pullRequest.FromRef != nil && pullRequest.FromRef.Repository != nil && pullRequest.FromRef.Repository.Project != nil {
		if strings.EqualFold(strings.TrimSpace(pullRequest.FromRef.Repository.Project.Key), targetProjectKey) && strings.EqualFold(strings.TrimSpace(safeString(pullRequest.FromRef.Repository.Slug)), targetSlug) {
			return true
		}
	}

	return false
}

func pullRequestIDString(pullRequest openapigenerated.RestPullRequest) string {
	if pullRequest.Id == nil {
		return "unknown"
	}

	return strconv.FormatInt(*pullRequest.Id, 10)
}

func safePullRequestFromRefDisplayID(reference *struct {
	DisplayId    *string `json:"displayId,omitempty"`
	Id           *string `json:"id,omitempty"`
	LatestCommit *string `json:"latestCommit,omitempty"`
	Repository   *struct {
		Archived      *bool                   `json:"archived,omitempty"`
		DefaultBranch *string                 `json:"defaultBranch,omitempty"`
		Description   *string                 `json:"description,omitempty"`
		Forkable      *bool                   `json:"forkable,omitempty"`
		HierarchyId   *string                 `json:"hierarchyId,omitempty"`
		Id            *int32                  `json:"id,omitempty"`
		Links         *map[string]interface{} `json:"links,omitempty"`
		Name          *string                 `json:"name,omitempty"`
		Origin        *struct {
			Archived      *bool                   `json:"archived,omitempty"`
			DefaultBranch *string                 `json:"defaultBranch,omitempty"`
			Description   *string                 `json:"description,omitempty"`
			Forkable      *bool                   `json:"forkable,omitempty"`
			HierarchyId   *string                 `json:"hierarchyId,omitempty"`
			Id            *int32                  `json:"id,omitempty"`
			Links         *map[string]interface{} `json:"links,omitempty"`
			Name          *string                 `json:"name,omitempty"`
			Partition     *int32                  `json:"partition,omitempty"`
			Project       *struct {
				Avatar      *string                                                             `json:"avatar,omitempty"`
				AvatarUrl   *string                                                             `json:"avatarUrl,omitempty"`
				Description *string                                                             `json:"description,omitempty"`
				Id          *int32                                                              `json:"id,omitempty"`
				Key         string                                                              `json:"key"`
				Links       *map[string]interface{}                                             `json:"links,omitempty"`
				Name        *string                                                             `json:"name,omitempty"`
				Public      *bool                                                               `json:"public,omitempty"`
				Scope       *string                                                             `json:"scope,omitempty"`
				Type        *openapigenerated.RestPullRequestFromRefRepositoryOriginProjectType `json:"type,omitempty"`
			} `json:"project,omitempty"`
			Public        *bool                                                         `json:"public,omitempty"`
			RelatedLinks  *map[string]interface{}                                       `json:"relatedLinks,omitempty"`
			ScmId         *string                                                       `json:"scmId,omitempty"`
			Scope         *string                                                       `json:"scope,omitempty"`
			Slug          *string                                                       `json:"slug,omitempty"`
			State         *openapigenerated.RestPullRequestFromRefRepositoryOriginState `json:"state,omitempty"`
			StatusMessage *string                                                       `json:"statusMessage,omitempty"`
		} `json:"origin,omitempty"`
		Partition *int32 `json:"partition,omitempty"`
		Project   *struct {
			Avatar      *string                                                       `json:"avatar,omitempty"`
			AvatarUrl   *string                                                       `json:"avatarUrl,omitempty"`
			Description *string                                                       `json:"description,omitempty"`
			Id          *int32                                                        `json:"id,omitempty"`
			Key         string                                                        `json:"key"`
			Links       *map[string]interface{}                                       `json:"links,omitempty"`
			Name        *string                                                       `json:"name,omitempty"`
			Public      *bool                                                         `json:"public,omitempty"`
			Scope       *string                                                       `json:"scope,omitempty"`
			Type        *openapigenerated.RestPullRequestFromRefRepositoryProjectType `json:"type,omitempty"`
		} `json:"project,omitempty"`
		Public        *bool                                                   `json:"public,omitempty"`
		RelatedLinks  *map[string]interface{}                                 `json:"relatedLinks,omitempty"`
		ScmId         *string                                                 `json:"scmId,omitempty"`
		Scope         *string                                                 `json:"scope,omitempty"`
		Slug          *string                                                 `json:"slug,omitempty"`
		State         *openapigenerated.RestPullRequestFromRefRepositoryState `json:"state,omitempty"`
		StatusMessage *string                                                 `json:"statusMessage,omitempty"`
	} `json:"repository,omitempty"`
	Type *openapigenerated.RestPullRequestFromRefType `json:"type,omitempty"`
}) string {
	if reference == nil {
		return ""
	}

	if reference.DisplayId != nil {
		return strings.TrimSpace(*reference.DisplayId)
	}

	if reference.Id != nil {
		return strings.TrimSpace(*reference.Id)
	}

	return ""
}

func safePullRequestToRefDisplayID(reference *struct {
	DisplayId    *string `json:"displayId,omitempty"`
	Id           *string `json:"id,omitempty"`
	LatestCommit *string `json:"latestCommit,omitempty"`
	Repository   *struct {
		Archived      *bool                   `json:"archived,omitempty"`
		DefaultBranch *string                 `json:"defaultBranch,omitempty"`
		Description   *string                 `json:"description,omitempty"`
		Forkable      *bool                   `json:"forkable,omitempty"`
		HierarchyId   *string                 `json:"hierarchyId,omitempty"`
		Id            *int32                  `json:"id,omitempty"`
		Links         *map[string]interface{} `json:"links,omitempty"`
		Name          *string                 `json:"name,omitempty"`
		Origin        *struct {
			Archived      *bool                   `json:"archived,omitempty"`
			DefaultBranch *string                 `json:"defaultBranch,omitempty"`
			Description   *string                 `json:"description,omitempty"`
			Forkable      *bool                   `json:"forkable,omitempty"`
			HierarchyId   *string                 `json:"hierarchyId,omitempty"`
			Id            *int32                  `json:"id,omitempty"`
			Links         *map[string]interface{} `json:"links,omitempty"`
			Name          *string                 `json:"name,omitempty"`
			Partition     *int32                  `json:"partition,omitempty"`
			Project       *struct {
				Avatar      *string                                                           `json:"avatar,omitempty"`
				AvatarUrl   *string                                                           `json:"avatarUrl,omitempty"`
				Description *string                                                           `json:"description,omitempty"`
				Id          *int32                                                            `json:"id,omitempty"`
				Key         string                                                            `json:"key"`
				Links       *map[string]interface{}                                           `json:"links,omitempty"`
				Name        *string                                                           `json:"name,omitempty"`
				Public      *bool                                                             `json:"public,omitempty"`
				Scope       *string                                                           `json:"scope,omitempty"`
				Type        *openapigenerated.RestPullRequestToRefRepositoryOriginProjectType `json:"type,omitempty"`
			} `json:"project,omitempty"`
			Public        *bool                                                       `json:"public,omitempty"`
			RelatedLinks  *map[string]interface{}                                     `json:"relatedLinks,omitempty"`
			ScmId         *string                                                     `json:"scmId,omitempty"`
			Scope         *string                                                     `json:"scope,omitempty"`
			Slug          *string                                                     `json:"slug,omitempty"`
			State         *openapigenerated.RestPullRequestToRefRepositoryOriginState `json:"state,omitempty"`
			StatusMessage *string                                                     `json:"statusMessage,omitempty"`
		} `json:"origin,omitempty"`
		Partition *int32 `json:"partition,omitempty"`
		Project   *struct {
			Avatar      *string                                                     `json:"avatar,omitempty"`
			AvatarUrl   *string                                                     `json:"avatarUrl,omitempty"`
			Description *string                                                     `json:"description,omitempty"`
			Id          *int32                                                      `json:"id,omitempty"`
			Key         string                                                      `json:"key"`
			Links       *map[string]interface{}                                     `json:"links,omitempty"`
			Name        *string                                                     `json:"name,omitempty"`
			Public      *bool                                                       `json:"public,omitempty"`
			Scope       *string                                                     `json:"scope,omitempty"`
			Type        *openapigenerated.RestPullRequestToRefRepositoryProjectType `json:"type,omitempty"`
		} `json:"project,omitempty"`
		Public        *bool                                                 `json:"public,omitempty"`
		RelatedLinks  *map[string]interface{}                               `json:"relatedLinks,omitempty"`
		ScmId         *string                                               `json:"scmId,omitempty"`
		Scope         *string                                               `json:"scope,omitempty"`
		Slug          *string                                               `json:"slug,omitempty"`
		State         *openapigenerated.RestPullRequestToRefRepositoryState `json:"state,omitempty"`
		StatusMessage *string                                               `json:"statusMessage,omitempty"`
	} `json:"repository,omitempty"`
	Type *openapigenerated.RestPullRequestToRefType `json:"type,omitempty"`
}) string {
	if reference == nil {
		return ""
	}

	if reference.DisplayId != nil {
		return strings.TrimSpace(*reference.DisplayId)
	}

	if reference.Id != nil {
		return strings.TrimSpace(*reference.Id)
	}

	return ""
}

func safeStringFromPullRequestState(state *openapigenerated.RestPullRequestState) string {
	if state == nil {
		return ""
	}

	return string(*state)
}

func mapStatusErrorFromBody(status int, body []byte) error {
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
