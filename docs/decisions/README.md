# Decision Records

This directory contains Architecture and Development Decision Records for this project.

## Structure

Each decision is a YAML file named:

- `NNN-short-kebab-title.yaml`

Required fields:

- `number`: sequential number (1, 2, 3, ...)
- `title`: short decision title
- `category`: `architecture` or `development`
- `decision`: what was decided
- `agent_instructions`: how coding agents should apply the decision
- `rationale`: why this was chosen
- `provenance`: `human`, `guided-ai`, or `autonomous-ai`

Optional fields:

- `status`: `proposed`, `accepted`, `superseded`, `deprecated`
- `superseded_by`: decision number that supersedes this decision
- `supersedes`: decision number or list of numbers this decision replaces
- `rejected_alternatives`: list of `{ alternative, reason }`

## Validation (Go)

Validate all decision records:

```bash
go run ./tools/adr-validator --validate
```

Validate specific files:

```bash
go run ./tools/adr-validator --validate docs/decisions/001-go-as-primary-implementation.yaml
```

Or use Task:

```bash
task quality:validate-decisions
```

## For AI Agents

On repo checkout, load all decision instructions and keep them in context before making architectural or workflow choices:

```bash
yq eval -o=json '. | {number: .number, title: .title, category: .category, agent_instructions: .agent_instructions}' docs/decisions/*.yaml
```

When user requests conflict with accepted decisions, cite the decision number and title, explain the conflict, and suggest either:

- a compliant alternative, or
- a new superseding decision record.
