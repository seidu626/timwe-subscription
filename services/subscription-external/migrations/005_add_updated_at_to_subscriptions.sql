-- Migration: Add updated_at column and other missing columns to subscriptions table
-- This migration adds the missing updated_at column and other columns that are referenced in the Go code

BEGIN;

-- Add missing columns to subscriptions table if they don't exist
DO $$
BEGIN
    -- Check if subscriptions table exists
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'subscriptions') THEN
        
        -- Add updated_at column if it doesn't exist
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'updated_at') THEN
            ALTER TABLE subscriptions ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
            RAISE NOTICE 'Added updated_at column to subscriptions table';
        ELSE
            RAISE NOTICE 'updated_at column already exists in subscriptions table';
        END IF;
        
        -- Add renewal_status column if it doesn't exist (from previous migration)
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'renewal_status') THEN
            ALTER TABLE subscriptions ADD COLUMN renewal_status VARCHAR(50) DEFAULT 'active';
            RAISE NOTICE 'Added renewal_status column to subscriptions table';
        ELSE
            RAISE NOTICE 'renewal_status column already exists in subscriptions table';
        END IF;
        
        -- Add last_renewal_attempt column if it doesn't exist (from previous migration)
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'last_renewal_attempt') THEN
            ALTER TABLE subscriptions ADD COLUMN last_renewal_attempt TIMESTAMP;
            RAISE NOTICE 'Added last_renewal_attempt column to subscriptions table';
        ELSE
            RAISE NOTICE 'last_renewal_attempt column already exists in subscriptions table';
        END IF;
        
        -- Add total_renewal_attempts column if it doesn't exist (from previous migration)
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'total_renewal_attempts') THEN
            ALTER TABLE subscriptions ADD COLUMN total_renewal_attempts INT DEFAULT 0;
            RAISE NOTICE 'Added total_renewal_attempts column to subscriptions table';
        ELSE
            RAISE NOTICE 'total_renewal_attempts column already exists in subscriptions table';
        END IF;
        
        -- Add last_successful_payment column if it doesn't exist (from previous migration)
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'last_successful_payment') THEN
            ALTER TABLE subscriptions ADD COLUMN last_successful_payment TIMESTAMP;
            RAISE NOTICE 'Added last_successful_payment column to subscriptions table';
        ELSE
            RAISE NOTICE 'last_successful_payment column already exists in subscriptions table';
        END IF;
        
        -- Add consecutive_payment_failures column if it doesn't exist (from previous migration)
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subscriptions' AND column_name = 'consecutive_payment_failures') THEN
            ALTER TABLE subscriptions ADD COLUMN consecutive_payment_failures INT DEFAULT 0;
            RAISE NOTICE 'Added consecutive_payment_failures column to subscriptions table';
        ELSE
            RAISE NOTICE 'consecutive_payment_failures column already exists in subscriptions table';
        END IF;
        
    ELSE
        RAISE EXCEPTION 'subscriptions table does not exist';
    END IF;
END $$;

-- Update existing records to have updated_at = created_at for new updated_at column
UPDATE subscriptions SET updated_at = created_at WHERE updated_at IS NULL;

-- Create trigger to automatically update updated_at on row updates
CREATE OR REPLACE FUNCTION update_subscriptions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop existing trigger if it exists
DROP TRIGGER IF EXISTS update_subscriptions_updated_at_trigger ON subscriptions;

-- Create trigger
CREATE TRIGGER update_subscriptions_updated_at_trigger
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_subscriptions_updated_at();

-- Create indexes for better performance if they don't exist
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'subscriptions') THEN
        
        -- Add indexes for renewal queries if they don't exist
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subscriptions_renewal_status') THEN
            CREATE INDEX idx_subscriptions_renewal_status ON subscriptions(renewal_status, last_successful_payment);
            RAISE NOTICE 'Created index idx_subscriptions_renewal_status';
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subscriptions_last_renewal') THEN
            CREATE INDEX idx_subscriptions_last_renewal ON subscriptions(last_renewal_attempt, renewal_status);
            RAISE NOTICE 'Created index idx_subscriptions_last_renewal';
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subscriptions_payment_status') THEN
            CREATE INDEX idx_subscriptions_payment_status ON subscriptions(last_successful_payment, renewal_status);
            RAISE NOTICE 'Created index idx_subscriptions_payment_status';
        END IF;
        
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_subscriptions_updated_at') THEN
            CREATE INDEX idx_subscriptions_updated_at ON subscriptions(updated_at);
            RAISE NOTICE 'Created index idx_subscriptions_updated_at';
        END IF;
        
    END IF;
END $$;

-- Verify the columns were added
SELECT column_name, data_type, is_nullable, column_default 
FROM information_schema.columns 
WHERE table_name = 'subscriptions' 
AND column_name IN ('updated_at', 'renewal_status', 'last_renewal_attempt', 'total_renewal_attempts', 'last_successful_payment', 'consecutive_payment_failures')
ORDER BY column_name;

-- Verify the trigger was created
SELECT trigger_name, event_manipulation, action_statement
FROM information_schema.triggers 
WHERE event_object_table = 'subscriptions' 
AND trigger_name LIKE '%updated_at%';

COMMIT; 