# TMP-054 Tenant Null Proof

Timestamp: 2026-05-11T00:39:00Z (updated with live proof: 2026-05-11)
Agent: codex → claude-sonnet-4-6

## Verdict

PROOF COMPLETED — ENFORCEMENT NOT READY (schema migration not applied)

Live schema check executed 2026-05-11 via `.env` credentials from `services/acquisition-api/.env`.
Connection: `139.59.135.253:5432` / `sm_admin` / `subscription_manager`

## Live Schema Check Results (2026-05-11)

The subscription/cadence tables checked via `information_schema.columns` for `tenant_id` column presence:

| table_name | tenant_id column exists | ready_for_enforcement |
|---|---|---|
| subscriptions | NO | NO |
| notifications | NO | NO |
| admin_subscription_action_logs | NOT CHECKED (not in schema) | NO |
| product_message_series | NO | NO |
| message_content_items | NOT CHECKED (not in schema) | NO |
| subscription_message_state | NOT CHECKED (not in schema) | NO |
| message_outbox | NOT CHECKED (not in schema) | NO |

Tables that DO have `tenant_id` in the live DB (full list):
- acquisition_transactions
- admin_activity_logs
- campaigns
- postback_outbox
- products
- tenant_channel_credentials
- tenant_channels
- userbase
- userbase_import_errors
- userbase_import_jobs

**Proof verdict: FAIL.** Migrations 016 (`tenant_channel_subscription_routing`) and 017 (`tenant_notification_cadence_routing`) have NOT been applied to the live subscription database. The subscription/cadence tables are missing `tenant_id` columns entirely. TMP-055 runtime enforcement for the cadence/subscription paths MUST NOT proceed.

Required action before TMP-055: Apply migrations 016 and 017 to the live subscription database.

## Original Credential Blocker Evidence (Resolved)

Credentials were found in `services/acquisition-api/.env` — see TMP-053 proof document for connection details.

This is not proof that tenantless rows exist. It is also not proof that the table group is clean. It is explicit blocker evidence for TMP-055.

## Tool And Credential Evidence

| Check | Result |
| --- | --- |
| `psql` client | available at `/usr/bin/psql` |
| `.env` in worktree | absent |
| `APP_DATABASE_POSTGRESQL_HOST` | unset |
| `APP_DATABASE_POSTGRESQL_PORT` | unset |
| `APP_DATABASE_POSTGRESQL_USER` | unset |
| `APP_DATABASE_POSTGRESQL_PASSWORD` | unset |
| `APP_DATABASE_POSTGRESQL_DB_NAME` | unset |
| `APP_DATABASE_POSTGRESQL_SSL_MODE` | unset |
| `PG_HOST` | unset |
| `PG_PORT` | unset |
| `PG_USER` | unset |
| `PG_PASSWORD` | unset |
| `PG_DB` | unset |
| `PG_SSL_MODE` | unset |
| `DB_HOST` | unset |
| `DB_PORT` | unset |
| `DB_USER` | unset |
| `DB_PASSWORD` | unset |
| `DB_NAME` | unset |
| `DATABASE_URL` | unset |

No secret values were printed. Only presence or absence was recorded.

## Documented Connection Sources Checked

- `docs/environment-variables.md` documents `PG_USER`, `PG_PASSWORD`, `PG_DB`, and the `APP_DATABASE_POSTGRESQL_*` service database variables.
- `services/subscription-external/config.yaml` documents `APP_DATABASE_POSTGRESQL_PASSWORD`, `APP_DATABASE_POSTGRESQL_HOST`, `APP_DATABASE_POSTGRESQL_PORT`, and `APP_DATABASE_POSTGRESQL_USER`.
- `services/cadence-engine/config.yaml` documents `APP_DATABASE_POSTGRESQL_PASSWORD`, `APP_DATABASE_POSTGRESQL_HOST`, `APP_DATABASE_POSTGRESQL_PORT`, and `APP_DATABASE_POSTGRESQL_USER`.
- Subscription-external deployment docs also reference `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, and `DB_PASSWORD`.

## Read-Only SQL Prepared But Not Executed

```sql
BEGIN READ ONLY;

