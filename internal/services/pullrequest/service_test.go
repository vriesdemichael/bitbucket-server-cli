package pullrequest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestPullRequestLifecycleReviewAndTaskOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/22":
			_, _ = fmt.Fprint(w, `{"id":22,"title":"Feature","state":"OPEN","open":true,"closed":false,"version":2,"participants":[{"role":"REVIEWER","status":"UNAPPROVED","approved":false,"user":{"name":"reviewer1","displayName":"Reviewer One"}}],"fromRef":{"displayId":"feature/a"},"toRef":{"displayId":"master"}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests":
			if !strings.Contains(readBody(t, request), "refs/heads/feature/new") {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprint(w, `{"errors":[{"message":"missing fromRef"}]}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"id":30,"title":"New PR","state":"OPEN","open":true,"closed":false,"fromRef":{"displayId":"feature/new"},"toRef":{"displayId":"master"}}`)
		case request.Method == http.MethodPut && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30":
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"OPEN","open":true,"closed":false,"version":3,"fromRef":{"displayId":"feature/new"},"toRef":{"displayId":"master"}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/merge":
			if request.URL.Query().Get("version") != "3" {
				w.WriteHeader(http.StatusConflict)
				_, _ = fmt.Fprint(w, `{"errors":[{"message":"version required"}]}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"MERGED","open":false,"closed":true}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/decline":
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"DECLINED","open":false,"closed":true}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/reopen":
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"OPEN","open":true,"closed":false}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/approve":
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"OPEN","open":true,"closed":false,"participants":[{"role":"REVIEWER","status":"APPROVED","approved":true,"user":{"name":"reviewer1"}}]}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/approve":
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"OPEN","open":true,"closed":false,"participants":[{"role":"REVIEWER","status":"UNAPPROVED","approved":false,"user":{"name":"reviewer1"}}]}`)
		case request.Method == http.MethodPut && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/participants/reviewer2":
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"OPEN","open":true,"closed":false,"participants":[{"role":"REVIEWER","status":"UNAPPROVED","approved":false,"user":{"name":"reviewer2"}}]}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/participants/reviewer2":
			_, _ = fmt.Fprint(w, `{"id":30,"title":"Updated PR","state":"OPEN","open":true,"closed":false,"participants":[]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/tasks":
			_, _ = fmt.Fprint(w, `{"isLastPage":true,"nextPageStart":0,"values":[{"id":500,"text":"Open task","state":"OPEN","resolved":false},{"id":501,"text":"Resolved task","state":"RESOLVED","resolved":true}]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/tasks":
			_, _ = fmt.Fprint(w, `{"id":502,"text":"New task","state":"OPEN","resolved":false}`)
		case request.Method == http.MethodPut && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/tasks/501":
			_, _ = fmt.Fprint(w, `{"id":501,"text":"Resolved task updated","state":"RESOLVED","resolved":true}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/30/tasks/501":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	service := NewService(httpclient.NewFromConfig(cfg))
	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	fetched, err := service.Get(context.Background(), repo, "22")
	if err != nil {
		t.Fatalf("expected get to succeed, got: %v", err)
	}
	if len(fetched.Reviewers) != 1 || fetched.Reviewers[0].Name != "reviewer1" {
		t.Fatalf("expected reviewer mapping in get response, got: %#v", fetched.Reviewers)
	}

	created, err := service.Create(context.Background(), repo, CreateInput{FromRef: "feature/new", ToRef: "master", Title: "New PR"})
	if err != nil || created.ID != 30 {
		t.Fatalf("expected create to succeed with id 30, got id=%d err=%v", created.ID, err)
	}

	updated, err := service.Update(context.Background(), repo, "30", UpdateInput{Title: "Updated PR", Version: 2})
	if err != nil || updated.Version != 3 {
		t.Fatalf("expected update to succeed with version 3, got version=%d err=%v", updated.Version, err)
	}

	merged, err := service.Merge(context.Background(), repo, "30", intPtr(3))
	if err != nil || merged.State != "MERGED" {
		t.Fatalf("expected merge to succeed, got state=%q err=%v", merged.State, err)
	}

	declined, err := service.Decline(context.Background(), repo, "30", nil)
	if err != nil || declined.State != "DECLINED" {
		t.Fatalf("expected decline to succeed, got state=%q err=%v", declined.State, err)
	}

	reopened, err := service.Reopen(context.Background(), repo, "30", nil)
	if err != nil || reopened.State != "OPEN" {
		t.Fatalf("expected reopen to succeed, got state=%q err=%v", reopened.State, err)
	}

	approved, err := service.Approve(context.Background(), repo, "30")
	if err != nil || len(approved.Reviewers) != 1 || !approved.Reviewers[0].Approved {
		t.Fatalf("expected approve to set reviewer approval, got reviewers=%#v err=%v", approved.Reviewers, err)
	}

	unapproved, err := service.Unapprove(context.Background(), repo, "30")
	if err != nil || len(unapproved.Reviewers) != 1 || unapproved.Reviewers[0].Approved {
		t.Fatalf("expected unapprove to clear reviewer approval, got reviewers=%#v err=%v", unapproved.Reviewers, err)
	}

	withReviewer, err := service.AddReviewer(context.Background(), repo, "30", "reviewer2")
	if err != nil || len(withReviewer.Reviewers) != 1 || withReviewer.Reviewers[0].Name != "reviewer2" {
		t.Fatalf("expected add reviewer to succeed, got reviewers=%#v err=%v", withReviewer.Reviewers, err)
	}

	withoutReviewer, err := service.RemoveReviewer(context.Background(), repo, "30", "reviewer2")
	if err != nil || len(withoutReviewer.Reviewers) != 0 {
		t.Fatalf("expected remove reviewer to succeed, got reviewers=%#v err=%v", withoutReviewer.Reviewers, err)
	}

	openTasks, err := service.ListTasks(context.Background(), repo, "30", TaskListOptions{State: "open", Limit: 20})
	if err != nil || len(openTasks) != 1 || openTasks[0].Resolved {
		t.Fatalf("expected open task filter to return one unresolved task, got tasks=%#v err=%v", openTasks, err)
	}

	createdTask, err := service.CreateTask(context.Background(), repo, "30", "New task")
	if err != nil || createdTask.ID != 502 {
		t.Fatalf("expected create task to succeed, got task=%#v err=%v", createdTask, err)
	}

	updatedTask, err := service.UpdateTask(context.Background(), repo, "30", "501", "Resolved task updated", boolPtr(true), nil)
	if err != nil || !updatedTask.Resolved {
		t.Fatalf("expected update task to succeed, got task=%#v err=%v", updatedTask, err)
	}

	if err := service.DeleteTask(context.Background(), repo, "30", "501", intPtr(3)); err != nil {
		t.Fatalf("expected delete task to succeed, got: %v", err)
	}
}

func TestPullRequestTaskAndUpdateValidation(t *testing.T) {
	service := NewService(httpclient.NewFromConfig(config.AppConfig{BitbucketURL: "http://localhost:7990"}))
	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	_, err := service.Update(context.Background(), repo, "30", UpdateInput{Version: 0})
	if err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected update validation error, got: %v", err)
	}

	_, err = service.UpdateTask(context.Background(), repo, "30", "501", "", nil, nil)
	if err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected update task validation error, got: %v", err)
	}

	_, err = service.ListTasks(context.Background(), repo, "30", TaskListOptions{State: "bad"})
	if err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected task state validation error, got: %v", err)
	}
}

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func readBody(t *testing.T, request *http.Request) string {
	t.Helper()
	bodyBytes, _ := io.ReadAll(request.Body)
	_ = request.Body.Close()
	return string(bodyBytes)
}
