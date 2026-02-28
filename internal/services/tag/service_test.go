package tag

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func newTagTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return NewService(client)
}

func TestTagServiceListCreateGetDelete(t *testing.T) {
	service := newTagTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/tags":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"displayId":"v1.0.0"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/tags":
			_, _ = w.Write([]byte(`{"displayId":"v1.0.1","latestCommit":"abc"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/tags/v1.0.1":
			_, _ = w.Write([]byte(`{"displayId":"v1.0.1"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/git/latest/projects/TEST/repos/demo/tags/v1.0.1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	tags, err := service.List(context.Background(), repo, ListOptions{Limit: 25, OrderBy: "ALPHABETICAL"})
	if err != nil || len(tags) != 1 {
		t.Fatalf("expected tags list, got len=%d err=%v", len(tags), err)
	}

	created, err := service.Create(context.Background(), repo, "v1.0.1", "abc", "release")
	if err != nil || created.DisplayId == nil || *created.DisplayId != "v1.0.1" {
		t.Fatalf("expected created tag, got %#v err=%v", created, err)
	}

	viewed, err := service.Get(context.Background(), repo, "v1.0.1")
	if err != nil || viewed.DisplayId == nil || *viewed.DisplayId != "v1.0.1" {
		t.Fatalf("expected viewed tag, got %#v err=%v", viewed, err)
	}

	if err := service.Delete(context.Background(), repo, "v1.0.1"); err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}
}

func TestTagServiceValidationAndStatusMapping(t *testing.T) {
	service := newTagTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	_, err := service.Create(context.Background(), repo, "", "abc", "")
	if err == nil || !strings.Contains(err.Error(), "tag name is required") {
		t.Fatalf("expected tag name validation error, got %v", err)
	}

	_, err = service.List(context.Background(), repo, ListOptions{Limit: 25})
	if err == nil || !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("expected mapped authorization error, got %v", err)
	}
}
