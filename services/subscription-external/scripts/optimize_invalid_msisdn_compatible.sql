-- =====================================================
-- COMPATIBLE INVALID MSISDN LOGS TABLE OPTIMIZATION SCRIPT
-- =====================================================
-- Purpose: Highly compatible optimization for invalid_msisdn_logs table
-- Target: Works across different PostgreSQL versions
-- Note: Archival removed - old records are needed for MSISDN generation
-- Author: System Optimization Team
-- Date: 2025-01-27
-- =====================================================

-- Display current status
\echo 'Starting compatible optimization of invalid_msisdn_logs table...'

-- =====================================================
-- PHASE 1: CREATE ESSENTIAL INDEXES
-- =====================================================

\echo 'Creating indexes for invalid_msisdn_logs table...'

-- Primary search index for MSISDN lookups (most important)
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_msisdn 
ON invalid_msisdn_logs(msisdn);

\echo 'Created index: idx_invalid_msisdn_logs_msisdn'

-- Composite index for time-based queries
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_created_at 
ON invalid_msisdn_logs(created_at DESC);

\echo 'Created index: idx_invalid_msisdn_logs_created_at'

-- Composite index for MSISDN + created_at (for recent lookups)
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_msisdn_created 
ON invalid_msisdn_logs(msisdn, created_at DESC);

\echo 'Created index: idx_invalid_msisdn_logs_msisdn_created'

-- Index for product-based queries
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_product 
ON invalid_msisdn_logs(product_id, created_at DESC);

\echo 'Created index: idx_invalid_msisdn_logs_product'

-- Index for response code filtering
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_response_code 
ON invalid_msisdn_logs(response_code, created_at DESC);

\echo 'Created index: idx_invalid_msisdn_logs_response_code'

-- =====================================================
-- PHASE 2: CREATE MONITORING VIEWS (COMPATIBLE)
-- =====================================================

\echo 'Creating compatible monitoring views...'

-- Create comprehensive performance monitoring view
CREATE OR REPLACE VIEW invalid_msisdn_performance AS
SELECT 
    'Table Size' as metric,
    pg_size_pretty(pg_total_relation_size('invalid_msisdn_logs')) as value
UNION ALL
SELECT 
    'Index Size' as metric,
    pg_size_pretty(pg_indexes_size('invalid_msisdn_logs')) as value
UNION ALL
SELECT 
    'Row Count' as metric,
    COUNT(*)::text as value
FROM invalid_msisdn_logs
UNION ALL
SELECT 
    'Unique MSISDNs' as metric,
    COUNT(DISTINCT msisdn)::text as value
FROM invalid_msisdn_logs
UNION ALL
SELECT 
    'Oldest Record' as metric,
    COALESCE(MIN(created_at)::text, 'No records') as value
FROM invalid_msisdn_logs
UNION ALL
SELECT 
    'Newest Record' as metric,
    COALESCE(MAX(created_at)::text, 'No records') as value
FROM invalid_msisdn_logs
UNION ALL
SELECT 
    'Records Today' as metric,
    COUNT(*)::text as value
FROM invalid_msisdn_logs
WHERE created_at >= CURRENT_DATE
UNION ALL
SELECT 
    'Records This Week' as metric,
    COUNT(*)::text as value
FROM invalid_msisdn_logs
WHERE created_at >= CURRENT_DATE - INTERVAL '7 days';

-- Create compatible index usage monitoring view
CREATE OR REPLACE VIEW invalid_msisdn_index_usage AS
SELECT 
    COALESCE(schemaname, 'public') as schemaname,
    COALESCE(relname, 'unknown') as tablename,
    COALESCE(indexrelname, 'unknown') as indexname,
    COALESCE(idx_scan, 0) as idx_scan,
    COALESCE(idx_tup_read, 0) as idx_tup_read,
    COALESCE(idx_tup_fetch, 0) as idx_tup_fetch,
    COALESCE(pg_size_pretty(pg_relation_size(indexrelid)), '0 bytes') as index_size
