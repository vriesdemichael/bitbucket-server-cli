package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	branchservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/branch"
	commentservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/comment"
	diffservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/diff"
	pullrequestservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/pullrequest"
	qualityservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/quality"
	reposettings "github.com/vriesdemichael/bitbucket-server-cli/internal/services/reposettings"
	tagservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/tag"
)

func resolveRepositoryReference(selector string, cfg config.AppConfig) (diffservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return diffservice.RepositoryRef{}, err
	}

	return diffservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

type repositorySelector struct {
	ProjectKey string
	Slug       string
}

func resolveRepositorySelector(selector string, cfg config.AppConfig) (repositorySelector, error) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		repoSlug := strings.TrimSpace(os.Getenv("BITBUCKET_REPO_SLUG"))
		if strings.TrimSpace(cfg.ProjectKey) == "" || repoSlug == "" {
			return repositorySelector{}, apperrors.New(
				apperrors.KindValidation,
				"repository is required (use --repo PROJECT/slug or set BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)",
				nil,
			)
		}

		return repositorySelector{ProjectKey: cfg.ProjectKey, Slug: repoSlug}, nil
	}

	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return repositorySelector{}, apperrors.New(apperrors.KindValidation, "--repo must be in PROJECT/slug format", nil)
	}

	return repositorySelector{ProjectKey: strings.TrimSpace(parts[0]), Slug: strings.TrimSpace(parts[1])}, nil
}

func resolveRepositorySettingsReference(selector string, cfg config.AppConfig) (reposettings.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return reposettings.RepositoryRef{}, err
	}

	return reposettings.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolvePullRequestRepositoryReference(selector string, cfg config.AppConfig) (pullrequestservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return pullrequestservice.RepositoryRef{}, err
	}

	return pullrequestservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolveTagRepositoryReference(selector string, cfg config.AppConfig) (tagservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return tagservice.RepositoryRef{}, err
	}

	return tagservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolveBranchRepositoryReference(selector string, cfg config.AppConfig) (branchservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return branchservice.RepositoryRef{}, err
	}

	return branchservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolveQualityRepositoryReference(selector string, cfg config.AppConfig) (qualityservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return qualityservice.RepositoryRef{}, err
	}

	return qualityservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolveCommentTarget(selector string, commitID string, pullRequestID string, cfg config.AppConfig) (commentservice.Target, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return commentservice.Target{}, err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedPullRequestID := strings.TrimSpace(pullRequestID)
	hasCommit := trimmedCommitID != ""
	hasPullRequest := trimmedPullRequestID != ""

	if hasCommit == hasPullRequest {
		return commentservice.Target{}, apperrors.New(apperrors.KindValidation, "exactly one of --commit or --pr is required", nil)
	}

	return commentservice.Target{
		Repository:    commentservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug},
		CommitID:      trimmedCommitID,
		PullRequestID: trimmedPullRequestID,
	}, nil
}

func resolveDiffOutputMode(patch, stat, nameOnly bool) (diffservice.OutputKind, error) {
	selected := 0
	if patch {
		selected++
	}
	if stat {
		selected++
	}
	if nameOnly {
		selected++
	}
	if selected > 1 {
		return "", apperrors.New(apperrors.KindValidation, "choose only one output mode: --patch, --stat, or --name-only", nil)
	}

	if patch {
		return diffservice.OutputKindPatch, nil
	}
	if stat {
		return diffservice.OutputKindStat, nil
	}
	if nameOnly {
		return diffservice.OutputKindNameOnly, nil
	}

	return diffservice.OutputKindRaw, nil
}

func writeDiffResult(writer io.Writer, asJSON bool, mode diffservice.OutputKind, result diffservice.Result) error {
	if asJSON {
		switch mode {
		case diffservice.OutputKindNameOnly:
			return writeJSON(writer, map[string]any{"names": result.Names})
		case diffservice.OutputKindStat:
			return writeJSON(writer, map[string]any{"stats": result.Stats})
		default:
			return writeJSON(writer, map[string]any{"patch": result.Patch})
		}
	}

	switch mode {
	case diffservice.OutputKindNameOnly:
		for _, name := range result.Names {
			fmt.Fprintln(writer, name)
		}
		return nil
	case diffservice.OutputKindStat:
		return writeJSON(writer, result.Stats)
	default:
		fmt.Fprint(writer, result.Patch)
		if result.Patch != "" && !strings.HasSuffix(result.Patch, "\n") {
			fmt.Fprintln(writer)
		}
		return nil
	}
}

func commentIDString(comment openapigenerated.RestComment) string {
	if comment.Id == nil {
		return "unknown"
	}

	return strconv.FormatInt(*comment.Id, 10)
}

func formatCommentSummary(comment openapigenerated.RestComment) string {
	text := ""
	if comment.Text != nil {
		text = strings.TrimSpace(*comment.Text)
	}
	if text == "" {
		text = "<empty>"
	}

	version := "?"
	if comment.Version != nil {
		version = strconv.Itoa(int(*comment.Version))
	}

	return fmt.Sprintf("[%s v%s] %s", commentIDString(comment), version, text)
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func safeInt32(value *int32) int32 {
	if value == nil {
		return 0
	}

	return *value
}

func safeInt64(value *int64) int64 {
	if value == nil {
		return 0
	}

	return *value
}

func safeStringSlice(values *[]string) []string {
	if values == nil {
		return []string{}
	}

	return *values
}

func safeUsers(values *[]openapigenerated.RestApplicationUser) []openapigenerated.RestApplicationUser {
	if values == nil {
		return []openapigenerated.RestApplicationUser{}
	}

	return *values
}

func normalizeAccessKeyIDs(values []int) ([]int32, error) {
	const maxInt32Value = int(^uint32(0) >> 1)

	normalized := make([]int32, 0, len(values))
	for _, value := range values {
		if value < 0 || value > maxInt32Value {
			return nil, apperrors.New(apperrors.KindValidation, "access-key-id must be between 0 and 2147483647", nil)
		}
		normalized = append(normalized, int32(value))
	}

	return normalized, nil
}

func safeStringFromTagType(tagType *openapigenerated.RestTagType) string {
	if tagType == nil {
		return ""
	}

	return string(*tagType)
}

func safeStringFromBuildState(state *openapigenerated.RestBuildStatusState) string {
	if state == nil {
		return ""
	}

	return string(*state)
}

func safeStringFromInsightResult(result *openapigenerated.RestInsightReportResult) string {
	if result == nil {
		return ""
	}

	return string(*result)
}
