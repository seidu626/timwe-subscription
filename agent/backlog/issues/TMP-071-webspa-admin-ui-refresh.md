---
id: TMP-071
title: webspa-admin operator UI refresh
class: vertical_slice
status: ready
scope_limit: "Improve the existing Angular webspa-admin shell, dashboard, and tenant catalog UI only; do not change backend APIs, auth flow, tenant resolution behavior, dependency manifests, database schema, migrations, or deployment configuration."
merge_policy: "Merge only after HVC, focused frontend build, and supervisor preflight pass."
non_goals:
  - "No backend behavior changes."
  - "No dependency manifest or lockfile changes."
  - "No API URL, Auth0, tenant resolution, database, migration, or deployment changes."
evidence_required:
  - "webspa-admin dashboard renders the existing KPI data in a clearer operator command-center layout"
  - "tenant catalog create/edit and member management preserve existing actions while improving hierarchy and scanability"
  - "global shell/header/sidebar polish keeps existing CoreUI and Angular Material dependencies"
  - "npm run build"
  - "hvc check agent/backlog/issues/TMP-071-webspa-admin-ui-refresh.md --fail-on block"
acceptance_tests:
  - "hvc check agent/backlog/issues/TMP-071-webspa-admin-ui-refresh.md --fail-on block"
  - "cd frontend/webspa-admin && npm run build"
actor: tenant operations admin
outcome: "The admin user can scan acquisition health, tenant workspace state, and tenant catalog operations faster from the existing webspa-admin surfaces."
entrypoint: "Authenticated webspa-admin routes under /dashboard and /tenants"
trigger: "Admin opens the portal after authentication or navigates to the tenant catalog."
broken_outcome: "The UI is functionally present but visually generic, card-heavy, and inconsistent across CoreUI and Angular Material surfaces, making key operator actions harder to scan."
expected_behavior: "The existing shell, dashboard, and tenant catalog retain all behavior while presenting a cohesive, high-density, production-grade operations interface."
desired_outcome: "A distinctive but restrained operator UI with clearer hierarchy, tabular numeric treatment, responsive layout, dark-mode compatibility, and no generic AI-style filler."
change_layers:
  - webspa-admin
  - angular-ui
  - design-system-css
verification_layers:
  - angular-build
  - hvc-classification
blocked_by: []
blocks: []
parallel_group: webspa-admin-ui
file_scope:
  allowed:
    - "frontend/webspa-admin/src/app/layout/default-layout/default-layout.component.html"
    - "frontend/webspa-admin/src/app/layout/default-layout/default-layout.component.scss"
    - "frontend/webspa-admin/src/app/layout/default-layout/default-header/default-header.component.html"
    - "frontend/webspa-admin/src/app/layout/default-layout/default-header/default-header.component.scss"
    - "frontend/webspa-admin/src/app/layout/default-layout/_nav.ts"
    - "frontend/webspa-admin/src/app/views/dashboard/dashboard.component.html"
    - "frontend/webspa-admin/src/app/views/dashboard/dashboard.component.scss"
    - "frontend/webspa-admin/src/app/views/dashboard/dashboard.component.ts"
    - "frontend/webspa-admin/src/app/views/pages/login/login.component.html"
    - "frontend/webspa-admin/src/app/views/pages/login/login.component.scss"
    - "frontend/webspa-admin/src/app/features/tenant/tenant-list/tenant-list.component.html"
    - "frontend/webspa-admin/src/app/features/tenant/tenant-list/tenant-list.component.scss"
    - "frontend/webspa-admin/src/scss/_custom.scss"
    - "frontend/webspa-admin/src/scss/_theme.scss"
    - "agent/backlog/issues/TMP-071-webspa-admin-ui-refresh.md"
    - "agent/state/TMP-071.work-order.json"
    - "agent/state/TMP-071.handoff.json"
    - ".agent/**"
    - ".harness/**"
    - ".agent-work/**"
  forbidden:
    - "services/**"
    - "frontend/webspa-admin/package.json"
    - "frontend/webspa-admin/package-lock.json"
    - "frontend/webspa-admin/src/environments/**"
    - "frontend/webspa-admin/src/app/core/**"
    - "docker-compose*.yml"
---

# TMP-071: webspa-admin operator UI refresh

## Actor / outcome

Tenant operations admins can scan acquisition performance and tenant catalog state quickly from the existing admin UI.

## Scope

Allowed:
- Existing webspa-admin shell/header/sidebar styles.
- Existing dashboard route markup/styles using current KPI fields.
- Existing tenant catalog markup/styles with unchanged create/edit/member actions.
- HVC, harness, and Mythos packet artifacts for this slice.

Forbidden:
- Backend services, API contracts, Auth0 behavior, tenant workspace resolution, dependency manifests, environment files, migrations, and deployment configuration.

## Acceptance

- Dashboard uses current KPI response fields with no invented metrics.
- Tenant catalog preserves current create/update/filter/member actions.
- Shell polish uses existing CoreUI/Angular Material components and assets.
- HVC and Angular build pass.
