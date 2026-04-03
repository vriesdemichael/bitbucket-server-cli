package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli/outputschemas"
)

func TestExportSchemas(t *testing.T) {
	dir := t.TempDir()

	if err := exportSchemas(dir); err != nil {
		t.Fatalf("exportSchemas: %v", err)
	}

	schemas := outputschemas.Schemas()
	for name := range schemas {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", name, err)
			continue
		}

		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Errorf("file %s is not valid JSON: %v", name, err)
		}
	}
}
