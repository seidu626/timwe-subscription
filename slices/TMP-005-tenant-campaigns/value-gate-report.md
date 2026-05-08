# TMP-005 Value Gate Report

Verdict: PASS

## Slice

Tenant admin campaigns are created, read, listed, updated, and enabled under tenant scope and bound to compatible tenant channels.

## Review Gates Applied

- Domain grounding applied: actors, language, invariants, and TMP-012 deferrals are captured in `domain-brief.md`.
- Story craft applied: acceptance criteria now use current channel capability vocabulary instead of an unsupported `REDIRECT` channel capability.
- Parallel critique applied: public route ambiguity, global slug access, tenant product mismatch, channel compatibility, seed SQL conflict targets, and schema bootstrap were addressed.
- Value gate applied against acceptance, failure modes, and invariant coverage after implementation.

## Acceptance Coverage

- Create compatible campaign: `AdminCreateForTenant` validates tenant product and channel compatibility, persists `tenant_id` and `channel_id`, and returns the created campaign.
- Public campaign resolve: router dispatches `/v1/campaigns/{tenant_key}/{slug}` to tenant-key lookup for enabled campaigns under active tenants.
- Duplicate slug inside tenant: partial unique index `idx_campaigns_tenant_slug` plus service conflict mapping returns `campaign_conflict`/409.
- Channel capability mismatch: OTP requires `optin` and `confirm`; mismatch maps to `ErrCampaignChannelCapabilityMismatch` and handler 422.
- Disabled public access: public tenant lookup filters `enabled = true`.
- Same slug in two tenants: global slug uniqueness is replaced with tenant-scoped uniqueness while legacy unscoped slugs remain protected.

## Verification

Commands run:

```bash
cd services/acquisition-api && go test ./internal/service ./internal/handler ./internal/repository ./internal/transport
cd services/acquisition-api && go test ./...
```

Results:

- Focused acquisition-api service, handler, repository, and transport tests passed.
- Full acquisition-api test suite passed.

## Test Quality

The repository does not contain `scripts/scan-test-quality.sh`; manual checks were applied. Tests assert concrete behavior for tenant/channel persistence, tenant product mismatch, channel capability mismatch, duplicate conflict mapping, tenant public route parsing, migration scoped uniqueness, and admin tenant context rejection.

## Gaps Deferred

- Host/header/gateway public tenant resolution and legacy public slug disambiguation are deferred to TMP-012.
- Campaign clone and postback-rule admin subroutes remain legacy/global in this slice and should be scoped before broad tenant rollout if those routes are retained.
