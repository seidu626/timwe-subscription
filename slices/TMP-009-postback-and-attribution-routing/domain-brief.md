# TMP-009 Domain Brief: Postback and Attribution Routing

## Actors

- ad-or-traffic-partner: receives conversion postbacks tied to the click identifier supplied into a tenant campaign (source: `slices/TMP-009-postback-and-attribution-routing/slice.yaml`, `services/acquisition-api/internal/domain/postback.go`).
- tenant admin: inspects and retries postback DLQ entries only for their tenant (source: `services/acquisition-api/internal/handler/postback_admin_handler.go`, `common/auth/tenantctx`).
- acquisition-api: owns campaign attribution, transaction state, postback template rendering, outbox creation, and dispatcher handoff (source: `services/acquisition-api/internal/service/transaction_service.go`, `services/acquisition-api/internal/repository/postback_repository.go`).
- postback dispatcher: claims pending outbox records, attempts HTTP delivery, and moves exhausted records to DLQ (source: `services/acquisition-api/internal/worker/postback_dispatcher.go`).
- subscription-external: calls the internal charge-success endpoint after successful charging (source: `services/acquisition-api/internal/handler/internal_handler.go`).

## Ubiquitous Language

- Postback outbox: persisted server-to-server delivery request with event, provider, rendered URL, retry status, and attempt state (source: `services/acquisition-api/internal/domain/postback.go`).
- Postback attempt: immutable delivery-attempt record attached to one outbox row (source: `services/acquisition-api/migrations/create_postback_tables.sql`).
- Conversion event: postback event emitted only after charge success, not after opt-in/subscription confirmation (source: `services/acquisition-api/internal/service/transaction_service.go`).
- Attribution: normalized provider/click/sub fields captured on acquisition transactions and used to render provider templates (source: `services/acquisition-api/internal/domain/transaction.go`, `services/acquisition-api/internal/service/ad_provider.go`).
- Tenant admin context: authenticated tenant identity stored on the FastHTTP request by admin middleware (source: `services/acquisition-api/internal/transport/admin.go`, `services/acquisition-api/internal/handler/admin_management_handler.go`).
- DLQ: terminal status for postbacks that exhausted dispatcher retry attempts and require operator action (source: `services/acquisition-api/internal/worker/postback_dispatcher.go`).

## Domain Invariants

- Postbacks generated for tenant transactions carry tenant ownership and, when available, campaign channel ownership (source: `slices/TMP-009-postback-and-attribution-routing/slice.yaml`, `services/acquisition-api/internal/domain/transaction.go`).
- Tenant admin postback reads/retries are tenant-scoped and must not reveal another tenant's DLQ rows (source: `services/acquisition-api/internal/handler/postback_admin_handler.go`).
- A template requiring click identity must not render and deliver an empty or malformed click parameter (source: `services/acquisition-api/internal/service/ad_provider.go`).
- Conversion postbacks are idempotent and recoverable: duplicate charge success does not create duplicate active deliveries, and failed/DLQ rows can be retried intentionally (source: `services/acquisition-api/internal/service/transaction_service.go`, `services/acquisition-api/internal/repository/postback_repository.go`).
- Default external postback context must not expose raw MSISDN; privacy-safe templates use `msisdn_hash` (source: `services/acquisition-api/internal/domain/postback.go`).

## Failure Modes

- Charge success:
  - Invalid input: missing `timwe_transaction_id` returns 400 at the internal handler.
  - Missing required attribution: a template that needs `click_id` records a failed postback row and does not enqueue delivery.
  - Duplicate/conflict: a transaction already marked `conversion_postback_sent` is ignored without another active outbox row.
  - Dependency failure: outbox insert failure is logged and does not panic the charge-success endpoint.
  - Authorization: internal endpoint requires HMAC headers before request body processing.
- Tenant admin postback diagnostics:
  - Invalid input: malformed transaction or postback UUID returns 400.
  - Missing required tenant context: admin request without accepted tenant identity is rejected.
  - Cross-tenant access: tenant mismatch returns not found/forbidden without row details.
  - Dependency failure: repository query/update failure returns an operator-visible server error.
- Dispatcher:
  - Dependency failure: HTTP timeout/non-2xx records an attempt and reschedules or moves to DLQ.
  - Concurrent access: dispatcher claims rows with `FOR UPDATE SKIP LOCKED`.
  - Duplicate/conflict: retry resets only failed/DLQ rows and preserves attempt history.

## User Journey

1. End subscriber completes a chargeable tenant campaign transaction.
2. subscription-external calls `POST /internal/acquisition/charge-success`.
3. acquisition-api finds the transaction, resolves campaign/postback rules, renders a provider-specific conversion URL from attribution, and writes a tenant/channel-owned `PENDING` outbox row.
4. Dispatcher sends the postback and records attempts; exhausted failures become `DLQ`.
5. Tenant admin lists or retries only their tenant's DLQ rows through admin postback endpoints.

Failure journeys:

1. Transaction lacks a click id required by the provider template -> acquisition-api records a failed/no-delivery outbox row and does not queue an invalid URL.
2. Campaign lacks provider postback template and no provider fallback exists -> skipped/no-template state is recorded for operator visibility.
3. Tenant admin retries another tenant's DLQ row -> 404/403 and no state change.

## Open Questions

- The current admin postback endpoints are diagnostics-oriented and do not expose a general `GET /v1/admin/postbacks` list without `transaction_id`; this slice scopes existing diagnostics/status/retry endpoints rather than building a new UI query surface.
- The existing dispatcher does not filter by tenant because workers should process all tenants; tenant isolation is enforced at outbox ownership and admin access boundaries.
