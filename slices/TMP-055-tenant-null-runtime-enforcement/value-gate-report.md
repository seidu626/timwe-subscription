# TMP-055 — Value Gate Report

Timestamp: 2026-05-11 (updated with live DB proof)
Agent: claude-sonnet-4-6 (auto-pilot v2026-05-11.4)

VALUE-GATE VERDICT: BLOCKED — LIVE PROOF CONFIRMS NOT READY

## Audit 1: Proof Prerequisites (Live DB Evidence)

Live database connection established via `services/acquisition-api/.env`:
- Host: `139.59.135.253:5432`
- DB: `subscription_manager`

### Acquisition Side (TMP-053 scope)

| table | tenantless_rows | enforcement_ready |
|---|---|---|
| acquisition_transactions | 74 | NO |
| admin_activity_logs | 0 | YES |
| campaigns | 5 | NO |
| postback_outbox | 2 | NO |
| products | 10 | NO |
| userbase | 4873 | NO |

**Result: 5 of 6 tables have non-zero tenantless rows. Backfill is incomplete.**

### Subscription/Cadence Side (TMP-054 scope)

Schema check via `information_schema.columns WHERE column_name = 'tenant_id'`:

Subscription/cadence tables (`subscriptions`, `notifications`, `product_message_series`, `message_outbox`, etc.) do NOT have a `tenant_id` column in the live database. Migrations 016 and 017 have not been applied.

**Result: Schema migration prerequisite is missing. Runtime cannot be enforced.**

## Audit 2: Required Evidence Scans

Runtime nullable candidates still present in source (per `rg` scan):
- `services/cadence-engine/internal/repository/postgres.go:41,42,616` — `sms.tenant_id IS NULL OR s.tenant_id IS NULL`
- `services/acquisition-api/internal/repository/reports_repository.go:446` — `(campaign.tenant_id IS NULL OR ...)`
- `services/acquisition-api/internal/repository/campaign_repository.go:63,282` — `WHERE ... tenant_id IS NULL`
- `services/acquisition-api/internal/service/transaction_service.go:893` — "falling back to legacy campaign slug"
- `services/subscription-external/internal/service/tenant_routing.go:134,142` — `legacyProviderConfig`

These are correct and expected: removing them now would break active data access for tenantless rows.

## Audit 3: Runtime/Test Baseline

Tests pass on the current codebase (source verified, runtime nullable paths in place):
- `go test ./internal/repository ./internal/service ./internal/handler` — acquisition-api: PASS (prior session)
- `go test ./internal/repository ./internal/adminhttp` — cadence-engine: PASS (prior session)
- `go test ./internal/service ./internal/repository ./internal/handler` — subscription-external: PASS (prior session)
- `hvc check agent/backlog/issues/*.md --fail-on block` — PASS

## Audit 4: Blocker Status

TMP-055 is definitively blocked. Two separate blocking conditions confirmed by live DB:

1. **Acquisition backfill incomplete**: 4,878 tenantless rows across campaigns, transactions, postback, products, userbase.
2. **Subscription/cadence migrations not applied**: tenant_id column missing from subscriptions and related tables.

### What must happen before TMP-055 can proceed:

| Prerequisite | Owner | Evidence needed |
|---|---|---|
| Complete nrg backfill for acquisition tables | Platform ops | All 6 acquisition tables report 0 tenantless rows |
| Apply migration 016 (tenant_channel_subscription_routing) | Platform ops | subscriptions.tenant_id column exists in live DB |
| Apply migration 017 (tenant_notification_cadence_routing) | Platform ops | product_message_series.tenant_id column exists |
| Re-run TMP-053 proof SQL → all zeros | Platform ops | tenant-null-proof.md updated with zero-row output |
| Re-run TMP-054 proof SQL → all zeros | Platform ops | tenant-null-proof.md updated with zero-row output |

## Pre-existing CI Baseline Carve-outs

None — slice is in planned state, no CI runs on slice branch.

## Enforcement Decision

**DO NOT remove nullable runtime paths until all proof prerequisites are satisfied.**

Removing `tenant_id IS NULL` fallbacks from acquisition campaigns, cadence due-state queries, or subscription routing while tenantless rows exist would cause:
- Campaign lookups by slug to return "not found" for 5 active tenantless campaigns
- Cadence due-state queries to miss subscriptions that lack tenant_id
- Acquisition transaction tenant lookups to fail for 74 active transactions
