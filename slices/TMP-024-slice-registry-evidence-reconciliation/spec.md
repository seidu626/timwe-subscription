# TMP-024 Spec

## Objective

Reconcile slice registry metadata so TMP-022 and TMP-023 point to their own accepted verification evidence.

## Broken Behavior

- TMP-022 is marked done but its verification block points at `cd common && go test ./...`.
- TMP-022 DoD path points at TMP-023's value-gate report.
- TMP-023 has accepted handoff and value-gate PASS evidence, but the manifest leaves it planned with empty verification.

## Expected Behavior

- TMP-022 automated verification is `cd services/landing-web && npm run build`.
- TMP-022 DoD path is `slices/TMP-022-landing-web-dynamic-route-build/value-gate-report.md`.
- TMP-023 state is `done`.
- TMP-023 automated verification is `cd common && go test ./...`.
- TMP-023 DoD path is `slices/TMP-023-common-package-test-failures/value-gate-report.md`.

## Acceptance Proof

```bash
jq empty slices/manifest.json
jq '.slices[] | select(.id=="TMP-022" or .id=="TMP-023")' slices/manifest.json
slice-harness status
slice-harness sync --dry-run
hvc check agent/backlog/issues/*.md --fail-on block
```
