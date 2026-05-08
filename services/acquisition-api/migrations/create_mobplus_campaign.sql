-- =============================================================================
-- Complete Mobplus Campaign with Click-ID Support
-- =============================================================================
-- Run against remote database:
--   psql -h 139.59.135.253 -U sm_admin -d subscription_manager -f create_mobplus_campaign.sql
--
-- Or via make target:
--   make db-exec-sql FILE=services/acquisition-api/migrations/create_mobplus_campaign.sql
-- =============================================================================

BEGIN;

-- =============================================================================
-- 1. Create the main Mobplus Campaign
-- =============================================================================
-- Replace YOUR_MOBPLUS_CAMPAIGN_KEY with the actual key from Mobplus dashboard
-- Example Mobplus postback URL format: http://m.mobplus.net/c/p/CAMPAIGN_KEY?txid=CLICK_ID

INSERT INTO campaigns (
    slug,
    language,
    country,
    operator,
    offer_product_id,
    pricepoint_id,
    partner_role_id,
    flow_type,
    short_code,
    sms_keyword,
    price,
    billing_cycle,
    terms_url,
    inline_terms_text,
    consent_required,
    consent_version,
    attribution_mapping,
    postback_rules,
    throttles,
    allowed_referrers,
    allowed_sources,
    landing_page_urls,
    enabled,
    created_by
) VALUES (
    -- Campaign identifier (URL-safe slug)
    'gh-airteltigo-mobplus-daily-v1',
    
    -- Language and geography
    'en',
    'GH',           -- Ghana
    'AirtelTigo',   -- Operator
    
    -- Product configuration (TIMWE product mapping)
    8509,           -- offer_product_id: Actual TIMWE product ID
    NULL,           -- pricepoint_id: Set if required by TIMWE
    2117,           -- partner_role_id: TIMWE partner role ID
    
    -- Flow configuration
    'OTP',          -- flow_type: OTP flow for MSISDN verification
    NULL,           -- short_code: Not needed for web flow
    NULL,           -- sms_keyword: Not needed for web flow
    
    -- Pricing (GHS)
    0.20,           -- price: 0.20 GHS daily
    'daily',        -- billing_cycle: daily renewal
    
    -- Compliance
    'https://your-domain.com/terms/gh-airteltigo',  -- terms_url
    'By subscribing, you agree to receive daily content updates at GHS 0.20/day. Text STOP to unsubscribe.',  -- inline_terms_text
    true,           -- consent_required
    '1.0',          -- consent_version
    
    -- ==========================================================================
    -- ATTRIBUTION MAPPING
    -- ==========================================================================
    -- Maps incoming URL parameters to canonical field names.
    -- This allows flexibility in how partners send click IDs.
    -- Common formats: click_id, txid (Mobplus), clickid, cid, subid
    '{
        "click_id": "click_id",
        "txid": "click_id",
        "clickid": "click_id",
        "cid": "click_id",
        "subid": "click_id",
        "transaction_id": "click_id",
        "sub1": "sub1",
        "sub2": "sub2",
        "sub3": "sub3",
        "sub4": "sub4",
        "sub5": "sub5",
        "campaign_id": "campaign_id",
        "offer_id": "offer_id",
        "aff_id": "aff_id",
        "adv_id": "adv_id",
        "pub_id": "pub_id",
        "source": "source"
    }'::jsonb,
    
    -- ==========================================================================
    -- POSTBACK RULES (Mobplus Conversion Tracking)
    -- ==========================================================================
    -- Triggered when charge-success is received from subscription-external
    -- Replace YOUR_MOBPLUS_CAMPAIGN_KEY with actual key from Mobplus
    -- 
    -- Available template variables:
    --   {click_id}       - The ad click ID
    --   {transaction_id} - Acquisition transaction UUID
    --   {campaign_slug}  - Campaign slug
    --   {msisdn_hash}    - SHA256 hash of MSISDN
    --   {payout}         - Charge amount
    --   {event}          - Event type (conversion)
    '{
        "conversion": {
            "mobplus": {
                "method": "GET",
                "url": "http://m.mobplus.net/c/p/YOUR_MOBPLUS_CAMPAIGN_KEY?txid={click_id}&pub_id={pub_id}"
            }
        }
    }'::jsonb,
    
    -- Throttling/rate limits
    '{
        "per_msisdn_per_day": 3,
        "per_ip_per_day": 50,
        "per_ip_per_hour": 10
    }'::jsonb,
    
    -- Security: allowed referrers and sources
    ARRAY['mobplus.net', 'm.mobplus.net', 'your-landing-domain.com'],  -- allowed_referrers
    ARRAY['mobplus', 'direct', 'organic'],                              -- allowed_sources
    
    -- Landing page URLs (for preview and routing)
    ARRAY[
        'https://your-landing-domain.com/lp/gh-airteltigo-mobplus-daily-v1',
        'http://localhost:3000/lp/gh-airteltigo-mobplus-daily-v1'
    ],
    
    -- Enable the campaign
    true,
    'system'
) ON CONFLICT (slug) WHERE tenant_id IS NULL DO UPDATE SET
    language = EXCLUDED.language,
    country = EXCLUDED.country,
    operator = EXCLUDED.operator,
    offer_product_id = EXCLUDED.offer_product_id,
    partner_role_id = EXCLUDED.partner_role_id,
    flow_type = EXCLUDED.flow_type,
    price = EXCLUDED.price,
    billing_cycle = EXCLUDED.billing_cycle,
    terms_url = EXCLUDED.terms_url,
    inline_terms_text = EXCLUDED.inline_terms_text,
    consent_required = EXCLUDED.consent_required,
    consent_version = EXCLUDED.consent_version,
    attribution_mapping = EXCLUDED.attribution_mapping,
    postback_rules = EXCLUDED.postback_rules,
    throttles = EXCLUDED.throttles,
    allowed_referrers = EXCLUDED.allowed_referrers,
    allowed_sources = EXCLUDED.allowed_sources,
    landing_page_urls = EXCLUDED.landing_page_urls,
    enabled = EXCLUDED.enabled,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'system';

