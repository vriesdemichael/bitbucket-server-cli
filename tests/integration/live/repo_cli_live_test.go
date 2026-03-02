//go:build live

package live_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLiveCLIRepoListAndComments(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 2)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	configureLiveCLIEnv(t, harness, seeded.Key, repo.Slug)

	repoListOutput, err := executeLiveCLI(t, "--json", "repo", "list", "--limit", "50")
	if err != nil {
		t.Fatalf("repo list failed: %v\noutput: %s", err, repoListOutput)
	}
	if !jsonArrayContainsSlug(t, repoListOutput, repo.Slug) {
		t.Fatalf("expected repo slug %s in repo list output: %s", repo.Slug, repoListOutput)
	}

	commitID := repo.CommitIDs[0]
	createCommitOutput, err := executeLiveCLI(t, "--json", "repo", "comment", "create", "--commit", commitID, "--text", "live cli commit comment")
	if err != nil {
		t.Fatalf("repo comment create (commit) failed: %v\noutput: %s", err, createCommitOutput)
	}
	commitCommentID, ok := commentIDFromCreateOutput(createCommitOutput)
	if !ok {
		t.Fatalf("expected comment id in commit create output: %s", createCommitOutput)
	}
	commitCommentVersion, _ := commentVersionFromCreateOutput(createCommitOutput)

	listCommitOutput, err := executeLiveCLI(t, "--json", "repo", "comment", "list", "--commit", commitID, "--path", "seed.txt", "--limit", "25")
	if err != nil {
		t.Fatalf("repo comment list (commit) failed: %v\noutput: %s", err, listCommitOutput)
	}
	if !jsonObjectHasCommentsArray(t, listCommitOutput) {
		t.Fatalf("expected comments array in commit list output: %s", listCommitOutput)
	}

	humanListCommitOutput, err := executeLiveCLI(t, "repo", "comment", "list", "--commit", commitID, "--path", "seed.txt", "--limit", "25")
	if err != nil {
		t.Fatalf("repo comment list (commit human) failed: %v\noutput: %s", err, humanListCommitOutput)
	}
	if !strings.Contains(humanListCommitOutput, "No comments found") && !strings.Contains(humanListCommitOutput, "[") {
		t.Fatalf("expected human comment list output, got: %s", humanListCommitOutput)
	}

	updateCommitArgs := []string{"--json", "repo", "comment", "update", "--commit", commitID, "--id", commitCommentID, "--text", "live cli commit comment updated"}
	if commitCommentVersion != "" {
		updateCommitArgs = append(updateCommitArgs, "--version", commitCommentVersion)
	}
	updateCommitOutput, err := executeLiveCLI(t, updateCommitArgs...)
	if err != nil {
		t.Fatalf("repo comment update (commit) failed: %v\noutput: %s", err, updateCommitOutput)
	}
	updatedCommitVersion, ok := commentVersionFromCreateOutput(updateCommitOutput)
	if !ok {
		t.Fatalf("expected version in commit update output: %s", updateCommitOutput)
	}

	deleteCommitArgs := []string{"repo", "comment", "delete", "--commit", commitID, "--id", commitCommentID}
	if updatedCommitVersion != "" {
		deleteCommitArgs = append(deleteCommitArgs, "--version", updatedCommitVersion)
	}
	deleteCommitOutput, err := executeLiveCLI(t, deleteCommitArgs...)
	if err != nil {
		t.Fatalf("repo comment delete (commit) failed: %v\noutput: %s", err, deleteCommitOutput)
	}
	if !strings.Contains(deleteCommitOutput, "Deleted comment") {
		t.Fatalf("expected human delete output, got: %s", deleteCommitOutput)
	}

	branch := fmt.Sprintf("lt-repo-cli-%d", time.Now().UnixNano()%100000)
	if err := harness.pushCommitOnBranch(seeded.Key, repo.Slug, branch, "repo-cli-feature.txt"); err != nil {
		t.Fatalf("push commit on branch failed: %v", err)
	}

	pullRequestID, err := harness.createPullRequest(ctx, seeded.Key, repo.Slug, branch, "master")
	if err != nil {
		t.Fatalf("create pull request failed: %v", err)
	}

	createPROutput, err := executeLiveCLI(t, "--json", "repo", "comment", "create", "--pr", pullRequestID, "--text", "live cli pr comment")
	if err != nil {
		t.Fatalf("repo comment create (pr) failed: %v\noutput: %s", err, createPROutput)
	}
	prCommentID, ok := commentIDFromCreateOutput(createPROutput)
	if !ok {
		t.Fatalf("expected comment id in pr create output: %s", createPROutput)
	}
	prCommentVersion, _ := commentVersionFromCreateOutput(createPROutput)

	listPROutput, err := executeLiveCLI(t, "--json", "repo", "comment", "list", "--pr", pullRequestID, "--path", "repo-cli-feature.txt", "--limit", "25")
	if err != nil {
		t.Fatalf("repo comment list (pr) failed: %v\noutput: %s", err, listPROutput)
	}
	if !jsonObjectHasCommentsArray(t, listPROutput) {
		t.Fatalf("expected comments array in pr list output: %s", listPROutput)
	}

	updatePRArgs := []string{"--json", "repo", "comment", "update", "--pr", pullRequestID, "--id", prCommentID, "--text", "live cli pr comment updated"}
	if prCommentVersion != "" {
		updatePRArgs = append(updatePRArgs, "--version", prCommentVersion)
	}
	updatePROutput, err := executeLiveCLI(t, updatePRArgs...)
	if err != nil {
		t.Fatalf("repo comment update (pr) failed: %v\noutput: %s", err, updatePROutput)
	}
	updatedPRVersion, ok := commentVersionFromCreateOutput(updatePROutput)
	if !ok {
		t.Fatalf("expected version in pr update output: %s", updatePROutput)
	}

	deletePRArgs := []string{"repo", "comment", "delete", "--pr", pullRequestID, "--id", prCommentID}
	if updatedPRVersion != "" {
		deletePRArgs = append(deletePRArgs, "--version", updatedPRVersion)
	}
	deletePROutput, err := executeLiveCLI(t, deletePRArgs...)
	if err != nil {
		t.Fatalf("repo comment delete (pr) failed: %v\noutput: %s", err, deletePROutput)
	}
	if !strings.Contains(deletePROutput, "Deleted comment") {
		t.Fatalf("expected human delete output, got: %s", deletePROutput)
	}
}

