---
id: TMP-057
title: "Report tenant-key scope resolution"
class: vertical_defect_slice
status: queued
parent_vertical_slice_id: TMP-010
scope_limit: "Fix acquisition-api admin reporting so a tenant-scoped identity with a tenant key can resolve the tenant report scope without allowing cross-tenant aggregation."
merge_policy: "Merge only after HVC, supervisor preflight, focused acquisition handler tests, and live acquisition-api restart verification pass."
evidence_required:
  - "Browser or HTTP evidence shows GET /v1/admin/reports/kpis returns 403 Forbidden for a tenant workspace request."
  - "go test ./internal/handler -run TestParseFilters"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
acceptance_tests:
  - "Tenant-scoped report requests with only tenant_key resolve to the canonical tenant_id before report queries run."
  - "all_tenants=true remains forbidden for non-platform identities."
  - "Unknown tenant keys remain forbidden and do not fall back to global reports."
actor: tenant-admin
outcome: "tenant admin dashboard KPIs load for the selected tenant workspace instead of failing with report-scope 403."
entrypoint: "GET /v1/admin/reports/kpis"
trigger: "Web admin loads dashboard KPIs with Authorization and X-Tenant-Key."
broken_outcome: "Reports parseFilters rejects a tenant-scoped identity that has tenant_key but no tenant_id with tenant_id_required."
expected_behavior: "Reports resolve the tenant key to a tenant_id and keep report SQL scoped to that tenant."
reproduction:
  command: "Open http://localhost:4200 dashboard; observe GET http://localhost:8084/v1/admin/reports/kpis?startDate=2026-04-11&endDate=2026-05-11"
  observed: "403 Forbidden from acquisition-api."
  expected: "200 OK or a data-source error after tenant scope is resolved; never a global unscoped report."
system_path:
  - "webspa-admin attaches X-Tenant-Key for workspace API calls."
  - "acquisition-api validates the Auth0 bearer token."
  - "Reports handler builds tenant-scoped ReportFilters."
  - "Reports repository applies tenant predicates to KPI queries."
change_layers:
  - backend-authz
  - backend-reporting
  - tests
verification_layers:
  - handler-tests
  - harness
  - runtime-smoke
blocked_by: []
blocks:
  - "TMP-021"
parallel_group: acquisition-reporting
non_goals:
  - "Do not loosen all_tenants authorization for tenant users."
  - "Do not add frontend token or tenant claim workarounds."
  - "Do not introduce global fallback reporting when tenant lookup fails."
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-057-report-tenant-key-scope.md"
    - "agent/state/TMP-057.work-order.json"
    - "agent/state/TMP-057.handoff.json"
    - "services/acquisition-api/internal/handler/reports_handler.go"
    - "services/acquisition-api/internal/handler/reports_handler_test.go"
    - "slices/manifest.json"
    - "slices/TMP-057-report-tenant-key-scope/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "frontend/**"
    - "docker-compose*.yml"
---

## Operator Story

As a tenant admin, I can open the admin dashboard and see KPI reports for my selected tenant workspace when my authenticated identity carries a tenant key.

## Acceptance Criteria

- Tenant-key-only report identity resolves through the tenant catalog to a tenant ID.
- Tenant users cannot request `all_tenants=true`.
- Unknown tenant keys return 403 instead of an unscoped report.
