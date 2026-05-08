# Tenant Multi-Channel Platform Domain Brief

Goal: extend the existing TIMWE subscription/acquisition repository into a tenant-aware, multi-channel platform without losing the current Ghana/TIMWE web acquisition, subscription, notification, cadence, postback, and reporting flows.

## Actors

- Platform operator: configures global platform services, deploys services, manages infrastructure, and sets defaults. Sources: `docker-compose.yml`, `Makefile`, `docs/environment-variables.md`.
- Tenant admin: manages tenant-owned products, userbase records, campaigns, message cadence, postbacks, and reports through protected admin surfaces. Sources: `services/acquisition-api/internal/transport/router.go`, `services/acquisition-api/internal/domain/admin_management.go`, `frontend/webspa-admin/src/app/app.routes.ts` from main checkout intake.
- Campaign operator: creates and updates campaigns, landing copy, assets, postback rules, enablement, landing URLs, and tracking configuration. Sources: `services/acquisition-api/internal/domain/campaign.go`, `services/acquisition-api/README.md`.
- End subscriber: enters or supplies MSISDN through landing, SMS, HE, or partner channel and receives opt-in, OTP, renewal, or opt-out outcomes. Sources: `services/landing-web/README.md`, `services/acquisition-api/internal/domain/transaction.go`, `services/subscription-external/internal/domain/subscription.go`.
- API-integrated partner: sends partner MT, charge, status, optout, and optin confirm calls. Sources: `services/subscription-external/internal/transport/router.go`, `services/subscription-external/internal/handler/partner_handler.go`.
- Ad or traffic partner: provides attribution identifiers and receives postbacks for subscribed/conversion/failure events. Sources: `services/acquisition-api/internal/service/ad_provider.go`, `services/acquisition-api/internal/domain/postback.go`.
- MNO or TIMWE upstream: receives outbound subscription actions and sends notification or webhook data. Sources: `services/subscription-external/README.md`, `services/subscription-partner/internal/handler/notification_webhook_handler.go`.
- Operations analyst: inspects health, failures, charging, queues, DLQ, metrics, renewal, and funnel reports. Sources: `services/subscription-external/internal/transport/router.go`, `services/acquisition-api/internal/domain/reports.go`, `ops/monitoring`.

## Ubiquitous Language

- Tenant: TO BE DEFINED. The code currently uses partner, product, campaign, country, operator, and channel fields, but no canonical tenant model or `tenant_id`.
- Channel: entry or delivery path such as SMS, HE, landing-web, MT, charge, or partner API. Sources: `entry_channel` in `services/pg_schema.sql`, `PartnerMTHandler(ctx, channel)` in `services/subscription-external/internal/transport/router.go`.
- Campaign: marketing configuration with slug, country, operator, flow type, offer product, partner role, attribution, postback, throttles, LP URLs, tracking config, and LP copy. Source: `services/acquisition-api/internal/domain/campaign.go`.
- Acquisition transaction: a web acquisition attempt with campaign, MSISDN, status, next action, attribution, consent, HE, TIMWE, and charge state. Source: `services/acquisition-api/internal/domain/transaction.go`.
- Flow type: `CLICK_TO_SMS`, `OTP`, `REDIRECT`, `MIXED`. Sources: `services/acquisition-api/internal/domain/campaign.go`, `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`.
- Next action: `OPEN_SMS`, `OTP`, `REDIRECT`, `SHOW_INSTRUCTIONS`, `SUBSCRIBED`. Source: `services/acquisition-api/internal/domain/transaction.go`.
- Partner role: TIMWE partner role identifier used across subscription, products, notifications, campaigns, and cadence. Sources: `services/pg_schema.sql`, `services/cadence-engine/internal/domain/types.go`.
- Product: purchasable subscription offer with product ID, price point, price, shortcode, and name. Sources: `services/pg_schema.sql`, `services/subscription-partner/internal/domain/product.go`.
- Userbase: MSISDN allowlist or segment records. Sources: `services/pg_schema.sql`, `services/acquisition-api/internal/domain/admin_management.go`.
- Postback outbox: queued external postback delivery with attempt, retry, status, and DLQ lifecycle. Sources: `services/acquisition-api/internal/domain/postback.go`, `services/acquisition-api/migrations/create_postback_tables.sql`.
- Message cadence: product message series, schedule rules, content items, subscription message state, and message outbox. Sources: `services/subscription-external/migrations/011_message_cadence_engine.sql`, `services/cadence-engine/internal/domain/types.go`.
- Renewal: opt-out/opt-in or direct charge renewal strategy with churn policy and priority retry. Source: `services/subscription-external/internal/domain/renewal.go`.
- Header Enrichment: operator-provided identity bootstrap and token exchange for MSISDN/operator detection. Sources: `services/acquisition-api/internal/handler/he_bootstrap_handler.go`, `docs/HE_HTTP_BOOTSTRAP_GUIDE.md`.

