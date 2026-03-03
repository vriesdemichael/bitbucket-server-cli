package branch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func newBranchTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return NewService(client)
}

func TestBranchServiceCoreCommands(t *testing.T) {
	service := newBranchTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/branches":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"displayId":"main","id":"refs/heads/main","latestCommit":"abc"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/branch-utils/latest/projects/TEST/repos/demo/branches":
			_, _ = w.Write([]byte(`{"displayId":"feature/demo","id":"refs/heads/feature/demo"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/branch-utils/latest/projects/TEST/repos/demo/branches":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/default-branch":
			_, _ = w.Write([]byte(`{"id":"refs/heads/main","displayId":"main"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/default-branch":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/branch-utils/latest/projects/TEST/repos/demo/branches/info/"):
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":"refs/heads/main","displayId":"main"}]}`))
		default:
			http.NotFound(w, r)
		}
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	branches, err := service.List(context.Background(), repo, ListOptions{Limit: 25, OrderBy: "ALPHABETICAL"})
	if err != nil || len(branches) != 1 {
		t.Fatalf("expected branches list success, len=%d err=%v", len(branches), err)
	}

	created, err := service.Create(context.Background(), repo, "feature/demo", "abc")
	if err != nil || created.DisplayId == nil || *created.DisplayId != "feature/demo" {
		t.Fatalf("expected create success, got %#v err=%v", created, err)
	}

	if err := service.Delete(context.Background(), repo, "feature/demo", "", false); err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}

	defaultRef, err := service.GetDefault(context.Background(), repo)
	if err != nil || defaultRef.DisplayId == nil || *defaultRef.DisplayId != "main" {
		t.Fatalf("expected default get success, got %#v err=%v", defaultRef, err)
	}

	if err := service.SetDefault(context.Background(), repo, "main"); err != nil {
		t.Fatalf("expected default set success, got %v", err)
	}

	refs, err := service.FindByCommit(context.Background(), repo, "abc", 25)
	if err != nil || len(refs) != 1 {
		t.Fatalf("expected model inspect success, len=%d err=%v", len(refs), err)
	}
}

func TestBranchServiceRestrictionsLifecycle(t *testing.T) {
	service := newBranchTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/branch-permissions/latest/projects/TEST/repos/demo/restrictions":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":12,"type":"read-only","matcher":{"id":"refs/heads/main","displayId":"main","type":{"id":"BRANCH"}},"groups":["devs"]}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/branch-permissions/latest/projects/TEST/repos/demo/restrictions/12":
			_, _ = w.Write([]byte(`{"id":12,"type":"read-only","matcher":{"id":"refs/heads/main","displayId":"main","type":{"id":"BRANCH"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/branch-permissions/latest/projects/TEST/repos/demo/restrictions":
			_, _ = w.Write([]byte(`{"id":12,"type":"read-only","matcher":{"id":"refs/heads/main","displayId":"main","type":{"id":"BRANCH"}}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/branch-permissions/latest/projects/TEST/repos/demo/restrictions/12":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	restrictions, err := service.ListRestrictions(context.Background(), repo, RestrictionListOptions{Limit: 25, Type: "read-only", MatcherType: "BRANCH", MatcherID: "refs/heads/main"})
	if err != nil || len(restrictions) != 1 {
		t.Fatalf("expected restriction list success, len=%d err=%v", len(restrictions), err)
	}

	created, err := service.CreateRestriction(context.Background(), repo, RestrictionUpsertInput{
		Type:           "read-only",
		MatcherType:    "BRANCH",
		MatcherID:      "refs/heads/main",
		MatcherDisplay: "main",
		Users:          []string{"alice"},
		Groups:         []string{"devs"},
	})
	if err != nil || created.Id == nil || *created.Id != 12 {
		t.Fatalf("expected restriction create success, got %#v err=%v", created, err)
	}

	updated, err := service.UpdateRestriction(context.Background(), repo, "12", RestrictionUpsertInput{
		Type:        "read-only",
		MatcherType: "BRANCH",
		MatcherID:   "refs/heads/main",
		Groups:      []string{"admins"},
	})
	if err != nil || updated.Id == nil || *updated.Id != 12 {
		t.Fatalf("expected restriction update success, got %#v err=%v", updated, err)
	}

	restriction, err := service.GetRestriction(context.Background(), repo, "12")
	if err != nil || restriction.Id == nil || *restriction.Id != 12 {
		t.Fatalf("expected restriction get success, got %#v err=%v", restriction, err)
	}

	if err := service.DeleteRestriction(context.Background(), repo, "12"); err != nil {
		t.Fatalf("expected restriction delete success, got %v", err)
	}
}

func TestBranchServiceValidationAndHelpers(t *testing.T) {
	service := newBranchTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	if _, err := service.Create(context.Background(), repo, "", "abc"); err == nil {
		t.Fatal("expected branch name validation error")
	}
	if err := service.Delete(context.Background(), repo, "", "", false); err == nil {
		t.Fatal("expected branch delete validation error")
	}
	if err := service.SetDefault(context.Background(), repo, " "); err == nil {
		t.Fatal("expected default branch validation error")
	}
	if _, err := service.FindByCommit(context.Background(), repo, "", 10); err == nil {
		t.Fatal("expected commit validation error")
	}

	if _, err := service.List(context.Background(), repo, ListOptions{}); err == nil || !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("expected mapped authorization error, got %v", err)
	}

	if _, err := service.ListRestrictions(context.Background(), repo, RestrictionListOptions{MatcherType: "bad"}); err == nil {
		t.Fatal("expected matcher type validation error")
	}

	if _, err := service.UpdateRestriction(context.Background(), repo, "abc", RestrictionUpsertInput{Type: "read-only", MatcherID: "refs/heads/main"}); err == nil {
		t.Fatal("expected restriction id parse validation error")
	}

	if normalizeBranchRef("main") != "refs/heads/main" {
		t.Fatal("expected normalizeBranchRef to add refs/heads prefix")
	}

	if err := mapStatusError(http.StatusCreated, nil); err != nil {
		t.Fatalf("expected nil for success status, got %v", err)
	}
	err := mapStatusError(http.StatusConflict, []byte("conflict"))
	if err == nil || apperrors.ExitCode(err) != 5 {
		t.Fatalf("expected conflict exit code 5, got %v (%d)", err, apperrors.ExitCode(err))
	}
}
