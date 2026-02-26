package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

func TestHealthAuthenticated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/1.0/projects" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewFromConfig(config.AppConfig{BitbucketURL: server.URL})
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !health.Healthy || !health.Authenticated {
		t.Fatalf("expected healthy authenticated state, got: %+v", health)
	}
}

func TestHealthUnauthorizedButReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewFromConfig(config.AppConfig{BitbucketURL: server.URL})
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !health.Healthy || health.Authenticated {
		t.Fatalf("expected healthy but unauthenticated state, got: %+v", health)
	}
}

func TestHealthServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewFromConfig(config.AppConfig{BitbucketURL: server.URL})
	_, err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	if apperrors.ExitCode(err) != 10 {
		t.Fatalf("expected transient exit code 10, got %d", apperrors.ExitCode(err))
	}
}
