# bitbucket-server-cli

Go CLI and client for Bitbucket Server/Data Center automation with **live-behavior testing** against Atlassian Bitbucket `9.4.16`.

## Current state

This repository is scaffolded with:
- minimal package and CLI skeleton
- local Bitbucket Docker stack skeleton
- test layout for unit + live integration suites

## Development workflow

- Taskfile is the primary command interface for recurring workflows.
- Decision records live in [docs/decisions](docs/decisions) and are validated with a Go validator.

Common commands:

- `task --list`
- `task quality:validate-decisions`
- `task docs:refresh-openapi`
- `task models:generate`
- `task models:verify`
- `task client:generate`
- `task client:verify`
- `task stack:up`
- `task stack:status`
- `go test -tags=live ./tests/integration/live -run TestOpenAPIParity`

Runtime environment variables:

- `BITBUCKET_URL` (default `http://localhost:7990`)
- `BITBUCKET_TOKEN` (optional)
- `BITBUCKET_USERNAME` + `BITBUCKET_PASSWORD` (optional basic auth; `BITBUCKET_USER` is accepted as alias for username)
- `ADMIN_USER` + `ADMIN_PASSWORD` fallback for local setup compatibility

Authentication workflow:

- `go run ./cmd/bbsc auth login --host http://localhost:7990 --username admin --password admin`
- `go run ./cmd/bbsc auth status`
- `go run ./cmd/bbsc auth logout`
- `go run ./cmd/bbsc diff refs main feature --repo TEST/my-repo`
- `go run ./cmd/bbsc diff pr 123 --repo TEST/my-repo --patch`
- `go run ./cmd/bbsc diff commit <sha> --repo TEST/my-repo --path seed.txt`
- `go run ./cmd/bbsc repo comment list --repo TEST/my-repo --commit <sha> --path seed.txt`
- `go run ./cmd/bbsc repo comment create --repo TEST/my-repo --pr 123 --text "Looks good"`
- `go run ./cmd/bbsc repo comment update --repo TEST/my-repo --commit <sha> --id 42 --text "Updated text"`
- `go run ./cmd/bbsc repo comment delete --repo TEST/my-repo --pr 123 --id 42`

Runtime config precedence:

1. CLI flags
2. Environment variables / `.env`
3. Stored config (`~/.config/bbsc/config.yaml`) + keyring/fallback secrets
4. Built-in defaults

API reference source:

- Atlassian Bitbucket Data Center REST docs for version 9.4
- Vendored OpenAPI reference: [docs/reference/atlassian/bitbucket-9.4-openapi.json](docs/reference/atlassian/bitbucket-9.4-openapi.json)
- Generated client baseline: [internal/openapi/generated/bitbucket_client.gen.go](internal/openapi/generated/bitbucket_client.gen.go)
- OpenAPI fix registry: [docs/openapi/fixes.yaml](docs/openapi/fixes.yaml)
- Repository settings command mapping: [docs/REPO_SETTINGS_COMMANDS.md](docs/REPO_SETTINGS_COMMANDS.md)
- Real server behavior must still be validated through live integration tests

## Licensing note

Atlassian Bitbucket Server/Data Center is proprietary software.

- The Docker image used for local integration testing is an official Atlassian distribution, not open-source software.
- Running it locally requires a valid license path (for example, an Atlassian trial/evaluation or paid license).
- Use of Bitbucket in this project must comply with Atlassian's EULA/terms (including any usage limits or expiry constraints).
