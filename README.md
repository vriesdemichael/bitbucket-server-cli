# bitbucket-server-cli

[![codecov](https://codecov.io/gh/vriesdemichael/bitbucket-server-cli/branch/main/graph/badge.svg?flag=combined_scoped)](https://codecov.io/gh/vriesdemichael/bitbucket-server-cli?flag=combined_scoped)

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
- `task docs:generate`
- `task docs:verify-generated`
- `task docs:validate`
- `task docs:bootstrap-pages VERSION=v0.1.0`
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
- Codecov upload source in CI is the committed scoped combined profile: `docs/quality/coverage.combined.scoped.out`
- Codecov badge/status is pinned to `combined_scoped` and excludes generated code paths
- Codecov components in `codecov.yml` split coverage views by `internal-api`, `cmd`, and `tools` for scoped reporting
- Live + combined metrics remain enforced by committed artifacts generated locally via Task hooks/workflow
- CI threshold configuration is code-based via `.github/coverage-thresholds.env` (`CI_COVERAGE_MIN_GLOBAL_COMBINED`, `CI_COVERAGE_MIN_PATCH`, `CI_COVERAGE_MIN_PATCH_LINES`, `CI_COVERAGE_MAX_UNCOVERED_SMALL_PATCH`, `CI_COVERAGE_MIN_CONTRACT`)
- CI ADR floors are enforced even when variables are configured: global >= 85 and patch >= 85

## GitHub Actions

- CI workflow: `.github/workflows/ci.yml`
	- Runs on pull requests to `main` and pushes to `main`
	- Executes `task quality:validate-decisions`
	- Executes `task test:go:safe` (targeted non-live package scope: `cmd/...`, `internal/...`, `tools/...`)
	- Uploads committed combined scoped coverage profile to Codecov
	- Executes `task quality:coverage:report:verify:committed` with configurable CI thresholds (subject to ADR floor minimums)
	- Publishes a final aggregate check named `CI Complete` (recommended PR required check)
	- Does not run `test:live` because live integration tests require Bitbucket/Postgres infrastructure
- Release workflow: `.github/workflows/release.yml`
	- Automatically runs on pushes to `main` (including PR merges)
	- Derives SemVer version bumps from Conventional Commits (`major` for breaking, `minor` for `feat`, `patch` otherwise)
	- Supports optional manual `workflow_dispatch` version override for controlled backfills/hotfixes
	- Builds and packages release binaries for Linux/macOS/Windows on amd64 + arm64
	- Generates a `sha256sums.txt` manifest for all packaged artifacts
	- Publishes GitHub-native build provenance attestations for release artifacts
	- Generates rich release notes from Conventional Commit history (breaking changes + commit/compare links), emits `changelog.json`, tags the commit, and publishes a GitHub release with attached artifacts
	- Publishes versioned docs to GitHub Pages (`gh-pages`) via `mike` with `latest` alias

Docs versioning/publishing:

- Built docs content is versioned by release tag (for example `v0.1.0`) and aliased to `latest`.
- Publication is wired into the release workflow and runs when a release is produced.
- Non-release changes are validated via CI (`task docs:build`) and pre-push (`task docs:validate`).
- First-time setup helper: `task docs:bootstrap-pages VERSION=<first-release-tag>` to create/populate `gh-pages` before enabling GitHub Pages settings.

## Binary installation (GitHub releases)

Download artifacts from the release page for a specific version:

- `bb_<version>_linux_amd64.tar.gz`
- `bb_<version>_linux_arm64.tar.gz`
- `bb_<version>_darwin_amd64.tar.gz`
- `bb_<version>_darwin_arm64.tar.gz`
- `bb_<version>_windows_amd64.zip`
- `bb_<version>_windows_arm64.zip`
- `sha256sums.txt`

Example (Linux amd64, version `v0.1.0`):

- `VERSION=v0.1.0`
- `curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/bb_${VERSION#v}_linux_amd64.tar.gz"`
- `curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/sha256sums.txt"`
- `sha256sum -c sha256sums.txt --ignore-missing`
- `tar -xzf "bb_${VERSION#v}_linux_amd64.tar.gz"`
- `install -m 0755 bb /usr/local/bin/bb`
- `bb --help`

Optional provenance verification (GitHub CLI):

- `gh attestation verify bb_${VERSION#v}_linux_amd64.tar.gz --repo vriesdemichael/bitbucket-server-cli`

Runtime environment variables:

- `BITBUCKET_URL` (default `http://localhost:7990`; usually set to your corporate Bitbucket URL)
- `BITBUCKET_TOKEN` (optional)
- `BITBUCKET_USERNAME` + `BITBUCKET_PASSWORD` (optional basic auth; `BITBUCKET_USER` is accepted as alias for username)
- `ADMIN_USER` + `ADMIN_PASSWORD` fallback for local setup compatibility
- `BB_CA_FILE` (optional path to PEM CA bundle for custom trust chains)
- `BB_INSECURE_SKIP_VERIFY` (optional bool, default `false`; disables TLS cert verification for local/dev only)
- `BB_REQUEST_TIMEOUT` (optional Go duration, default `20s`)
- `BB_RETRY_COUNT` (optional non-negative integer, default `2`)
- `BB_RETRY_BACKOFF` (optional Go duration, default `250ms`)
- `BB_LOG_LEVEL` (optional: `error|warn|info|debug`, default `error`)
- `BB_LOG_FORMAT` (optional: `text|jsonl`, default `text`)

Equivalent global CLI flags (highest precedence):

- `--ca-file`
- `--insecure-skip-verify`
- `--request-timeout`
- `--retry-count`
- `--retry-backoff`
- `--log-level`
- `--log-format`
- `--dry-run` (server-mutating Bitbucket commands only; excludes local auth/config mutators and emits dry-run preview output)

Authentication workflow:

- `bb auth login --host https://bitbucket.acme.corp --token "$BB_TOKEN"`
- `bb auth status`
- `bb auth server list`
- `bb auth server use --host https://bitbucket.acme.corp`
- `bb --request-timeout 45s --retry-count 4 --retry-backoff 500ms auth status`
- `bb --log-level debug auth status`
- `bb --log-level warn --log-format jsonl auth status 2> diagnostics.jsonl`

Dry-run behavior:

- `--dry-run` applies only to server-mutating Bitbucket commands; local auth/config mutators are out of scope.
- `--dry-run` emits structured previews in both human and `--json` mode and does not execute write side effects.
- Implemented mutation command families use `stateful` planning backed by live server reads/prediction rather than write execution. This includes the main branch, tag, repo settings/security, project, reviewer, hook, repo admin, pull request, comment, insights, and build mutation flows.
- Preview payloads explicitly report `planning_mode`, `capability`, predicted action, reason, and blocking/required-state details when available.
- Static preview support remains as a safety fallback for unsupported or future paths, but the primary rollout target is stateful planning for server mutations.
- Live integration tests validate no-side-effect behavior by capturing server context before a dry-run, executing the dry-run preview, and asserting the relevant server state is unchanged afterward.

Repository context behavior:

- `--repo PROJECT/slug` always has highest precedence.
- When `--repo` is omitted, bb tries to infer repository context from local git remotes that match an authenticated host profile.
- Inference also populates the `--repo` flag internally, so commands that mark `--repo` as required continue to work without explicitly passing it.
- If multiple remotes map to different repositories, bb returns an ambiguity error and asks you to pass `--repo` and/or select a server with `auth server use --host`.
- Non-repository directories (or remotes that do not match authenticated hosts) fall back to the normal repository-required validation message.

Diagnostics and supportability notes:

- Diagnostics are emitted to `stderr` so command result output contracts on `stdout` remain unchanged.
- Use `--log-format jsonl` for machine-filterable CI logs and support attachments.
- Request diagnostics include endpoint path, HTTP status, retry count, and duration.
- Sensitive values are redacted (token/password/secret/auth credentials and sensitive URL query values).

JSON output contract (`--json`):

- Every machine-mode response is wrapped in a versioned envelope: `{ "version": "v2", "data": <command payload>, "meta": { "contract": "bb.machine" } }`.
- `data` preserves the existing command-specific shape (arrays/objects/scalars) inside the envelope.
- Backward-compatible changes for `v2`: additive fields only.
- Breaking changes (field removal/rename/type changes or envelope shape changes) require a new contract version and migration notes.
- `bb auth logout`
- `bb diff refs main feature --repo TEST/my-repo`
- `bb diff pr 123 --repo TEST/my-repo --patch`
- `bb diff commit <sha> --repo TEST/my-repo --path seed.txt`
- `bb repo comment list --repo TEST/my-repo --commit <sha> --path seed.txt`
- `bb repo comment create --repo TEST/my-repo --pr 123 --text "Looks good"`
- `bb repo comment update --repo TEST/my-repo --commit <sha> --id 42 --text "Updated text"`
- `bb repo comment delete --repo TEST/my-repo --pr 123 --id 42`
- `bb tag list --repo TEST/my-repo --limit 50 --order-by ALPHABETICAL`
- `bb tag create v1.2.3 --repo TEST/my-repo --start-point <sha> --message "release v1.2.3"`
- `bb tag view v1.2.3 --repo TEST/my-repo`
- `bb tag delete v1.2.3 --repo TEST/my-repo`
- `bb build status set <sha> --key ci/main --state SUCCESSFUL --url https://ci.example/build/42`
- `bb build status get <sha> --order-by NEWEST --limit 25`
- `bb build status stats <sha> --include-unique`
- `bb build required list --repo TEST/my-repo`
- `bb build required create --repo TEST/my-repo --body '{"buildParentKeys":["ci"],"refMatcher":{"id":"refs/heads/master"}}'`
- `bb insights report set <sha> lint --repo TEST/my-repo --body '{"title":"Lint","result":"PASS","data":[{"title":"warnings","type":"NUMBER","value":{"value":0}}]}'`
- `bb insights report get <sha> lint --repo TEST/my-repo`
- `bb insights annotation add <sha> lint --repo TEST/my-repo --body '[{"externalId":"lint-1","message":"Fix warning","severity":"MEDIUM","path":"seed.txt","line":1}]'`
- `bb bulk plan -f docs/examples/bulk-policy.yaml -o .tmp/bulk-plan.json`
- `bb bulk apply --from-plan .tmp/bulk-plan.json`
- `bb bulk status <operation-id>`
- Bulk JSON Schemas: `docs/reference/schemas/bulk-policy.schema.json`, `docs/reference/schemas/bulk-plan.schema.json`, `docs/reference/schemas/bulk-apply-status.schema.json`

Bulk policy workflow:

- Policy schema fields: `apiVersion`, `selector`, `operations`
- Selector support: `projectKey`, `repoPattern`, and explicit `repositories`
- Initial operation types: `repo.permission.user.grant`, `repo.permission.group.grant`, `repo.webhook.create`, `repo.pull-request-settings.required-all-tasks-complete`, `repo.pull-request-settings.required-approvers-count`, `build.required.create`
- `bulk plan` performs no writes and emits a deterministic reviewed plan artifact with a `planHash`; this is the preview/dry-run mechanism for bulk workflows
- `bulk apply` executes only operations embedded in the reviewed plan and persists result status under the local BB config directory (override with `BB_BULK_STATUS_DIR`)
- Example policy: `docs/examples/bulk-policy.yaml`
- Schema export command: `task docs:export-bulk-schemas`

Runtime config precedence:

1. CLI flags
2. Environment variables / `.env`
3. Git remote inference (repo + host context, when `--repo` is omitted and a unique authenticated remote match exists)
4. Stored config (`~/.config/bb/config.yaml`) + keyring/fallback secrets
5. Built-in defaults

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
