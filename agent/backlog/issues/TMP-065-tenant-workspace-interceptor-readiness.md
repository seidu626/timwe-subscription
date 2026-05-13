---
id: TMP-065
title: Tenant workspace interceptor readiness
class: vertical_defect_slice
status: ready
scope_limit: "Fix only webspa-admin tenant workspace HTTP readiness and the denial copy for backend workspace denials; do not change backend services, Auth0 configuration, dependency manifests, API URLs, database schema, migrations, or deployment configuration."
merge_policy: "Merge only after HVC, focused frontend tests, admin build, whitespace check, and supervisor preflight pass."
evidence_required:
  - "tenant-workspace.interceptor spec proves loading requests wait for a ready workspace"
  - "page403 spec proves forbidden/not-found copy is not reported as missing assignment"
  - "npm run build"
  - "hvc check agent/backlog/issues/TMP-065-tenant-workspace-interceptor-readiness.md --fail-on block"
acceptance_tests:
  - "hvc check agent/backlog/issues/TMP-065-tenant-workspace-interceptor-readiness.md --fail-on block"
  - "cd frontend/webspa-admin && npm run build"
actor: tenant admin user
outcome: "The Angular admin app opens protected tenant-scoped pages for an account that already has the NRG tenant workspace."
entrypoint: "Protected admin routes and their initial tenant-scoped HTTP requests from http://localhost:4200"
trigger: "Admin navigates directly to a protected admin page immediately after authentication or page reload."
broken_outcome: "The first workspace API request can be forwarded while the tenant workspace state is still loading, omits X-Tenant-Key, receives a backend tenant-context denial, and lands the user on /403 even though the UI later shows assigned tenant NRG."
expected_behavior: "Tenant-scoped HTTP requests wait until the workspace is no longer loading before forwarding, so assigned-tenant requests include X-Tenant-Key on the first attempt."
desired_outcome: "A user with assigned tenant NRG reaches protected admin pages without a false tenant-workspace-unavailable screen, and real backend denials use accurate denial copy."
reproduction:
  command: "Start webspa-admin, authenticate, and navigate directly to a protected admin route while tenant workspace resolution is still loading."
  observed: "The denial page says Tenant workspace unavailable while the same page shows Assigned tenant NRG / nrg."
  expected: "The protected route's initial API requests include X-Tenant-Key: nrg, or real backend denials explain forbidden/not-found instead of missing assignment."
system_path:
  - "tenantWorkspaceGuard waits for tenant workspace readiness."
  - "TenantWorkspaceInterceptor currently takes the first workspace emission and can forward a loading state."
  - "HttpErrorInterceptor redirects workspace 403/404 responses to /403."
  - "Page403Component renders denial copy from route reason."
change_layers:
  - webspa-admin
  - tenant-workspace-http
verification_layers:
  - frontend-unit-test
  - angular-build
blocked_by: []
blocks: []
parallel_group: admin-tenant-workspace
file_scope:
  allowed:
    - "frontend/webspa-admin/src/app/core/http-interceptors/tenant-workspace.interceptor.ts"
    - "frontend/webspa-admin/src/app/core/http-interceptors/tenant-workspace.interceptor.spec.ts"
    - "frontend/webspa-admin/src/app/views/pages/page403/page403.component.ts"
    - "frontend/webspa-admin/src/app/views/pages/page403/page403.component.spec.ts"
    - "agent/backlog/issues/TMP-065-tenant-workspace-interceptor-readiness.md"
    - "agent/state/TMP-065.work-order.json"
    - "agent/state/TMP-065.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**"
    - "frontend/webspa-admin/package.json"
    - "frontend/webspa-admin/package-lock.json"
    - "frontend/webspa-admin/src/environments/**"
    - "frontend/webspa-admin/src/app/core/guards/**"
    - "docker-compose*.yml"
---

# TMP-065: Tenant workspace interceptor readiness

## Actor / outcome

Tenant-scoped admin users with an assigned tenant can open protected admin pages without first hitting the tenant workspace denial screen.

## Problem

The route guard waits for the tenant workspace service to finish resolving, but the HTTP interceptor forwards workspace API requests immediately. Early admin data requests can leave while the workspace state is still `loading`, so they omit `X-Tenant-Key`. Backends then return tenant-context 403/404 responses and the error interceptor navigates to `/403`, even though the workspace stream later shows an assigned tenant.

## Scope

Allowed:
- `frontend/webspa-admin/src/app/core/http-interceptors/tenant-workspace.interceptor.ts`
- `frontend/webspa-admin/src/app/core/http-interceptors/tenant-workspace.interceptor.spec.ts`
- `frontend/webspa-admin/src/app/views/pages/page403/page403.component.ts`
- `frontend/webspa-admin/src/app/views/pages/page403/page403.component.spec.ts`
- `.agent/**`
- `agent/backlog/issues/TMP-065-tenant-workspace-interceptor-readiness.md`
- `agent/state/TMP-065.work-order.json`
- `agent/state/TMP-065.handoff.json`

Forbidden:
- Backend services
- Database migrations
- Auth0 tenant assignment model
- API endpoint URLs
- Dependency manifests
- Build and deployment configuration

## Acceptance

- A workspace API request emitted while `workspace.loading` is true waits for the first non-loading workspace state before forwarding.
- The forwarded request includes `X-Tenant-Key` when a current tenant is available.
- The 403 page does not describe backend forbidden/not-found denials as a missing account tenant assignment.
- Focused admin tests and build pass.
