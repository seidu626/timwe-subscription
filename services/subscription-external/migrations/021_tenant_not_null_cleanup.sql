-- TMP-075: enforce tenant ownership for subscription/cadence tables after
-- live proof shows there are no remaining tenantless rows. Keep channel_id
-- nullable because provider channel proof is a separate compatibility track.

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
        ('subscriptions'),
        ('admin_subscription_action_logs'),
        ('product_message_series'),
        ('message_content_items'),
        ('subscription_message_state'),
        ('message_outbox')
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

ALTER TABLE subscriptions
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE admin_subscription_action_logs
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE product_message_series
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE message_content_items
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE subscription_message_state
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE message_outbox
  ALTER COLUMN tenant_id SET NOT NULL;
