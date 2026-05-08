# TMP-017 Domain Brief: Billing Charge Ownership

## Actors

- Platform operator: decides service ownership and deployability for charge-capable tenant channels. Source: `slices/TMP-017-billing-charge-ownership/slice.yaml`.
- API-integrated partner: calls partner MT and direct charge endpoints through tenant channel configuration. Source: `services/subscription-external/internal/handler/partner_handler.go`.
- Tenant admin / operations analyst: relies on tenant-scoped charge, notification, postback, and report state not leaking across tenants. Source: `slices/manifest.json`.
- TIMWE / MNO upstream: receives routed charge requests and later emits callback/notification state. Source: `services/subscription-external/internal/service/subscription.go`.

## Ubiquitous Language

- Charge owner: the single service allowed to create the tenant-scoped charge event used by renewal, reports, and postback correlation. Source: `slices/decisions/TMP-017-charge-ownership.md`.
- Tenant route: tenant/channel context resolved from headers or channel credentials before provider work begins. Source: `domain.TenantRouteContext`.
- Charge notification: `notifications` row with `type = 'CHARGE'` and `transaction_uuid` used as the charge ownership record. Source: `domain.MapChargeToNotification`.
- Idempotency key: caller/header supplied transaction UUID for a charge attempt, with a bounded generated fallback for clients that do not send one. Source: `PartnerChargeHandler` and `chargeIdempotencyKey`.
- Disabled billing service: legacy service present in the repository but not deployable as tenant charge owner. Source: `docker-compose.yml` and ADR.

## Invariants

- One charge event has one tenant-scoped owner. Enforced by ADR, `subscription-external` write path, and partial unique indexes on charge notifications.
- Tenant and channel context must survive charge routing. Enforced by canonical tenant route propagation into `NotificationRequest`.
- The disabled billing service must not receive active gateway charge or MT routes. Enforced by KrakenD template/static config retargeting and `scripts/validate-charge-ownership.sh`.
- Duplicate charge attempts with the same tenant/idempotency identity must not create a second ownership record. Enforced by repository duplicate-key handling.
- Legacy rows remain compatible while tenant rows get stricter ownership. Enforced by nullable tenant/channel columns and separate legacy partial unique index.

## Failure Modes

- Missing tenant/channel context: partner charge handler rejects required tenant context before service work.
- Unsupported charge capability or missing provider credential: tenant provider resolution fails closed before upstream charge call.
- Provider dependency failure: `RequestCharge` returns the provider error and does not create ownership state.
- Duplicate charge event: upstream receives the resolved `external-tx-id`, and the repository treats PostgreSQL unique violation as idempotent duplicate.
- Post-provider ownership persistence failure: API returns provider success and logs the failed ownership write to avoid inducing a caller retry double charge.
- Disabled billing dependency: validation fails if `billing:` is enabled without tenant/channel ownership, or if KrakenD routes still target billing ownership.
- Cross-tenant charge/report lookup: downstream report slices must preserve tenant filters; this slice creates tenant-owned charge data for those filters.

## User Journey

1. Platform operator reviews charge architecture and records `subscription-external` as the tenant-platform charge owner.
2. API-integrated partner calls the tenant-aware direct charge endpoint.
3. `subscription-external` resolves tenant/channel provider configuration, forwards a charge idempotency key to the provider, and records a tenant/channel `CHARGE` notification.
4. Duplicate retries reuse the same idempotency identity and do not create a second charge owner.
5. Deployment validation confirms active gateway routes do not point charge ownership at disabled billing.
