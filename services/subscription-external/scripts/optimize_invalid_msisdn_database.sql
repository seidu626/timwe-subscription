-- =====================================================
-- INVALID MSISDN LOGS TABLE OPTIMIZATION SCRIPT
-- =====================================================
-- Purpose: Optimize invalid_msisdn_logs table for efficient searching
-- Target: Handle millions of records efficiently
-- Note: Archival removed - old records are needed for MSISDN generation
-- Author: System Optimization Team
-- Date: 2025-01-27
-- =====================================================

-- =====================================================
-- PHASE 1: CREATE ESSENTIAL INDEXES (IMMEDIATE IMPACT)
-- =====================================================

-- Note: CREATE INDEX CONCURRENTLY cannot run inside a transaction block
-- So we create indexes separately with proper error handling

-- Primary search index for MSISDN lookups (most important)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_msisdn') THEN
        RAISE NOTICE 'Creating index: idx_invalid_msisdn_logs_msisdn';
    ELSE
        RAISE NOTICE 'Index idx_invalid_msisdn_logs_msisdn already exists';
    END IF;
END $$;

-- Create the index only if it doesn't exist
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_msisdn 
ON invalid_msisdn_logs(msisdn);

-- Composite index for time-based queries
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_created_at') THEN
        RAISE NOTICE 'Creating index: idx_invalid_msisdn_logs_created_at';
    ELSE
        RAISE NOTICE 'Index idx_invalid_msisdn_logs_created_at already exists';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_created_at 
ON invalid_msisdn_logs(created_at DESC);

-- Composite index for MSISDN + created_at (for recent lookups)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_msisdn_created') THEN
        RAISE NOTICE 'Creating index: idx_invalid_msisdn_logs_msisdn_created';
    ELSE
        RAISE NOTICE 'Index idx_invalid_msisdn_logs_msisdn_created already exists';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_msisdn_created 
ON invalid_msisdn_logs(msisdn, created_at DESC);

-- Index for product-based queries (if needed)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_product') THEN
        RAISE NOTICE 'Creating index: idx_invalid_msisdn_logs_product';
    ELSE
        RAISE NOTICE 'Index idx_invalid_msisdn_logs_product already exists';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_product 
ON invalid_msisdn_logs(product_id, created_at DESC);

-- Index for response code filtering
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_response_code') THEN
        RAISE NOTICE 'Creating index: idx_invalid_msisdn_logs_response_code';
    ELSE
        RAISE NOTICE 'Index idx_invalid_msisdn_logs_response_code already exists';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_response_code 
ON invalid_msisdn_logs(response_code, created_at DESC);

-- =====================================================
-- PHASE 2: CREATE MONITORING AND PERFORMANCE VIEWS
-- =====================================================

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
    MIN(created_at)::text as value
FROM invalid_msisdn_logs
UNION ALL
SELECT 
    'Newest Record' as metric,
    MAX(created_at)::text as value
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

-- Create index usage monitoring view
CREATE OR REPLACE VIEW invalid_msisdn_index_usage AS
SELECT 
    schemaname,
    relname as tablename,
    indexrelname as indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size
FROM pg_stat_user_indexes
WHERE relname = 'invalid_msisdn_logs'
ORDER BY idx_scan DESC;

-- Create query performance monitoring view (only if pg_stat_statements is available)
CREATE OR REPLACE VIEW invalid_msisdn_query_stats AS
SELECT 
    COALESCE(query, 'pg_stat_statements not available') as query,
    COALESCE(calls::text, '0') as calls,
    COALESCE(total_exec_time::text, '0') as total_exec_time,
    COALESCE(mean_exec_time::text, '0') as mean_exec_time,
    COALESCE(max_exec_time::text, '0') as max_exec_time,
    COALESCE(rows::text, '0') as rows,
    COALESCE(shared_blks_hit::text, '0') as shared_blks_hit,
    COALESCE(shared_blks_read::text, '0') as shared_blks_read
