# TMP-055 — Value Gate Report

Timestamp: 2026-05-11T22:52:46Z
Agent: codex

VALUE-GATE VERDICT: BLOCKED

## Audit 1: Proof Prerequisites

- TMP-053 acquisition proof SQL can now run with credentials (from `.env`/service env files):
  - All required tables in TMP-053 include `tenant_id`.
  - Tenant-null row counts are non-zero for key tables (for example all `campaigns`, `acquisition_transactions`, `postback_outbox`, `products` rows are tenant-null).
- TMP-054 subscription/cadence proof SQL cannot currently run with `tenant_id` predicates because tenant-owned tables in the documented runtime set are missing `tenant_id` columns in the checked environment:
  - `subscriptions`
  - `notifications`
  - `admin_subscription_action_logs`
  - `product_message_series`
  - `message_content_items`
  - `subscription_message_state`
  - `message_outbox`

## Audit 2: Required Evidence Scans

- Current nullable runtime candidates still exist:
  - `services/cadence-engine/internal/repository/postgres.go` includes `sms.tenant_id IS NULL OR s.tenant_id IS NULL` and similar legacy predicates.
  - `services/acquisition-api/internal/repository/reports_repository.go` still uses `(campaign.tenant_id IS NULL OR ...)`.
  - `services/acquisition-api/internal/repository/campaign_repository.go` still includes `tenant_id IS NULL` filters for legacy slug resolution paths.
  - `services/acquisition-api/internal/service/transaction_service.go` still logs “fallback to legacy campaign slug”.
  - Legacy idempotency/ownership migration artifacts (`017_*`, `018_*`, `seed_campaign`, `add_tenant_z_campaign_binding`) still retain `WHERE tenant_id IS NULL` partial behavior where expected by migration history.

## Audit 3: Runtime/Test Baseline

- `go test ./internal/repository ./internal/service ./internal/handler` for `services/acquisition-api` passes.
- `go test ./internal/repository ./internal/adminhttp` for `services/cadence-engine` passes.
- `go test ./internal/service ./internal/repository ./internal/handler` for `services/subscription-external` passes.
- `hvc check agent/backlog/issues/*.md --fail-on block` passes for `TMP-055`.

## Audit 4: Blocker Status

- `TMP-055` remains blocked until **live zero-row proof** is available for the active nullable runtime candidates and the subscription/cadence tenant columns are present in proof queries.
- This slice must not proceed with runtime enforcement changes until the proof obligations from TMP-053 and TMP-054 are satisfied.
