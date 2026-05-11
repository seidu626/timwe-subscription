# TMP-054 Domain Brief: Subscription Cadence Tenant Null Proof

## Actors

- Platform operator: needs a read-only answer before tenant `NOT NULL` enforcement is attempted.
- Subscription-external service: owns subscription, notification, and admin subscription action rows.
- Cadence worker: joins subscription, message series, message state, content, and outbox rows while selecting due cadence work.
- Canonical tenant: `nrg`, the expected owner for rows that were tenantless before TMP-050.

## Ubiquitous Language

- Tenantless row: a row in a tenant-owned table where `tenant_id IS NULL`.
- Proof table group: `subscriptions`, `notifications`, `admin_subscription_action_logs`, `product_message_series`, `message_content_items`, `subscription_message_state`, and `message_outbox`.
- Read-only proof: `SELECT`-only row counts for `tenant_id IS NULL`, with no migration, DDL, DML, or service runtime changes.
- Credential blocker: absence of documented database connection environment needed to run the proof safely.

## Domain Invariants

- TMP-054 must not mutate a remote database.
- TMP-054 must not edit service code, migration files, compose files, or dependency manifests.
- Every proof table must have a `tenant_id IS NULL` row-count result before TMP-055 can claim enforcement readiness.
- If documented connection environment is unavailable, the slice must block with explicit missing env/tool evidence instead of fabricating counts.

## Source Mapping

- `subscriptions`, `notifications`, and `admin_subscription_action_logs` receive nullable tenant ownership in `services/subscription-external/migrations/016_tenant_channel_subscription_routing.sql`.
- `product_message_series`, `message_content_items`, `subscription_message_state`, and `message_outbox` receive nullable tenant ownership in `services/subscription-external/migrations/017_tenant_notification_cadence_routing.sql`.
- `notifications` still has a tenantless charge idempotency lane in `services/subscription-external/migrations/018_charge_ownership_idempotency.sql`.
- Cadence due-state selection joins `subscription_message_state`, `subscriptions`, and `product_message_series` with NULL-tolerant tenant predicates in `services/cadence-engine/internal/repository/postgres.go`.
- Cadence missing-state selection joins `subscriptions` and `product_message_series` with a NULL-tolerant tenant predicate in `services/cadence-engine/internal/repository/postgres.go`.

## Failure Modes

- Missing credentials: `psql` exists but documented DB env variables are unset, so live row counts cannot run.
- Unsafe proof substitution: static migration evidence confirms nullable columns exist, but it does not prove live row counts are zero.
- Runtime leakage: cadence NULL-tolerant joins can continue matching rows across incomplete ownership until proof and enforcement remove the nullable path.

## User Journey

1. Platform operator prepares documented PostgreSQL connection environment.
2. Operator runs the TMP-054 read-only SQL.
3. The SQL returns total and `tenant_id IS NULL` counts for every proof table.
4. TMP-055 proceeds only if every target table reports zero tenantless rows.

## Open Questions

- Live row-count proof is blocked in this worktree because no documented DB connection variables are set and no `.env` file is present.
