//go:build live

package live_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLiveGovernanceCLI(t *testing.T) {
	harness := newLiveHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project failed: %v", err)
	}

	repo := seeded.Repos[0]
	configureLiveCLIEnv(t, harness, seeded.Key, repo.Slug)

	// --- Issue 31: Group Permissions ---
	// Test listing groups (even if empty)
	output, err := executeLiveCLI(t, "--json", "project", "permissions", "groups", "list", seeded.Key)
	if err != nil {
		t.Fatalf("project group permissions list failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, `"groups"`) {
		t.Fatalf("expected groups in output: %s", output)
	}

	output, err = executeLiveCLI(t, "--json", "repo", "settings", "security", "permissions", "groups", "list", "--repo", seeded.Key+"/"+repo.Slug)
	if err != nil {
		t.Fatalf("repo group permissions list failed: %v\noutput: %s", err, output)
	}

	// Try to grant to stash-users if it exists (usually does in local stack)
	_, _ = executeLiveCLI(t, "project", "permissions", "groups", "grant", seeded.Key, "stash-users", "PROJECT_READ")
	_, _ = executeLiveCLI(t, "repo", "settings", "security", "permissions", "groups", "grant", "stash-users", "REPO_READ", "--repo", seeded.Key+"/"+repo.Slug)

	// --- Issue 32: Reviewers & Hooks ---
	// Test listing hooks
	output, err = executeLiveCLI(t, "--json", "hook", "list", "--project", seeded.Key)
	if err != nil {
		t.Fatalf("project hook list failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, `"hooks"`) {
		t.Fatalf("expected hooks in output: %s", output)
	}

	// Hook lifecycle on a built-in hook (com.atlassian.bitbucket.server.bitbucket-bundled-hooks:verify-committer-hook)
	hookKey := "com.atlassian.bitbucket.server.bitbucket-bundled-hooks:verify-committer-hook"
	_, _ = executeLiveCLI(t, "--json", "hook", "enable", hookKey, "--repo", seeded.Key+"/"+repo.Slug)

	// Test hook configuration (get)
	output, err = executeLiveCLI(t, "--json", "hook", "configure", hookKey, "--repo", seeded.Key+"/"+repo.Slug)
	if err == nil {
		// Try to update hook configuration (even with empty settings)
		_, _ = executeLiveCLI(t, "--json", "hook", "configure", hookKey, "{}", "--repo", seeded.Key+"/"+repo.Slug)
	}

	_, _ = executeLiveCLI(t, "--json", "hook", "disable", hookKey, "--repo", seeded.Key+"/"+repo.Slug)

	// Test listing reviewer conditions
	output, err = executeLiveCLI(t, "--json", "reviewer", "condition", "list", "--project", seeded.Key)
	if err != nil {
		t.Fatalf("project reviewer list failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, `"conditions"`) {
		t.Fatalf("expected conditions in output: %s", output)
	}

	// Reviewer condition lifecycle
	output, err = executeLiveCLI(t, "--json", "reviewer", "condition", "create", `{"requiredApprovals": 1}`, "--repo", seeded.Key+"/"+repo.Slug)
	if err == nil {
		// If successfully created (depends on default-reviewers plugin), we'll try to extract the ID and update/delete it
		var id string
		if strings.Contains(output, `"id":`) {
			// Basic extraction for JSON output
			parts := strings.Split(output, `"id":`)
			if len(parts) > 1 {
				idStr := strings.TrimSpace(strings.Split(parts[1], ",")[0])
				idStr = strings.TrimSpace(strings.Split(idStr, "}")[0])
				id = idStr
			}
		}

		if id != "" {
			_, _ = executeLiveCLI(t, "--json", "reviewer", "condition", "update", id, `{"requiredApprovals": 2}`, "--repo", seeded.Key+"/"+repo.Slug)
			_, _ = executeLiveCLI(t, "--json", "reviewer", "condition", "delete", id, "--repo", seeded.Key+"/"+repo.Slug)
		}
	}

	// --- Issue 33: PR Governance ---
	// Test getting PR settings
	output, err = executeLiveCLI(t, "--json", "repo", "settings", "pull-requests", "get", "--repo", seeded.Key+"/"+repo.Slug)
	if err != nil {
		t.Fatalf("repo PR settings get failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, `"pull_request_settings"`) {
		t.Fatalf("expected pull_request_settings in output: %s", output)
	}

	// Test updating PR strategy (safe update)
	output, err = executeLiveCLI(t, "--json", "repo", "settings", "pull-requests", "set-strategy", "merge-base", "--repo", seeded.Key+"/"+repo.Slug)
	if err != nil {
		// Some strategies might not be available depending on plugin config, but we try anyway
		t.Logf("repo set-strategy attempt output: %s", output)
	}

	// Test listing merge checks
	output, err = executeLiveCLI(t, "--json", "repo", "settings", "pull-requests", "merge-checks", "list", "--repo", seeded.Key+"/"+repo.Slug)
	if err != nil {
		t.Fatalf("repo merge-checks list failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, `"merge_checks"`) {
		t.Fatalf("expected merge_checks in output: %s", output)
	}
}
