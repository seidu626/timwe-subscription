# Subscription processor: invalid records, subscription-only execution and progress

Scope: record the two verified INVALID_MSISDN responses, make new processor jobs
persist invalid results synchronously, suppress application SMS/renewal side effects,
and process only source rows 51-1050 with live CLI progress. No schema changes.

Contract: BatchOptinRequest.subscription_only (optional boolean, default false).
True requires explicit MSISDNs and a non-SMS entry channel. GET batch?capabilities=1
is authenticated and advertises subscription_only and invalid_msisdn_logging.
The CLI requires these capabilities before enqueue and always requests this mode.
Older services fail the preflight before any provider call. Existing callers retain
their behavior when the flag is absent.

Server behavior in this mode: retain canonical tenant/channel context for persistence;
submit opt-in only; no SMS fallback, renewal, notification or charging follow-up;
write provider INVALID_MSISDN evidence synchronously and propagate storage errors;
skip asynchronous destructive invalid-number cleanup. Provider-managed notifications
are outside the application's opt-in contract and are not claimed suppressed.

CLI polls report processed/total, successful and failed counts. Existing durable
checkpoint and no-replay rules remain. New mode is included in checkpoint identity.

Validation: contract tests for capability negotiation and flag propagation, HTTP
provider fixture tests for no fallback/follow-up and invalid evidence, disposable
PostgreSQL verification where repository persistence changes, race tests and builds.
Deploy only subscription-external if required for the negotiated contract. Verify
health and capabilities before the authorized 1000-number batch. Preserve root's
uncommitted config edit. No additional source rows or repeat batches.

Architecture: extend the existing request/service path; do not create a second
subscription implementation or embed database credentials in the CLI. Repository
logging remains the canonical writer to invalid_msisdn_logs. New symbols/tests are
limited to enforcement at those boundaries and observable progress.
