# TMP-055 - Value Gate Report

Timestamp: 2026-05-14
Agent: codex

VALUE-GATE VERDICT: PASS WITH FOLLOW-UPS

## Live Proof

Credential source: `/home/xper626/workspace/apps/timwe-subscription/.env`

### Acquisition/Admin Tables

| table | tenantless_rows | enforcement_decision |
|---|---:|---|
| acquisition_transactions | 0 | enforce runtime tenant ownership |
| campaigns | 0 | remove tenantless campaign runtime use from transaction/report/callback paths |
| postback_outbox | 0 | eligible for future schema hardening |
| products | 0 | eligible for future schema hardening |
| userbase | 0 | eligible for future schema hardening |
| admin_activity_logs | 1 | leave schema nullable until row is reconciled |

### Subscription/Cadence Tables

| table | tenantless_rows | enforcement_decision |
|---|---:|---|
| admin_subscription_action_logs | 0 | eligible for future schema hardening |
| message_outbox | 0 | eligible for future schema hardening |
| product_message_series | 0 | remove legacy partial uniqueness lane |
| subscription_message_state | 0 | enforce runtime tenant ownership |
| subscriptions | 0 | enforce runtime tenant ownership |
| notifications | 10 | keep notification legacy/idempotency compatibility |

`product_message_states` and `cadence_delivery_ledger` from older proof notes are not live table names in this schema. The live cadence state table is `subscription_message_state`.

## Runtime Enforcement Changes

| acceptance | result | evidence |
|---|---|---|
| Acquisition transaction creation no longer depends on `tenant_id IS NULL` slug-only campaign lookup. | PASS | `TransactionService.CreateTransaction` now requires a tenant key and resolves campaigns through `GetByTenantKeyAndSlug`; signed public tenant headers are copied into the request by the HTTP handler when the body omits `tenant_key`. |
| Acquisition reports no longer join tenant-owned transactions through nullable campaign ownership. | PASS | `transactionCampaignPredicate` now requires `campaign.tenant_id = acquisition_transactions.tenant_id`. |
| Acquisition postback template lookup no longer falls back from tenant-owned transactions to slug-only campaign lookup. | PASS | `campaignForTransaction` and callback template lookup fail closed when a transaction tenant cannot be resolved. |
| Cadence due/missing-state queries no longer accept NULL tenant matches after proof. | PASS | `ClaimDueStatesTx` and `ListMissingStates` now require tenant equality with subscriptions. |
| Forward migrations clean legacy partial indexes or nullable constraints only where proof exists. | PASS | Added forward migrations dropping `idx_campaigns_legacy_slug` and `idx_product_message_series_legacy_key`; acquisition cleanup is included in the service-local schema bootstrap after campaign tenant binding. Notification legacy index remains because live notifications still contain tenantless rows. |

## Residual Follow-Ups

- Reconcile the single tenantless `admin_activity_logs` row before applying not-null enforcement there.
- Reconcile 10 tenantless `notifications` rows before removing notification charge legacy idempotency paths or `idx_notifications_charge_legacy_tx_uuid`.
- `CampaignRepository.GetBySlug` and `ListEnabled` still expose explicit public legacy compatibility against `tenant_id IS NULL`; live proof shows zero matching campaign rows, and transaction/report/callback runtime paths no longer use it. Removing or rejecting those public endpoints is a separate API compatibility decision.
- `subscription-external` still has `legacyProviderConfig` for explicit no-tenant partner compatibility. Removing that is a separate partner-contract decision because no-tenant callbacks are still documented as tolerated in onboarding docs.
- Channel nullable compatibility remains in cadence joins. TMP-055 evidence only proved tenant ownership, not channel ownership completeness.

## Verification

Required verification commands:

- `rg -n "tenant_id IS NULL|idx_.*legacy|legacyProviderConfig|falling back to legacy campaign slug" services/acquisition-api services/subscription-external services/cadence-engine -g '!**/vendor/**'`
- `cd services/acquisition-api && go test ./internal/repository ./internal/service ./internal/handler`
- `cd services/cadence-engine && go test ./internal/repository ./internal/adminhttp`
- `cd services/subscription-external && go test ./internal/service ./internal/repository ./internal/handler`
- `hvc check agent/backlog/issues/*.md --fail-on block`
