-- Migration: Add charge tracking columns to acquisition_transactions
-- Required for Mobplus integration: fire conversion postback only on charge success

-- Add charge tracking columns
ALTER TABLE acquisition_transactions
ADD COLUMN IF NOT EXISTS charged_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS charge_payout VARCHAR(50),
ADD COLUMN IF NOT EXISTS conversion_postback_sent BOOLEAN DEFAULT false;

-- Create index for charge status lookups
CREATE INDEX IF NOT EXISTS idx_acq_trans_charged_at ON acquisition_transactions(charged_at);
CREATE INDEX IF NOT EXISTS idx_acq_trans_conversion_sent ON acquisition_transactions(conversion_postback_sent);

-- Add CHARGED status to the check constraint (if it doesn't exist)
-- Note: PostgreSQL doesn't allow easy alteration of CHECK constraints, so we drop and recreate
DO $$
BEGIN
    -- Check if we need to update the constraint
    IF EXISTS (
        SELECT 1 FROM information_schema.check_constraints 
        WHERE constraint_name = 'acquisition_transactions_status_check'
    ) THEN
        -- Drop the existing constraint
        ALTER TABLE acquisition_transactions DROP CONSTRAINT IF EXISTS acquisition_transactions_status_check;
    END IF;
    
    -- Add updated constraint with CHARGED status
    ALTER TABLE acquisition_transactions 
    ADD CONSTRAINT acquisition_transactions_status_check 
    CHECK (status IN ('PENDING', 'ACTION_REQUIRED', 'CONFIRM_REQUIRED', 'SUBSCRIBED', 'CHARGED', 'FAILED', 'CANCELLED'));
    
EXCEPTION WHEN OTHERS THEN
    -- Constraint might not exist or already be updated
    RAISE NOTICE 'Status constraint update skipped: %', SQLERRM;
END $$;

-- Add unique constraint for idempotent conversion postbacks
-- This prevents duplicate postbacks for the same (provider, click_id, event)
CREATE UNIQUE INDEX IF NOT EXISTS idx_acq_trans_unique_conversion 
ON acquisition_transactions(ad_provider, click_id) 
WHERE conversion_postback_sent = true;

COMMENT ON COLUMN acquisition_transactions.charged_at IS 'Timestamp when charge was confirmed by subscription-external';
COMMENT ON COLUMN acquisition_transactions.charge_payout IS 'Payout amount for conversion postback';
COMMENT ON COLUMN acquisition_transactions.conversion_postback_sent IS 'True if conversion postback has been enqueued (idempotency flag)';
