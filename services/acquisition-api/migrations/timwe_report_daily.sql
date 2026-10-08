-- TIMWE Charges Consolidated daily observations. Apply before enabling TIMWE_REPORT_SOURCES.
-- Rollback: stop the importer, then DROP TABLE timwe_report_daily; DROP TABLE timwe_report_syncs.
CREATE TABLE IF NOT EXISTS timwe_report_syncs (
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    source_key TEXT NOT NULL,
    shortcode TEXT NOT NULL,
    product_id INTEGER NOT NULL,
    partner_id INTEGER NOT NULL,
    window_start DATE,
    window_end DATE,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_error_code TEXT,
    PRIMARY KEY (tenant_id, source_key),
    CONSTRAINT timwe_report_syncs_source_key_nonempty CHECK (length(trim(source_key)) > 0),
    CONSTRAINT timwe_report_syncs_product_positive CHECK (product_id > 0)
);

CREATE TABLE IF NOT EXISTS timwe_report_daily (
    tenant_id UUID NOT NULL,
    source_key TEXT NOT NULL,
    report_day DATE NOT NULL,
    partner_id INTEGER NOT NULL,
    shortcode TEXT NOT NULL,
    product_id INTEGER NOT NULL,
    product_name TEXT NOT NULL,
    pricepoint_id INTEGER NOT NULL,
    pricepoint_value NUMERIC(20, 4) NOT NULL,
    success_billings BIGINT NOT NULL,
    revenue NUMERIC(20, 4) NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, source_key, report_day, partner_id, shortcode, product_id, pricepoint_id),
    FOREIGN KEY (tenant_id, source_key) REFERENCES timwe_report_syncs(tenant_id, source_key) ON DELETE CASCADE,
    CONSTRAINT timwe_report_daily_success_nonnegative CHECK (success_billings >= 0),
    CONSTRAINT timwe_report_daily_revenue_nonnegative CHECK (revenue >= 0)
);

CREATE INDEX IF NOT EXISTS idx_timwe_report_daily_tenant_day
    ON timwe_report_daily (tenant_id, report_day DESC, product_id, shortcode);
