package jsonoutput

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

func TestWriteSuccess(t *testing.T) {
	buffer := &bytes.Buffer{}

	err := Write(buffer, map[string]any{"status": "ok"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "\"version\": \"v1\"") {
		t.Fatalf("expected version field in output, got %s", output)
	}
	if !strings.Contains(output, "\"contract\": \"bbsc.machine\"") {
		t.Fatalf("expected contract field in output, got %s", output)
	}
	if !strings.Contains(output, "\"status\": \"ok\"") {
		t.Fatalf("expected payload field in output, got %s", output)
	}
}

func TestWriteMarshalFailure(t *testing.T) {
	err := Write(&bytes.Buffer{}, map[string]any{"invalid": func() {}})
	if err == nil {
		t.Fatal("expected marshal failure")
	}
	if apperrors.KindOf(err) != apperrors.KindInternal {
		t.Fatalf("expected internal kind, got %q", apperrors.KindOf(err))
	}
	if !strings.Contains(err.Error(), "failed to encode JSON output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteWriterFailure(t *testing.T) {
	err := Write(failingWriter{}, map[string]any{"status": "ok"})
	if err == nil {
		t.Fatal("expected write failure")
	}
	if apperrors.KindOf(err) != apperrors.KindInternal {
		t.Fatalf("expected internal kind, got %q", apperrors.KindOf(err))
	}
	if !strings.Contains(err.Error(), "failed to write JSON output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("boom")
}
