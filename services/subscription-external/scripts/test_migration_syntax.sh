#!/bin/bash
# Test SQL syntax of migration files
# File: scripts/test_migration_syntax.sh

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-subscription_manager}"
DB_USER="${DB_USER:-sm_admin}"
MIGRATION_FILE="../migrations/005_add_updated_at_to_subscriptions.sql"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================="
echo "    TESTING MIGRATION SQL SYNTAX"
echo "========================================="
echo ""

echo "Migration file: $MIGRATION_FILE"
echo ""

# Check if migration file exists
if [ ! -f "$MIGRATION_FILE" ]; then
    echo -e "${RED}❌ Migration file not found: $MIGRATION_FILE${NC}"
    exit 1
fi

# Check database connectivity
echo -e "${BLUE}Checking database connectivity...${NC}"
if pg_isready -h "$DB_HOST" -p "$DB_PORT" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Database is accessible${NC}"
else
    echo -e "${RED}❌ Cannot connect to database${NC}"
    exit 1
fi

echo ""

# Test SQL syntax by parsing the file
echo -e "${BLUE}Testing SQL syntax...${NC}"

# Create a temporary test database for syntax checking
TEST_DB="test_migration_$(date +%s)"
echo "Creating temporary test database: $TEST_DB"

# Create test database
if createdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$TEST_DB" 2>/dev/null; then
    echo -e "${GREEN}✅ Test database created${NC}"
else
    echo -e "${YELLOW}⚠️  Test database already exists, using existing one${NC}"
fi

# Create a minimal subscriptions table for testing
echo "Creating test subscriptions table..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" -c "
CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    partner_role_id INTEGER NOT NULL,
    user_identifier VARCHAR(50) NOT NULL,
    user_identifier_type VARCHAR(20) NOT NULL DEFAULT 'MSISDN',
    product_id INTEGER NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    start_date TIMESTAMP DEFAULT NOW(),
    end_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
" > /dev/null 2>&1

echo -e "${GREEN}✅ Test table created${NC}"

# Test the migration syntax
echo "Testing migration syntax..."
if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" -f "$MIGRATION_FILE" > /tmp/migration_test.log 2>&1; then
    echo -e "${GREEN}✅ Migration syntax is valid${NC}"
    
    # Show what was created
    echo ""
    echo -e "${BLUE}Verification results:${NC}"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" -c "
        SELECT column_name, data_type, is_nullable, column_default 
        FROM information_schema.columns 
        WHERE table_name = 'subscriptions' 
        AND column_name IN ('updated_at', 'renewal_status', 'last_renewal_attempt', 'total_renewal_attempts', 'last_successful_payment', 'consecutive_payment_failures')
        ORDER BY column_name;
    "
    
    echo ""
    echo -e "${BLUE}Trigger verification:${NC}"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" -c "
        SELECT trigger_name, event_manipulation, action_statement
        FROM information_schema.triggers 
        WHERE event_object_table = 'subscriptions' 
        AND trigger_name LIKE '%updated_at%';
    "
    
else
    echo -e "${RED}❌ Migration syntax has errors${NC}"
    echo "Error output:"
    cat /tmp/migration_test.log
    echo ""
    echo -e "${RED}Please fix the syntax errors before applying the migration.${NC}"
    
    # Clean up test database
    dropdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$TEST_DB" 2>/dev/null || true
    exit 1
fi

# Clean up test database
echo ""
echo -e "${BLUE}Cleaning up test database...${NC}"
if dropdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$TEST_DB" 2>/dev/null; then
    echo -e "${GREEN}✅ Test database cleaned up${NC}"
else
    echo -e "${YELLOW}⚠️  Could not clean up test database (may have been dropped already)${NC}"
fi

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}    SYNTAX TEST COMPLETED SUCCESSFULLY${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo "The migration file has valid SQL syntax and can be applied safely."
echo "Run the apply_updated_at_migration.sh script to apply it to your production database." 