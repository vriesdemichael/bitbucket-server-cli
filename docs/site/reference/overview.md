# Command Reference Overview

## Generated reference model

Command reference pages are generated from the CLI command tree and include the same sections as
`bb ... --help` output:

- Usage
- Available commands
- Flags
- Global flags

Source and generation path:

- Command tree source: `internal/cli/`
- Export tool: `tools/cli-docs-export/main.go`
- Generated page: `docs/site/reference/commands/index.md`

## Regenerate command docs

```bash
task docs:export-command-reference
```

or regenerate all docs artifacts:

```bash
task docs:generate
```

## Drift checks

The docs workflow verifies generated content is up to date:

```bash
task docs:verify-generated
```

Use [All Commands](commands/index.md) for full command and argument details.
