# RFC: Canonical Tenancy Model

Status: Proposed draft only. This RFC is not approved and does not authorize backend, frontend, migration, credential, deployment, dependency, or secret changes.

Cross-runtime peer review status: unavailable in this worker run. The available tool surface exposed same-runtime subagents and GitHub PR review actions, but no repo-enabled non-Codex reviewer. Same-runtime review does not satisfy the architecture RFC peer-grill gate.

## Context

- Current state: tenant behavior is documented across `docs/admin-tenant-account-mapping.md`, `docs/tenant-channel-onboarding.md`, `docs/tenant-platform-migration-runbook.md`, and `docs/tenant-nullability-enforcement-plan.md`.
- Problem: those documents each describe a correct fragment, but downstream implementation needs one canonical model for identity, membership, tenant/channel hierarchy, resolver precedence, credentials, migration gates, and platform-vs-tenant authorization.
- Constraints: this slice is docs-only; existing runtime behavior, migrations, secrets, dependencies, and deployment configuration are out of scope.
- Non-goals:
  - Do not approve this RFC.
  - Do not add, remove, or rewrite service code.
  - Do not add migrations or mutate live data.
  - Do not persist or expose raw credential material.
  - Do not create a compatibility shim around legacy tenantless behavior.

## Requirements

- Functional:
  - Define one tenant identity model that separates internal UUID ownership from external stable keys.
  - Define the tenant/channel hierarchy and the uniqueness boundary for channel keys.
  - Define the admin membership source of truth.
  - Define platform scope separately from selected tenant workspace scope.
  - Define onboarding states and activation gates.
  - Define credential references as tenant/channel-scoped references, not raw business data.
  - Define resolver precedence for admin, gateway, public partner, and service-to-service paths.
  - Define migration and enforcement gates before downstream implementation WorkOrders proceed.
- Non-functional:
  - Fail closed on ambiguous tenant or channel context.
  - Keep tenant/channel identifiers safe for logs and metrics; never log PII, raw headers, rendered secret values, or request bodies.
  - Preserve auditability for tenant activation, admin membership changes, credential rotations, and enforcement decisions.
  - Keep rollback operationally realistic: revert code/config changes or restore database backups; do not add active rollback-to-tenantless behavior.
- Invariants:
  - No protected path may fall back to a global tenant.
  - Tenant-scoped admins may act only inside an allowed tenant workspace.
  - Platform-scoped admins must select a tenant workspace before tenant-scoped actions.
  - A channel belongs to exactly one tenant.
  - Tenant/channel production traffic requires an active channel, enabled capability, and active credential reference for the required purpose.
  - Raw provider credentials, callback secrets, and postback signing secrets are never stored in portal-visible tenant/channel configuration.
  - A request with conflicting tenant or channel sources is refused before state is read or written.
- Compatibility requirements:
  - Existing docs that use string `tenant_id` examples, such as `tenant-a`, are treated as legacy/auth/bootstrap aliases for `tenant_key`, not as proof of the internal tenant UUID.
  - Development-only shared TimWe credential fallback remains a local/sandbox exception only when the service explicitly enables it.
  - Historical nullable tenant columns are removed only through forward migrations after row-count proof and runtime enforcement proof.

## Source Reconciliation

| Existing document | Canonical status in this RFC | Superseded or constrained fragments |
| --- | --- | --- |
| `docs/admin-tenant-account-mapping.md` | Source input for Auth0 identity, bootstrap platform markers, selected tenant workspace, and repo-owned `tenant_admin_memberships`. | Any wording that treats Auth0/JWT tenant claims as the durable source of tenant membership is superseded. JWT claims may bootstrap or narrow access, but the repo-owned membership table is the tenant assignment source of truth. |
| `docs/tenant-channel-onboarding.md` | Source input for partner contract identity, tenant/channel API paths, credential reference posture, resolver precedence, callback signing, idempotency, and smoke checks. | Any successful production tenant/channel flow without both tenant and channel identity is superseded. Query-only tenant context remains allowed only after gateway trust is verified. |
| `docs/tenant-platform-migration-runbook.md` | Source input for canonical `nrg` tenant backfill, duplicate checks, dry-run/apply proof, and forward-only operational posture. | Any rollback-to-null or permanent tenantless ownership path is superseded. Database backup restore remains the rollback mechanism for live data. |
| `docs/tenant-nullability-enforcement-plan.md` | Source input for row-count proof, nullable-path classification, residual gap handling, and forward-only NOT NULL enforcement. | Any runtime nullable tenant compatibility path is constrained to an explicitly documented residual gap with proof, owner, and follow-up WorkOrder. |

