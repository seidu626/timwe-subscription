# Plan Critique And Amendments

This pass reviews the first tenant multi-channel intake package against the stated target: use the current subscription/acquisition repository as intake for a tenant-aware, multi-channel platform.

## Critique

1. The original roadmap correctly starts with tenant context, but it did not make public tenant resolution explicit enough. Landing pages, campaign reads, HE bootstrap, callbacks, and gateway routing need one shared policy for host/path/header/signed-token resolution before duplicate campaign slugs can be safe.
2. Channel modeling was present, but inbound callback correlation was implicit. TIMWE/MNO callbacks, partner webhooks, notification events, and charge-success callbacks need tenant/channel correlation rules independent of outbound credential routing.
3. The admin frontend was inventoried but not converted into work. A tenant platform is not complete if APIs are tenant-aware while `webspa-admin` can still present global campaign, product, userbase, cadence, postback, or report state.
4. Secret and config risks were documented but not framed as release-blocking work. The current compose/config surface includes credential-shaped values, and tenant credential binding cannot be safely delivered without rotation, redaction, and environment hygiene.
5. Partner onboarding was missing as its own workstream. Multi-channel platforms need contracts: API versioning, callback templates, credential exchange, channel capability docs, and sandbox validation.
6. Billing remains ambiguous. The disabled `services/billing` service and existing subscription-external charge/renewal flows need an ownership decision before charge-capable tenant channels scale.
7. Several slices were sized at the upper edge. The plan should treat each large slice as a thin vertical path first and push bulk migration or UI breadth behind proof of tenant isolation.

## Amendments Applied

1. Added `TMP-012-public-tenant-routing` for gateway, landing, campaign, and callback tenant resolution.
2. Added `TMP-013-inbound-callback-correlation` for TIMWE/MNO/partner callback tenant/channel attribution.
3. Added `TMP-014-admin-portal-tenant-workspace` for visible tenant workspace, route guards, filters, and cross-tenant UI denial behavior.
4. Added `TMP-015-platform-ops-secret-observability` for secret hygiene, config rotation posture, tenant metrics labels, and operational dashboards.
5. Added `TMP-016-partner-channel-onboarding-contracts` for partner/channel API contracts, sandbox fixtures, callback templates, and onboarding runbooks.
6. Added `TMP-017-billing-charge-ownership` for deciding and proving charge ownership across the disabled billing service and subscription-external charge flows.
7. Tightened roadmap sequencing so public tenant routing gates tenant acquisition, inbound callback correlation gates notification/cadence routing, and ops hardening gates production readiness.
8. Integrated Claude async review findings by adding `TMP-018-tenant-claim-and-service-auth-contract`, `TMP-019-tenant-asset-namespacing`, and `TMP-020-tenant-observability-baseline`.

## Slice Readiness Rule

Before implementation starts on any slice, the implementer must confirm:

- Actor and entrypoint are specific enough to test end to end.
- Tenant isolation has both positive and negative test criteria.
- Channel capability behavior is defined where the slice touches acquisition, subscription, notification, charge, or postback paths.
- Existing single-tenant Ghana/TIMWE behavior remains covered.
- Secret, PII, and audit implications are either handled in-slice or explicitly delegated to a dependent slice.
