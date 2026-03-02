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

func TestDiffRefsDefaultAndNameOnlyModes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/latest/projects/PRJ/repos/demo/patch" {
			http.NotFound(writer, request)
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

	defaultResult, err := service.DiffRefs(context.Background(), DiffRefsInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		From:       "main",
		To:         "feature",
	})
	if err != nil {
		t.Fatalf("expected no error for default output, got: %v", err)
	}
	if !strings.Contains(defaultResult.Patch, "seed.txt") {
		t.Fatalf("expected default patch output, got: %q", defaultResult.Patch)
	}

	nameOnlyResult, err := service.DiffRefs(context.Background(), DiffRefsInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		From:       "main",
		To:         "feature",
		Output:     OutputKindNameOnly,
	})
	if err != nil {
		t.Fatalf("expected no error for name-only output, got: %v", err)
	}
	if len(nameOnlyResult.Names) != 1 || nameOnlyResult.Names[0] != "seed.txt" {
		t.Fatalf("expected parsed changed names, got: %#v", nameOnlyResult.Names)
	}
}

func TestDiffRefsPatchAndStatErrorBranches(t *testing.T) {
	t.Run("patch success", func(t *testing.T) {
		service := newDiffServiceWithHandler(t, func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("diff --git a/x.txt b/x.txt\n"))
		})

		result, err := service.DiffRefs(context.Background(), DiffRefsInput{
			Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			From:       "main",
			To:         "feature",
			Output:     OutputKindPatch,
		})
		if err != nil {
			t.Fatalf("expected patch success, got: %v", err)
		}
		if !strings.Contains(result.Patch, "diff --git") {
			t.Fatalf("expected patch payload, got: %q", result.Patch)
		}
	})

	t.Run("patch status error", func(t *testing.T) {
		service := newDiffServiceWithHandler(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte("missing"))
		})

		_, err := service.DiffRefs(context.Background(), DiffRefsInput{
			Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			From:       "main",
			To:         "feature",
			Output:     OutputKindPatch,
		})
		if err == nil {
			t.Fatal("expected not found error")
		}
		if apperrors.ExitCode(err) != 4 {
			t.Fatalf("expected not found exit code 4, got %d (%v)", apperrors.ExitCode(err), err)
		}
	})

	t.Run("stat status error", func(t *testing.T) {
		service := newDiffServiceWithHandler(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusConflict)
			_, _ = writer.Write([]byte("conflict"))
		})

		_, err := service.DiffRefs(context.Background(), DiffRefsInput{
			Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			From:       "main",
			To:         "feature",
			Output:     OutputKindStat,
		})
		if err == nil {
			t.Fatal("expected conflict error")
		}
		if apperrors.ExitCode(err) != 5 {
			t.Fatalf("expected conflict exit code 5, got %d (%v)", apperrors.ExitCode(err), err)
		}
	})
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

func TestDiffValidationBranches(t *testing.T) {
	service := NewService(nil)

	_, err := service.DiffRefs(context.Background(), DiffRefsInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		From:       "",
		To:         "feature",
		Output:     OutputKindRaw,
	})
	if err == nil {
		t.Fatal("expected validation error for missing from")
	}

	_, err = service.DiffPR(context.Background(), DiffPRInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}})
	if err == nil {
		t.Fatal("expected validation error for missing pull request id")
	}

	_, err = service.DiffCommit(context.Background(), DiffCommitInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}})
	if err == nil {
		t.Fatal("expected validation error for missing commit id")
	}

	_, err = service.DiffRefs(context.Background(), DiffRefsInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		From:       "main",
		To:         "feature",
		Output:     OutputKind("unknown"),
	})
	if err == nil {
		t.Fatal("expected validation error for unsupported output")
	}
}

func TestDiffHelpers(t *testing.T) {
	if pathOrDot("") != "." {
		t.Fatal("expected empty path to map to dot")
	}
	if pathOrDot(" seed.txt ") != "seed.txt" {
		t.Fatal("expected path trimming")
	}

	diffText := strings.Join([]string{
		"diff --git a/a.txt b/a.txt",
		"diff --git a/a.txt b/a.txt",
		"diff --git a/dev/null b/new.txt",
		"diff --git a/old.txt b/dev/null",
	}, "\n")

	names := extractNamesFromUnifiedDiff(diffText)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %#v", len(names), names)
	}
	if names[0] != "a.txt" || names[1] != "new.txt" || names[2] != "old.txt" {
		t.Fatalf("unexpected names extraction: %#v", names)
	}
}

