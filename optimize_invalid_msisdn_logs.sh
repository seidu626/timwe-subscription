#!/bin/bash
# Invalid MSISDN Logs Optimization Script
# Purpose: Optimize invalid_msisdn_logs table for efficient searching
# Usage: ./optimize_invalid_msisdn_logs.sh [database_name] [action]

set -e

# Configuration
DB_NAME=${1:-"subscription_db"}
ACTION=${2:-"all"}  # Options: indexes, cache, archive, partition, all
DB_USER=${DB_USER:-"postgres"}
DB_HOST=${DB_HOST:-"localhost"}
DB_PORT=${DB_PORT:-"5432"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to execute SQL
execute_sql() {
    local sql="$1"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$sql"
}

# Function to create indexes
create_indexes() {
    log "Creating indexes for invalid_msisdn_logs table..."
    
    # Check if indexes already exist
    local index_exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc \
        "SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_msisdn';")
    
    if [ "$index_exists" -eq "0" ]; then
        log "Creating index on msisdn column..."
        execute_sql "CREATE INDEX CONCURRENTLY idx_invalid_msisdn_logs_msisdn ON invalid_msisdn_logs(msisdn);"
    else
        warning "Index idx_invalid_msisdn_logs_msisdn already exists"
    fi
    
    # Create index on created_at
    index_exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc \
        "SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_created_at';")
    
    if [ "$index_exists" -eq "0" ]; then
        log "Creating index on created_at column..."
        execute_sql "CREATE INDEX CONCURRENTLY idx_invalid_msisdn_logs_created_at ON invalid_msisdn_logs(created_at DESC);"
    else
        warning "Index idx_invalid_msisdn_logs_created_at already exists"
    fi
    
    # Create composite index
    index_exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc \
        "SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs' AND indexname = 'idx_invalid_msisdn_logs_msisdn_created';")
    
    if [ "$index_exists" -eq "0" ]; then
        log "Creating composite index on msisdn and created_at..."
        execute_sql "CREATE INDEX CONCURRENTLY idx_invalid_msisdn_logs_msisdn_created ON invalid_msisdn_logs(msisdn, created_at DESC);"
    else
        warning "Index idx_invalid_msisdn_logs_msisdn_created already exists"
    fi
    
    # Analyze table
    log "Analyzing table to update statistics..."
    execute_sql "ANALYZE invalid_msisdn_logs;"
    
    log "Indexes created successfully!"
}

# Function to create archive table and procedure
create_archive_system() {
    log "Setting up archiving system..."
    
    # Create archive table
    log "Creating archive table..."
    execute_sql "
    CREATE TABLE IF NOT EXISTS invalid_msisdn_logs_archive (
        LIKE invalid_msisdn_logs INCLUDING ALL
    );"
    
    # Create archive function
    log "Creating archive function..."
    execute_sql "
    CREATE OR REPLACE FUNCTION archive_old_invalid_msisdns() 
    RETURNS void AS \$\$
    DECLARE
        archived_count INTEGER;
    BEGIN
        -- Move records older than 3 months to archive
        WITH moved AS (
            INSERT INTO invalid_msisdn_logs_archive
            SELECT * FROM invalid_msisdn_logs
            WHERE created_at < NOW() - INTERVAL '3 months'
            RETURNING 1
        )
        SELECT COUNT(*) INTO archived_count FROM moved;
        
        -- Delete archived records from main table
        DELETE FROM invalid_msisdn_logs
        WHERE created_at < NOW() - INTERVAL '3 months';
        
        RAISE NOTICE 'Archived % records', archived_count;
    END;
    \$\$ LANGUAGE plpgsql;"
    
    log "Archive system created successfully!"
}

# Function to create monitoring views
create_monitoring_views() {
    log "Creating monitoring views..."
    
    # Create performance monitoring view
    execute_sql "
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
        'Oldest Record' as metric,
        MIN(created_at)::text as value
    FROM invalid_msisdn_logs;"
    
    log "Monitoring views created successfully!"
}

# Function to show current status
show_status() {
    log "Current table status:"
    execute_sql "
    SELECT 
        COUNT(*) as total_records,
        pg_size_pretty(pg_total_relation_size('invalid_msisdn_logs')) as total_size,
        MIN(created_at) as oldest_record,
        MAX(created_at) as newest_record
    FROM invalid_msisdn_logs;"
    
    log "Existing indexes:"
    execute_sql "
    SELECT indexname, indexdef 
    FROM pg_indexes 
    WHERE tablename = 'invalid_msisdn_logs';"
}

# Main execution
main() {
    log "Starting optimization for invalid_msisdn_logs table"
    log "Database: $DB_NAME, Action: $ACTION"
    
    # Show current status
    show_status
    
    case "$ACTION" in
        indexes)
            create_indexes
            ;;
        archive)
            create_archive_system
            ;;
        monitor)
            create_monitoring_views
            ;;
        status)
            # Status already shown
            ;;
        all)
            create_indexes
            create_archive_system
            create_monitoring_views
            ;;
        *)
            error "Invalid action. Use: indexes, archive, monitor, status, or all"
            ;;
    esac
    
    log "Optimization complete!"
}

# Run main function
main
