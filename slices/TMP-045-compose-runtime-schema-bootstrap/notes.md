# TMP-045 Notes

- User auto-approved the TMP-034/TMP-035/TMP-036 schema provisioning decisions in the session directive.
- `ops/db/bootstrap/001_runtime_base.sql` intentionally does not define `message_outbox`, `postback_outbox`, or `postback_attempts`.
- Initial bootstrap proof failed when runtime base defined `landing_versions` with a foreign key on `campaigns.slug`; the table is not part of the worker startup path and was removed to avoid blocking the existing tenant slug migration.
- Explorer review confirmed notification/cadence minimal runtime relations: `message_outbox`, `subscriptions`, `message_content_items`, `subscription_message_state`, `product_message_series`, `message_schedule_rules`.
- Explorer review confirmed acquisition-api owns canonical postback schema and postback-dispatcher consumes it.
