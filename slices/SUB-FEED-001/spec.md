# SUB-FEED-001: Subscribe an explicit MSISDN feed

Status: local implementation and verification concluded; not shipped. Branch: agent/codex/subscription-processor.

Operator outcome: run subscription-processor with a file, HTTP feed or PostgreSQL
source and process only supplied MSISDNs through the existing subscription batch API.
Stdin is also supported. No generated numbers and no database schema changes.

## Contract and boundaries

Reuse internal/domain.BatchOptinRequest and the existing asynchronous POST/status
endpoints. Require tenant/channel/products and internal HMAC; sign status polling.
Validate the full finite snapshot, preserve number strings, normalize a leading
plus, deduplicate and submit bounded chunks. PostgreSQL queries must run read-only.
A dry-run reads sources but never invokes the subscription API.

Persist submission intent and accepted job IDs. Resume known jobs; do not replay
uncertain enqueue outcomes. Reject checkpoint reuse across changed source/routes.
Do not claim exactly-once delivery: the server is in-memory and lacks enqueue
idempotency. No production calls, deployment, schema writes or dependencies added.

## Architecture and prune assessment

This is a new source-driven command, a permanent product capability beside the
existing random-generation campaign command, not a replacement/compatibility fork.
The existing command is package main with intertwined random campaign scheduling,
metrics and hot reload; extracting it would expand this change into its behavior.
The new command reuses the domain request and server subscription implementation.
Source acquisition, configuration, recovery and API orchestration stay local to
cmd/subscription-processor. No second subscription service or new endpoint.
Deleting source.go/database.go removes the requested source capability; deleting
checkpoint.go removes restart safeguards. Tests verify boundaries and failure
behavior. README/config/justfile expose operator setup and build/run commands.

## Acceptance

- Text, CSV, JSON, HTTP and PostgreSQL inputs validated before POST; empty/invalid
  feeds and source failures cannot trigger random generation.
- Explicit MSISDNs and routing submitted, HMAC on POST and GET, final short batch.
- Same normalized number once per feed, counts reported, failure exit status.
- Accepted jobs resume without another POST; ambiguous POST cannot replay.
- PostgreSQL integration against a separately created disposable test database;
  read-only enforcement proven, no application database touched.
- Build and focused race tests pass; provider execution remains unverified.

## Verification commands

From services/subscription-external:

```sh
go test -race -count=1 ./cmd/subscription-processor
go vet ./cmd/subscription-processor
go build -o /tmp/timwe-subscription-processor ./cmd/subscription-processor
```

The PostgreSQL test requires TEST_SUBSCRIPTION_PROCESSOR_DATABASE_URL targeting
subscription_processor_test. Without it, database verification is explicitly skipped.
