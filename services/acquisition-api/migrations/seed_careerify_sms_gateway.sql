-- Bind the careerify tenant's SMS gateway credential (purpose sms_api) for
-- Dayline login OTP delivery. The secret itself is NOT stored here: the row
-- references env://CAREERIFY_SMS_GATEWAY_CONFIG, which must contain the
-- gateway JSON blob (see docs/tenant-channel-onboarding.md, "SMS Gateway
-- Credential") in the acquisition-api container environment, e.g. for
-- Arkesel v2:
--   {"url":"https://sms.arkesel.com/api/v2/sms/send","method":"POST",
--    "headers":{"api-key":"<KEY>"},
--    "body_template":"{\"sender\":\"{{sender}}\",\"message\":\"{{text}}\",\"recipients\":[\"{{msisdn}}\"]}",
--    "sender_id":"Dayline","success_field":"status","success_value":"success"}
-- Idempotent: skipped if the tenant already has an ACTIVE sms_api credential
-- (also enforced by uniq_tenant_channel_credentials_sms_api_active).
INSERT INTO tenant_channel_credentials
    (id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display, secret_fingerprint, created_by)
SELECT
    gen_random_uuid(),
    t.id,
    c.id,
    'sms_api',
    1,
    'ACTIVE',
    'env://CAREERIFY_SMS_GATEWAY_CONFIG',
    'env://CAREERIFY_SMS_GATEWAY_CONFIG',
    encode(sha256('env://CAREERIFY_SMS_GATEWAY_CONFIG'::bytea), 'hex'),
    'seed_careerify_sms_gateway'
FROM tenants t
JOIN tenant_channels c ON c.tenant_id = t.id AND c.status = 'ACTIVE'
WHERE t.tenant_key = 'careerify'
  AND NOT EXISTS (
      SELECT 1 FROM tenant_channel_credentials existing
      WHERE existing.tenant_id = t.id
        AND existing.purpose = 'sms_api'
        AND existing.status = 'ACTIVE'
  )
ORDER BY c.created_at
LIMIT 1;
