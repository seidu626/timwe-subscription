# Subscription processor

`subscription-processor` subscribes MSISDNs supplied by a file, HTTP feed, stdin,
or PostgreSQL query. It uses the same asynchronous batch API as `batch-processor`:
`POST /api/v1/subscription-external/batch`, followed by signed status polling.
Every request contains a nonempty `msisdns` list and its actual count. It never
requests random number generation.

Every batch uses `subscription_only: true`. The server must advertise this
capability before the processor submits. This path performs the TIMWE opt-in and
persists the subscription with its resolved tenant/channel. It does not trigger
application SMS, SMS fallback, renewal, or charging follow-ups. SMS entry mode is
rejected. Provider-managed messages are outside this application control.

TIMWE `INVALID_MSISDN` responses are saved synchronously in `invalid_msisdn_logs`;
a persistence failure fails the item. Numbers already in that table are skipped
with a failed item result before contacting TIMWE. Only `OPTIN_ALREADY_ACTIVE` and
`OPTIN_ACTIVE_WAIT_CHARGING` results are persisted as successful subscriptions; pending
confirmation or unexpected results require reconciliation. Successful processing
does not imply a completed charge.

While polling, the binary prints state, processed/total, successful and failed
counts whenever progress changes. Checkpoints from the earlier processor mode
are intentionally incompatible; retain them as history and use a fresh checkpoint
for a new, nonoverlapping feed.

## Configure and run

Run these commands from this directory:

```sh
just build
./subscription-processor -config config.json -source /path/to/msisdns.csv -dry-run
./subscription-processor -config config.json -source /path/to/msisdns.csv
```

First edit `config.json`: set `base_url`, `tenant_key`, `channel_key`, `telco`,
`entry_channel`, and `product_ids` for the intended subscription route. The sample
leaves tenant, channel and products empty so it cannot submit accidentally. Paths
are relative to the process working directory. Set `INTERNAL_API_SECRET` through
your existing environment/secret manager for a real run; it signs both POST and
GET requests. No secret belongs in the JSON configuration.

`-dry-run` reads and validates the source and reports unique numbers, duplicates,
and planned batches. It does not call the subscription API or write progress.
It still contacts an HTTP/database source and requires that source's credentials.
There is no subscription side effect until the entire source has been read and
validated. Use a feed of numbers authorized for the configured subscriptions;
format validation does not establish eligibility or consent.

## Sources

| Source | Configuration or flag | Input |
| --- | --- | --- |
| File | `"source": "msisdns.csv"` or `-source /path/to/feed` | Text, CSV or JSON |
| HTTP feed | `"source": "https://feed.example/msisdns"` | One HTTP 200 response with text, CSV or JSON |
| Database | `"source": "database"` | PostgreSQL query returning one column named `msisdn` |
| stdin | `-source -` | Text, CSV or JSON piped into the process |

The default `format: "auto"` detects JSON by its opening bracket/brace; otherwise
it reads CSV, which also handles a one-number-per-line text file. `-format text`,
`-format csv`, or `-format json` overrides this choice. JSON accepts a string array
or an object containing a `msisdns` string array. Multi-column CSV must have exactly
one `msisdn` header (case-insensitive); a single-column file may omit its header.

```text
msisdn
233240000001
233240000002
```

```json
{"msisdns": ["233240000001", "233240000002"]}
```

These are illustrative values, not an authorized live feed. Numbers must contain
7–15 ASCII digits, optionally prefixed by `+`. Surrounding whitespace and the `+`
are removed; leading zeroes are preserved. The processor does not infer country
codes or convert local numbers. Use a consistent format accepted by the service.
Duplicates are removed after this normalization, keeping the first occurrence.
Invalid or empty records reject the whole feed before any batch is created (blank
lines in text files are ignored). Spreadsheet scientific notation is rejected.

For an authenticated HTTP feed, set `source_token_env` to the **name** of the
environment variable holding its bearer token. This token is separate from
`INTERNAL_API_SECRET`. Redirects are rejected for both feed and API requests.

For PostgreSQL, configure:

```json
{
  "source": "database",
  "database_url_env": "SUBSCRIPTION_SOURCE_DATABASE_URL",
  "database_query": "SELECT msisdn::text AS msisdn FROM subscription_feed WHERE eligible = true ORDER BY msisdn"
}
```

Merge these settings into the full configuration. Set the named environment
variable to your PostgreSQL connection URL. Choose your existing table, eligibility
predicate and a stable `ORDER BY`; the processor does not create a table or mark
rows as consumed. It reads the query in a read-only, repeatable-read transaction
with a request timeout. Exactly one result set and one non-null column named
`msisdn` are required. The database driver is PostgreSQL, already used by this
service; other database engines are not supported by this command.

Each invocation consumes one finite snapshot. HTTP pagination, continuous polling,
queue acknowledgement and database row leasing are not implemented. The source
is buffered in memory, capped by `max_source_bytes` (default 64 MiB); database rows
are capped by their JSON representation. For a large campaign, supply bounded
snapshots with a separate `state_file` for each.

## Batching and recovery

`batch_size` controls chunk size, including the shorter final chunk.
`wait_between_calls` sets the delay between chunks. `poll_interval`,
`max_polling_duration`, and `request_timeout` bound status checks and I/O.

The checkpoint records a fingerprint of the ordered, normalized feed, API URL,
routing, products and batch size, plus the current job ID and aggregate counts.
It contains no raw MSISDNs. Completed chunks are not replayed. Rerun with the same
configuration and input to resume polling an accepted job. Changed input or routing
is rejected; use the original snapshot to resume, and a new checkpoint for a
separate campaign. For HTTP/database feeds that may change, preserve their snapshot
before starting if reliable later resumption is required.

The API does not support idempotent enqueue and stores jobs in memory. The client
therefore saves submission intent **before** sending each POST. If the response
is lost or lacks a job ID, it stops with `submitting: true`; it never automatically
reposts. Reconcile with the service before changing this checkpoint. If the job is
known, record its `job_id` and set `submitting` to `false`; if non-acceptance has
been established, clear `submitting` to allow the same chunk to submit. A missing
job after a server restart likewise needs reconciliation, not automatic replay.
This is duplicate prevention for known client state, not exactly-once delivery.

A fully accounted terminal batch (including individual subscription failures)
advances the checkpoint. Failures contribute to the final nonzero exit status.
Cancelled jobs, early job failures with incomplete totals, inconsistent responses,
HTTP errors and polling timeouts stop processing while retaining the current job.
SIGINT/SIGTERM cancels client HTTP/database work; an already accepted server job
may continue. Rerun to observe its result.

An exclusive `<state_file>.lock` prevents concurrent runs using the same checkpoint.
A hard kill can leave this lock behind. Remove it only after confirming the old
process has stopped. Different checkpoint files represent independent campaigns
and do not deduplicate against each other. Keep checkpoints on a local Linux
filesystem supporting atomic rename and fsync. Logs show counts and job IDs,
not source records, credentials or server response bodies.

## Verification

```sh
just test
```

The focused suite checks parsing, HTTP sources, explicit batch requests, HMAC,
short final chunks, duplicate removal, dry-run, failure handling and resumption
against local HTTP contract fixtures. It does not contact TIMWE.

For the real PostgreSQL source integration test, provide
`TEST_SUBSCRIPTION_PROCESSOR_DATABASE_URL` pointing to a disposable database named
`subscription_processor_test`, then run `just test`. The test verifies the database
name before executing queries. Without this variable, that integration test is
explicitly skipped. It checks snapshot reads, validation and read-only enforcement.
