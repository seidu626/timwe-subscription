# TMP-025 Spec

## Objective

Reconcile TMP-021 release verification metadata with the accepted blocked handoff and full-system value-gate evidence.

## Broken Behavior

- TMP-021 manifest state is `done` while `.agent/tasks.json` says `blocked`.
- TMP-021 automated verification is `cd services/landing-web && npm run build`, which belongs to TMP-022.
- TMP-021 DoD path points to TMP-022 value-gate evidence.
- TMP-021 value-gate report still says `PENDING` even though its accepted handoff is blocked.

## Expected Behavior

- TMP-021 manifest state is `blocked`.
- TMP-021 automated verification lists release-matrix evidence commands.
- TMP-021 DoD path is `slices/TMP-021-full-system-verification/value-gate-report.md`.
- TMP-021 value-gate report verdict is `BLOCKED` with `outcome:blocked`.

## Acceptance Proof

```bash
jq empty slices/manifest.json
jq '.slices[] | select(.id=="TMP-021")' slices/manifest.json
test -f slices/TMP-021-full-system-verification/value-gate-report.md
slice-harness status
slice-harness sync --dry-run
hvc check agent/backlog/issues/*.md --fail-on block
```
