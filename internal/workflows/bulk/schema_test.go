package bulk

import (
	"encoding/json"
	"testing"
)

func TestSchemasExposeExpectedArtifacts(t *testing.T) {
	schemas := Schemas()
	expected := []string{
		"bulk-policy.schema.json",
		"bulk-plan.schema.json",
		"bulk-apply-status.schema.json",
	}

	for _, name := range expected {
		schema, ok := schemas[name]
		if !ok {
			t.Fatalf("expected schema %s", name)
		}
		if _, err := json.Marshal(schema); err != nil {
			t.Fatalf("schema %s is not JSON-serializable: %v", name, err)
		}
		if schema["$schema"] != jsonSchemaVersion {
			t.Fatalf("expected schema version %s, got %#v", jsonSchemaVersion, schema["$schema"])
		}
	}
}

func TestPolicySchemaReferencesSupportedOperationTypes(t *testing.T) {
	schema := PolicyJSONSchema()
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("expected $defs in policy schema")
	}
	operation, ok := defs["Operation"].(map[string]any)
	if !ok {
		t.Fatal("expected Operation definition")
	}
	oneOf, ok := operation["oneOf"].([]any)
	if !ok || len(oneOf) != len(supportedOperationTypes) {
		t.Fatalf("expected %d operation schema variants, got %#v", len(supportedOperationTypes), operation["oneOf"])
	}
}
