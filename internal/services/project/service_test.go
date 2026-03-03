package project

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func newProjectTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return NewService(client)
}

func TestProjectServiceCoreCommands(t *testing.T) {
	service := newProjectTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"key":"PRJ","name":"Project"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ":
			_, _ = w.Write([]byte(`{"key":"PRJ","name":"Project"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/latest/projects":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key":"PRJ2","name":"Project 2"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/latest/projects/PRJ":
			_, _ = w.Write([]byte(`{"key":"PRJ","name":"Project Updated"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/api/latest/projects/PRJ":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	list, err := service.List(context.Background(), ListOptions{Limit: 25, Name: "Project"})
	if err != nil || len(list) != 1 {
		t.Fatalf("expected list success, len=%d err=%v", len(list), err)
	}

	get, err := service.Get(context.Background(), "PRJ")
	if err != nil || get.Key == nil || *get.Key != "PRJ" {
		t.Fatalf("expected get success, got %#v err=%v", get, err)
	}

	created, err := service.Create(context.Background(), CreateInput{Key: "PRJ2", Name: "Project 2", Description: "desc"})
	if err != nil || created.Key == nil || *created.Key != "PRJ2" {
		t.Fatalf("expected create success, got %#v err=%v", created, err)
	}

	updated, err := service.Update(context.Background(), "PRJ", UpdateInput{Name: "Project Updated", Description: "desc"})
	if err != nil || updated.Name == nil || *updated.Name != "Project Updated" {
		t.Fatalf("expected update success, got %#v err=%v", updated, err)
	}

	if err := service.Delete(context.Background(), "PRJ"); err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}
}

func TestProjectServiceValidation(t *testing.T) {
	service := newProjectTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	})

	if _, err := service.Get(context.Background(), ""); err == nil {
		t.Fatal("expected get key validation error")
	}

	if _, err := service.Create(context.Background(), CreateInput{Key: "", Name: "abc"}); err == nil {
		t.Fatal("expected create key validation error")
	}
	if _, err := service.Create(context.Background(), CreateInput{Key: "abc", Name: ""}); err == nil {
		t.Fatal("expected create name validation error")
	}

	if _, err := service.Update(context.Background(), "", UpdateInput{Name: "abc"}); err == nil {
		t.Fatal("expected update key validation error")
	}

	if err := service.Delete(context.Background(), ""); err == nil {
		t.Fatal("expected delete key validation error")
	}

	if _, err := service.List(context.Background(), ListOptions{}); err == nil || !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("expected mapped authorization error, got %v", err)
	}
}

func TestProjectServicePagination(t *testing.T) {
	calls := 0
	service := newProjectTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"isLastPage":false,"nextPageStart":1,"values":[{"key":"PRJ1"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"key":"PRJ2"}]}`))
	})

	projects, err := service.List(context.Background(), ListOptions{Limit: 0})
	if err != nil || len(projects) != 2 {
		t.Fatalf("expected paginated list, len=%d err=%v", len(projects), err)
	}
}

func TestProjectServiceTransientAndMapping(t *testing.T) {
	transientService := newProjectTestService(t, func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		connection, _, hijackErr := hijacker.Hijack()
		if hijackErr == nil {
			_ = connection.Close()
		}
	})

	if _, err := transientService.List(context.Background(), ListOptions{}); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected list transient error, got %v", err)
	}
	if _, err := transientService.Get(context.Background(), "PRJ"); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected get transient error, got %v", err)
	}
	if _, err := transientService.Create(context.Background(), CreateInput{Key: "P", Name: "N"}); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected create transient error, got %v", err)
	}
	if _, err := transientService.Update(context.Background(), "PRJ", UpdateInput{}); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected update transient error, got %v", err)
	}
	if err := transientService.Delete(context.Background(), "PRJ"); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected delete transient error, got %v", err)
	}

	service := newProjectTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"isLastPage":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/latest/projects/PRJ":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	})

	list, err := service.List(context.Background(), ListOptions{})
	if err != nil || len(list) != 0 {
		t.Fatalf("expected empty list success, got %v", err)
	}

	if _, err := service.Get(context.Background(), "PRJ"); err == nil || apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found get error, got %v", err)
	}

	if created, err := service.Create(context.Background(), CreateInput{Key: "P", Name: "N"}); err != nil || created.Key != nil {
		t.Fatalf("expected empty create success, got %v", err)
	}

	if updated, err := service.Update(context.Background(), "P", UpdateInput{}); err != nil || updated.Key != nil {
		t.Fatalf("expected empty update success, got %v", err)
	}

	testMapStatusErrors(t)
}

func testMapStatusErrors(t *testing.T) {
	if err := mapStatusError(http.StatusBadRequest, nil); err == nil || apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected validation error")
	}
	if err := mapStatusError(http.StatusUnauthorized, nil); err == nil || apperrors.ExitCode(err) != 3 {
		t.Fatalf("expected auth error")
	}
	if err := mapStatusError(http.StatusNotFound, nil); err == nil || apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found error")
	}
	if err := mapStatusError(http.StatusConflict, nil); err == nil || apperrors.ExitCode(err) != 5 {
		t.Fatalf("expected conflict error")
	}
	if err := mapStatusError(http.StatusTooManyRequests, []byte("rate")); err == nil || apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected transient rate error")
	}
	if err := mapStatusError(http.StatusTeapot, nil); err == nil || apperrors.ExitCode(err) != 1 {
		t.Fatalf("expected permanent error")
	}
}
