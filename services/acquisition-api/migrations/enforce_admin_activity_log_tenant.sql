-- TMP-074: tenant-create activity belongs to the created tenant, not the
-- canonical migration tenant. Fail rather than silently enforcing NOT NULL
-- while tenantless rows remain.

UPDATE admin_activity_logs AS l
SET tenant_id = t.id
FROM tenants AS t
WHERE l.tenant_id IS NULL
  AND l.entity_type = 'tenant'
  AND l.entity_id = t.id::text;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM admin_activity_logs WHERE tenant_id IS NULL) THEN
    RAISE EXCEPTION 'admin_activity_logs still contains tenantless rows';
  END IF;
END $$;

ALTER TABLE admin_activity_logs
  ALTER COLUMN tenant_id SET NOT NULL;
