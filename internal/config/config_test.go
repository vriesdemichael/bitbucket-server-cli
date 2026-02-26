package config

import (
	"path/filepath"
	"testing"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "")
	t.Setenv("BITBUCKET_VERSION_TARGET", "")
	t.Setenv("BITBUCKET_PROJECT_KEY", "")

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if config.BitbucketURL != defaultBitbucketURL {
		t.Fatalf("expected default url %q, got %q", defaultBitbucketURL, config.BitbucketURL)
	}
}

func TestLoadFromEnvInvalidURL(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "://broken")
	t.Setenv("BITBUCKET_PROJECT_KEY", "TEST")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFromEnvNormalizesURLAndAliasUsername(t *testing.T) {
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "localhost:7990")
	t.Setenv("BITBUCKET_USER", "admin")
	t.Setenv("BITBUCKET_PASSWORD", "admin")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if config.BitbucketURL != "http://localhost:7990" {
		t.Fatalf("expected normalized URL, got %q", config.BitbucketURL)
	}

	if config.BitbucketUsername != "admin" {
		t.Fatalf("expected username from BITBUCKET_USER alias, got %q", config.BitbucketUsername)
	}
}

func TestSaveLoginAndLoadStoredConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")

	_, err := SaveLogin(LoginInput{
		Host:       "localhost:7990",
		Username:   "admin",
		Password:   "admin",
		SetDefault: true,
	})
	if err != nil {
		t.Fatalf("expected save login to succeed, got: %v", err)
	}

	stored, err := LoadStoredConfig()
	if err != nil {
		t.Fatalf("expected load stored config to succeed, got: %v", err)
	}

	if stored.DefaultHost == "" {
		t.Fatal("expected default host to be set")
	}

	profile, ok := stored.Hosts[stored.DefaultHost]
	if !ok {
		t.Fatal("expected stored host profile")
	}

	if profile.URL != "http://localhost:7990" {
		t.Fatalf("unexpected stored URL: %q", profile.URL)
	}

	if profile.AuthMode != "basic" {
		t.Fatalf("unexpected auth mode: %q", profile.AuthMode)
	}
}
