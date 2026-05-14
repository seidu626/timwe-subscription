-- TMP-055: product_message_series has no remaining tenantless rows in the
-- credentialed schema proof. Drop the legacy partial uniqueness lane while
-- leaving notification idempotency compatibility in place until notifications
-- also have zero tenantless rows.

DROP INDEX IF EXISTS idx_product_message_series_legacy_key;
