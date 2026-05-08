#!/bin/bash
# Invalid MSISDN Logs Optimization Deployment Script
# Purpose: Deploy the complete optimization system for invalid_msisdn_logs table
# Author: System Optimization Team
# Date: 2025-01-27

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DB_CONFIG_FILE="$PROJECT_ROOT/config.yaml"
LOG_FILE="/tmp/invalid_msisdn_optimization.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$LOG_FILE"
    exit 1
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$LOG_FILE"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$LOG_FILE"
}

# Function to check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if PostgreSQL client is available
    if ! command -v psql &> /dev/null; then
        error "PostgreSQL client (psql) is not installed"
    fi
    
    # Check if Go is available
    if ! command -v go &> /dev/null; then
        error "Go is not installed"
    fi
    
    # Check if Redis client is available
    if ! command -v redis-cli &> /dev/null; then
        warning "Redis client (redis-cli) is not installed - Redis operations will be skipped"
    fi
    
    # Check if config file exists
    if [[ ! -f "$DB_CONFIG_FILE" ]]; then
        error "Database config file not found: $DB_CONFIG_FILE"
    fi
    
    log "Prerequisites check completed"
}

# Function to extract database configuration
extract_db_config() {
    log "Extracting database configuration..."
    
    # Use yq or similar tool to extract config (fallback to grep if not available)
    if command -v yq &> /dev/null; then
        DB_HOST=$(yq eval '.DB.POSTGRESQL.HOST' "$DB_CONFIG_FILE")
        DB_PORT=$(yq eval '.DB.POSTGRESQL.PORT' "$DB_CONFIG_FILE")
        DB_USER=$(yq eval '.DB.POSTGRESQL.USER' "$DB_CONFIG_FILE")
        DB_PASSWORD=$(yq eval '.DB.POSTGRESQL.PASSWORD' "$DB_CONFIG_FILE")
        DB_NAME=$(yq eval '.DB.POSTGRESQL.DB_NAME' "$DB_CONFIG_FILE")
    else
        # Fallback to grep (less reliable but works)
        DB_HOST=$(grep -A 5 "POSTGRESQL:" "$DB_CONFIG_FILE" | grep "HOST:" | awk '{print $2}')
        DB_PORT=$(grep -A 5 "POSTGRESQL:" "$DB_CONFIG_FILE" | grep "PORT:" | awk '{print $2}')
        DB_USER=$(grep -A 5 "POSTGRESQL:" "$DB_CONFIG_FILE" | grep "USER:" | awk '{print $2}')
        DB_PASSWORD=$(grep -A 5 "POSTGRESQL:" "$DB_CONFIG_FILE" | grep "PASSWORD:" | awk '{print $2}')
        DB_NAME=$(grep -A 5 "POSTGRESQL:" "$DB_CONFIG_FILE" | grep "DB_NAME:" | awk '{print $2}')
    fi
    
    # Validate extracted values
    if [[ -z "$DB_HOST" || -z "$DB_PORT" || -z "$DB_USER" || -z "$DB_NAME" ]]; then
        error "Failed to extract database configuration"
    fi
    
    # Set environment variables
    export PGPASSWORD="$DB_PASSWORD"
    export PGHOST="$DB_HOST"
    export PGPORT="$DB_PORT"
    export PGUSER="$DB_USER"
    export PGDATABASE="$DB_NAME"
    
    log "Database configuration extracted successfully"
    log "Host: $DB_HOST, Port: $DB_PORT, User: $DB_USER, Database: $DB_NAME"
}

# Function to test database connection
test_db_connection() {
    log "Testing database connection..."
    
    if ! psql -c "SELECT 1;" &> /dev/null; then
        error "Failed to connect to database"
    fi
    
    log "Database connection successful"
}

# Function to backup current table
backup_table() {
    log "Creating backup of invalid_msisdn_logs table..."
    
    BACKUP_FILE="/tmp/invalid_msisdn_logs_backup_$(date +%Y%m%d_%H%M%S).sql"
    
    if pg_dump -t invalid_msisdn_logs > "$BACKUP_FILE"; then
        log "Backup created successfully: $BACKUP_FILE"
    else
        error "Failed to create backup"
    fi
}

# Function to deploy database optimizations
deploy_database_optimizations() {
    log "Deploying database optimizations..."
    
    # Determine which SQL script to use
    if [[ "$USE_COMPATIBLE" == "true" ]]; then
        SQL_SCRIPT="$SCRIPT_DIR/optimize_invalid_msisdn_compatible.sql"
        log "Using compatible optimization script (works across PostgreSQL versions)"
    elif [[ "$USE_SIMPLE" == "true" ]]; then
        SQL_SCRIPT="$SCRIPT_DIR/optimize_invalid_msisdn_simple.sql"
        log "Using simple optimization script (no CONCURRENTLY)"
    else
        SQL_SCRIPT="$SCRIPT_DIR/optimize_invalid_msisdn_database.sql"
        log "Using full optimization script (with CONCURRENTLY support)"
    fi
    
    if [[ ! -f "$SQL_SCRIPT" ]]; then
        error "SQL optimization script not found: $SQL_SCRIPT"
    fi
    
    log "Running database optimization script: $(basename "$SQL_SCRIPT")"
    if psql -f "$SQL_SCRIPT"; then
        log "Database optimizations deployed successfully"
    else
        error "Failed to deploy database optimizations"
    fi
}

# Function to build and deploy Go optimizations
deploy_go_optimizations() {
    log "Building and deploying Go optimizations..."
    
    cd "$PROJECT_ROOT"
    
    # Download dependencies
    log "Downloading Go dependencies..."
    if ! go mod download; then
        error "Failed to download Go dependencies"
    fi
    
    # Build the application
    log "Building application..."
    if ! go build -o bin/subscription-external ./cmd/main.go; then
        error "Failed to build application"
    fi
    
    log "Go optimizations built successfully"
}

