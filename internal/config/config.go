package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	defaultBitbucketURL           = "http://localhost:7990"
	defaultBitbucketVersionTarget = "9.4.16"
	defaultProjectKey             = "TEST"
	keyringServiceName            = "bbsc"
)

type AppConfig struct {
	BitbucketURL           string
	BitbucketVersionTarget string
	ProjectKey             string
	BitbucketToken         string
	BitbucketUsername      string
	BitbucketPassword      string
	AuthSource             string
}

type StoredConfig struct {
	DefaultHost     string                   `yaml:"default_host,omitempty"`
	Hosts           map[string]StoredProfile `yaml:"hosts,omitempty"`
	InsecureSecrets map[string]StoredSecret  `yaml:"insecure_secrets,omitempty"`
}

type StoredProfile struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username,omitempty"`
	AuthMode string `yaml:"auth_mode,omitempty"`
}

type StoredSecret struct {
	Token    string `yaml:"token,omitempty"`
	Password string `yaml:"password,omitempty"`
}

type LoginInput struct {
	Host       string
	Username   string
	Password   string
	Token      string
	SetDefault bool
}

type LoginResult struct {
	Host                string
	AuthMode            string
	UsedInsecureStorage bool
}

func LoadFromEnv() (AppConfig, error) {
	_ = godotenv.Load(".env")
	storedConfig, _ := LoadStoredConfig()

	envHost := strings.TrimSpace(os.Getenv("BITBUCKET_URL"))
	resolvedURL := ""
	if envHost != "" {
		resolvedURL = normalizeURL(envHost)
	} else if storedConfig.DefaultHost != "" {
		if profile, ok := storedConfig.Hosts[storedConfig.DefaultHost]; ok {
			resolvedURL = normalizeURL(profile.URL)
		}
	}
	if resolvedURL == "" {
		resolvedURL = defaultBitbucketURL
	}

	config := AppConfig{
		BitbucketURL:           resolvedURL,
		BitbucketVersionTarget: envOrDefault("BITBUCKET_VERSION_TARGET", defaultBitbucketVersionTarget),
		ProjectKey:             envOrDefault("BITBUCKET_PROJECT_KEY", defaultProjectKey),
		BitbucketToken:         envOrDefault("BITBUCKET_TOKEN", ""),
		BitbucketUsername:      envOrDefault("BITBUCKET_USERNAME", envOrDefault("BITBUCKET_USER", envOrDefault("ADMIN_USER", ""))),
		BitbucketPassword:      envOrDefault("BITBUCKET_PASSWORD", envOrDefault("ADMIN_PASSWORD", "")),
		AuthSource:             "env/default",
	}

	if os.Getenv("BBSC_DISABLE_STORED_CONFIG") != "1" {
		stored, foundStored := resolveStoredCredentials(storedConfig, config.BitbucketURL)
		if foundStored {
			if config.BitbucketUsername == "" && stored.BitbucketUsername != "" {
				config.BitbucketUsername = stored.BitbucketUsername
			}
			if config.BitbucketToken == "" && stored.BitbucketToken != "" {
				config.BitbucketToken = stored.BitbucketToken
			}
			if config.BitbucketPassword == "" && stored.BitbucketPassword != "" {
				config.BitbucketPassword = stored.BitbucketPassword
			}
			if config.BitbucketToken != "" || (config.BitbucketUsername != "" && config.BitbucketPassword != "") {
				config.AuthSource = "stored"
			}
		}

		if os.Getenv("BITBUCKET_TOKEN") != "" || os.Getenv("BITBUCKET_USERNAME") != "" || os.Getenv("BITBUCKET_USER") != "" || os.Getenv("BITBUCKET_PASSWORD") != "" || os.Getenv("ADMIN_USER") != "" || os.Getenv("ADMIN_PASSWORD") != "" {
			config.AuthSource = "env"
		}
	}

	if err := config.Validate(); err != nil {
		return AppConfig{}, err
	}

	return config, nil
}

func SaveLogin(input LoginInput) (LoginResult, error) {
	host := normalizeURL(strings.TrimSpace(input.Host))
	if host == "" {
		return LoginResult{}, apperrors.New(apperrors.KindValidation, "host is required", nil)
	}

	hasToken := strings.TrimSpace(input.Token) != ""
	hasBasic := strings.TrimSpace(input.Username) != "" || strings.TrimSpace(input.Password) != ""
	if hasToken == hasBasic {
		return LoginResult{}, apperrors.New(apperrors.KindValidation, "provide either token or username/password", nil)
	}
	if hasBasic && (strings.TrimSpace(input.Username) == "" || strings.TrimSpace(input.Password) == "") {
		return LoginResult{}, apperrors.New(apperrors.KindValidation, "username and password must be provided together", nil)
	}

	stored, _ := LoadStoredConfig()
	if stored.Hosts == nil {
		stored.Hosts = map[string]StoredProfile{}
	}
	if stored.InsecureSecrets == nil {
		stored.InsecureSecrets = map[string]StoredSecret{}
	}

	profile := StoredProfile{URL: host}
	result := LoginResult{Host: host}

	if hasToken {
		profile.AuthMode = "token"
		result.AuthMode = "token"
	} else {
		profile.AuthMode = "basic"
		profile.Username = strings.TrimSpace(input.Username)
		result.AuthMode = "basic"
	}

	key := hostKey(host)
	insecure := StoredSecret{}
	if hasToken {
		if err := keyring.Set(keyringServiceName, key+":token", strings.TrimSpace(input.Token)); err != nil {
			insecure.Token = strings.TrimSpace(input.Token)
			result.UsedInsecureStorage = true
		}
		_ = keyring.Delete(keyringServiceName, key+":password")
	} else {
		if err := keyring.Set(keyringServiceName, key+":password", strings.TrimSpace(input.Password)); err != nil {
			insecure.Password = strings.TrimSpace(input.Password)
			result.UsedInsecureStorage = true
		}
		_ = keyring.Delete(keyringServiceName, key+":token")
	}

	if insecure.Token != "" || insecure.Password != "" {
		stored.InsecureSecrets[key] = insecure
	} else {
		delete(stored.InsecureSecrets, key)
	}

	stored.Hosts[key] = profile
	if input.SetDefault || stored.DefaultHost == "" {
		stored.DefaultHost = key
	}

	if err := SaveStoredConfig(stored); err != nil {
		return LoginResult{}, err
	}

	return result, nil
}

