# TMP-011 Value Gate Report

Verdict: PASS

## Criteria Coverage

| criterion_id | test_file | test_name | assertion_type | verdict | evidence |
| --- | --- | --- | --- | --- | --- |
| AC-1 | `scripts/db-migrate-tenant-platform.sh` | `run_dry_run` | static script review | PASS | Dry-run prints per-table eligible counts, blocked channel-scope counts, duplicate-group counts, `default_tenant_present`, and `readiness=READY_FOR_APPLY` or `BLOCKED` while using a read-only default-tenant lookup. |
| AC-2 | `scripts/db-migrate-tenant-platform.sh` | `apply_migration` | static script review | PASS | Apply uses batched `UPDATE ... WHERE tenant_id IS NULL` / `channel_id IS NULL` backfill, inserts or reuses `tenant_key=legacy-default`, and aborts before writes when duplicate/conflict checks fail. |
| AC-3 | `scripts/db-migrate-tenant-platform.sh` | `rollback_migration` | static script review | PASS | Rollback updates only rows currently on the default tenant back to `NULL` and deletes the default tenant row when no references remain. |
| AC-4 | `docs/tenant-platform-migration-runbook.md` | `verification_queries` | documentation review | PASS | Runbook names the entrypoints, verification SQL, and the legacy-default tenant evidence path used by the migration operator. |

## Failure / Edge Coverage

- Unmapped legacy row: covered by dry-run eligibility counts and the `readiness` summary.
- Constraint violation: covered by duplicate-group checks for campaigns, products, subscriptions, and cadence series.
- Large table lock risk: covered by batched apply and rollback loops with `BATCH_SIZE` defaulting to 500.
- Rollback required: covered by the rollback mode and orphan-default-tenant cleanup.

## Invariants

- Existing single-tenant compatibility remains nullable during this slice: covered by the fact that no `NOT NULL` enforcement is added.
- Re-runs are idempotent: covered by the `tenant_id IS NULL` / `tenant_id = default_tenant_id` predicates in apply and rollback.

## Evidence

- `make db-migrate-tenant-platform-dry-run`
- `make db-migrate-tenant-platform`
- `make db-rollback-tenant-platform`
- `docs/tenant-platform-migration-runbook.md`
- Throwaway PostgreSQL smoke: `SMOKE_OK initial_dry_run_blocked apply_idempotent dry_run_ready rollback_restored`
