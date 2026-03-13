# JSON Schemas

## Bulk workflow schemas

The project publishes JSON schemas for bulk policy planning and execution workflows.

- [bulk-policy.schema.json](schemas/bulk-policy.schema.json)
- [bulk-plan.schema.json](schemas/bulk-plan.schema.json)
- [bulk-apply-status.schema.json](schemas/bulk-apply-status.schema.json)

Schema source-of-truth is generated from Go workflow models in `internal/workflows/bulk/schema.go`.

## Regenerate schemas

```bash
task docs:export-bulk-schemas
task docs:publish-bulk-schemas
```

or regenerate all docs artifacts:

```bash
task docs:generate
```

## IDE integration for YAML policy files

Add a schema comment at the top of a bulk policy YAML file:

```yaml
# yaml-language-server: $schema=https://vriesdemichael.github.io/bitbucket-server-cli/latest/reference/schemas/bulk-policy.schema.json
apiVersion: bb.io/v1alpha1
selector:
  projectKey: TEST
operations:
  - type: repo.permission.user.grant
    username: ci-bot
    permission: REPO_WRITE
```

Equivalent repository-relative schema association is also valid for local development:

```yaml
# yaml-language-server: $schema=../reference/schemas/bulk-policy.schema.json
```

## Schema usage guidance

- Use policy schema for authoring bulk policy YAML/JSON input files.
- Use plan schema to validate reviewed plan artifacts produced by `bb bulk plan`.
- Use apply-status schema to validate outputs from `bb bulk apply` and `bb bulk status`.
