package outputschemas_test

import (
	"strings"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli/outputschemas"
)

func TestSchemasReturnNonEmpty(t *testing.T) {
	schemas := outputschemas.Schemas()

	if len(schemas) == 0 {
		t.Fatal("Schemas() returned no schemas")
	}

	for name, schema := range schemas {
		if schema == nil {
			t.Errorf("schema %s is nil", name)
		}

		if _, ok := schema["$schema"]; !ok {
			t.Errorf("schema %s missing $schema field", name)
		}

		if _, ok := schema["$id"]; !ok {
			t.Errorf("schema %s missing $id field", name)
		}

		id, _ := schema["$id"].(string)
		if !strings.Contains(id, name) {
			t.Errorf("schema %s $id %q does not contain the file name", name, id)
		}

		props, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Errorf("schema %s missing or invalid properties", name)
			continue
		}

		for _, field := range []string{"version", "data", "meta"} {
			if _, ok := props[field]; !ok {
				t.Errorf("schema %s missing envelope property %q", name, field)
			}
		}
	}
}
