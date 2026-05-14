-- TMP-074: notifications are tenant-owned after the canonical backfill.
-- Keep channel_id nullable for providers that do not supply channel context.

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM notifications WHERE tenant_id IS NULL) THEN
    RAISE EXCEPTION 'notifications still contains tenantless rows';
  END IF;
END $$;

DROP INDEX IF EXISTS idx_notifications_charge_legacy_tx_uuid;

ALTER TABLE notifications
  ALTER COLUMN tenant_id SET NOT NULL;
