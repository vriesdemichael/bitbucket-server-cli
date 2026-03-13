package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func configureDryRunEnv(t *testing.T, serverURL, projectKey, repoSlug string) {
	t.Helper()
	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", serverURL)
	t.Setenv("BITBUCKET_PROJECT_KEY", projectKey)
	t.Setenv("BITBUCKET_REPO_SLUG", repoSlug)
	t.Setenv("BITBUCKET_TOKEN", "test-token")
}

func TestInsightsAndPRDryRunPredictionBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/existing":
			_, _ = writer.Write([]byte(`{"key":"existing","title":"Existing"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/missing":
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/lint/annotations":
			_, _ = writer.Write([]byte(`{"annotations":[{"externalId":"ann1","message":"note","severity":"LOW"}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests":
			_, _ = writer.Write([]byte(`{"isLastPage":true,"values":[{"id":20,"title":"Existing","state":"OPEN","open":true,"closed":false,"fromRef":{"displayId":"feature/demo"},"toRef":{"displayId":"master"}}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/20":
			_, _ = writer.Write([]byte(`{"id":20,"title":"Same","description":"Same desc","state":"OPEN","open":true,"closed":false,"participants":[{"role":"REVIEWER","status":"APPROVED","approved":true,"user":{"name":"alice"}}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/21":
			_, _ = writer.Write([]byte(`{"id":21,"title":"Merged","state":"MERGED","open":false,"closed":true,"participants":[]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/22":
			_, _ = writer.Write([]byte(`{"id":22,"title":"Declined","state":"DECLINED","open":false,"closed":true,"participants":[]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/pull-requests/20/tasks":
			_, _ = writer.Write([]byte(`{"isLastPage":true,"values":[{"id":700,"text":"same task","state":"OPEN","resolved":false}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	configureDryRunEnv(t, server.URL, "TEST", "demo")

	out, err := executeTestCLI(t, "--json", "--dry-run", "insights", "report", "set", "abc", "existing", "--body", `{"title":"x","result":"PASS"}`)
	if err != nil || !strings.Contains(out, `"predicted_action": "update"`) {
		t.Fatalf("expected report set dry-run update prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "insights", "report", "delete", "abc", "missing")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected report delete dry-run no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "insights", "annotation", "add", "abc", "lint", "--body", `[{"externalId":"ann2","message":"m","severity":"LOW"}]`)
	if err != nil || !strings.Contains(out, `"predicted_action": "create"`) {
		t.Fatalf("expected annotation add dry-run create prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "insights", "annotation", "delete", "abc", "lint", "--external-id", "ann1")
	if err != nil || !strings.Contains(out, `"predicted_action": "delete"`) {
		t.Fatalf("expected annotation delete dry-run delete prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "insights", "annotation", "delete", "abc", "lint", "--external-id", "absent")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected annotation delete dry-run no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "create", "--from-ref", "feature/demo", "--to-ref", "master", "--title", "Same")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected pr create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "update", "20", "--title", "Same", "--description", "Same desc", "--version", "1")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "merge", "21")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr merge no-op prediction for merged PR, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "merge", "22")
	if err != nil || !strings.Contains(out, `"predicted_action": "blocked"`) {
		t.Fatalf("expected pr merge blocked prediction for declined PR, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "decline", "22")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr decline no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "reopen", "20")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr reopen no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "review", "approve", "20")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr approve no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "review", "unapprove", "21")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr unapprove no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "review", "reviewer", "add", "20", "--user", "alice")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr reviewer add no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "review", "reviewer", "remove", "21", "--user", "bob")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr reviewer remove no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "task", "create", "20", "--text", "dry-run task")
	if err != nil || !strings.Contains(out, `"predicted_action": "create"`) {
		t.Fatalf("expected pr task create dry-run create prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "task", "update", "20", "--task", "700", "--text", "same task", "--resolved=false")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr task update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "task", "update", "20", "--task", "999", "--text", "missing")
	if err != nil || !strings.Contains(out, `"predicted_action": "blocked"`) {
		t.Fatalf("expected pr task update blocked prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "pr", "task", "delete", "20", "--task", "999")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pr task delete no-op prediction, err=%v output=%s", err, out)
	}
}

func TestGovernanceAndRepoDryRunPredictionBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/conditions":
			_, _ = writer.Write([]byte(`[{"id":1,"requiredApprovals":1,"reviewers":[{"name":"alice"}]}]`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/repos/demo/conditions":
			_, _ = writer.Write([]byte(`[{"id":1,"requiredApprovals":1,"reviewers":[{"name":"alice"}]}]`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/PRJ/settings/hooks":
			_, _ = writer.Write([]byte(`{"values":[],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/permissions/users":
			_, _ = writer.Write([]byte(`{"values":[{"user":{"name":"alice"},"permission":"REPO_WRITE"}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/permissions/groups":
			_, _ = writer.Write([]byte(`{"values":[{"group":{"name":"devs"},"permission":"REPO_READ"}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/webhooks":
			_, _ = writer.Write([]byte(`[{"id":42,"name":"ci","url":"http://h"}]`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/settings/pull-requests":
			_, _ = writer.Write([]byte(`{"requiredAllTasksComplete":true,"requiredApprovers":{"enabled":true,"count":"2"},"mergeConfig":{"defaultStrategy":{"id":"merge-base"}}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/1":
			_, _ = writer.Write([]byte(`{"id":1,"text":"same comment","version":1}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/commits/abc/comments/2":
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	configureDryRunEnv(t, server.URL, "TEST", "demo")

	out, err := executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "delete", "2", "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected reviewer delete no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "create", `{"requiredApprovals":1,"reviewers":[{"name":"alice"}]}`, "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected reviewer create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "update", "2", `{"requiredApprovals":2}`, "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "blocked"`) {
		t.Fatalf("expected reviewer update blocked prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "hook", "enable", "missing", "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "blocked"`) {
		t.Fatalf("expected hook enable blocked prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "hook", "disable", "missing", "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "blocked"`) {
		t.Fatalf("expected hook disable blocked prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "hook", "configure", "missing", `{"required":true}`, "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "blocked"`) {
		t.Fatalf("expected hook configure blocked prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "security", "permissions", "users", "grant", "alice", "repo_write")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected repo users grant no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "security", "permissions", "users", "revoke", "nobody")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected repo users revoke no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "security", "permissions", "groups", "grant", "devs", "repo_read")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected repo groups grant no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "security", "permissions", "groups", "revoke", "devs")
	if err != nil || !strings.Contains(out, `"predicted_action": "delete"`) {
		t.Fatalf("expected repo groups revoke delete prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "workflow", "webhooks", "create", "ci", "http://h")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected webhook create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "workflow", "webhooks", "delete", "99")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected webhook delete no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "pull-requests", "update", "--required-all-tasks-complete=true")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pull-request update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "pull-requests", "update-approvers", "--count", "2")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pull-request update-approvers no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "settings", "pull-requests", "set-strategy", "merge-base")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected pull-request set-strategy no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "comment", "update", "--commit", "abc", "--id", "1", "--text", "same comment")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected repo comment update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "comment", "delete", "--commit", "abc", "--id", "2")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected repo comment delete no-op prediction, err=%v output=%s", err, out)
	}
}

func TestBranchProjectAdminTagDryRunPredictionBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/branches":
			_, _ = writer.Write([]byte(`{"values":[{"id":"refs/heads/master","displayId":"master"}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/TEST/repos/demo/default-branch":
			_, _ = writer.Write([]byte(`{"id":"refs/heads/master","displayId":"master"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/branch-permissions/latest/projects/TEST/repos/demo/restrictions":
			_, _ = writer.Write([]byte(`{"values":[{"id":10,"type":"read-only","matcher":{"id":"refs/heads/master","type":{"id":"BRANCH"}},"users":[{"name":"alice"}],"groups":["devs"],"accessKeys":[{"key":{"id":7}}]}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/branch-permissions/latest/projects/TEST/repos/demo/restrictions/10":
			_, _ = writer.Write([]byte(`{"id":10,"type":"read-only","matcher":{"id":"refs/heads/master","type":{"id":"BRANCH"}},"users":[{"name":"alice"}],"groups":["devs"],"accessKeys":[{"key":{"id":7}}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/branch-permissions/latest/projects/TEST/repos/demo/restrictions/99":
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/build-status/latest/commits/abc":
			_, _ = writer.Write([]byte(`{"values":[{"key":"ci","state":"SUCCESSFUL","url":"http://build"}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/required-builds/latest/projects/TEST/repos/demo/conditions":
			_, _ = writer.Write([]byte(`{"values":[{"id":5}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/PRJ":
			_, _ = writer.Write([]byte(`{"key":"PRJ","name":"Project","description":"Desc"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/PRZ":
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/PRJ/permissions/users":
			_, _ = writer.Write([]byte(`{"values":[{"user":{"name":"alice"},"permission":"PROJECT_READ"}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/PRJ/permissions/groups":
			_, _ = writer.Write([]byte(`{"values":[{"group":{"name":"devs"},"permission":"PROJECT_WRITE"}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/1.0/projects/PRJ/repos":
			_, _ = writer.Write([]byte(`{"values":[{"slug":"repo","name":"repo","public":false,"project":{"key":"PRJ"}}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/PRJ/repos/demo/tags":
			_, _ = writer.Write([]byte(`{"values":[{"id":"refs/tags/v1","displayId":"v1"}],"isLastPage":true}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/api/latest/projects/PRJ/repos/demo/tags/missing":
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	configureDryRunEnv(t, server.URL, "TEST", "demo")

	out, err := executeTestCLI(t, "--json", "--dry-run", "branch", "create", "master", "--start-point", "master")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected branch create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "branch", "default", "set", "master")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected branch default no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "branch", "model", "update", "master")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected branch model update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "branch", "restriction", "create", "--type", "read-only", "--matcher-type", "BRANCH", "--matcher-id", "refs/heads/master")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected branch restriction create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "branch", "restriction", "update", "10", "--type", "read-only", "--matcher-type", "BRANCH", "--matcher-id", "refs/heads/master", "--user", "alice", "--group", "devs", "--access-key-id", "7")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected branch restriction update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "branch", "restriction", "delete", "99")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected branch restriction delete no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "build", "status", "set", "abc", "--key", "ci", "--state", "SUCCESSFUL", "--url", "http://build")
	if err != nil || !strings.Contains(out, `"predicted_action": "update"`) {
		t.Fatalf("expected build status set update prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "build", "required", "create", "--body", `{"buildParentKeys":["ci"]}`)
	if err != nil || !strings.Contains(out, `"predicted_action": "create"`) {
		t.Fatalf("expected build required create prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "build", "required", "update", "5", "--body", `{"buildParentKeys":["ci"]}`)
	if err != nil || !strings.Contains(out, `"predicted_action": "update"`) {
		t.Fatalf("expected build required update prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "build", "required", "delete", "99")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected build required delete no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "project", "create", "PRJ", "--name", "Project")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected project create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "project", "update", "PRJ", "--name", "Project", "--description", "Desc")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected project update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "project", "delete", "PRZ")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected project delete no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "project", "permissions", "users", "grant", "PRJ", "alice", "PROJECT_READ")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected project users grant no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "project", "permissions", "groups", "grant", "PRJ", "devs", "PROJECT_WRITE")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected project groups grant no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "admin", "create", "--project", "PRJ", "--name", "repo")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected repo admin create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "admin", "fork", "--repo", "PRJ/demo", "--name", "forked")
	if err != nil || !strings.Contains(out, `"predicted_action": "create"`) {
		t.Fatalf("expected repo admin fork create prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "repo", "admin", "update", "--repo", "PRJ/demo")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected repo admin update no-op prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "tag", "create", "v1", "--repo", "PRJ/demo", "--start-point", "master")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected tag create conflict prediction, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "tag", "delete", "missing", "--repo", "PRJ/demo")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected tag delete no-op prediction, err=%v output=%s", err, out)
	}
}

func TestReviewerDryRunRepositoryAndProjectBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/repos/demo/conditions":
			_, _ = writer.Write([]byte(`[{"id":1,"requiredApprovals":1}]`))
		case request.Method == http.MethodGet && request.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/conditions":
			_, _ = writer.Write([]byte(`[{"id":1,"requiredApprovals":1}]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	configureDryRunEnv(t, server.URL, "TEST", "demo")

	out, err := executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "delete", "1", "--repo", "PRJ/demo")
	if err != nil || !strings.Contains(out, `"predicted_action": "delete"`) {
		t.Fatalf("expected reviewer repo delete dry-run to predict delete, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "delete", "2", "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "no-op"`) {
		t.Fatalf("expected reviewer project delete dry-run to predict no-op, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "create", `{"requiredApprovals":1}`, "--repo", "PRJ/demo")
	if err != nil || !strings.Contains(out, `"predicted_action": "conflict"`) {
		t.Fatalf("expected reviewer repo create dry-run to predict conflict, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "create", `{"requiredApprovals":2}`, "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "create"`) {
		t.Fatalf("expected reviewer project create dry-run to predict create, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "update", "1", `{"requiredApprovals":1}`, "--repo", "PRJ/demo")
	if err != nil || !strings.Contains(out, `"predicted_action": "update"`) {
		t.Fatalf("expected reviewer repo update dry-run to predict update, err=%v output=%s", err, out)
	}

	out, err = executeTestCLI(t, "--json", "--dry-run", "reviewer", "condition", "update", "2", `{"requiredApprovals":2}`, "--project", "PRJ")
	if err != nil || !strings.Contains(out, `"predicted_action": "blocked"`) {
		t.Fatalf("expected reviewer project update dry-run to predict blocked, err=%v output=%s", err, out)
	}
}
