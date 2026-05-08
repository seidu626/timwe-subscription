# TMP-011 Domain Brief: Migration And Data Isolation

Post-hoc reconciliation note: this domain brief was added after implementation to align the shipped slice artifacts with the domain-grounding contract. It summarizes existing `slice.yaml`, issue, script, runbook, and value-gate evidence; it does not introduce new runtime scope.

## Actors

- Platform operator: runs dry-run, apply, rollback, and verification commands for tenant-platform migration. Source: `slices/TMP-011-migration-and-data-isolation/slice.yaml`.
- Tenant admin: depends on legacy campaigns, products, subscriptions, notifications, postbacks, and cadence state remaining available after default-tenant backfill. Source: `slices/roadmap.md`.
- Existing Ghana/TIMWE subscriber flow: must keep working while legacy rows are assigned to `tenant_key=legacy-default`. Source: `docs/tenant-platform-migration-runbook.md`.
- Migration script: performs read-only readiness checks, batched backfill, and rollback for eligible global rows. Source: `scripts/db-migrate-tenant-platform.sh`.

## Ubiquitous Language

- Default tenant: compatibility tenant with `tenant_key=legacy-default` used to own pre-tenant production rows during migration. Source: `scripts/db-migrate-tenant-platform.sh`.
- Dry-run: non-mutating readiness mode that reports table counts, unmapped rows, duplicate groups, and whether apply is safe. Source: `Makefile`, `scripts/db-migrate-tenant-platform.sh`.
- Backfill: batched update of eligible rows with missing tenant/channel ownership into default-tenant ownership. Source: `scripts/db-migrate-tenant-platform.sh`.
- Rollback: compatibility path that clears default-tenant ownership from rows touched by this migration and removes the default tenant when unreferenced. Source: `scripts/db-migrate-tenant-platform.sh`.
- Constraint readiness: evidence that scoped uniqueness and not-null enforcement can be applied without breaking legacy data. Source: `docs/tenant-platform-migration-runbook.md`.

## Domain Invariants

- Dry-run must not mutate data; it only reads default-tenant presence, table counts, unmapped rows, and conflicts. Source: `scripts/db-migrate-tenant-platform.sh`.
- Migration must be idempotent: reruns update only still-unmapped eligible rows and preserve already backfilled rows. Source: `slices/TMP-011-migration-and-data-isolation/value-gate-report.md`.
- Legacy compatibility remains nullable in this slice; final `NOT NULL` enforcement is deferred until verification proves no unmapped rows remain. Source: `slices/TMP-011-migration-and-data-isolation/slice.yaml`.
- Rollback must only affect rows owned by the default tenant and must not erase non-default tenant ownership. Source: `scripts/db-migrate-tenant-platform.sh`.
- Evidence must include table-level row counts, conflict counts, tenant identity, and readiness state. Source: `docs/tenant-platform-migration-runbook.md`.

## Failure Modes

- Dry-run finds missing default tenant or unmapped rows: readiness reports `BLOCKED` and apply must not proceed.
- Duplicate scoped values exist under the default tenant: conflict counts are reported and constraint enforcement is blocked.
- Apply is interrupted mid-batch: rerun continues from remaining `NULL` ownership rows without changing already migrated rows.
- Rollback is needed after smoke failure: rollback restores nullable compatibility for default-tenant rows and preserves non-default tenant data.
- Production operator tries to run without evidence capture: value gate fails because verification artifacts cannot prove table-level safety.

## User Journey

1. Platform operator runs `make db-migrate-tenant-platform-dry-run`.
2. Script reports default tenant presence, eligible counts, conflicts, unmapped rows, and readiness.
3. When readiness is safe, operator runs `make db-migrate-tenant-platform`.
4. Script backfills legacy global rows into `tenant_key=legacy-default` in idempotent batches.
5. Operator records verification queries from `docs/tenant-platform-migration-runbook.md`.
6. If smoke fails, operator runs `make db-rollback-tenant-platform` and confirms nullable compatibility is restored.

Failure journeys:

1. Duplicate legacy campaign/product/subscription/cadence values are detected -> apply is blocked until conflicts are resolved.
2. Rerun after partial migration -> already migrated rows stay unchanged and remaining eligible rows are backfilled.
3. Rollback after failure -> rows owned by other tenants remain untouched.

## Open Questions

- Final `NOT NULL` tenant enforcement is intentionally out of scope for this slice and belongs after production backfill evidence is accepted.
- Production database execution is an operator decision; this artifact only proves the executable migration path and evidence contract.
