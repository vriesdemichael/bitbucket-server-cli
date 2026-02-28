# Scripts

Planned scripts:
- `seed_bitbucket.py`: create project/users/repos for deterministic live tests
- `reset_local_stack.ps1`: clear and recreate local state
- `wait_for_bitbucket.py`: poll readiness endpoint before live test execution

Not implemented in minimal scaffold.

Coverage tooling:
- Local coverage gate is implemented as Go tool: `go run ./tools/coverage-gate`
- Task integration:
	- `task test:unit:coverage` generates `.tmp/coverage.out`
	- `task quality:coverage` enforces global + patch thresholds
