# ADR 039: Built-in MCP server with explicit host scoping and token capability restriction

This page is generated from `docs/decisions/*.yaml` by `task docs:export-adr-markdown`. Do not edit manually.

- Number: `039`
- Title: `Built-in MCP server with explicit host scoping and token capability restriction`
- Category: `architecture`
- Status: `accepted`
- Provenance: `guided-ai`
- Source: `docs/decisions/039-built-in-mcp-server-with-host-scoping-and-token-restriction.yaml`

## Decision

Expose a curated set of high-value Bitbucket operations as MCP tools via bb ai mcp serve. The server uses stdio transport for IDE-native integration. When more than one Bitbucket server context is configured, --host is required and the server exits immediately with an actionable error if it is omitted. An optional --token flag scopes all API calls made by the server to the supplied PAT, restricting capabilities to that token's rights. An optional --tools allowlist and --exclude denylist allow further narrowing of the exposed tool surface. A companion bb ai mcp tools command lists all available tools with name and description to support allowlist/denylist construction.

## Agent Instructions

When generating MCP server configuration for an IDE (e.g. VS Code, Cursor), always emit bb ai mcp serve as the server command. If the user has multiple Bitbucket instances configured (detectable via bb auth server list), prompt for --host before generating the config snippet. Document the --token flag as the recommended way to run a read-only MCP server instance: instruct users to create a read-only PAT and pass it via --token. Never silently pick one host when multiple are configured; always surface the ambiguity.
Tier-1 tools to expose by default (highest need in autonomous workflows):
  get_pull_request, list_pr_comments, list_pr_tasks, get_build_status,
  resolve_ref, clone_repository, list_tags

Tier-2 tools (common in multi-step workflows, enabled by default):
  search_repositories, list_pull_requests, create_pull_request, add_pr_comment,
  list_branches, list_required_builds

Tier-3 tools (targeted; enabled by default but commonly excluded via --exclude):
  compare_refs, list_commits, get_commit, create_tag, set_build_status,
  submit_pr_review, merge_pull_request

## Rationale

Stdio transport is the canonical IDE-native MCP transport; HTTP would require port management and is not supported out of the box by most IDE MCP clients. Explicit --host enforcement prevents silent use of the wrong Bitbucket instance in multi-tenant setups, which is a real failure mode in enterprise environments. Token-scoped servers allow teams to run a read-only instance alongside a write-enabled instance in the same IDE session without additional access-control logic inside the server itself. The tools/exclude flags put capability control in the hands of the user without requiring code changes.

## Rejected Alternatives

- `HTTP transport instead of stdio`: Requires port management, firewall considerations, and is not IDE-native. Stdio is the standard for local MCP servers.
- `Implicitly select the active server context when multiple exist`: Silently targets the wrong instance in multi-tenant setups. The error is often discovered late and is hard to debug.
- `Per-tool token scoping`: Adds significant complexity. Scoping the entire server to one token is sufficient for the two-instance (read/write) pattern and is easier to reason about.
- `No tool filtering flags; users configure via separate config file`: Adds indirection. Flags at serve time are composable, scriptable, and don't require a config schema.
