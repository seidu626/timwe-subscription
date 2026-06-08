---
id: TMP-039
title: "Operational slice domain brief reconciliation"
class: operational_slice
status: done
scope_limit: "Add missing domain grounding evidence for manifest-backed operational verification slices. Do not change product source, runtime behavior, schemas, compose files, dependencies, package manifests, or branch state."
merge_policy: "Merge only after HVC, slice-harness, supervisor preflight, JSON validity, value-gate evidence, and file-scope checks pass."
evidence_required:
  - "slices/TMP-021-full-system-verification/domain-brief.md"
  - "slices/TMP-024-slice-registry-evidence-reconciliation/domain-brief.md"
  - "slices/TMP-025-tmp021-metadata-reconciliation/domain-brief.md"
  - "slices/TMP-026-webspa-submodule-verification/domain-brief.md"
  - "slices/TMP-033-tmp032-ledger-state-reconciliation/domain-brief.md"
  - "slices/TMP-039-operational-domain-brief-reconciliation/value-gate-report.md"
acceptance_tests:
  - "test -f slices/TMP-021-full-system-verification/domain-brief.md"
  - "test -f slices/TMP-024-slice-registry-evidence-reconciliation/domain-brief.md"
  - "test -f slices/TMP-025-tmp021-metadata-reconciliation/domain-brief.md"
  - "test -f slices/TMP-026-webspa-submodule-verification/domain-brief.md"
  - "test -f slices/TMP-033-tmp032-ledger-state-reconciliation/domain-brief.md"
  - "jq empty slices/manifest.json agent/state/TMP-039.work-order.json agent/state/TMP-039.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Release-verification evidence has domain grounding for every manifest-backed operational slice that previously lacked it."
entrypoint: "slices/TMP-021, TMP-024, TMP-025, TMP-026, and TMP-033 domain-brief.md files"
trigger: "Verifier audits operational slice evidence completeness after supervisor reports no ready tasks."
broken_outcome: "Operational verification slices can have specs and value gates without actor, outcome, invariant, entrypoint, and risk grounding."
expected_behavior: "Each reconciled slice has a concise domain brief tied to its existing issue, spec, and value-gate evidence."
system_path:
  - "Verifier inspects slice evidence directories."
  - "Missing domain-brief.md files are identified for operational slices."
  - "Domain briefs are added without changing runtime behavior."
  - "Control-plane and value-gate checks prove the reconciliation is metadata-only."
change_layers:
  - evidence
  - harness
verification_layers:
  - control-plane
  - metadata
blocked_by: []
blocks: []
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-039-operational-domain-brief-reconciliation.md"
    - "agent/state/TMP-039.work-order.json"
    - "agent/state/TMP-039.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-039-operational-domain-brief-reconciliation/**"
    - "slices/TMP-021-full-system-verification/domain-brief.md"
    - "slices/TMP-024-slice-registry-evidence-reconciliation/domain-brief.md"
    - "slices/TMP-025-tmp021-metadata-reconciliation/domain-brief.md"
    - "slices/TMP-026-webspa-submodule-verification/domain-brief.md"
    - "slices/TMP-033-tmp032-ledger-state-reconciliation/domain-brief.md"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**"
    - "common/**"
    - "frontend/**"
    - "ops/**"
    - "docker-compose*.yml"
    - "Makefile"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
    - ".git/**"
---

## Operator Story

As a platform operator, I can inspect release-verification slices and see domain grounding for each operational evidence slice so release-readiness claims are tied to actors, outcomes, invariants, entrypoints, and risks.

## Acceptance Criteria

- TMP-021, TMP-024, TMP-025, TMP-026, and TMP-033 each have a `domain-brief.md`.
- Each added domain brief names actor, business outcome, domain invariant, entrypoint, trigger, and risk.
- TMP-039 records the reconciliation through issue, work order, slice spec, value gate, and handoff evidence.
- No product source, runtime, schema, compose, dependency, package, or branch-integration files change.
