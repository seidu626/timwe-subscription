-- =============================================================================
-- Generate Level23 Campaign Share Information
-- =============================================================================
-- Run:
--   make db-exec-sql FILE=services/acquisition-api/migrations/generate_level23_share_info.sql
-- =============================================================================

\echo ''
\echo '=============================================================================='
\echo 'LEVEL23 CAMPAIGN SHARE INFORMATION'
\echo '=============================================================================='
\echo ''

SELECT
    'Campaign Slug' AS field,
    slug AS value
FROM campaigns WHERE slug = 'gh-airteltigo-level23-daily-v1'
UNION ALL
SELECT 'Country', country FROM campaigns WHERE slug = 'gh-airteltigo-level23-daily-v1'
UNION ALL
SELECT 'Operator', operator FROM campaigns WHERE slug = 'gh-airteltigo-level23-daily-v1'
UNION ALL
SELECT 'Flow Type', flow_type FROM campaigns WHERE slug = 'gh-airteltigo-level23-daily-v1'
UNION ALL
SELECT 'Status', CASE WHEN enabled THEN 'ACTIVE' ELSE 'DISABLED' END FROM campaigns WHERE slug = 'gh-airteltigo-level23-daily-v1';

\echo ''
\echo 'CAMPAIGN LINK TEMPLATE TO SHARE'
\echo '-------------------------------'
\echo ''
\echo 'http://139.59.135.253:3000/lp/gh-airteltigo-level23-daily-v1?provider=generic&click_id={tracker}'
\echo ''
\echo 'Alternative alias (same behavior):'
\echo 'http://139.59.135.253:3000/lp/gh-airteltigo-level23-daily-v1?provider=generic&txid={tracker}'
\echo ''
\echo 'Note: use {tracker} macro from Level23. It is captured as canonical click_id.'
\echo ''

\echo 'CONFIGURED CONVERSION POSTBACK'
\echo '------------------------------'

SELECT
    postback_rules->'conversion'->'generic'->>'url' AS conversion_postback_url
FROM campaigns
WHERE slug = 'gh-airteltigo-level23-daily-v1';

\echo ''
\echo 'LATEST CAMPAIGN UPDATE TIME'
\echo '---------------------------'

SELECT
    updated_at::text AS updated_at
FROM campaigns
WHERE slug = 'gh-airteltigo-level23-daily-v1';

\echo ''
\echo '=============================================================================='
\echo 'END OF SHARE INFORMATION'
\echo '=============================================================================='
\echo ''
