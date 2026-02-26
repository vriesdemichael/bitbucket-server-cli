package repository

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

func TestListRepositoriesAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/1.0/repos" {
			http.NotFound(w, request)
			return
		}

		start := request.URL.Query().Get("start")
		if start == "" || start == "0" {
			_, _ = fmt.Fprint(w, `{"values":[{"slug":"repo1","name":"Repo One","public":false,"project":{"key":"PRJ"}}],"isLastPage":false,"nextPageStart":1}`)
			return
		}

		_, _ = fmt.Fprint(w, `{"values":[{"slug":"repo2","name":"Repo Two","public":true,"project":{"key":"PRJ"}}],"isLastPage":true,"nextPageStart":2}`)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("BITBUCKET_TOKEN", "")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	client := httpclient.NewFromConfig(cfg)
	service := NewService(client)

	repos, err := service.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}

	if repos[0].Slug != "repo1" || repos[1].Slug != "repo2" {
		t.Fatalf("unexpected repo results: %#v", repos)
	}
}

func TestListRepositoriesAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"errors":[{"message":"Authentication required"}]}`)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	client := httpclient.NewFromConfig(cfg)
	service := NewService(client)

	_, err = service.List(context.Background(), 10)
	if err == nil {
		t.Fatal("expected auth error")
	}

	if apperrors.ExitCode(err) != 3 {
		t.Fatalf("expected auth exit code 3, got %d (%v)", apperrors.ExitCode(err), err)
	}
}

func TestListRepositoriesByProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/1.0/projects/TEST/repos" {
			http.NotFound(w, request)
			return
		}

		_, _ = fmt.Fprint(w, `{"values":[{"slug":"repo1","name":"Repo One","public":false,"project":{"key":"TEST"}}],"isLastPage":true,"nextPageStart":1}`)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	client := httpclient.NewFromConfig(cfg)
	service := NewService(client)

	repos, err := service.ListByProject(context.Background(), "TEST", 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	if repos[0].ProjectKey != "TEST" || repos[0].Slug != "repo1" {
		t.Fatalf("unexpected repo results: %#v", repos)
	}
}
