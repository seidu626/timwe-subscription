---
id: TMP-043
title: "Release decision ADR templates"
class: operational_slice
status: done
scope_limit: "Create pending ADR templates for the remaining approval-gated release blockers. Do not record approvals and do not change schema, migrations, runtime code, compose files, dependencies, package manifests, credentials, submodule contents, or branch state."
merge_policy: "Merge only after HVC, slice-harness, supervisor preflight, JSON validity, value-gate evidence, and file-scope checks pass."
evidence_required:
  - "slices/decisions/TMP-026-webspa-admin-source-reproducibility.md"
  - "slices/decisions/TMP-034-acquisition-runtime-schema-provisioning.md"
  - "slices/decisions/TMP-035-notification-message-outbox-schema.md"
  - "slices/decisions/TMP-036-postback-outbox-schema.md"
  - "slices/decisions/TMP-037-landing-web-dependency-remediation.md"
  - "slices/decisions/TMP-038-local-main-integration-strategy.md"
  - "slices/TMP-043-release-decision-templates/value-gate-report.md"
acceptance_tests:
  - "test -f slices/decisions/TMP-026-webspa-admin-source-reproducibility.md"
  - "test -f slices/decisions/TMP-034-acquisition-runtime-schema-provisioning.md"
  - "test -f slices/decisions/TMP-035-notification-message-outbox-schema.md"
  - "test -f slices/decisions/TMP-036-postback-outbox-schema.md"
  - "test -f slices/decisions/TMP-037-landing-web-dependency-remediation.md"
  - "test -f slices/decisions/TMP-038-local-main-integration-strategy.md"
  - "jq empty slices/manifest.json agent/state/TMP-043.work-order.json agent/state/TMP-043.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "The remaining release-verification approvals have fillable pending ADR templates without implying approval."
entrypoint: "docs/agent/release-decision-packet-2026-05-09.md"
trigger: "Decision packet identifies approval artifacts as the next gate."
broken_outcome: "The packet says approvals can be recorded in ADRs, but no release-blocker ADR templates exist."
expected_behavior: "Each remaining approval-gated blocker has a pending ADR template with context, choices, consequences, and post-decision proof."
system_path:
  - "Verifier reads the release decision packet."
  - "Verifier creates pending ADR templates for each blocked decision area."
  - "Operator fills one template to approve, defer, or reject a path."
change_layers:
  - evidence
  - harness
verification_layers:
  - control-plane
  - metadata
blocked_by: []
blocks:
  - "TMP-021"
  - "TMP-026"
  - "TMP-034"
  - "TMP-035"
  - "TMP-036"
  - "TMP-037"
  - "TMP-038"
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-043-release-decision-templates.md"
    - "agent/state/TMP-043.work-order.json"
    - "agent/state/TMP-043.handoff.json"
    - "slices/manifest.json"
    - "slices/decisions/README.md"
    - "slices/decisions/TMP-026-webspa-admin-source-reproducibility.md"
    - "slices/decisions/TMP-034-acquisition-runtime-schema-provisioning.md"
    - "slices/decisions/TMP-035-notification-message-outbox-schema.md"
    - "slices/decisions/TMP-036-postback-outbox-schema.md"
    - "slices/decisions/TMP-037-landing-web-dependency-remediation.md"
    - "slices/decisions/TMP-038-local-main-integration-strategy.md"
    - "slices/TMP-043-release-decision-templates/**"
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
    - "*.sql"
    - ".git/**"
---

## Operator Story

As a platform operator, I can fill a pending ADR template for any remaining release blocker, so approval is recorded before implementation begins.

## Acceptance Criteria

- Pending ADR templates exist for TMP-026, TMP-034, TMP-035, TMP-036, TMP-037, and TMP-038.
- Each template states `Approval recorded: no`.
- Each template includes context, decision required, consequences, post-decision proof, and slice impact.
- No runtime, source, schema, dependency, submodule, or branch-integration files change.
