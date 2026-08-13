-- One ACTIVE otp_api credential per tenant, across ALL channels.
--
-- Same reasoning as uniq_tenant_channel_credentials_sms_api_active: delegated
-- OTP resolves the provider by tenant alone
-- (ArkeselOTPProvider.resolveOTPConfig), so two ACTIVE otp_api rows under
-- different channels would make the login path nondeterministic. Here that is
-- sharper than for sms_api, because the row also decides WHICH authentication
-- path a tenant is on, not merely which gateway delivers the message.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_tenant_channel_credentials_otp_api_active
    ON tenant_channel_credentials (tenant_id)
    WHERE purpose = 'otp_api' AND status = 'ACTIVE';
