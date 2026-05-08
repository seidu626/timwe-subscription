-- slice-harness: allow-new-canonical-path: TMP-017 makes charge ownership idempotent.
-- Direct billing charge events are owned by subscription-external. These partial
-- indexes make tenant charge ownership idempotent while keeping legacy rows compatible.

CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_charge_tenant_tx_uuid
    ON notifications (tenant_id, transaction_uuid)
    WHERE type = 'CHARGE'
      AND tenant_id IS NOT NULL
      AND transaction_uuid IS NOT NULL
      AND transaction_uuid <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_charge_legacy_tx_uuid
    ON notifications (transaction_uuid)
    WHERE type = 'CHARGE'
      AND tenant_id IS NULL
      AND transaction_uuid IS NOT NULL
      AND transaction_uuid <> '';
