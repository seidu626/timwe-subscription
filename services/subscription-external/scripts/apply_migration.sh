#!/bin/bash
# Apply database migration for resubscription tracking
# File: scripts/apply_migration.sh

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
MIGRATION_DIR="${MIGRATION_DIR:-../migrations}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================="
echo "    DATABASE MIGRATION TOOL"
echo "========================================="
echo ""

# Function to run SQL file
run_sql_file() {
    local file=$1
    local description=$2
    
    echo -e "${BLUE}Applying: $description${NC}"
    echo "File: $file"
    
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$file" > /tmp/migration_output.log 2>&1; then
        echo -e "${GREEN}✅ Success${NC}"
        return 0
    else
        echo -e "${RED}❌ Failed${NC}"
        echo "Error output:"
        cat /tmp/migration_output.log
        return 1
    fi
}

# Function to check if migration was already applied
check_migration_applied() {
    local check_query="SELECT EXISTS (
        SELECT 1 FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name = 'resubscription_tracking'
    )"
    
    result=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "$check_query" | tr -d ' ')
    
    if [ "$result" = "t" ]; then
        return 0
    else
        return 1
    fi
}

# Function to create backup
create_backup() {
    local backup_file="backup_$(date +%Y%m%d_%H%M%S).sql"
    
    echo -e "${BLUE}Creating backup: $backup_file${NC}"
    
    if pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        -t subscriptions -t invalid_msisdn_logs -t products \
        -f "/tmp/$backup_file"; then
        echo -e "${GREEN}✅ Backup created: /tmp/$backup_file${NC}"
        return 0
    else
        echo -e "${RED}❌ Backup failed${NC}"
        return 1
    fi
}

# Main execution
echo "Database: $DB_NAME@$DB_HOST:$DB_PORT"
echo "User: $DB_USER"
echo ""

# Check database connectivity
echo -e "${BLUE}Checking database connectivity...${NC}"
if pg_isready -h "$DB_HOST" -p "$DB_PORT" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Database is accessible${NC}"
else
    echo -e "${RED}❌ Cannot connect to database${NC}"
    exit 1
fi

echo ""

# Check if migration already applied
echo -e "${BLUE}Checking migration status...${NC}"
if check_migration_applied; then
    echo -e "${YELLOW}⚠️  Migration appears to be already applied${NC}"
    read -p "Do you want to continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Migration cancelled"
        exit 0
    fi
else
    echo -e "${GREEN}✅ Migration not yet applied${NC}"
fi

echo ""

# Create backup
read -p "Create backup before migration? (Y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    if ! create_backup; then
        echo -e "${RED}Backup failed. Aborting migration.${NC}"
        exit 1
    fi
fi

echo ""

# Apply migration
echo "========================================="
echo "         APPLYING MIGRATION"
echo "========================================="
echo ""

MIGRATION_FILE="$MIGRATION_DIR/001_resubscription_tracking.sql"

if [ ! -f "$MIGRATION_FILE" ]; then
    echo -e "${RED}❌ Migration file not found: $MIGRATION_FILE${NC}"
    exit 1
fi

# Start transaction and apply migration
echo -e "${BLUE}Starting migration transaction...${NC}"

cat > /tmp/migration_transaction.sql << EOF
BEGIN;

-- Show current state
SELECT 'Current tables:' as info;
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'public' 
AND table_name LIKE '%subscription%'
ORDER BY table_name;

-- Apply migration
\i $MIGRATION_FILE

-- Verify migration
SELECT 'Tables after migration:' as info;
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'public' 
AND table_name LIKE '%resubscription%'
ORDER BY table_name;

-- Check new columns
SELECT 'New columns on subscriptions table:' as info;
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'subscriptions' 
AND column_name LIKE '%charging%' OR column_name LIKE '%resubscribe%';

COMMIT;
EOF

if run_sql_file "/tmp/migration_transaction.sql" "Resubscription Tracking Migration"; then
    echo ""
    echo -e "${GREEN}=========================================${NC}"
    echo -e "${GREEN}    MIGRATION COMPLETED SUCCESSFULLY${NC}"
    echo -e "${GREEN}=========================================${NC}"
    echo ""
    
    # Verify tables created
    echo "Verification:"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
        SELECT table_name, 
               (SELECT COUNT(*) FROM information_schema.columns WHERE table_name = t.table_name) as column_count
        FROM information_schema.tables t
        WHERE table_schema = 'public' 
        AND table_name IN (
            'resubscription_tracking',
            'resubscription_checkpoints',
            'resubscription_errors',
            'resubscription_queue'
        )
        ORDER BY table_name;
    "
    
    echo ""
    echo "Next steps:"
    echo "1. Run preflight checks: ./scripts/preflight_check.sh"
    echo "2. Start monitoring: ./scripts/monitor_resubscription.sh"
    echo "3. Begin pilot test with small batch"
    
else
    echo ""
    echo -e "${RED}=========================================${NC}"
    echo -e "${RED}       MIGRATION FAILED${NC}"
    echo -e "${RED}=========================================${NC}"
    echo ""
    echo "Please check the error output above and fix any issues."
    echo "The database has been rolled back to its previous state."
    exit 1
fi