FROM pg_stat_user_indexes
WHERE relname = 'invalid_msisdn_logs'
UNION ALL
SELECT 
    'Info' as schemaname,
    'Note' as tablename,
    'Check pg_stat_user_indexes availability' as indexname,
    0 as idx_scan,
    0 as idx_tup_read,
    0 as idx_tup_fetch,
    '0 bytes' as index_size
WHERE NOT EXISTS (
    SELECT 1 FROM pg_stat_user_indexes WHERE relname = 'invalid_msisdn_logs'
)
ORDER BY idx_scan DESC;

-- Create simple index list view as fallback
CREATE OR REPLACE VIEW invalid_msisdn_indexes AS
SELECT 
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes 
WHERE tablename = 'invalid_msisdn_logs'
ORDER BY indexname;

-- =====================================================
-- PHASE 3: CREATE MAINTENANCE FUNCTIONS
-- =====================================================

\echo 'Creating maintenance functions...'

-- Function to analyze and vacuum table
CREATE OR REPLACE FUNCTION maintain_invalid_msisdn_logs() 
RETURNS TEXT AS $$
DECLARE
    start_time TIMESTAMP;
    end_time TIMESTAMP;
    result TEXT;
    row_count BIGINT;
BEGIN
    start_time := NOW();
    
    -- Get row count
    SELECT COUNT(*) INTO row_count FROM invalid_msisdn_logs;
    
    -- Analyze table to update statistics
    ANALYZE invalid_msisdn_logs;
    
    -- Vacuum to reclaim space (if needed)
    VACUUM ANALYZE invalid_msisdn_logs;
    
    end_time := NOW();
    result := 'Maintenance completed in ' || EXTRACT(EPOCH FROM (end_time - start_time)) || ' seconds. Processed ' || row_count || ' records.';
    
    RAISE NOTICE '%', result;
    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Function to get table information (compatible)
