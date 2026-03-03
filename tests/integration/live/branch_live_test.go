//go:build live

package live_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLiveCLIBranchLifecycle(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 2)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	configureLiveCLIEnv(t, harness, seeded.Key, repo.Slug)

	commitID := repo.CommitIDs[0]
	branchName := fmt.Sprintf("live-cli-branch-%d", time.Now().UnixNano()%100000)

	createBranchOutput, err := executeLiveCLI(t, "--json", "branch", "create", branchName, "--start-point", commitID)
	if err != nil {
		t.Fatalf("branch create failed: %v\noutput: %s", err, createBranchOutput)
	}
	createBranchPayload := decodeJSONMap(t, createBranchOutput)
	branchMap, ok := createBranchPayload["branch"].(map[string]any)
	if !ok || asString(branchMap["displayId"]) != branchName {
		t.Fatalf("expected created branch %s, got: %s", branchName, createBranchOutput)
	}

	listBranchOutput, err := executeLiveCLI(t, "branch", "list", "--filter", branchName)
	if err != nil {
		t.Fatalf("branch list (human) failed: %v\noutput: %s", err, listBranchOutput)
	}
	if !strings.Contains(listBranchOutput, branchName) {
		t.Fatalf("expected branch name in human branch list output, got: %s", listBranchOutput)
	}

	setDefaultOutput, err := executeLiveCLI(t, "--json", "branch", "default", "set", branchName)
	if err != nil {
		t.Fatalf("branch default set failed: %v\noutput: %s", err, setDefaultOutput)
	}
	if asString(decodeJSONMap(t, setDefaultOutput)["status"]) != "ok" {
		t.Fatalf("expected branch default set status ok, got: %s", setDefaultOutput)
	}

	getDefaultOutput, err := executeLiveCLI(t, "--json", "branch", "default", "get")
	if err != nil {
		t.Fatalf("branch default get failed: %v\noutput: %s", err, getDefaultOutput)
	}
	getDefaultPayload := decodeJSONMap(t, getDefaultOutput)
	defaultBranchMap, ok := getDefaultPayload["default_branch"].(map[string]any)
	if !ok || asString(defaultBranchMap["displayId"]) != branchName {
		t.Fatalf("expected default branch %s, got: %s", branchName, getDefaultOutput)
	}

	// Switch back to master to delete
	_, err = executeLiveCLI(t, "branch", "default", "set", "master")
	if err != nil {
		t.Fatalf("revert branch default set failed: %v", err)
	}

	deleteBranchOutput, err := executeLiveCLI(t, "--json", "branch", "delete", branchName)
	if err != nil {
		t.Fatalf("branch delete failed: %v\noutput: %s", err, deleteBranchOutput)
	}
	if asString(decodeJSONMap(t, deleteBranchOutput)["status"]) != "ok" {
		t.Fatalf("expected branch delete status ok, got: %s", deleteBranchOutput)
	}
}

func TestLiveCLIBranchRestrictionLifecycle(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	configureLiveCLIEnv(t, harness, seeded.Key, repo.Slug)

	createOutput, err := executeLiveCLI(
		t, "--json", "branch", "restriction", "create",
		"--type", "read-only",
		"--matcher-id", "refs/heads/master",
	)
	if err != nil {
		t.Fatalf("restriction create failed: %v\noutput: %s", err, createOutput)
	}
	createPayload := decodeJSONMap(t, createOutput)
	restrictionMap, ok := createPayload["restriction"].(map[string]any)
	if !ok {
		t.Fatalf("expected restriction object in create output, got: %s", createOutput)
	}
	var restrictionID string
	if id, ok := numericOrStringID(restrictionMap["id"]); ok {
		restrictionID = id
	} else {
		t.Fatalf("expected id in restriction object, got: %v", restrictionMap)
	}

	getOutput, err := executeLiveCLI(t, "branch", "restriction", "get", restrictionID)
	if err != nil {
		t.Fatalf("restriction get failed: %v\noutput: %s", err, getOutput)
	}
	if !strings.Contains(getOutput, "id="+restrictionID) || !strings.Contains(getOutput, "type=read-only") {
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

	listOutput, err := executeLiveCLI(t, "branch", "restriction", "list")
	if err != nil {
		t.Fatalf("restriction list failed: %v\noutput: %s", err, listOutput)
	}
	if !strings.Contains(listOutput, restrictionID) || !strings.Contains(listOutput, "no-deletes") {
		t.Fatalf("expected restriction in human list output, got: %s", listOutput)
	}

	deleteOutput, err := executeLiveCLI(t, "--json", "branch", "restriction", "delete", restrictionID)
	if err != nil {
		t.Fatalf("restriction delete failed: %v\noutput: %s", err, deleteOutput)
	}
	if asString(decodeJSONMap(t, deleteOutput)["status"]) != "ok" {
		t.Fatalf("expected delete status ok, got: %s", deleteOutput)
	}
}
