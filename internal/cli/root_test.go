package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthStatusSmoke(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "http://localhost:7990")
	t.Setenv("BITBUCKET_VERSION_TARGET", "9.4.16")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"auth", "status"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Target Bitbucket") {
		t.Fatalf("expected auth status output, got: %s", buffer.String())
	}

	if !strings.Contains(buffer.String(), "auth=none") {
		t.Fatalf("expected auth mode in output, got: %s", buffer.String())
	}

	if !strings.Contains(buffer.String(), "source=env/default") {
		t.Fatalf("expected auth source in output, got: %s", buffer.String())
	}
}

func TestAuthStatusJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "http://localhost:7990")
	t.Setenv("BITBUCKET_VERSION_TARGET", "9.4.16")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "auth", "status"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid json output, got: %s (%v)", buffer.String(), err)
	}

	if parsed["bitbucket_url"] != "http://localhost:7990" {
		t.Fatalf("unexpected bitbucket_url: %q", parsed["bitbucket_url"])
	}

	if parsed["auth_mode"] != "none" {
		t.Fatalf("unexpected auth_mode: %q", parsed["auth_mode"])
	}

	if parsed["auth_source"] != "env/default" {
		t.Fatalf("unexpected auth_source: %q", parsed["auth_source"])
	}
}

func TestAdminHealthSmoke(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/api/1.0/projects" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"admin", "health"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buffer.String(), "Bitbucket health: OK") {
		t.Fatalf("expected health output, got: %s", buffer.String())
	}
}

func TestAdminHealthJSON(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	command := NewRootCommand()
	buffer := &bytes.Buffer{}
	command.SetOut(buffer)
	command.SetErr(buffer)
	command.SetArgs([]string{"--json", "admin", "health"})

	err := command.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid json output, got: %s (%v)", buffer.String(), err)
	}

	if healthy, ok := parsed["healthy"].(bool); !ok || !healthy {
		t.Fatalf("expected healthy=true, got: %#v", parsed["healthy"])
	}
}
