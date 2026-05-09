---
id: TMP-044
title: "Stale value-gate evidence reconciliation"
class: operational_slice
status: done
scope_limit: "Reconcile completed value-gate reports whose historical verification notes are superseded by later successful full-system checks. Do not change product source, runtime behavior, schemas, compose files, dependency manifests, lockfiles, credentials, submodule contents, or branch state."
merge_policy: "Merge only after refreshed verification commands, HVC, slice-harness, supervisor preflight, JSON validity, value-gate evidence, and file-scope checks pass."
evidence_required:
  - "slices/TMP-044-stale-value-gate-evidence-reconciliation/value-gate-report.md"
  - "slices/TMP-006-tenant-acquisition-flow/value-gate-report.md"
  - "slices/TMP-012-public-tenant-routing/value-gate-report.md"
  - "slices/TMP-018-tenant-claim-and-service-auth-contract/value-gate-report.md"
acceptance_tests:
  - "cd services/landing-web && npm ci && npm run build"
  - "cd services/notification && go test ./..."
  - "jq empty slices/manifest.json agent/state/TMP-044.work-order.json agent/state/TMP-044.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness sync --dry-run"
  - "agent-supervisor preflight with worktree-local temp config"
actor: platform-operator
outcome: "TMP-006, TMP-012, and TMP-018 reports identify current superseding verification evidence instead of leaving stale missing-dependency or module-hygiene notes ambiguous."
entrypoint: "slices/*/value-gate-report.md"
trigger: "Full-system audit finds TMP-006, TMP-012, and TMP-018 reports with older blocked verification notes that later checks supersede."
system_path:
  - "Verifier scans completed slice value-gate reports for stale could-not-run or blocker notes."
  - "Verifier reruns current targeted checks for the affected surfaces."
  - "Verifier appends superseding evidence notes without rewriting historical results."
change_layers:
  - evidence
  - harness
verification_layers:
  - metadata
  - tests
  - frontend-build
blocked_by: []
blocks:
  - "TMP-021"
parallel_group: release-verification-metadata
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-044-stale-value-gate-evidence-reconciliation.md"
    - "agent/state/TMP-044.work-order.json"
    - "agent/state/TMP-044.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-006-tenant-acquisition-flow/value-gate-report.md"
    - "slices/TMP-012-public-tenant-routing/value-gate-report.md"
    - "slices/TMP-018-tenant-claim-and-service-auth-contract/value-gate-report.md"
    - "slices/TMP-044-stale-value-gate-evidence-reconciliation/**"
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

As a platform operator, I can see when stale value-gate notes have been superseded by current full-system evidence, so release readiness is not judged against obsolete missing-dependency or module-hygiene observations.

## Acceptance Criteria

- TMP-006 and TMP-012 value-gate reports keep their historical landing-web `node_modules` note and append current `npm ci` plus `npm run build` evidence.
- TMP-018 value-gate report keeps its historical notification module-hygiene note and appends current `services/notification` test evidence.
- The reconciliation slice records the current commands, results, and remaining vulnerability/approval caveat.
- No product source, schema, dependency manifest, lockfile, compose, credential, submodule, or branch-integration files change.
