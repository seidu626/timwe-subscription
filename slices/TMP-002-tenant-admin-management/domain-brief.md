# TMP-002 Domain Brief: Tenant Admin Management Scope

## Domain Grounding

Tenant admins manage product inventory and subscriber allow/block lists inside one tenant workspace. Product IDs and MSISDNs are external business identifiers, so they may repeat across tenants; the platform-owned tenant UUID is the isolation boundary.

## Actors

- tenant-admin: lists and mutates products, userbase rows, import jobs, and activity logs for the current tenant.
- platform-operator: creates tenants and later migrates legacy global rows into a default tenant.
- acquisition/subscription services: downstream consumers that will bind tenant products to campaigns in later slices.

## Story Craft

As a tenant-admin, I want to manage products and userbase records scoped to my tenant so that campaign and subscription setup uses only my tenant's offer inventory and subscriber base.

Primary journey:
1. Trusted auth middleware resolves tenant identity from accepted Auth0/service claims.
2. Admin handler resolves the active tenant through the tenant registry.
3. Product and userbase filters receive the tenant UUID before reaching repositories.
4. Mutations write rows with tenant_id and append tenant-tagged admin activity logs.
5. Import jobs and row errors remain visible only to the tenant that created them.

## Invariants

- No admin management repository query may list, read, update, delete, or import without a tenant_id.
- Raw tenant headers are not accepted as authorization context.
- Duplicate product_id and msisdn are unique only within a tenant boundary.
- Import files cannot override tenant_id; JSON rows with tenant_id are rejected.
- Admin activity logs for tenant-scoped mutations carry tenant_id.

## Failure Modes Covered

- Missing accepted tenant identity returns the existing tenant-context error path.
- Unknown or inactive tenant resolves to the same forbidden response as TMP-001.
- Cross-tenant product/userbase/import reads become not-found by tenant-filtered SQL.
- Duplicate product creation maps unique violations to the admin conflict error.
- Invalid import rows are stored against the tenant-owned import job.

## Non-Blocking Constraint

Product dependency counting still checks current campaign/subscription references globally because campaign tenant binding is scheduled for TMP-005 and subscription routing for TMP-007. This is conservative: it may block deletion more often than ideal, but it does not leak or delete another tenant's data.
