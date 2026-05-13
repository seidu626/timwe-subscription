-- Seed the careerify tenant, its web-gh-airteltigo channel, and provider credentials.
-- Forward-only, idempotent: each INSERT uses ON CONFLICT ... DO NOTHING so a second run
-- is a safe no-op.  No rows outside these three are touched.

-- ── 1. Tenant ───────────────────────────────────────────────────────────────────────────────
INSERT INTO tenants (id, tenant_key, name, status, default_country, metadata_json)
VALUES (
    gen_random_uuid(),
    'careerify',
    'Careerify',
    'ACTIVE',
    'GH',
    '{}'::jsonb
)
ON CONFLICT (tenant_key) DO NOTHING;

-- ── 2. Channel ──────────────────────────────────────────────────────────────────────────────
-- Resolved via a sub-select on tenant_key so the channel is bound to the correct tenant UUID
-- even if the row already existed before this script ran.
INSERT INTO tenant_channels (id, tenant_id, channel_key, provider, country, operator, capabilities, status)
SELECT
    gen_random_uuid(),
    t.id,
    'web-gh-airteltigo',
    'timwe',
    'GH',
    'AirtelTigo Ghana',
    ARRAY['optin','confirm','mt','charge']::text[],
    'ACTIVE'
FROM tenants t
WHERE t.tenant_key = 'careerify'
ON CONFLICT (tenant_id, channel_key) DO NOTHING;

-- ── 3. Provider credential ──────────────────────────────────────────────────────────────────
-- secret_ref uses the env:// prefix (satisfies chk_tenant_channel_credentials_secret_ref).
-- secret_fingerprint is a 64-character hex string (satisfies the length=64 check).
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
    created_by
)
SELECT
    gen_random_uuid(),
    t.id,
    c.id,
    'provider_api',
    1,
    'ACTIVE',
    'env://CAREERIFY_TIMWE_API_SECRET',
    'careerify-timwe-api',
    'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2',
    'seed_migration'
FROM tenants t
JOIN tenant_channels c
    ON c.tenant_id = t.id
    AND c.channel_key = 'web-gh-airteltigo'
WHERE t.tenant_key = 'careerify'
ON CONFLICT (tenant_id, channel_id, purpose, version) DO NOTHING;
