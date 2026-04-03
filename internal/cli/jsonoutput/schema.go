package jsonoutput

const jsonSchemaVersion = "https://json-schema.org/draft/2020-12/schema"

// SchemaBaseURL is the base URL under which output schemas are published.
const SchemaBaseURL = "https://vriesdemichael.github.io/bitbucket-server-cli/latest/reference/schemas/output/"

// EnvelopeSchemaFor builds a full bb.machine v2 envelope schema whose data
// field is constrained to the supplied dataSchema.  title and description are
// shown in documentation tooling.
func EnvelopeSchemaFor(schemaFileName, title, description string, dataSchema map[string]any) map[string]any {
	return map[string]any{
		"$schema":              jsonSchemaVersion,
		"$id":                  SchemaBaseURL + schemaFileName,
		"title":                title,
		"description":          description,
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"version": map[string]any{"const": ContractVersion},
			"data":    dataSchema,
			"meta":    metaSchema(),
		},
		"required": []any{"version", "data", "meta"},
	}
}

func metaSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"contract": map[string]any{"const": ContractName},
		},
		"required": []any{"contract"},
	}
}
