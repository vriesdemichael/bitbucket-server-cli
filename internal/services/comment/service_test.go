package comment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func newCommentTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return NewService(client)
}

func TestTargetContext(t *testing.T) {
	ctx := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, CommitID: "abc"}.Context()
	if ctx.Type != "commit" || ctx.CommitID != "abc" {
		t.Fatalf("unexpected commit context: %+v", ctx)
	}

	prCtx := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, PullRequestID: "12"}.Context()
	if prCtx.Type != "pull_request" || prCtx.PullRequestID != "12" {
		t.Fatalf("unexpected pull request context: %+v", prCtx)
	}
}

func TestServiceListCommitAndPullRequest(t *testing.T) {
	service := newCommentTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments":
			if r.URL.Query().Get("path") != "seed.txt" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("missing path"))
				return
			}
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":1,"text":"c1","version":1}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/12/comments":
			if r.URL.Query().Get("path") != "seed.txt" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("missing path"))
				return
			}
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":2,"text":"pr1","version":1}]}`))
		default:
			http.NotFound(w, r)
		}
	})

	commitComments, err := service.List(context.Background(), Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, CommitID: "abc"}, "seed.txt", 25)
	if err != nil || len(commitComments) != 1 {
		t.Fatalf("expected commit comment list, got len=%d err=%v", len(commitComments), err)
	}

	prComments, err := service.List(context.Background(), Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, PullRequestID: "12"}, "seed.txt", 25)
	if err != nil || len(prComments) != 1 {
		t.Fatalf("expected pr comment list, got len=%d err=%v", len(prComments), err)
	}
}

func TestServiceCreateGetUpdateDeleteCommit(t *testing.T) {
	service := newCommentTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":10,"text":"created","version":0}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/10":
			_, _ = w.Write([]byte(`{"id":10,"text":"current","version":2}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/10":
			_, _ = w.Write([]byte(`{"id":10,"text":"updated","version":3}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/10":
			if r.URL.Query().Get("version") != "2" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("missing version"))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	target := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, CommitID: "abc"}

	created, err := service.Create(context.Background(), target, "hello")
	if err != nil || created.Id == nil || *created.Id != 10 {
		t.Fatalf("expected created comment, got %#v err=%v", created, err)
	}

	updated, err := service.Update(context.Background(), target, "10", "changed", nil)
	if err != nil || updated.Version == nil || *updated.Version != 3 {
		t.Fatalf("expected updated comment, got %#v err=%v", updated, err)
	}

	resolvedVersion, err := service.Delete(context.Background(), target, "10", nil)
	if err != nil || resolvedVersion == nil || *resolvedVersion != 2 {
		t.Fatalf("expected delete with resolved version, got %v err=%v", resolvedVersion, err)
	}
}

func TestServiceValidationAndStatusMapping(t *testing.T) {
	service := newCommentTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	})

	_, err := service.List(context.Background(), Target{}, "", 25)
	if err == nil || !strings.Contains(err.Error(), "repository must be specified") {
		t.Fatalf("expected repository validation error, got %v", err)
	}

	_, err = service.List(context.Background(), Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, CommitID: "abc"}, "seed.txt", 25)
	if err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected mapped auth error, got %v", err)
	}
}

func TestServicePullRequestPaginationAndCRUDFallbacks(t *testing.T) {
	service := newCommentTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/12/comments":
			if r.URL.Query().Get("start") == "1" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":2,"text":"page2","version":1}]}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"isLastPage":false,"nextPageStart":1,"values":[{"id":1,"text":"page1","version":1}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/12/comments":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/12/comments/22":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/12/comments/22":
			if r.URL.Query().Get("version") != "7" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("missing version=7"))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/12/comments/22":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	})

	target := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, PullRequestID: "12"}

	comments, err := service.List(context.Background(), target, "seed.txt", 2)
	if err != nil {
		t.Fatalf("expected paginated pr list to succeed, got: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments from pagination, got: %d", len(comments))
	}

	created, err := service.Create(context.Background(), target, "new pr comment")
	if err != nil {
		t.Fatalf("expected pr create to succeed, got: %v", err)
	}
	if created.Text == nil || *created.Text != "new pr comment" {
		t.Fatalf("expected fallback created text payload, got: %#v", created)
	}

	providedVersion := int32(7)
	updated, err := service.Update(context.Background(), target, "22", "updated text", &providedVersion)
	if err != nil {
		t.Fatalf("expected pr update to succeed, got: %v", err)
	}
	if updated.Text == nil || *updated.Text != "updated text" {
		t.Fatalf("expected fallback updated text payload, got: %#v", updated)
	}

	resolvedVersion, err := service.Delete(context.Background(), target, "22", &providedVersion)
	if err != nil {
		t.Fatalf("expected pr delete to succeed, got: %v", err)
	}
	if resolvedVersion == nil || *resolvedVersion != 7 {
		t.Fatalf("expected resolved version 7, got: %v", resolvedVersion)
	}

	got, err := service.Get(context.Background(), target, "22")
	if err != nil {
		t.Fatalf("expected pr get to succeed, got: %v", err)
	}
	if got.Id != nil || got.Text != nil {
		t.Fatalf("expected zero-value comment for empty successful response, got: %#v", got)
	}
}

