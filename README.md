# Bitbucket Server CLI (`bb`)

[![codecov](https://codecov.io/gh/vriesdemichael/bitbucket-server-cli/branch/main/graph/badge.svg?flag=combined_scoped)](https://codecov.io/gh/vriesdemichael/bitbucket-server-cli?flag=combined_scoped)

`bb` is a production-focused CLI for automating Bitbucket Server/Data Center workflows.
It combines scriptable machine output, safe dry-run planning, and high-coverage live-behavior
validation against real Bitbucket APIs.

It is designed as the `gh`-style CLI experience for Bitbucket Server/Data Center, including
repository cloning and browser navigation ergonomics tailored to Bitbucket-hosted projects.

## Why teams adopt `bb`

- **Operationally safe by default**: dry-run planning for server mutations and explicit bulk plan/apply workflows.
- **Automation friendly**: stable JSON envelope contract (`bb.machine`, `v2`) for CI/CD and internal tooling.
- **Spec-driven API interactions**: client/server interactions are derived from Bitbucket Server's official OpenAPI spec.
- **Git-native ergonomics**: repository discovery from matching remotes to reduce repetitive `--repo` usage.
- **Enterprise-ready auth model**: token/basic auth with persisted server contexts and secure credential handling.
- **Live-tested command behavior**: command workflows are validated against a real Bitbucket Data Center server, not mocks alone.

## What you can do with it

- Manage repositories, permissions, hooks, branches, tags, commits, and refs.
- Work with pull requests, comments, build statuses, and merge checks.
- Run project/admin operations and cross-repository search.
- Apply policy-driven multi-repository changes via bulk plan/review/apply workflows.
- Clone repositories and open repository pages quickly (`bb repo clone`, `bb browse`).

## Quick start

Install from Releases (Linux amd64 example):

```bash
VERSION=v1.0.0
curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/bb_${VERSION#v}_linux_amd64.tar.gz"
curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/sha256sums.txt"
sha256sum -c sha256sums.txt --ignore-missing
tar -xzf "bb_${VERSION#v}_linux_amd64.tar.gz"
install -m 0755 bb /usr/local/bin/bb
```

Authenticate and run first commands:

```bash
bb auth login --host https://bitbucket.acme.corp --token "$BB_TOKEN"
bb auth status
bb repo clone PLATFORM/api
bb browse --repo PLATFORM/api
bb search repos --limit 20
bb --json auth status
```

## Docs

- Full docs site: <https://vriesdemichael.github.io/bitbucket-server-cli/latest/>
- Installation and Quickstart: `docs/site/installation-and-quickstart.md`
- Basic Usage: `docs/site/basic-usage.md`
- Advanced Topics: `docs/site/advanced/index.md`
- Command Reference (generated): `docs/site/reference/commands/index.md`
- ADR Index: `docs/site/adr/index.md`

## Compatibility and contracts

- Primary target: Atlassian Bitbucket Data Center `9.4.x`
- API contract source: Atlassian Bitbucket `9.4` OpenAPI (`docs/reference/atlassian/bitbucket-9.4-openapi.json`)
- CLI identity and machine contract: `bb` / `bb.machine` `v2`
- JSON schemas for bulk policy/plan/status published in docs and versioned with releases

## For contributors

This README is an adopter-focused landing page.

- Development workflows and project tasks: `Taskfile.yml`
- Decision records: `docs/decisions/`
- Generated docs and docs tooling: `docs/site/`, `tools/cli-docs-export/`, `tools/adr-markdown-export/`

## License and platform note

Atlassian Bitbucket Server/Data Center is proprietary software.
Use of local Docker images and server instances must comply with Atlassian licensing terms.
