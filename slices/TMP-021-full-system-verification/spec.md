# TMP-021 Release Verification Matrix Spec

## Story

As a platform operator, I can inspect a release verification matrix so that readiness is based on real build, test, runtime, feature, and blocked-check evidence.

## Domain Grounding

- Actor: platform-operator.
- Business outcome: the operator has an evidence-backed release-readiness matrix for every discovered runnable component and implemented tenant-platform feature.
- Domain invariant: full-system verification must distinguish passed, fixed, blocked, failed, not applicable, and not implemented states; build success alone must not imply feature readiness.
- Entrypoint: `docs/agent/full-system-verification-2026-05-09.md`.
- Trigger: operator requests end-to-end release verification.
- Risk: release readiness can be overstated if blocked runtime checks, missing submodules, dependency approval gates, or local/remote branch divergence are hidden.

## Acceptance Criteria

- Service inventory lists every discovered runnable component with canonical or derived build, test, start, and smoke commands.
- Feature inventory maps implemented tenant-platform features to source evidence, invariants, interfaces, and verification method.
- Verification matrix records command results with one of: passed, fixed, failed, blocked, not applicable, or not implemented.
- Control-plane drift, git divergence, runtime blockers, and environment limitations are documented explicitly.
- Value-gate report maps the audit criteria to concrete commands and artifacts.
- Follow-up defects or approval gates are tracked as narrower slices instead of being hidden in the release matrix.

## Failure Modes

- Local and remote branch histories diverge: record the integration check as blocked and require a maintainer-directed strategy.
- Clean admin frontend checkout is not reproducible: record the `frontend/webspa-admin` gitlink blocker and require a publish, repoint, or replacement decision.
- Compose runtime reaches service startup but lacks required schema: record the affected service, missing relation, and required provisioning decision.
- Dependency remediation requires a breaking upgrade: record the approval gate and the required UI regression proof.
- External services or credentials are unavailable: mark live-flow checks blocked or partially verified instead of using tests or builds as proxy proof.

## Evidence

- `docs/agent/full-system-verification-2026-05-09.md`
- `slices/TMP-021-full-system-verification/domain-brief.md`
- `slices/TMP-021-full-system-verification/slice.yaml`
- `slices/TMP-021-full-system-verification/value-gate-report.md`
- `agent/state/TMP-021.handoff.json`

## Out Of Scope

- Production deployment.
- Dependency additions or upgrades.
- Schema rewrites or migration ownership changes.
- Product feature implementation.
- Branch resets, destructive integration, or conflict resolution without maintainer direction.

## Current Verdict

TMP-021 remains blocked by TMP-026, TMP-034, TMP-035, TMP-036, TMP-037, and TMP-038. This spec closes the missing story/spec artifact gap only; it does not approve or implement any blocked release decision.
