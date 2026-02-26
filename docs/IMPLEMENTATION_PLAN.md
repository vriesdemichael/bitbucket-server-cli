# Implementation plan: `bitbucket-server-cli`

## 1) Goal and constraints

### Goal
Build a dependable Go client + CLI for Bitbucket Server/Data Center operations, validated against **real server behavior** (not only docs), targeting **Atlassian Bitbucket 9.4.16**.

### Constraints
- Keep this repo independent from `cog-lib-bitbucket` while reusing design lessons.
- Prefer deterministic local integration tests over shared/live external server tests.
- Preserve support for undocumented/quirky behavior by turning findings into executable tests.
- Do not run the full local stack by default in quick test runs.

---

## 2) Recommended architecture

Use a layered architecture that keeps wrappers, but narrows each layer’s responsibility.

### Layers
1. **Transport layer**
   - shared Go HTTP client
   - auth, timeout, retry policy, pagination primitives
2. **Bitbucket API services**
   - focused modules by domain (`repo`, `pr`, `issues`, `webhook`, `project`)
   - map raw responses to typed models
3. **Workflow layer**
   - higher-level operations (e.g., “create cookiecutter repo”)
4. **CLI layer**
   - command groups (`auth`, `repo`, `pr`, `issue`, `admin`)
   - maps args/options to workflow calls

### Why this still keeps wrapper benefits
You keep one public, stable interface while isolating undocumented behavior handling in dedicated parsing/normalization code + live tests.

---

## 3) What to keep from current repo

From existing `cog-lib-bitbucket`:
- Keep domain intent and command workflows around repo creation and PR automation.
- Keep strict typed model validation and schema export approach.
- Keep retry and throttling concepts (but centralize transport behavior).

---

## 4) What to drop or redesign

- Drop monolithic “all-in-one” service classes mixing HTTP, parsing, and workflow logic.
- Drop tests that depend on a shared corporate Bitbucket instance for routine CI.
- Redesign git interactions behind a backend interface (implementation can be swapped).

---

## 5) Live behavior testing strategy (primary)

## Test levels
1. **Unit tests (fast)**
   - model parsing, URL building, error mapping
   - no network
2. **Integration live tests (primary correctness gate for server behavior)**
   - runs against local Bitbucket 9.4.16 + Postgres via Docker
   - validates real API + auth + permissions + odd behavior
3. **Optional external environment smoke**
   - tiny suite against a remote sandbox if needed

## Test markers
- unit tests run in default local checks
- live tests run explicitly against local Bitbucket stack
- destructive live tests are isolated and opt-in

## Determinism tactics
- Seed known users/projects/repos before tests.
- Use unique per-test namespace/project keys where feasible.
- Provide reset script for state cleanup between runs.

---

## 6) Local stack for Bitbucket 9.4.16

### Compose services
- `postgres` (Bitbucket DB)
- `bitbucket` at `9.4.16`

### Notes
- Bitbucket startup can take several minutes.
- Requires enough memory/CPU and valid license (trial/eval acceptable for local dev).
- Keep app data as bind-mount for easier debugging/reset.

---

## 7) CLI structure (gh-inspired)

### Command groups
- `auth`: login/token/status/logout
- `repo`: create/list/get/delete/clone/labels/webhooks
- `pr`: list/view/create/comment/approve/decline/merge
- `issue`: list/view/create/comment/transition (if enabled)
- `admin`: local stack health/seed/reset

### Output principles
- consistent human output
- `--json` for machine usage
- predictable exit codes and error payloads

---

## 8) Migration phases

### Phase A: foundation
- scaffold package, CLI entrypoint, config model
- docker compose skeleton for local stack
- test markers + live test harness skeleton

### Phase B: repo + auth APIs
- implement project/repo CRUD and auth checks
- add live tests for repo create/get/list/delete and permission errors

### Phase C: PR APIs
- implement PR list/create/comments/changes/merge checks
- codify undocumented behavior with explicit test cases

### Phase D: workflow parity
- port current high-value workflows (cookiecutter repo creation etc.)
- keep backward-compatible command aliases where useful

### Phase E: hardening
- add retries/timeouts/telemetry hooks
- finalize CLI UX and JSON schemas

---

## 9) Initial keep/drop checklist

### Keep now
- typed models idea
- workflow semantics
- rich CLI ergonomics

### Drop now
- direct dependence on old monolithic wrappers
- live external server as default test target

### Add now
- local live stack
- seeded integration harness
- clear package boundaries

---

## 10) Definition of done for first milestone

First milestone is complete when:
- local Bitbucket 9.4.16 stack is documented and runnable
- `bbsc auth status` and `bbsc repo list` work against local instance
- at least one live integration test passes in manual/local run
- docs include reset + troubleshooting steps
