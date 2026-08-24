# TMP-080: Dayline hygiene closeout

## Scope decisions

The completeness review listed four hygiene items. One is already satisfied:
the admin console tenant workspace has guided gateway-credential forms for
provider_api, sms_api (base URL, key, header, sender id, body template,
delivery-confirmation field), and otp_api (delegated OTP), plus a test-SMS
proof button (tenant-list.component.ts, shipped in earlier slices, covered by
tenant-list.credential-form.spec.ts). No duplicate form is built.

Remaining items:

1. **Contract: marketplace shape.** `GET /v1/app/catalog` without a `tenant`
   param returns `{"tenants": [{"tenant_key", "tenant_name", "products":
   [...]}]}` (AppMarketplaceTenant); the contract doc only documents the
   tenant-filtered `{"products": [...]}` shape. Document both, including the
   per-product `tenant`/`tenant_name` fields.
2. **Regenerate stale `krakend/krakend.json`.** The live gateway renders
   flexible config from `krakend/config/krakend.tmpl`; the static
   `krakend/krakend.json` (consumed by scripts/check-krakend-query-forwarding.py
   and scripts/validate-charge-ownership.sh) predates the /v1/app/* routes.
   Re-render it from the template (krakend container, FC_OUT) with the default
   settings dir, then re-run the query-forwarding check.
3. **Router tests for the /v1/app routes** in
   services/acquisition-api/internal/transport: OPTIONS preflight returns 204
   with app CORS headers; wrong methods return 405 (with CORS set); authorized
   routes actually dispatch to AppHandler (proven by the nil-validator 401
   fail-closed behavior); unknown /v1/app path falls through to 404.

## Non-goals

- Deploying the regenerated krakend.json or image to the droplet.
- New console UI (credential form already exists).

## Verification

- `cd services/acquisition-api && go build ./... && go test ./...`
- `just krakend-query-forwarding-check` green against the regenerated file.
