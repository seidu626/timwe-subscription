# TMP-013 Domain Brief: Inbound Callback Correlation

## Actors

- mno-or-timwe-upstream: sends subscription, delivery, charge, and renewal callbacks into platform endpoints (source: `slices/TMP-013-inbound-callback-correlation/slice.yaml`).
- subscription-external notification monitor: scans persisted TIMWE charge notifications and forwards charge-success events to acquisition-api (source: `services/subscription-external/internal/worker/notification_monitor.go`).
- acquisition-api: correlates inbound telco callbacks and internal charge-success events to acquisition transactions before mutating transaction/postback state (source: `services/acquisition-api/internal/handler/callback_handler.go`, `services/acquisition-api/internal/service/transaction_service.go`).
- tenant admin: later observes notification, postback, and operations state scoped to their tenant/channel (source: `slices/TMP-008-notification-and-cadence-routing/slice.yaml`, `slices/TMP-010-tenant-reporting-operations/slice.yaml`).
- ad-or-traffic-partner: receives postbacks generated from correlated callback outcomes (source: `services/acquisition-api/internal/domain/postback.go`).

## Ubiquitous Language

- Tenant correlation: tenant id carried by a request or recovered from a tenant-owned transaction before state mutation (source: `common/auth/tenantctx`, `services/acquisition-api/internal/domain/transaction.go`).
- Channel correlation: channel id carried by a callback/notification or recovered from the tenant campaign for outbox ownership (source: `services/acquisition-api/internal/domain/campaign.go`, `services/acquisition-api/internal/domain/postback.go`).
- TIMWE transaction id: provider-side transaction identifier stored as `timwe_transaction_id` and used for callback/charge-success correlation (source: `services/acquisition-api/internal/repository/transaction_repository.go`).
- Charge success: internal event sent by subscription-external after CHARGE notifications, used to advance transaction state and enqueue conversion postbacks (source: `services/acquisition-api/internal/service/transaction_service.go`).
- Callback replay: provider retry for a transaction that is already subscribed/charged; it must acknowledge without duplicate downstream effects (source: `services/acquisition-api/internal/handler/callback_handler.go`).
- Uncorrelatable callback: inbound callback missing a transaction correlation key; it must not fall back to global MSISDN mutation (source: `slices/TMP-013-inbound-callback-correlation/slice.yaml`).

## Domain Invariants

- Inbound events cannot mutate another tenant's transaction: tenant-bearing callbacks and charge-success events use tenant plus TIMWE transaction id before update (source: `services/acquisition-api/internal/repository/transaction_repository.go`).
- Uncorrelatable callbacks do not mutate state: `/v1/callbacks/{telco}` rejects missing `transaction_id` before repository access (source: `services/acquisition-api/internal/handler/callback_handler.go`).
- Legacy callback compatibility is bounded: a callback without tenant id may still use the TIMWE transaction id, but no MSISDN-only global lookup remains (source: `services/acquisition-api/internal/handler/callback_handler.go`).
- Callback-generated postbacks preserve tenant/channel ownership: outbox rows use transaction tenant and callback/campaign channel (source: `services/acquisition-api/internal/handler/callback_handler.go`).
- Charge-success events preserve tenant/channel context from notification rows into acquisition processing (source: `services/subscription-external/internal/repository/postgres.go`, `services/subscription-external/internal/worker/notification_monitor.go`, `services/subscription-external/internal/service/acquisition_client.go`).
- Replay of already subscribed/charged telco callbacks is idempotently acknowledged without another update/postback enqueue (source: `services/acquisition-api/internal/handler/callback_handler.go`).

## Failure Modes

- `/v1/callbacks/{telco}`:
  - Invalid input: malformed JSON returns 400.
  - Missing required: absent `transaction_id` returns 422 with no repository mutation.
  - Cross-tenant mismatch: tenant id that does not match transaction tenant returns 403.
  - Duplicate/replay: already subscribed/charged transaction returns an idempotent 200.
  - Dependency failure: transaction lookup failure returns 404.
- `/internal/acquisition/charge-success`:
  - Invalid input: missing `timwe_transaction_id` returns an error before lookup.
  - Missing tenant on legacy event: accepted for compatibility, but tenant-scoped rows use tenant lookup when present.
  - Cross-tenant mismatch: request tenant different from transaction tenant returns `charge success tenant mismatch`.
  - Dependency failure: transaction/campaign/postback repository failures return/log existing errors.
- subscription-external notification monitor:
  - Missing transaction UUID: notification is skipped without charge-success call.
  - Duplicate/replay: Redis dedup by notification row id prevents repeated async sends.
  - Dependency failure: notification fetch failure aborts the scan cycle with an error.

## User Journey

1. TIMWE sends a CHARGE notification that has tenant/channel persisted on the notification row.
2. subscription-external scans the row and calls acquisition-api charge-success with tenant id, channel id, TIMWE transaction id, MSISDN, product id, and charged timestamp.
3. acquisition-api looks up the transaction by tenant plus TIMWE transaction id, verifies tenant match, advances eligible state to subscribed/charged, and enqueues tenant/channel-owned conversion postback work.
4. TIMWE or an MNO sends a telco callback to `/v1/callbacks/{telco}` with `transaction_id` and optional tenant/channel.
5. acquisition-api rejects uncorrelatable callbacks, rejects cross-tenant mismatch, idempotently acknowledges replay, and stamps callback-generated postbacks with tenant/channel.

Failure journeys:

1. Callback includes only MSISDN and status -> 422; no global MSISDN transaction is updated.
2. Callback includes tenant A and a transaction owned by tenant B -> 403 or not found through tenant-scoped lookup; no postback is queued.
3. Charge-success carries tenant A for a tenant B transaction -> service returns tenant mismatch before state transition.

## Open Questions

- `subscription-partner` and `services/notification` still have legacy global notification entrypoints. This slice preserves tenant/channel charge-success propagation through subscription-external and acquisition; full tenant-scoped notification listing, worker dispatch, and cadence behavior remain in TMP-008.
- Durable inbound callback quarantine/audit tables are not yet present. Current behavior rejects uncorrelatable callbacks and records ordinary logs; a future hardening slice can add a callback ledger if audit retention is required.
