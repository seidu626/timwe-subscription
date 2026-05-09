# TMP-018 Value Gate Report

Verdict: PASS

Timestamp: 2026-05-08T04:18:00Z
Agent: codex

## Audit 1: Acceptance Criteria Coverage

- Criterion: Typed tenant claims are extracted
  - Test file: `common/auth/auth0jwt/claims_test.go`
  - Test name: `TestClaimsUnmarshalExtractsTenantRoleAndPlatformScope`
  - Assertion type: body/state assertions on tenant_id, tenant_key, org_id, subject, roles, permissions, platform scope, trust source
  - Verdict: COVERED
- Criterion: Trusted service header is accepted
  - Test file: `common/auth/tenantctx/trusted_service_test.go`
  - Test name: `TestIdentityFromTrustedHeadersAcceptsSignedTenantContext`, `TestMiddlewareAttachesIdentityToRequestContext`
  - Assertion type: identity state and middleware context assertions
  - Verdict: COVERED
- Criterion: Audience mismatch rejected before identity exists
  - Test file: `common/auth/auth0jwt/validator_test.go`, `services/acquisition-api/internal/transport/admin_test.go`
  - Test name: `TestValidateBearerRejectsAudienceMismatch`, `TestAdminRequireRejectsAudienceMismatchBeforeIdentity`
  - Assertion type: error substring, 401 status, and missing identity assertion
  - Verdict: COVERED
- Criterion: Forged service header rejected
  - Test file: `common/auth/tenantctx/trusted_service_test.go`
  - Test name: `TestIdentityFromTrustedHeadersRejectsForgedSignature`
  - Assertion type: error assertion
  - Verdict: COVERED
- Criterion: Tenant claim mismatch / tampering rejected
  - Test file: `common/auth/tenantctx/trusted_service_test.go`
  - Test name: `TestIdentityFromTrustedHeadersBindsTenantKeyToSignature`
  - Assertion type: positive signed key assertion and negative tamper assertion
  - Verdict: COVERED
- Criterion: Clock skew and replay window
  - Test file: `common/auth/tenantctx/trusted_service_test.go`
  - Test name: `TestIdentityFromTrustedHeadersRejectsExpiredTimestamp`, `TestMiddlewareRejectsReplayNonce`
  - Assertion type: expired timestamp and duplicate nonce rejection
  - Verdict: COVERED
- Criterion: Cadence static admin token attaches explicit platform scope
  - Test file: `services/cadence-engine/internal/adminhttp/access_test.go`
  - Test name: `TestRequireWithStaticAdminTokenAttachesPlatformIdentity`
  - Assertion type: request context identity assertions on platform scope, service id, and trust source
  - Verdict: COVERED

## Audit 2: Failure Mode Coverage

- invalid_input: malformed/forged signature covered by `TestIdentityFromTrustedHeadersRejectsForgedSignature`.
- missing_required: missing/unconfigured service secret and header checks are enforced in `IdentityFromTrustedRequest`; `TestIdentityFromTrustedHeadersRejectsMissingTenant` exercises fail-closed missing tenant behavior.
- duplicate_conflict: nonce replay covered by `TestMiddlewareRejectsReplayNonce`.
- dependency_failure: unconfigured Auth0 validator behavior is preserved by existing admin middleware behavior; service secret missing fails closed in trusted header resolver.
- authorization: audience mismatch covered by validator and acquisition admin tests.

Verdict: PASS

## Audit 3: Domain Invariant Preservation

- Tenant context has one typed representation
  - Positive: `Claims.Identity()` and middleware context tests prove JWT and trusted-service flows produce `tenantctx.Identity`.
  - Negative: audience mismatch and forged header tests prove invalid paths attach no identity.
  - Verdict: PRESERVED
- Invalid issuer/audience/signature fails before identity is attached
  - Positive: valid token/header tests attach identity.
  - Negative: `TestAdminRequireRejectsAudienceMismatchBeforeIdentity`, `TestIdentityFromTrustedHeadersRejectsForgedSignature`.
  - Verdict: PRESERVED
- Platform scope is explicit
  - Positive: `TestClaimsUnmarshalExtractsTenantRoleAndPlatformScope`.
  - Negative: platform scope derives only from explicit role/permission checks in `tenantctx.PlatformScoped`.
  - Verdict: PRESERVED
- Service-auth tenant context is signed, time-bound, tenant-bound, and nonce-protected
  - Positive: signed tenant header accepted.
  - Negative: tenant key tamper, expired timestamp, forged signature, replay nonce.
  - Verdict: PRESERVED

## Audit 4: User Journey Completeness

- Platform operator configures Auth0/domain audience: implemented through existing validator constructor; tested with `NewWithKeyfunc` unit constructor for deterministic validation.
- Tenant admin calls admin endpoint with Auth0 JWT: implemented in acquisition admin middleware; tested in `TestAdminRequireStoresTenantIdentity`.
- Validator returns typed claims: implemented in `auth0jwt.Claims`; tested.
- Admin middleware attaches `tenantctx.Identity`: implemented for acquisition and cadence; acquisition JWT path and cadence static-token path tested directly.
- Internal service sends signed tenant context: implemented as reusable signed header helper/middleware in `tenantctx`; tested.

Failure journey coverage: wrong audience, forged service header, stale timestamp, nonce replay, and tenant-key tampering are complete.

Verdict: PASS

## Audit 5: Test Quality

Scanner unavailable: `scripts/scan-test-quality.sh` is not present in this repository. Manual anti-pattern check performed for new tests:

- Assertion-free tests: 0 observed.
- Status-only assertions: 0 status-only tests; status checks include missing identity or context assertions.
- No negative tests: negative auth tests added for audience mismatch, forged signature, missing tenant context, stale timestamp, nonce replay, and tenant-key tampering.
- Mock everything: RSA validator tests use real signed JWTs with test keyfunc; service-auth tests use real HMAC signatures.

Verdict: PASS

## Verification Commands

- `cd common && go test ./auth/...`
- `cd services/acquisition-api && go test ./internal/transport`
- `cd services/cadence-engine && go test ./internal/adminhttp`
- `git diff --check`

Attempted repository vendor guard:

- `scripts/check-vendor-sync.sh`
- Result: blocked by existing `services/notification` module hygiene (`golang.org/x/exp/slog` missing go.sum entry via `github.com/sagikazarmark/slog-shim`). TMP-018 does not modify notification; accidental diagnostic go.mod/go.sum changes were reverted.

## Current superseding evidence

TMP-044 reran the current notification verification from `origin/main`. `cd services/notification && go test ./...` passed with 18 tests across 11 packages, so the historical notification module-hygiene note above is no longer a current blocker for this slice. No notification source, module, or sum files changed for this reconciliation.

## Follow-Up Risks

- The older hardcoded `dgrijalva/jwt-go` middlewares remain out of TMP-018 scope and should be handled by a later security cleanup slice.
- KrakenD header stripping/injection contract is documented by the slice but not yet modified; TMP-012 owns public/gateway routing.
- Full route-level tenant claim mismatch enforcement requires concrete route tenant resolution from TMP-012/TMP-001.
- `scripts/check-vendor-sync.sh` currently cannot complete until the unrelated notification module go.sum drift is repaired.
