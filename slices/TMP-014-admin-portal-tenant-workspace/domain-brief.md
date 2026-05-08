# TMP-014 Domain Brief: Admin Portal Tenant Workspace

Post-hoc reconciliation note: this domain brief was added after implementation to align the shipped slice artifacts with the domain-grounding contract. It summarizes existing `slice.yaml`, issue, value-gate, and nested admin-portal evidence; it does not introduce new runtime scope.

## Actors

- Tenant admin: opens `frontend/webspa-admin` and must see only tenant-scoped navigation and resources. Source: `slices/TMP-014-admin-portal-tenant-workspace/slice.yaml`.
- Platform operator: may select or switch tenant context when platform-scoped permissions allow it. Source: `slices/TMP-014-admin-portal-tenant-workspace/slice.yaml`.
- Admin API client: carries tenant context and handles 403/404 tenant denials from backend services. Source: `slices/TMP-014-admin-portal-tenant-workspace/value-gate-report.md`.
- Auth0 login callback / workspace guard: resolves the active tenant workspace before admin pages load. Source: `slices/TMP-014-admin-portal-tenant-workspace/slice.yaml`.

## Ubiquitous Language

- Tenant workspace: the active tenant context shown in the admin shell and used by subsequent admin routes/API calls. Source: `slices/TMP-014-admin-portal-tenant-workspace/slice.yaml`.
- Route guard: frontend protection that blocks routes when tenant context is missing, disabled, or tampered. Source: `slices/TMP-014-admin-portal-tenant-workspace/value-gate-report.md`.
- Tenant selector: platform-operator affordance for choosing a tenant without exposing it to ordinary tenant admins. Source: `slices/TMP-014-admin-portal-tenant-workspace/slice.yaml`.
- Denial state: explicit 403/404/empty-state UI path when the backend rejects tenant access or membership is missing. Source: `slices/TMP-014-admin-portal-tenant-workspace/value-gate-report.md`.
- Nested admin checkout: `frontend/webspa-admin` is represented in the superproject as a gitlink to the verified admin implementation commit. Source: `slices/TMP-014-admin-portal-tenant-workspace/value-gate-report.md`.

## Domain Invariants

- Frontend tenant context must match backend tenant enforcement; UI may not display resource data after the API denies tenant access.
- Tenant admins cannot switch to or request another tenant through URL/query tampering.
- Missing tenant assignment must block resource list requests instead of falling back to global data.
- Disabled tenant state must be visible and bounded without leaking another tenant's workspace.
- Platform-operator tenant selection is allowed only for platform-scoped identity.

## Failure Modes

- URL tampering: tenant admin modifies route/query to tenant B -> guard or API handling renders denial/not-found.
- Missing membership: authenticated user has no tenant assignment -> workspace is blocked and resource calls are not issued.
- Stale tenant: tenant is disabled after login -> mutation clears workspace and surfaces disabled state.
- Backend denial: API returns 403/404 -> interceptor routes to denial handling instead of rendering stale data.
- Empty tenant: new tenant has no campaigns/products/userbase -> UI renders empty state without cross-tenant fallback.

## User Journey

1. Tenant admin completes login and opens the admin portal.
2. Workspace guard resolves tenant assignment and loads the current tenant workspace.
3. Header/navigation show tenant context and route only into tenant-scoped resources.
4. API client sends tenant-aware requests through the trusted auth/gateway path.
5. Denial, missing assignment, disabled tenant, or empty resource states render bounded UI states.

Failure journeys:

1. Tenant admin tampers with tenant route/query -> UI blocks access and does not show other-tenant data.
2. User has no tenant membership -> workspace guard blocks before list calls.
3. API returns tenant denial -> interceptor clears or redirects to a denial path.

## Open Questions

- The parent repository stores `frontend/webspa-admin` as a gitlink; the detailed implementation tests live in the nested admin repository at commit `2ad95b1`.
- A fuller visual redesign and BI dashboard expansion are out of scope; this slice only proves tenant workspace guardrails.
