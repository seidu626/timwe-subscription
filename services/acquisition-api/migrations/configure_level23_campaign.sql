-- =============================================================================
-- Configure Level23 Campaign (Generic Provider + Tracker Mapping)
-- =============================================================================
-- Purpose:
--   Configure campaign slug `gh-airteltigo-level23-daily-v1` so:
--   1) Traffic link uses provider=generic
--   2) Incoming Level23 tracker macro is passed as click_id/txid on landing URL
--   3) Conversion postback sends tracker={click_id} to Level23
--
-- Run:
--   make db-exec-sql FILE=services/acquisition-api/migrations/configure_level23_campaign.sql
-- =============================================================================

BEGIN;

DO $$
DECLARE
    campaign_count INT;
BEGIN
    SELECT COUNT(*)
    INTO campaign_count
    FROM campaigns
    WHERE slug = 'gh-airteltigo-level23-daily-v1';

    IF campaign_count = 0 THEN
        RAISE EXCEPTION 'Campaign slug not found: gh-airteltigo-level23-daily-v1';
    END IF;
END $$;

-- Keep existing postback_rules/providers and set only conversion.generic.
UPDATE campaigns
SET postback_rules = jsonb_set(
        COALESCE(postback_rules, '{}'::jsonb),
        '{conversion,generic}',
        '{
            "method": "GET",
            "url": "https://postback.level23.nl/?currency=USD&handler=10844&hash=1c2d51e38d4bf6b3fba837c64f7390bd&tracker={click_id}"
        }'::jsonb,
        true
    ),
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'system'
WHERE slug = 'gh-airteltigo-level23-daily-v1';

COMMIT;

\echo ''
\echo '=============================================================================='
\echo 'LEVEL23 CAMPAIGN CONFIGURED'
\echo '=============================================================================='
\echo ''
\echo 'Campaign slug: gh-airteltigo-level23-daily-v1'
\echo ''
\echo 'Share this campaign link template with Level23 traffic team:'
\echo ''
\echo '  http://139.59.135.253:3000/lp/gh-airteltigo-level23-daily-v1?provider=generic&click_id={tracker}'
\echo ''
\echo 'Alternative alias (also supported):'
\echo ''
\echo '  http://139.59.135.253:3000/lp/gh-airteltigo-level23-daily-v1?provider=generic&txid={tracker}'
\echo ''
\echo 'Configured conversion postback:'
\echo ''
\echo '  https://postback.level23.nl/?currency=USD&handler=10844&hash=1c2d51e38d4bf6b3fba837c64f7390bd&tracker={click_id}'
\echo ''
\echo '=============================================================================='
\echo ''
