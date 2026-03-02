package reposettings

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func TestListRepositoryPermissionUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/latest/projects/PRJ/repos/demo/permissions/users" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"values":[{"permission":"REPO_ADMIN","user":{"name":"admin","displayName":"Admin User"}}],"isLastPage":true}`))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	users, err := service.ListRepositoryPermissionUsers(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(users) != 1 || users[0].Name != "admin" || users[0].Permission != "REPO_ADMIN" {
		t.Fatalf("unexpected users payload: %#v", users)
	}
}

func TestListRepositoryWebhooks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/latest/projects/PRJ/repos/demo/webhooks" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"values":[{"name":"ci-hook"}],"size":1}`))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	webhooks, err := service.ListRepositoryWebhooks(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if webhooks.Count != 1 {
		t.Fatalf("expected webhook count 1, got %d", webhooks.Count)
	}
}

func TestGetRepositoryPullRequestSettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/latest/projects/PRJ/repos/demo/settings/pull-requests" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"requiredAllTasksComplete":true}`))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	settings, err := service.GetRepositoryPullRequestSettings(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if requiredTasks, ok := settings["requiredAllTasksComplete"].(bool); !ok || !requiredTasks {
		t.Fatalf("expected requiredAllTasksComplete=true, got %#v", settings["requiredAllTasksComplete"])
	}
}

func TestPermissionsNotFoundMapsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("missing"))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	_, err = service.ListRepositoryPermissionUsers(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found exit code 4, got %d (%v)", apperrors.ExitCode(err), err)
	}
}

func TestGrantRepositoryUserPermission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/api/latest/projects/PRJ/repos/demo/permissions/users" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("permission") != "REPO_WRITE" || request.URL.Query().Get("name") != "alice" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("invalid query"))
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	if err := service.GrantRepositoryUserPermission(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, "alice", "repo_write"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateRepositoryWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/latest/projects/PRJ/repos/demo/webhooks" {
			http.NotFound(writer, request)
			return
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), "\"name\":\"ci\"") || !strings.Contains(string(body), "\"url\":\"http://example.local/hook\"") {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("invalid body"))
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"name":"ci","url":"http://example.local/hook"}`))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	payload, err := service.CreateRepositoryWebhook(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, WebhookCreateInput{
		Name:   "ci",
		URL:    "http://example.local/hook",
		Events: []string{"repo:refs_changed"},
		Active: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if payload == nil {
		t.Fatal("expected created webhook payload")
	}
}

func TestUpdateRepositoryPullRequestRequiredAllTasks(t *testing.T) {
	hitUpdate := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.URL.Path == "/api/latest/projects/PRJ/repos/demo/settings/pull-requests" {
			hitUpdate = true
			body, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(body), `"requiredAllTasksComplete":true`) {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte("missing requiredAllTasksComplete=true"))
				return
			}
			writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = writer.Write([]byte(`{"requiredAllTasksComplete":true}`))
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	settings, err := service.UpdateRepositoryPullRequestRequiredAllTasks(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !hitUpdate {
		t.Fatal("expected update call to be issued")
	}
	if value, ok := settings["requiredAllTasksComplete"].(bool); !ok || !value {
		t.Fatalf("expected requiredAllTasksComplete=true, got %#v", settings["requiredAllTasksComplete"])
	}
}

func TestDeleteRepositoryWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete || request.URL.Path != "/api/latest/projects/PRJ/repos/demo/webhooks/42" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	if err := service.DeleteRepositoryWebhook(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, "42"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateRepositoryPullRequestRequiredApproversCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/latest/projects/PRJ/repos/demo/settings/pull-requests" {
			http.NotFound(writer, request)
			return
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"requiredApprovers":2`) {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("missing requiredApprovers payload"))
			return
		}
		writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = writer.Write([]byte(`{"requiredApprovers":2}`))
	}))
	defer server.Close()

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	service := NewService(client)
	settings, err := service.UpdateRepositoryPullRequestRequiredApproversCount(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, 2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if value, ok := settings["requiredApprovers"].(float64); !ok || int(value) != 2 {
		t.Fatalf("expected required approvers count 2, got %#v", settings["requiredApprovers"])
	}
}