## Canonical Model

### Tenant Identity

The tenant record is the durable ownership root.

| Field | Purpose | Rules |
| --- | --- | --- |
| `tenant.id` | Internal UUID primary key. | Used for database ownership, joins, foreign keys, and audit records. Never trust a request-supplied UUID without resolving and authorizing it against `tenant_key` or membership. |
| `tenant.tenant_key` | Stable external key. | Lowercase slug used in URLs, headers, gateway context, Auth0/bootstrap tenant lists, logs, metrics, and partner contracts. Unique across active tenants. |
| `tenant.display_name` | Human label. | May change without changing identity. |
| `tenant.status` | Lifecycle state. | Drives activation, suspension, and retirement gates. |
| `tenant.metadata/defaults` | Operational defaults. | Country, currency, owner, product/userbase/cadence defaults, and other non-secret configuration. |

`tenant_key` is the external identity. `tenant.id` is the internal ownership identity. The term `tenant_id` must be used carefully: in database and service-domain code it means UUID; in historical Auth0/bootstrap examples it is a legacy alias that must be normalized to `tenant_key` unless a UUID has been resolved from the tenant catalog.

Tenant key renames are not part of this RFC. If needed, they require an alias/migration RFC so old keys do not become hidden compatibility paths.

### Tenant / Channel Hierarchy

A tenant owns zero or more channels. A channel is the routing and capability boundary for provider integrations.

| Entity | Identity | Scope | Required relationships |
| --- | --- | --- | --- |
| Tenant | `tenant.id`, `tenant.tenant_key` | Platform-wide ownership root. | Owns memberships, channels, defaults, audit, reports, and tenant-owned data. |
| Channel | `channel.id`, `channel.channel_key` | Unique within one tenant. | Belongs to one tenant; carries provider/partner mapping, capabilities, callback/postback URLs, and credential references. |
| Capability | Name such as `optin`, `confirm`, `mt`, `charge`, `callback`, `postback`. | Channel-scoped. | Must be enabled before a handler or worker performs that operation. |
| Partner/provider mapping | `partner_key`, provider realm/role/operator mapping. | Channel-scoped. | Ambiguous mappings fail closed until onboarding chooses one mapping. |

External partner APIs require both `tenant_key` and `channel_key`. `channel_key` is not globally unique and must never be resolved without tenant context.

### Admin Membership Source Of Truth

Auth0 proves who the principal is. The repository owns tenant workspace access.

- Auth0 subject, email, email verification, roles, permissions, and platform markers are identity inputs.
- `tenant_admin_memberships` is the source of truth for tenant-scoped admin access.
- JWT tenant claims and Auth0 profile tenant lists can bootstrap or narrow an admin's candidate tenant list, but they do not replace active membership rows.
- A single active membership may be stamped automatically as the selected tenant workspace.
- Multiple active memberships require an explicit selected `X-Tenant-Key` that matches one active membership.
- Platform-scoped admins are identified by approved Auth0 platform roles/permissions or approved bootstrap email configuration, but they still select a tenant workspace before tenant-scoped actions.

Tenant-scoped admins cannot list or mutate other tenant catalog records. Platform-scoped operators can list/update tenant catalog records and manage memberships, subject to explicit platform authorization.

### Platform Scope Vs Tenant Scope

Platform scope is permission to administer the platform. Tenant scope is permission to act inside one tenant workspace.

Platform-scoped operations include:

- tenant catalog list/update;
- tenant profile creation or status changes;
- tenant admin membership upserts;
- platform-wide secret/configuration administration;
- explicitly authorized all-tenant reporting or operational views.

Tenant-scoped operations include:

- tenant dashboard and reports;
- tenant channel creation and configuration;
- campaign, product, userbase, cadence, callback, notification, postback, and credential-reference operations for one selected tenant workspace.

Rules:

