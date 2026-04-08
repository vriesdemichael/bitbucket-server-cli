package outputschemas

import "github.com/vriesdemichael/bitbucket-server-cli/internal/cli/jsonoutput"

func updateSchemas() map[string]map[string]any {
	return map[string]map[string]any{
		"output.update.schema.json": jsonoutput.EnvelopeSchemaFor(
			"output.update.schema.json",
			"bb update output",
			"JSON output schema for `bb update --json`. Data describes the current version, latest release, selected artifact, checksum status, and whether an update was applied or only previewed.",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"current_version":            map[string]any{"type": "string"},
					"latest_version":             map[string]any{"type": "string"},
					"update_available":           map[string]any{"type": "boolean"},
					"up_to_date":                 map[string]any{"type": "boolean"},
					"applied":                    map[string]any{"type": "boolean"},
					"dry_run":                    map[string]any{"type": "boolean"},
					"install_path":               map[string]any{"type": "string"},
					"release_url":                map[string]any{"type": "string"},
					"asset_name":                 map[string]any{"type": "string"},
					"asset_url":                  map[string]any{"type": "string"},
					"checksum_asset_name":        map[string]any{"type": "string"},
					"checksum_available":         map[string]any{"type": "boolean"},
					"checksum_verified":          map[string]any{"type": "boolean"},
					"current_version_comparable": map[string]any{"type": "boolean"},
					"latest_version_comparable":  map[string]any{"type": "boolean"},
					"target_platform":            map[string]any{"type": "string"},
					"planned_action":             map[string]any{"type": "string"},
					"comparison":                 map[string]any{"type": "string"},
				},
				"required": []any{
					"current_version",
					"latest_version",
					"update_available",
					"up_to_date",
					"applied",
					"dry_run",
					"checksum_available",
					"checksum_verified",
					"current_version_comparable",
					"latest_version_comparable",
				},
			},
		),
	}
}
