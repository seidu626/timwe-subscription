# TMP-005 Domain Brief: Tenant Campaign Binding

## Actors

- tenant-admin: creates, lists, reads, updates, and enables campaigns inside a resolved active tenant context.
- campaign-operator: configures campaign flow, product mapping, and channel binding for tenant acquisition.
- landing runtime: fetches an enabled public campaign using explicit tenant key and slug.
- channel routing services: later use the campaign channel binding to select tenant provider credentials.

## Ubiquitous Language

- Tenant: active isolation boundary from `tenants`, resolved from trusted admin identity.
- Campaign: acquisition configuration row with slug, copy, flow, product mapping, and now tenant/channel ownership.
- Channel: tenant-owned provider/country/operator route with capabilities such as `optin`, `confirm`, `mt`, and `charge`.
- Product mapping: tenant product row whose `product_id` and `price_point_id` must match campaign offer fields.
- Public campaign: DTO from `Campaign.ToPublic()` that excludes internal product, postback, throttling, channel, and admin fields.

## Domain Invariants

- Admin campaign CRUD requires resolved active tenant context.
- Campaign slug uniqueness is scoped by tenant; legacy unscoped campaigns remain unique while unmigrated.
- Campaign product mapping must resolve inside the same tenant.
- Campaign channel binding must resolve inside the same tenant and be active.
- OTP campaigns require channel capabilities `optin` and `confirm`.
- Public tenant campaign lookup uses explicit `{tenant_key}/{slug}` and returns only enabled campaigns for active tenants.
- Legacy `/v1/campaigns/{slug}` remains available for existing unscoped traffic until TMP-012 completes routing migration.

## Failure Modes

- Missing tenant context: admin campaign list/create/read/update/enable return 403 before service mutation.
- Duplicate tenant slug: database uniqueness maps to `campaign_conflict` and handler returns 409.
- Tenant product mismatch: create/update returns 400 and does not persist a campaign.
- Channel not found/cross-tenant/country mismatch: create/update returns 400.
- Inactive channel: create/update returns conflict.
- Capability mismatch: OTP campaign on a channel missing `optin` or `confirm` returns 422.
- Disabled public campaign: explicit public tenant route returns 404.

## User Journey

1. Tenant admin calls `POST /v1/admin/campaigns` with tenant JWT context, `channel_id`, offer product mapping, and flow.
2. Handler resolves the active tenant and passes tenant id to the campaign service.
3. Service validates tenant product mapping and tenant channel compatibility.
4. Repository inserts a campaign row with `tenant_id` and `channel_id`; duplicate slug inside that tenant returns 409.
5. Landing runtime calls `GET /v1/campaigns/{tenant_key}/{slug}` and receives a public-safe campaign only when tenant and campaign are active.

Failure journeys:

1. Admin omits tenant context -> 403.
2. Admin uses another tenant's product or channel -> 400.
3. Admin binds OTP to a channel without `confirm` -> 422.
4. Public caller requests disabled or unknown tenant campaign -> 404.

## Review Amendments Applied

- Independent critique required replacing global admin campaign slug access with tenant-scoped methods.
- Product validation was changed from global product lookup to tenant-scoped product lookup.
- Channel compatibility was defined in current channel vocabulary: `OTP => optin + confirm`, `MIXED => optin`, `CLICK_TO_SMS => mt`, `REDIRECT => active channel only`.
- Public route ambiguity is bounded to an explicit `/v1/campaigns/{tenant_key}/{slug}` route; host/header/gateway routing is deferred to TMP-012.

## Open Questions

- TMP-012 must decide legacy `/v1/campaigns/{slug}` behavior once tenant routing is available from host, gateway, or trusted service context.
- Campaign clone and postback-rule admin subroutes remain legacy/global and should be tenant-scoped in a follow-up hardening slice if they stay exposed.
