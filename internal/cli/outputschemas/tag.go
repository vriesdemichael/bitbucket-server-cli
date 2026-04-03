package outputschemas

import (
	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli/jsonoutput"
)

// restTagSchema returns a partial JSON schema for a single RestTag object.
// Fields match the openapi-generated RestTag struct (all optional per the API).
func restTagSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"id":              map[string]any{"type": "string", "description": "Full tag ref name (e.g. refs/tags/v1.0.0)."},
			"displayId":       map[string]any{"type": "string", "description": "Short tag name (e.g. v1.0.0)."},
			"type":            map[string]any{"type": "string", "enum": []any{"TAG", "ANNOTATED_TAG"}},
			"latestCommit":    map[string]any{"type": "string", "description": "SHA1 of the tagged commit."},
			"latestChangeset": map[string]any{"type": "string"},
			"hash":            map[string]any{"type": "string", "description": "SHA1 of the tag object (annotated tags only)."},
		},
	}
}

func tagSchemas() map[string]map[string]any {
	return map[string]map[string]any{
		"output.tag.list.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.tag.list.schema.json",
			"bb tag list output",
			"JSON output schema for `bb tag list --json`. Data is an array of tag objects.",
			map[string]any{
				"type":  "array",
				"items": restTagSchema(),
			},
		),
		"output.tag.view.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.tag.view.schema.json",
			"bb tag view output",
			"JSON output schema for `bb tag view --json`. Data is a single tag object.",
			restTagSchema(),
		),
		"output.tag.create.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.tag.create.schema.json",
			"bb tag create output",
			"JSON output schema for `bb tag create --json`. Data is the newly created tag object.",
			restTagSchema(),
		),
		"output.tag.delete.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.tag.delete.schema.json",
			"bb tag delete output",
			"JSON output schema for `bb tag delete --json`. Data confirms the deleted tag name.",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"status": map[string]any{"const": "ok"},
					"tag":    map[string]any{"type": "string", "description": "Name of the deleted tag."},
				},
				"required": []any{"status", "tag"},
			},
		),
	}
}
