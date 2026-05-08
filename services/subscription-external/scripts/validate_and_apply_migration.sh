#!/bin/bash
# Comprehensive Database Migration Validation and Application
# File: scripts/validate_and_apply_migration.sh

set -e

# Configuration
DB_HOST="${DB_HOST:-139.59.135.253}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
MIGRATION_FILE="../migrations/001_resubscription_tracking.sql"
BACKUP_DIR="/tmp/migration_backups"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Log file
LOG_FILE="/tmp/migration_validation_$(date +%Y%m%d_%H%M%S).log"

# Function to log messages
log_message() {
    local level=$1
    local message=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        "INFO") echo -e "${BLUE}[$timestamp] INFO: $message${NC}" | tee -a "$LOG_FILE" ;;
        "SUCCESS") echo -e "${GREEN}[$timestamp] SUCCESS: $message${NC}" | tee -a "$LOG_FILE" ;;
        "WARNING") echo -e "${YELLOW}[$timestamp] WARNING: $message${NC}" | tee -a "$LOG_FILE" ;;
        "ERROR") echo -e "${RED}[$timestamp] ERROR: $message${NC}" | tee -a "$LOG_FILE" ;;
        "CRITICAL") echo -e "${PURPLE}[$timestamp] CRITICAL: $message${NC}" | tee -a "$LOG_FILE" ;;
    esac
}

# Function to check prerequisites
check_prerequisites() {
    log_message "INFO" "Checking prerequisites..."
    
    # Check if psql is available
    if ! command -v psql &> /dev/null; then
        log_message "ERROR" "PostgreSQL client (psql) is not installed"
        exit 1
    fi
    
    # Check if pg_dump is available
    if ! command -v pg_dump &> /dev/null; then
        log_message "ERROR" "PostgreSQL dump utility (pg_dump) is not installed"
        exit 1
    fi
    
    # Check if migration file exists
    if [ ! -f "$MIGRATION_FILE" ]; then
        log_message "ERROR" "Migration file not found: $MIGRATION_FILE"
        exit 1
    fi
    
    # Create backup directory
    mkdir -p "$BACKUP_DIR"
    
    log_message "SUCCESS" "Prerequisites check passed"
}

# Function to check database connectivity
check_database_connectivity() {
    log_message "INFO" "Checking database connectivity..."
    
    if pg_isready -h "$DB_HOST" -p "$DB_PORT" > /dev/null 2>&1; then
        log_message "SUCCESS" "Database is accessible at $DB_HOST:$DB_PORT"
    else
        log_message "ERROR" "Cannot connect to database at $DB_HOST:$DB_PORT"
        exit 1
    fi
    
    # Test actual connection with credentials
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
        log_message "SUCCESS" "Database authentication successful"
    else
        log_message "ERROR" "Database authentication failed for user $DB_USER"
        exit 1
    fi
}

# Function to analyze current database state
analyze_database_state() {
    log_message "INFO" "Analyzing current database state..."
    
    # Check existing tables
    local existing_tables=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name IN ('subscriptions', 'invalid_msisdn_logs', 'resubscription_tracking', 'resubscription_checkpoints')
        ORDER BY table_name;
    " | tr -d ' ')
    
    log_message "INFO" "Existing tables: $existing_tables"
    
    # Check subscriptions table structure
    local subscription_columns=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT column_name 
        FROM information_schema.columns 
        WHERE table_name = 'subscriptions' 
        AND column_name IN ('charging_failure_count', 'last_charging_failure_at', 'resubscribe_status')
        ORDER BY column_name;
    " | tr -d ' ')
    
    log_message "INFO" "Subscription table charging-related columns: $subscription_columns"
    
    # Check if migration is needed
    local migration_needed=false
    
    if [[ ! "$existing_tables" =~ "resubscription_tracking" ]]; then
        migration_needed=true
        log_message "WARNING" "resubscription_tracking table missing"
    fi
    
    if [[ ! "$existing_tables" =~ "resubscription_checkpoints" ]]; then
        migration_needed=true
        log_message "WARNING" "resubscription_checkpoints table missing"
    fi
    
    if [[ ! "$subscription_columns" =~ "charging_failure_count" ]]; then
        migration_needed=true
        log_message "WARNING" "charging_failure_count column missing from subscriptions table"
    fi
    
    if [ "$migration_needed" = true ]; then
        log_message "CRITICAL" "Database migration is required"
        return 1
    else
        log_message "SUCCESS" "Database appears to be up to date"
        return 0
    fi
}

