# Missing `updated_at` Column Fix

## Issue Description

The application was encountering the following error:

```
pq: column "updated_at" of relation "subscriptions" does not exist
```

This error occurred in the `RenewalRepository.UpdateSubscriptionRenewalStatus` method when trying to execute the SQL query:

```sql
UPDATE subscriptions 
SET renewal_status = $1, updated_at = NOW()
WHERE user_identifier = $2 AND product_id = $3
```

## Root Cause

The `subscriptions` table in the database schema (`services/pg_schema.sql`) was missing several columns that are referenced in the Go code:

1. **`updated_at`** - Referenced in multiple SQL queries for tracking when records were last modified
2. **`renewal_status`** - Used for tracking subscription renewal status
3. **`last_renewal_attempt`** - Tracks when the last renewal attempt was made
4. **`total_renewal_attempts`** - Counts total renewal attempts
5. **`last_successful_payment`** - Tracks the last successful payment date
6. **`consecutive_payment_failures`** - Counts consecutive payment failures

## Solution

### 1. Migration File

Created `services/subscription-external/migrations/005_add_updated_at_to_subscriptions.sql` which:

- Adds all missing columns to the `subscriptions` table
- Sets appropriate default values
- Creates a trigger to automatically update `updated_at` on row modifications
- Adds performance indexes for the new columns
- Includes comprehensive verification queries

### 2. Application Scripts

Created two scripts to help with the migration:

#### `apply_updated_at_migration.sh`
- Applies the migration to the database
- Creates a backup before applying changes
- Verifies the migration was successful
- Shows detailed results

#### `test_migration_syntax.sh`
- Tests the SQL syntax in a temporary database
- Validates that the migration will work correctly
- Helps catch syntax errors before applying to production

## Usage

### Step 1: Test the Migration
```bash
cd services/subscription-external/scripts
./test_migration_syntax.sh
```

### Step 2: Apply the Migration
```bash
cd services/subscription-external/scripts
./apply_updated_at_migration.sh
```

## Database Changes

The migration will add the following columns to the `subscriptions` table:

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `updated_at` | TIMESTAMP | CURRENT_TIMESTAMP | Automatically updated on row modifications |
| `renewal_status` | VARCHAR(50) | 'active' | Current renewal status |
| `last_renewal_attempt` | TIMESTAMP | NULL | Last renewal attempt timestamp |
| `total_renewal_attempts` | INT | 0 | Total number of renewal attempts |
| `last_successful_payment` | TIMESTAMP | NULL | Last successful payment timestamp |
| `consecutive_payment_failures` | INT | 0 | Count of consecutive payment failures |

## Triggers and Indexes

### Automatic Update Trigger
A trigger is created to automatically set `updated_at = NOW()` whenever a row is updated.

### Performance Indexes
- `idx_subscriptions_renewal_status` - For queries filtering by renewal status
- `idx_subscriptions_last_renewal` - For queries filtering by last renewal attempt
- `idx_subscriptions_payment_status` - For queries filtering by payment status
- `idx_subscriptions_updated_at` - For queries filtering by update time

## Verification

After applying the migration, you can verify the changes:

```sql
-- Check columns were added
SELECT column_name, data_type, is_nullable, column_default 
FROM information_schema.columns 
WHERE table_name = 'subscriptions' 
AND column_name IN ('updated_at', 'renewal_status', 'last_renewal_attempt', 'total_renewal_attempts', 'last_successful_payment', 'consecutive_payment_failures')
ORDER BY column_name;

-- Check trigger was created
SELECT trigger_name, event_manipulation, action_statement
FROM information_schema.triggers 
WHERE event_object_table = 'subscriptions' 
AND trigger_name LIKE '%updated_at%';
```

## Impact

- **Positive**: Fixes the application error and allows renewal operations to work correctly
- **Performance**: Adds useful indexes for better query performance
- **Data Integrity**: Ensures `updated_at` is always current
- **Backward Compatibility**: Existing data is preserved and updated appropriately

## Rollback

If you need to rollback the migration, you can:

1. Drop the added columns:
```sql
ALTER TABLE subscriptions DROP COLUMN IF EXISTS updated_at;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS renewal_status;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_renewal_attempt;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS total_renewal_attempts;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS last_successful_payment;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS consecutive_payment_failures;
```

2. Drop the trigger:
```sql
DROP TRIGGER IF EXISTS update_subscriptions_updated_at_trigger ON subscriptions;
DROP FUNCTION IF EXISTS update_subscriptions_updated_at();
```

3. Drop the indexes:
```sql
DROP INDEX IF EXISTS idx_subscriptions_renewal_status;
DROP INDEX IF EXISTS idx_subscriptions_last_renewal;
DROP INDEX IF EXISTS idx_subscriptions_payment_status;
DROP INDEX IF EXISTS idx_subscriptions_updated_at;
```

## Notes

- The migration is idempotent - it can be run multiple times safely
- Existing records will have `updated_at` set to `created_at` initially
- The migration includes comprehensive error handling and logging
- All changes are wrapped in a transaction for safety 