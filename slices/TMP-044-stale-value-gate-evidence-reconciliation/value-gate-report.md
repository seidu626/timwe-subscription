# TMP-044 Value Gate Report

- Timestamp: 2026-05-09T10:18:00Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:passed

## Acceptance Coverage

| Criterion | Status | Evidence |
|---|---|---|
| TMP-006 and TMP-012 landing-web notes are superseded by current proof | PASS | `cd services/landing-web && npm ci` passed; `cd services/landing-web && npm run build` passed and listed `/api/campaigns/[tenant]`, `/api/campaigns/[tenant]/[slug]`, `/lp/[tenant]`, and `/lp/[tenant]/[slug]`. |
| TMP-018 notification module-hygiene note is superseded by current proof | PASS | `cd services/notification && go test ./...` passed: 18 tests across 11 packages. |
| Historical notes remain visible | PASS | Reports append a "Current superseding evidence" section instead of deleting original results. |
| Approval-gated dependency remediation remains blocked | PASS | `npm ci` still reports one moderate and one high vulnerability and recommends force remediation; no package manifest or lockfile changed. TMP-037 remains the approval gate. |
| No product source or runtime file changes | PASS | File-scope review is limited to evidence, manifest, task, and state artifacts. |

## Commands

```bash
cd services/notification && go test ./...
cd services/landing-web && npm ci
cd services/landing-web && npm run build
jq empty slices/manifest.json agent/state/TMP-044.work-order.json agent/state/TMP-044.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness sync --dry-run
agent-supervisor --config /tmp/timwe-supervisor-artifact-20260509-101738.json preflight
```

## Result

PASS for stale-evidence reconciliation. Full-system release readiness remains blocked by TMP-021 child blockers and approval gates.
