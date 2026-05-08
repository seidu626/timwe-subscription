# TMP-018 Domain Brief

## Actors

- Platform operator: defines tenant claim, role, service-auth, and trust-boundary contract for all platform services (source: `slices/TMP-018-tenant-claim-and-service-auth-contract/slice.yaml`).
- Tenant admin: authenticates through Auth0-backed admin routes and should carry tenant identity into protected admin operations (source: `services/acquisition-api/internal/transport/admin.go`, `services/cadence-engine/internal/adminhttp/access.go`).
- Internal service: calls protected service-to-service routes using signed headers rather than public client-supplied tenant data (source: `services/acquisition-api/internal/handler/internal_handler.go`, `services/subscription-external/internal/service/acquisition_client.go`).
- Gateway: validates edge concerns and forwards only trusted context to services (source: `krakend/config/templates/AdminApiEndpoint.tmpl`, `krakend/config/templates/TimweApiEndpoint.tmpl`).

## Ubiquitous Language

- Auth0 JWT validator: common RS256/JWKS validator for admin routes (source: `common/auth/auth0jwt/validator.go`).
- Registered claims: issuer, subject, audience, expiry, issued-at, and token ID from `jwt.RegisteredClaims` (source: `common/auth/auth0jwt/validator.go`).
- Tenant claim: custom JWT fields such as `tenant_id`, `tenant_key`, `org_id`, namespaced role/permission claims, or Auth0 organization data that identify the tenant context (source: `slices/TMP-018-tenant-claim-and-service-auth-contract/slice.yaml`).
- Platform scope: role or permission that allows all-tenant/platform operations, e.g. `platform_operator` or `platform:all_tenants` (source: `slices/decisions/README.md`, `slices/TMP-018-tenant-claim-and-service-auth-contract/slice.yaml`).
- Trusted service header: signed internal tenant context with method, path, timestamp, nonce, tenant, service id, optional body hash, and HMAC signature (source: `services/acquisition-api/internal/handler/internal_handler.go` existing HMAC convention; generalized in this slice).
- Request context: Go `context.Context` or fasthttp user values that carry resolved identity after validation (source: `services/acquisition-api/internal/handler/he_context.go` uses `ctx.SetUserValue`; cadence uses `r.Context()`).

## Domain Invariants

- Tenant context has one typed representation: JWT and trusted-service flows must produce `tenantctx.Identity`, not unrelated string parsing in each service (source: `slices/TMP-018-tenant-claim-and-service-auth-contract/slice.yaml`).
- Invalid issuer/audience/signature must fail before identity is attached to request context (source: `common/auth/auth0jwt/validator.go`, `slices/TMP-018-tenant-claim-and-service-auth-contract/slice.yaml`).
- Platform scope is explicit: tenant admins must not become platform-scoped unless role or permission says so (source: `slices/TMP-018-tenant-claim-and-service-auth-contract/slice.yaml`).
- Service-auth tenant context must be signed, time-bound, tenant-bound, and nonce-protected; unsigned, stale, replayed, or tampered headers are not trust signals (source: `services/acquisition-api/internal/handler/internal_handler.go` timestamp/HMAC behavior; `slices/TMP-018-tenant-claim-and-service-auth-contract/slice.yaml`).

## Failure Modes

- Operation: Validate admin bearer token
  - Invalid input: malformed or non-Bearer Authorization header returns missing/invalid token.
  - Missing required: missing token returns unauthorized and no identity is attached.
  - Duplicate/conflict: tenant route and token tenant mismatch must produce authorization denial in follow-on route enforcement.
  - Dependency failure: unconfigured Auth0 validator makes admin access unavailable rather than permissive.
  - Concurrent access: validator must not mutate shared claims between requests.
  - Authorization: audience or issuer mismatch rejects before identity exists.

- Operation: Resolve trusted service headers
  - Invalid input: malformed timestamp or signature returns unauthorized.
  - Missing required: missing tenant/service/timestamp/signature/nonce where nonce enforcement is enabled fails closed.
  - Duplicate/conflict: tenant id/key tampering after signature creation is rejected.
  - Dependency failure: missing service secret rejects instead of accepting headers.
  - Concurrent access: middleware attaches identity to only the current request context.
  - Authorization: public clients cannot self-assert trusted service headers without a valid signature.

## User Journey

1. Platform operator configures Auth0 domain/audience and trusted service secret.
2. Tenant admin calls an admin endpoint with an Auth0 JWT.
3. Validator checks bearer format, RS256 signature, issuer, audience, expiry, issued-at, and clock skew.
4. Validator returns typed claims with tenant, org, roles, permissions, subject, and platform scope.
5. Admin middleware attaches `tenantctx.Identity` to the request.
6. Internal service sends tenant context with method, path, service id, timestamp, nonce, optional body hash, and HMAC signature.
7. Trusted middleware verifies the signature and attaches `tenantctx.Identity`.

Failure journeys:

1. Tenant admin sends token with wrong audience -> validator rejects and middleware attaches no identity.
2. External client sends forged tenant/service headers -> trusted middleware rejects and attaches no identity.
3. Service header timestamp is outside skew window -> trusted middleware rejects replay.
4. Service header nonce is reused -> trusted middleware rejects replay.

## Open Questions

- Which tenant claim source is final: Auth0 organization, custom namespaced claim, internal tenant table mapping, or hybrid?
- Which service-auth contract is canonical for every service: new `X-Service-*` headers, existing `X-Internal-*` HMAC extension, mTLS, or gateway-signed headers?
- What is the final role/permission vocabulary for tenant admin, campaign operator, operations analyst, platform operator, and service account?