- Tenant-scoped routes require resolved tenant scope before data access.
- `all_tenants=true` is platform-only and must be denied to tenant-scoped identities.
- Platform users do not get implicit cross-tenant writes on tenant-scoped routes; they must select the target tenant.
- Unknown or unauthorized tenant keys return a denial, not a global fallback.

### Onboarding Lifecycle

The proposed lifecycle is:

1. `draft`: tenant intent exists, but no production traffic may route to it.
2. `profile_created`: stable `tenant_key`, display name, owner, status, and country/currency defaults are recorded.
3. `membership_ready`: at least one authorized admin membership is active, or an approved platform operator is responsible for completion.
4. `channel_configured`: at least one channel has a stable `channel_key`, provider/partner mapping, callback/postback URLs, and enabled capabilities.
5. `credentials_bound`: every required provider/callback/postback purpose has an active credential reference.
6. `defaults_ready`: product, userbase, renewal cadence, billing cadence, notification, and postback defaults are configured for the tenant/channel.
7. `smoke_passed`: tenant/channel resolution, capability refusal, signed callback handling, postback delivery, cross-tenant rejection, and credential-reference resolution have recorded evidence.
8. `active`: production traffic may route to the tenant/channel.
9. `suspended`: traffic is blocked or limited without deleting historical ownership.
10. `retired`: new traffic is disabled and cleanup/retention policy owns the remaining data.

Activation requires smoke evidence and credential rotation if any credential-like value was exposed during setup or agent work.

### Credential Reference Boundaries

Credential references are tenant/channel-scoped configuration records that point to secrets managed elsewhere.

Required properties:

- `tenant_id` and `channel_id`;
- credential purpose, such as `provider_api`, `callback_hmac`, or `postback_signing`;
- provider or integration name;
- secret reference URI or key;
- active version/status;
- redacted display metadata only;
- rotation owner, cadence, and last rotation evidence.

Boundaries:

- Raw `TIMWE_API_KEY`, `TIMWE_PSK`, callback shared secrets, postback signing secrets, and equivalent provider auth material must not be stored in docs, portal-visible config, business tables, examples, tickets, or screenshots.
- Platform-wide shared secrets, such as JWT validation, database credentials, Redis credentials, Auth0 validation settings, and monitoring credentials, are not tenant/channel credentials.
- Deployment credentials, object storage access keys, and platform tokens are deployment operations material, not tenant identity.
- Local development fallbacks may exist only as explicit development/sandbox settings and must not activate production tenant/channel traffic.

### Resolver Precedence

All services should use the canonical resolver path for tenant/channel identity rather than inline header, query, or JWT parsing.

Admin route precedence:

1. Verify the authenticated principal.
2. Derive platform scope from approved Auth0 roles/permissions or approved bootstrap platform email configuration.
3. Resolve active tenant memberships from `tenant_admin_memberships`.
4. Intersect JWT/Auth0 tenant claims with active memberships when claims are present.
5. Apply selected workspace headers only after the principal is authorized:
   - platform-scoped user: selected tenant must exist in the tenant catalog;
   - tenant-scoped user: selected tenant must match an active membership;
   - no selected tenant: use the single active membership when exactly one exists, otherwise deny tenant-scoped actions.
6. If both `X-Tenant-Key` and `X-Tenant-Id` are present, resolve and cross-check them. A mismatch is a denial.

External tenant/channel route precedence:

1. Trusted path captures on `/api/external/v1/{tenant_key}/{channel_key}/...` resolve to `tenant_key` and `channel_key`.
2. Header context accepts `X-Tenant-Key` and `X-Channel-Key` when present.
3. Header and query values must agree after lowercase normalization.
4. If header and query conflict, refuse with HTTP 409 before handler state changes.
5. Query-only tenant/channel context is accepted only when gateway trust is verified through signed service context.
6. A request that supplies `tenant_key` without required `channel_key` for channel-owned operations is denied.
7. Unknown tenant, unknown channel, disabled channel, or missing capability is denied.

Service-to-service precedence:

1. Verify the signed service context.
2. Resolve tenant/channel from signed context headers or gateway-forwarded query values.
3. Require operation-specific capability and credential reference.
4. Refuse missing, expired, replayed, mismatched, or unsigned context without provider calls or writes.

Error posture:

