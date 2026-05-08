# TMP-012 Domain Brief: Public Tenant Routing

## Actors

- end-subscriber: opens a public landing path and expects the correct tenant campaign.
- landing runtime: fetches campaign metadata and client campaign config for the landing page.
- trusted gateway: forwards signed tenant context after host/path mapping.
- acquisition-api: resolves tenant campaign requests and HE bootstrap campaign routes.
- telco/operator proxy: enters HE bootstrap behind the existing trusted proxy gate.

## Ubiquitous Language

- Tenant route: explicit public path segment `{tenant_key}` used with campaign slug.
- Legacy route: existing `/lp/{slug}` and `/v1/campaigns/{slug}` routes for unscoped campaigns.
- Trusted tenant headers: HMAC-signed `X-Tenant-*` service context from `tenantctx`.
- Public campaign: public-safe DTO returned after campaign and tenant status checks.
- HE campaign route: campaign route stored in bootstrap token and used for HTTPS redirect.

## Domain Invariants

- Tenant-owned campaigns are never served through ambiguous slug-only public lookup.
- Explicit public path `/v1/campaigns/{tenant_key}/{slug}` resolves only active tenants and enabled campaigns.
- Raw public tenant headers are rejected unless the trusted-service signature is valid.
- Landing-web preserves tenant context when fetching metadata and client campaign config.
- HE bootstrap preserves tenant route context through redirect.

## Failure Modes

- Ambiguous slug without tenant context: legacy acquisition lookup filters to `tenant_id IS NULL`, so tenant-owned duplicates return 404 instead of arbitrary selection.
- Forged tenant header: unsigned or invalid trusted tenant headers return 403 before public lookup.
- Disabled tenant: tenant-key campaign lookup joins active tenants only.
- Invalid HE tenant route: unsafe tenant/slug path segments return 400.
- Missing landing tenant: legacy `/lp/{slug}` remains compatible only with legacy unscoped campaigns.

## User Journey

1. Subscriber opens `/lp/{tenant_key}/{slug}`.
2. Landing metadata and client fetch `/api/campaigns/{tenant_key}/{slug}`.
3. Landing proxy calls acquisition-api `/v1/campaigns/{tenant_key}/{slug}`.
4. Acquisition-api returns public campaign config only for active tenant and enabled campaign.
5. If HE capture runs, `/v1/he/bootstrap/campaign/{tenant_key}/{slug}` redirects back to `/lp/{tenant_key}/{slug}` with HE token.

Failure journeys:

1. Subscriber opens `/lp/{slug}` for a tenant-owned campaign -> campaign lookup does not fall back to another tenant.
2. Public caller supplies raw `X-Tenant-Key` without signature -> 403.
3. HE bootstrap path contains unsafe segments -> 400.

## Review Amendments Applied

- Parallel critique required explicit handling for legacy slug ambiguity, landing-web tenant path compatibility, forged tenant headers, and HE tenant route preservation.
- Transaction and callback tenant persistence are deliberately deferred to TMP-006 and TMP-013 to avoid mixing campaign routing with transaction ownership.

## Open Questions

- Host-based tenant mapping and KrakenD config fixtures still need an environment-specific routing contract.
- Transactions still submit `campaign_slug`; TMP-006 must attach tenant campaign identity before subscription flow work begins.
