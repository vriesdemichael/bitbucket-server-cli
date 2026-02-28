package comment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
