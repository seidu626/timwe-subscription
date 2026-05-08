---
id: TMP-014
title: "Admin portal tenant workspace"
class: vertical_slice
status: done
parent_vertical_slice_id: TMP-014
scope_limit: "Implement tenant workspace route guards and tenant API denial handling in frontend/webspa-admin only. Do not change migration, ops hardening, or partner onboarding contracts."
merge_policy: "Merge only after HVC, supervisor preflight, UI verification evidence, and value-gate report pass."
evidence_required:
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "agent-supervisor --config .harness/config.json preflight"
  - "frontend/webspa-admin verification command if available"
acceptance_tests:
  - "jq empty slices/manifest.json"
  - "slice-harness status"
  - "cd frontend/webspa-admin && npm test -- --watch=false"
non_goals:
  - "No backend tenant migration enforcement."
  - "No credential or observability hardening."
actor: tenant-admin
outcome: "Tenant admins see and mutate only their tenant resources in the admin UI."
entrypoint: "frontend/webspa-admin routes and API client"
trigger: "Tenant admin opens the admin portal"
system_path:
  - "Frontend route guard resolves tenant workspace context."
  - "API client carries tenant context and handles 403/404 tenant denials."
  - "Empty or disabled tenant workspace renders a bounded state."
change_layers:
  - frontend
  - tests
  - docs
  - harness
verification_layers:
  - frontend
  - tests
blocked_by:
  - TMP-002
  - TMP-003
  - TMP-005
blocks: []
parallel_group: tenant-platform-ui
file_scope:
  allowed:
    - "frontend/webspa-admin/**"
    - "slices/TMP-014-admin-portal-tenant-workspace/**"
    - "agent/backlog/issues/TMP-014-admin-portal-tenant-workspace.md"
    - "agent/state/TMP-014.work-order.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**"
    - "Makefile"
---

## Operator story

As a tenant admin, I can use the admin portal from inside a tenant workspace and cannot view or mutate another tenant's resources.

## Acceptance criteria

- Tenant admins see tenant workspace context and tenant-scoped navigation only.
- URL tampering or missing tenant assignment produces explicit denial or empty-state behavior.
- Disabled tenant state is visible without exposing other tenant data.
- Value-gate report names tests or manual evidence for happy, failure, edge, and invariant criteria.
