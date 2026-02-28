# bitbucket-server-cli

Go CLI and client for Bitbucket Server/Data Center automation with **live-behavior testing** against Atlassian Bitbucket `9.4.16`.

## Current state

This repository is an actively implemented CLI with:
- production-style command groups for auth, repo settings, comments, diff, tags, build status/merge checks, and code insights
- local Bitbucket Docker stack support for live behavior validation
- unit tests and live integration test suites

Current caveats:
- `bbsc pr list` is currently a placeholder (`not_implemented`)
- `bbsc issue list` is currently a placeholder (`not_implemented`)

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
- `task test:unit:coverage`
- `task quality:coverage`
- `task stack:up`
- `task stack:status`
- `go test -tags=live ./tests/integration/live -run TestOpenAPIParity`

Local coverage gate (not enforced in GitHub Actions due live infra constraints):

- Global unit coverage threshold: `85%`
- Patch coverage threshold versus `main`: `85%`
- Configurable via Task vars in `Taskfile.yml`:
	- `COVERAGE_MIN_TOTAL`
	- `COVERAGE_MIN_PATCH`
	- `COVERAGE_BASE_REF`

## GitHub Actions

- CI workflow: `.github/workflows/ci.yml`
	- Runs on pull requests to `main` and pushes to `main`
	- Executes `task quality:validate-decisions`
	- Executes `task test:unit`
	- Publishes a final aggregate check named `CI Complete` (recommended PR required check)
	- Does not run `test:live` because live integration tests require Bitbucket/Postgres infrastructure
- Release workflow: `.github/workflows/release.yml`
	- Manual trigger only via `workflow_dispatch`
	- Requires a version input (for example `v0.1.0`)
	- Generates release notes from Conventional Commit history, tags the commit, and publishes a GitHub release

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
- `go run ./cmd/bbsc tag list --repo TEST/my-repo --limit 50 --order-by ALPHABETICAL`
- `go run ./cmd/bbsc tag create v1.2.3 --repo TEST/my-repo --start-point <sha> --message "release v1.2.3"`
- `go run ./cmd/bbsc tag view v1.2.3 --repo TEST/my-repo`
- `go run ./cmd/bbsc tag delete v1.2.3 --repo TEST/my-repo`
- `go run ./cmd/bbsc build status set <sha> --key ci/main --state SUCCESSFUL --url https://ci.example/build/42`
- `go run ./cmd/bbsc build status get <sha> --order-by NEWEST --limit 25`
- `go run ./cmd/bbsc build status stats <sha> --include-unique`
- `go run ./cmd/bbsc build required list --repo TEST/my-repo`
- `go run ./cmd/bbsc build required create --repo TEST/my-repo --body '{"buildParentKeys":["ci"],"refMatcher":{"id":"refs/heads/master"}}'`
- `go run ./cmd/bbsc insights report set <sha> lint --repo TEST/my-repo --body '{"title":"Lint","result":"PASS","data":[{"title":"warnings","type":"NUMBER","value":{"value":0}}]}'`
- `go run ./cmd/bbsc insights report get <sha> lint --repo TEST/my-repo`
- `go run ./cmd/bbsc insights annotation add <sha> lint --repo TEST/my-repo --body '[{"externalId":"lint-1","message":"Fix warning","severity":"MEDIUM","path":"seed.txt","line":1}]'`

Runtime config precedence:

1. CLI flags
2. Environment variables / `.env`
3. Stored config (`~/.config/bbsc/config.yaml`) + keyring/fallback secrets
4. Built-in defaults

## Script contract status and gaps

`--json` output is supported broadly, but script contracts are not yet fully formalized for long-term automation stability.

Current gaps:
- JSON responses are not yet wrapped in a versioned envelope for all commands
- output schemas are not published as explicit compatibility contracts
- no dedicated golden-contract test layer for machine-output stability

Desired target state:
- versioned machine envelope (for example `version`, `data`, optional `meta`)
- documented compatibility policy for adding/deprecating/renaming fields
- contract-focused tests that fail on unplanned output shape changes

## Bulk operations wanted for enterprise use

For large corporate Bitbucket estates, single-repository imperative commands are not enough. Useful bulk capabilities include:

- apply one settings policy to many repositories selected by project/team pattern
- bulk permission grants/revocations with preview and validation before apply
- bulk webhook create/update/delete with drift detection and reconciliation reporting
- batch build-required-check and pull-request-settings updates across repo sets
- plan/apply workflows with `--dry-run` and machine-readable summary output

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
