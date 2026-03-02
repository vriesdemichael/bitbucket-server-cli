# Quality Coverage Artifacts

This directory contains committed quality artifacts used for local pre-push and GitHub Actions verification.

## Files

- `coverage-report.json`: deterministic snapshot generated from combined unit + live coverage and operation mapping metrics.
- `generated-operation-contracts.json`: mapping of generated client operations used by hand-written services to test files that provide contract coverage.

## Workflow

- Update artifacts before push: `task quality:coverage:report:update`
- Local verify (recompute + compare): `task quality:coverage:report:verify`
- CI verify (artifact threshold check only): `task quality:coverage:report:verify:committed`

## Notes

- Live integration tests are not run in GitHub Actions due Bitbucket license/infrastructure constraints.
- This report-driven workflow keeps CI reproducible by validating committed local quality outputs.
