package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowseCLICommandsMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/repo/files":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":["file1.txt"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/repo/raw/file1.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(`raw test content`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/repo/browse/file1.txt":
			if r.URL.Query().Get("blame") == "true" {
				_, _ = w.Write([]byte(`{"lines":[{"text":"hello blame"}],"blame":{"author":{"name":"test"}}}`))
			} else {
				_, _ = w.Write([]byte(`{"lines":[{"text":"hello file"}]}`))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/repo/commits":
			if r.URL.Query().Get("path") == "file1.txt" {
				_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":"hist-abc","displayId":"hist-abc","message":"hist"}]}`))
			} else {
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "repo")
	t.Setenv("BITBUCKET_TOKEN", "test-token")

	// Tree
	out, err := executeTestCLI(t, "repo", "browse", "tree")
	if err != nil {
		t.Fatalf("tree failed: %v", err)
	}
	if !strings.Contains(out, "file1.txt") {
		t.Fatalf("unexpected tree output: %s", out)
	}

	out, err = executeTestCLI(t, "--json", "repo", "browse", "tree")
	if err != nil {
		t.Fatalf("tree json failed: %v", err)
	}
	if !strings.Contains(out, `"files"`) {
		t.Fatalf("unexpected tree json output: %s", out)
	}

	// Raw
	out, err = executeTestCLI(t, "repo", "browse", "raw", "file1.txt")
	if err != nil {
		t.Fatalf("raw failed: %v", err)
	}
	if !strings.Contains(out, "raw test content") {
		t.Fatalf("unexpected raw output: %s", out)
	}

	// File
	out, err = executeTestCLI(t, "repo", "browse", "file", "file1.txt")
	if err != nil {
		t.Fatalf("file failed: %v", err)
	}
	if !strings.Contains(out, "hello file") {
		t.Fatalf("unexpected file output: %s", out)
	}

	out, err = executeTestCLI(t, "--json", "repo", "browse", "file", "file1.txt")
	if err != nil {
		t.Fatalf("file json failed: %v", err)
	}
	if !strings.Contains(out, `"hello file"`) {
		t.Fatalf("unexpected file json output: %s", out)
	}

	// Blame
	out, err = executeTestCLI(t, "repo", "browse", "blame", "file1.txt")
	if err != nil {
		t.Fatalf("blame failed: %v", err)
	}
	if !strings.Contains(out, "hello blame") {
		t.Fatalf("unexpected blame output: %s", out)
	}

	out, err = executeTestCLI(t, "--json", "repo", "browse", "blame", "file1.txt")
	if err != nil {
		t.Fatalf("blame json failed: %v", err)
	}
	if !strings.Contains(out, `"hello blame"`) {
		t.Fatalf("unexpected blame json output: %s", out)
	}

	// History
	out, err = executeTestCLI(t, "repo", "browse", "history", "file1.txt")
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	if !strings.Contains(out, "hist-abc\thist") {
		t.Fatalf("unexpected history output: %s", out)
	}

	out, err = executeTestCLI(t, "--json", "repo", "browse", "history", "file1.txt")
	if err != nil {
		t.Fatalf("history json failed: %v", err)
	}
	if !strings.Contains(out, `"commits"`) {
		t.Fatalf("unexpected history json output: %s", out)
	}
}

func TestBrowseCLIValidation(t *testing.T) {
	_, err := executeTestCLI(t, "repo", "browse", "raw")
	if err == nil {
		t.Fatal("expected err missing arg")
	}

	_, err = executeTestCLI(t, "repo", "browse", "file")
	if err == nil {
		t.Fatal("expected err missing arg")
	}

	_, err = executeTestCLI(t, "repo", "browse", "blame")
	if err == nil {
		t.Fatal("expected err missing arg")
	}

	_, err = executeTestCLI(t, "repo", "browse", "history")
	if err == nil {
		t.Fatal("expected err missing arg")
	}

	_, err = executeTestCLI(t, "repo", "browse", "tree", "a", "b")
	if err == nil {
		t.Fatal("expected err too many args")
	}
}
