-- Series delivery-health aggregates group message_outbox by
-- (series_id, status, created_at); the FK on series_id has no index, so
-- without this each series costs a full outbox scan.
-- Apply with CONCURRENTLY outside a transaction on live databases:
--   CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_message_outbox_series_status_created
--     ON message_outbox (series_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_message_outbox_series_status_created
    ON message_outbox (series_id, status, created_at);