## Domain Invariants

- Campaign slug is globally unique today; tenant platform must preserve uniqueness per tenant and reject cross-tenant collisions at API and DB levels. Current source: `campaigns.slug UNIQUE` in `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`.
- Subscription identity is currently partner role + user identifier + product; tenant platform must scope the same identity by tenant and channel where applicable. Current source: `subscriptions` in `services/pg_schema.sql`.
- Consent records must remain immutable and tied to the acquisition transaction and landing version/hash. Source: `consents` in `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`.
- Postbacks must be idempotent and recoverable through outbox status, attempts, retry, and DLQ. Sources: `services/acquisition-api/internal/domain/postback.go`, `services/postback-dispatcher/README.md`.
- Admin mutations must be authenticated and auditable. Sources: `/v1/admin/*` auth in `services/acquisition-api/internal/transport/router.go`, `AdminActivityLog` in `services/acquisition-api/internal/domain/admin_management.go`.
- Channel/provider credentials must never be exposed in public campaign, landing page, postback response bodies, logs, or client-side bundles. Current risk source: `.env` files exist locally and compose/schema files include credential-shaped values.
- Tenant reports must not aggregate data across tenants unless the actor is explicitly platform-scoped. Current report filters support date, campaign, and country only in `services/acquisition-api/internal/domain/reports.go`.
- HE simulation must never run in production. Source: fail-fast guard in `services/acquisition-api/cmd/main.go`.
- Message cadence outbox idempotency key must remain unique. Source: `message_outbox.idempotency_key UNIQUE` in `services/subscription-external/migrations/011_message_cadence_engine.sql`.

## Failure Modes

### Tenant context propagation

- Missing required: admin/API request has no tenant context after login or gateway pass-through, so protected endpoints must return 401/403 and perform no mutation.
- Invalid input: tenant ID is malformed or unknown, so repositories return not found instead of falling back to global data.
- Duplicate/conflict: tenant slug/name conflicts with an existing tenant, so API returns 409 and no partial tenant is created.
- Dependency failure: Auth0 or gateway claims unavailable, so service rejects protected request instead of guessing tenant.
- Concurrent access: two creates for same tenant key race, DB unique constraint wins and one request returns deterministic conflict.
- Authorization: tenant admin requests another tenant's campaign or product, so API returns 403/404 without leaking existence.

### Channel catalog and credential binding

- Missing required: channel lacks provider, capability, country/operator scope, or credential reference, so create/update returns 400.
- Invalid input: unsupported capability or flow type is rejected before it can be used by acquisition/subscription.
- Duplicate/conflict: same provider/channel key for a tenant conflicts and returns 409.
- Dependency failure: secret storage unavailable, so credentials are not stored in DB plaintext and the channel remains inactive.
- Concurrent access: channel enable/disable races preserve one final version with audit record.
- Authorization: tenant admin cannot inspect credentials or channels from another tenant.

### Tenant campaigns and acquisition

