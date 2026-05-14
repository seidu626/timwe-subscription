# TMP-074 Tenant Null Residual Cleanup

## Evidence

- Live admin_activity_logs tenantless rows: 1 before, 0 after.
- Live notifications tenantless rows: 10 before, 0 after.
- Live campaigns tenantless rows: 0 before and after.
- Tenant-create activity row repaired to the created tenant (careerify), not blanket nrg.
- Legacy campaign and product_message_series partial indexes were dropped on the live schema.
- admin_activity_logs.tenant_id and notifications.tenant_id are now NOT NULL on the live schema.