# Function to test optimizations
test_optimizations() {
    log "Testing optimizations..."
    
    # Test database performance
    log "Testing database query performance..."
    psql -c "SELECT test_query_performance();" || warning "Performance testing failed"
    
    # Test monitoring views
    log "Testing monitoring views..."
    psql -c "SELECT * FROM invalid_msisdn_performance LIMIT 5;" || warning "Performance view test failed"
    
    # Test archive function
    log "Testing archive function..."
    psql -c "SELECT get_archive_stats();" || warning "Archive stats test failed"
    
    log "Optimization testing completed"
}

# Function to setup monitoring and maintenance
setup_monitoring() {
    log "Setting up monitoring and maintenance..."
    
    # Create maintenance script (no archival since old records are needed)
    MAINTENANCE_SCRIPT="/usr/local/bin/maintain_invalid_msisdn_logs.sh"
    cat > "$MAINTENANCE_SCRIPT" << 'EOF'
#!/bin/bash
# Maintenance script for invalid_msisdn_logs table
# Note: No archival - old records are needed for MSISDN generation
cd "$(dirname "$0")/../../services/subscription-external"
psql -c "SELECT maintain_invalid_msisdn_logs();"
psql -c "SELECT get_table_bloat_info();"
psql -c "SELECT get_invalid_msisdn_stats();"
EOF
    
    chmod +x "$MAINTENANCE_SCRIPT"
    log "Maintenance script created: $MAINTENANCE_SCRIPT"
    
    log "Note: Archival system not implemented - old records are preserved for MSISDN generation"
}

# Function to show deployment summary
show_deployment_summary() {
    log "===================================================="
    log "INVALID MSISDN LOGS OPTIMIZATION DEPLOYMENT COMPLETE"
    log "===================================================="
    
    # Show current table status
    log "Current table status:"
    psql -c "SELECT * FROM invalid_msisdn_performance;" || warning "Failed to get performance data"
    
    # Show index information
    log "Index information:"
    psql -c "SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'invalid_msisdn_logs';" || warning "Failed to get index information"
    
    # Show next steps
    log "===================================================="
    log "NEXT STEPS:"
    log "1. Monitor query performance using the created views"
    log "2. Use maintain_invalid_msisdn_logs() for regular maintenance"
    log "3. Consider migrating to partitioned table when ready"
    log "4. Monitor Bloom Filter performance in Go application"
    log "5. Old records are preserved for MSISDN generation (no archival)"
    log "===================================================="
}

# Function to rollback changes
rollback_changes() {
    warning "Rolling back changes..."
    
    if [[ -f "$BACKUP_FILE" ]]; then
        log "Restoring from backup: $BACKUP_FILE"
        if psql -f "$BACKUP_FILE"; then
            log "Rollback completed successfully"
        else
            error "Rollback failed"
        fi
    else
        error "Backup file not found for rollback"
    fi
}

# Main execution
main() {
    log "Starting Invalid MSISDN Logs Optimization Deployment"
    log "Project Root: $PROJECT_ROOT"
    log "Script Directory: $SCRIPT_DIR"
    
    # Set up error handling
    trap 'error "Deployment failed at line $LINENO"' ERR
    trap 'rollback_changes' EXIT
    
    # Execute deployment steps
    check_prerequisites
    extract_db_config
    test_db_connection
    backup_table
    deploy_database_optimizations
    deploy_go_optimizations
    test_optimizations
    setup_monitoring
    
    # Remove rollback trap on success
    trap - EXIT
    
    show_deployment_summary
    
    log "Deployment completed successfully!"
}

# Check if script is run with --help
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --help, -h         Show this help message"
    echo "  --test-only        Only run tests, skip deployment"
    echo "  --rollback         Rollback to previous state"
    echo "  --compatible       Use compatible SQL script (recommended, works across PostgreSQL versions)"
    echo "  --simple           Use simple SQL script (no CONCURRENTLY)"
    echo "  --full             Use full SQL script (with CONCURRENTLY support)"
    echo ""
    echo "This script deploys the complete optimization system for the invalid_msisdn_logs table."
    echo "It includes database optimizations, Go code updates, and monitoring setup."
    echo ""
    echo "Examples:"
    echo "  $0                 # Deploy with compatible script (default)"
    echo "  $0 --compatible    # Deploy with compatible script (explicit)"
    echo "  $0 --simple        # Deploy with simple script"
    echo "  $0 --full          # Deploy with full script (CONCURRENTLY)"
    echo "  $0 --test-only     # Only test optimizations"
    exit 0
fi

# Set default options
USE_COMPATIBLE="true"  # Default to compatible script for maximum compatibility

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --compatible)
            USE_COMPATIBLE="true"
            USE_SIMPLE="false"
            shift
            ;;
        --simple)
            USE_COMPATIBLE="false"
            USE_SIMPLE="true"
            shift
            ;;
        --full)
            USE_COMPATIBLE="false"
            USE_SIMPLE="false"
            shift
            ;;
        --test-only)
            TEST_ONLY="true"
            shift
            ;;
        --rollback)
            ROLLBACK="true"
            shift
            ;;
        *)
            # Unknown option, keep for backward compatibility
            break
            ;;
    esac
done

# Handle different execution modes
if [[ "$TEST_ONLY" == "true" ]]; then
    log "Running in test-only mode..."
    extract_db_config
    test_db_connection
    test_optimizations
    show_deployment_summary
    exit 0
fi

if [[ "$ROLLBACK" == "true" ]]; then
    log "Running in rollback mode..."
    extract_db_config
    test_db_connection
    rollback_changes
    exit 0
fi

# Run main deployment
main 