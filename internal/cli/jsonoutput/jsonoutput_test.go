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
	if !strings.Contains(output, "\"version\": \"v2\"") {
		t.Fatalf("expected version field in output, got %s", output)
	}
	if !strings.Contains(output, "\"contract\": \"bb.machine\"") {
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

func TestEnvelopeSchemaFor(t *testing.T) {
	dataSchema := map[string]any{"type": "string"}
	schema := EnvelopeSchemaFor("test.schema.json", "Test Title", "Test description", dataSchema)

	if schema["$schema"] != jsonSchemaVersion {
		t.Errorf("expected $schema=%q, got %q", jsonSchemaVersion, schema["$schema"])
	}
	expected := SchemaBaseURL + "test.schema.json"
	if schema["$id"] != expected {
		t.Errorf("expected $id=%q, got %q", expected, schema["$id"])
	}
	if schema["title"] != "Test Title" {
		t.Errorf("unexpected title: %v", schema["title"])
	}
	if schema["description"] != "Test description" {
		t.Errorf("unexpected description: %v", schema["description"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	for _, field := range []string{"version", "data", "meta"} {
		if _, ok := props[field]; !ok {
			t.Errorf("missing envelope property %q", field)
		}
	}
	if props["data"] == nil {
		t.Error("expected data property to be set")
	}

	req, ok := schema["required"].([]any)
	if !ok || len(req) != 3 {
		t.Fatalf("expected required=[version,data,meta], got %v", schema["required"])
	}
}