- Tenant key conflict: HTTP 409.
- Missing tenant/channel for a required operation: HTTP 400 or domain-specific tenant-context denial.
- Unknown tenant/channel: deny; do not store tenantless rows.
- Tenant-scoped unauthorized access: HTTP 403.

## Options Considered

### Option A: External `tenant_key` / `channel_key`, Internal UUID Ownership

- Summary: Use stable external keys for routes, headers, gateway context, and Auth0/bootstrap lists; resolve to UUIDs for database ownership and joins.
- Pros:
  - Matches existing partner contract and admin workspace behavior.
  - Keeps database identity immutable.
  - Makes logs/metrics safe and readable.
  - Keeps channel keys tenant-scoped.
- Cons:
  - Requires careful normalization where older docs use string `tenant_id`.
  - Requires cross-checking when both key and UUID are supplied.
- Risks:
  - Hidden compatibility code may continue treating `tenant_id` strings as authoritative.
  - Key rename pressure could create aliases without an explicit migration plan.
- Cost: moderate documentation and implementation cleanup, low data-model disruption.

### Option B: Auth0/JWT Tenant Claims As Source Of Truth

- Summary: Treat Auth0 profile/token tenant claims as the durable tenant assignment model.
- Pros:
  - Simpler first-pass admin UI wiring.
  - Less backend membership logic.
- Cons:
  - Tenant access changes become external identity-provider state rather than repo-audited business state.
  - Harder to audit membership changes alongside tenant/channel activation.
  - Does not match existing `tenant_admin_memberships` direction.
- Risks:
  - Stale tokens or profile values can grant or deny tenant access incorrectly.
  - Platform-vs-tenant scope becomes harder to enforce consistently.
- Cost: low short-term cost, high operational and authorization risk.

### Option C: Canonical `nrg` Default Tenant For All Legacy Paths

- Summary: Keep tenantless or ambiguous runtime paths by assigning them to `nrg` or falling back to `nrg`.
- Pros:
  - Eases migration of existing tenantless data.
  - Preserves legacy behavior during rollout.
- Cons:
  - Makes tenant isolation depend on implicit defaults.
  - Conflicts with the invariant that protected paths do not fall back to a global tenant.
  - Hides missing tenant/channel identity in provider flows.
- Risks:
  - Cross-tenant data exposure or incorrect billing/notification routing.
  - Permanent nullable compatibility paths survive after migration.
- Cost: low immediate cost, high long-term safety cost.

## Decision

- Chosen option: Option A, pending independent review and operator approval.
- Why:
  - It aligns the partner API, admin workspace, migration runbook, and nullability enforcement plan around one model.
  - It separates external identity from internal ownership.
  - It gives downstream WorkOrders a clear fail-closed contract.
  - It preserves development/sandbox fallbacks without allowing production global credential fallback.
- Why not the alternatives:
  - Auth0/JWT-only tenant assignment does not provide repo-owned membership auditability.
  - A canonical default tenant is useful for migration backfill, but unsafe as an active runtime fallback.

This decision is proposed only. Approval requires cross-runtime peer review or an explicit operator decision.

## Implementation Plan

1. Route this RFC through independent peer review and operator approval.
2. After approval, update existing docs so this RFC becomes the canonical reference and the fragment docs become implementation-specific runbooks.
3. Update downstream WorkOrders to consume the canonical terms:
   - `tenant_key` for external identity;
   - UUID `tenant.id` / `tenant_id` for database ownership;
   - `tenant_admin_memberships` for tenant-scoped admin access;
   - selected workspace headers only after authorization;
   - tenant/channel credential references only.
4. Dispatch implementation slices in dependency order with focused tests and cross-tenant smoke evidence.

## Migration And Enforcement Gates

Downstream implementation must not treat this draft as approval. After approval, each implementation WorkOrder must satisfy the gates relevant to its scope:

- RFC approval gate: link to the approved RFC version and peer-review or operator-approval evidence.
- Data proof gate: show row counts and duplicate/conflict checks for the table group being migrated or constrained.
- Runtime proof gate: prove the affected handlers, repositories, workers, and reports resolve tenant/channel context before data access.
- Nullable cleanup gate: add only forward migrations after the target table group has zero tenantless live rows or an approved table-specific residual plan.
- Resolver gate: prove header/query/path/JWT/service-context conflicts fail closed and unknown tenant/channel values do not fall back to global behavior.
- Credential gate: prove raw provider, callback, and postback secrets are neither persisted nor returned; only active credential references are resolved.
- Capability gate: prove channel operations deny missing or disabled capabilities before provider calls or writes.
- Cross-tenant smoke gate: prove at least one same-tenant success and one cross-tenant or mismatched-context refusal for each public/admin/runtime path being changed.
- Control-plane gate: run the WorkOrder's HVC/schema/reconcile checks and record evidence before archive.

