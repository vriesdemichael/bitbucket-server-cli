# ADR 033: Automated Conventional Commit release on main

This page is generated from `docs/decisions/*.yaml` by `task docs:export-adr-markdown`. Do not edit manually.

- Number: `033`
- Title: `Automated Conventional Commit release on main`
- Category: `development`
- Status: `accepted`
- Supersedes: `007`
- Provenance: `guided-ai`
- Source: `docs/decisions/033-automated-conventional-commit-release-on-main.yaml`

## Decision

Automate releases from GitHub Actions on push events to main (including PR merges) using deterministic Semantic Versioning derived from Conventional Commits. The release workflow computes the next version, skips publication when no releasable Conventional Commit exists, builds cross-platform artifacts, generates checksums, keyless Sigstore/cosign signatures with Rekor-backed bundles for every published artifact, provenance attestations, creates or updates the tag/release, and publishes both markdown release notes and a machine-readable changelog artifact.

## Agent Instructions

Treat .github/workflows/release.yml as an automated post-merge release pipeline for main. Version bump rules are: major for breaking changes ("!" or BREAKING CHANGE footer), minor for feat, patch for other valid Conventional Commit types. Preserve deterministic behavior (no interactive/manual release decisions in normal flow), keep idempotent handling for existing tags/releases, maintain dual changelog outputs (human markdown plus machine-readable JSON), and keep the release signing identity pinned to refs/heads/main because self-update verifies the signed checksum manifest against that exact GitHub Actions workflow identity. Manual workflow_dispatch version input is allowed only as an explicit override for controlled backfills/hotfixes.

## Rationale

The project already enforces Conventional Commits and CI gates, making deterministic post-merge release automation predictable and auditable without introducing external release orchestration complexity. This reduces operator toil and release timing ambiguity while preserving safety through branch protection and required checks on PRs before merge. Publishing Rekor-backed keyless signatures for the checksum manifest and archives lets clients hard-fail self-update on publisher identity mismatches instead of trusting release metadata alone, while idempotent publication and explicit fallback override keep operations robust when retries or exceptional release corrections are needed.

## Rejected Alternatives

- `Keep release workflow manual-only via workflow_dispatch`: Adds avoidable operator overhead and inconsistent release timing after merge.
- `Use external release orchestration tooling (for example release-please)`: Increased debugging complexity and lower operational predictability for this repository's preferred setup.
- `Publish on every push regardless of commit semantics`: Violates Conventional Commit version intent and increases accidental/noise releases.
