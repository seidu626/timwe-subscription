# TMP-049 Domain Brief: Acquisition Campaign Slug Migration Startup

## Actors

- Platform operator: starts `acquisition-api` against an existing subscription database and expects schema bootstrap to be idempotent.
- Acquisition API runtime: runs admin management schema migrations before serving tenant acquisition endpoints.
- Legacy acquisition data model: still contains foreign keys from `campaign_slug` columns to `campaigns(slug)`.

## Ubiquitous Language

- Legacy campaign slug foreign key: any PostgreSQL foreign key that references `campaigns(slug)` before tenant-scoped campaign identity replaces global slug identity.
- Global campaign slug uniqueness: the old `campaigns_slug_key` constraint that prevents duplicate campaign slugs across tenants.
- Tenant acquisition flow migration: `add_tenant_zz_acquisition_flow.sql`, which adds tenant ownership to acquisition transactions and tenant-scoped acquisition indexes.

## Domain Invariants

- Tenant-owned duplicate campaign slugs require removing global slug uniqueness.
- Removing global slug uniqueness must first remove all explicit dependencies on `campaigns(slug)`.
- Startup schema bootstrap must not use broad `CASCADE` drops because it should not delete unrelated database objects.
- Tests must not connect to or mutate the operator's live database.

## Failure Mode

Operation: acquisition-api startup schema bootstrap.

- Broken outcome: startup exits while running `add_tenant_zz_acquisition_flow.sql` because `campaigns_slug_key` cannot be dropped while legacy foreign keys still depend on it.
- Expected behavior: the migration discovers and drops every legacy foreign key that references `campaigns(slug)`, then drops `campaigns_slug_key` and proceeds with tenant-scoped acquisition indexes.

## User Journey

1. Platform operator starts `acquisition-api`.
2. Runtime connects to PostgreSQL.
3. Admin management schema bootstrap runs acquisition tenant migrations.
4. Migration removes legacy slug foreign keys explicitly.
5. Migration removes the obsolete global slug uniqueness constraint.
6. Runtime continues startup instead of exiting.

