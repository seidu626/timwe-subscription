# TMP-017 Spec: Platform Has One Tenant-Aware Charge Owner

## Story

As a platform operator, I want direct charge ownership settled before tenant channels scale, so that billing, renewal, postback, and reporting state remain consistent.

## Scope

- Record the ownership decision in an ADR.
- Keep `services/billing` disabled for tenant-platform charge flows.
- Retarget legacy KrakenD charge/MT routes to the `subscription-external` owner.
- Persist successful tenant direct charges as `subscription-external` charge notifications.
- Add an idempotency guard so one tenant charge event creates at most one ownership record.
- Forward resolved charge idempotency to the upstream provider as `external-tx-id`.

## Acceptance Mapping

- Charge ownership decision recorded: `slices/decisions/TMP-017-charge-ownership.md`.
- Tenant charge route proven: `TestRequestChargeRoutesThroughTenantProviderConfig`.
- Split ownership conflict prevented: migration `018_charge_ownership_idempotency.sql`, `CreateChargeNotificationOnce`, and duplicate-key test.
- Disabled billing dependency fails explicitly: `scripts/validate-charge-ownership.sh`.
- Post-provider ownership write failure does not induce double charge: `TestRequestChargeReturnsProviderSuccessWhenOwnershipRecordFails`.
- Cross-tenant lookup groundwork: charge ownership rows carry `tenant_id` and `channel_id` for tenant-filtered reporting/ops slices.
- Legacy renewal compatibility: nullable tenant/channel fields and legacy partial unique index keep non-tenant charge rows compatible.

## Out Of Scope

- Re-enabling or rewriting `services/billing`.
- A finance ledger or payment reconciliation subsystem.
- Full tenant reporting UI changes; those belong to TMP-010.

## Verification

- `cd services/subscription-external && go test ./internal/domain ./internal/repository ./internal/service`
- `scripts/validate-charge-ownership.sh`
- `python3 -m json.tool krakend/krakend.json`
- `python3 -m json.tool slices/manifest.json`
- `git diff --check`
