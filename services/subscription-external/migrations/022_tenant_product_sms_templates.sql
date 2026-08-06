-- slice-harness: allow-new-canonical-path: requested tenant/product SMS template migration.
-- Additive outbox design: cadence rows keep their series/content reference;
-- direct confirmation rows carry message_text instead. Existing rows remain valid.

BEGIN;

CREATE TABLE IF NOT EXISTS tenant_product_sms_templates (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    product_id  INTEGER NOT NULL,
    event_type  TEXT NOT NULL DEFAULT 'USER_OPTIN',
    enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    template    TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, product_id, event_type),
    CHECK (product_id > 0),
    CHECK (event_type ~ '^[A-Z][A-Z0-9_]*$'),
    CHECK (length(btrim(template)) BETWEEN 1 AND 2000)
);

ALTER TABLE message_outbox
    ALTER COLUMN series_id DROP NOT NULL,
    ALTER COLUMN content_item_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS message_text TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'message_outbox'::regclass
          AND conname = 'message_outbox_content_source_check'
    ) THEN
        ALTER TABLE message_outbox
            ADD CONSTRAINT message_outbox_content_source_check CHECK (
                (series_id IS NOT NULL AND content_item_id IS NOT NULL AND message_text IS NULL)
                OR
                (series_id IS NULL AND content_item_id IS NULL AND length(btrim(message_text)) > 0)
            ) NOT VALID;
    END IF;
END $$;

ALTER TABLE message_outbox
    VALIDATE CONSTRAINT message_outbox_content_source_check;

DROP TRIGGER IF EXISTS update_tenant_product_sms_templates_updated_at ON tenant_product_sms_templates;
CREATE TRIGGER update_tenant_product_sms_templates_updated_at
    BEFORE UPDATE ON tenant_product_sms_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;

-- Example only; intentionally not seeded by this migration:
-- INSERT INTO tenant_product_sms_templates (tenant_id, product_id, event_type, enabled, template)
-- SELECT id, 32535, 'USER_OPTIN', TRUE,
--        'Welcome to product {{product_id}} on {{large_account}}. Subscriber ending {{msisdn}} is active.'
-- FROM tenants WHERE tenant_key = 'careerify'
-- ON CONFLICT (tenant_id, product_id, event_type) DO UPDATE
-- SET enabled = EXCLUDED.enabled, template = EXCLUDED.template, updated_at = NOW();
