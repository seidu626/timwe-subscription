# TMP-013 Spec: Inbound Callback Correlation

## Story

As a mno-or-timwe-upstream, I want callbacks to be correlated through the agreed tenant/channel route so that subscription, charge, and postback state updates land only in the intended tenant.

## Acceptance Criteria

- Happy path: subscription-external CHARGE notifications with tenant/channel context forward that context to acquisition charge-success, and acquisition uses tenant plus TIMWE transaction id before mutating the transaction.
- Happy path: telco callbacks with `transaction_id`, tenant, and channel update the correlated transaction and enqueue tenant/channel-owned postback work.
- Failure: telco callbacks without `transaction_id` return 422 and do not fall back to MSISDN-only lookup.
- Failure: charge-success with mismatched tenant returns a tenant mismatch error before state transition.
- Failure: telco callbacks with mismatched tenant return 403 or not found before state transition.
- Edge: legacy telco callbacks without tenant id can still correlate by TIMWE transaction id, but never by global MSISDN.
- Edge: callback replay for a transaction already subscribed or charged returns idempotent success without duplicate postback enqueue.
- Invariant: callback-generated postbacks carry tenant ownership and callback/campaign channel ownership when available.

## Scope

- In scope: acquisition-api telco callback correlation, acquisition charge-success tenant verification, subscription-external notification monitor tenant/channel propagation, and focused tests.
- Out of scope: tenant-scoped notification listing, cadence worker routing, admin UI changes, durable callback quarantine ledger, and new provider callback shapes.

## Evidence

- `services/acquisition-api/internal/handler/callback_handler.go`
- `services/acquisition-api/internal/handler/callback_handler_test.go`
- `services/acquisition-api/internal/repository/transaction_repository.go`
- `services/acquisition-api/internal/repository/transaction_repository_test.go`
- `services/acquisition-api/internal/service/transaction_service.go`
- `services/acquisition-api/internal/service/postback_routing_test.go`
- `services/subscription-external/internal/repository/postgres.go`
- `services/subscription-external/internal/repository/subscription.interface.go`
- `services/subscription-external/internal/service/acquisition_client.go`
- `services/subscription-external/internal/worker/notification_monitor.go`