func TestLiveCLIRepoSettingsSurface(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	configureLiveCLIEnv(t, harness, seeded.Key, repo.Slug)

	permissionListOutput, err := executeLiveCLI(t, "--json", "repo", "settings", "security", "permissions", "users", "list", "--limit", "100")
	if err != nil {
		t.Fatalf("repo settings permissions users list failed: %v\noutput: %s", err, permissionListOutput)
	}
	permissionListPayload := decodeJSONMap(t, permissionListOutput)
	if _, ok := permissionListPayload["users"]; !ok {
		t.Fatalf("expected users field in permissions list output: %s", permissionListOutput)
	}

	username := harness.config.BitbucketUsername
	if strings.TrimSpace(username) != "" {
		grantOutput, grantErr := executeLiveCLI(t, "--json", "repo", "settings", "security", "permissions", "users", "grant", username, "repo_write")
		if grantErr != nil {
			t.Fatalf("repo settings permissions users grant failed: %v\noutput: %s", grantErr, grantOutput)
		}
		if asString(decodeJSONMap(t, grantOutput)["status"]) != "ok" {
			t.Fatalf("expected grant status ok, got: %s", grantOutput)
		}
	}

	webhooksListOutput, err := executeLiveCLI(t, "--json", "repo", "settings", "workflow", "webhooks", "list")
	if err != nil {
		t.Fatalf("repo settings workflow webhooks list failed: %v\noutput: %s", err, webhooksListOutput)
	}
	webhooksListPayload := decodeJSONMap(t, webhooksListOutput)
	if _, ok := webhooksListPayload["webhooks"]; !ok {
		t.Fatalf("expected webhooks field in webhooks list output: %s", webhooksListOutput)
	}

	webhookName := fmt.Sprintf("lt-cli-webhook-%d", time.Now().UnixNano()%100000)
	createWebhookOutput, err := executeLiveCLI(t, "--json", "repo", "settings", "workflow", "webhooks", "create", webhookName, "http://localhost:65535/hook", "--event", "repo:refs_changed")
	if err != nil {
		t.Fatalf("repo settings workflow webhooks create failed: %v\noutput: %s", err, createWebhookOutput)
	}
	webhookID, ok := webhookIDFromCreateOutput(createWebhookOutput)
	if ok {
		deleteWebhookOutput, deleteErr := executeLiveCLI(t, "--json", "repo", "settings", "workflow", "webhooks", "delete", webhookID)
		if deleteErr != nil {
			t.Fatalf("repo settings workflow webhooks delete failed: %v\noutput: %s", deleteErr, deleteWebhookOutput)
		}
		if asString(decodeJSONMap(t, deleteWebhookOutput)["status"]) != "ok" {
			t.Fatalf("expected webhook delete status ok, got: %s", deleteWebhookOutput)
		}
	}

	pullRequestsGetOutput, err := executeLiveCLI(t, "--json", "repo", "settings", "pull-requests", "get")
	if err != nil {
		t.Fatalf("repo settings pull-requests get failed: %v\noutput: %s", err, pullRequestsGetOutput)
	}
	getPayload := decodeJSONMap(t, pullRequestsGetOutput)
	if _, ok := getPayload["pull_request_settings"]; !ok {
		t.Fatalf("expected pull_request_settings field in get output: %s", pullRequestsGetOutput)
	}

	pullRequestsUpdateOutput, err := executeLiveCLI(t, "--json", "repo", "settings", "pull-requests", "update", "--required-all-tasks-complete=true")
	if err != nil {
		t.Fatalf("repo settings pull-requests update failed: %v\noutput: %s", err, pullRequestsUpdateOutput)
	}
	if asString(decodeJSONMap(t, pullRequestsUpdateOutput)["status"]) != "ok" {
		t.Fatalf("expected pull-requests update status ok, got: %s", pullRequestsUpdateOutput)
	}

	pullRequestsApproversOutput, err := executeLiveCLI(t, "--json", "repo", "settings", "pull-requests", "update-approvers", "--count", "2")
	if err != nil {
		t.Fatalf("repo settings pull-requests update-approvers failed: %v\noutput: %s", err, pullRequestsApproversOutput)
	}
	if asString(decodeJSONMap(t, pullRequestsApproversOutput)["status"]) != "ok" {
		t.Fatalf("expected pull-requests update-approvers status ok, got: %s", pullRequestsApproversOutput)
	}

	humanPermissionListOutput, err := executeLiveCLI(t, "repo", "settings", "security", "permissions", "users", "list", "--limit", "10")
	if err != nil {
		t.Fatalf("repo settings permissions users list (human) failed: %v\noutput: %s", err, humanPermissionListOutput)
	}
	if strings.TrimSpace(humanPermissionListOutput) == "" {
		t.Fatalf("expected non-empty human permissions output")
	}

	humanWebhooksListOutput, err := executeLiveCLI(t, "repo", "settings", "workflow", "webhooks", "list")
	if err != nil {
		t.Fatalf("repo settings webhooks list (human) failed: %v\noutput: %s", err, humanWebhooksListOutput)
	}
	if !strings.Contains(humanWebhooksListOutput, "Webhooks configured:") {
		t.Fatalf("expected human webhooks output, got: %s", humanWebhooksListOutput)
	}

	humanPullRequestsGetOutput, err := executeLiveCLI(t, "repo", "settings", "pull-requests", "get")
	if err != nil {
		t.Fatalf("repo settings pull-requests get (human) failed: %v\noutput: %s", err, humanPullRequestsGetOutput)
	}
	if !strings.Contains(humanPullRequestsGetOutput, "Required tasks complete:") {
		t.Fatalf("expected human pull-request settings output, got: %s", humanPullRequestsGetOutput)
	}

	humanPullRequestsUpdateOutput, err := executeLiveCLI(t, "repo", "settings", "pull-requests", "update", "--required-all-tasks-complete=false")
	if err != nil {
		t.Fatalf("repo settings pull-requests update (human) failed: %v\noutput: %s", err, humanPullRequestsUpdateOutput)
	}
	if !strings.Contains(humanPullRequestsUpdateOutput, "Updated pull-request settings") {
		t.Fatalf("expected human pull-requests update output, got: %s", humanPullRequestsUpdateOutput)
	}

	humanPullRequestsApproversOutput, err := executeLiveCLI(t, "repo", "settings", "pull-requests", "update-approvers", "--count", "1")
	if err != nil {
		t.Fatalf("repo settings pull-requests update-approvers (human) failed: %v\noutput: %s", err, humanPullRequestsApproversOutput)
	}
	if !strings.Contains(humanPullRequestsApproversOutput, "Updated pull-request settings") {
		t.Fatalf("expected human pull-requests update-approvers output, got: %s", humanPullRequestsApproversOutput)
	}
}

