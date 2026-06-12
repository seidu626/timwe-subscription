# Careerify Opt-In Key Selection Fix Evidence

Date: 2026-06-12
WorkOrder: `WO-POPULATE-THE-NRG-TENANT-CAREERIFY-TIMWE-RECORDS`
Tenant: `careerify`
Channel: `web-gh-airteltigo`
Service: `subscription-external`

## Issue

The tenant-routed partner opt-in API reached TIMWE at `/subscription/optin/2117` but TIMWE returned `NOT AUTHORIZED TO USE THE API`.

The route resolved the Careerify tenant/channel credential and endpoint correctly. The failure was local key selection: `SendMT` was using the tenant `mt_api_key` for the TIMWE subscription opt-in endpoint when a distinct MT key was present.

## Provider Proof

A direct TIMWE opt-in call using the same endpoint and app-equivalent payload succeeded when the Careerify tenant subscription `api_key` was used. The provider response was `SUCCESS` with `OPTIN_PREACTIVE_WAIT_CONF`.

Evidence from the live diagnostic is stored in redacted logs:

- `.harness/logs/20260612-careerify-live-optin.summary`
- `.harness/logs/20260612-careerify-live-optin-subext-log-excerpt.redacted.txt`
- `.harness/logs/20260612-careerify-direct-optin.summary`
- `.harness/logs/20260612-careerify-direct-optin-WEB.response.redacted.json`

No second live provider opt-in was sent for this fix verification to avoid duplicate external side effects on the same subscriber.

## Fix

`services/subscription-external/internal/service/subscription.go` now sends `providerCfg.APIKey` for `/subscription/optin/{partnerRoleID}` on both the primary request and the SMS retry path.

This preserves multi-tenant isolation because `providerCfg` is resolved through the tenant provider router for the request's tenant/channel route. The fix does not use global TIMWE credentials and does not hardcode Careerify values.

The tenant credential model still stores `mt_api_key`; it is no longer selected for this subscription opt-in endpoint.

The touched contract fixture was also changed to use a synthetic MSISDN instead of the live subscriber number used during diagnosis.

## Verification

Focused tests:

- `go test ./internal/service -run 'TestSendMT_UsesSubscriptionAPIKeyForOptinWhenMTAPIKeySet|TestSendMT_FallsBackToAPIKeyWhenMTAPIKeyAbsent|TestSendMT_UsesPostmanOptinContract|TestSendMTRoutesThroughTenantProviderConfig'`
- `go test ./internal/service -run 'TestSendOptoutWithRetry_UsesPostmanOptoutContract|TestSendStatusCheckWithRetry_UsesPostmanStatusContract|TestSendOptinConfirmWithRetry_UsesPostmanConfirmContract'`

Broader tests:

- `go test ./internal/service`
- `go test ./internal/handler -run 'TestTenantRouteFromGatewayHeaders|TestPartnerSubscription|TestGatewayRoute|Test.*Tenant|Test.*Partner'`
- `go test ./internal/...`
- `go test ./cmd`

Build and health smoke:

- `go build -o subscription-external ./cmd`
- Rebuilt `subscription-external` binary started locally on `127.0.0.1:8083`.
- `GET /health` returned HTTP 200.
- Smoke process was stopped after verification.

Verification logs:

- `.harness/logs/20260612-subext-service-key-selection-test.log`
- `.harness/logs/20260612-subext-service-subscription-contract-test.log`
- `.harness/logs/20260612-subext-internal-service-test.log`
- `.harness/logs/20260612-subext-handler-tenant-route-test.log`
- `.harness/logs/20260612-subext-internal-all-test.log`
- `.harness/logs/20260612-subext-cmd-test.log`
- `.harness/logs/20260612-subext-build.log`
- `.harness/logs/20260612-subext-health-smoke.status`
- `.harness/logs/20260612-subext-health-smoke.body`
- `.harness/logs/20260612-subext-service-key-selection-test-rerun.log`
- `.harness/logs/20260612-subext-internal-all-test-rerun.log`
- `.harness/logs/20260612-subext-cmd-test-rerun.log`
- `.harness/logs/20260612-subext-build-rerun.log`
- `.harness/logs/20260612-subext-health-smoke-rerun.status`
- `.harness/logs/20260612-subext-health-smoke-rerun.body`
- `.harness/logs/20260612-key-fix-narrow-secret-scan-rerun.log`
