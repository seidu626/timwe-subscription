-- Seed the canonical nrg tenant channel and provider credential reference.
-- Forward-only, idempotent, and reference-only: this migration does not store
-- raw provider credential material in the database.

INSERT INTO tenants (id, tenant_key, name, status, default_country, metadata_json)
VALUES (
    gen_random_uuid(),
    'nrg',
    'NRG',
    'ACTIVE',
    'GH',
    jsonb_build_object('migration', 'TMP-050', 'kind', 'canonical-default')
)
ON CONFLICT (tenant_key) DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    default_country = EXCLUDED.default_country,
    metadata_json = EXCLUDED.metadata_json,
    updated_at = NOW();

INSERT INTO tenant_channels (id, tenant_id, channel_key, provider, country, operator, capabilities, status)
SELECT
    gen_random_uuid(),
    t.id,
    'web-gh-airteltigo',
    'timwe',
    'GH',
    'AirtelTigo',
    ARRAY['optin','confirm','mt','charge']::text[],
    'ACTIVE'
FROM tenants t
WHERE t.tenant_key = 'nrg'
ON CONFLICT (tenant_id, channel_key) DO UPDATE SET
    provider = EXCLUDED.provider,
    country = EXCLUDED.country,
    operator = EXCLUDED.operator,
    capabilities = EXCLUDED.capabilities,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO tenant_channel_credentials (
    id,
    tenant_id,
    channel_id,
    purpose,
    version,
    status,
    secret_ref,
    secret_ref_display,
    secret_fingerprint,
    created_by,
    activated_at
)
SELECT
    gen_random_uuid(),
    t.id,
    c.id,
    'provider_api',
    1,
    'ACTIVE',
    'env://NRG_TIMWE_API_SECRET',
    'nrg-timwe-api',
    'e941c0e71c96a51f26e41bbd28541428e543cef6bfc616a8ad0814b8ea6197e5',
    'seed_migration',
    NOW()
FROM tenants t
JOIN tenant_channels c
    ON c.tenant_id = t.id
    AND c.channel_key = 'web-gh-airteltigo'
WHERE t.tenant_key = 'nrg'
ON CONFLICT (tenant_id, channel_id, purpose, version) DO UPDATE SET
    status = EXCLUDED.status,
    secret_ref = EXCLUDED.secret_ref,
    secret_ref_display = EXCLUDED.secret_ref_display,
    secret_fingerprint = EXCLUDED.secret_fingerprint,
    updated_at = NOW(),
    activated_at = COALESCE(tenant_channel_credentials.activated_at, EXCLUDED.activated_at),
    deactivated_at = NULL;
