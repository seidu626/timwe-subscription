-- TMP-075: enforce tenant ownership for acquisition/admin tables after live
-- proof shows there are no remaining tenantless rows.

SET lock_timeout = '10s';
SET statement_timeout = '10min';

DO $$
DECLARE
  target_table text;
  tenantless_exists boolean;
BEGIN
  FOR target_table IN
    SELECT table_name
    FROM (
      VALUES
        ('campaigns'),
        ('acquisition_transactions'),
        ('postback_outbox'),
        ('products'),
        ('userbase'),
        ('userbase_import_jobs'),
        ('userbase_import_errors')
    ) AS target(table_name)
    WHERE EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'public'
        AND table_name = target.table_name
        AND column_name = 'tenant_id'
    )
    AND EXISTS (
      SELECT 1
      FROM pg_catalog.pg_class c
      JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname = 'public'
        AND c.relname = target.table_name
    )
  LOOP
    EXECUTE format(
      'SELECT EXISTS (SELECT 1 FROM public.%I WHERE tenant_id IS NULL)',
      target_table
    )
    INTO tenantless_exists;

    IF tenantless_exists THEN
      RAISE EXCEPTION '% still contains tenantless rows', target_table;
    END IF;
  END LOOP;
END $$;

ALTER TABLE campaigns
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE acquisition_transactions
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE postback_outbox
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE products
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE userbase
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE userbase_import_jobs
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE userbase_import_errors
  ALTER COLUMN tenant_id SET NOT NULL;
