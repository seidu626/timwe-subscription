-- Tenant acquisition transaction ownership.
-- Runs after tenant campaign binding so tenant-owned duplicate slugs are safe.

ALTER TABLE acquisition_transactions
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE RESTRICT;

DO $$
DECLARE
    legacy_fk RECORD;
BEGIN
    FOR legacy_fk IN
        SELECT
            format('%I.%I', dependent_namespace.nspname, dependent_table.relname) AS dependent_table_name,
            constraint_record.conname AS constraint_name
        FROM pg_constraint AS constraint_record
        JOIN pg_class AS dependent_table
            ON dependent_table.oid = constraint_record.conrelid
        JOIN pg_namespace AS dependent_namespace
            ON dependent_namespace.oid = dependent_table.relnamespace
        JOIN pg_class AS referenced_table
            ON referenced_table.oid = constraint_record.confrelid
        JOIN pg_namespace AS referenced_namespace
            ON referenced_namespace.oid = referenced_table.relnamespace
        JOIN pg_attribute AS referenced_column
            ON referenced_column.attrelid = referenced_table.oid
        WHERE constraint_record.contype = 'f'
          AND referenced_namespace.nspname = 'public'
          AND referenced_table.relname = 'campaigns'
          AND referenced_column.attname = 'slug'
          AND referenced_column.attnum = ANY (constraint_record.confkey)
    LOOP
        EXECUTE format(
            'ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I',
            legacy_fk.dependent_table_name,
            legacy_fk.constraint_name
        );
    END LOOP;
END $$;

ALTER TABLE campaigns
    DROP CONSTRAINT IF EXISTS campaigns_slug_key;

DROP INDEX IF EXISTS idx_campaigns_slug;

CREATE INDEX IF NOT EXISTS idx_acq_trans_tenant_campaign_msisdn
    ON acquisition_transactions (tenant_id, campaign_slug, msisdn, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_acq_trans_tenant_status
    ON acquisition_transactions (tenant_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_acq_trans_tenant_click
    ON acquisition_transactions (tenant_id, ad_provider, click_id, created_at DESC);
