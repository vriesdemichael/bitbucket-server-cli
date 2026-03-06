package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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
