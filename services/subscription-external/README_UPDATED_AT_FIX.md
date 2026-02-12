# Quick Fix for Missing `updated_at` Column

## Problem
The application is failing with this error:
```
pq: column "updated_at" of relation "subscriptions" does not exist
```

## Quick Solution

### 1. Test the migration (recommended)
```bash
cd services/subscription-external/scripts
./test_migration_syntax.sh
```

### 2. Apply the migration
```bash
cd services/subscription-external/scripts
./apply_updated_at_migration.sh
```

## What This Fixes

- ✅ Adds missing `updated_at` column to `subscriptions` table
- ✅ Adds other missing columns (`renewal_status`, `last_renewal_attempt`, etc.)
- ✅ Creates automatic trigger to update `updated_at` on row changes
- ✅ Adds performance indexes for better query performance
- ✅ Fixes the renewal system errors

## Files Created

- `migrations/005_add_updated_at_to_subscriptions.sql` - Database migration
- `scripts/apply_updated_at_migration.sh` - Apply migration script
- `scripts/test_migration_syntax.sh` - Test migration script
- `docs/UPDATED_AT_COLUMN_FIX.md` - Detailed documentation

## After Fix

The application should work without the "column does not exist" error. The renewal system will be able to:

- Update subscription renewal status
- Track renewal attempts
- Monitor payment status
- Maintain audit trails with `updated_at` timestamps

## Need Help?

See `docs/UPDATED_AT_COLUMN_FIX.md` for detailed information and troubleshooting. 