package outputschemas

import (
	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli/jsonoutput"
)

// repositoryRefSchema returns a partial schema for the repository reference
// object emitted inline in commit and branch command outputs.  Fields match
// the service.RepositoryRef struct (no JSON tags → Go field names used as-is).
func repositoryRefSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"ProjectKey": map[string]any{"type": "string"},
			"Slug":       map[string]any{"type": "string"},
		},
	}
}

// restCommitSchema returns a partial JSON schema for a single RestCommit.
func restCommitSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"id":                 map[string]any{"type": "string", "description": "Full commit SHA1."},
			"displayId":          map[string]any{"type": "string", "description": "Abbreviated commit SHA1."},
			"message":            map[string]any{"type": "string"},
			"authorTimestamp":    map[string]any{"type": "integer"},
			"committerTimestamp": map[string]any{"type": "integer"},
		},
	}
}

func commitSchemas() map[string]map[string]any {
	return map[string]map[string]any{
		"output.commit.list.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.commit.list.schema.json",
			"bb commit list output",
			"JSON output schema for `bb commit list --json`. Data contains the repository reference and a list of commits.",
			map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"properties": map[string]any{
					"repository": repositoryRefSchema(),
					"commits": map[string]any{
						"type":  "array",
						"items": restCommitSchema(),
					},
				},
				"required": []any{"repository", "commits"},
			},
		),
		"output.commit.get.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.commit.get.schema.json",
			"bb commit get output",
			"JSON output schema for `bb commit get --json`. Data contains the repository reference and a single commit.",
			map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"properties": map[string]any{
					"repository": repositoryRefSchema(),
					"commit":     restCommitSchema(),
				},
				"required": []any{"repository", "commit"},
			},
		),
	}
}
