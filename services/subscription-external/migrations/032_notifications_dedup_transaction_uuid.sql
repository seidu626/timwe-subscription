-- 032: one notifications row per (tenant_id, type, transaction_uuid).
-- TIMWE re-posts a callback until it gets a 2xx, and nothing stopped the
-- replays from landing twice. Measured on prod 2026-10-08: 6,742 duplicate
-- groups, 7,220 extra rows, up to 10 copies ~30 min apart, almost all
-- USER_RENEWED 2024-11..2025-11. Writers now insert with ON CONFLICT DO
-- NOTHING, so a replay becomes a no-op 200. This index also backs
-- CreateChargeNotificationOnce, whose 018 index was never applied on prod.
-- Nothing references notifications by foreign key.
--
-- Step 1 keeps the oldest row of each group and archives the rest. It is
-- safe to re-run: it only picks up duplicates that are still present.
BEGIN;
SET LOCAL statement_timeout = '15min';

CREATE TABLE IF NOT EXISTS notifications_dedup_backup_20261008 AS
    SELECT * FROM notifications WITH NO DATA;

INSERT INTO notifications_dedup_backup_20261008
SELECT n.*
FROM notifications n
JOIN (
    SELECT id, row_number() OVER (PARTITION BY tenant_id, type, transaction_uuid ORDER BY id) AS copy_no
    FROM notifications
    WHERE transaction_uuid <> ''
) ranked ON ranked.id = n.id AND ranked.copy_no > 1;

DELETE FROM notifications n
USING notifications_dedup_backup_20261008 b
WHERE n.id = b.id;

COMMIT;

-- Step 2 runs outside a transaction on live databases. If a new duplicate
-- slips in between the steps, the build fails and leaves an INVALID index:
-- DROP INDEX CONCURRENTLY it, re-run step 1, then retry.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_notifications_tenant_type_tx_uuid
    ON notifications (tenant_id, type, transaction_uuid)
    WHERE transaction_uuid <> '';

-- Rollback:
--   DROP INDEX CONCURRENTLY IF EXISTS idx_notifications_tenant_type_tx_uuid;
--   INSERT INTO notifications SELECT * FROM notifications_dedup_backup_20261008
--     WHERE id NOT IN (SELECT id FROM notifications);
