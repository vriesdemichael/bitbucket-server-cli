package diff

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type OutputKind string

const (
	OutputKindRaw      OutputKind = "raw"
	OutputKindPatch    OutputKind = "patch"
	OutputKindStat     OutputKind = "stat"
	OutputKindNameOnly OutputKind = "name_only"
)

type RepositoryRef struct {
	ProjectKey string
	Slug       string
}

type DiffRefsInput struct {
	Repository RepositoryRef
	From       string
	To         string
	Path       string
	Output     OutputKind
}

type DiffPRInput struct {
	Repository    RepositoryRef
	PullRequestID string
	Output        OutputKind
}

type DiffCommitInput struct {
	Repository RepositoryRef
	CommitID   string
	Path       string
}

type Result struct {
	Patch string   `json:"patch,omitempty"`
	Stats any      `json:"stats,omitempty"`
	Names []string `json:"names,omitempty"`
}

type Service struct {
	client *openapigenerated.ClientWithResponses
}

func NewService(client *openapigenerated.ClientWithResponses) *Service {
	return &Service{client: client}
}

func (service *Service) DiffRefs(ctx context.Context, input DiffRefsInput) (Result, error) {
	if err := validateRepoRef(input.Repository); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(input.From) == "" || strings.TrimSpace(input.To) == "" {
		return Result{}, apperrors.New(apperrors.KindValidation, "from and to refs are required", nil)
	}
	if input.Output == "" {
		input.Output = OutputKindRaw
	}

	from := strings.TrimSpace(input.From)
	to := strings.TrimSpace(input.To)

	switch input.Output {
	case OutputKindPatch:
		if strings.TrimSpace(input.Path) != "" {
			return Result{}, apperrors.New(apperrors.KindValidation, "--path is not supported with patch output for ref diffs", nil)
		}
		response, err := service.client.StreamPatchWithResponse(ctx, input.Repository.ProjectKey, input.Repository.Slug, &openapigenerated.StreamPatchParams{
			Since: &from,
			Until: &to,
		})
		if err != nil {
			return Result{}, apperrors.New(apperrors.KindTransient, "failed to stream patch", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return Result{}, err
		}
		return Result{Patch: string(response.Body)}, nil
	case OutputKindStat:
		response, err := service.client.GetDiffStatsSummary1WithResponse(
			ctx,
			input.Repository.ProjectKey,
			input.Repository.Slug,
			pathOrDot(input.Path),
			&openapigenerated.GetDiffStatsSummary1Params{From: &from, To: &to},
		)
		if err != nil {
			return Result{}, apperrors.New(apperrors.KindTransient, "failed to get diff stats", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return Result{}, err
		}
		return Result{Stats: response.ApplicationjsonCharsetUTF8200}, nil
	case OutputKindRaw, OutputKindNameOnly:
		body, err := service.streamRefRawDiff(ctx, input.Repository, input.Path, from, to)
		if err != nil {
			return Result{}, err
		}
		if input.Output == OutputKindNameOnly {
			return Result{Names: extractNamesFromUnifiedDiff(body)}, nil
		}
		return Result{Patch: body}, nil
	default:
		return Result{}, apperrors.New(apperrors.KindValidation, "unsupported diff output mode", nil)
	}
}

func (service *Service) DiffPR(ctx context.Context, input DiffPRInput) (Result, error) {
	if err := validateRepoRef(input.Repository); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(input.PullRequestID) == "" {
		return Result{}, apperrors.New(apperrors.KindValidation, "pull request id is required", nil)
	}
	if input.Output == "" {
		input.Output = OutputKindRaw
	}

	prID := strings.TrimSpace(input.PullRequestID)

	switch input.Output {
	case OutputKindPatch:
		response, err := service.client.StreamPatch1WithResponse(ctx, input.Repository.ProjectKey, input.Repository.Slug, prID)
		if err != nil {
			return Result{}, apperrors.New(apperrors.KindTransient, "failed to stream pull request patch", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return Result{}, err
		}
		return Result{Patch: string(response.Body)}, nil
	case OutputKindStat:
		response, err := service.client.GetDiffStatsSummary2WithResponse(ctx, input.Repository.ProjectKey, input.Repository.Slug, prID, ".", nil)
		if err != nil {
			return Result{}, apperrors.New(apperrors.KindTransient, "failed to get pull request diff stats", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return Result{}, err
		}
		return Result{Stats: response.ApplicationjsonCharsetUTF8200}, nil
	case OutputKindRaw, OutputKindNameOnly:
		response, err := service.client.StreamRawDiff2WithResponse(ctx, input.Repository.ProjectKey, input.Repository.Slug, prID, nil)
		if err != nil {
			return Result{}, apperrors.New(apperrors.KindTransient, "failed to stream pull request diff", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return Result{}, err
		}
		diffText := string(response.Body)
		if input.Output == OutputKindNameOnly {
			return Result{Names: extractNamesFromUnifiedDiff(diffText)}, nil
		}
		return Result{Patch: diffText}, nil
	default:
		return Result{}, apperrors.New(apperrors.KindValidation, "unsupported diff output mode", nil)
	}
}

func (service *Service) DiffCommit(ctx context.Context, input DiffCommitInput) (Result, error) {
	if err := validateRepoRef(input.Repository); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(input.CommitID) == "" {
		return Result{}, apperrors.New(apperrors.KindValidation, "commit id is required", nil)
	}

	commitID := strings.TrimSpace(input.CommitID)
	body, err := service.streamRefRawDiff(ctx, input.Repository, input.Path, "", commitID)
	if err != nil {
		return Result{}, err
	}

	return Result{Patch: body}, nil
}

func (service *Service) streamRefRawDiff(ctx context.Context, repo RepositoryRef, path, from, to string) (string, error) {
	params := &openapigenerated.StreamRawDiffParams{Until: &to}
	if from != "" {
		params.Since = &from
	}

	if strings.TrimSpace(path) == "" {
		response, err := service.client.StreamPatchWithResponse(ctx, repo.ProjectKey, repo.Slug, &openapigenerated.StreamPatchParams{Since: params.Since, Until: params.Until})
		if err != nil {
			return "", apperrors.New(apperrors.KindTransient, "failed to stream raw diff", err)
		}
		if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
			return "", err
		}
		return string(response.Body), nil
	}

	response, err := service.client.StreamRawDiff1WithResponse(ctx, repo.ProjectKey, repo.Slug, strings.TrimSpace(path), &openapigenerated.StreamRawDiff1Params{Since: params.Since, Until: params.Until})
	if err != nil {
		return "", apperrors.New(apperrors.KindTransient, "failed to stream raw diff for file path", err)
	}
	if err := mapStatusError(response.StatusCode(), response.Body); err != nil {
		return "", err
	}

	return string(response.Body), nil
}

func validateRepoRef(repo RepositoryRef) error {
	if strings.TrimSpace(repo.ProjectKey) == "" || strings.TrimSpace(repo.Slug) == "" {
		return apperrors.New(apperrors.KindValidation, "repository must be specified as project/repo", nil)
	}

	return nil
}

func pathOrDot(value string) string {
	if strings.TrimSpace(value) == "" {
		return "."
	}

	return strings.TrimSpace(value)
}

func extractNamesFromUnifiedDiff(diffText string) []string {
	lines := strings.Split(diffText, "\n")
	seen := map[string]struct{}{}
	names := make([]string, 0)

	for _, line := range lines {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		candidate := strings.TrimPrefix(parts[3], "b/")
		if candidate == "/dev/null" || candidate == "" {
			candidate = strings.TrimPrefix(parts[2], "a/")
		}
		if candidate == "" || candidate == "/dev/null" {
			continue
		}

		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		names = append(names, candidate)
	}

	return names
}

func mapStatusError(status int, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}

	message := strings.TrimSpace(string(bytes.TrimSpace(body)))
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