CREATE OR REPLACE FUNCTION get_table_info() 
RETURNS TABLE(
    table_name TEXT,
    total_size TEXT,
    table_size TEXT,
    index_size TEXT,
    row_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        'invalid_msisdn_logs'::TEXT as table_name,
        pg_size_pretty(pg_total_relation_size('invalid_msisdn_logs')) as total_size,
        pg_size_pretty(pg_relation_size('invalid_msisdn_logs')) as table_size,
        pg_size_pretty(pg_indexes_size('invalid_msisdn_logs')) as index_size,
        (SELECT COUNT(*) FROM invalid_msisdn_logs) as row_count;
END;
$$ LANGUAGE plpgsql;

-- Function to get comprehensive table statistics
CREATE OR REPLACE FUNCTION get_invalid_msisdn_stats() 
RETURNS TABLE(
    metric TEXT,
    value TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT * FROM invalid_msisdn_performance;
EXCEPTION
    WHEN OTHERS THEN
        -- Fallback if view has issues
        RETURN QUERY
        SELECT 
            'Error' as metric,
            'Unable to get performance stats: ' || SQLERRM as value;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- PHASE 4: CREATE PERFORMANCE TESTING FUNCTIONS
-- =====================================================

\echo 'Creating performance testing functions...'

-- Function to test query performance (compatible)
CREATE OR REPLACE FUNCTION test_query_performance() 
RETURNS TABLE(
    test_name TEXT,
    execution_time_ms NUMERIC,
    rows_returned BIGINT
) AS $$
DECLARE
    start_time TIMESTAMP;
    end_time TIMESTAMP;
    test_msisdns TEXT[] := ARRAY['233123456789', '233987654321', '233111111111', '233222222222', '233333333333'];
    row_count BIGINT;
BEGIN
    -- Test 1: Simple MSISDN lookup
    start_time := NOW();
    SELECT COUNT(*) INTO row_count FROM invalid_msisdn_logs WHERE msisdn = ANY(test_msisdns);
    end_time := NOW();
    
    RETURN QUERY SELECT 
        'Simple MSISDN lookup'::TEXT,
        EXTRACT(EPOCH FROM (end_time - start_time)) * 1000,
        row_count;
    
    -- Test 2: Time-based lookup (with error handling)
    BEGIN
        start_time := NOW();
        SELECT COUNT(*) INTO row_count FROM invalid_msisdn_logs WHERE created_at >= NOW() - INTERVAL '30 days';
        end_time := NOW();
        
        RETURN QUERY SELECT 
            'Time-based lookup (30 days)'::TEXT,
            EXTRACT(EPOCH FROM (end_time - start_time)) * 1000,
            row_count;
    EXCEPTION
        WHEN OTHERS THEN
            RETURN QUERY SELECT 
                'Time-based lookup (30 days)'::TEXT,
                0::NUMERIC,
                0::BIGINT;
    END;
    
    -- Test 3: Complex query with filters
    BEGIN
        start_time := NOW();
        SELECT COUNT(*) INTO row_count FROM invalid_msisdn_logs WHERE msisdn LIKE '233%' AND created_at >= NOW() - INTERVAL '7 days';
        end_time := NOW();
        
        RETURN QUERY SELECT 
            'Complex query (7 days, prefix filter)'::TEXT,
            EXTRACT(EPOCH FROM (end_time - start_time)) * 1000,
            row_count;
    EXCEPTION
        WHEN OTHERS THEN
            RETURN QUERY SELECT 
                'Complex query (7 days, prefix filter)'::TEXT,
                0::NUMERIC,
                0::BIGINT;
    END;
    
    -- Test 4: DISTINCT MSISDN lookup (common operation)
    BEGIN
        start_time := NOW();
        SELECT COUNT(*) INTO row_count FROM (SELECT DISTINCT msisdn FROM invalid_msisdn_logs WHERE msisdn LIKE '233%') t;
        end_time := NOW();
        
        RETURN QUERY SELECT 
            'DISTINCT MSISDN lookup'::TEXT,
            EXTRACT(EPOCH FROM (end_time - start_time)) * 1000,
            row_count;
    EXCEPTION
        WHEN OTHERS THEN
            RETURN QUERY SELECT 
                'DISTINCT MSISDN lookup'::TEXT,
                0::NUMERIC,
                0::BIGINT;
    END;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- PHASE 5: FINAL OPTIMIZATION STEPS
-- =====================================================

\echo 'Analyzing table to update statistics...'

-- Analyze table to update statistics
ANALYZE invalid_msisdn_logs;

-- Display summary
\echo '===================================================='
\echo 'INVALID MSISDN LOGS OPTIMIZATION COMPLETE'
\echo '===================================================='

-- Show current table status (compatible)
SELECT 
    'Current table size: ' || pg_size_pretty(pg_total_relation_size('invalid_msisdn_logs')) as status
UNION ALL
SELECT 
    'Current row count: ' || COUNT(*)::text as status
FROM invalid_msisdn_logs
UNION ALL
SELECT 
    'Indexes created: ' || COUNT(*)::text as status
FROM pg_indexes 
WHERE tablename = 'invalid_msisdn_logs';

\echo '===================================================='
\echo 'Available monitoring commands:'
\echo '  SELECT * FROM invalid_msisdn_performance;'
\echo '  SELECT * FROM invalid_msisdn_index_usage;'
\echo '  SELECT * FROM invalid_msisdn_indexes;'
\echo '  SELECT get_table_info();'
\echo '  SELECT test_query_performance();'
\echo '===================================================='
\echo 'Maintenance commands:'
\echo '  SELECT maintain_invalid_msisdn_logs();'
\echo '  SELECT get_invalid_msisdn_stats();'
\echo '===================================================='
\echo 'Note: Old records are preserved for MSISDN generation'
\echo 'Optimization completed successfully!'
\echo '====================================================' 