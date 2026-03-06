# bitbucket-server-cli

[![codecov](https://codecov.io/gh/vriesdemichael/bitbucket-server-cli/branch/main/graph/badge.svg)](https://codecov.io/gh/vriesdemichael/bitbucket-server-cli)

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
- `task test:unit:coverage`
- `task test:live:coverage`
- `task quality:coverage`
- `task quality:coverage:report:update`
- `task quality:coverage:report:verify`
- `task quality:coverage:report:verify:committed`
- `task stack:up`
- `task stack:status`
- `go test -tags=live ./tests/integration/live -run TestOpenAPIParity`

Coverage/reporting workflow:

- combined metric source is unit + live coverage merged into one report
- coverage gates: global combined coverage >= 85% (maintained source scope)
- patch gate applies to maintained source scope (`cmd/` + `internal/`, generated excluded)
- patch gate policy: >=85% when coverable patch lines >= 30, otherwise allow up to 2 uncovered changed lines
- patch baseline: compare against up-to-date `origin/main`
- pre-commit hook gate: `task quality:coverage:origin-main` (runs live tests and enforces both thresholds)
- local report update (commit this artifact): `task quality:coverage:report:update`
- local pre-push verification (recompute + compare): `task quality:coverage:report:verify`
- CI verification (committed artifact only): `task quality:coverage:report:verify:committed`
- committed report file: `docs/quality/coverage-report.json`
- generated operation contract manifest: `docs/quality/generated-operation-contracts.json`
- Codecov upload sources in CI are committed combined profiles: `docs/quality/coverage.combined.raw.out` and `docs/quality/coverage.combined.scoped.out`
- Codecov reports two combined views via flags in `codecov.yml`: `combined_raw` and `combined_scoped`
- Live + combined metrics remain enforced by committed artifacts generated locally via Task hooks/workflow
- CI threshold configuration is code-based via `.github/coverage-thresholds.env` (`CI_COVERAGE_MIN_GLOBAL_COMBINED`, `CI_COVERAGE_MIN_PATCH`, `CI_COVERAGE_MIN_PATCH_LINES`, `CI_COVERAGE_MAX_UNCOVERED_SMALL_PATCH`, `CI_COVERAGE_MIN_CONTRACT`)
- CI ADR floors are enforced even when variables are configured: global >= 85 and patch >= 85

## GitHub Actions

- CI workflow: `.github/workflows/ci.yml`
	- Runs on pull requests to `main` and pushes to `main`
	- Executes `task quality:validate-decisions`
	- Executes `task test:go:safe` (targeted non-live package scope: `cmd/...`, `internal/...`, `tools/...`)
	- Uploads committed combined raw/scoped coverage profiles to Codecov
	- Executes `task quality:coverage:report:verify:committed` with configurable CI thresholds (subject to ADR floor minimums)
	- Publishes a final aggregate check named `CI Complete` (recommended PR required check)
	- Does not run `test:live` because live integration tests require Bitbucket/Postgres infrastructure
- Release workflow: `.github/workflows/release.yml`
	- Manual trigger only via `workflow_dispatch`
	- Requires a version input (for example `v0.1.0`)
	- Builds and packages release binaries for Linux/macOS/Windows on amd64 + arm64
	- Generates a `sha256sums.txt` manifest for all packaged artifacts
	- Publishes GitHub-native build provenance attestations for release artifacts
	- Generates release notes from Conventional Commit history, tags the commit, and publishes a GitHub release with attached artifacts

## Binary installation (GitHub releases)

Download artifacts from the release page for a specific version:

- `bbsc_<version>_linux_amd64.tar.gz`
- `bbsc_<version>_linux_arm64.tar.gz`
- `bbsc_<version>_darwin_amd64.tar.gz`
- `bbsc_<version>_darwin_arm64.tar.gz`
- `bbsc_<version>_windows_amd64.zip`
- `bbsc_<version>_windows_arm64.zip`
- `sha256sums.txt`

Example (Linux amd64, version `v0.1.0`):

- `VERSION=v0.1.0`
- `curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/bbsc_${VERSION#v}_linux_amd64.tar.gz"`
- `curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/sha256sums.txt"`
- `sha256sum -c sha256sums.txt --ignore-missing`
- `tar -xzf "bbsc_${VERSION#v}_linux_amd64.tar.gz"`
- `chmod +x bbsc`
- `./bbsc --help`

Optional provenance verification (GitHub CLI):

- `gh attestation verify bbsc_${VERSION#v}_linux_amd64.tar.gz --repo vriesdemichael/bitbucket-server-cli`

Source-based fallback remains available via `go run ./cmd/bbsc --help`.

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
