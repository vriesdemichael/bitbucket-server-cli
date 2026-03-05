//go:build live

package live_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLiveCLIBranchLifecycle(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	configureLiveCLIEnv(t, harness, seeded.Key, repo.Slug)

	branchName := "feature/live-test-branch"
	startPoint := repo.CommitIDs[0]

	// Create branch
	createOutput, err := executeLiveCLI(t, "--json", "branch", "create", branchName, "--start-point", startPoint)
	if err != nil {
		t.Fatalf("branch create failed: %v\noutput: %s", err, createOutput)
	}
	createPayload := decodeJSONMap(t, createOutput)
	branchObj, ok := createPayload["branch"].(map[string]any)
	if !ok {
		branchObj = createPayload
	}
	if asString(branchObj["displayId"]) != branchName {
		t.Fatalf("expected branch displayId %s, got: %s", branchName, createOutput)
	}

	// List branches (human output)
	listOutput, err := executeLiveCLI(t, "branch", "list")
	if err != nil {
		t.Fatalf("branch list failed: %v\noutput: %s", err, listOutput)
	}
	if !strings.Contains(listOutput, branchName) {
		t.Fatalf("expected branch %s in list output, got: %s", branchName, listOutput)
	}

	// Get default branch
	defaultOutput, err := executeLiveCLI(t, "--json", "branch", "default", "get")
	if err != nil {
		t.Fatalf("branch default get failed: %v\noutput: %s", err, defaultOutput)
	}
	defaultPayload := decodeJSONMap(t, defaultOutput)
	defaultBranchObj, ok := defaultPayload["default_branch"].(map[string]any)
	if !ok {
		defaultBranchObj = defaultPayload
	}
	if asString(defaultBranchObj["displayId"]) == "" && asString(defaultBranchObj["id"]) == "" {
		t.Fatalf("expected default branch displayId or id, got: %s", defaultOutput)
	}

	/*
		// Find by commit
		time.Sleep(1 * time.Second)
		findOutput, err := executeLiveCLI(t, "--json", "branch", "model", "inspect", startPoint)
		if err != nil {
			t.Fatalf("branch model inspect failed: %v\noutput: %s", err, findOutput)
		}
		findPayload := decodeJSONMap(t, findOutput)
		refs, ok := findPayload["refs"].([]any)
		if !ok || len(refs) == 0 {
			t.Fatalf("expected refs in branch model inspect output, got: %s", findOutput)
		}
	*/

	// Delete branch
	deleteOutput, err := executeLiveCLI(t, "branch", "delete", branchName)
	if err != nil {
		t.Fatalf("branch delete failed: %v\noutput: %s", err, deleteOutput)
	}
}

func TestLiveCLIBranchRestrictionLifecycle(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	configureLiveCLIEnv(t, harness, seeded.Key, repo.Slug)

	// Create restriction
	createOutput, err := executeLiveCLI(
		t, "--json", "branch", "restriction", "create",
		"--type", "read-only",
		"--matcher-id", "refs/heads/master",
	)
	if err != nil {
		t.Fatalf("restriction create failed: %v\noutput: %s", err, createOutput)
	}
	createPayload := decodeJSONMap(t, createOutput)
	restrictionID := ""
	if restriction, ok := createPayload["restriction"].(map[string]any); ok {
		restrictionID = asString(restriction["id"])
	} else {
		restrictionID = asString(createPayload["id"])
	}

	if restrictionID == "" {
		t.Fatalf("expected restriction id in output, got: %s", createOutput)
	}

	// Get restriction
	getOutput, err := executeLiveCLI(t, "branch", "restriction", "get", restrictionID)
	if err != nil {
		t.Fatalf("restriction get failed: %v\noutput: %s", err, getOutput)
	}
	if !strings.Contains(getOutput, restrictionID) || !strings.Contains(getOutput, "read-only") {
		t.Fatalf("expected id and type in human get output, got: %s", getOutput)
	}

	updateOutput, err := executeLiveCLI(
		t, "--json", "branch", "restriction", "update", restrictionID,
		"--type", "no-deletes",
		"--matcher-id", "refs/heads/master",
	)
	if err != nil {
		t.Fatalf("restriction update failed: %v\noutput: %s", err, updateOutput)
	}

	// Update the ID as it changed during delete+recreate (our update implementation for single restrictions)
	updatePayload := decodeJSONMap(t, updateOutput)
	if restriction, ok := updatePayload["restriction"].(map[string]any); ok {
		restrictionID = asString(restriction["id"])
	} else {
		restrictionID = asString(updatePayload["id"])
	}

	listOutput, err := executeLiveCLI(t, "branch", "restriction", "list")
	if err != nil {
		t.Fatalf("restriction list failed: %v\noutput: %s", err, listOutput)
	}
	if !strings.Contains(listOutput, restrictionID) || !strings.Contains(listOutput, "no-deletes") {
		t.Fatalf("expected restriction %s in human list output, got: %s", restrictionID, listOutput)
	}

	deleteOutput, err := executeLiveCLI(t, "--json", "branch", "restriction", "delete", restrictionID)
	if err != nil {
		t.Fatalf("restriction delete failed: %v\noutput: %s", err, deleteOutput)
	}
	if asString(decodeJSONMap(t, deleteOutput)["status"]) != "ok" {
		t.Fatalf("expected delete status ok, got: %s", deleteOutput)
	}
}