func Logout(host string) error {
	stored, _ := LoadStoredConfig()
	hostURL := normalizeURL(strings.TrimSpace(host))
	if hostURL == "" {
		if stored.DefaultHost == "" {
			return apperrors.New(apperrors.KindNotFound, "no stored host to logout", nil)
		}
		hostURL = stored.DefaultHost
	}

	key := hostKey(hostURL)
	_ = keyring.Delete(keyringServiceName, key+":token")
	_ = keyring.Delete(keyringServiceName, key+":password")

	delete(stored.Hosts, key)
	delete(stored.InsecureSecrets, key)
	if stored.DefaultHost == key {
		stored.DefaultHost = ""
		for next := range stored.Hosts {
			stored.DefaultHost = next
			break
		}
	}

	return SaveStoredConfig(stored)
}

func LoadStoredConfig() (StoredConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return StoredConfig{}, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StoredConfig{Hosts: map[string]StoredProfile{}, InsecureSecrets: map[string]StoredSecret{}}, nil
		}
		return StoredConfig{}, err
	}

	var stored StoredConfig
	if err := yaml.Unmarshal(raw, &stored); err != nil {
		return StoredConfig{}, err
	}
	if stored.Hosts == nil {
		stored.Hosts = map[string]StoredProfile{}
	}
	if stored.InsecureSecrets == nil {
		stored.InsecureSecrets = map[string]StoredSecret{}
	}

	return stored, nil
}

func SaveStoredConfig(stored StoredConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	encoded, err := yaml.Marshal(stored)
	if err != nil {
		return err
	}

	return os.WriteFile(path, encoded, 0o600)
}

func ConfigPath() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("BBSC_CONFIG_PATH")); custom != "" {
		return custom, nil
	}

	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, "bbsc", "config.yaml"), nil
}

func resolveStoredCredentials(stored StoredConfig, runtimeURL string) (AppConfig, bool) {
	if stored.Hosts == nil || len(stored.Hosts) == 0 {
		return AppConfig{}, false
	}

	key := hostKey(runtimeURL)
	profile, ok := stored.Hosts[key]
	if !ok {
		if stored.DefaultHost == "" {
			return AppConfig{}, false
		}
		profile, ok = stored.Hosts[stored.DefaultHost]
		if !ok {
			return AppConfig{}, false
		}
		key = stored.DefaultHost
	}

	resolved := AppConfig{BitbucketURL: normalizeURL(profile.URL), BitbucketUsername: profile.Username}

	if token, err := keyring.Get(keyringServiceName, key+":token"); err == nil && strings.TrimSpace(token) != "" {
		resolved.BitbucketToken = token
	}
	if password, err := keyring.Get(keyringServiceName, key+":password"); err == nil && strings.TrimSpace(password) != "" {
		resolved.BitbucketPassword = password
	}

	if resolved.BitbucketToken == "" || resolved.BitbucketPassword == "" {
		if insecure, ok := stored.InsecureSecrets[key]; ok {
			if resolved.BitbucketToken == "" {
				resolved.BitbucketToken = insecure.Token
			}
			if resolved.BitbucketPassword == "" {
				resolved.BitbucketPassword = insecure.Password
			}
		}
	}

	return resolved, true
}

func (config AppConfig) Validate() error {
	parsedURL, err := url.Parse(config.BitbucketURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return apperrors.New(
			apperrors.KindValidation,
			fmt.Sprintf("BITBUCKET_URL is invalid: %q", config.BitbucketURL),
			err,
		)
	}

	projectKey := strings.TrimSpace(config.ProjectKey)
	if projectKey == "" {
		return apperrors.New(apperrors.KindValidation, "BITBUCKET_PROJECT_KEY cannot be empty", nil)
	}

	if len(projectKey) > 20 {
		return apperrors.New(apperrors.KindValidation, "BITBUCKET_PROJECT_KEY cannot exceed 20 characters", nil)
	}

	if (config.BitbucketUsername == "") != (config.BitbucketPassword == "") {
		return apperrors.New(
			apperrors.KindValidation,
			"BITBUCKET_USERNAME and BITBUCKET_PASSWORD must be set together",
			nil,
		)
	}

	return nil
}

func (config AppConfig) AuthMode() string {
	if config.BitbucketToken != "" {
		return "token"
	}

	if config.BitbucketUsername != "" && config.BitbucketPassword != "" {
		return "basic"
	}

	return "none"
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func normalizeURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}

	if strings.Contains(trimmed, "://") {
		return trimmed
	}

	return "http://" + trimmed
}

func hostKey(hostURL string) string {
	parsed, err := url.Parse(normalizeURL(hostURL))
	if err != nil {
		return normalizeURL(hostURL)
	}

	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}
