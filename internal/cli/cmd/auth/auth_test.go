package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
)

func TestStatusJSONUsesHostOverride(t *testing.T) {
	t.Setenv("BITBUCKET_URL", "http://initial.example")

	var seenHost string
	cmd := New(Dependencies{
		JSONEnabled: func() bool { return true },
		LoadConfig: func() (config.AppConfig, error) {
			seenHost = os.Getenv("BITBUCKET_URL")
			return config.AppConfig{
				BitbucketURL:           seenHost,
				BitbucketVersionTarget: "9.4.16",
				AuthSource:             "env/default",
			}, nil
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	buffer := &bytes.Buffer{}
	cmd.SetOut(buffer)
	cmd.SetErr(buffer)
	cmd.SetArgs([]string{"status", "--host", "http://override.example"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if seenHost != "http://override.example" {
		t.Fatalf("expected host override to be applied, got %q", seenHost)
	}

	var parsed map[string]string
	if err := json.Unmarshal(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected json output, got %q (%v)", buffer.String(), err)
	}

	if parsed["bitbucket_url"] != "http://override.example" {
		t.Fatalf("expected overridden bitbucket_url, got %q", parsed["bitbucket_url"])
	}
	if parsed["auth_mode"] != "none" {
		t.Fatalf("expected auth_mode none, got %q", parsed["auth_mode"])
	}
}

func TestStatusMissingDependenciesReturnError(t *testing.T) {
	cmd := New(Dependencies{})
	buffer := &bytes.Buffer{}
	cmd.SetOut(buffer)
	cmd.SetErr(buffer)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when dependencies are missing")
	}
}

func TestAuthCommandAdditionalBranches(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")

	t.Run("login resolves host from loaded config when host flag missing", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://resolved.local:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				encoded, err := json.Marshal(payload)
				if err != nil {
					return err
				}
				_, err = writer.Write(encoded)
				return err
			},
		})

		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"login", "--token", "abc", "--set-default=true"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("login failed: %v", err)
		}
		if !strings.Contains(out.String(), "resolved.local") {
			t.Fatalf("expected resolved host in output, got: %s", out.String())
		}
	})

	t.Run("logout json path", func(t *testing.T) {
		if _, err := config.SaveLogin(config.LoginInput{Host: "http://logout.local:7990", Token: "tok", SetDefault: true}); err != nil {
			t.Fatalf("save login for logout: %v", err)
		}

		cmd := New(Dependencies{
			JSONEnabled: func() bool { return true },
			LoadConfig: func() (config.AppConfig, error) {
				return config.LoadFromEnv()
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				encoded, err := json.Marshal(payload)
				if err != nil {
					return err
				}
				_, err = writer.Write(encoded)
				return err
			},
		})

		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"logout", "--host", "http://logout.local:7990"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("logout json failed: %v", err)
		}
		if !strings.Contains(out.String(), "\"status\":\"ok\"") {
			t.Fatalf("expected json ok status, got: %s", out.String())
		}
	})

	t.Run("default dependency handlers return configured errors", func(t *testing.T) {
		cmd := New(Dependencies{JSONEnabled: func() bool { return true }})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"login", "--token", "abc"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected dependency error when LoadConfig default is used")
		}

		cmd = New(Dependencies{JSONEnabled: func() bool { return true }, LoadConfig: func() (config.AppConfig, error) {
			return config.AppConfig{BitbucketURL: "http://x.local:7990"}, nil
		}})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"status"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected dependency error when WriteJSON default is used")
		}
	})
}

