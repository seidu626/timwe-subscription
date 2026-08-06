# Distributed Systems Review: opt-in confirmation SMS

| Concern | Current behavior | Risk | Required change/test |
|---|---|---|---|
| Idempotency key | Carrier callbacks expose `externalTxId`; outbox has a unique key | Duplicate callbacks could send duplicate SMS | Hash event + template ID + external transaction ID; duplicate callback unit test |
| Duplicate delivery | Worker is at-least-once and retries failed jobs | Replays can repeat enqueue | Atomic `INSERT ... ON CONFLICT DO NOTHING` in callback transaction |
| Out-of-order events | Confirmation handles only `USER_OPTIN` | Later events could arrive before an opt-in callback | Event-specific template lookup; no behavior added for other event types |
| Retry backoff | Existing worker applies bounded backoff and max attempts | Provider outage grows pending outbox | Preserve worker retry configuration and metrics |
| Timeout | Existing MT dispatcher has an explicit HTTP timeout | Provider can stall a worker | Preserve configured `HTTPTimeout` |
| Partial failure | Callback record and enqueue previously had no shared operation | Callback could persist without its confirmation | Persist notification and enqueue in one local DB transaction |
| Transaction | One PostgreSQL database owns notifications, subscriptions, and outbox | Split commits produce inconsistent state | One transaction; rollback on lookup/enqueue errors |
| Compensation | SMS is forward-only after provider acceptance | Sent SMS cannot be recalled | No saga; idempotency prevents repeat intent |
| Consistency | Worker polls committed outbox rows | SMS delivery is eventually consistent | Commit makes work visible atomically; existing worker handles convergence |
| Cache | Template dispatch does not use cache | Stale enabled state could send after disable | Read template from PostgreSQL on each callback |
| Clock | `NOW()` controls immediate availability | App clock skew could delay a row | Use database clock |
| Backpressure | Existing database outbox absorbs provider slowdown | Table growth during outage | Preserve pending-status indexes and worker metrics |
| Poison message | Worker marks exhausted retries failed | Permanent provider rejection can loop | Preserve max attempts and FAILED state |
| Observability | Worker records tenant-safe metrics and structured status | Failures must be attributable without PII | Preserve tenant/channel safe labels; never log rendered text/MSISDN |
| Alert signal | Existing worker exposes dispatch status metrics | Queue failures could be missed | Existing FAILED/retry metrics remain the operator signal |
| Safe replay | Unique deterministic enqueue key survives callback replay | Replay after restart could duplicate side effects | Database uniqueness is the replay guard; SQL assertion covers row shape |

Open guards: none. Residual risk: migration apply proof requires the supervisor scratch PostgreSQL run specified by the task.
