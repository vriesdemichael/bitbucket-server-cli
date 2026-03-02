package pullrequest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

func TestListPullRequestsWithPaginationAndFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/pull-requests" {
			http.NotFound(w, request)
			return
		}

		start := request.URL.Query().Get("start")
		if start == "" || start == "0" {
			_, _ = fmt.Fprint(w, `{"values":[{"id":1,"title":"Open PR","state":"OPEN","open":true,"closed":false,"fromRef":{"displayId":"feature/a"},"toRef":{"displayId":"master"},"author":{"user":{"displayName":"A"}}}],"isLastPage":false,"nextPageStart":1}`)
			return
		}

		_, _ = fmt.Fprint(w, `{"values":[{"id":2,"title":"Merged PR","state":"MERGED","open":false,"closed":true,"fromRef":{"displayId":"feature/b"},"toRef":{"displayId":"master"},"author":{"user":{"name":"b-user"}}}],"isLastPage":true,"nextPageStart":2}`)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	service := NewService(httpclient.NewFromConfig(cfg))

	results, err := service.List(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, ListOptions{State: "all", Limit: 1, SourceBranch: "feature/b"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 filtered pull request, got %d", len(results))
	}
	if results[0].ID != 2 || results[0].Author != "b-user" {
		t.Fatalf("unexpected mapped pull request: %#v", results[0])
	}
}

func TestListPullRequestsValidationAndAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"errors":[{"message":"Authentication required"}]}`)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	service := NewService(httpclient.NewFromConfig(cfg))

	_, err = service.List(context.Background(), RepositoryRef{}, ListOptions{})
	if err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected validation error exit code 2, got: %v", err)
	}

	_, err = service.List(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, ListOptions{State: "invalid"})
	if err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected state validation error exit code 2, got: %v", err)
	}

	_, err = service.List(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, ListOptions{State: "open", Start: -1})
	if err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected start validation error exit code 2, got: %v", err)
	}

	_, err = service.List(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, ListOptions{State: "open", Limit: 5})
	if err == nil || apperrors.ExitCode(err) != 3 {
		t.Fatalf("expected auth error exit code 3, got: %v", err)
	}
}

func TestPullRequestHelperBranches(t *testing.T) {
	if normalized, err := normalizeState(""); err != nil || normalized != "open" {
		t.Fatalf("expected empty state to normalize to open, got=%q err=%v", normalized, err)
	}

	closedPR := PullRequest{Open: false, Closed: true, SourceBranch: "feature/a", TargetBranch: "master"}
	if !matchesFilters(closedPR, "closed", "refs/heads/feature/a", "master") {
		t.Fatal("expected closed pull request to match closed-state and normalized branch filters")
	}

	openPR := PullRequest{Open: true, Closed: false, SourceBranch: "feature/a", TargetBranch: "master"}
	if matchesFilters(openPR, "closed", "", "") {
		t.Fatal("expected open pull request to be excluded by closed-state filter")
	}

	if branchDisplayName(nil) != "" {
		t.Fatal("expected empty branch display for nil ref")
	}
	if branchDisplayName(&pullRequestRef{ID: "refs/heads/fallback"}) != "refs/heads/fallback" {
		t.Fatal("expected branch display to fall back to ref id when display id is missing")
	}
}