- Missing required: campaign lacks tenant, product, partner role, flow type, or channel binding, so API rejects it.
- Invalid input: campaign binds to a channel that does not support the requested flow, so API returns 422.
- Duplicate/conflict: same campaign slug under same tenant conflicts; same slug under different tenant is allowed only when public routing includes tenant context.
- Dependency failure: TIMWE/subscription-external unavailable, so transaction becomes failed/pending according to existing semantics and postback is not falsely emitted.
- Concurrent access: repeated pending acquisition reuses pending transaction TTL per tenant/campaign/MSISDN.
- Authorization: public campaign fetch cannot disclose disabled or cross-tenant campaigns.

### Subscription, notification, cadence, and renewal routing

- Missing required: service-to-service subscription request lacks tenant/channel, so request is rejected rather than routed to default TIMWE credentials.
- Invalid input: channel cannot perform MT/charge/optin/renewal, so the operation returns a capability error.
- Duplicate/conflict: duplicate notifications or cadence jobs preserve idempotency keys and do not send duplicate messages.
- Dependency failure: TIMWE, notification worker, or postback target timeout results in retry/outbox state, not data loss.
- Concurrent access: multiple worker replicas claim outbox work without duplicate sends.
- Authorization: tenant admin can only see queues, notifications, cadence, renewal, and failures for own tenant.

### Reporting and operations

- Missing required: report request without tenant context fails for tenant actors.
- Invalid input: unsupported channel/provider filter returns 400.
- Duplicate/conflict: exported report job IDs are unique per tenant.
- Dependency failure: partial data source failure is visible in health/ops output, not silently omitted.
- Concurrent access: long exports do not block dashboard reads.
- Authorization: platform operator may aggregate, tenant admin may not cross tenants.

## User Journey

1. Platform operator creates or imports a tenant with identity, admin access policy, and default locale/country scope.
2. Tenant admin signs in and sees only tenant-scoped products, userbase, campaigns, channels, transactions, reports, postbacks, cadence, and operations.
3. Tenant admin configures one channel/provider with capabilities and credential reference.
4. Tenant admin creates a campaign bound to tenant product and channel.
5. End subscriber reaches tenant campaign landing path or HE bootstrap path.
6. Acquisition API creates a tenant-scoped transaction, calls the correct channel/provider integration, and returns the next action.
7. TIMWE/MNO or partner callbacks update subscription, notification, charge, postback, cadence, and renewal state under the same tenant.
8. Ad partner receives tenant/provider-specific conversion postback.
9. Tenant admin views funnel, campaign, postback, subscription, notification, cadence, and ops reports filtered to their tenant.

Failure journeys:

1. Tenant admin attempts to read another tenant's campaign -> protected API returns 403/404 and logs an authorization denial.
2. Campaign binds an unsupported channel flow -> admin API returns 422 with no DB mutation.
3. Public landing URL omits tenant context while slug is ambiguous -> landing returns 404 or tenant-resolution error rather than using the wrong tenant.
4. Subscription-external receives request without tenant/channel -> rejects with service-to-service contract error rather than using global credentials.
5. Postback target fails repeatedly -> outbox attempts reach DLQ and tenant admin can requeue only own tenant rows.

## Open Questions

- Tenant source of truth: new internal tenant table, Auth0 organization, external CRM, or both?
- Tenant isolation target: shared DB with `tenant_id`, schema-per-tenant, or database-per-tenant for premium tenants?
- Secret storage: database encrypted fields, external vault, cloud secret manager, or existing environment config?
- Public tenant routing: subdomain, path segment, campaign slug namespace, signed campaign token, or gateway host mapping?
- Channel taxonomy: SMS, HE, web landing, partner API, USSD, WhatsApp, email, push, billing charge, or only current TIMWE/SMS/web first?
- Billing service status: revive disabled billing service or keep renewal/charge flows in subscription-external for first tenant platform phase?
- Admin portal source control: `frontend/webspa-admin` appeared as a nested/untracked checkout in the main tree, so integration ownership needs confirmation before implementation.
