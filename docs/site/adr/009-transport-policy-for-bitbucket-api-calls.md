# ADR 009: Transport policy for Bitbucket API calls

This page is generated from `docs/decisions/*.yaml` by `task docs:export-adr-markdown`. Do not edit manually.

- Number: `009`
- Title: `Transport policy for Bitbucket API calls`
- Category: `architecture`
- Status: `accepted`
- Provenance: `guided-ai`
- Source: `docs/decisions/009-transport-policy-for-bitbucket-api-calls.yaml`

## Decision

Centralize all HTTP transport behavior in a shared client package that provides authentication injection, timeout defaults, retry/backoff policy, rate-limit handling, and pagination primitives.

## Agent Instructions

Do not implement ad-hoc HTTP behavior in service or workflow packages. New endpoints must use the shared transport interfaces and error mapping. Keep retry behavior explicit and safe for idempotent operations.

## Rationale

Bitbucket behavior and reliability concerns should be handled once and reused everywhere. Centralization prevents subtle drift in auth, timeout, and pagination logic.

## Rejected Alternatives

- `Per-service custom HTTP clients`: Produces inconsistent behavior and duplicated reliability logic.
