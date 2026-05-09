# TMP-036 Domain Brief

- Actor: platform-operator
- Business outcome: Postback dispatcher polling runs in compose without missing postback_outbox relation errors.
- Domain invariant: full-system verification must not claim end-to-end readiness while this blocker remains unresolved.
- Entrypoint: docker compose postback-dispatcher runtime startup
- Trigger: Verifier runs targeted postback-dispatcher smoke after TMP-032 DB env fix.
- Risk: Schema/migration provisioning is approval-gated by repo risk boundaries. The postback_outbox ownership/provisioning path must be selected before implementation.

## Story Craft

The story is concrete and testable: Postback dispatcher starts and connects to DB, then polling logs pq: relation postback_outbox does not exist against the empty compose DB. The expected outcome is: The compose DB provisioning path applies postback outbox schema before postback-dispatcher polling.

## Roadmap To Slices

This is a blocked follow-up slice under TMP-021. It records the smallest independently verifiable blocker without implementing approval-gated changes.