-- =============================================================================
-- 2. Verify the campaign was created
-- =============================================================================
SELECT 
    id,
    slug,
    country,
    operator,
    offer_product_id,
    partner_role_id,
    flow_type,
    price,
    billing_cycle,
    enabled,
    postback_rules->>'conversion' IS NOT NULL AS has_conversion_postback,
    array_length(landing_page_urls, 1) AS landing_page_count,
    created_at,
    updated_at
FROM campaigns 
WHERE slug = 'gh-airteltigo-mobplus-daily-v1';

-- =============================================================================
-- 3. Generate a sample click_id for testing
-- =============================================================================
-- In production, click_ids are generated by:
--   A) The /v1/click/out endpoint (server-minted UUID)
--   B) Mobplus tracking link (passed as txid parameter)
--
-- For testing, you can generate a UUID and use it as click_id:

-- Generate a test click_id
DO $$
DECLARE
    test_click_id UUID := gen_random_uuid();
BEGIN
    RAISE NOTICE '';
    RAISE NOTICE '============================================================';
    RAISE NOTICE 'COMPLETE CAMPAIGN CREATED: gh-airteltigo-mobplus-daily-v1';
    RAISE NOTICE '============================================================';
    RAISE NOTICE '';
    RAISE NOTICE 'Sample Test Click ID: %', test_click_id;
    RAISE NOTICE '';
    RAISE NOTICE 'Testing URLs:';
    RAISE NOTICE '  Landing Page: https://your-landing-domain.com/lp/gh-airteltigo-mobplus-daily-v1?click_id=%', test_click_id;
    RAISE NOTICE '  Local Dev:    http://localhost:3000/lp/gh-airteltigo-mobplus-daily-v1?click_id=%', test_click_id;
    RAISE NOTICE '';
    RAISE NOTICE 'Mobplus URL Format (with txid):';
    RAISE NOTICE '  https://your-landing-domain.com/lp/gh-airteltigo-mobplus-daily-v1?txid=%', test_click_id;
    RAISE NOTICE '';
    RAISE NOTICE 'Click-Out Redirect URL (generates server-minted click_id):';
    RAISE NOTICE '  GET /v1/click/out?partner=mobplus&dest=landing_web&campaign=gh-airteltigo-mobplus-daily-v1';
    RAISE NOTICE '';
    RAISE NOTICE 'To verify conversion postback, check postback_outbox after charge:';
    RAISE NOTICE '  SELECT * FROM postback_outbox WHERE transaction_id IN (';
    RAISE NOTICE '    SELECT id FROM acquisition_transactions WHERE click_id = ''%''', test_click_id;
    RAISE NOTICE '  );';
    RAISE NOTICE '';
END $$;

COMMIT;

-- =============================================================================
-- USAGE INSTRUCTIONS
-- =============================================================================
-- 
-- 1. SHARE WITH MOBPLUS:
--    Provide this URL template to Mobplus for their tracking:
--    
--    https://your-landing-domain.com/lp/gh-airteltigo-mobplus-daily-v1?txid={click_id}
--    
--    Where {click_id} is their macro that expands to the Mobplus click ID.
--
-- 2. CLICK-OUT FLOW (SERVER-MINTED CLICK_ID):
--    If you want to generate click_ids yourself before redirecting to Mobplus:
--    
--    a. Configure click-out destination in acquisition-api config
--    b. User hits: GET /v1/click/out?partner=mobplus&dest=mobplus_track&campaign=gh-airteltigo-mobplus-daily-v1
--    c. Server generates UUID click_id, stores in outbound_clicks table
--    d. Sets click_id cookie and redirects to Mobplus tracking URL
--    e. Mobplus redirects back to landing page with same click_id
--
-- 3. CONVERSION POSTBACK:
--    When a user successfully subscribes and charge succeeds:
--    
--    a. subscription-external calls POST /internal/acquisition/charge-success
--    b. acquisition-api marks transaction as CHARGED
--    c. acquisition-api enqueues postback to postback_outbox table
--    d. postback-dispatcher picks up and sends:
--       GET http://m.mobplus.net/c/p/YOUR_CAMPAIGN_KEY?txid={actual_click_id}
--
-- 4. UPDATE MOBPLUS CAMPAIGN KEY:
--    Once you have the actual Mobplus campaign key, update the postback URL:
--    
--    UPDATE campaigns 
--    SET postback_rules = jsonb_set(
--        postback_rules,
--        '{conversion,mobplus,url}',
--        '"http://m.mobplus.net/c/p/ACTUAL_CAMPAIGN_KEY?txid={click_id}"'
--    )
--    WHERE slug = 'gh-airteltigo-mobplus-daily-v1';
--
-- =============================================================================
