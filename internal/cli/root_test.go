package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestTagCreateJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/tags" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"displayId":"v1.0.0","latestCommit":"abc123","type":"ANNOTATED"}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "tag", "create", "v1.0.0", "--start-point", "abc123"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "v1.0.0") {
		t.Fatalf("expected created tag output, got: %s", buffer.String())
	}
}

func TestBuildStatusSetJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/rest/build-status/latest/commits/abc123" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "build", "status", "set", "abc123", "--key", "ci/main", "--state", "SUCCESSFUL", "--url", "https://example.invalid/build/1"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "\"status\": \"ok\"") {
		t.Fatalf("expected ok response, got: %s", buffer.String())
	}
}

func TestInsightsReportSetJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/rest/insights/latest/projects/TEST/repos/demo/commits/abc123/reports/lint" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"key":"lint","title":"Lint","result":"PASS"}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "insights", "report", "set", "abc123", "lint", "--body", `{"title":"Lint","result":"PASS","data":[{"title":"warnings","type":"NUMBER","value":{"value":0}}]}`})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "\"key\": \"lint\"") {
		t.Fatalf("expected report key in output, got: %s", buffer.String())
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

func TestPRListJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/latest/dashboard/pull-requests" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"isLastPage":true,"values":[{"id":123,"state":"OPEN","title":"Demo PR","fromRef":{"displayId":"feature/demo"},"toRef":{"displayId":"master","repository":{"slug":"demo","project":{"key":"TEST"}}}}]}`))
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
	command.SetArgs([]string{"--json", "pr", "list", "--repo", "TEST/demo"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid json output, got: %s (%v)", buffer.String(), err)
	}

	if count, ok := parsed["count"].(float64); !ok || int(count) != 1 {
		t.Fatalf("expected count=1, got: %#v", parsed["count"])
	}
}

func TestIssueListJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/jira/latest/projects/TEST/repos/demo/pull-requests/123/issues" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`[{"key":"DEMO-42","url":"https://jira.example/DEMO-42"}]`))
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
	command.SetArgs([]string{"--json", "issue", "list", "--pr", "123"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "DEMO-42") {
		t.Fatalf("expected issue key in output, got: %s", buffer.String())
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

func TestRepoCommentListCommitJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/rest/api/latest/projects/TEST/repos/demo/commits/abc123/comments" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("path") != "seed.txt" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("path query must be seed.txt"))
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"values":[{"id":101,"text":"hello commit","version":1}],"isLastPage":true}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "repo", "comment", "list", "--commit", "abc123", "--path", "seed.txt"})

	if err := command.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid json output, got: %s (%v)", buffer.String(), err)
	}

	contextPayload, ok := parsed["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context payload, got: %#v", parsed["context"])
	}

	if contextPayload["type"] != "commit" || contextPayload["commit_id"] != "abc123" {
		t.Fatalf("unexpected context payload: %#v", contextPayload)
	}

	comments, ok := parsed["comments"].([]any)
	if !ok || len(comments) != 1 {
		t.Fatalf("expected one comment in output, got: %#v", parsed["comments"])
	}
}

func TestRepoCommentCreatePRJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || !strings.HasSuffix(request.URL.Path, "/projects/TEST/repos/demo/pull-requests/77/comments") {
			http.NotFound(writer, request)
			return
		}

		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"text":"hello pr"`) {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("missing text"))
			return
		}

		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id":202,"text":"hello pr","version":0}`))
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "repo", "comment", "create", "--pr", "77", "--text", "hello pr"})

	if err := command.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid json output, got: %s (%v)", buffer.String(), err)
	}

	contextPayload, ok := parsed["context"].(map[string]any)
	if !ok || contextPayload["type"] != "pull_request" || contextPayload["pull_request_id"] != "77" {
		t.Fatalf("unexpected context payload: %#v", parsed["context"])
	}
}

func TestRepoCommentDeleteAutoResolvesVersion(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	var getCommentCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc123/comments/300":
			getCommentCalls.Add(1)
			writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = writer.Write([]byte(`{"id":300,"text":"to-delete","version":4}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc123/comments/300":
			if request.URL.Query().Get("version") != "4" {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte("expected version=4"))
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"repo", "comment", "delete", "--commit", "abc123", "--id", "300"})

	if err := command.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if getCommentCalls.Load() != 1 {
		t.Fatalf("expected one version lookup call, got: %d", getCommentCalls.Load())
	}

	if !strings.Contains(buffer.String(), "Deleted comment 300 (version=4)") {
		t.Fatalf("expected delete output with resolved version, got: %s", buffer.String())
	}
}