func TestRepositorySettingsHelperCoverage(t *testing.T) {
	permission, err := normalizeRepositoryPermission(" repo_read ")
	if err != nil || string(permission) != "REPO_READ" {
		t.Fatalf("expected REPO_READ normalization, got permission=%q err=%v", permission, err)
	}

	_, err = normalizeRepositoryPermission("invalid")
	if err == nil {
		t.Fatal("expected validation error for invalid permission")
	}

	if err := mapStatusError(http.StatusCreated, nil); err != nil {
		t.Fatalf("expected nil for success status, got: %v", err)
	}

	tests := []struct {
		status   int
		exitCode int
	}{
		{status: http.StatusBadRequest, exitCode: 2},
		{status: http.StatusUnauthorized, exitCode: 3},
		{status: http.StatusForbidden, exitCode: 3},
		{status: http.StatusNotFound, exitCode: 4},
		{status: http.StatusConflict, exitCode: 5},
		{status: http.StatusTooManyRequests, exitCode: 10},
		{status: http.StatusInternalServerError, exitCode: 10},
		{status: http.StatusNotAcceptable, exitCode: 1},
	}

	for _, testCase := range tests {
		err := mapStatusError(testCase.status, []byte("err"))
		if err == nil {
			t.Fatalf("expected error for status %d", testCase.status)
		}
		if apperrors.ExitCode(err) != testCase.exitCode {
			t.Fatalf("expected exit code %d for status %d, got %d", testCase.exitCode, testCase.status, apperrors.ExitCode(err))
		}
	}
}

func TestRepositorySettingsJSONFallbackAndValidationBranches(t *testing.T) {
	service := newServiceWithBaseURL(t, func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/latest/projects/PRJ/repos/demo/webhooks":
			_, _ = writer.Write([]byte(`[1,2]`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/latest/projects/PRJ/repos/demo/webhooks":
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte("created"))
		case request.Method == http.MethodPost && request.URL.Path == "/api/latest/projects/PRJ/repos/demo/settings/pull-requests":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("updated"))
		default:
			http.NotFound(writer, request)
		}
	})

	repo := RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}

	webhooks, err := service.ListRepositoryWebhooks(context.Background(), repo)
	if err != nil {
		t.Fatalf("expected no error listing array webhooks payload, got: %v", err)
	}
	if webhooks.Count != 2 {
		t.Fatalf("expected webhook count=2 from array payload, got: %d", webhooks.Count)
	}

	created, err := service.CreateRepositoryWebhook(context.Background(), repo, WebhookCreateInput{Name: "ci", URL: "http://example.local/hook"})
	if err != nil {
		t.Fatalf("expected no error creating webhook with non-json response, got: %v", err)
	}
	if created != nil {
		t.Fatalf("expected nil payload for non-json create response, got: %#v", created)
	}

	allTasksSettings, err := service.UpdateRepositoryPullRequestRequiredAllTasks(context.Background(), repo, true)
	if err != nil {
		t.Fatalf("expected no error updating all tasks with fallback response, got: %v", err)
	}
	if value, ok := allTasksSettings["requiredAllTasksComplete"].(bool); !ok || !value {
		t.Fatalf("expected fallback requiredAllTasksComplete=true, got: %#v", allTasksSettings)
	}

	approverSettings, err := service.UpdateRepositoryPullRequestRequiredApproversCount(context.Background(), repo, 3)
	if err != nil {
		t.Fatalf("expected no error updating approvers with fallback response, got: %v", err)
	}
	if value, ok := approverSettings["requiredApprovers"].(int); !ok || value != 3 {
		t.Fatalf("expected fallback requiredApprovers=3, got: %#v", approverSettings)
	}

	_, err = service.UpdateRepositoryPullRequestRequiredApproversCount(context.Background(), repo, -1)
	if err == nil {
		t.Fatal("expected validation error for negative approvers count")
	}

	if err := service.DeleteRepositoryWebhook(context.Background(), repo, " "); err == nil {
		t.Fatal("expected validation error for empty webhook id")
	}
}

func newServiceWithBaseURL(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	return NewService(client)
}

