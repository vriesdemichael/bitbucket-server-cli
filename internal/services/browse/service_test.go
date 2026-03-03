package browse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func newBrowseTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return NewService(client)
}

func TestBrowseServiceCoreCommands(t *testing.T) {
	service := newBrowseTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/files":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":["file1.txt", "dir/file2.txt"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/files/dir":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":["file2.txt"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/raw/file1.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(`raw content`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/browse/file1.txt":
			_, _ = w.Write([]byte(`{"lines":[{"text":"hello"}]}`))
		default:
			http.NotFound(w, r)
		}
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	files, err := service.Tree(context.Background(), repo, "", TreeOptions{Limit: 25})
	if err != nil || len(files) != 2 {
		t.Fatalf("expected tree success, len=%d err=%v", len(files), err)
	}

	dirFiles, err := service.Tree(context.Background(), repo, "dir", TreeOptions{Limit: 25})
	if err != nil || len(dirFiles) != 1 {
		t.Fatalf("expected dir tree success, len=%d err=%v", len(dirFiles), err)
	}

	raw, err := service.Raw(context.Background(), repo, "file1.txt", "")
	if err != nil || string(raw) != "raw content" {
		t.Fatalf("expected raw success, got %s err=%v", string(raw), err)
	}

	file, err := service.File(context.Background(), repo, "file1.txt", FileOptions{Blame: true})
	if err != nil || !strings.Contains(string(file), "hello") {
		t.Fatalf("expected file success, got %s err=%v", string(file), err)
	}
}

func TestBrowseServiceValidation(t *testing.T) {
	service := newBrowseTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	if _, err := service.Raw(context.Background(), repo, "", ""); err == nil {
		t.Fatal("expected raw path validation error")
	}

	if _, err := service.File(context.Background(), repo, "", FileOptions{}); err == nil {
		t.Fatal("expected file path validation error")
	}

	if _, err := service.Tree(context.Background(), repo, "", TreeOptions{}); err == nil || !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("expected mapped authorization error, got %v", err)
	}
}

func TestBrowseServicePagination(t *testing.T) {
	calls := 0
	service := newBrowseTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"isLastPage":false,"nextPageStart":1,"values":["file1.txt"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"isLastPage":true,"values":["file2.txt"]}`))
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	files, err := service.Tree(context.Background(), repo, "", TreeOptions{Limit: 0})
	if err != nil || len(files) != 2 {
		t.Fatalf("expected paginated list, len=%d err=%v", len(files), err)
	}
}

func TestBrowseServiceTransientAndMapping(t *testing.T) {
	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	transientService := newBrowseTestService(t, func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		connection, _, hijackErr := hijacker.Hijack()
		if hijackErr == nil {
			_ = connection.Close()
		}
	})

	if _, err := transientService.Tree(context.Background(), repo, "", TreeOptions{}); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected tree transient error, got %v", err)
	}
	if _, err := transientService.Tree(context.Background(), repo, "dir", TreeOptions{}); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected tree1 transient error, got %v", err)
	}
	if _, err := transientService.Raw(context.Background(), repo, "file.txt", "abc"); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected raw transient error, got %v", err)
	}
	if _, err := transientService.File(context.Background(), repo, "file.txt", FileOptions{At: "abc"}); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected file transient error, got %v", err)
	}

	service := newBrowseTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/files":
			_, _ = w.Write([]byte(`{"isLastPage":true}`))
		case strings.Contains(r.URL.Path, "raw"):
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		default:
			http.NotFound(w, r)
		}
	})

	files, err := service.Tree(context.Background(), repo, "", TreeOptions{})
	if err != nil || len(files) != 0 {
		t.Fatalf("expected empty tree success, got %v", err)
	}

	if _, err := service.Raw(context.Background(), repo, "file.txt", ""); err == nil || apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found get error, got %v", err)
	}

	if err := validateRepositoryRef(RepositoryRef{}); err == nil {
		t.Fatalf("expected validate error")
	}

	testMapStatusErrors(t)
}

func testMapStatusErrors(t *testing.T) {
	if err := mapStatusError(http.StatusBadRequest, nil); err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected validation error")
	}
	if err := mapStatusError(http.StatusUnauthorized, nil); err == nil || apperrors.ExitCode(err) != 3 {
		t.Fatalf("expected auth error")
	}
	if err := mapStatusError(http.StatusNotFound, nil); err == nil || apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found error")
	}
	if err := mapStatusError(http.StatusConflict, nil); err == nil || apperrors.ExitCode(err) != 5 {
		t.Fatalf("expected conflict error")
	}
	if err := mapStatusError(http.StatusTooManyRequests, []byte("rate")); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected transient rate error")
	}
	if err := mapStatusError(http.StatusTeapot, nil); err == nil || apperrors.ExitCode(err) != 1 {
		t.Fatalf("expected permanent error")
	}
}
