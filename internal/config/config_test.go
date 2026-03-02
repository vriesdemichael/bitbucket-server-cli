package config

import (
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
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

func TestConfigAuthModeAndLogoutBranches(t *testing.T) {
	if (AppConfig{}).AuthMode() != "none" {
		t.Fatal("expected auth mode none")
	}
	if (AppConfig{BitbucketToken: "t"}).AuthMode() != "token" {
		t.Fatal("expected auth mode token")
	}
	if (AppConfig{BitbucketUsername: "u", BitbucketPassword: "p"}).AuthMode() != "basic" {
		t.Fatal("expected auth mode basic")
	}

	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	if err := Logout(""); err == nil || apperrors.ExitCode(err) != 4 {
		t.Fatalf("expected not found logout error when no stored host, got: %v", err)
	}
}

func TestResolveStoredCredentialsAndLoadFromStoredHost(t *testing.T) {
	if _, ok := resolveStoredCredentials(StoredConfig{}, "http://localhost:7990"); ok {
		t.Fatal("expected not found when stored config is empty")
	}

	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")
	t.Setenv("BITBUCKET_URL", "")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_USER", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	stored := StoredConfig{
		DefaultHost: "http://stored.local:7990",
		Hosts: map[string]StoredProfile{
			"http://stored.local:7990": {URL: "http://stored.local:7990", Username: "stored-user", AuthMode: "basic"},
		},
		InsecureSecrets: map[string]StoredSecret{
			"http://stored.local:7990": {Password: "stored-pass"},
		},
	}
	if err := SaveStoredConfig(stored); err != nil {
		t.Fatalf("save stored config: %v", err)
	}

	resolved, ok := resolveStoredCredentials(stored, "http://unknown.local:7990")
	if !ok {
		t.Fatal("expected stored credentials via default host")
	}
	if resolved.BitbucketURL != "http://stored.local:7990" || resolved.BitbucketUsername != "stored-user" || resolved.BitbucketPassword != "stored-pass" {
		t.Fatalf("unexpected resolved stored credentials: %+v", resolved)
	}

	loaded, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("load from env with stored host failed: %v", err)
	}
	if loaded.BitbucketURL != "http://stored.local:7990" {
		t.Fatalf("expected stored default host URL, got %q", loaded.BitbucketURL)
	}
	if loaded.AuthSource != "stored" {
		t.Fatalf("expected auth source stored, got %q", loaded.AuthSource)
	}
}

func TestSaveLoginValidationBranches(t *testing.T) {
	if _, err := SaveLogin(LoginInput{}); err == nil {
		t.Fatal("expected validation error for missing host")
	}
	if _, err := SaveLogin(LoginInput{Host: "localhost:7990", Token: "t", Username: "u", Password: "p"}); err == nil {
		t.Fatal("expected mutually exclusive auth input validation error")
	}
	if _, err := SaveLogin(LoginInput{Host: "localhost:7990", Username: "u"}); err == nil {
		t.Fatal("expected username/password pair validation error")
	}
}

func TestLoadFromEnvEnvSourceOverridesStored(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")
	t.Setenv("BITBUCKET_URL", "http://stored.local:7990")
	t.Setenv("BITBUCKET_USERNAME", "env-user")
	t.Setenv("BITBUCKET_PASSWORD", "env-pass")

	stored := StoredConfig{
		DefaultHost: "http://stored.local:7990",
		Hosts: map[string]StoredProfile{
			"http://stored.local:7990": {URL: "http://stored.local:7990", Username: "stored-user", AuthMode: "basic"},
		},
		InsecureSecrets: map[string]StoredSecret{
			"http://stored.local:7990": {Password: "stored-pass"},
		},
	}
	if err := SaveStoredConfig(stored); err != nil {
		t.Fatalf("save stored config: %v", err)
	}

	loaded, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("load from env failed: %v", err)
	}
	if loaded.AuthSource != "env" {
		t.Fatalf("expected env auth source override, got %q", loaded.AuthSource)
	}
	if loaded.BitbucketUsername != "env-user" {
		t.Fatalf("expected env username to win, got %q", loaded.BitbucketUsername)
	}
}

