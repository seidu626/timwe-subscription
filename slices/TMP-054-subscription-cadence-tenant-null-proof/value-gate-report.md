# TMP-054 Value Gate Report

Timestamp: 2026-05-11 (updated with live schema proof by claude-sonnet-4-6)
Agent: codex (original) / claude-sonnet-4-6 (live schema check)

VALUE-GATE VERDICT: BLOCKED — SCHEMA MIGRATION NOT APPLIED TO LIVE DB

## Audit 1: Criteria Coverage

| criterion | status | evidence |
| --- | --- | --- |
| Subscription/cadence tenant-owned tables have row-count proof for `tenant_id IS NULL`. | BLOCKED — deeper than credentials | Live schema check confirms `tenant_id` column does not exist in `subscriptions` or related tables. |
| Cadence runtime nullable join candidates are mapped to the tables they depend on. | PASS | `tenant-null-proof.md` maps `ClaimDueStatesTx` and `ListMissingStates` to `subscriptions`, `product_message_series`, and `subscription_message_state`. |
| No remote database mutation is performed. | PASS | Only `SELECT` on `information_schema.columns` run. |

## Audit 2: Live Schema Check Results (2026-05-11)

Via `information_schema.columns WHERE column_name = 'tenant_id'`:

Tables WITH `tenant_id` in live DB:
- acquisition_transactions, admin_activity_logs, campaigns, postback_outbox, products, tenant_channel_credentials, tenant_channels, userbase, userbase_import_errors, userbase_import_jobs

Tables WITHOUT `tenant_id` in live DB (subscription/cadence group):
- subscriptions — MISSING tenant_id
- notifications — MISSING tenant_id
- product_message_series — MISSING tenant_id
- message_outbox — MISSING tenant_id (not confirmed, column absent from schema results)

**Migrations 016 and 017 have NOT been applied to the live database.**

## Audit 3: Domain Invariants

- No service or migration edits: PASS.
- No remote DB mutation: PASS.
- No secret disclosure: PASS.
- Enforcement readiness: BLOCKED. Cannot run NULL row-count queries because the `tenant_id` column doesn't exist yet.

## Audit 4: User Journey

- Operator now has definitive evidence of schema migration state: COMPLETE.
- TMP-055 has a clear, evidence-based blocker: COMPLETE.

## Audit 5: Test Quality

- `hvc check agent/backlog/issues/*.md --fail-on block`: PASS.
- No source tests were added (read-only proof slice).

## Gaps

Two sequential prerequisites before TMP-055:
1. Apply migration 016 (`016_tenant_channel_subscription_routing.sql`) to live DB.
2. Apply migration 017 (`017_tenant_notification_cadence_routing.sql`) to live DB.
3. Rerun TMP-054 proof SQL — every target table must report `tenantless_rows = 0`.
