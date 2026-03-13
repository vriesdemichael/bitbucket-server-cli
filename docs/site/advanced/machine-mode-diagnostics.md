# Machine Mode and Diagnostics

## Machine mode contract

Use global `--json` for machine-consumable output.

Envelope shape:

```json
{
  "version": "v2",
  "data": {},
  "meta": {
    "contract": "bb.machine"
  }
}
```

`data` holds command-specific payloads. Additive fields are allowed in `v2`; breaking changes require versioning.

## Diagnostics behavior

- Diagnostics are emitted to `stderr` to preserve `stdout` contracts.
- Use `--log-format jsonl` for machine-filterable diagnostics.
- Use `--log-level` to tune verbosity (`error`, `warn`, `info`, `debug`).
- Sensitive values are redacted from diagnostic output.

Example:

```bash
bb --json --log-level warn --log-format jsonl auth status 2> diagnostics.jsonl
```

## Recommended scripting pattern

1. Use `--json` and parse only the `data` payload needed for automation.
2. Keep diagnostics in separate stderr capture.
3. Validate bulk artifacts against published schemas when integrating with CI.
