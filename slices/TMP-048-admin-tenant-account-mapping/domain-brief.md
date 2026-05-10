# TMP-048 Domain Brief: Admin Tenant Account Mapping

## Actors

- Platform operator: grants all-tenant admin access and needs a safe way to recover admin workspace access for approved principals. Source: `agent/backlog/issues/TMP-048-admin-tenant-account-mapping.md`.
- Tenant admin: opens `frontend/webspa-admin` with a tenant assignment and must remain constrained to that tenant. Source: `slices/TMP-014-admin-portal-tenant-workspace/domain-brief.md`.
- Auth0 authenticated admin: supplies JWT/profile claims consumed by `TenantWorkspaceService` and `auth0jwt.Claims`. Source: `frontend/webspa-admin/src/app/core/services/tenant-workspace.service.ts`, `common/auth/auth0jwt/claims.go`.
- Admin API client: sends selected tenant headers after workspace resolution. Source: `frontend/webspa-admin/src/app/core/http-interceptors/tenant-workspace.interceptor.ts`.

## Ubiquitous Language

- Tenant workspace: active tenant context used by admin UI and API calls. Source: `frontend/webspa-admin/src/app/core/services/tenant-workspace.service.ts`.
- Platform-scoped identity: admin identity allowed to operate across tenants, derived from platform roles/permissions or verified bootstrap email. Source: `common/auth/tenantctx/identity.go`, `services/acquisition-api/internal/transport/admin.go`.
- Bootstrap admin: explicitly configured email that receives platform scope before a database membership subsystem exists. Source: `docs/admin-tenant-account-mapping.md`.
- Selected tenant context: `X-Tenant-Key` / `X-Tenant-Id` headers attached by the frontend after platform tenant selection. Source: `frontend/webspa-admin/src/app/core/http-interceptors/tenant-workspace.interceptor.ts`.

## Domain Invariants

- Missing tenant assignment must not fall back to global data for ordinary tenant admins. Source: `slices/TMP-014-admin-portal-tenant-workspace/domain-brief.md`.
- Selected tenant headers are trusted only after the authenticated identity is platform scoped. Source: `services/acquisition-api/internal/transport/admin.go`.
- Tenant-scoped and unscoped identities cannot escalate by sending tenant headers. Source: `services/acquisition-api/internal/transport/admin_test.go`.
- New user account mapping remains Auth0 claim-based until a dedicated membership slice introduces a database model. Source: `docs/admin-tenant-account-mapping.md`.

## Failure Modes

- Operation: Resolve frontend workspace
  - Missing tenant: ordinary authenticated user with no tenant or bootstrap mapping gets `missing-tenant`.
  - Unknown bootstrap tenant list: bootstrap email without configured tenant workspaces still cannot enter a tenant workspace.
  - Multiple tenants: platform user must select one tenant before tenant-specific routes are ready.
- Operation: Authorize backend tenant context
  - Unscoped token plus tenant header: header is ignored and tenant routes still fail tenant-context checks.
  - Tenant-scoped token plus alternate tenant header: header is ignored because the token is not platform scoped.
  - Platform token without selection: platform-only operations may proceed, but tenant-specific operations still require tenant context.

## User Journey

1. Platform operator configures bootstrap admin emails and tenant workspaces.
2. Named admin authenticates through Auth0.
3. `webspa-admin` recognizes the email, marks the user platform-scoped, and loads configured tenant workspaces.
4. Admin selects a tenant when multiple workspaces are configured.
5. API requests carry the selected tenant header.
6. `acquisition-api` applies the selected tenant only because the identity is platform scoped.

Failure journeys:

1. Ordinary user sends `X-Tenant-Key` without platform scope -> backend ignores the header and tenant-specific handlers return tenant-context denial.
2. Bootstrap admin has multiple tenants and no selection -> frontend remains in `selection-required`.
3. New user has no Auth0 tenant claims or platform marker -> frontend shows missing tenant and does not enter protected workspace.

## Open Questions

- The repo has no live tenant catalog endpoint consumed by `webspa-admin`; production tenant workspace lists must come from Auth0 metadata or runtime bootstrap config until a future slice adds database-backed admin membership and tenant discovery.
