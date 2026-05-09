# TMP-039 Spec

## Objective

Add missing domain grounding artifacts for manifest-backed operational verification slices that already have specs and value-gate reports.

## Broken Behavior

TMP-021, TMP-024, TMP-025, TMP-026, and TMP-033 have slice specs and value-gate evidence, but no `domain-brief.md`. That leaves actor, outcome, invariant, entrypoint, and risk grounding implicit for those operational slices.

## Expected Behavior

- `slices/TMP-021-full-system-verification/domain-brief.md` exists.
- `slices/TMP-024-slice-registry-evidence-reconciliation/domain-brief.md` exists.
- `slices/TMP-025-tmp021-metadata-reconciliation/domain-brief.md` exists.
- `slices/TMP-026-webspa-submodule-verification/domain-brief.md` exists.
- `slices/TMP-033-tmp032-ledger-state-reconciliation/domain-brief.md` exists.
- TMP-039 records the reconciliation with issue, work order, manifest, task, value-gate, and handoff evidence.
- No product source, runtime, schema, compose, dependency, package, or branch-integration files change.

## Acceptance Proof

```bash
test -f slices/TMP-021-full-system-verification/domain-brief.md
test -f slices/TMP-024-slice-registry-evidence-reconciliation/domain-brief.md
test -f slices/TMP-025-tmp021-metadata-reconciliation/domain-brief.md
test -f slices/TMP-026-webspa-submodule-verification/domain-brief.md
test -f slices/TMP-033-tmp032-ledger-state-reconciliation/domain-brief.md
jq empty slices/manifest.json agent/state/TMP-039.work-order.json agent/state/TMP-039.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
agent-supervisor preflight with worktree-local temp config
agent-supervisor auto-loop --max-rounds 1 with worktree-local temp config
git diff --check
git diff --name-only
```
