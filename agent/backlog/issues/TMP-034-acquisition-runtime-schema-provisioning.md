---
id: TMP-034
title: "Acquisition runtime schema provisioning"
class: vertical_defect_slice
status: done
scope_limit: "Classify, approve, and resolve the acquisition-api runtime schema provisioning blocker discovered after TMP-030. Runtime implementation is carried by TMP-045."
merge_policy: "Merge after HVC, supervisor preflight, value-gate evidence, TMP-045 implementation evidence, and file-scope checks pass."
evidence_required:
  - "targeted acquisition-api runtime probe"
  - "docs/agent/full-system-verification-2026-05-09.md"
  - "slices/TMP-030-acquisition-compose-build-context/value-gate-report.md"
  - "slices/TMP-034-acquisition-runtime-schema-provisioning/value-gate-report.md"
  - "slices/TMP-045-compose-runtime-schema-bootstrap/value-gate-report.md"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "test -f slices/TMP-034-acquisition-runtime-schema-provisioning/value-gate-report.md"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status"
  - "slice-harness sync --dry-run"
actor: platform-operator
outcome: "Acquisition API starts in the compose runtime with base products/userbase schema available before admin migrations run."
entrypoint: "docker compose acquisition-api runtime startup"
trigger: "Verifier runs bounded compose runtime smoke after TMP-030 image build fix."
broken_outcome: "Acquisition API exits during admin schema bootstrap because add_admin_management_tables.sql expects relation products in the empty compose DB."
expected_behavior: "The compose DB schema provisioning path creates or migrates products and userbase before add_admin_management_tables.sql runs, so acquisition-api reaches health checks."
reproduction:
  command: "targeted acquisition-api runtime probe after TMP-030 compose image build fix"
  observed: "Container exits during admin schema bootstrap: failed to execute admin schema migration add_admin_management_tables.sql because relation products does not exist."
  expected: "Acquisition API starts, applies required schema in order, and responds to /health."
system_path:
  - "Full-system verifier reads the release matrix blocker."
  - "Blocker is classified into a concrete slice."
  - "Operator sees the approval or decision gate before implementation."
  - "Future implementation can run the listed acceptance proof after the gate is cleared."
change_layers:
  - harness
  - evidence
verification_layers:
  - control-plane
  - metadata
blocked_by: []
blocks:
  - "TMP-021"
parallel_group: release-verification-blockers
file_scope:
  allowed:
  - "agent/backlog/issues/TMP-034-acquisition-runtime-schema-provisioning.md"
  - "agent/state/TMP-034.work-order.json"
  - "agent/state/TMP-034.handoff.json"
  - "slices/manifest.json"
  - "slices/TMP-034-acquisition-runtime-schema-provisioning/**"
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
---

## Operator Story

As a platform-operator, I can see TMP-034 as a distinct blocked slice so the full-system verification backlog does not hide this blocker inside prose.

## Acceptance Criteria

- Schema/migration change approval is recorded before implementation.
- TMP-045 reruns a targeted clean PostgreSQL bootstrap proof for acquisition-api prerequisites.
- Registry slice source scope remains evidence-only; runtime implementation is isolated to TMP-045.
