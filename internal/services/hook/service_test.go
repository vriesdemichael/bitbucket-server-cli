package hook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func TestHookService(t *testing.T) {
	var projectHookCount atomic.Int32
	var repoHookCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/settings/hooks":
			if projectHookCount.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"values":[{"enabled":true,"details":{"key":"hook1","name":"Hook 1"}}],"isLastPage":false,"nextPageStart":1}`))
			} else {
				_, _ = w.Write([]byte(`{"values":[],"isLastPage":true}`))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/demo/settings/hooks":
			if repoHookCount.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"values":[{"enabled":false,"details":{"key":"hook2","name":"Hook 2"}}],"isLastPage":false,"nextPageStart":1}`))
			} else {
				_, _ = w.Write([]byte(`{"values":[],"isLastPage":true}`))
			}
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/PRJ/settings/hooks/hook1/enabled":
			_, _ = w.Write([]byte(`{"enabled":true}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/api/latest/projects/PRJ/settings/hooks/hook1/enabled":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/demo/settings/hooks/hook2/enabled":
			_, _ = w.Write([]byte(`{"enabled":true}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/demo/settings/hooks/hook2/enabled":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/settings/hooks/hook1/settings":
			_, _ = w.Write([]byte(`{"foo":"bar"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/PRJ/settings/hooks/hook1/settings":
			_, _ = w.Write([]byte(`{"foo":"bar"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/demo/settings/hooks/hook2/settings":
			_, _ = w.Write([]byte(`{"baz":123}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/PRJ/repos/demo/settings/hooks/hook2/settings":
			_, _ = w.Write([]byte(`{"baz":123}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	service := NewService(client)

	projHooks, err := service.ListProjectHooks(context.Background(), "PRJ", 1)
	if err != nil || len(projHooks) != 1 {
		t.Fatalf("list project hooks failed: %v", err)
	}

	repoHooks, err := service.ListRepositoryHooks(context.Background(), "PRJ", "demo", 1)
	if err != nil || len(repoHooks) != 1 {
		t.Fatalf("list repository hooks failed: %v", err)
	}

	if _, err := service.EnableProjectHook(context.Background(), "PRJ", "hook1"); err != nil {
		t.Fatalf("enable project hook failed: %v", err)
	}

	if err := service.DisableProjectHook(context.Background(), "PRJ", "hook1"); err != nil {
		t.Fatalf("disable project hook failed: %v", err)
	}

	if _, err := service.EnableRepositoryHook(context.Background(), "PRJ", "demo", "hook2"); err != nil {
		t.Fatalf("enable repository hook failed: %v", err)
	}

	if err := service.DisableRepositoryHook(context.Background(), "PRJ", "demo", "hook2"); err != nil {
		t.Fatalf("disable repository hook failed: %v", err)
	}

	if _, err := service.GetProjectHookSettings(context.Background(), "PRJ", "hook1"); err != nil {
		t.Fatalf("get project hook settings failed: %v", err)
	}

	if _, err := service.SetProjectHookSettings(context.Background(), "PRJ", "hook1", map[string]any{"foo": "bar"}); err != nil {
		t.Fatalf("set project hook settings failed: %v", err)
	}

	if _, err := service.GetRepositoryHookSettings(context.Background(), "PRJ", "demo", "hook2"); err != nil {
		t.Fatalf("get repository hook settings failed: %v", err)
	}

	if _, err := service.SetRepositoryHookSettings(context.Background(), "PRJ", "demo", "hook2", map[string]any{"baz": 123}); err != nil {
		t.Fatalf("set repository hook settings failed: %v", err)
	}
}

func TestHookServiceNetworkErrors(t *testing.T) {
	client, _ := openapigenerated.NewClientWithResponses("http://invalid-url-that-does-not-exist.local")
	service := NewService(client)
	ctx := context.Background()

	_, _ = service.ListProjectHooks(ctx, "P", 100)
	_, _ = service.ListRepositoryHooks(ctx, "P", "S", 100)
	_, _ = service.EnableProjectHook(ctx, "P", "h1")
	_ = service.DisableProjectHook(ctx, "P", "h1")
	_, _ = service.EnableRepositoryHook(ctx, "P", "S", "h1")
	_ = service.DisableRepositoryHook(ctx, "P", "S", "h1")
	_, _ = service.GetProjectHookSettings(ctx, "P", "h1")
	_, _ = service.GetRepositoryHookSettings(ctx, "P", "S", "h1")
	_, _ = service.SetProjectHookSettings(ctx, "P", "h1", map[string]any{"a": 1})
	_, _ = service.SetRepositoryHookSettings(ctx, "P", "S", "h1", map[string]any{"a": 1})
}
func TestHookServiceValidation(t *testing.T) {
	service := NewService(nil)
	ctx := context.Background()
	if _, err := service.ListProjectHooks(ctx, "", 100); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.ListRepositoryHooks(ctx, "", "", 100); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.EnableProjectHook(ctx, "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := service.DisableProjectHook(ctx, "", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.EnableRepositoryHook(ctx, "", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := service.DisableRepositoryHook(ctx, "", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.GetProjectHookSettings(ctx, "", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.GetRepositoryHookSettings(ctx, "", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.SetProjectHookSettings(ctx, "", "", nil); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.SetRepositoryHookSettings(ctx, "", "", "", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestHookServiceErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	service := NewService(client)
	ctx := context.Background()

	if _, err := service.ListProjectHooks(ctx, "P", 100); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.EnableProjectHook(ctx, "P", "h1"); err == nil {
		t.Fatal("expected error")
	}
	if err := service.DisableProjectHook(ctx, "P", "h1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.GetProjectHookSettings(ctx, "P", "h1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.SetProjectHookSettings(ctx, "P", "h1", map[string]any{"a": 1}); err == nil {
		t.Fatal("expected error")
	}

	if _, err := service.ListRepositoryHooks(ctx, "P", "S", 100); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.EnableRepositoryHook(ctx, "P", "S", "h1"); err == nil {
		t.Fatal("expected error")
	}
	if err := service.DisableRepositoryHook(ctx, "P", "S", "h1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.GetRepositoryHookSettings(ctx, "P", "S", "h1"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.SetRepositoryHookSettings(ctx, "P", "S", "h1", map[string]any{"a": 1}); err == nil {
		t.Fatal("expected error")
	}
}