func TestLogoutExplicitHostRemovesProfile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)

	stored := StoredConfig{
		DefaultHost: "http://one.local:7990",
		Hosts: map[string]StoredProfile{
			"http://one.local:7990": {URL: "http://one.local:7990", AuthMode: "token"},
			"http://two.local:7990": {URL: "http://two.local:7990", AuthMode: "basic", Username: "admin"},
		},
		InsecureSecrets: map[string]StoredSecret{
			"http://one.local:7990": {Token: "t"},
			"http://two.local:7990": {Password: "p"},
		},
	}
	if err := SaveStoredConfig(stored); err != nil {
		t.Fatalf("save stored config: %v", err)
	}

	if err := Logout("http://one.local:7990"); err != nil {
		t.Fatalf("logout explicit host failed: %v", err)
	}

	after, err := LoadStoredConfig()
	if err != nil {
		t.Fatalf("load stored config: %v", err)
	}
	if _, ok := after.Hosts["http://one.local:7990"]; ok {
		t.Fatal("expected logged out host removed")
	}
	if after.DefaultHost != "http://two.local:7990" {
		t.Fatalf("expected default host rotated to remaining profile, got %q", after.DefaultHost)
	}
}

func TestSaveLoginTokenAndMapInitialization(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write empty config: %v", err)
	}

	result, err := SaveLogin(LoginInput{Host: "stored.local:7990", Token: "token-1", SetDefault: false})
	if err != nil {
		t.Fatalf("save token login failed: %v", err)
	}
	if result.AuthMode != "token" {
		t.Fatalf("expected token auth mode, got %q", result.AuthMode)
	}

	stored, err := LoadStoredConfig()
	if err != nil {
		t.Fatalf("load stored config: %v", err)
	}
	if stored.DefaultHost == "" {
		t.Fatal("expected default host to be set when config had none")
	}
	if stored.Hosts[stored.DefaultHost].AuthMode != "token" {
		t.Fatalf("expected stored token auth mode, got %q", stored.Hosts[stored.DefaultHost].AuthMode)
	}
}

func TestLoadStoredConfigInvalidYAML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(": invalid yaml"), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	if _, err := LoadStoredConfig(); err == nil {
		t.Fatal("expected yaml decode error")
	}
}

func TestValidateAndHostKeyBranches(t *testing.T) {
	if err := (AppConfig{BitbucketURL: "http://localhost:7990", ProjectKey: ""}).Validate(); err == nil {
		t.Fatal("expected empty project key validation error")
	}

	if err := (AppConfig{BitbucketURL: "http://localhost:7990", ProjectKey: "THIS_PROJECT_KEY_IS_TOO_LONG"}).Validate(); err == nil {
		t.Fatal("expected project key max length validation error")
	}

	if err := (AppConfig{BitbucketURL: "http://localhost:7990", ProjectKey: "TEST", BitbucketUsername: "user"}).Validate(); err == nil {
		t.Fatal("expected username/password pairing validation error")
	}

	if hostKey("://bad") == "" {
		t.Fatal("expected hostKey fallback value for invalid URL")
	}
}

func TestLoadFromEnvUsesStoredTokenBranch(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")
	t.Setenv("BITBUCKET_URL", "http://stored.local:7990")
	t.Setenv("BITBUCKET_TOKEN", "")
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_USER", "")
	t.Setenv("BITBUCKET_PASSWORD", "")
	t.Setenv("ADMIN_USER", "")
	t.Setenv("ADMIN_PASSWORD", "")

	stored := StoredConfig{
		DefaultHost: "http://stored.local:7990",
		Hosts: map[string]StoredProfile{
			"http://stored.local:7990": {URL: "http://stored.local:7990", AuthMode: "token"},
		},
		InsecureSecrets: map[string]StoredSecret{
			"http://stored.local:7990": {Token: "stored-token"},
		},
	}
	if err := SaveStoredConfig(stored); err != nil {
		t.Fatalf("save stored config: %v", err)
	}

	loaded, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("load from env failed: %v", err)
	}
	if loaded.BitbucketToken != "stored-token" {
		t.Fatalf("expected stored token branch to populate token, got %q", loaded.BitbucketToken)
	}
	if loaded.AuthSource != "stored" {
		t.Fatalf("expected stored auth source, got %q", loaded.AuthSource)
	}
}
