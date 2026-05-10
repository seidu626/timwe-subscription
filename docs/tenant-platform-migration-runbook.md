# Tenant Platform Migration Runbook

This runbook covers TMP-050, the canonical `nrg` tenant backfill used to move existing tenantless rows into tenant-isolated ownership.

## Entry Points

- `make db-migrate-tenant-platform-dry-run`
- `make db-migrate-tenant-platform`

## Dry Run

Dry run is read-only. It reports:

- canonical tenant readiness for `tenant_key=nrg`
- whether the canonical tenant already exists or will be created on apply
- per-table tenantless row counts
- per-table duplicate groups that would become conflicts after backfill
- overall readiness

Recommended command:

```bash
make db-migrate-tenant-platform-dry-run
```

The dry-run script does not update tenant ownership on any table rows and does not create the canonical tenant record.

## Apply

Apply runs a batched, idempotent backfill for tenantless rows.

Recommended command:

```bash
make db-migrate-tenant-platform
```

Implementation details:

- `nrg` is inserted if missing, or reused if already present
- rows are updated when `tenant_id IS NULL`
- rows with `channel_id` are still eligible when `tenant_id IS NULL`
- batches default to 500 rows and can be overridden with `BATCH_SIZE`
- the script aborts before changing data if duplicate/conflict checks fail

## Verification Queries

Use these checks when reviewing a release or a migration window:

```sql
SELECT tenant_key, id, status
FROM tenants
WHERE tenant_key = 'nrg';

SELECT 'campaigns' AS table_name, COUNT(*) AS tenantless_rows
FROM campaigns
WHERE tenant_id IS NULL
UNION ALL
SELECT 'subscriptions', COUNT(*)
FROM subscriptions
WHERE tenant_id IS NULL
UNION ALL
SELECT 'postback_outbox', COUNT(*)
FROM postback_outbox
WHERE tenant_id IS NULL;
```

Additional conflict checks used by the dry-run:

```sql
SELECT slug, COUNT(*)
FROM campaigns
WHERE tenant_id IS NULL
GROUP BY slug
HAVING COUNT(*) > 1;

SELECT partner_role_id, user_identifier, product_id, COUNT(*)
FROM subscriptions
WHERE tenant_id IS NULL
GROUP BY partner_role_id, user_identifier, product_id
HAVING COUNT(*) > 1;
```

## Operational Notes

- This slice does not enforce `NOT NULL` tenant constraints.
- The migration is safe to rerun after a partial run because the apply path only updates rows where `tenant_id IS NULL`.
- Rollback uses database backup restore or a git revert of the migration tooling, not an active codepath that restores tenantless production data.
