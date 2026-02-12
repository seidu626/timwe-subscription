#!/bin/bash
# Apply migration to add updated_at column to subscriptions table
# File: scripts/apply_updated_at_migration.sh

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
echo "    ADD UPDATED_AT COLUMN MIGRATION"
echo "========================================="
echo ""

echo "Database: $DB_NAME@$DB_HOST:$DB_PORT"
echo "User: $DB_USER"
echo "Migration file: $MIGRATION_FILE"
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

# Check if migration file exists
if [ ! -f "$MIGRATION_FILE" ]; then
    echo -e "${RED}❌ Migration file not found: $MIGRATION_FILE${NC}"
    exit 1
fi

# Check current state of subscriptions table
echo -e "${BLUE}Checking current subscriptions table structure...${NC}"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    SELECT column_name, data_type, is_nullable, column_default 
    FROM information_schema.columns 
    WHERE table_name = 'subscriptions' 
    ORDER BY ordinal_position;
"

echo ""

# Check if updated_at column already exists
echo -e "${BLUE}Checking if updated_at column already exists...${NC}"
EXISTS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
    SELECT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'subscriptions' AND column_name = 'updated_at'
    );
" | tr -d ' ')

if [ "$EXISTS" = "t" ]; then
    echo -e "${YELLOW}⚠️  updated_at column already exists in subscriptions table${NC}"
    echo "Migration not needed."
    exit 0
fi

echo -e "${BLUE}updated_at column does not exist. Proceeding with migration...${NC}"
echo ""

# Create backup
read -p "Create backup before migration? (Y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    echo -e "${BLUE}Creating backup...${NC}"
    BACKUP_FILE="backup_subscriptions_$(date +%Y%m%d_%H%M%S).sql"
    
    if pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
        -t subscriptions -f "/tmp/$BACKUP_FILE"; then
        echo -e "${GREEN}✅ Backup created: /tmp/$BACKUP_FILE${NC}"
    else
        echo -e "${RED}❌ Backup failed${NC}"
        read -p "Continue without backup? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
fi

echo ""

# Apply migration
echo -e "${BLUE}Applying migration...${NC}"
echo "File: $MIGRATION_FILE"

if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$MIGRATION_FILE"; then
    echo -e "${GREEN}✅ Migration applied successfully${NC}"
else
    echo -e "${RED}❌ Migration failed${NC}"
    exit 1
fi

echo ""

# Verify migration
echo -e "${BLUE}Verifying migration...${NC}"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    SELECT column_name, data_type, is_nullable, column_default 
    FROM information_schema.columns 
    WHERE table_name = 'subscriptions' AND column_name = 'updated_at';
"

echo ""

# Check trigger
echo -e "${BLUE}Checking trigger...${NC}"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    SELECT trigger_name, event_manipulation, action_statement
    FROM information_schema.triggers 
    WHERE event_object_table = 'subscriptions' 
    AND trigger_name LIKE '%updated_at%';
"

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}    MIGRATION COMPLETED SUCCESSFULLY${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo "The updated_at column has been added to the subscriptions table."
echo "The application should now work without the 'column does not exist' error." 