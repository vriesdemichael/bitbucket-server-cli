package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthStatusSmoke(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "http://localhost:7990")
	t.Setenv("BITBUCKET_VERSION_TARGET", "9.4.16")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"auth", "status"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Target Bitbucket") {
		t.Fatalf("expected auth status output, got: %s", buffer.String())
	}

	if !strings.Contains(buffer.String(), "auth=none") {
		t.Fatalf("expected auth mode in output, got: %s", buffer.String())
	}

	if !strings.Contains(buffer.String(), "source=env/default") {
		t.Fatalf("expected auth source in output, got: %s", buffer.String())
	}
}

func TestAuthStatusJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "http://localhost:7990")
	t.Setenv("BITBUCKET_VERSION_TARGET", "9.4.16")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "auth", "status"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid json output, got: %s (%v)", buffer.String(), err)
	}

	if parsed["bitbucket_url"] != "http://localhost:7990" {
		t.Fatalf("unexpected bitbucket_url: %q", parsed["bitbucket_url"])
	}

	if parsed["auth_mode"] != "none" {
		t.Fatalf("unexpected auth_mode: %q", parsed["auth_mode"])
	}

	if parsed["auth_source"] != "env/default" {
		t.Fatalf("unexpected auth_source: %q", parsed["auth_source"])
	}
}

func TestAdminHealthSmoke(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/1.0/projects" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"admin", "health"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Bitbucket health: OK") {
		t.Fatalf("expected health output, got: %s", buffer.String())
	}
}

func TestAdminHealthJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "admin", "health"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid json output, got: %s (%v)", buffer.String(), err)
	}

	if healthy, ok := parsed["healthy"].(bool); !ok || !healthy {
		t.Fatalf("expected healthy=true, got: %#v", parsed["healthy"])
	}
}

func TestDiffRefsNameOnly(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/patch" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte("diff --git a/a.txt b/a.txt\ndiff --git a/dir/b.go b/dir/b.go\n"))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"diff", "refs", "main", "feature", "--name-only"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "a.txt") || !strings.Contains(output, "dir/b.go") {
		t.Fatalf("expected changed files in output, got: %s", output)
	}
}

func TestDiffRefsRejectsMultipleOutputModes(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "http://localhost:7990")
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"diff", "refs", "main", "feature", "--patch", "--stat"})

	err := command.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRepoSettingsSecurityPermissionsUsersList(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/permissions/users" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"values":[{"permission":"REPO_ADMIN","user":{"name":"admin","displayName":"Admin User"}}],"isLastPage":true}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "security", "permissions", "users", "list"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Admin User") || !strings.Contains(buffer.String(), "REPO_ADMIN") {
		t.Fatalf("expected user permissions output, got: %s", buffer.String())
	}
}

func TestRepoSettingsWorkflowWebhooksList(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/webhooks" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"values":[{"name":"ci"}],"size":1}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "workflow", "webhooks", "list"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Webhooks configured: 1") {
		t.Fatalf("expected webhook count output, got: %s", buffer.String())
	}
}

func TestRepoSettingsPullRequestsGet(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/settings/pull-requests" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"requiredAllTasksComplete":true,"requiredApprovers":{"enabled":true,"count":"2"}}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "pull-requests", "get"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "Required tasks complete: true") || !strings.Contains(output, "Required approvers: 2") {
		t.Fatalf("expected pull request settings summary, got: %s", output)
	}
}

func TestRepoSettingsSecurityPermissionsUsersGrant(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/permissions/users" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("name") != "alice" || request.URL.Query().Get("permission") != "REPO_WRITE" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("invalid query"))
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "security", "permissions", "users", "grant", "alice", "repo_write"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Granted REPO_WRITE to alice") {
		t.Fatalf("expected permission grant output, got: %s", buffer.String())
	}
}

func TestRepoSettingsWorkflowWebhooksCreate(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/webhooks" {
			http.NotFound(writer, request)
			return
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"name":"ci-hook"`) || !strings.Contains(string(body), `"url":"http://example.local/hook"`) {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("invalid payload"))
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"name":"ci-hook"}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "workflow", "webhooks", "create", "ci-hook", "http://example.local/hook", "--event", "repo:refs_changed"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Webhook created: ci-hook") {
		t.Fatalf("expected webhook create output, got: %s", buffer.String())
	}
}

func TestRepoSettingsPullRequestsUpdate(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/settings/pull-requests" {
			body, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(body), `"requiredAllTasksComplete":true`) {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte("invalid payload"))
				return
			}
			writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = writer.Write([]byte(`{"requiredAllTasksComplete":true}`))
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "pull-requests", "update", "--required-all-tasks-complete=true"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Updated pull-request settings: requiredAllTasksComplete=true") {
		t.Fatalf("expected pull-request update output, got: %s", buffer.String())
	}
}

func TestRepoSettingsWorkflowWebhooksDelete(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete || request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/webhooks/42" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "workflow", "webhooks", "delete", "42"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Webhook deleted: 42") {
		t.Fatalf("expected webhook delete output, got: %s", buffer.String())
	}
}

func TestRepoSettingsPullRequestsUpdateApprovers(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/settings/pull-requests" {
			http.NotFound(writer, request)
			return
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"requiredApprovers":2`) {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("invalid payload"))
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"requiredApprovers":2}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "settings", "pull-requests", "update-approvers", "--count", "2"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Updated pull-request settings: requiredApprovers=2") {
		t.Fatalf("expected pull-request approvers update output, got: %s", buffer.String())
	}
}
