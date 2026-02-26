# Migration checklist from cog-lib-bitbucket

## Keep / port first
- repo creation flow (including webhook/default reviewer setup)
- pull request create/list/comment/delete pathways
- token-based auth handling

## Keep / port later
- advanced diff/commit retrieval features
- convenience wrappers not used by core workflows

## Defer initially
- full issue management if not required by immediate automation
- backward compatibility shims for every old function name

## Verification
- every ported behavior must have at least one live integration test
- add a regression test for each undocumented server quirk discovered
