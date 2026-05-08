-- =============================================================================
-- Generate Mobplus Campaign Share Information
-- =============================================================================
-- Generates a complete package of information to share with Mobplus
-- Run: make db-exec-sql FILE=services/acquisition-api/migrations/generate_mobplus_share_info.sql
-- =============================================================================

\echo ''
\echo '=============================================================================='
\echo 'MOBPLUS CAMPAIGN SHARE INFORMATION'
\echo '=============================================================================='
\echo ''

-- Generate campaign details with tracking URLs
SELECT
    '📋 CAMPAIGN DETAILS' AS section,
    '-------------------' AS separator;

SELECT
    'Campaign Slug' AS field,
    slug AS value
FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Country', country FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Operator', operator FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Price', price::text || ' GHS/day' FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Flow Type', flow_type FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Status', CASE WHEN enabled THEN 'ACTIVE' ELSE 'DISABLED' END FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1';

\echo ''
\echo '📌 TRACKING URL FOR MOBPLUS'
\echo '=============================='
\echo ''
\echo 'Share this URL template with Mobplus for their tracking:'
\echo ''
\echo '  https://YOUR_DOMAIN/lp/gh-airteltigo-mobplus-daily-v1?txid={click_id}&pub_id={pub_id}'
\echo ''
\echo 'Where {click_id} is their macro for the Mobplus click ID, and {pub_id} is the publisher/sub-affiliate id (optional).'
\echo ''
\echo 'Alternative parameter names supported:'
\echo '  - txid={click_id}      (Mobplus standard)'
\echo '  - click_id={click_id}  (Generic)'
\echo '  - clickid={click_id}   (Alternative)'
\echo '  - cid={click_id}       (Short form)'
\echo ''

\echo '📤 CONVERSION POSTBACK URL'
\echo '==========================='
\echo ''
\echo 'We will send conversion postbacks to Mobplus on CHARGE SUCCESS (server-to-server; not visible in browser Network):'
\echo ''

SELECT
    'Postback URL Template' AS info,
    postback_rules->'conversion'->'mobplus'->>'url' AS url
FROM campaigns
WHERE slug = 'gh-airteltigo-mobplus-daily-v1';

\echo ''
\echo '⚠️  IMPORTANT: Replace YOUR_MOBPLUS_CAMPAIGN_KEY in the postback URL'
\echo '    with the actual campaign key from Mobplus dashboard.'
\echo ''

\echo '🧪 TEST CLICK ID'
\echo '================='
\echo ''

SELECT
    'Sample Click ID for Testing' AS description,
    gen_random_uuid()::text AS click_id;

\echo ''
\echo 'Full test URL:'

SELECT
    'https://YOUR_DOMAIN/lp/gh-airteltigo-mobplus-daily-v1?txid=' || gen_random_uuid()::text AS test_url;

\echo ''
\echo '📊 ATTRIBUTION PARAMETERS ACCEPTED'
\echo '==================================='
\echo ''

SELECT
    key AS "Parameter Name",
    value::text AS "Maps To"
FROM campaigns,
     jsonb_each_text(attribution_mapping)
WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
ORDER BY key;

\echo ''
\echo '📅 CAMPAIGN METADATA'
\echo '===================='
\echo ''

SELECT
    'Created At' AS field,
    created_at::text AS value
FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Updated At', updated_at::text FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Product ID', offer_product_id::text FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1'
UNION ALL
SELECT 'Partner Role ID', partner_role_id::text FROM campaigns WHERE slug = 'gh-airteltigo-mobplus-daily-v1';

\echo ''
\echo '=============================================================================='
\echo 'END OF SHARE INFORMATION'
\echo '=============================================================================='
\echo ''
