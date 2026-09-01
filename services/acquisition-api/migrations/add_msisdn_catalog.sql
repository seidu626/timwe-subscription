CREATE TABLE IF NOT EXISTS msisdn_catalog (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    msisdn VARCHAR(15) NOT NULL,
    region CHAR(2) NOT NULL,
    telco VARCHAR(32) NOT NULL,
    verification_status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    dnd BOOLEAN NOT NULL DEFAULT FALSE,
    invalid BOOLEAN NOT NULL DEFAULT FALSE,
    invalid_reason TEXT,
    source VARCHAR(32) NOT NULL DEFAULT 'MANUAL',
    generation_batch_id VARCHAR(128),
    verification_provider VARCHAR(32),
    verification_reference VARCHAR(128),
    verification_attempts INTEGER NOT NULL DEFAULT 0,
    verified_at TIMESTAMPTZ,
    last_verification_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_msisdn_catalog_region CHECK (region IN ('GH', 'NG')),
    CONSTRAINT chk_msisdn_catalog_status CHECK (verification_status IN ('PENDING', 'VERIFIED', 'INVALID', 'ERROR')),
    CONSTRAINT chk_msisdn_catalog_digits CHECK (msisdn ~ '^[0-9]{12,13}$'),
    CONSTRAINT chk_msisdn_catalog_attempts CHECK (verification_attempts >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_msisdn_catalog_tenant_region_msisdn
    ON msisdn_catalog (tenant_id, region, msisdn);

CREATE INDEX IF NOT EXISTS idx_msisdn_catalog_lookup
    ON msisdn_catalog (tenant_id, region, telco, verification_status, dnd, invalid, id DESC);

CREATE INDEX IF NOT EXISTS idx_msisdn_catalog_tenant_newest
    ON msisdn_catalog (tenant_id, id DESC);

CREATE INDEX IF NOT EXISTS idx_msisdn_catalog_msisdn_prefix
    ON msisdn_catalog (tenant_id, msisdn text_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_msisdn_catalog_pending_verification
    ON msisdn_catalog (tenant_id, id)
    WHERE region = 'GH' AND telco = 'MTN' AND verification_status IN ('PENDING', 'ERROR') AND NOT dnd AND NOT invalid;

CREATE INDEX IF NOT EXISTS idx_msisdn_catalog_batch
    ON msisdn_catalog (tenant_id, generation_batch_id)
    WHERE generation_batch_id IS NOT NULL;

-- Small aggregate table used by the management UI. Statement-level transition
-- triggers update one row per affected pool, avoiding COUNT(*) scans across the
-- full catalog and avoiding one counter write per imported number.
CREATE TABLE IF NOT EXISTS msisdn_catalog_pool_counts (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    region CHAR(2) NOT NULL,
    telco VARCHAR(32) NOT NULL,
    verification_status VARCHAR(16) NOT NULL,
    dnd BOOLEAN NOT NULL,
    invalid BOOLEAN NOT NULL,
    record_count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, region, telco, verification_status, dnd, invalid),
    CONSTRAINT chk_msisdn_catalog_pool_count CHECK (record_count >= 0)
);

INSERT INTO msisdn_catalog_pool_counts
    (tenant_id, region, telco, verification_status, dnd, invalid, record_count)
SELECT tenant_id, region, telco, verification_status, dnd, invalid, COUNT(*)
FROM msisdn_catalog
GROUP BY tenant_id, region, telco, verification_status, dnd, invalid
ON CONFLICT (tenant_id, region, telco, verification_status, dnd, invalid)
DO UPDATE SET record_count = EXCLUDED.record_count, updated_at = NOW();

CREATE OR REPLACE FUNCTION refresh_msisdn_catalog_counts_after_insert()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO msisdn_catalog_pool_counts
        (tenant_id, region, telco, verification_status, dnd, invalid, record_count)
    SELECT tenant_id, region, telco, verification_status, dnd, invalid, COUNT(*)
    FROM new_catalog_rows
    GROUP BY tenant_id, region, telco, verification_status, dnd, invalid
    ON CONFLICT (tenant_id, region, telco, verification_status, dnd, invalid)
    DO UPDATE SET record_count = msisdn_catalog_pool_counts.record_count + EXCLUDED.record_count, updated_at = NOW();
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION refresh_msisdn_catalog_counts_after_update()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE msisdn_catalog_pool_counts AS counts
    SET record_count = GREATEST(0, counts.record_count - delta.removed), updated_at = NOW()
    FROM (
        SELECT tenant_id, region, telco, verification_status, dnd, invalid, COUNT(*) AS removed
        FROM old_catalog_rows
        GROUP BY tenant_id, region, telco, verification_status, dnd, invalid
    ) AS delta
    WHERE counts.tenant_id = delta.tenant_id AND counts.region = delta.region
      AND counts.telco = delta.telco AND counts.verification_status = delta.verification_status
      AND counts.dnd = delta.dnd AND counts.invalid = delta.invalid;

    INSERT INTO msisdn_catalog_pool_counts
        (tenant_id, region, telco, verification_status, dnd, invalid, record_count)
    SELECT tenant_id, region, telco, verification_status, dnd, invalid, COUNT(*)
    FROM new_catalog_rows
    GROUP BY tenant_id, region, telco, verification_status, dnd, invalid
    ON CONFLICT (tenant_id, region, telco, verification_status, dnd, invalid)
    DO UPDATE SET record_count = msisdn_catalog_pool_counts.record_count + EXCLUDED.record_count, updated_at = NOW();

    DELETE FROM msisdn_catalog_pool_counts WHERE record_count = 0;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION refresh_msisdn_catalog_counts_after_delete()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE msisdn_catalog_pool_counts AS counts
    SET record_count = GREATEST(0, counts.record_count - delta.removed), updated_at = NOW()
    FROM (
        SELECT tenant_id, region, telco, verification_status, dnd, invalid, COUNT(*) AS removed
        FROM old_catalog_rows
        GROUP BY tenant_id, region, telco, verification_status, dnd, invalid
    ) AS delta
    WHERE counts.tenant_id = delta.tenant_id AND counts.region = delta.region
      AND counts.telco = delta.telco AND counts.verification_status = delta.verification_status
      AND counts.dnd = delta.dnd AND counts.invalid = delta.invalid;
    DELETE FROM msisdn_catalog_pool_counts WHERE record_count = 0;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_msisdn_catalog_counts_insert ON msisdn_catalog;
CREATE TRIGGER trg_msisdn_catalog_counts_insert
AFTER INSERT ON msisdn_catalog
REFERENCING NEW TABLE AS new_catalog_rows
FOR EACH STATEMENT EXECUTE FUNCTION refresh_msisdn_catalog_counts_after_insert();

DROP TRIGGER IF EXISTS trg_msisdn_catalog_counts_update ON msisdn_catalog;
CREATE TRIGGER trg_msisdn_catalog_counts_update
AFTER UPDATE ON msisdn_catalog
REFERENCING OLD TABLE AS old_catalog_rows NEW TABLE AS new_catalog_rows
FOR EACH STATEMENT EXECUTE FUNCTION refresh_msisdn_catalog_counts_after_update();

DROP TRIGGER IF EXISTS trg_msisdn_catalog_counts_delete ON msisdn_catalog;
CREATE TRIGGER trg_msisdn_catalog_counts_delete
AFTER DELETE ON msisdn_catalog
REFERENCING OLD TABLE AS old_catalog_rows
FOR EACH STATEMENT EXECUTE FUNCTION refresh_msisdn_catalog_counts_after_delete();
