# Tenant Nullability Enforcement Plan

## Canonical Tenancy RFC Relationship

This enforcement plan is a source input for the proposed canonical tenancy model in `docs/architecture/canonical-tenancy-model.md`. Until that RFC is approved, this document remains the nullable-path proof plan. If the RFC is approved, it supersedes any interpretation that nullable tenant ownership is a permanent runtime capability; residual nullable paths must stay explicitly documented, proven, and assigned to follow-up WorkOrders before forward-only enforcement.

TMP-052 audited remaining `tenant_id IS NULL` paths after the canonical `nrg` migration. The enforcement plan is forward-only:

1. Keep TMP-050 migration script and runbook predicates as the operator proof surface.
2. Prove acquisition/admin table groups have zero tenantless rows.
3. Prove subscription/cadence table groups have zero tenantless rows.
4. Collapse runtime nullable joins and lookups into tenant-aware canonical paths.
5. Add forward migrations for NOT NULL constraints and legacy partial-index cleanup after proof.

Existing migrations must not be rewritten to pretend historical nullable columns never existed. Cleanup belongs in new migrations that can run against the live schema safely.

## 2026-05-14 proof refresh

The credentialed schema now proves enough ownership to remove the active acquisition/cadence runtime tenant fallbacks:

- `campaigns`, `acquisition_transactions`, `products`, `userbase`, and `postback_outbox` have zero `tenant_id IS NULL` rows.
- `subscriptions`, `product_message_series`, `subscription_message_state`, `message_outbox`, and `admin_subscription_action_logs` have zero `tenant_id IS NULL` rows.

Enforced runtime paths:

- Acquisition transaction creation requires a tenant key and uses tenant-scoped campaign lookup. The HTTP handler accepts the already supported signed public tenant headers and copies the verified tenant key into the transaction request when the body omits `tenant_key`.
- Acquisition reporting and postback template lookup no longer join or fall back through nullable campaign ownership.
- Subscription provider runtime paths require tenant/channel context and resolve TIMWE provider credentials through the tenant provider router; empty tenant routes now fail closed instead of using global compatibility credentials.
- Cadence due-state and missing-state queries require tenant equality between subscription state, subscriptions, and product message series.

Two residual groups remain intentionally nullable:

- `admin_activity_logs` still has one tenantless row.
- `notifications` still has ten tenantless rows, so notification charge idempotency cleanup remains follow-up work.
- Public campaign `GetBySlug`/`ListEnabled` still contain explicit `tenant_id IS NULL` compatibility. Live proof shows zero rows in that lane, and transaction/report/callback paths no longer depend on it; removing the public compatibility endpoints requires a separate API decision.

Do not apply broad `SET NOT NULL` constraints until those residual rows are reconciled and channel ownership proof is collected separately.
