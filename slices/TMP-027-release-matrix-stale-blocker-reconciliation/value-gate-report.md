# TMP-027 Value Gate Report

- Timestamp: 2026-05-09T02:09:58Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:verified

## Audit 1: Acceptance Criteria Coverage

- Subscription-partner default tests pass: COVERED by `cd services/subscription-partner && go test ./...`.
- Notification default tests pass: COVERED by `cd services/notification && go test ./...`.
- Canonical local service build passes: COVERED by `make build-all-local`.
- Stale dependency/vendor blocker removed from TMP-021: COVERED by release matrix and TMP-021 value-gate update.
- Current blockers remain visible: COVERED by blocked checks and blocking gates.

Audit 1 result: PASS.

## Audit 2: Failure Mode Coverage

- Historical failure retained after current pass: COVERED by removing the stale blocker.
- Unrelated blockers hidden: COVERED by retaining compose runtime, webspa-admin, dependency vulnerability, and local-main divergence blockers.
- Generated binary committed: COVERED by restoring `services/notification/notification-worker` after `make build-all-local` and checking git status.

Audit 2 result: PASS.

## Audit 3: Domain Invariant Preservation

- Release evidence reflects current source truth: PRESERVED.
- Retiring one blocker does not imply full readiness: PRESERVED.
- No product/dependency/vendor changes: PRESERVED.

Audit 3 result: PASS.

## Commands

```bash
cd services/subscription-partner && go test ./...
cd services/notification && go test ./...
make build-all-local
make clean
git restore --source=HEAD -- services/notification/notification-worker
jq empty slices/manifest.json agent/state/TMP-027.work-order.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness sync --dry-run
```

Result: PASS.