func TestMapStatusErrorCoverage(t *testing.T) {
	if err := mapStatusError(http.StatusOK, nil); err != nil {
		t.Fatalf("expected nil on 2xx, got: %v", err)
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

func TestDiffPRPatchAndStatModes(t *testing.T) {
	t.Run("patch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("diff --git a/p.txt b/p.txt\n"))
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
			Output:        OutputKindPatch,
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(result.Patch, "diff --git") {
			t.Fatalf("expected patch output, got: %q", result.Patch)
		}
	})

	t.Run("stat", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"size":1,"isLastPage":true,"values":[]}`))
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
			Output:        OutputKindStat,
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if result.Stats == nil {
			t.Fatal("expected stats payload")
		}
	})
}

func TestDiffPRErrorBranches(t *testing.T) {
	t.Run("patch status error", func(t *testing.T) {
		service := newDiffServiceWithHandler(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte("unauthorized"))
		})

		_, err := service.DiffPR(context.Background(), DiffPRInput{
			Repository:    RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			PullRequestID: "7",
			Output:        OutputKindPatch,
		})
		if err == nil {
			t.Fatal("expected authentication error")
		}
		if apperrors.ExitCode(err) != 3 {
			t.Fatalf("expected auth exit code 3, got %d (%v)", apperrors.ExitCode(err), err)
		}
	})

	t.Run("stat status error", func(t *testing.T) {
		service := newDiffServiceWithHandler(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNotAcceptable)
			_, _ = writer.Write([]byte("not acceptable"))
		})

		_, err := service.DiffPR(context.Background(), DiffPRInput{
			Repository:    RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			PullRequestID: "7",
			Output:        OutputKindStat,
		})
		if err == nil {
			t.Fatal("expected permanent error")
		}
		if apperrors.ExitCode(err) != 1 {
			t.Fatalf("expected permanent exit code 1, got %d (%v)", apperrors.ExitCode(err), err)
		}
	})

	t.Run("raw status error", func(t *testing.T) {
		service := newDiffServiceWithHandler(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte("rate limited"))
		})

		_, err := service.DiffPR(context.Background(), DiffPRInput{
			Repository:    RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			PullRequestID: "7",
			Output:        OutputKindRaw,
		})
		if err == nil {
			t.Fatal("expected transient error")
		}
		if apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient exit code 10, got %d (%v)", apperrors.ExitCode(err), err)
		}
	})
}

func TestDiffRefsStatAndCommitWithPath(t *testing.T) {
	t.Run("refs stat", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"size":1,"isLastPage":true,"values":[]}`))
		}))
		defer server.Close()

		client, err := openapigenerated.NewClientWithResponses(server.URL)
		if err != nil {
			t.Fatalf("create generated client: %v", err)
		}

		service := NewService(client)
		result, err := service.DiffRefs(context.Background(), DiffRefsInput{
			Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			From:       "main",
			To:         "feature",
			Path:       "seed.txt",
			Output:     OutputKindStat,
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if result.Stats == nil {
			t.Fatal("expected stats payload")
		}
	})

	t.Run("commit path", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("diff --git a/seed.txt b/seed.txt\n"))
		}))
		defer server.Close()

		client, err := openapigenerated.NewClientWithResponses(server.URL)
		if err != nil {
			t.Fatalf("create generated client: %v", err)
		}

		service := NewService(client)
		result, err := service.DiffCommit(context.Background(), DiffCommitInput{
			Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
			CommitID:   "abc123",
			Path:       "seed.txt",
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.Contains(result.Patch, "diff --git") {
			t.Fatalf("expected patch output, got: %q", result.Patch)
		}
	})
}

func TestDiffPRDefaultRawAndUnsupportedOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/latest/projects/PRJ/repos/demo/pull-requests/42.diff" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte("diff --git a/a.txt b/a.txt\n"))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	result, err := service.DiffPR(context.Background(), DiffPRInput{
		Repository:    RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		PullRequestID: "42",
	})
	if err != nil {
		t.Fatalf("expected no error for default raw mode, got: %v", err)
	}
	if !strings.Contains(result.Patch, "diff --git") {
		t.Fatalf("expected default raw patch output, got: %q", result.Patch)
	}

	_, err = service.DiffPR(context.Background(), DiffPRInput{
		Repository:    RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		PullRequestID: "42",
		Output:        OutputKind("unsupported"),
	})
	if err == nil {
		t.Fatal("expected validation error for unsupported output mode")
	}
}

func TestDiffCommitStatusErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("missing commit"))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	_, err = service.DiffCommit(context.Background(), DiffCommitInput{
		Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"},
		CommitID:   "abc123",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
	if apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found exit code 4, got %d (%v)", apperrors.ExitCode(err), err)
	}
}

func TestDiffRefsTransportFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	}))
	baseURL := server.URL
	server.Close()

	client, err := openapigenerated.NewClientWithResponses(baseURL)
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
		t.Fatal("expected transient transport error")
	}
	if apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected transient exit code 10, got %d (%v)", apperrors.ExitCode(err), err)
	}
}

func TestDiffValidationAndHelperEdgeBranches(t *testing.T) {
	service := NewService(nil)

	_, err := service.DiffRefs(context.Background(), DiffRefsInput{Repository: RepositoryRef{}, From: "main", To: "feature"})
	if err == nil {
		t.Fatal("expected repository validation error")
	}

	_, err = service.DiffPR(context.Background(), DiffPRInput{Repository: RepositoryRef{ProjectKey: "PRJ"}, PullRequestID: "1", Output: OutputKindRaw})
	if err == nil {
		t.Fatal("expected repository validation error for diff pr")
	}

	_, err = service.DiffCommit(context.Background(), DiffCommitInput{Repository: RepositoryRef{ProjectKey: "PRJ"}, CommitID: "abc"})
	if err == nil {
		t.Fatal("expected repository validation error for diff commit")
	}

	names := extractNamesFromUnifiedDiff(strings.Join([]string{
		"diff --git",
		"diff --git a",
		"diff --git a/ b/",
		"diff --git a//dev/null b//dev/null",
	}, "\n"))
	if len(names) != 0 {
		t.Fatalf("expected no extracted names from malformed lines, got: %#v", names)
	}

	err = mapStatusError(http.StatusBadRequest, []byte("   "))
	if err == nil {
		t.Fatal("expected validation error for bad request")
	}
	if !strings.Contains(err.Error(), "Bad Request") {
		t.Fatalf("expected status text fallback in error message, got: %v", err)
	}
}

func TestDiffTransportFailureBranches(t *testing.T) {
	closedService := func(t *testing.T) *Service {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		}))
		baseURL := server.URL
		server.Close()

		client, err := openapigenerated.NewClientWithResponses(baseURL)
		if err != nil {
			t.Fatalf("create generated client: %v", err)
		}

		return NewService(client)
	}

	t.Run("diff refs patch transport", func(t *testing.T) {
		service := closedService(t)
		_, err := service.DiffRefs(context.Background(), DiffRefsInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, From: "main", To: "feature", Output: OutputKindPatch})
		if err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient transport error, got: %v", err)
		}
	})

	t.Run("diff refs stat transport", func(t *testing.T) {
		service := closedService(t)
		_, err := service.DiffRefs(context.Background(), DiffRefsInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, From: "main", To: "feature", Output: OutputKindStat})
		if err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient transport error, got: %v", err)
		}
	})

	t.Run("diff pr patch transport", func(t *testing.T) {
		service := closedService(t)
		_, err := service.DiffPR(context.Background(), DiffPRInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, PullRequestID: "1", Output: OutputKindPatch})
		if err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient transport error, got: %v", err)
		}
	})

	t.Run("diff pr stat transport", func(t *testing.T) {
		service := closedService(t)
		_, err := service.DiffPR(context.Background(), DiffPRInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, PullRequestID: "1", Output: OutputKindStat})
		if err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient transport error, got: %v", err)
		}
	})

	t.Run("diff pr raw transport", func(t *testing.T) {
		service := closedService(t)
		_, err := service.DiffPR(context.Background(), DiffPRInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, PullRequestID: "1", Output: OutputKindRaw})
		if err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient transport error, got: %v", err)
		}
	})

	t.Run("diff commit path transport", func(t *testing.T) {
		service := closedService(t)
		_, err := service.DiffCommit(context.Background(), DiffCommitInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, CommitID: "abc", Path: "seed.txt"})
		if err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient transport error, got: %v", err)
		}
	})

	t.Run("diff commit path status error", func(t *testing.T) {
		service := newDiffServiceWithHandler(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte("missing"))
		})

		_, err := service.DiffCommit(context.Background(), DiffCommitInput{Repository: RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, CommitID: "abc", Path: "seed.txt"})
		if err == nil || apperrors.ExitCode(err) != 4 {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})
}

func newDiffServiceWithHandler(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	return NewService(client)
}
