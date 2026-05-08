# Slice TMP-009: Postback and Attribution Routing

## User Story

As an ad-or-traffic-partner, I can receive configured conversion postbacks generated from tenant campaign attribution and charge success so that campaign billing and optimization are attributed to the right provider.

## Acceptance Criteria

- When charge success is received for a tenant transaction with provider attribution, click id, and conversion template, acquisition-api creates a `postback_outbox` row with `tenant_id`, `channel_id`, provider, rendered URL, and `PENDING` status.
- When a tenant admin lists postbacks by transaction/status or retries a DLQ row, repository queries and updates are constrained by that tenant id.
- When another tenant attempts to read or retry a DLQ row, the API returns 404 or 403 and does not mutate the row.
- When a template requires `click_id` or `txid` and attribution lacks click identity, no deliverable URL is queued; a failed outbox row records the reason.
- When no conversion template or provider fallback exists, the system records skipped/no-template state for operator visibility without panic.
- When charge success is replayed after conversion postback has already been marked sent, no duplicate active postback is emitted.
- When dispatcher retries exhaust, existing attempt history and DLQ transition behavior are preserved.
- Default postback rendering does not put raw MSISDN in outbound URLs or fallback payloads; use `msisdn_hash` unless a future explicit approved template allows raw identifiers.

## Layers Touched

- Schema / migrations: add tenant/channel ownership and failure reason to `postback_outbox`.
- Domain: add ownership and failure metadata to `PostbackOutbox`.
- Repository: create, scan, query, stats, and retry by tenant; preserve worker global claim path.
- Service: stamp tenant/channel on outbox rows, fail-record missing click/no-template states, preserve idempotence.
- API / handlers: tenant-scope admin postback diagnostics and retry endpoints.
- Tests: template rendering failure, PII guard, tenant-scoped repository queries, admin cross-tenant retry, charge-success enqueue behavior.

## Out of Scope

- New provider marketplace or partner onboarding docs.
- Billing reconciliation UI.
- Dispatcher per-tenant scheduling or quotas.
- Production dashboard changes; TMP-010/TMP-015 own broader reporting and observability.

## Definition of Done

- `services/acquisition-api` Go tests pass.
- `git diff --check` passes.
- Value gate report exists and passes.
- Tests prove tenant-owned outbox creation, tenant-scoped retry/list behavior, missing click failure recording, duplicate charge-success idempotence, and PII-safe rendering.
