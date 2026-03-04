//go:build live

package live_test

import (
	"context"
	"testing"
	"time"
)

func TestLiveCLIRepoAdminLifecycle(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project failed: %v", err)
	}

	configureLiveCLIEnv(t, harness, seeded.Key, "test-repo")

	// Create
	createOutput, err := executeLiveCLI(t, "--json", "repo", "admin", "create", "--project", seeded.Key, "--name", "test-repo", "--description", "test desc")
	if err != nil {
		t.Fatalf("repo create failed: %v\noutput: %s", err, createOutput)
	}
	createPayload := decodeJSONMap(t, createOutput)
	repoObj, ok := createPayload["repository"].(map[string]any)
	if !ok || asString(repoObj["name"]) != "test-repo" {
		t.Fatalf("expected repository object with name, got: %s", createOutput)
	}

	// Update
	updateOutput, err := executeLiveCLI(t, "--json", "repo", "admin", "update", "--name", "test-repo-updated")
	if err != nil {
		t.Fatalf("repo update failed: %v\noutput: %s", err, updateOutput)
	}
	updatePayload := decodeJSONMap(t, updateOutput)
	updateObj, ok := updatePayload["repository"].(map[string]any)
	if !ok || asString(updateObj["name"]) != "test-repo-updated" {
		t.Fatalf("expected repository updated name, got: %s", updateOutput)
	}

	// Delete
	deleteOutput, err := executeLiveCLI(t, "--json", "repo", "admin", "delete")
	if err != nil {
		t.Fatalf("repo delete failed: %v\noutput: %s", err, deleteOutput)
	}
	deletePayload := decodeJSONMap(t, deleteOutput)
	if asString(deletePayload["status"]) != "ok" {
		t.Fatalf("expected delete status ok, got: %s", deleteOutput)
	}
}