## Downstream WorkOrders Consuming This Decision

| WorkOrder / issue | Consumption requirement |
| --- | --- |
| `TMP-048` admin tenant account mapping | Keep Auth0 as identity proof and `tenant_admin_memberships` as tenant assignment source of truth. Normalize legacy string `tenant_id` examples as `tenant_key`. |
| `TMP-051` tenant catalog admin UI/API | Keep tenant catalog list/update platform-scoped. Tenant-scoped admins must not list or mutate other tenants. |
| `TMP-055` tenant nullable runtime enforcement | Collapse active nullable runtime paths into tenant-aware canonical ownership after proof; no global fallback as canonical behavior. |
| `TMP-057` report tenant-key scope resolution | Resolve `X-Tenant-Key` through the tenant catalog to UUID ownership; deny unknown keys and tenant-user all-tenant aggregation. |
| `TMP-065` tenant workspace interceptor readiness | Ensure initial tenant-scoped frontend requests wait for selected workspace readiness and attach authorized tenant key. |
| `TMP-071` webspa-admin operator UI refresh | Preserve tenant catalog/workspace behavior while improving UI; do not change tenancy semantics. |
| `TMP-074` tenant nullable residual cleanup | Resolve residual tenantless rows and public slug-only compatibility through proof and forward-only cleanup. |

## Rollout And Rollback

- Rollout:
  - Treat this draft as a review artifact until approved.
  - After approval, reference it from tenant implementation WorkOrders before code changes.
  - Require each implementation WorkOrder to name which canonical rule it consumes.
- Rollback:
  - Before approval: remove or revise this document.
  - After implementation: use normal code/config rollback or database backup restore; do not add active rollback-to-tenantless behavior.
- Migration impact:
  - `nrg` backfill remains a migration tool for assigning existing tenantless rows.
  - Runtime paths still need proof before NOT NULL constraints or nullable compatibility removal.
  - Existing migrations are not rewritten; cleanup uses forward migrations.
- Observability:
  - Logs and metrics may include bounded tenant/channel keys and operation status.
  - Logs and metrics must not include raw MSISDN, rendered secrets, request bodies, raw headers, or credential values.

## Verification

- Tests:
  - Downstream WorkOrders must run their focused backend/frontend tests.
  - Tenant/channel resolver changes must prove conflict, missing context, unknown tenant/channel, disabled capability, and cross-tenant refusal.
  - Credential changes must prove raw values are not persisted or returned.
- Metrics:
  - Tenant/channel labels are low-cardinality keys.
  - Unknown/denied tenant-context events are observable without PII or secrets.
- Manual checks:
  - Confirm each downstream WorkOrder links to this RFC after approval.
  - Confirm activation evidence exists before a tenant/channel becomes active.
  - Confirm credential-like values exposed during onboarding are rotated before production activation.
- Failure signals:
  - Successful tenant-scoped operation without tenant context.
  - Header/query tenant conflict accepted.
  - Tenant-scoped admin listing all tenant catalog rows.
  - Provider call using shared TimWe credentials in production tenant/channel traffic.
  - Runtime query preserving `tenant_id IS NULL` as a canonical path after enforcement proof.
- Premortem decision: `WARN` - no separate premortem packet was run in this docs-only draft. Treat peer review as blocked until a non-Codex reviewer or explicit operator approval is available.

## Open Questions

- Should tenant lifecycle states be persisted exactly as proposed here, or mapped onto existing tenant/channel status enums?
- Is tenant key rename support needed, or should tenant keys be immutable forever?
- Which secret-reference URI scheme is canonical for provider, callback, and postback credentials?
- Which platform roles/permissions are the final production markers for platform scope?
- How long should legacy no-tenant notification callbacks remain accepted, and what evidence closes that compatibility path?
- Which runtime owns the required cross-runtime peer review before this RFC can become an approved decision?
