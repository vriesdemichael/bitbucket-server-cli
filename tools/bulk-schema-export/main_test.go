package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	bulkworkflow "github.com/vriesdemichael/bitbucket-server-cli/internal/workflows/bulk"
)

func TestExportSchemasWritesAllExpectedFiles(t *testing.T) {
	outputDir := t.TempDir()

	if err := exportSchemas(outputDir); err != nil {
		t.Fatalf("exportSchemas failed: %v", err)
	}

	expected := make([]string, 0, len(bulkworkflow.Schemas()))
	for name := range bulkworkflow.Schemas() {
		expected = append(expected, name)
	}
	sort.Strings(expected)

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("readdir failed: %v", err)
	}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d schema files, got %d", len(expected), len(entries))
	}

	for _, name := range expected {
		path := filepath.Join(outputDir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected exported schema %s: %v", name, err)
		}
		if !strings.HasSuffix(string(raw), "\n") {
			t.Fatalf("expected trailing newline in %s", name)
		}
		if !strings.Contains(string(raw), `"$schema": "https://json-schema.org/draft/2020-12/schema"`) {
			t.Fatalf("expected draft schema marker in %s", name)
		}
	}
}

func TestExportSchemasReturnsErrorForInvalidOutputPath(t *testing.T) {
	base := t.TempDir()
	filePath := filepath.Join(base, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup file failed: %v", err)
	}

	err := exportSchemas(filePath)
	if err == nil {
		t.Fatal("expected exportSchemas to fail when output path is a file")
	}
	if !strings.Contains(err.Error(), "create output directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
