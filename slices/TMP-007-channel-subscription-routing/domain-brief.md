# TMP-007 Domain Brief: Channel Subscription Routing

## Actors

- api-integrated-partner: calls partner-facing MT, charge, status, optout, and confirm endpoints and must supply tenant/channel context for tenant-scoped routing (source: `slices/TMP-007-channel-subscription-routing/slice.yaml`, `services/subscription-external/internal/handler/partner_handler.go`).
- acquisition service: starts tenant campaign opt-in flows and forwards campaign-derived tenant/channel context into subscription-external (source: `services/acquisition-api/internal/service/transaction_service.go`, `services/acquisition-api/internal/service/timwe_client.go`).
- subscription-external service: builds TIMWE provider requests, persists subscription/notification/admin action state, and owns retry/circuit-breaker behavior for outbound calls (source: `services/subscription-external/internal/service/subscription.go`, `services/subscription-external/internal/repository/postgres.go`).
- tenant admin / platform operator: provisions tenant channels and credential references that routing must consume without exposing raw secrets (source: `slices/TMP-003-channel-catalog/domain-brief.md`, `slices/TMP-004-channel-credential-binding/domain-brief.md`).

## Ubiquitous Language

- Tenant context: trusted tenant identity carried by JWT or signed service headers; service-to-service calls must use the shared HMAC header contract rather than unsigned tenant headers (source: `common/auth/tenantctx/identity.go`, `common/auth/tenantctx/trusted_service.go`).
- Channel: tenant-owned provider/country/operator/capability catalog entry, distinct from legacy TIMWE entry channel strings such as `WEB`, `SMS`, or `INTERNAL` (source: `services/acquisition-api/internal/domain/admin_management.go`, `services/acquisition-api/migrations/add_tenant_channels.sql`).
- Capability: closed operation set currently allowing `optin`, `confirm`, `mt`, and `charge`; this slice treats `status` and `optout` as subscription operations that require explicit routing policy rather than free-form fallback (source: `services/acquisition-api/internal/service/admin_management_service.go`, `slices/TMP-007-channel-subscription-routing/slice.yaml`).
- Credential reference: opaque secret pointer stored in `tenant_channel_credentials`; the database stores references, displays, and fingerprints only, never raw credential values (source: `services/acquisition-api/migrations/add_tenant_channel_credentials.sql`, `slices/TMP-004-channel-credential-binding/domain-brief.md`).
- TIMWE provider config: outbound URL, partner role, API key/authentication material, realm, and retry behavior currently read from global config and used by subscription-external request builders (source: `common/config/config.go`, `services/subscription-external/internal/service/subscription.go`, `services/subscription-external/internal/service/admin_actions.go`).
- Subscription identity: currently keyed by partner role, user identifier, and product; TMP-007 must add tenant ownership so two tenants can subscribe the same MSISDN/product without conflict (source: `services/subscription-external/internal/repository/postgres.go`).

## Domain Invariants

- Tenant-scoped routing never falls back to global TIMWE credentials when tenant/channel context is present or required (source: `slices/TMP-007-channel-subscription-routing/slice.yaml`).
- Tenant/channel context for service-to-service requests must be signed and body/path-bound; unsigned `X-Tenant-*` headers are not trusted (source: `common/auth/tenantctx/trusted_service.go`).
- A route can use only an active channel owned by the tenant and only for operations allowed by that channel's capabilities (source: `services/acquisition-api/migrations/add_tenant_channels.sql`, `slices/TMP-003-channel-catalog/domain-brief.md`).
- Provider calls require an active credential for `(tenant_id, channel_id, purpose='provider_api')`; missing or unresolvable credentials fail before any outbound provider call (source: `services/acquisition-api/migrations/add_tenant_channel_credentials.sql`, `slices/TMP-004-channel-credential-binding/domain-brief.md`).
- Secret values and authentication headers must not appear in HTTP responses, admin action details, or structured logs (source: `slices/TMP-004-channel-credential-binding/domain-brief.md`, `services/subscription-external/internal/service/admin_actions.go`).
- Subscription and notification persistence for tenant-routed calls includes tenant ownership so duplicate MSISDN/product pairs across tenants do not collide (source: `services/subscription-external/internal/repository/postgres.go`, `slices/TMP-007-channel-subscription-routing/slice.yaml`).