# Function to create comprehensive backup
create_comprehensive_backup() {
    local backup_file="$BACKUP_DIR/comprehensive_backup_$(date +%Y%m%d_%H%M%S).sql"
    
    log_message "INFO" "Creating comprehensive database backup..."
    
    # Backup critical tables
    if pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        -t subscriptions -t invalid_msisdn_logs -t products -t userbase \
        -t resubscription_tracking -t resubscription_checkpoints \
        --no-owner --no-privileges \
        -f "$backup_file"; then
        log_message "SUCCESS" "Backup created: $backup_file"
        
        # Create backup info file
        cat > "$backup_file.info" << EOF
Backup Information:
- Created: $(date)
- Database: $DB_NAME@$DB_HOST:$DB_PORT
- User: $DB_USER
- Tables: subscriptions, invalid_msisdn_logs, products, userbase, resubscription_tracking, resubscription_checkpoints
- Purpose: Pre-migration backup for charging failure resubscription implementation
- Rollback: Use this file to restore if migration fails
EOF
        log_message "SUCCESS" "Backup info file created: $backup_file.info"
    else
        log_message "ERROR" "Backup creation failed"
        exit 1
    fi
}

# Function to apply migration
apply_migration() {
    log_message "INFO" "Applying database migration..."
    
    # Apply the migration file
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        -f "$MIGRATION_FILE" > /tmp/migration_output.log 2>&1; then
        log_message "SUCCESS" "Migration applied successfully"
        
        # Show migration output for verification
        log_message "INFO" "Migration output:"
        cat /tmp/migration_output.log | tee -a "$LOG_FILE"
    else
        log_message "ERROR" "Migration failed"
        log_message "ERROR" "Migration output:"
        cat /tmp/migration_output.log | tee -a "$LOG_FILE"
        return 1
    fi
}

# Function to validate migration
validate_migration() {
    log_message "INFO" "Validating migration results..."
    
    local validation_queries=(
        "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('resubscription_tracking', 'resubscription_checkpoints');"
        "SELECT COUNT(*) as column_count FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name IN ('charging_failure_count', 'last_charging_failure_at', 'resubscribe_status');"
        "SELECT COUNT(*) as index_count FROM pg_indexes WHERE tablename IN ('resubscription_tracking', 'resubscription_checkpoints');"
        "SELECT COUNT(*) as function_count FROM information_schema.routines WHERE routine_schema = 'public' AND routine_name LIKE '%resubscription%';"
    )
    
    local query_names=(
        "Required tables"
        "Subscription table columns"
        "Required indexes"
        "Required functions"
    )
    
    for i in "${!validation_queries[@]}"; do
        local result=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "${validation_queries[$i]}" | tr -d ' ')
        local expected=""
        
        case $i in
            0) expected="2" ;; # 2 tables
            1) expected="3" ;; # 3 columns
            2) expected="4" ;; # 4 indexes
            3) expected="3" ;; # 3 functions
        esac
        
        if [ "$result" = "$expected" ]; then
            log_message "SUCCESS" "${query_names[$i]}: $result/$expected ✅"
        else
            log_message "WARNING" "${query_names[$i]}: $result/$expected ⚠️"
        fi
    done
}

# Function to test migration with sample data
test_migration_with_sample_data() {
    log_message "INFO" "Testing migration with sample data..."
    
    # Insert test checkpoint
    local test_batch_id="test_$(date +%s)"
    local insert_query="
        INSERT INTO resubscription_checkpoints 
        (batch_id, total_count, status, started_at) 
        VALUES ('$test_batch_id', 100, 'test', NOW());
    "
    
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$insert_query" > /dev/null 2>&1; then
        log_message "SUCCESS" "Test checkpoint inserted successfully"
        
        # Verify insertion
        local test_result=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
            SELECT COUNT(*) FROM resubscription_checkpoints WHERE batch_id = '$test_batch_id';
        " | tr -d ' ')
        
        if [ "$test_result" = "1" ]; then
            log_message "SUCCESS" "Test data verification passed"
            
            # Clean up test data
            psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
                DELETE FROM resubscription_checkpoints WHERE batch_id = '$test_batch_id';
            " > /dev/null 2>&1
            log_message "INFO" "Test data cleaned up"
        else
            log_message "WARNING" "Test data verification failed"
        fi
    else
        log_message "WARNING" "Test data insertion failed"
    fi
}

