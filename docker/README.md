# Local Bitbucket stack

This folder contains the local stack for behavior-accurate integration testing.

## Target
- Atlassian Bitbucket Data Center/Server: `9.4.16`
- PostgreSQL backend

## Notes
- First startup can take several minutes.
- A valid Bitbucket license (trial/eval is fine) is required during setup.
- Keep this stack for local/live tests only; unit tests must not depend on it.

## Planned commands
- Start: `docker compose -f docker/compose.yml up -d`
- Stop: `docker compose -f docker/compose.yml down`
- Reset state: remove `docker/bitbucket/data` and `docker/postgres/data`

Not executed by scaffold step.
