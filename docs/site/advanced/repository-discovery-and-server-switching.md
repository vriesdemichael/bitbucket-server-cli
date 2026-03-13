# Repository Discovery and Server Switching

## Why this matters

When you run `bb` inside a local git repository, the CLI can infer `PROJECT/slug` and host
from matching remotes. This reduces repeated `--repo` flags while still keeping behavior explicit.

## Repository discovery behavior

Discovery runs only when a command has a `--repo` flag and you did not set it.

`bb` inspects git remotes and tries to parse Bitbucket-style URLs such as:

- `https://bitbucket.acme.corp/scm/PLAT/payments-api.git`
- `ssh://git@bitbucket.acme.corp:7999/scm/PLAT/payments-api.git`
- `git@bitbucket.acme.corp:scm/PLAT/payments-api.git`

If a remote host matches an authenticated/stored server context, `bb` infers:

- `BITBUCKET_URL`
- `BITBUCKET_PROJECT_KEY`
- `BITBUCKET_REPO_SLUG`
- and sets the effective `--repo` value to `PROJECT/slug`

Human mode emits a banner on `stderr`:

```text
Using repository context from git remote "origin": PLAT/payments-api on https://bitbucket.acme.corp
```

JSON mode suppresses that banner to preserve machine output contracts on `stdout`.

## Precedence and safety

Repository selection precedence for repo-scoped commands:

1. Explicit `--repo PROJECT/slug`
2. Git remote discovery (if exactly one matching remote context exists)
3. `BITBUCKET_PROJECT_KEY` + `BITBUCKET_REPO_SLUG`

Host and auth source precedence remains:

1. CLI flags
2. Environment variables / `.env`
3. Git remote inference host override (when `--repo` is inferred from a matching authenticated remote)
4. Stored config (`~/.config/bb/config.yaml`) + keyring-backed credentials
5. Built-in defaults

## Ambiguity and fallback behavior

- If multiple remotes map to different repositories, discovery fails with a validation error and asks you to pass `--repo` and/or choose a server.
- If you are outside a git repository, discovery is skipped.
- If remotes do not match authenticated server hosts, discovery is skipped.

## Server switching workflow

Use server contexts to control which host is active by default:

```bash
bb auth server list
bb auth server use --host https://bitbucket.acme.corp
bb auth status
```

Expected human output:

```text
Active server set to https://bitbucket.acme.corp
Target Bitbucket: https://bitbucket.acme.corp (expected version 9.4.16, auth=token, source=stored/default)
```

Expected JSON output (example):

```json
{
  "version": "v2",
  "data": {
    "status": "ok",
    "default_host": "https://bitbucket.acme.corp"
  },
  "meta": {
    "contract": "bb.machine"
  }
}
```

## Recommended team pattern

- Keep one stored context per server (`bb auth login --host ...`).
- Switch active context with `bb auth server use --host ...` before running automation.
- Still pass `--repo` in CI for maximal explicitness.
