package bulk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	qualityservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/quality"
	reposettings "github.com/vriesdemichael/bitbucket-server-cli/internal/services/reposettings"
)

func TestServiceRunnerOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, _ := openapi.NewClientWithResponsesFromConfig(config.AppConfig{BitbucketURL: server.URL})
	runner := NewServiceRunner(reposettings.NewService(client), qualityservice.NewService(client))
	repo := RepositoryTarget{ProjectKey: "PRJ", Slug: "repo"}

	testCases := []OperationSpec{
		{Type: OperationRepoPermissionUserGrant, Username: "u", Permission: "REPO_READ"},
		{Type: OperationRepoPermissionGroupGrant, Group: "g", Permission: "REPO_ADMIN"},
		{Type: OperationRepoWebhookCreate, Name: "h", URL: "http://h", Events: []string{"repo:refs_changed"}, Active: boolPtr(true)},
		{Type: OperationRepoPullRequestRequiredAllTasksComplete, RequiredAllTasksComplete: boolPtr(true)},
		{Type: OperationRepoPullRequestRequiredApproversCount, Count: intPtr(1)},
		{Type: OperationBuildRequiredCreate, Payload: map[string]any{"foo": "bar"}},
	}

	for _, tc := range testCases {
		t.Run(tc.Type, func(t *testing.T) {
			_, err := runner.Run(context.Background(), repo, tc)
			if err != nil {
				t.Fatalf("run failed: %v", err)
			}
		})
	}
}

func TestServiceRunnerUnconfigured(t *testing.T) {
	t.Run("nil runner", func(t *testing.T) {
		var runner *ServiceRunner
		_, err := runner.Run(context.Background(), RepositoryTarget{}, OperationSpec{Type: OperationRepoPermissionUserGrant})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing services", func(t *testing.T) {
		runner := &ServiceRunner{}
		repo := RepositoryTarget{ProjectKey: "P", Slug: "s"}
		ops := []OperationSpec{
			{Type: OperationRepoPermissionUserGrant},
			{Type: OperationRepoPermissionGroupGrant},
			{Type: OperationRepoWebhookCreate},
			{Type: OperationRepoPullRequestRequiredAllTasksComplete, RequiredAllTasksComplete: boolPtr(true)},
			{Type: OperationRepoPullRequestRequiredApproversCount, Count: intPtr(1)},
			{Type: OperationBuildRequiredCreate},
		}
		for _, op := range ops {
			_, err := runner.Run(context.Background(), repo, op)
			if err == nil {
				t.Fatalf("expected error for %s", op.Type)
			}
		}
	})
}
