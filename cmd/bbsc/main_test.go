package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

func TestExecuteRootCommandWithoutDiagnostics(t *testing.T) {
	t.Setenv("BBSC_LOG_LEVEL", "")
	t.Setenv("BBSC_LOG_FORMAT", "")

	cmd := &cobra.Command{Use: "test", RunE: func(command *cobra.Command, args []string) error {
		return apperrors.New(apperrors.KindValidation, "invalid input", nil)
	}}

	buffer := &bytes.Buffer{}
	exitCode := executeRootCommand(cmd, buffer)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	output := strings.TrimSpace(buffer.String())
	if output != "validation: invalid input" {
		t.Fatalf("expected only plain error output, got %q", output)
	}
}

func TestExecuteRootCommandWithDiagnosticsJSONL(t *testing.T) {
	t.Setenv("BBSC_LOG_LEVEL", "error")
	t.Setenv("BBSC_LOG_FORMAT", "jsonl")

	cmd := &cobra.Command{Use: "test", RunE: func(command *cobra.Command, args []string) error {
		return apperrors.New(apperrors.KindValidation, "invalid input", nil)
	}}

	buffer := &bytes.Buffer{}
	exitCode := executeRootCommand(cmd, buffer)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected diagnostics + error line, got %q", buffer.String())
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("expected first line to be json diagnostics, got %q (%v)", lines[0], err)
	}

	if payload["level"] != "error" {
		t.Fatalf("expected diagnostics level error, got %v", payload["level"])
	}
	if payload["error_kind"] != "validation" {
		t.Fatalf("expected diagnostics error_kind validation, got %v", payload["error_kind"])
	}
	if _, ok := payload["correlation_id"].(string); !ok {
		t.Fatalf("expected correlation_id string, got %T", payload["correlation_id"])
	}

	if lines[len(lines)-1] != "validation: invalid input" {
		t.Fatalf("expected final user-facing error line, got %q", lines[len(lines)-1])
	}
}

func TestKindFallbackForPlainErrors(t *testing.T) {
	t.Setenv("BBSC_LOG_LEVEL", "error")
	t.Setenv("BBSC_LOG_FORMAT", "jsonl")

	cmd := &cobra.Command{Use: "test", RunE: func(command *cobra.Command, args []string) error {
		return errors.New("plain failure")
	}}

	buffer := &bytes.Buffer{}
	exitCode := executeRootCommand(cmd, buffer)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("expected first line json diagnostics, got %q (%v)", lines[0], err)
	}

	if payload["error_kind"] != "internal" {
		t.Fatalf("expected internal kind fallback, got %v", payload["error_kind"])
	}
}

func TestExecuteRootCommandSuccess(t *testing.T) {
	t.Setenv("BBSC_LOG_LEVEL", "")
	t.Setenv("BBSC_LOG_FORMAT", "")

	cmd := &cobra.Command{Use: "test", RunE: func(command *cobra.Command, args []string) error {
		return nil
	}}

	buffer := &bytes.Buffer{}
	exitCode := executeRootCommand(cmd, buffer)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if buffer.String() != "" {
		t.Fatalf("expected no stderr output on success, got %q", buffer.String())
	}
}

func TestLoggerFromEnvironmentBranches(t *testing.T) {
	t.Run("partial config uses defaults", func(t *testing.T) {
		t.Setenv("BBSC_LOG_LEVEL", "")
		t.Setenv("BBSC_LOG_FORMAT", "jsonl")

		buffer := &bytes.Buffer{}
		logger, enabled := loggerFromEnvironment(buffer)
		if !enabled {
			t.Fatal("expected logger to be enabled")
		}

		logger.Error("test", map[string]any{"k": "v"})
		if !strings.Contains(buffer.String(), "\"level\":\"error\"") {
			t.Fatalf("expected default error level in json output, got %q", buffer.String())
		}
	})

	t.Run("invalid level disables logger", func(t *testing.T) {
		t.Setenv("BBSC_LOG_LEVEL", "trace")
		t.Setenv("BBSC_LOG_FORMAT", "jsonl")

		buffer := &bytes.Buffer{}
		logger, enabled := loggerFromEnvironment(buffer)
		if enabled {
			t.Fatal("expected logger to be disabled for invalid level")
		}

		logger.Error("ignored", map[string]any{"k": "v"})
		if buffer.String() != "" {
			t.Fatalf("expected no output from disabled logger, got %q", buffer.String())
		}
	})

	t.Run("invalid format disables logger", func(t *testing.T) {
		t.Setenv("BBSC_LOG_LEVEL", "error")
		t.Setenv("BBSC_LOG_FORMAT", "yaml")

		buffer := &bytes.Buffer{}
		_, enabled := loggerFromEnvironment(buffer)
		if enabled {
			t.Fatal("expected logger to be disabled for invalid format")
		}
	})
}
