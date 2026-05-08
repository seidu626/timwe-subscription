# Slice TMP-007: Channel Subscription Routing

## User story

As an api-integrated-partner, I can call subscription and charge APIs with trusted tenant/channel context so that subscription-external uses the correct tenant provider configuration instead of global TIMWE credentials.

## Demo script

1. Create or seed an active tenant with an active channel that supports `optin`, `confirm`, `mt`, and `charge`, plus an active `provider_api` credential reference.
2. Start subscription-external with the credential resolver configured for local secret references and trusted service header validation enabled.
3. Send a signed `POST /api/external/v1/WEB/mt` request for that tenant/channel and observe the fake TIMWE server receives the tenant provider URL, partner role, API key, authentication header, and payload.
4. Send the same request without signed tenant context and observe a 400/403 response with no fake TIMWE request.
5. Send a charge request through a channel without `charge` capability and observe `422 unsupported_channel_operation`.
6. Start an acquisition transaction for a tenant campaign and confirm that the subscription-external opt-in call uses the campaign channel context.

## Acceptance criteria

- [ ] When a partner sends MT/optin with signed tenant context and an active channel credential, subscription-external resolves `(tenant_id, channel_id, purpose='provider_api')`, uses the tenant provider config for the outbound request, and persists tenant/channel ownership on resulting subscription state.
- [ ] When acquisition confirms a tenant campaign transaction, the service-to-service opt-in request carries the transaction tenant/channel context into subscription-external and no global TIMWE credential is used.
- [ ] When tenant/channel context is missing, unsigned, expired, replayed, or mismatched, the API returns 400/403 and no provider request occurs.
- [ ] When the requested operation is not allowed by the tenant channel capabilities, the API returns `422 unsupported_channel_operation` and no provider request occurs.
- [ ] When the active credential reference is missing or cannot be resolved, the API returns a dependency/configuration error, logs only tenant/channel identifiers and redacted credential metadata, and makes no provider call.
- [ ] When two tenants use the same MSISDN/product, subscription existence checks and upserts remain tenant-scoped and do not collide.
- [ ] When an upstream TIMWE timeout or retryable failure occurs, existing retry/circuit-breaker behavior is preserved without duplicate tenant subscription rows.
- [ ] When `OPTIN_CONFIG_NOT_FOUND` would trigger the current hard-coded SMS retry, the retry only proceeds through an explicitly resolved tenant channel with the required capability.

## Layers touched

- **Schema / migrations**: `services/subscription-external/migrations/016_tenant_channel_subscription_routing.sql` adds nullable `tenant_id` and `channel_id` to `subscriptions`, `notifications`, and admin action logs; adds tenant-scoped indexes/unique constraints while preserving legacy rows.
- **Storage / RLS**: N/A - repository-level tenant filters are used; no Postgres RLS framework exists in this repo.
- **API / handlers**: `services/subscription-external/internal/handler/partner_handler.go`, `services/subscription-external/internal/handler/admin_subscription_handler.go`, `services/subscription-external/internal/transport/router.go`.
- **Business logic**: `services/subscription-external/internal/service/subscription.go`, `services/subscription-external/internal/service/admin_actions.go`, plus a tenant channel resolver module under `services/subscription-external/internal/service`.
- **UI components**: N/A - no admin portal UI change in this slice; TMP-014 owns portal workspace behavior.
- **Types / contracts**: `services/subscription-external/internal/domain/subscription.go`, `services/subscription-external/internal/domain/admin_action.go`, `common/auth/tenantctx`, and `services/acquisition-api/internal/service/timwe_client.go`.
- **Tests (unit + e2e)**: subscription-external handler/service/repository tests for tenant context, capability gating, credential failures, tenant-scoped subscription identity, and fake-provider outbound config; acquisition-api client/service tests for tenant/channel propagation.
- **Config / env vars**: local credential resolver config for `env://` style credential references and trusted service header secret reuse; no raw tenant secrets in YAML or compose files.
- **Observability**: structured logs include safe `tenant_id`, `channel_id`, `provider`, and `operation`; logs and admin action details exclude API keys, auth headers, PSKs, token values, and raw secret references.
- **Docs**: `slices/TMP-007-channel-subscription-routing/domain-brief.md`, this `spec.md`, and `slices/manifest.json`.

## Design Decisions

> Stamped by improve-codebase-architecture during /slice-spec.

- **Module shape:** Add one tenant provider routing module in subscription-external that resolves channel, capability, credential, and TIMWE request config before existing send methods build outbound requests.
- **Depth vs leverage:** The deep module hides duplicated provider URL/header/auth selection behind a small routing interface, so MT, charge, status, optout, confirm, and admin actions share the same tenant-routing rules.
- **Locality:** Routing decisions belong beside subscription-external's outbound provider calls; acquisition-api only passes trusted tenant/channel context and does not learn TIMWE credential details.
- **Adapter boundary:** Credential dereferencing is an adapter behind the routing module; `env://` can support local tests while production vault behavior remains a TMP-015 concern.
- **Notes:** Do not broaden this slice into new non-TIMWE providers, full vault integration, callback correlation, or notification/cadence tenant routing.

## Out of scope

- New non-TIMWE provider implementation.
- Production vault vendor integration or secret rotation runbooks beyond the resolver interface and local/env-backed test adapter.
- Notification worker and cadence tenant routing; TMP-008 owns those flows.
- Inbound TIMWE/MNO/partner callback correlation; TMP-013 owns callback tenant/channel matching.
- Billing service ownership decisions; TMP-017 owns split billing/charge ownership.
- Admin portal screens for tenant channel routing; TMP-014 owns UI workspace behavior.
- Migrating all legacy global subscription rows into a default tenant; TMP-011 owns legacy migration isolation.

## Risks and mitigations

- **Risk**: The current service builds TIMWE URLs and auth headers in several methods, making partial tenantization easy. **Mitigation**: introduce a shared routing/config module and require tests for every in-scope operation.
- **Risk**: Free-form path or body channel strings could be mistaken for tenant channel identity. **Mitigation**: resolve tenant channel by signed tenant context plus channel id/key, and treat legacy entry-channel strings as provider payload fields only.
- **Risk**: The existing hard-coded SMS retry can bypass declared tenant channel capabilities. **Mitigation**: route retry attempts through the same channel resolver or fail closed.
- **Risk**: Credential references can be logged through admin action request/response capture. **Mitigation**: redact auth headers and secret references before logs/audit serialization, with tests asserting absence.
- **Risk**: Adding tenant columns to live subscription tables could break legacy records. **Mitigation**: keep tenant/channel columns nullable for legacy rows and add tenant-scoped indexes without dropping legacy compatibility in this slice.

## Feature flag?

No - tenant-scoped routing is selected by trusted tenant/channel context while legacy unscoped compatibility remains on legacy calls until TMP-011 migration isolation.

## Definition of Done (slice-specific)

- [ ] subscription-external full Go test suite passes.
- [ ] acquisition-api tests covering subscription-external client propagation pass.
- [ ] Tests prove no outbound provider call occurs for missing tenant context, unsupported capability, credential lookup failure, and forged trusted headers.
- [ ] Tests prove tenant-scoped subscription identity permits the same MSISDN/product in two tenants.
- [ ] Tests or log capture prove provider credentials and secret references are redacted.
- [ ] `git diff --check` passes.

## Estimated layers of work

Roughly 14-15 files, one subscription-external migration, one tenant provider routing module, handler/domain changes for context propagation, acquisition client propagation, and focused tests. If implementation needs more than one migration plus broad callback/notification changes, split the excess into TMP-013 or TMP-008 rather than expanding this slice.
