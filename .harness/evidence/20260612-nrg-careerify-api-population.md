# NRG / Careerify TIMWE API Population Evidence

Date: 2026-06-12
WorkOrder: WO-POPULATE-THE-NRG-TENANT-CAREERIFY-TIMWE-RECORDS

## Scope

Populate and verify tenant-specific TIMWE metadata through supported APIs without persisting raw API keys or PSKs in repository artifacts.

## API Population

- Careerify product `32535` is present through `GET /v1/admin/products` with `X-Tenant-Key: careerify`.
  - Evidence: `.harness/logs/20260612-acq-18084-careerify-products-list.body`
  - Values: name `Careerify`, billing pricepoint `70946`, shortcode `6555`.
- Careerify product upsert through `POST /v1/admin/products/batch` returned 200.
  - Evidence: `.harness/logs/20260612-acq-18084-careerify-product-upsert.status`
- NRG product upsert through `POST /v1/admin/products/batch` returned 500 because the legacy global unique constraint `ux_products_product_id_name` rejects another `32535/Careerify` row across tenants.
  - Evidence: `.harness/logs/20260612-acq-18084-nrg-careerify-product-upsert.status`
  - Service excerpt: `.harness/logs/20260612-acq-18084-nrg-careerify-product-upsert.body`
  - This was not bypassed with direct SQL.
- NRG and Careerify channels are active via `GET /v1/admin/channels`.
  - Evidence: `.harness/logs/20260612-acq-18084-nrg-channels-list.body`
  - Evidence: `.harness/logs/20260612-acq-18084-careerify-channels-list.body`
  - Channel key: `web-gh-airteltigo`.
  - Provider: `timwe`.
  - Capabilities: `optin`, `confirm`, `mt`, `charge`.
- NRG and Careerify active provider credentials are bound through `GET /v1/admin/channels/{channel_id}/credentials`.
  - Evidence: `.harness/logs/20260612-acq-18084-nrg-credentials-list-refresh.body`
  - Evidence: `.harness/logs/20260612-acq-18084-careerify-credentials-list-refresh.body`
  - Active refs are redacted `env://` references; API responses do not expose raw secrets.

## Runtime Verification

- Health checks returned 200 for:
  - notification: `http://127.0.0.1:8082/health`
  - subscription-external: `http://127.0.0.1:8083/health`
  - acquisition-api: `http://127.0.0.1:18084/health`
- Subscription-external trusted gateway route probes returned `400 INVALID_REQUEST` after resolving tenant/channel for both tenants, intentionally using malformed JSON to avoid calling live TIMWE.
  - Evidence: `.harness/logs/20260612-subext-careerify-route-probes.summary`
  - Evidence: `.harness/logs/20260612-subext-nrg-route-probes.summary`
  - Covered: optin, confirm, optout, status, mt, charge.
- Careerify notification callbacks returned 200 for all six callback surfaces.
  - Evidence: `.harness/logs/20260612-careerify-notification-refresh.summary`
  - Covered: MO, MT DN, user opt-in, user renewed, user opt-out, charge.
- Tenant provider credential env refs are present in the live subscription-external process with redacted proof.
  - Evidence: `.harness/logs/20260612-subext-credential-env-redacted.txt`
  - Careerify includes subscription API key, send-MT API key, PSK, partner service `2170`, partner role `2117`, MCC `620`, MNC `03`, large account `6555`, free MT pricepoint `44084`, MO pricepoint `44083`, billing pricepoint `70946`, and HE IV key.
  - NRG includes subscription API key, PSK, partner service `2170`, partner role `2117`, MCC `620`, MNC `03`, large account `6555`, free MT pricepoint `44084`, MO pricepoint `44083`, and billing pricepoint `70946`.

## Tests

- `go test ./internal/service -run 'Test.*Tenant|Test.*Provider|Test.*Credential|Test.*Pricepoint|Test.*MT'`
  - Workdir: `services/subscription-external`
  - Evidence: `.harness/logs/20260612-go-test-subext-service-tenant.log`
- `go test ./internal/handler -run 'TestTenantRouteFromGatewayHeaders|TestGatewayRouteStatus|TestPartnerSubscription|Test.*Gateway'`
  - Workdir: `services/subscription-external`
  - Evidence: `.harness/logs/20260612-go-test-subext-handler-partner.log`
- `go test ./internal/handler -run 'TestHandleNotification|TestListNotifications|TestAdminRequire|TestTenant'`
  - Workdir: `services/notification`
  - Evidence: `.harness/logs/20260612-go-test-notification-handler-tenant.log`

## Secrets Gate

- Raw operator-provided API keys and PSK were removed from `.harness/runs/tenant-gaps-20260610/members/onboarding/payloads.md`.
- Path-only scans with `rg --no-ignore -l --fixed-strings` returned zero repository artifact hits for the supplied subscription API key, send-MT API key, and PSK.
  - Evidence: `.harness/logs/20260612-secret-scan-subscription-key-paths.txt`
  - Evidence: `.harness/logs/20260612-secret-scan-mt-key-paths.txt`
  - Evidence: `.harness/logs/20260612-secret-scan-psk-paths.txt`

## Residual Risk

- Live TIMWE opt-in, MT, charge, opt-out, confirm, and status provider calls were not executed because the active base URL points at the real provider. Those calls can create subscriptions, send messages, or charge users and require explicit production-impact approval.
- Tenant-scoped product duplication for the same `product_id/name` is blocked by a legacy global unique constraint. Fixing that requires an approved schema/migration change.
