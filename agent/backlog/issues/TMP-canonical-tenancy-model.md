---
id: TMP-canonical-tenancy-model
issue_id: ISSUE-CREATE-A-DOCS-ONLY-BOUNDED-ENABLER-FOR-T
work_order_id: WO-CREATE-A-DOCS-ONLY-BOUNDED-ENABLER-FOR-T2-DRAFT
goal_id: goal-20260608T064113Z-create-a-docs-only-bound
title: "Canonical tenancy model RFC"
class: bounded_enabler
status: queued
scope_limit: "Draft the proposed canonical tenancy model RFC and supporting docs/control-plane artifacts only. Do not approve the RFC or change services, frontend, migrations, dependencies, deployment, env, or secrets."
merge_policy: "Merge only after docs diff review, whitespace check, schema validate-all, reconcile check, and result capsule evidence pass."
evidence_required:
  - "docs/architecture/canonical-tenancy-model.md defines tenant identity, tenant/channel hierarchy, admin membership source of truth, platform-vs-tenant scope, onboarding lifecycle, credential-reference boundaries, resolver precedence, migration/enforcement gates, and downstream WorkOrders."
  - "Existing tenant docs are reconciled as source inputs or superseded fragments."
  - "git diff --check"
  - "agent-hub schema validate-all --root . --json"
  - "agent-hub reconcile --root . --check --json"
acceptance_tests:
  - "RFC is clearly marked proposed/draft and not approved."
  - "RFC records cross-runtime peer review as unavailable unless a repo-enabled non-Codex reviewer is found."
  - "No backend, frontend, migration, dependency, deployment, env, or secret files are changed."
actor: platform-operator
outcome: "downstream backend/frontend tenant-management workers have a proposed canonical tenancy model to review before implementation."
entrypoint: "docs/architecture/canonical-tenancy-model.md"
trigger: "T2 canonical tenancy model bounded enabler is dispatched before tenant-management implementation WorkOrders consume conflicting tenancy fragments."
system_path:
  - "Read the T2 context pack."
  - "Reconcile tenant source docs."
  - "Draft the proposed RFC."
  - "Record peer-review gate status."
  - "Run docs/control-plane verification."
change_layers:
  - docs-architecture
  - backlog-control-plane
  - slice-control-plane
verification_layers:
  - whitespace
  - schema
  - reconcile
consumed_by:
  - TMP-048
  - TMP-051
  - TMP-055
  - TMP-057
  - TMP-065
  - TMP-071
  - TMP-074
parallel_group: T2-canonical-tenancy-model
non_goals:
  - "Do not approve the RFC."
  - "Do not implement tenant resolver, membership, credential, frontend, migration, or deployment changes."
  - "Do not add or rotate secrets."
file_scope:
  allowed:
    - "docs/architecture/canonical-tenancy-model.md"
    - "docs/admin-tenant-account-mapping.md"
    - "docs/tenant-channel-onboarding.md"
    - "docs/tenant-platform-migration-runbook.md"
    - "docs/tenant-nullability-enforcement-plan.md"
    - "agent/backlog/issues/TMP-canonical-tenancy-model.md"
    - "slices/T2-canonical-tenancy-model/**"
    - ".harness/runs/parallel-T2-rfc-20260608-0742/**"
  forbidden:
    - "services/**"
    - "frontend/**"
    - "common/**"
    - "scripts/**"
    - "ops/**"
    - "docker-compose*.yml"
    - "go.mod"
    - "go.sum"
    - "package.json"
    - "package-lock.json"
    - "**/.env"
    - "secrets/**"
---

## Operator Story

As a platform operator, I can review one proposed tenancy model before downstream tenant-management WorkOrders implement resolver, membership, credential, UI, reporting, and enforcement behavior from inconsistent fragments.

## Acceptance

- The RFC is present at `docs/architecture/canonical-tenancy-model.md`.
- The RFC is marked proposed/draft and not approved.
- The RFC reconciles the existing tenant docs as source inputs or superseded fragments.
- The RFC lists downstream WorkOrders that consume the decision.
- Verification evidence is recorded in the T2 result capsule.
