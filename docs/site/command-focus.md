# Command Focus

The CLI centers around grouped commands for discoverability and consistency.

## Key groups

- `auth`
- `repo`
- `pr`
- `issue`
- `admin`
- `search`

## Discovery-first usage

Use top-level search for cross-project discovery:

```bash
go run ./cmd/bb search repos --name demo
go run ./cmd/bb search prs --state OPEN
```

Detailed reference pages will be added as part of the next documentation pass.