## Failure Modes

- Partner MT:
  - Invalid input: malformed JSON or missing MSISDN/product remains a 400-style request failure.
  - Missing required tenant/channel: no trusted tenant identity or no channel identifier returns 400/403 and no provider call.
  - Duplicate/conflict: same MSISDN/product in a different tenant must not conflict with this tenant's subscription record.
  - Dependency failure: missing credential reference or unresolvable secret returns a configuration/dependency error before TIMWE is called.
  - Authorization: forged or unsigned tenant headers are rejected.
- Partner charge:
  - Invalid input: missing charge fields returns 400 and does not create a notification.
  - Missing required tenant/channel: tenant-routed charge without channel fails before provider call.
  - Unsupported operation: channel without `charge` capability returns `unsupported_channel_operation`.
  - Dependency failure: provider timeout follows existing retry/circuit-breaker behavior without duplicate tenant records.
  - Authorization: tenant/channel mismatch returns not found or forbidden.
- Admin optin / optout / confirm / status:
  - Invalid input: operation-specific required fields such as `transactionAuthCode` for confirm remain enforced.
  - Missing required tenant/channel: tenant admin request without resolvable tenant/channel fails before URL/header construction.
  - Unsupported operation: operation not allowed by the channel returns 422.
  - Dependency failure: credential resolution failure writes redacted audit state only.
  - Authorization: admin action history and details do not expose another tenant's routing context.
- Acquisition optin:
  - Invalid input: transaction without tenant/campaign context cannot be promoted into provider routing.
  - Missing required channel: tenant campaign without channel_id returns a dependency/config error.
  - Duplicate/conflict: retrying the same acquisition flow reuses tenant-scoped transaction/subscription identity only within the same tenant.
  - Dependency failure: TIMWE timeout does not create duplicate subscription rows or cross-tenant postback state.
  - Authorization: service-to-service headers must match the transaction tenant.

## User Journey

1. API-integrated partner calls `POST /api/external/v1/{channel}/mt` with signed tenant service context and a tenant channel identifier or key.
2. subscription-external verifies the tenant signature, resolves the active tenant channel, checks the `mt` or `optin` capability required by the operation, resolves the active provider credential reference, and builds the provider request from tenant channel config.
3. subscription-external sends TIMWE with tenant credentials, persists subscription/notification/admin audit rows with tenant/channel ownership, and returns the provider response without secret material.
4. Acquisition flow calls subscription-external with tenant/channel context derived from the campaign/transaction rather than relying on global TIMWE config.

Failure journeys:
1. Partner omits signed tenant context -> request is rejected and no provider request is attempted.
2. Tenant channel lacks the requested operation capability -> 422 `unsupported_channel_operation`, no provider call.
3. Channel credential is missing or cannot be resolved -> dependency/config error with redacted logs and audit details.
4. Existing `SMS` retry would bypass tenant channel policy -> retry is allowed only if it resolves to an active tenant channel with the required capability.

## Open Questions

- The local repository does not include `docs/SLICE-METHODOLOGY.md`; this brief uses `slices/README.md`, slice YAML, and code as the active methodology sources.
- TMP-004 stores credential references but does not ship a production vault backend. TMP-007 should provide an interface and a local/env-backed resolver for testability, while TMP-015 owns production secret operations hardening.
- The channel catalog currently allows `optin`, `confirm`, `mt`, and `charge`; this slice must either extend the closed set for `status`/`optout` or map those operations to explicit routing policy without pretending free-form entry channels are tenant channels.
