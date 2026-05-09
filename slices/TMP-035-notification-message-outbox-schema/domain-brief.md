# TMP-035 Domain Brief

- Actor: platform-operator
- Business outcome: Notification worker dispatch loop runs in compose without missing message_outbox relation errors.
- Domain invariant: full-system verification must not claim end-to-end readiness while this blocker remains unresolved.
- Entrypoint: docker compose notification-worker runtime startup
- Trigger: Verifier runs targeted notification-worker smoke after TMP-031 DB env fix.
- Risk: Schema/migration provisioning is approval-gated by repo risk boundaries. The message_outbox ownership/provisioning path must be selected before implementation.

## Story Craft

The story is concrete and testable: Notification worker starts and exposes metrics, then dispatcher logs pq: relation message_outbox does not exist against the empty compose DB. The expected outcome is: The compose DB provisioning path applies the message cadence/outbox schema before notification-worker dispatch polling.

## Roadmap To Slices

This is a blocked follow-up slice under TMP-021. It records the smallest independently verifiable blocker without implementing approval-gated changes.
