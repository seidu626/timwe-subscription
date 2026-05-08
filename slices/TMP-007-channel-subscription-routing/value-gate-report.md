# TMP-007 Value Gate Report

- Timestamp: 2026-05-08T06:41:49Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:verified

## Audit 1: Acceptance Criteria Coverage

- Partner MT/optin resolves tenant channel credential and sends tenant provider config: COVERED by `services/subscription-external/internal/service/tenant_routing_test.go::TestSendMTRoutesThroughTenantProviderConfig`, which asserts resolver input, tenant URL path, API key, auth header, and provider payload.
- Acquisition opt-in carries transaction tenant/channel into subscription-external: COVERED by `services/acquisition-api/internal/service/timwe_client_test.go::TestTIMWEClient_OptInWithTenantSignsTenantChannelContext`, which asserts signed tenant headers and `channelId` propagation.
- Missing, unsigned, or forged tenant context returns 400/403 with no provider call: COVERED by `common/auth/tenantctx/trusted_service_test.go` signature/replay/body-binding coverage and `services/subscription-external/internal/handler/partner_handler_tenant_test.go::TestPartnerMTHandlerRejectsMissingTenantContextBeforeProviderCall`.
- Unsupported channel operation returns `422 unsupported_channel_operation` with no provider call: COVERED by `services/subscription-external/internal/service/tenant_routing_test.go::TestTenantRoutingOperationAllowedRequiresExplicitCapability` and `TestSendMTFailsClosedBeforeProviderCallWhenTenantRoutingRejects`.
- Missing or unresolvable credential fails before provider call and redacts secret material: COVERED by `TestEnvProviderCredentialResolver`, `TestSendMTFailsClosedBeforeProviderCallWhenTenantRoutingRejects`, and `TestRedactProviderHeaders`.
- Same MSISDN/product can be scoped by tenant ownership: COVERED by `TestMapMTRequestToSubscriptionRequestPreservesTenantChannel` plus the tenant-scoped upsert path in `services/subscription-external/internal/repository/postgres.go`.
- Upstream retry/circuit-breaker behavior remains intact: COVERED by `services/subscription-external/internal/service/subscription_external_tx_id_test.go` and full subscription-external service suite.
- `OPTIN_CONFIG_NOT_FOUND` SMS retry continues through resolved route config: COVERED by shared `SendMT` routing before initial and retry payload dispatch.

Audit 1 result: PASS, 8/8 criteria covered.

## Audit 2: Failure Mode Coverage

- Invalid input: COVERED by existing handler and payload builder tests for malformed requests and missing required subscription fields.
- Missing required tenant/channel: COVERED by partner handler tenant-context rejection.
- Duplicate/conflict: COVERED by tenant/channel preservation in subscription request mapping and tenant-scoped repository upsert code path.
- Dependency failure: COVERED by credential missing/unresolvable resolver tests and no-provider-call service test.
- Authorization: COVERED by trusted-service signature, body hash, timestamp, and replay tests in `common/auth/tenantctx`.

Audit 2 result: PASS, required failure categories covered.

## Audit 3: Domain Invariant Preservation

- No global TIMWE credential fallback for tenant-routed calls: PRESERVED by tenant provider config service test and fail-closed routing errors.
- Service-to-service tenant context must be signed and body/path-bound: PRESERVED by trusted-service tests and acquisition client signing test.
- Channel operation must be explicitly allowed: PRESERVED by capability policy and fail-closed service tests.
- Provider credential must resolve before outbound provider call: PRESERVED by credential resolver and fail-closed service tests.
- Secrets must not appear in captured request/audit metadata: PRESERVED by redaction test.
- Tenant/channel ownership persists into subscription/notification DTOs: PRESERVED by mapping tests and repository insert/upsert code.

Audit 3 result: PASS, 6/6 invariants preserved.

## Audit 4: User Journey Completeness

- Partner MT with signed tenant context resolves tenant route and sends tenant provider request: COMPLETE.
- Acquisition opt-in signs and forwards campaign tenant/channel context: COMPLETE.
- Missing tenant context is rejected before provider call: COMPLETE.
- Unsupported capability is rejected before provider call: COMPLETE.
- Missing credential is rejected before provider call: COMPLETE.
- Provider retry behavior remains covered by existing retry tests: COMPLETE.

Audit 4 result: PASS.

## Audit 5: Test Quality

Command:

```bash
/home/xper626/.agents/skills/value-gate/scripts/scan-test-quality.sh 'services/subscription-external/internal/**/*_test.go' 'services/acquisition-api/internal/**/*_test.go'
```

Results:

- Files scanned: 36
- Assertion-free tests: 0
- Status-only assertions: 0
- Total assertions: 292
- Status-only ratio: 0%
- Mock-heavy files: 0
- Zero-negative files reported: 3. The reported files are not assertion-free or status-only; one is a focused negative handler test, and the repository files belong to broader acquisition persistence coverage.

Audit 5 result: PASS.

## Verification Commands

```bash
cd services/subscription-external && go test ./...
cd services/acquisition-api && go test ./...
git diff --check
```

All commands passed on 2026-05-08.
