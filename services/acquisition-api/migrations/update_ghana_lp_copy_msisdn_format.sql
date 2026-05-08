-- =============================================================================
-- Update GH Campaign LP Copy for Local 9-Digit MSISDN Input
-- =============================================================================
-- Purpose:
--   Align existing Ghana campaign copy with strict landing-page MSISDN formatting
--   and mark rows as updated to avoid accidental reprocessing:
--   - lp_copy.en.msisdnPlaceholder: "Mobile number (9 digits)"
--   - lp_copy.en.phoneInvalid: "Enter a valid 9-digit mobile number."
--   - tracking_config.gh_with_new_placeholder: 1
--
-- Run:
--   make db-exec-sql FILE=services/acquisition-api/migrations/update_ghana_lp_copy_msisdn_format.sql
-- =============================================================================

BEGIN;

UPDATE campaigns
SET
  lp_copy = jsonb_set(
              jsonb_set(
                COALESCE(lp_copy, '{}'::jsonb),
                '{en,msisdnPlaceholder}',
                '"Mobile number (9 digits)"'::jsonb,
                true
              ),
              '{en,phoneInvalid}',
              '"Enter a valid 9-digit mobile number."'::jsonb,
              true
            ),
  tracking_config = jsonb_set(
                      COALESCE(tracking_config, '{}'::jsonb),
                      '{gh_with_new_placeholder}',
                      '1'::jsonb,
                      true
                    ),
  updated_at = CURRENT_TIMESTAMP,
  updated_by = 'system'
WHERE country = 'GH'
  AND (
    CASE
      WHEN (tracking_config ->> 'gh_with_new_placeholder') ~ '^[0-9]+$'
        THEN (tracking_config ->> 'gh_with_new_placeholder')::int
      ELSE 0
    END
  ) = 0;

COMMIT;

\echo ''
\echo '=============================================================================='
\echo 'GH CAMPAIGN LP COPY UPDATED'
\echo '=============================================================================='
\echo ''
\echo 'Updated fields for country=GH campaigns:'
\echo '  lp_copy.en.msisdnPlaceholder -> Mobile number (9 digits)'
\echo '  lp_copy.en.phoneInvalid      -> Enter a valid 9-digit mobile number.'
\echo '  tracking_config.gh_with_new_placeholder -> 1'
\echo ''
\echo '=============================================================================='
\echo ''