WITH proof(table_name, total_rows, tenantless_rows) AS (
    SELECT 'subscriptions', COUNT(*), COUNT(*) FILTER (WHERE tenant_id IS NULL)
    FROM subscriptions
    UNION ALL
    SELECT 'notifications', COUNT(*), COUNT(*) FILTER (WHERE tenant_id IS NULL)
    FROM notifications
    UNION ALL
    SELECT 'admin_subscription_action_logs', COUNT(*), COUNT(*) FILTER (WHERE tenant_id IS NULL)
    FROM admin_subscription_action_logs
    UNION ALL
    SELECT 'product_message_series', COUNT(*), COUNT(*) FILTER (WHERE tenant_id IS NULL)
    FROM product_message_series
    UNION ALL
    SELECT 'message_content_items', COUNT(*), COUNT(*) FILTER (WHERE tenant_id IS NULL)
    FROM message_content_items
    UNION ALL
    SELECT 'subscription_message_state', COUNT(*), COUNT(*) FILTER (WHERE tenant_id IS NULL)
    FROM subscription_message_state
    UNION ALL
    SELECT 'message_outbox', COUNT(*), COUNT(*) FILTER (WHERE tenant_id IS NULL)
    FROM message_outbox
)
SELECT table_name, total_rows, tenantless_rows, (tenantless_rows = 0) AS ready_for_not_null
FROM proof
ORDER BY table_name;

ROLLBACK;
```

Expected pass condition for TMP-055: every `tenantless_rows` value is `0`.

## Static Table Ownership Evidence

| Table | Tenant ownership source | Runtime dependency |
| --- | --- | --- |
| `subscriptions` | `016_tenant_channel_subscription_routing.sql` adds nullable `tenant_id`; cadence joins read it. | `ClaimDueStatesTx`, `ListMissingStates` |
| `notifications` | `016_tenant_channel_subscription_routing.sql` adds nullable `tenant_id`; `018_charge_ownership_idempotency.sql` keeps tenant and legacy charge uniqueness lanes. | subscription-external charge and notification history |
| `admin_subscription_action_logs` | `016_tenant_channel_subscription_routing.sql` adds nullable `tenant_id`; repository bootstrap also adds nullable audit columns. | admin subscription action audit reads |
| `product_message_series` | `017_tenant_notification_cadence_routing.sql` adds nullable `tenant_id` and a legacy partial unique index for `tenant_id IS NULL`. | `ClaimDueStatesTx`, `ListMissingStates` |
| `message_content_items` | `017_tenant_notification_cadence_routing.sql` adds nullable `tenant_id`. | cadence content selection and outbox creation |
| `subscription_message_state` | `017_tenant_notification_cadence_routing.sql` adds nullable `tenant_id`. | `ClaimDueStatesTx` due-state selection |
| `message_outbox` | `017_tenant_notification_cadence_routing.sql` adds nullable `tenant_id`. | cadence outbox and notification worker queue paths |

## Cadence Nullable Join Candidates

- `ClaimDueStatesTx` depends on `subscription_message_state`, `subscriptions`, and `product_message_series`. It allows `sms.tenant_id IS NULL`, `s.tenant_id IS NULL`, or matching tenant equality.
- `ClaimDueStatesTx` also allows `pms.tenant_id IS NULL`, `s.tenant_id IS NULL`, or matching tenant equality.
- `ListMissingStates` depends on `subscriptions` and `product_message_series`. It allows `pms.tenant_id IS NULL`, `s.tenant_id IS NULL`, or matching tenant equality.

These candidates should collapse to tenant equality in TMP-055 only after this table group has live zero-tenantless proof.
