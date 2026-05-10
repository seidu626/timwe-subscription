# TMP-044 Stale Value-Gate Evidence Reconciliation Spec

## Story

As a platform operator, I can see current superseding verification evidence in completed value-gate reports, so stale missing-dependency notes do not obscure current release-readiness facts.

## Acceptance Criteria

- TMP-006 records that current `services/landing-web` locked install and production build pass.
- TMP-012 records that current `services/landing-web` locked install and production build pass.
- TMP-018 records that current `services/notification` package tests pass.
- Historical blocker notes remain visible as historical evidence.
- Dependency vulnerability approval remains blocked under TMP-037 rather than silently remediated.
- No source, schema, compose, dependency manifest, lockfile, credential, submodule, or branch-integration files change.

## Failure Modes

- Stale note removed instead of annotated: fails audit because historical evidence is lost.
- Current command not rerun: fails audit because the superseding claim lacks live proof.
- Dependency remediation attempted: fails audit because TMP-037 requires explicit approval.
- Source files changed: fails file-scope review.

## Evidence

- `cd services/landing-web && npm ci`
- `cd services/landing-web && npm run build`
- `cd services/notification && go test ./...`
- `slices/TMP-044-stale-value-gate-evidence-reconciliation/value-gate-report.md`
