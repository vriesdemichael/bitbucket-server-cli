# Local Bitbucket stack

This folder contains the local stack for behavior-accurate integration testing.

## Target
- Atlassian Bitbucket Data Center/Server: `9.4.16`
- PostgreSQL backend

## Notes
- First startup can take several minutes.
- A valid Bitbucket license (trial/eval is fine) is required during setup.
- Keep this stack for local/live tests only; unit tests must not depend on it.
- Compose loads `../.env` and passes `BITBUCKET_LICENSE_KEY` into the Bitbucket container for setup automation scripts.

## Licensing
- Bitbucket Server/Data Center is proprietary Atlassian software (not open source).
- This local stack is for licensed evaluation/development use and must follow Atlassian's EULA/terms.
- Ensure your team handles license activation, renewal, and usage limits appropriately.

## Planned commands
- Start: `docker compose -f docker/compose.yml up -d`
- Stop: `docker compose -f docker/compose.yml down`
- Reset state: remove `docker/bitbucket/data` and `docker/postgres/data`

Not executed by scaffold step.