FROM (
    SELECT 
        query,
        calls,
        total_exec_time,
        mean_exec_time,
        max_exec_time,
        rows,
        shared_blks_hit,
        shared_blks_read
    FROM pg_stat_statements
    WHERE query LIKE '%invalid_msisdn_logs%'
    ORDER BY total_exec_time DESC
    LIMIT 10
) AS stats
UNION ALL
SELECT 
    'Note: Enable pg_stat_statements extension for detailed query statistics' as query,
    '' as calls,
    '' as total_exec_time,
    '' as mean_exec_time,
    '' as max_exec_time,
    '' as rows,
    '' as shared_blks_hit,
    '' as shared_blks_read
WHERE NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements');

-- =====================================================
-- PHASE 3: CREATE PARTITIONING SYSTEM (LONG-TERM SCALABILITY)
-- =====================================================

-- Create partitioned table structure (for future migration)
-- Note: This keeps all data accessible but organized by time for better performance
CREATE TABLE IF NOT EXISTS invalid_msisdn_logs_partitioned (
    id BIGSERIAL,
    msisdn VARCHAR(15) NOT NULL,
    product_id INTEGER,
    pricepoint_id INTEGER,
    partner_role_id INTEGER,
    entry_channel VARCHAR(50),
    request_id VARCHAR(100),
    response_code VARCHAR(50),
    response_message TEXT,
    subscription_result VARCHAR(100),
    subscription_error TEXT,
    external_tx_id VARCHAR(255),
    transaction_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create partitions for current and future months
DO $$
DECLARE
    partition_date DATE;
    partition_name TEXT;
    start_date TEXT;
    end_date TEXT;
BEGIN
    -- Create partitions for the next 12 months
    FOR i IN 0..11 LOOP
        partition_date := CURRENT_DATE + (i || ' months')::INTERVAL;
        partition_name := 'invalid_msisdn_logs_' || 
                         EXTRACT(YEAR FROM partition_date)::TEXT || '_' ||
                         LPAD(EXTRACT(MONTH FROM partition_date)::TEXT, 2, '0');
        
        start_date := EXTRACT(YEAR FROM partition_date)::TEXT || '-' ||
                     LPAD(EXTRACT(MONTH FROM partition_date)::TEXT, 2, '0') || '-01';
        
        end_date := EXTRACT(YEAR FROM partition_date)::TEXT || '-' ||
                   LPAD(EXTRACT(MONTH FROM partition_date)::TEXT, 2, '0') || '-01' ||
                   ' + 1 month';
        
        -- Only create if partition doesn't exist
        IF NOT EXISTS (
            SELECT 1 FROM pg_class c 
            JOIN pg_namespace n ON n.oid = c.relnamespace 
            WHERE c.relname = partition_name AND n.nspname = 'public'
        ) THEN
            EXECUTE format('CREATE TABLE %I PARTITION OF invalid_msisdn_logs_partitioned
                          FOR VALUES FROM (%L) TO (%L)', 
                          partition_name, start_date, end_date);
            RAISE NOTICE 'Created partition: %', partition_name;
        ELSE
            RAISE NOTICE 'Partition % already exists', partition_name;
        END IF;
    END LOOP;
END $$;

-- Create indexes on partitioned table
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_part_msisdn 
ON invalid_msisdn_logs_partitioned(msisdn);

CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_part_created 
ON invalid_msisdn_logs_partitioned(created_at DESC);

-- =====================================================
-- PHASE 4: CREATE MAINTENANCE FUNCTIONS
-- =====================================================

-- Function to analyze and vacuum table
CREATE OR REPLACE FUNCTION maintain_invalid_msisdn_logs() 
RETURNS TEXT AS $$
DECLARE
    start_time TIMESTAMP;
    end_time TIMESTAMP;
    result TEXT;
BEGIN
    start_time := NOW();
    
    -- Analyze table to update statistics
    ANALYZE invalid_msisdn_logs;
    
    -- Vacuum to reclaim space (if needed)
    VACUUM ANALYZE invalid_msisdn_logs;
    
    end_time := NOW();
    result := 'Maintenance completed in ' || EXTRACT(EPOCH FROM (end_time - start_time)) || ' seconds';
    
    RAISE NOTICE '%', result;
    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Function to get table bloat information
CREATE OR REPLACE FUNCTION get_table_bloat_info() 
RETURNS TABLE(
    table_name TEXT,
    total_size TEXT,
    table_size TEXT,
    index_size TEXT,
    bloat_ratio NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        'invalid_msisdn_logs'::TEXT as table_name,
        pg_size_pretty(pg_total_relation_size('invalid_msisdn_logs')) as total_size,
        pg_size_pretty(pg_relation_size('invalid_msisdn_logs')) as table_size,
        pg_size_pretty(pg_indexes_size('invalid_msisdn_logs')) as index_size,
        ROUND(
            (pg_total_relation_size('invalid_msisdn_logs') - pg_relation_size('invalid_msisdn_logs'))::NUMERIC / 
            pg_total_relation_size('invalid_msisdn_logs') * 100, 2
        ) as bloat_ratio;
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
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- PHASE 5: CREATE PERFORMANCE TESTING FUNCTIONS
-- =====================================================

-- Function to test query performance
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
    
    -- Test 2: Time-based lookup
    start_time := NOW();
    SELECT COUNT(*) INTO row_count FROM invalid_msisdn_logs WHERE created_at >= NOW() - INTERVAL '30 days';
    end_time := NOW();
    
    RETURN QUERY SELECT 
        'Time-based lookup (30 days)'::TEXT,
        EXTRACT(EPOCH FROM (end_time - start_time)) * 1000,
        row_count;
    
    -- Test 3: Complex query with filters
    start_time := NOW();
    SELECT COUNT(*) INTO row_count FROM invalid_msisdn_logs WHERE msisdn LIKE '233%' AND created_at >= NOW() - INTERVAL '7 days';
    end_time := NOW();
    
    RETURN QUERY SELECT 
        'Complex query (7 days, prefix filter)'::TEXT,
        EXTRACT(EPOCH FROM (end_time - start_time)) * 1000,
        row_count;
    
    -- Test 4: DISTINCT MSISDN lookup (common operation)
    start_time := NOW();
    SELECT COUNT(*) INTO row_count FROM (SELECT DISTINCT msisdn FROM invalid_msisdn_logs WHERE msisdn LIKE '233%') t;
    end_time := NOW();
    
    RETURN QUERY SELECT 
        'DISTINCT MSISDN lookup'::TEXT,
        EXTRACT(EPOCH FROM (end_time - start_time)) * 1000,
        row_count;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- PHASE 6: FINAL OPTIMIZATION STEPS
-- =====================================================

-- Analyze all tables to update statistics
ANALYZE invalid_msisdn_logs;
ANALYZE invalid_msisdn_logs_partitioned;

-- Create a summary report
DO $$
DECLARE
    table_size TEXT;
    row_count BIGINT;
    index_count INTEGER;
BEGIN
    -- Get current table information
    SELECT pg_size_pretty(pg_total_relation_size('invalid_msisdn_logs')) INTO table_size;
    SELECT COUNT(*) INTO row_count FROM invalid_msisdn_logs;
    SELECT COUNT(*) INTO index_count FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs';
    
    RAISE NOTICE '====================================================';
    RAISE NOTICE 'INVALID MSISDN LOGS OPTIMIZATION COMPLETE';
    RAISE NOTICE '====================================================';
    RAISE NOTICE 'Current table size: %', table_size;
    RAISE NOTICE 'Current row count: %', row_count;
    RAISE NOTICE 'Indexes created: %', index_count;
    RAISE NOTICE '====================================================';
    RAISE NOTICE 'Next steps:';
    RAISE NOTICE '1. Monitor query performance using the created views';
    RAISE NOTICE '2. Use maintain_invalid_msisdn_logs() for regular maintenance';
    RAISE NOTICE '3. Consider migrating to partitioned table when ready';
    RAISE NOTICE '4. Monitor Bloom Filter performance in Go application';
    RAISE NOTICE '5. Old records are preserved for MSISDN generation';
    RAISE NOTICE '====================================================';
END $$; 