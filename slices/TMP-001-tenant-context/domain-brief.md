# TMP-001 Domain Brief

## Actors

- Platform operator: creates tenant roots and can see platform-scoped tenant state. Sources: `slices/TMP-001-tenant-context/slice.yaml`, `common/auth/tenantctx/identity.go`.
- Tenant admin: resolves only the tenant assigned by accepted admin-auth claims. Sources: `common/auth/auth0jwt/claims.go`, `services/acquisition-api/internal/transport/admin.go`.
- Acquisition API admin service: protected admin surface that owns the first tenant walking skeleton. Sources: `services/acquisition-api/internal/transport/router.go`, `services/acquisition-api/internal/handler/admin_management_handler.go`.

## Ubiquitous Language

- Tenant: new root scope for later products, campaigns, channels, reports, and operations. TO BE DEFINED by TMP-001 as a row with `id`, `tenant_key`, `name`, `status`, `default_country`, and public-safe metadata.
- Tenant key: normalized, unique, human-readable tenant reference used when Auth0 claims do not yet carry the UUID. Source: `slices/TMP-001-tenant-context/slice.yaml`.
- Tenant context: `tenantctx.Identity` attached by admin auth middleware after JWT validation; direct raw tenant headers are not trusted by TMP-001 admin routes. Sources: `common/auth/tenantctx/identity.go`, `services/acquisition-api/internal/transport/admin.go`.
- Platform scoped: role or permission such as `platform_operator` or `platform:all_tenants` that allows global tenant creation. Source: `common/auth/tenantctx/identity.go`.
- Admin activity log: auditable record in `admin_activity_logs` for admin mutations. Source: `services/acquisition-api/migrations/add_admin_management_tables.sql`.

## Domain Invariants

- Tenant creation is platform-only: tenant-scoped admins cannot create sibling tenants. Enforced by handler/service identity checks from `tenantctx.Identity`.
- Tenant key is normalized and unique: one canonical key maps to one tenant. Enforced by service normalization plus a database unique constraint.
- Tenant create is auditable atomically: a tenant row must not commit without the matching `admin_activity_logs` row. Enforced by repository transaction.
- Protected current-tenant lookup has no global fallback: missing tenant context fails before lookup and raw tenant headers are ignored on direct admin service routes.
- Unknown and inactive tenant-scoped current lookups do not reveal existence differences to tenant actors. Enforced by same external response status/body shape.

## Failure Modes

- Create tenant
  - Invalid input: missing/invalid key, name, status, country, or metadata returns 400.
  - Missing required: no platform-scoped identity returns 403 and performs no mutation.
  - Duplicate/conflict: duplicate normalized tenant key returns 409 without leaking database driver text.
  - Dependency failure: audit insert or DB transaction failure rolls back tenant insert and returns 500.
  - Concurrent access: database uniqueness makes one create succeed and the other conflict.
  - Authorization: tenant-scoped admin cannot create tenants.
- Resolve current tenant
  - Invalid input: malformed tenant claim or unknown tenant returns a fail-closed response.
  - Missing required: no tenant claim/key in accepted identity returns 403 with no fallback.
  - Duplicate/conflict: not applicable for read path.
  - Dependency failure: repository lookup error returns 500 without default tenant behavior.
  - Concurrent access: tenant status changes between auth and lookup must be checked at lookup time.
  - Authorization: inactive and unknown tenant-scoped responses do not disclose tenant existence.

## User Journey

1. Platform operator calls `POST /v1/admin/tenants` with a platform-scoped admin JWT and tenant payload.
2. Acquisition API validates platform scope, normalizes the tenant key, inserts the tenant, and inserts the audit log in the same transaction.
3. API returns `201` with tenant id, key, status, and audit log reference.
4. Tenant admin calls `GET /v1/admin/tenants/current` with a JWT carrying `tenant_id` or `tenant_key`.
5. API reads `tenantctx.Identity`, resolves the tenant, rejects inactive or unknown tenants without fallback, and returns the tenant DTO.

Failure journeys:

1. Tenant admin posts tenant creation -> `403`, no tenant row, no audit row.
2. Direct client sends raw `X-Tenant-Id` without JWT tenant claim -> current endpoint rejects and performs no lookup.
3. Platform operator repeats a tenant key -> `409`, no duplicate row, no SQL driver string in response.
4. Tenant claim points to inactive or unknown tenant -> same external response shape for tenant actors.

## Open Questions

- Whether Auth0 Organizations should become the source of truth after the walking skeleton.
- Whether tenant metadata needs a strict schema in TMP-002 or later.
- Whether platform operators should receive a distinct inactive-vs-unknown diagnostic endpoint later.