# Function to generate rollback script
generate_rollback_script() {
    local rollback_file="$BACKUP_DIR/rollback_$(date +%Y%m%d_%H%M%S).sql"
    
    log_message "INFO" "Generating rollback script..."
    
    cat > "$rollback_file" << 'EOF'
-- Rollback script for resubscription tracking migration
-- WARNING: This will remove all resubscription tracking data

BEGIN;

-- Drop functions
DROP FUNCTION IF EXISTS get_batch_progress(VARCHAR);
DROP FUNCTION IF EXISTS identify_charging_failures(INTEGER, INTEGER);
DROP FUNCTION IF EXISTS get_error_distribution(VARCHAR);

-- Drop triggers
DROP TRIGGER IF EXISTS update_resubscription_tracking_updated_at ON resubscription_tracking;
DROP TRIGGER IF EXISTS update_resubscription_checkpoints_updated_at ON resubscription_checkpoints;

-- Drop trigger functions
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables
DROP TABLE IF EXISTS resubscription_tracking CASCADE;
DROP TABLE IF EXISTS resubscription_checkpoints CASCADE;

-- Drop indexes
DROP INDEX IF EXISTS idx_subscriptions_charging_failure;
DROP INDEX IF EXISTS idx_subscriptions_resubscribe_status;
DROP INDEX IF EXISTS idx_resubscription_tracking_batch;
DROP INDEX IF EXISTS idx_resubscription_tracking_status;

-- Remove columns from subscriptions table
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_charging_failure_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS charging_failure_count;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS charging_failure_reason;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_resubscribe_attempt_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS resubscribe_attempt_count;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS resubscribe_status;

COMMIT;

-- Note: To restore from backup, use: psql -h <host> -p <port> -U <user> -d <db> -f <backup_file>
EOF
    
    log_message "SUCCESS" "Rollback script generated: $rollback_file"
}

# Main execution
main() {
    echo "========================================="
    echo "  DATABASE MIGRATION VALIDATION & APPLICATION"
    echo "========================================="
    echo ""
    echo "Database: $DB_NAME@$DB_HOST:$DB_PORT"
    echo "User: $DB_USER"
    echo "Migration: $MIGRATION_FILE"
    echo "Log: $LOG_FILE"
    echo ""
    
    # Initialize log file
    echo "Migration validation started at $(date)" > "$LOG_FILE"
    
    # Execute steps
    check_prerequisites
    check_database_connectivity
    
    # Check if migration is needed
    if analyze_database_state; then
        log_message "SUCCESS" "Database is already up to date. No migration needed."
        echo ""
        echo "✅ Database migration validation completed successfully!"
        echo "📋 Log file: $LOG_FILE"
        exit 0
    fi
    
    # Migration is needed
    log_message "CRITICAL" "Database migration required. Proceeding with migration process..."
    
    # Create backup
    create_comprehensive_backup
    
    # Generate rollback script
    generate_rollback_script
    
    # Confirm before proceeding
    echo ""
    echo -e "${YELLOW}⚠️  WARNING: This will modify the database schema${NC}"
    echo "Backup created: $BACKUP_DIR/"
    echo "Rollback script: $BACKUP_DIR/rollback_*.sql"
    echo ""
    read -p "Do you want to proceed with the migration? (y/N): " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_message "INFO" "Migration cancelled by user"
        echo "Migration cancelled. Database remains unchanged."
        exit 0
    fi
    
    # Apply migration
    if apply_migration; then
        # Validate migration
        validate_migration
        
        # Test migration
        test_migration_with_sample_data
        
        log_message "SUCCESS" "Database migration completed successfully!"
        echo ""
        echo "✅ Database migration completed successfully!"
        echo "📋 Log file: $LOG_FILE"
        echo "💾 Backup location: $BACKUP_DIR/"
        echo "🔄 Rollback script: $BACKUP_DIR/rollback_*.sql"
        echo ""
        echo "Next steps:"
        echo "1. Deploy enhanced services to staging"
        echo "2. Test endpoints with sample data"
        echo "3. Run pilot test with 1,000 records"
    else
        log_message "ERROR" "Migration failed. Check log file for details."
        echo ""
        echo "❌ Migration failed!"
        echo "📋 Log file: $LOG_FILE"
        echo "💾 Backup available: $BACKUP_DIR/"
        echo "🔄 Use rollback script if needed: $BACKUP_DIR/rollback_*.sql"
        exit 1
    fi
}

# Execute main function
main "$@" 