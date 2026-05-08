# ADR: TMP-017 Charge Ownership

- Status: Accepted
- Date: 2026-05-08
- Slice: TMP-017 billing-charge-ownership

## Decision

`subscription-external` is the tenant-platform owner for charge-capable channel flows.

The legacy `services/billing` service remains disabled and is not a tenant-platform charge ledger. It may not be re-enabled for tenant charge traffic until it has tenant/channel ownership, idempotency, authorization, and migration coverage equivalent to `subscription-external`.

## Rationale

- `subscription-external` already owns `POST /api/external/v1/charge/dob`, tenant channel provider routing, renewal/churn state, charge success notification scanning, and acquisition postback correlation.
- Charge notifications already carry `tenant_id` and `channel_id` through `notifications`, `FetchChargeSuccessNotifications`, and `ChargeSuccessRequest`.
- `services/billing` only stores `billing_transactions(msisdn, product_id, amount, status, created_at, updated_at)` through an MSISDN lookup surface and has no tenant/channel authz, idempotency key, provider routing, renewal integration, or compose runtime.
- Splitting writes between the disabled billing service and `subscription-external` would create two ledgers for one upstream charge event.

## Boundary

`subscription-external` owns:

- tenant-routed provider charge requests;
- charge notifications and transaction UUID correlation;
- renewal and charging failure state;
- charge-success callbacks to acquisition/postback flows;
- tenant/channel charge reporting source rows until a future ledger service is explicitly migrated.

`services/billing` owns no tenant-platform charge writes in the current architecture.

Legacy gateway routes for `/api/v1/{realm}/{channel}/mt/{partnerRole}` and `/api/v1/{realm}/charge/dob/{partnerRole}` remain present for compatibility, but KrakenD retargets them to `subscription-external` canonical partner endpoints instead of the disabled billing owner.

Charge idempotency is resolved in this order:

1. Client or gateway supplied request `idempotencyKey`.
2. `external-tx-id` header forwarded by KrakenD.
3. A bounded generated fallback keyed by tenant, channel, product, subscriber, context, and minute bucket.

`subscription-external` forwards the resolved idempotency key to the upstream provider as `external-tx-id` and stores it as the charge notification transaction UUID. If the upstream provider succeeds but local ownership persistence fails, the API returns the provider success and logs the ownership error instead of causing an automatic caller retry that could double charge.

## Migration Posture

No production charge traffic is migrated into `services/billing` in this slice. A future ledger replacement must provide:

- tenant/channel columns and constraints;
- idempotency per upstream charge event;
- tenant-scoped read APIs;
- migration plan from `subscription-external` charge notifications/renewal state;
- explicit compose/Kubernetes dependency wiring.

## Guardrail

`scripts/validate-charge-ownership.sh` fails if the compose billing service is enabled without a tenant-aware billing schema, if the ADR is missing, or if KrakenD routes still point charge/MT backend patterns at the legacy billing shape.
