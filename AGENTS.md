# AGENTS.md

## Setup & verification commands (use these exact commands)
- Install: `make setup`
- Lint: `make lint`
- Test: `make test`
- Typecheck: `make typecheck`

## Review rubric (P0/P1 only)
### P0 (must fix)
- Auth/authz bypass (missing middleware, object-level auth failures)
- Injection risks (SQL/command injection, SSRF), unsafe deserialization
- Secrets/PII leakage in logs or telemetry
- Data loss / irreversible migrations without rollback
- Broken builds / failing CI

### P1 (strongly recommended)
- Missing tests for behavior changes
- Likely correctness regressions / unhandled edge cases
- Major perf regressions (N+1 queries, unbounded loops)

## Change discipline
- Prefer minimal diffs (no unrelated refactors).
- Preserve public APIs unless explicitly required.
- If behavior changes, add tests that fail-before/pass-after.

## Architecture constraints (example)
- `src/core/**` must not import `src/adapters/**`
- Keep IO at the edges; business logic stays pure where possible.
