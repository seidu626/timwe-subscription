# TMP-027 Domain Brief

## Actors

- Platform operator: reviews release-readiness evidence and decides whether the tenant-platform can be released or needs blocked follow-up work. Source: `slices/TMP-021-full-system-verification/slice.yaml`.
- Verification agent: runs current build, test, harness, and artifact checks and records evidence without changing product behavior. Source: `agent/backlog/issues/TMP-027-release-matrix-stale-blocker-reconciliation.md`.

## Ubiquitous Language

- Release matrix: the tracked full-system verification artifact at `docs/agent/full-system-verification-2026-05-09.md`.
- Blocker: a current condition that prevents full verification, such as missing runtime env, unavailable submodule commit, or approval-gated dependency/security work.
- Stale blocker: a previously true failure that current source evidence no longer reproduces.
- Canonical local build: `make build-all-local`, the Makefile target that compiles local service binaries.

## Domain Invariants

- Release-readiness evidence must reflect current source, not historical failure rows. Enforced by rerunning current commands before changing TMP-021 evidence.
- Retiring one blocker must not hide unrelated blockers. Enforced by preserving compose runtime, webspa-admin, dependency vulnerability, and local-main divergence gates.
- Evidence reconciliation must not mutate product code, dependency metadata, vendor trees, package manifests, lockfiles, or frontend source. Enforced by file scope and `git diff` review.

## Failure Modes

- Stale failure remains listed: operator may spend time on already-resolved dependency/vendor work.
- Overcorrection: matrix could imply full release readiness even though runtime env and submodule checks remain blocked.
- Generated artifact drift: `make build-all-local` writes service binaries; generated binary changes must be cleaned before commit.
- Harness drift: adding TMP-027 can leave manifest, `.agent/tasks.json`, and supervisor ledger out of sync unless preflight repair is run.

## User Journey

1. Platform operator opens the release matrix.
2. Verification agent runs current service tests and canonical build.
3. Release matrix shows subscription-partner and notification build/test checks as passed.
4. Release matrix still shows current non-local blockers for runtime env, webspa-admin submodule commit, dependency vulnerability approval, and local-main divergence.

Failure path:
1. If any rerun command fails, TMP-027 remains blocked and TMP-021 keeps the relevant blocker.
2. If generated binaries remain modified, the slice fails scope control.

## Open Questions

- Whether the local primary `main` branch should be reconciled with `origin/main` remains an operator integration decision.
- Whether to upgrade Next/PostCSS dependencies remains an approval-gated dependency/security decision.

