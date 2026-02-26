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
