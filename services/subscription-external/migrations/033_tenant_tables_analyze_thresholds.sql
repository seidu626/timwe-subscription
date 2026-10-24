-- 033: re-analyze the tenant-keyed tables after 1% churn instead of 10%.
-- Measured on prod 2026-10-24: the Talentrivia USSD run added ~153k
-- subscriptions rows for a tenant the planner statistics had never seen.
-- Autoanalyze waits for 10% churn (~270k rows here), so the planner kept
-- estimating 1 row for (tenant_id, product_id) and served the per-callback
-- lookup on (tenant_id, user_identifier, product_id) from
-- idx_subscriptions_tenant_product_status: 196 ms and 25,761 buffers per call
-- instead of 0.08 ms. Each TIMWE callback held a connection that long, the
-- pools filled and the shared 50-slot cluster refused new connections
-- (SQLSTATE 53300). A manual ANALYZE at 10:16Z cleared it.
--
-- Storage parameters only. ALTER TABLE ... SET for autovacuum parameters
-- takes SHARE UPDATE EXCLUSIVE, which does not block reads or writes.
-- Revert: ALTER TABLE <t> RESET (autovacuum_analyze_scale_factor, autovacuum_analyze_threshold);
ALTER TABLE subscriptions SET (autovacuum_analyze_scale_factor = 0.01, autovacuum_analyze_threshold = 1000);
ALTER TABLE notifications SET (autovacuum_analyze_scale_factor = 0.01, autovacuum_analyze_threshold = 1000);
ALTER TABLE subscription_message_state SET (autovacuum_analyze_scale_factor = 0.01, autovacuum_analyze_threshold = 1000);