func TestCommentMapStatusErrorCoverage(t *testing.T) {
	if err := mapStatusError(http.StatusOK, nil); err != nil {
		t.Fatalf("expected nil for success status, got: %v", err)
	}

	tests := []struct {
		status   int
		exitCode int
	}{
		{status: http.StatusBadRequest, exitCode: 2},
		{status: http.StatusUnauthorized, exitCode: 3},
		{status: http.StatusForbidden, exitCode: 3},
		{status: http.StatusNotFound, exitCode: 4},
		{status: http.StatusConflict, exitCode: 5},
		{status: http.StatusTooManyRequests, exitCode: 10},
		{status: http.StatusInternalServerError, exitCode: 10},
		{status: http.StatusNotAcceptable, exitCode: 1},
	}

	for _, testCase := range tests {
		err := mapStatusError(testCase.status, []byte("boom"))
		if err == nil {
			t.Fatalf("expected error for status %d", testCase.status)
		}
		if apperrors.ExitCode(err) != testCase.exitCode {
			t.Fatalf("expected exit code %d for status %d, got %d", testCase.exitCode, testCase.status, apperrors.ExitCode(err))
		}
	}
}

func TestCommentServiceAdditionalBranches(t *testing.T) {
	t.Run("validation branches", func(t *testing.T) {
		service := newCommentTestService(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		target := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, CommitID: "abc"}

		if _, err := service.List(context.Background(), target, "", 10); err == nil {
			t.Fatal("expected path validation error")
		}
		if _, err := service.Create(context.Background(), target, " "); err == nil {
			t.Fatal("expected comment text validation error")
		}
		if _, err := service.Update(context.Background(), target, "", "text", nil); err == nil {
			t.Fatal("expected comment id validation error")
		}
		if _, err := service.Update(context.Background(), target, "10", "", nil); err == nil {
			t.Fatal("expected update text validation error")
		}
		if _, err := service.Delete(context.Background(), target, "", nil); err == nil {
			t.Fatal("expected delete comment id validation error")
		}
		if _, err := service.Get(context.Background(), target, ""); err == nil {
			t.Fatal("expected get comment id validation error")
		}
	})

	t.Run("commit pagination and fallback payloads", func(t *testing.T) {
		service := newCommentTestService(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments":
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("start") == "1" {
					_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":2,"text":"c2","version":1}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"isLastPage":false,"nextPageStart":1,"values":[{"id":1,"text":"c1","version":1}]}`))
			case r.Method == http.MethodPost && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments":
				w.WriteHeader(http.StatusCreated)
			case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/10":
				w.WriteHeader(http.StatusOK)
			case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/10":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":10,"version":5}`))
			case r.Method == http.MethodDelete && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/10":
				w.WriteHeader(http.StatusNoContent)
			default:
				http.NotFound(w, r)
			}
		})

		target := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, CommitID: "abc"}

		comments, err := service.List(context.Background(), target, "seed.txt", 2)
		if err != nil || len(comments) != 2 {
			t.Fatalf("expected paginated commit comments, got len=%d err=%v", len(comments), err)
		}

		created, err := service.Create(context.Background(), target, "new")
		if err != nil {
			t.Fatalf("expected create fallback success, got %v", err)
		}
		if created.Text == nil || *created.Text != "new" {
			t.Fatalf("expected fallback create payload, got %#v", created)
		}

		updated, err := service.Update(context.Background(), target, "10", "updated", nil)
		if err != nil {
			t.Fatalf("expected update fallback success, got %v", err)
		}
		if updated.Text == nil || *updated.Text != "updated" {
			t.Fatalf("expected fallback update payload, got %#v", updated)
		}

		resolved, err := service.Delete(context.Background(), target, "10", nil)
		if err != nil {
			t.Fatalf("expected delete success, got %v", err)
		}
		if resolved == nil || *resolved != 5 {
			t.Fatalf("expected resolved delete version 5, got %v", resolved)
		}
	})

	t.Run("transport and status error branches", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		baseURL := server.URL
		server.Close()

		client, err := openapigenerated.NewClientWithResponses(baseURL + "/rest")
		if err != nil {
			t.Fatalf("create client: %v", err)
		}
		service := NewService(client)

		commitTarget := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, CommitID: "abc"}
		prTarget := Target{Repository: RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, PullRequestID: "12"}

		if _, err := service.Create(context.Background(), commitTarget, "x"); err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient commit create transport error, got %v", err)
		}
		if _, err := service.Create(context.Background(), prTarget, "x"); err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient pr create transport error, got %v", err)
		}

		statusService := newCommentTestService(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte("conflict"))
		})
		if _, err := statusService.Get(context.Background(), prTarget, "1"); err == nil || apperrors.ExitCode(err) != 5 {
			t.Fatalf("expected conflict mapping for get, got %v", err)
		}
	})
}
