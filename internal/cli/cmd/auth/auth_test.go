package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli/jsonoutput"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type fakeUsersClient struct {
	response *openapigenerated.GetUsers2Response
	err      error
}

func (client *fakeUsersClient) GetUsers2WithResponse(ctx context.Context, params *openapigenerated.GetUsers2Params, reqEditors ...openapigenerated.RequestEditorFn) (*openapigenerated.GetUsers2Response, error) {
	if client.err != nil {
		return nil, client.err
	}
	return client.response, nil
}

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
			return jsonoutput.Write(writer, payload)
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
	if err := decodeJSONEnvelopeData(buffer.Bytes(), &parsed); err != nil {
		t.Fatalf("expected json output, got %q (%v)", buffer.String(), err)
	}

	if parsed["bitbucket_url"] != "http://override.example" {
		t.Fatalf("expected overridden bitbucket_url, got %q", parsed["bitbucket_url"])
	}
	if parsed["auth_mode"] != "none" {
		t.Fatalf("expected auth_mode none, got %q", parsed["auth_mode"])
	}
}

func decodeJSONEnvelopeData(raw []byte, target any) error {
	var envelope struct {
		Version string `json:"version"`
		Data    any    `json:"data"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}

	if strings.TrimSpace(envelope.Version) == "" {
		return os.ErrInvalid
	}

	if envelope.Data == nil {
		return os.ErrInvalid
	}

	encodedData, err := json.Marshal(envelope.Data)
	if err != nil {
		return err
	}

	return json.Unmarshal(encodedData, target)
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
	configPath := filepath.Join(t.TempDir(), "bb", "config.yaml")
	t.Setenv("BB_CONFIG_PATH", configPath)
	t.Setenv("BB_DISABLE_STORED_CONFIG", "")

	t.Run("login with positional host arg stores credentials", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://resolved.local:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
		})

		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"login", "http://resolved.local:7990", "--token", "abc", "--set-default=true"})
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
				return jsonoutput.Write(writer, payload)
			},
		})

		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{"logout", "--host", "http://logout.local:7990"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("logout json failed: %v", err)
		}
		if !strings.Contains(out.String(), "\"status\": \"ok\"") {
			t.Fatalf("expected json ok status, got: %s", out.String())
		}
	})

	t.Run("default dependency handlers return configured errors", func(t *testing.T) {
		cmd := New(Dependencies{JSONEnabled: func() bool { return true }})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"status"})
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
	configPath := filepath.Join(t.TempDir(), "bb", "config.yaml")
	t.Setenv("BB_CONFIG_PATH", configPath)
	t.Setenv("BB_DISABLE_STORED_CONFIG", "")

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
			return jsonoutput.Write(writer, payload)
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
			return jsonoutput.Write(writer, payload)
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
	configPath := filepath.Join(t.TempDir(), "bb", "config.yaml")
	t.Setenv("BB_CONFIG_PATH", configPath)
	t.Setenv("BB_DISABLE_STORED_CONFIG", "")

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			return jsonoutput.Write(writer, payload)
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
			return jsonoutput.Write(writer, payload)
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
			return jsonoutput.Write(writer, payload)
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
	t.Setenv("BB_CONFIG_PATH", configPath)
	t.Setenv("BB_DISABLE_STORED_CONFIG", "")

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			return jsonoutput.Write(writer, payload)
		},
	})

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"server", "list"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when config path points to directory")
	}

	configPath = filepath.Join(t.TempDir(), "bb", "config.yaml")
	t.Setenv("BB_CONFIG_PATH", configPath)
	if _, err := config.SaveLogin(config.LoginInput{Host: "http://example.local:7990", Token: "t", SetDefault: true}); err != nil {
		t.Fatalf("save login: %v", err)
	}

	cmd = New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			return jsonoutput.Write(writer, payload)
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
			return jsonoutput.Write(writer, payload)
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
	configPath := filepath.Join(t.TempDir(), "bb", "config.yaml")
	t.Setenv("BB_CONFIG_PATH", configPath)
	t.Setenv("BB_DISABLE_STORED_CONFIG", "")

	if _, err := config.SaveLogin(config.LoginInput{Host: "http://basic.local:7990", Username: "alice", Password: "secret", SetDefault: true}); err != nil {
		t.Fatalf("save basic login: %v", err)
	}

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.LoadFromEnv()
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			return jsonoutput.Write(writer, payload)
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

func TestTokenURLCommand(t *testing.T) {
	t.Run("human output with explicit host", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
		})

		buffer := &bytes.Buffer{}
		cmd.SetOut(buffer)
		cmd.SetErr(buffer)
		cmd.SetArgs([]string{"token-url", "--host", "https://bitbucket.acme.corp"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("token-url command failed: %v", err)
		}
		if !strings.Contains(buffer.String(), "https://bitbucket.acme.corp/plugins/servlet/access-tokens/manage") {
			t.Fatalf("expected PAT URL in output, got: %s", buffer.String())
		}
	})

	t.Run("json output with config host fallback", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return true },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://localhost:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
		})

		buffer := &bytes.Buffer{}
		cmd.SetOut(buffer)
		cmd.SetErr(buffer)
		cmd.SetArgs([]string{"token-url"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("token-url json command failed: %v", err)
		}
		if !strings.Contains(buffer.String(), `"token_url": "http://localhost:7990/plugins/servlet/access-tokens/manage"`) {
			t.Fatalf("expected token_url in json output, got: %s", buffer.String())
		}
	})

	t.Run("invalid host returns validation error", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
		})

		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"token-url", "--host", "://bad"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected validation error for invalid host")
		}
	})
}

func TestIdentityCommand(t *testing.T) {
	t.Run("human output", func(t *testing.T) {
		displayName := "Automation User"
		email := "automation@example.local"
		name := "svc-automation"
		slug := "svc-automation"
		id := int32(42)
		userType := openapigenerated.RestApplicationUserTypeNORMAL
		active := true

		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://example.local:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
			NewUsersClient: func(cfg config.AppConfig) (usersClient, error) {
				return &fakeUsersClient{response: &openapigenerated.GetUsers2Response{
					HTTPResponse: &http.Response{StatusCode: 200},
					ApplicationjsonCharsetUTF8200: &openapigenerated.RestApplicationUser{
						DisplayName:  &displayName,
						EmailAddress: &email,
						Name:         &name,
						Slug:         &slug,
						Id:           &id,
						Type:         &userType,
						Active:       &active,
					},
				}}, nil
			},
		})

		buffer := &bytes.Buffer{}
		cmd.SetOut(buffer)
		cmd.SetErr(buffer)
		cmd.SetArgs([]string{"identity"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("identity command failed: %v", err)
		}
		if !strings.Contains(buffer.String(), "Automation User") || !strings.Contains(buffer.String(), "name=svc-automation") {
			t.Fatalf("expected identity summary output, got: %s", buffer.String())
		}
	})

	t.Run("json output", func(t *testing.T) {
		name := "svc-bot"
		slug := "svc-bot"
		active := true

		cmd := New(Dependencies{
			JSONEnabled: func() bool { return true },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://example.local:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
			NewUsersClient: func(cfg config.AppConfig) (usersClient, error) {
				return &fakeUsersClient{response: &openapigenerated.GetUsers2Response{
					HTTPResponse:                  &http.Response{StatusCode: 200},
					ApplicationjsonCharsetUTF8200: &openapigenerated.RestApplicationUser{Name: &name, Slug: &slug, Active: &active},
				}}, nil
			},
		})

		buffer := &bytes.Buffer{}
		cmd.SetOut(buffer)
		cmd.SetErr(buffer)
		cmd.SetArgs([]string{"identity"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("identity json command failed: %v", err)
		}
		if !strings.Contains(buffer.String(), `"slug": "svc-bot"`) {
			t.Fatalf("expected identity slug in json output, got: %s", buffer.String())
		}
	})

	t.Run("api failure surfaces error", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://example.local:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
			NewUsersClient: func(cfg config.AppConfig) (usersClient, error) {
				return &fakeUsersClient{err: errors.New("boom")}, nil
			},
		})

		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"whoami"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected identity lookup error")
		}
	})

	t.Run("host override setenv failure returns error", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://example.local:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
			NewUsersClient: func(cfg config.AppConfig) (usersClient, error) {
				return &fakeUsersClient{}, nil
			},
		})

		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"identity", "--host", string([]byte{'a', 0, 'b'})})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected host override validation error")
		}
	})

	t.Run("identity status error is mapped", func(t *testing.T) {
		cmd := New(Dependencies{
			JSONEnabled: func() bool { return false },
			LoadConfig: func() (config.AppConfig, error) {
				return config.AppConfig{BitbucketURL: "http://example.local:7990"}, nil
			},
			WriteJSON: func(writer io.Writer, payload any) error {
				return jsonoutput.Write(writer, payload)
			},
			NewUsersClient: func(cfg config.AppConfig) (usersClient, error) {
				return &fakeUsersClient{response: &openapigenerated.GetUsers2Response{HTTPResponse: &http.Response{StatusCode: 401}, Body: []byte("unauthorized")}}, nil
			},
		})

		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"identity"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected mapped auth error")
		}
		if apperrors.ExitCode(err) != 3 {
			t.Fatalf("expected auth error exit code 3, got %d (%v)", apperrors.ExitCode(err), err)
		}
	})

	t.Run("human summary fallback unknown", func(t *testing.T) {
		if got := identityHumanSummary(authIdentity{}); !strings.Contains(got, "unknown") {
			t.Fatalf("expected unknown fallback, got %q", got)
		}
	})
}
func TestPersonalAccessTokenURL(t *testing.T) {
	t.Run("generic URL without user slug", func(t *testing.T) {
		got, err := personalAccessTokenURL("https://bitbucket.corp", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "https://bitbucket.corp/plugins/servlet/access-tokens/manage" {
			t.Fatalf("unexpected URL: %q", got)
		}
	})

	t.Run("per-user URL with slug", func(t *testing.T) {
		got, err := personalAccessTokenURL("https://bitbucket.corp", "alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "https://bitbucket.corp/plugins/servlet/access-tokens/users/alice/manage" {
			t.Fatalf("unexpected URL: %q", got)
		}
	})

	t.Run("empty host returns validation error", func(t *testing.T) {
		_, err := personalAccessTokenURL("", "alice")
		if err == nil {
			t.Fatal("expected validation error for empty host")
		}
	})
}

func TestTokenURLCommandWithUserSlug(t *testing.T) {
	slug := "alice"
	respondWith := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"alice","slug":%q,"displayName":"Alice","id":1,"type":"NORMAL","active":true}`, slug)
	}
	server := httptest.NewServer(http.HandlerFunc(respondWith))
	defer server.Close()

	cmd := New(Dependencies{
		JSONEnabled: func() bool { return false },
		LoadConfig: func() (config.AppConfig, error) {
			return config.AppConfig{BitbucketURL: server.URL, BitbucketToken: "tok"}, nil
		},
		WriteJSON: func(writer io.Writer, payload any) error {
			return jsonoutput.Write(writer, payload)
		},
		NewUsersClient: func(cfg config.AppConfig) (usersClient, error) {
			return openapi.NewClientWithResponsesFromConfig(cfg)
		},
	})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"token-url"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("token-url with slug failed: %v", err)
	}

	got := buf.String()
	expected := fmt.Sprintf("/plugins/servlet/access-tokens/users/%s/manage", slug)
	if !strings.Contains(got, expected) {
		t.Fatalf("expected per-user PAT URL in output, got: %q", got)
	}
}