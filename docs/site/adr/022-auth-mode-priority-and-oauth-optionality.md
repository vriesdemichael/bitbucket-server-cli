# ADR 022: Auth mode priority and OAuth optionality

This page is generated from `docs/decisions/*.yaml` by `task docs:export-adr-markdown`. Do not edit manually.

- Number: `022`
- Title: `Auth mode priority and OAuth optionality`
- Category: `architecture`
- Status: `accepted`
- Provenance: `guided-ai`
- Source: `docs/decisions/022-auth-mode-priority-and-oauth-optionality.yaml`

## Decision

Use a practical auth priority for runtime operations: CLI flags/env and stored credentials with token or basic auth are first-class and required for milestone delivery. OAuth is supported as an optional advanced mode and is explicitly non-blocking for local Docker/evaluation environments.

## Agent Instructions

Implement and maintain token/basic auth as the default operational path. Do not block milestone features on OAuth availability. If OAuth is added, keep it additive and preserve fallback behavior to token/basic. Document authentication mode in status output and troubleshooting guidance.

## Rationale

Bitbucket Server/Data Center OAuth setup is not consistently available in local or restricted environments and often requires extra administrative configuration. Token/basic auth provides reliable behavior for live testing and day-to-day development while keeping a path open for stronger auth deployments.

## Rejected Alternatives

- `OAuth-only authentication`: Incompatible with many local/eval setups and increases onboarding friction.
- `No OAuth support at all`: Prevents future enterprise integrations where OAuth is mandated.
