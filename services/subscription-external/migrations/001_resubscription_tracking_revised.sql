-- REVISED Migration script for resubscription tracking
-- File: migrations/001_resubscription_tracking_revised.sql

-- =========================================
-- 1. Add tracking columns to subscriptions table
-- =========================================

ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS last_charging_failure_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS charging_failure_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS charging_failure_reason VARCHAR(255),
ADD COLUMN IF NOT EXISTS last_resubscribe_attempt_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS resubscribe_attempt_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS resubscribe_status VARCHAR(50) DEFAULT NULL;

-- Add indexes for the new columns
CREATE INDEX IF NOT EXISTS idx_subscriptions_charging_failure 
    ON subscriptions(last_charging_failure_at, charging_failure_count)
    WHERE charging_failure_count > 0;

CREATE INDEX IF NOT EXISTS idx_subscriptions_resubscribe_status 
    ON subscriptions(resubscribe_status)
    WHERE resubscribe_status IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_resubscribe_attempt 
    ON subscriptions(last_resubscribe_attempt_at)
    WHERE last_resubscribe_attempt_at IS NOT NULL;

-- =========================================
-- 2. Create resubscription tracking table
-- =========================================

CREATE TABLE IF NOT EXISTS resubscription_tracking (
    id SERIAL PRIMARY KEY,
    subscription_id INTEGER NOT NULL,
    msisdn VARCHAR(15) NOT NULL,
    product_id INTEGER NOT NULL,
    original_status VARCHAR(50),
    attempt_number INTEGER DEFAULT 1,
    process_batch_id VARCHAR(100),
    
    -- Unsubscribe tracking
    unsubscribe_status VARCHAR(50),
    unsubscribe_at TIMESTAMP,
    unsubscribe_error TEXT,
    
    -- Resubscribe tracking
    resubscribe_status VARCHAR(50),
    resubscribe_at TIMESTAMP,
    resubscribe_error TEXT,
    
    -- Entry channel used for resubscription
    entry_channel VARCHAR(50),
    
    -- General error tracking
    error_message TEXT,
    error_code VARCHAR(100),
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure we don't process the same subscription twice in a batch
    UNIQUE(subscription_id, process_batch_id)
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_resubscription_tracking_batch 
    ON resubscription_tracking(process_batch_id);

CREATE INDEX IF NOT EXISTS idx_resubscription_tracking_status 
    ON resubscription_tracking(resubscribe_status);

CREATE INDEX IF NOT EXISTS idx_resubscription_tracking_msisdn 
    ON resubscription_tracking(msisdn);

CREATE INDEX IF NOT EXISTS idx_resubscription_tracking_product 
    ON resubscription_tracking(product_id);

CREATE INDEX IF NOT EXISTS idx_resubscription_tracking_created 
    ON resubscription_tracking(created_at);

-- Composite index for common queries
CREATE INDEX IF NOT EXISTS idx_resubscription_tracking_msisdn_product_batch 
    ON resubscription_tracking(msisdn, product_id, process_batch_id);

-- =========================================
-- 3. Create checkpoint table for recovery
-- =========================================

CREATE TABLE IF NOT EXISTS resubscription_checkpoints (
    id SERIAL PRIMARY KEY,
    batch_id VARCHAR(100) UNIQUE NOT NULL,
    batch_type VARCHAR(50) DEFAULT 'resubscription',
    
    -- Progress tracking
    total_count INTEGER NOT NULL,
    processed_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    skipped_count INTEGER DEFAULT 0,
    
    -- Checkpoint data
    last_processed_id INTEGER,
    last_processed_msisdn VARCHAR(15),
    last_checkpoint_data JSONB,
    
    -- Status tracking
    status VARCHAR(50) DEFAULT 'pending', -- pending, in_progress, completed, failed, cancelled
    
    -- Configuration used
    config JSONB,
    
    -- Error tracking
    error_message TEXT,
    error_count INTEGER DEFAULT 0,
    
    -- Timing
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    
    -- Performance metrics
    avg_processing_time_ms NUMERIC,
    total_processing_time_sec NUMERIC
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_checkpoints_batch_id 
    ON resubscription_checkpoints(batch_id);

CREATE INDEX IF NOT EXISTS idx_checkpoints_status 
    ON resubscription_checkpoints(status);

CREATE INDEX IF NOT EXISTS idx_checkpoints_started 
    ON resubscription_checkpoints(started_at);

-- =========================================
-- 4. Create error tracking table
-- =========================================

CREATE TABLE IF NOT EXISTS resubscription_errors (
    id SERIAL PRIMARY KEY,
    batch_id VARCHAR(100),
    msisdn VARCHAR(15),
    product_id INTEGER,
    error_type VARCHAR(100),
    error_code VARCHAR(100),
    error_message TEXT,
    error_details JSONB,
    retry_count INTEGER DEFAULT 0,
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_resubscription_errors_batch 
    ON resubscription_errors(batch_id);

CREATE INDEX IF NOT EXISTS idx_resubscription_errors_msisdn 
    ON resubscription_errors(msisdn);

CREATE INDEX IF NOT EXISTS idx_resubscription_errors_type 
    ON resubscription_errors(error_type);

CREATE INDEX IF NOT EXISTS idx_resubscription_errors_unresolved 
    ON resubscription_errors(resolved) 
    WHERE resolved = FALSE;

-- =========================================
-- 5. Create processing queue table
-- =========================================

CREATE TABLE IF NOT EXISTS resubscription_queue (
    id SERIAL PRIMARY KEY,
    subscription_id INTEGER NOT NULL,
    msisdn VARCHAR(15) NOT NULL,
    product_id INTEGER NOT NULL,
    entry_channel VARCHAR(50),
    priority INTEGER DEFAULT 5, -- 1-10, 1 being highest priority
    status VARCHAR(50) DEFAULT 'pending', -- pending, processing, completed, failed
    batch_id VARCHAR(100),
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    scheduled_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processing_started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_queue_status_priority 
    ON resubscription_queue(status, priority) 
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_queue_batch 
    ON resubscription_queue(batch_id);

CREATE INDEX IF NOT EXISTS idx_queue_scheduled 
    ON resubscription_queue(scheduled_at) 
    WHERE status = 'pending';

-- =========================================
-- 6. Create statistics view
-- =========================================

CREATE OR REPLACE VIEW resubscription_statistics AS
SELECT 
    rc.batch_id,
    rc.status as batch_status,
    rc.total_count,
    rc.processed_count,
    rc.success_count,
    rc.failure_count,
    rc.skipped_count,
    CASE 
        WHEN rc.processed_count > 0 THEN 
            ROUND(rc.success_count::numeric / rc.processed_count * 100, 2)
        ELSE 0
    END as success_rate,
    CASE 
        WHEN rc.processed_count > 0 THEN 
            ROUND(rc.failure_count::numeric / rc.processed_count * 100, 2)
        ELSE 0
    END as failure_rate,
    ROUND(rc.processed_count::numeric / NULLIF(rc.total_count, 0) * 100, 2) as progress_pct,
    rc.started_at,
    rc.updated_at,
    rc.completed_at,
    EXTRACT(EPOCH FROM (COALESCE(rc.completed_at, NOW()) - rc.started_at)) as duration_seconds,
    CASE 
        WHEN rc.processed_count > 0 AND EXTRACT(EPOCH FROM (NOW() - rc.started_at)) > 0 THEN
            ROUND(rc.processed_count / EXTRACT(EPOCH FROM (NOW() - rc.started_at)), 2)
        ELSE 0
    END as rate_per_second,
    CASE 
        WHEN rc.processed_count > 0 AND rc.total_count > rc.processed_count THEN
            ROUND((rc.total_count - rc.processed_count) / 
                  NULLIF(rc.processed_count / NULLIF(EXTRACT(EPOCH FROM (NOW() - rc.started_at)), 0), 0) / 3600, 2)
        ELSE 0
    END as estimated_hours_remaining
FROM resubscription_checkpoints rc;

-- =========================================
-- 7. Create monitoring functions
-- =========================================

-- Function to get current batch progress
CREATE OR REPLACE FUNCTION get_batch_progress(p_batch_id VARCHAR)
RETURNS TABLE (
    batch_id VARCHAR,
    progress_pct NUMERIC,
    success_rate NUMERIC,
    rate_per_second NUMERIC,
    estimated_completion TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        rc.batch_id,
        ROUND(rc.processed_count::numeric / NULLIF(rc.total_count, 0) * 100, 2) as progress_pct,
        ROUND(rc.success_count::numeric / NULLIF(rc.processed_count, 0) * 100, 2) as success_rate,
        ROUND(rc.processed_count::numeric / NULLIF(EXTRACT(EPOCH FROM (NOW() - rc.started_at)), 0), 2) as rate_per_second,
        CASE 
            WHEN rc.processed_count > 0 AND rc.total_count > rc.processed_count THEN
                NOW() + INTERVAL '1 second' * ((rc.total_count - rc.processed_count) / 
                        NULLIF(rc.processed_count::numeric / NULLIF(EXTRACT(EPOCH FROM (NOW() - rc.started_at)), 0), 0))
            ELSE NOW()
        END as estimated_completion
    FROM resubscription_checkpoints rc
    WHERE rc.batch_id = p_batch_id;
END;
$$ LANGUAGE plpgsql;

-- CORRECTED Function to identify charging failures using notifications table
CREATE OR REPLACE FUNCTION identify_charging_failures_from_notifications(
    p_limit INTEGER DEFAULT NULL,
    p_offset INTEGER DEFAULT 0,
    p_days_threshold INTEGER DEFAULT 30
)
RETURNS TABLE (
    subscription_id INTEGER,
    msisdn VARCHAR,
    product_id INTEGER,
    entry_channel VARCHAR,
    subscription_date TIMESTAMP,
    last_charge_date TIMESTAMP,
    days_without_charge INTEGER,
    charging_status VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    WITH charge_data AS (
        SELECT 
            n.msisdn,
            n.product_id,
            MAX(n.created_at) as last_charge
        FROM notifications n
        WHERE n.type IN ('CHARGE', 'USER_RENEWED')
        GROUP BY n.msisdn, n.product_id
    )
    SELECT 
        s.id as subscription_id,
        s.user_identifier as msisdn,
        s.product_id,
        s.entry_channel,
        s.created_at as subscription_date,
        cd.last_charge as last_charge_date,
        CASE 
            WHEN cd.last_charge IS NULL THEN 
                EXTRACT(DAY FROM NOW() - s.created_at)::INTEGER
            ELSE 
                EXTRACT(DAY FROM NOW() - cd.last_charge)::INTEGER
        END as days_without_charge,
        CASE
            WHEN cd.last_charge IS NULL THEN 'NEVER_CHARGED'
            WHEN cd.last_charge < NOW() - INTERVAL '30 days' THEN 'STALE_CHARGE'
            ELSE 'RECENT_CHARGE'
        END as charging_status
    FROM subscriptions s
    LEFT JOIN charge_data cd ON s.user_identifier = cd.msisdn 
                              AND s.product_id = cd.product_id
    WHERE (s.status = 'active' OR s.status IS NULL)
      AND s.created_at < NOW() - INTERVAL '1 day'
      AND (cd.last_charge IS NULL 
           OR cd.last_charge < NOW() - (p_days_threshold || ' days')::INTERVAL)
    ORDER BY days_without_charge DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$ LANGUAGE plpgsql;

-- Function to get error distribution
CREATE OR REPLACE FUNCTION get_error_distribution(p_batch_id VARCHAR)
RETURNS TABLE (
    error_type VARCHAR,
    error_message TEXT,
    error_count BIGINT,
    percentage NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    WITH error_counts AS (
        SELECT 
            COALESCE(error_code, 'UNKNOWN') as error_type,
            error_message,
            COUNT(*) as error_count
        FROM resubscription_tracking
        WHERE process_batch_id = p_batch_id
        AND resubscribe_status = 'failed'
        GROUP BY error_code, error_message
    ),
    total_errors AS (
        SELECT SUM(error_count) as total
        FROM error_counts
    )
    SELECT 
        ec.error_type,
        ec.error_message,
        ec.error_count,
        ROUND(ec.error_count::numeric / NULLIF(te.total, 0) * 100, 2) as percentage
    FROM error_counts ec
    CROSS JOIN total_errors te
    ORDER BY ec.error_count DESC;
END;
$$ LANGUAGE plpgsql;

-- =========================================
-- 8. Create trigger for updated_at
-- =========================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers (using DROP IF EXISTS to avoid errors)
DROP TRIGGER IF EXISTS update_resubscription_tracking_updated_at ON resubscription_tracking;
CREATE TRIGGER update_resubscription_tracking_updated_at 
    BEFORE UPDATE ON resubscription_tracking
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_resubscription_checkpoints_updated_at ON resubscription_checkpoints;
CREATE TRIGGER update_resubscription_checkpoints_updated_at 
    BEFORE UPDATE ON resubscription_checkpoints
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_resubscription_queue_updated_at ON resubscription_queue;
CREATE TRIGGER update_resubscription_queue_updated_at 
    BEFORE UPDATE ON resubscription_queue
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =========================================
-- 9. Grant permissions
-- =========================================

-- More specific permissions (avoid using ALL when possible)
GRANT SELECT, INSERT, UPDATE, DELETE ON resubscription_tracking TO sm_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON resubscription_checkpoints TO sm_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON resubscription_errors TO sm_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON resubscription_queue TO sm_admin;
GRANT SELECT ON resubscription_statistics TO sm_admin;

-- Grant sequence permissions
GRANT USAGE, SELECT ON SEQUENCE resubscription_tracking_id_seq TO sm_admin;
GRANT USAGE, SELECT ON SEQUENCE resubscription_checkpoints_id_seq TO sm_admin;
GRANT USAGE, SELECT ON SEQUENCE resubscription_errors_id_seq TO sm_admin;
GRANT USAGE, SELECT ON SEQUENCE resubscription_queue_id_seq TO sm_admin;

-- Grant function execution
GRANT EXECUTE ON FUNCTION get_batch_progress(VARCHAR) TO sm_admin;
GRANT EXECUTE ON FUNCTION identify_charging_failures_from_notifications(INTEGER, INTEGER, INTEGER) TO sm_admin;
GRANT EXECUTE ON FUNCTION get_error_distribution(VARCHAR) TO sm_admin;
GRANT EXECUTE ON FUNCTION update_updated_at_column() TO sm_admin;

-- =========================================
-- 10. Verification and Rollback Support
-- =========================================

-- Create migration tracking table if not exists
CREATE TABLE IF NOT EXISTS migration_history (
    id SERIAL PRIMARY KEY,
    migration_name VARCHAR(255) UNIQUE NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    rollback_script TEXT
);

-- Record this migration
INSERT INTO migration_history (migration_name, rollback_script)
VALUES ('001_resubscription_tracking', '
-- Rollback script for 001_resubscription_tracking
DROP VIEW IF EXISTS resubscription_statistics CASCADE;
DROP FUNCTION IF EXISTS get_batch_progress(VARCHAR) CASCADE;
DROP FUNCTION IF EXISTS identify_charging_failures_from_notifications(INTEGER, INTEGER, INTEGER) CASCADE;
DROP FUNCTION IF EXISTS get_error_distribution(VARCHAR) CASCADE;
DROP TRIGGER IF EXISTS update_resubscription_tracking_updated_at ON resubscription_tracking;
DROP TRIGGER IF EXISTS update_resubscription_checkpoints_updated_at ON resubscription_checkpoints;
DROP TRIGGER IF EXISTS update_resubscription_queue_updated_at ON resubscription_queue;
DROP TABLE IF EXISTS resubscription_queue CASCADE;
DROP TABLE IF EXISTS resubscription_errors CASCADE;
DROP TABLE IF EXISTS resubscription_checkpoints CASCADE;
DROP TABLE IF EXISTS resubscription_tracking CASCADE;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_charging_failure_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS charging_failure_count;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS charging_failure_reason;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_resubscribe_attempt_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS resubscribe_attempt_count;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS resubscribe_status;
DELETE FROM migration_history WHERE migration_name = ''001_resubscription_tracking'';
')
ON CONFLICT (migration_name) DO NOTHING;

-- =========================================
-- 11. Final Verification
-- =========================================

DO $$
DECLARE
    v_table_count INTEGER;
    v_function_count INTEGER;
    v_index_count INTEGER;
BEGIN
    -- Count created tables
    SELECT COUNT(*) INTO v_table_count
    FROM information_schema.tables 
    WHERE table_schema = 'public' 
    AND table_name IN (
        'resubscription_tracking',
        'resubscription_checkpoints',
        'resubscription_errors',
        'resubscription_queue'
    );
    
    -- Count created functions
    SELECT COUNT(*) INTO v_function_count
    FROM information_schema.routines
    WHERE routine_schema = 'public'
    AND routine_name IN (
        'get_batch_progress',
        'identify_charging_failures_from_notifications',
        'get_error_distribution'
    );
    
    -- Count created indexes
    SELECT COUNT(*) INTO v_index_count
    FROM pg_indexes
    WHERE schemaname = 'public'
    AND tablename LIKE 'resubscription%';
    
    RAISE NOTICE '===========================================';
    RAISE NOTICE 'Migration 001_resubscription_tracking completed';
    RAISE NOTICE '===========================================';
    RAISE NOTICE 'Tables created: %', v_table_count;
    RAISE NOTICE 'Functions created: %', v_function_count;
    RAISE NOTICE 'Indexes created: %', v_index_count;
    RAISE NOTICE '';
    RAISE NOTICE 'Tables:';
    RAISE NOTICE '  ✓ resubscription_tracking';
    RAISE NOTICE '  ✓ resubscription_checkpoints';
    RAISE NOTICE '  ✓ resubscription_errors';
    RAISE NOTICE '  ✓ resubscription_queue';
    RAISE NOTICE '';
    RAISE NOTICE 'Views:';
    RAISE NOTICE '  ✓ resubscription_statistics';
    RAISE NOTICE '';
    RAISE NOTICE 'Functions:';
    RAISE NOTICE '  ✓ get_batch_progress()';
    RAISE NOTICE '  ✓ identify_charging_failures_from_notifications()';
    RAISE NOTICE '  ✓ get_error_distribution()';
    RAISE NOTICE '';
    
    IF v_table_count < 4 THEN
        RAISE EXCEPTION 'Not all tables were created successfully';
    END IF;
    
    IF v_function_count < 3 THEN
        RAISE WARNING 'Not all functions were created successfully';
    END IF;
END $$;
