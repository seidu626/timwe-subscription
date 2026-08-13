-- One ACTIVE sms_api credential per tenant, across ALL channels.
--
-- tenant_channel_credentials enforces ACTIVE-uniqueness per
-- (tenant_id, channel_id, purpose), which is correct for provider_api but
-- ambiguous for sms_api: login OTP delivery resolves the gateway by tenant
-- alone (TenantSMSSender.resolveGatewayConfig), so two ACTIVE sms_api rows
-- under different channels would make OTP routing nondeterministic.
-- Rotation flows (deactivate old + activate new in one transaction) are
-- unaffected: the index is evaluated per-row exactly like the existing
-- (tenant_id, channel_id, purpose) ACTIVE index.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_tenant_channel_credentials_sms_api_active
    ON tenant_channel_credentials (tenant_id)
    WHERE purpose = 'sms_api' AND status = 'ACTIVE';
