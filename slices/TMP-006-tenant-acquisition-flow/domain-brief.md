# TMP-006 Domain Brief: Tenant Acquisition Flow

## Actors

- end-subscriber: submits MSISDN and consent from a tenant campaign landing page.
- landing-web runtime: posts campaign slug, tenant key, attribution, consent, MSISDN, and HE bootstrap headers.
- acquisition-api: resolves campaign ownership, creates transactions, confirms OTP, and returns next action state.
- tenant campaign owner: previously configured tenant product, campaign, consent, and channel binding.
- subscription-external/TIMWE: existing downstream opt-in and confirm provider until TMP-007 adds tenant channel routing.

## Ubiquitous Language

- Tenant key: public route key sent by landing-web as `tenant_key`.
- Tenant id: durable database ownership key persisted on acquisition transactions.
- Campaign slug: public campaign label; tenant-owned slugs are not globally unique.
- Pending reuse: returning an existing pending/action-required transaction for the same campaign and MSISDN inside a TTL.
- Throttle: per-campaign, per-MSISDN/IP limiter from campaign config.
- Next action: OTP, redirect, open SMS, subscribed, or instruction state returned to the landing runtime.

## Domain Invariants

- Tenant-key transaction requests resolve campaigns by `(tenant_key, slug)`.
- Slug-only transaction requests remain legacy and can only resolve unscoped campaigns.
- Tenant-owned transactions persist `tenant_id`.
- Pending reuse, click-id reuse, and throttle checks are tenant-scoped for tenant campaigns.
- Consent is validated before provider calls and remains create-time state.
- Confirm and charge paths must not fall back to another tenant's campaign when a transaction carries tenant ownership.

## Failure Modes

- Tenant campaign mismatch: tenant key and slug lookup fails; API returns not found and creates no transaction.
- Missing consent: service returns 400-equivalent error before TIMWE opt-in.
- Duplicate provider click id: tenant campaigns check click id inside tenant scope before reusing.
- Same MSISDN and slug in different tenants: tenant-scoped pending reuse and throttles keep attempts isolated.
- Legacy unscoped campaign: still works with `tenant_id = NULL` for compatibility.
- Provider failure: existing transaction status and next-action behavior are preserved; conversion postbacks remain terminal-event only.

## User Journey

1. Subscriber opens `/lp/{tenant_key}/{slug}` and submits MSISDN/consent.
2. landing-web posts `/api/transactions` with `tenant_key` and `campaign_slug`.
3. acquisition-api resolves the enabled campaign under the active tenant.
4. service creates an acquisition transaction with tenant id, product context, attribution, HE context, consent, status, and next action.
5. confirm/status paths use transaction-scoped product and recover tenant campaign context when tenant id exists.

Failure journeys:

1. Tenant key does not own slug -> no transaction is created.
2. Consent-required campaign submitted without consent -> request fails before provider call.
3. Same provider click id appears in two tenants -> tenant-scoped lookup avoids cross-tenant reuse.

## Review Amendments Applied

- Parallel critique required tenant id persistence, tenant-scoped pending reuse/throttle/click-id checks, and repair of campaign slug FK migration ordering.
- Domain review clarified that tenant channel credential/provider routing belongs to TMP-007 and inbound callback correlation belongs to TMP-013.

## Open Questions

- A future schema pass should consider storing immutable `campaign_id` and `channel_id` on transactions rather than relying on `(tenant_id, campaign_slug)`.
- Full read-side tenant scanning can be broadened when confirm/status/callback slices expose tenant identity in responses or admin filters.
