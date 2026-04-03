package outputschemas

import (
	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli/jsonoutput"
)

// restRepositorySchema returns a partial JSON schema for a RestRepository object.
func restRepositorySchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"id":            map[string]any{"type": "integer"},
			"name":          map[string]any{"type": "string"},
			"slug":          map[string]any{"type": "string"},
			"description":   map[string]any{"type": "string"},
			"defaultBranch": map[string]any{"type": "string"},
			"archived":      map[string]any{"type": "boolean"},
			"forkable":      map[string]any{"type": "boolean"},
			"hierarchyId":   map[string]any{"type": "string"},
			"state":         map[string]any{"type": "string"},
			"statusMessage": map[string]any{"type": "string"},
		},
	}
}

func repoSchemas() map[string]map[string]any {
	return map[string]map[string]any{
		"output.repo.list.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.repo.list.schema.json",
			"bb repo list output",
			"JSON output schema for `bb repo list --json`. Data is an array of repository objects.",
			map[string]any{
				"type":  "array",
				"items": restRepositorySchema(),
			},
		),
	}
}
