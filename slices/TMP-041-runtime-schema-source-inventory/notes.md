# TMP-041 Notes

- TMP-034 acquisition-api runtime blocker maps to `products` and `userbase` base tables in `services/pg_schema.sql`.
- `services/pg_schema.sql` is hand-maintained DDL, not a numbered migration. It also contains a duplicate `listResponse` declaration, so it should not be treated as a clean migration runner without review.
- TMP-035 notification-worker blocker maps to `message_outbox` in `services/subscription-external/migrations/011_message_cadence_engine.sql`.
- TMP-036 postback-dispatcher blocker maps to `postback_outbox` in both:
  - `services/acquisition-api/migrations/create_postback_tables.sql`
  - `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`
- The duplicate `postback_outbox` definitions make canonical migration ordering a release decision; this slice does not choose or apply that ordering.
- Remaining blocker: approved migration provisioning/orchestration for the compose runtime.
