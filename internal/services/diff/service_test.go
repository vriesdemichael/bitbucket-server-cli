package diff

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func TestDiffRefsRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/latest/projects/PRJ/repos/demo/patch" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("since") != "refs/heads/main" || request.URL.Query().Get("until") != "refs/heads/feature" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("missing refs"))
			return
		}
		_, _ = writer.Write([]byte("diff --git a/seed.txt b/seed.txt\n"))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	result, err := service.DiffRefs(context.Background(), DiffRefsInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		From:       "refs/heads/main",
		To:         "refs/heads/feature",
		Output:     OutputKindRaw,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(result.Patch, "seed.txt") {
		t.Fatalf("expected diff body, got: %q", result.Patch)
	}
}

func TestDiffPRNameOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/latest/projects/PRJ/repos/demo/pull-requests/12.diff" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte("diff --git a/a.txt b/a.txt\ndiff --git a/dir/b.go b/dir/b.go\n"))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	result, err := service.DiffPR(context.Background(), DiffPRInput{
		Repository:    RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		PullRequestID: "12",
		Output:        OutputKindNameOnly,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Names) != 2 || result.Names[0] != "a.txt" || result.Names[1] != "dir/b.go" {
		t.Fatalf("unexpected names: %#v", result.Names)
	}
}

func TestDiffRefsPatchWithPathRejected(t *testing.T) {
	service := NewService(nil)
	_, err := service.DiffRefs(context.Background(), DiffRefsInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		From:       "main",
		To:         "feature",
		Path:       "seed.txt",
		Output:     OutputKindPatch,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected exit code 2, got %d (%v)", apperrors.ExitCode(err), err)
	}
}

func TestDiffRefsNotFoundMapsToNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("missing"))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	_, err = service.DiffRefs(context.Background(), DiffRefsInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		From:       "main",
		To:         "feature",
		Output:     OutputKindRaw,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found exit code 4, got %d (%v)", apperrors.ExitCode(err), err)
	}
}
