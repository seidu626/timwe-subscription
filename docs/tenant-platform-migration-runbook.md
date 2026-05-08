# Tenant Platform Migration Runbook

This runbook covers TMP-011, the legacy default-tenant backfill used to migrate global rows into tenant-isolated ownership without mutating unrelated UI or partner-contract surfaces.

## Entry points

- `make db-migrate-tenant-platform-dry-run`
- `make db-migrate-tenant-platform`
- `make db-rollback-tenant-platform`

## Dry run

Dry run is read-only from the data model point of view. It reports:

- default tenant readiness for `tenant_key=legacy-default`
- whether the default tenant already exists
- per-table eligible legacy row counts
- per-table rows that are blocked because they still have channel scope without tenant ownership
- per-table duplicate groups that would become conflicts after backfill
- overall readiness

Recommended command:

```bash
make db-migrate-tenant-platform-dry-run
```

The dry-run script does not update tenant ownership on any table rows and does not create the default tenant record.

## Apply

Apply runs a batched, idempotent backfill for eligible legacy rows only.

Recommended command:

```bash
make db-migrate-tenant-platform
```

Implementation details:

- `legacy-default` is inserted if missing, or reused if already present
- rows are updated only when `tenant_id IS NULL`
- for tables that also have `channel_id`, only rows with `channel_id IS NULL` are eligible
- batches default to 500 rows and can be overridden with `BATCH_SIZE`
- the script aborts before changing data if duplicate/conflict checks fail

## Rollback

Rollback restores nullable tenant compatibility by moving rows assigned to the default tenant back to `tenant_id = NULL`.

Recommended command:

```bash
make db-rollback-tenant-platform
```

Rollback behavior:

- rows backfilled to `legacy-default` are restored to nullable tenant ownership
- the default tenant row is deleted if no migrated rows still reference it
- the script remains safe to rerun because it only touches rows currently on the default tenant

## Verification queries

Use these checks when reviewing a release or a migration window:

```sql
SELECT tenant_key, id, status
FROM tenants
WHERE tenant_key = 'legacy-default';

SELECT 'campaigns' AS table_name, COUNT(*) AS eligible_rows
FROM campaigns
WHERE tenant_id IS NULL AND channel_id IS NULL
UNION ALL
SELECT 'subscriptions', COUNT(*)
FROM subscriptions
WHERE tenant_id IS NULL AND channel_id IS NULL
UNION ALL
SELECT 'postback_outbox', COUNT(*)
FROM postback_outbox
WHERE tenant_id IS NULL AND channel_id IS NULL;
```

Additional conflict checks used by the dry-run:

```sql
SELECT slug, COUNT(*)
FROM campaigns
WHERE tenant_id IS NULL AND channel_id IS NULL
GROUP BY slug
HAVING COUNT(*) > 1;

SELECT partner_role_id, user_identifier, product_id, COUNT(*)
FROM subscriptions
WHERE tenant_id IS NULL AND channel_id IS NULL
GROUP BY partner_role_id, user_identifier, product_id
HAVING COUNT(*) > 1;
```

## Operational notes

- This slice does not enforce `NOT NULL` tenant constraints.
- The migration is safe to rerun after a partial run because the apply path only updates rows where `tenant_id IS NULL`.
- The rollback path is likewise idempotent because it only touches rows currently assigned to `legacy-default`.
