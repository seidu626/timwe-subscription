-- OPT-IN: binding this credential switches the careerify tenant's Dayline app
-- login from the local OTP lifecycle to delegated OTP, where the provider
-- mints, delivers and verifies the code. Do not run it to "enable SMS": the
-- local lifecycle already works through the sms_api credential, and the two
-- are alternatives, not layers. Revoke the row (status <> 'ACTIVE') to switch
-- back; in-flight OTPs then fail verification and users retry.
--
-- The secret itself is NOT stored here: the row references
-- env://CAREERIFY_OTP_GATEWAY_CONFIG, which must contain the provider JSON
-- blob (see docs/tenant-channel-onboarding.md, "Delegated OTP Credential") in
-- the acquisition-api container environment, e.g. for Arkesel:
--   {"generate_url":"https://sms.arkesel.com/api/otp/generate",
--    "verify_url":"https://sms.arkesel.com/api/otp/verify",
--    "headers":{"api-key":"<KEY>"},"sender_id":"Dayline"}
-- Arkesel note: OTP requires the account's MAIN api key and is billed to the
-- main balance, not the SMS balance.
--
-- Idempotent: skipped if the tenant already has an ACTIVE otp_api credential
-- (also enforced by uniq_tenant_channel_credentials_otp_api_active).
INSERT INTO tenant_channel_credentials
    (id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display, secret_fingerprint, created_by)
SELECT
    gen_random_uuid(),
    t.id,
    c.id,
    'otp_api',
    1,
    'ACTIVE',
    'env://CAREERIFY_OTP_GATEWAY_CONFIG',
    'env://CAREERIFY_OTP_GATEWAY_CONFIG',
    encode(sha256('env://CAREERIFY_OTP_GATEWAY_CONFIG'::bytea), 'hex'),
    'seed_careerify_otp_gateway'
FROM tenants t
JOIN tenant_channels c ON c.tenant_id = t.id AND c.status = 'ACTIVE'
WHERE t.tenant_key = 'careerify'
  AND NOT EXISTS (
      SELECT 1 FROM tenant_channel_credentials existing
      WHERE existing.tenant_id = t.id
        AND existing.purpose = 'otp_api'
        AND existing.status = 'ACTIVE'
  )
ORDER BY c.created_at
LIMIT 1;