func jsonArrayContainsSlug(t *testing.T, output string, slug string) bool {
	t.Helper()

	items := make([]map[string]any, 0)
	if err := unmarshalJSONArray(output, &items); err != nil {
		t.Fatalf("expected json array output, got parse error %v for: %s", err, output)
	}

	for _, item := range items {
		if asString(item["slug"]) == slug {
			return true
		}
	}

	return false
}

func jsonObjectHasCommentsArray(t *testing.T, output string) bool {
	t.Helper()

	payload := decodeJSONMap(t, output)
	_, ok := payload["comments"].([]any)
	return ok
}

func commentIDFromCreateOutput(output string) (string, bool) {
	payload := map[string]any{}
	if err := unmarshalJSONObject(output, &payload); err != nil {
		return "", false
	}

	comment, ok := payload["comment"].(map[string]any)
	if !ok {
		return "", false
	}

	return numericOrStringID(comment["id"])
}

func commentVersionFromCreateOutput(output string) (string, bool) {
	payload := map[string]any{}
	if err := unmarshalJSONObject(output, &payload); err != nil {
		return "", false
	}

	comment, ok := payload["comment"].(map[string]any)
	if !ok {
		return "", false
	}

	return numericOrStringID(comment["version"])
}

func webhookIDFromCreateOutput(output string) (string, bool) {
	payload := map[string]any{}
	if err := unmarshalJSONObject(output, &payload); err != nil {
		return "", false
	}

	webhook, ok := payload["webhook"].(map[string]any)
	if !ok {
		return "", false
	}

	return numericOrStringID(webhook["id"])
}

func numericOrStringID(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	case float64:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case int:
		return strconv.Itoa(typed), true
	default:
		return "", false
	}
}

func unmarshalJSONObject(value string, target *map[string]any) error {
	return json.Unmarshal([]byte(value), target)
}

func unmarshalJSONArray(value string, target *[]map[string]any) error {
	return json.Unmarshal([]byte(value), target)
}