func TestServerListAndUseCommands(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")

	if _, err := config.SaveLogin(config.LoginInput{Host: "http://one.local:7990", Token: "token-1", SetDefault: true}); err != nil {
		t.Fatalf("save login one: %v", err)
	}
	if _, err := config.SaveLogin(config.LoginInput{Host: "http://two.local:7990", Token: "token-2", SetDefault: false}); err != nil {
		t.Fatalf("save login two: %v", err)
	}

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	listOutput := &bytes.Buffer{}
	cmd.SetOut(listOutput)
	cmd.SetErr(listOutput)
	cmd.SetArgs([]string{"server", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("server list failed: %v", err)
	}

	listText := listOutput.String()
	if !strings.Contains(listText, "* http://one.local:7990") {
		t.Fatalf("expected default host marker in list output, got: %s", listText)
	}
	if !strings.Contains(listText, "http://two.local:7990") {
		t.Fatalf("expected second host in list output, got: %s", listText)
	}

	useOutput := &bytes.Buffer{}
	cmd = New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})
	cmd.SetOut(useOutput)
	cmd.SetErr(useOutput)
	cmd.SetArgs([]string{"server", "use", "--host", "http://two.local:7990"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("server use failed: %v", err)
	}

	if !strings.Contains(useOutput.String(), "Active server set to http://two.local:7990") {
		t.Fatalf("unexpected server use output: %s", useOutput.String())
	}

	stored, err := config.LoadStoredConfig()
	if err != nil {
		t.Fatalf("load stored config: %v", err)
	}
	if stored.DefaultHost != "http://two.local:7990" {
		t.Fatalf("expected default host to switch, got: %q", stored.DefaultHost)
	}
}

func TestServerCommandsJSONAndEmptyStates(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	emptyBuffer := &bytes.Buffer{}
	cmd.SetOut(emptyBuffer)
	cmd.SetErr(emptyBuffer)
	cmd.SetArgs([]string{"server", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("server list empty failed: %v", err)
	}
	if !strings.Contains(emptyBuffer.String(), "No stored server contexts") {
		t.Fatalf("expected empty contexts message, got: %s", emptyBuffer.String())
	}

	if _, err := config.SaveLogin(config.LoginInput{Host: "http://json.local:7990", Token: "token", SetDefault: true}); err != nil {
		t.Fatalf("save login json host: %v", err)
	}

	jsonCmd := New(Dependencies{
		JSONEnabled: func() bool { return true },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	jsonBuffer := &bytes.Buffer{}
	jsonCmd.SetOut(jsonBuffer)
	jsonCmd.SetErr(jsonBuffer)
	jsonCmd.SetArgs([]string{"server", "list"})
	if err := jsonCmd.Execute(); err != nil {
		t.Fatalf("server list json failed: %v", err)
	}
	if !strings.Contains(jsonBuffer.String(), "servers") {
		t.Fatalf("expected servers in json payload, got: %s", jsonBuffer.String())
	}

	useCmd := New(Dependencies{
		JSONEnabled: func() bool { return true },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	useBuffer := &bytes.Buffer{}
	useCmd.SetOut(useBuffer)
	useCmd.SetErr(useBuffer)
	useCmd.SetArgs([]string{"server", "use", "http://json.local:7990"})
	if err := useCmd.Execute(); err != nil {
		t.Fatalf("server use positional failed: %v", err)
	}
	if !strings.Contains(useBuffer.String(), "default_host") {
		t.Fatalf("expected default_host in json output, got: %s", useBuffer.String())
	}
}

func TestServerCommandsErrorBranches(t *testing.T) {
	configPath := t.TempDir()
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"server", "list"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when config path points to directory")
	}

	configPath = filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	if _, err := config.SaveLogin(config.LoginInput{Host: "http://example.local:7990", Token: "t", SetDefault: true}); err != nil {
		t.Fatalf("save login: %v", err)
	}

	cmd = New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"server", "use"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected validation error when host is missing")
	}

	cmd = New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"server", "use", "--host", "http://unknown.local:7990"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected not-found error when selecting unknown host")
	}
}

func TestServerListIncludesUsernameForBasicAuth(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "bbsc", "config.yaml")
	t.Setenv("BBSC_CONFIG_PATH", configPath)
	t.Setenv("BBSC_DISABLE_STORED_CONFIG", "")

	if _, err := config.SaveLogin(config.LoginInput{Host: "http://basic.local:7990", Username: "alice", Password: "secret", SetDefault: true}); err != nil {
		t.Fatalf("save basic login: %v", err)
	}

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = writer.Write(encoded)
			return err
		},
	})

	buffer := &bytes.Buffer{}
	cmd.SetOut(buffer)
	cmd.SetErr(buffer)
	cmd.SetArgs([]string{"server", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("server list failed: %v", err)
	}

	if !strings.Contains(buffer.String(), "user=alice") {
		t.Fatalf("expected username in server list output, got: %s", buffer.String())
	}
}
