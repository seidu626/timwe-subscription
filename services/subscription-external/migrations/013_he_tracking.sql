-- Migration: Add Header Enrichment (HE) tracking columns to acquisition_transactions
-- This enables tracking whether transactions used real MNO HE headers or simulation

-- Add HE tracking columns
ALTER TABLE acquisition_transactions
ADD COLUMN IF NOT EXISTS he_source VARCHAR(20),
ADD COLUMN IF NOT EXISTS he_msisdn VARCHAR(20),
ADD COLUMN IF NOT EXISTS he_operator VARCHAR(50);

-- Add comments for documentation
COMMENT ON COLUMN acquisition_transactions.he_source IS 'Source of HE identity: REAL (MNO headers), SIMULATED (testing), or NULL (no HE)';
COMMENT ON COLUMN acquisition_transactions.he_msisdn IS 'MSISDN from HE headers (may differ from form-submitted MSISDN)';
COMMENT ON COLUMN acquisition_transactions.he_operator IS 'Detected operator name from MCC/MNC or MSISDN prefix';

-- Create index for querying by HE source (useful for analytics)
CREATE INDEX IF NOT EXISTS idx_acquisition_transactions_he_source
ON acquisition_transactions(he_source)
WHERE he_source IS NOT NULL;
