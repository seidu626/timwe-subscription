# PostgreSQL reliability repair evidence

The repair is based on deployed source `4e8eb57`. It changes subscription-external and the subscription-processor CLI. No new subscription batch, provider replay, production migration, or historical data backfill is part of verification.

## Changes

- Apply finite PostgreSQL pool settings before the startup ping. Deployment limits are five open connections, two idle connections, a five-minute connection lifetime, and a ten-second connection timeout. Batch concurrency is five workers per job and five simultaneous opt-ins across jobs.
- Use the live tenant-scoped userbase conflict key `(tenant_id, msisdn)` and constrain blacklist eligibility and subscription cleanup by tenant. Ignore stale negative exclusion cache entries and stop writing them.
- Make one provider attempt for subscription-only processing. Retry transient local persistence at most three times after confirmed acceptance, with bounded database calls. Preserve ambiguous provider outcomes for reconciliation.
- Preserve each failed item's source index, job-specific identity hash, provider attempt ID, provider transaction ID, original local tracking ID, acceptance time, and persistence result where available. The CLI saves private atomic receipts before checkpoint advancement and validates item hashes against the original input.
- Emit plain JSON log levels and safe subscription-only diagnostics. Retire the unsafe legacy upload endpoint with HTTP 410 directing clients to the authenticated admin import.
- Preserve application no-SMS behavior and existing invalid-MSISDN persistence for subscription-only jobs.

## Verification record

The disposable PostgreSQL 16 test database is bound to localhost and separate from production. Tests exercise pool contention, tenant conflict/upsert behavior, tenant-constrained deletion, stale negative cache recovery, provider call counts, failure receipts, cancellation, and no-SMS behavior.

Final gate results and deployment receipts are recorded in the private task artifact directory:
`/home/xper626/workspace/tmp/timwe-pg-reliability-20260919`.

- Full subscription-external `go test -race ./...`: passed with all three test database variables pointing to the disposable database.
- Full subscription-external `go vet ./...`: passed.
- Common configuration and logging `go test -race` and `go vet`: passed.
- Independent database review: passed after the stale-negative-cache repair.
- Independent execution and CLI review: passed after the provider, cancellation, recovery-correlation and terminal-status repairs.
- CLI receipt tests cover 5,000 detailed failures, failed atomic writes, hash mismatch, inconsistent terminal totals, and cancelled partial results without replay or checkpoint advancement.

## Historical recovery boundary

The last 5,000-row job has 25 TIMWE-accepted subscriptions without local rows and 26 blacklist writes that failed. None has an exact retained identity join: every masked number matches multiple records in the bounded source snapshot. Read-only live correlation searches found no alternate identity record. The safe historical write count is zero. Recovery requires exact provider/request audit evidence; provider requests must not be replayed.

## Separate existing scope

The random-number generator, backfill filters, and non-subscription-only legacy eligibility APIs still use global exclusion contracts. Converting those APIs requires threading verified tenant identity through their callers. They are separate from the explicit-source subscription-only path repaired here. Other applications also share the PostgreSQL cluster; this service's finite pool cannot guarantee their capacity use.

Server job state remains in memory; a service restart loses pollable jobs. The CLI's 16 MiB response ceiling is covered for 5,000 detailed failures; substantially larger batches need an explicit size limit or paginated receipts before general queue use. These limits are retained explicitly rather than treating the service as a durable queue.