func TestRepositorySettingsAdditionalBranches(t *testing.T) {
	t.Run("permission users pagination and defaults", func(t *testing.T) {
		service := newServiceWithBaseURL(t, func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet || request.URL.Path != "/api/latest/projects/PRJ/repos/demo/permissions/users" {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
			if request.URL.Query().Get("limit") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte("expected default limit=100"))
				return
			}
			if request.URL.Query().Get("start") == "1" {
				_, _ = writer.Write([]byte(`{"isLastPage":true,"values":[{"permission":"REPO_WRITE","user":{"name":"bob"}}]}`))
				return
			}
			_, _ = writer.Write([]byte(`{"isLastPage":false,"nextPageStart":1,"values":[{"permission":"REPO_READ","user":{"name":"alice","displayName":"Alice"}}]}`))
		})

		users, err := service.ListRepositoryPermissionUsers(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, 0)
		if err != nil {
			t.Fatalf("expected paginated permission users success, got: %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("expected 2 users from pagination, got: %d", len(users))
		}
	})

	t.Run("webhooks invalid json and transport", func(t *testing.T) {
		invalidService := newServiceWithBaseURL(t, func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("not-json"))
		})
		if _, err := invalidService.ListRepositoryWebhooks(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}); err == nil {
			t.Fatal("expected invalid json payload error")
		}

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		}))
		baseURL := server.URL
		server.Close()

		client, err := openapigenerated.NewClientWithResponses(baseURL)
		if err != nil {
			t.Fatalf("create generated client: %v", err)
		}
		transportService := NewService(client)
		if _, err := transportService.ListRepositoryWebhooks(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}); err == nil || apperrors.ExitCode(err) != 10 {
			t.Fatalf("expected transient transport error, got: %v", err)
		}
	})

	t.Run("permission and webhook validations", func(t *testing.T) {
		service := newServiceWithBaseURL(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
		})

		if err := service.GrantRepositoryUserPermission(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, " ", "REPO_READ"); err == nil {
			t.Fatal("expected username validation error")
		}
		if _, err := service.CreateRepositoryWebhook(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, WebhookCreateInput{Name: "", URL: "http://example.local"}); err == nil {
			t.Fatal("expected webhook name validation error")
		}
		if _, err := service.CreateRepositoryWebhook(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, WebhookCreateInput{Name: "ci", URL: ""}); err == nil {
			t.Fatal("expected webhook url validation error")
		}
	})

	t.Run("pull request settings decode and status branches", func(t *testing.T) {
		decodeService := newServiceWithBaseURL(t, func(writer http.ResponseWriter, request *http.Request) {
			switch {
			case request.Method == http.MethodGet && request.URL.Path == "/api/latest/projects/PRJ/repos/demo/settings/pull-requests":
				_, _ = writer.Write([]byte(`[]`))
			case request.Method == http.MethodPost && request.URL.Path == "/api/latest/projects/PRJ/repos/demo/settings/pull-requests":
				_, _ = writer.Write([]byte(`[]`))
			default:
				http.NotFound(writer, request)
			}
		})

		if _, err := decodeService.GetRepositoryPullRequestSettings(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}); err == nil {
			t.Fatal("expected decode error for pull request settings map")
		}
		if _, err := decodeService.UpdateRepositoryPullRequestRequiredAllTasks(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, true); err == nil {
			t.Fatal("expected decode error for all tasks update map")
		}
		if _, err := decodeService.UpdateRepositoryPullRequestRequiredApproversCount(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}, 2); err == nil {
			t.Fatal("expected decode error for approvers update map")
		}

		statusService := newServiceWithBaseURL(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte("unauthorized"))
		})
		if _, err := statusService.GetRepositoryPullRequestSettings(context.Background(), RepositoryRef{ProjectKey: "PRJ", Slug: "demo"}); err == nil || apperrors.ExitCode(err) != 3 {
			t.Fatalf("expected auth mapping, got: %v", err)
		}
	})

	t.Run("validate repository ref branch", func(t *testing.T) {
		service := newServiceWithBaseURL(t, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})
		if _, err := service.ListRepositoryPermissionUsers(context.Background(), RepositoryRef{}, 10); err == nil {
			t.Fatal("expected repository validation error")
		}
	})
}
