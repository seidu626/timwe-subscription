# TMP-052 Nullable Path Audit

Audit command:

```bash
rg -n "tenant_id IS NULL|tenant_id = NULL|WHERE .*tenant_id IS NULL|tenant_id UUID" services scripts docs frontend -g '!**/node_modules/**'
```

## Classification Summary

| Group | Representative evidence | Runtime? | Classification | Decision |
|---|---|---:|---|---|
| Canonical migration dry-run/apply predicates | `scripts/db-migrate-tenant-platform.sh:51`, `scripts/db-migrate-tenant-platform.sh:63`, `scripts/db-migrate-tenant-platform.sh:75`, `scripts/db-migrate-tenant-platform.sh:87`, `scripts/db-migrate-tenant-platform.sh:99` | No | keep_as_permanent_capability | Keep until NOT NULL enforcement ships. These predicates are the migration proof surface for rows that still need `nrg` ownership. |
| Migration runbook verification queries | `docs/tenant-platform-migration-runbook.md:41`, `docs/tenant-platform-migration-runbook.md:57`, `docs/tenant-platform-migration-runbook.md:61`, `docs/tenant-platform-migration-runbook.md:65`, `docs/tenant-platform-migration-runbook.md:73`, `docs/tenant-platform-migration-runbook.md:79` | No | keep_as_permanent_capability | Keep as operational verification docs while TMP-050 backfill and later enforcement are staged. |
| Acquisition campaign slug-only runtime lookups | `services/acquisition-api/internal/repository/campaign_repository.go:63`, `services/acquisition-api/internal/repository/campaign_repository.go:282` | Yes | collapse_into_canonical | Replace with canonical tenant-aware lookup, likely defaulting unqualified legacy traffic to `nrg` or requiring tenant-qualified routes. This needs a focused implementation slice. |
| Acquisition transaction/report nullable campaign join | `services/acquisition-api/internal/repository/reports_repository.go:446` | Yes | collapse_into_canonical | The join should use tenant equality once transactions and campaigns have canonical ownership. Needs tests around tenant-filtered reporting. |
| Acquisition migration legacy slug index | `services/acquisition-api/migrations/add_tenant_z_campaign_binding.sql:30` | Migration | needs_human_decision | The historical partial index cannot be deleted blindly because it may already be applied. Emit a forward-only cleanup slice instead of rewriting history in TMP-052. |
| Acquisition seed tenantless upserts | `services/acquisition-api/migrations/create_mobplus_campaign.sql:141`, `services/acquisition-api/migrations/seed_campaign.sql:110`, `services/acquisition-api/migrations/seed_campaign.sql:158`, `services/acquisition-api/migrations/seed_campaign.sql:203` | Migration seed | delete_now | Fresh/bootstrap seeds still create or update tenantless campaigns. They should be removed or rewritten to canonical `nrg` ownership in the enforcement slice, not retained as a compatibility lane. |
| Acquisition/admin nullable tenant DDL | `services/acquisition-api/migrations/add_admin_management_tables.sql:68`, `services/acquisition-api/migrations/add_admin_management_tables.sql:71`, `services/acquisition-api/migrations/add_admin_management_tables.sql:74`, `services/acquisition-api/migrations/add_admin_management_tables.sql:77`, `services/acquisition-api/migrations/add_admin_management_tables.sql:80`, `services/acquisition-api/migrations/add_tenant_zz_acquisition_flow.sql:4` | Migration | needs_human_decision | These are historical nullable additions. Enforce only through a new forward migration after read-only row-count proof. |
| Subscription-external nullable tenant DDL | `services/subscription-external/migrations/016_tenant_channel_subscription_routing.sql:5`, `services/subscription-external/migrations/016_tenant_channel_subscription_routing.sql:9`, `services/subscription-external/migrations/016_tenant_channel_subscription_routing.sql:13`, `services/subscription-external/internal/repository/postgres.go:918`, `services/subscription-external/internal/repository/postgres.go:949` | Mixed | needs_human_decision | Runtime bootstrap still creates nullable audit columns. Needs live table proof plus service-level change before NOT NULL. |
| Cadence nullable runtime matching | `services/cadence-engine/internal/repository/postgres.go:41`, `services/cadence-engine/internal/repository/postgres.go:42`, `services/cadence-engine/internal/repository/postgres.go:616` | Yes | collapse_into_canonical | Replace NULL-tolerant joins with tenant equality after subscription/cadence rows have canonical ownership proof. |
| Subscription/cadence legacy partial uniqueness | `services/subscription-external/migrations/017_tenant_notification_cadence_routing.sql:27`, `services/subscription-external/migrations/018_charge_ownership_idempotency.sql:12` | Migration | needs_human_decision | Historical partial indexes preserve uniqueness for tenantless rows. Do not edit existing migrations; add forward cleanup once row counts prove zero tenantless rows. |

## Prune Notes

- `delete_now`: acquisition seed tenantless upserts should be removed or rewritten in the follow-up enforcement slice.
- `collapse_into_canonical`: acquisition runtime lookups, report joins, and cadence nullable joins should collapse into tenant-aware equality paths.
- `keep_as_permanent_capability`: the TMP-050 migration script and runbook retain `tenant_id IS NULL` strictly as observability and backfill eligibility until enforcement completes.
- `needs_human_decision`: historical migrations and runtime bootstrap DDL need forward migrations, not in-place rewrites.

## Enforcement Decision

No NOT NULL migration was added in TMP-052. The audit found active runtime nullable paths and historical nullable schema additions, but the slice has no live database proof that every touched table has zero `tenant_id IS NULL` rows after the `nrg` backfill. Enforcement is deferred to emitted implementation slices.
