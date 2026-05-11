# TMP-051 Domain Brief: Tenant Catalog Admin UI and API

## Actors

- Operator: authenticated admin with platform scope who can list and update tenant catalog records.
- Tenant-scoped admin: authenticated admin bound to one tenant who must not list or mutate other tenant catalog records.
- Tenant catalog: the authoritative tenant record set used by admin APIs and workspace selection.

## Ubiquitous Language

- Tenant catalog: records in `tenants` with `tenant_key`, `name`, `status`, `default_country`, and metadata.
- Operator scope: trusted platform-scoped identity from Auth0/bootstrap claims.
- Tenant lifecycle state: `ACTIVE` or `INACTIVE`, enforced by service validation and database constraints.
- Tenant workspace guard: frontend route protection that keeps admin routes inside an accepted tenant workspace.

## Domain Invariants

- Only operator-scoped identities can list or update tenant catalog records.
- Tenant-scoped identities receive `403 Forbidden` for tenant catalog list/update operations.
- Tenant updates are audit logged with the target tenant as the catalog entity.
- Tenant catalog UI stays inside the existing guarded admin layout.

## Architecture Notes

- Module: `AdminManagementService`.
- Interface: `ListTenants(identity, filter)` and `UpdateTenant(id, input, identity, actor, requestID)`.
- Implementation: repository-backed tenant list/update plus activity log creation.
- Depth: callers supply identity and intent; the module hides scope checks, validation, SQL, and audit behavior.
- Locality: tenant catalog API behavior lives beside existing tenant create/current behavior rather than a parallel tenant client.

## Prune Classification

- Candidate: separate tenant catalog admin module.
- Class: `collapse_into_canonical`.
- Action: extended the existing `AdminManagementService`/repository/handler path.
- Deletion test: a separate module would duplicate identity, validation, and audit interfaces already owned by admin management.

## User Journey

1. Operator opens the guarded Tenants route.
2. UI calls `GET /v1/admin/tenants`.
3. Backend authorizes platform scope and returns paged tenant catalog records.
4. Operator edits name, status, default country, or metadata.
5. UI calls `PATCH /v1/admin/tenants/{id}`.
6. Backend validates the patch, updates the tenant, and writes an audit log entry.
7. Tenant-scoped admins receive forbidden responses for list and update attempts.
