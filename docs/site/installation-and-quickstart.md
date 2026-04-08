# Installation and Quickstart

## Install on Windows via WinGet

```powershell
winget install vriesdemichael.bb
```

## Install on Windows via Scoop

```powershell
scoop bucket add scoop https://github.com/vriesdemichael/scoop
scoop install scoop/bb
```

## Install from release artifacts

1. Select a release version (example: `v0.1.0`).
2. Download the platform archive and `sha256sums.txt` from GitHub Releases.
3. Verify checksums and run `bb --help`.

Linux amd64 example:

```bash
VERSION=v0.1.0
curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/bb_${VERSION#v}_linux_amd64.tar.gz"
curl -LO "https://github.com/vriesdemichael/bitbucket-server-cli/releases/download/${VERSION}/sha256sums.txt"
sha256sum -c sha256sums.txt --ignore-missing
tar -xzf "bb_${VERSION#v}_linux_amd64.tar.gz"
install -m 0755 bb /usr/local/bin/bb
bb --help
```

Optional provenance verification:

```bash
gh attestation verify bb_${VERSION#v}_linux_amd64.tar.gz --repo vriesdemichael/bitbucket-server-cli
```

## Authenticate to Bitbucket

```bash
bb auth token-url --host https://bitbucket.acme.corp
bb auth login --host https://bitbucket.acme.corp --token "$BB_TOKEN"
bb auth status
```

## First useful commands

```bash
bb repo clone PLATFORM/api
bb browse --repo PLATFORM/api
bb search repos --limit 20
bb search prs --state OPEN
bb --json auth status
```

## Runtime flags and environment variables

Global runtime controls are available as flags and environment variables:

- `--ca-file` / `BB_CA_FILE`
- `--insecure-skip-verify` / `BB_INSECURE_SKIP_VERIFY`
- `--request-timeout` / `BB_REQUEST_TIMEOUT`
- `--retry-count` / `BB_RETRY_COUNT`
- `--retry-backoff` / `BB_RETRY_BACKOFF`
- `--log-level` / `BB_LOG_LEVEL`
- `--log-format` / `BB_LOG_FORMAT`

See [Basic Usage](basic-usage.md) for precedence, dry-run behavior, machine mode, and diagnostics guidance.
