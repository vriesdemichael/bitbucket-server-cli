package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchReposCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/1.0/repos" {
			http.NotFound(w, r)
			return
		}

		name := r.URL.Query().Get("name")
		if name == "demo" {
			_, _ = w.Write([]byte(`{"values":[{"slug":"demo","name":"Demo","project":{"key":"TEST"}}],"isLastPage":true}`))
			return
		}

		_, _ = w.Write([]byte(`{"values":[],"isLastPage":true}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	options := &rootOptions{JSON: false}
	cmd := newSearchCommand(options)

	output := new(bytes.Buffer)
	cmd.SetOut(output)
	cmd.SetArgs([]string{"repos", "demo"})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !bytes.Contains(output.Bytes(), []byte("TEST/demo\tDemo")) {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

func TestSearchCommitsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Commit API Request: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits" {
			_, _ = w.Write([]byte(`{"values":[{"id":"abcdef","displayId":"abcdef","message":"Fix bug"}],"isLastPage":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	options := &rootOptions{JSON: false}
	cmd := newSearchCommand(options)

	output := new(bytes.Buffer)
	cmd.SetOut(output)
	cmd.SetArgs([]string{"commits", "--repo", "TEST/demo"})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !bytes.Contains(output.Bytes(), []byte("abcdef\tFix bug")) {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

func TestSearchPRsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("PR API Request: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/api/1.0/dashboard/pull-requests" {
			_, _ = w.Write([]byte(`{"values":[{"id":42,"title":"Fix bug","state":"OPEN","open":true,"toRef":{"repository":{"slug":"demo","project":{"key":"TEST"}}}}],"isLastPage":true}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/rest/api/latest/projects/TEST/repos/demo/pull-requests") {
			_, _ = w.Write([]byte(`{"values":[{"id":43,"title":"Update docs","state":"OPEN","open":true}],"isLastPage":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)

	t.Run("dashboard", func(t *testing.T) {
		options := &rootOptions{JSON: false}
		cmd := newSearchCommand(options)
		output := new(bytes.Buffer)
		cmd.SetOut(output)
		cmd.SetArgs([]string{"prs"})

		err := cmd.ExecuteContext(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !bytes.Contains(output.Bytes(), []byte("[TEST/demo] #42\tOPEN\tFix bug")) {
			t.Fatalf("unexpected output: %s", output.String())
		}
	})

	t.Run("repo", func(t *testing.T) {
		options := &rootOptions{JSON: false}
		cmd := newSearchCommand(options)
		output := new(bytes.Buffer)
		cmd.SetOut(output)
		cmd.SetArgs([]string{"prs", "--repo", "TEST/demo"})

		err := cmd.ExecuteContext(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !bytes.Contains(output.Bytes(), []byte("#43\tOPEN\tUpdate docs")) {
			t.Fatalf("unexpected output: %s", output.String())
		}
	})
}
