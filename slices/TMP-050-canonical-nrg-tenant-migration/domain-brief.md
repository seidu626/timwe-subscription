# TMP-050 Domain Brief: Canonical nrg Tenant Migration

## Actors

- Platform operator: runs tenant migration dry-run and apply commands during a migration window.
- Existing production data: rows created before tenant ownership was mandatory.
- Canonical tenant: `nrg`, the sole default tenant owner for existing tenantless rows.

## Ubiquitous Language

- Canonical tenant: the one tenant that owns pre-tenant production rows.
- Tenantless row: a row where `tenant_id IS NULL`.
- Backfill: batched update that assigns tenantless rows to the canonical tenant.
- Compatibility path: any active path that preserves tenantless production ownership or restores rows to `tenant_id = NULL`.

## Domain Invariants

- Existing tenantless production data must be assigned to `nrg`.
- Rows that already have `channel_id` but lack `tenant_id` are still tenantless and must be assigned to `nrg`.
- Active migration tooling must not keep rollback-to-null behavior.
- Dry-run must remain read-only.

## Architecture Notes

- Module: tenant platform migration script.
- Interface: `make db-migrate-tenant-platform-dry-run` and `make db-migrate-tenant-platform`.
- Implementation: Bash/SQL migration runner that reports tenantless counts, upserts `nrg`, and backfills rows in batches.
- Depth: the Make targets hide DB connection details, table inventory, conflict checks, and batching behind two commands.
- Locality: canonical tenant defaults and eligibility predicates now live in one migration module rather than scattered rollback or compatibility branches.

## Prune Classification

- Candidate: `legacy-default` tenant migration path.
- Class: `collapse_into_canonical`.
- Action: replaced with `nrg`.
- Deletion test: keeping rollback-to-null would preserve the tenantless data path, so it was removed from active tooling.

## User Journey

1. Platform operator runs `make db-migrate-tenant-platform-dry-run`.
2. Script reports `canonical_tenant_key=nrg`, tenantless row counts, and duplicate conflicts.
3. Platform operator runs `make db-migrate-tenant-platform`.
4. Script upserts `nrg`.
5. Script backfills every configured `tenant_id IS NULL` row to `nrg`.
6. Frontend bootstrap admin defaults resolve the `nrg` workspace.

