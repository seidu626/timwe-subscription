-- =============================================================================
-- Web Acquisition Campaign Seed Data
-- =============================================================================
-- This file seeds example campaigns with proper postback_rules configuration
-- for ad partners like Mobplus.
--
-- POSTBACK_RULES JSON STRUCTURE
-- -----------------------------
-- The postback_rules column expects a JSON structure where:
--   - Top-level keys are event types: "conversion", "subscribed", etc.
--   - Second-level keys are provider names: "mobplus", "generic", etc.
--   - Each provider entry contains:
--       - method: HTTP method ("GET" or "POST")
--       - url: URL template with {variable} placeholders
--       - headers: Optional map of HTTP headers
--       - body: Optional JSON body template for POST requests
--
-- Available URL template variables:
--   {click_id}       - The ad click ID (canonical from landing page)
--   {transaction_id} - The acquisition transaction UUID
--   {campaign_slug}  - The campaign slug
--   {msisdn_hash}    - SHA256 hash of the MSISDN (privacy-safe)
--   {payout}         - The charge payout amount (if available)
--   {event}          - The event type (conversion, subscribed, etc.)
--   {pub_id}         - Publisher ID (if provided in attribution data)
--
-- Example postback_rules for Mobplus:
-- {
--   "conversion": {
--     "mobplus": {
--       "method": "GET",
--       "url": "http://m.mobplus.net/c/p/YOUR_CAMPAIGN_KEY?txid={click_id}"
--     }
--   }
-- }
--
-- ATTRIBUTION_MAPPING JSON STRUCTURE
-- ----------------------------------
-- Maps incoming URL parameters to canonical attribution fields.
-- Format: { "canonical_name": "incoming_param_name", ... }
--
-- Common mappings for Mobplus:
-- {
--   "click_id": "click_id",   -- Primary click ID
--   "txid": "click_id",       -- Alias for click_id (Mobplus uses txid)
--   "clickid": "click_id",    -- Another common alias
--   "sub1": "sub1",
--   "sub2": "sub2",
--   "sub3": "sub3",
--   "campaign_id": "campaign_id",
--   "offer_id": "offer_id",
--   "aff_id": "aff_id"
-- }
-- =============================================================================

-- Example 1: Mobplus Campaign (Ghana / AirtelTigo)
-- Replace YOUR_MOBPLUS_CAMPAIGN_KEY with the actual key from Mobplus
INSERT INTO campaigns (
    slug, language, country, operator,
    offer_product_id, pricepoint_id, partner_role_id,
    flow_type, short_code, sms_keyword,
    price, billing_cycle,
    terms_url, consent_required, consent_version,
    attribution_mapping, postback_rules,
    throttles, enabled
) VALUES (
    'gh-tigo-mobplus-daily',
    'en',
    'GH',
    'AirtelTigo',
    8509,  -- Adjust to actual TIMWE product ID
    NULL,
    2117,  -- Adjust to actual partner role ID
    'OTP',
    NULL,
    NULL,
    5.00,
    'daily',
    'https://your-domain.com/terms/gh-tigo',
    true,
    '1.0',
    -- Attribution mapping: maps incoming params to canonical names
    '{
        "click_id": "click_id",
        "txid": "click_id",
        "clickid": "click_id",
        "cid": "click_id",
        "subid": "click_id",
        "sub1": "sub1",
        "sub2": "sub2",
        "sub3": "sub3",
        "sub4": "sub4",
        "sub5": "sub5",
        "campaign_id": "campaign_id",
        "offer_id": "offer_id",
        "aff_id": "aff_id",
        "adv_id": "adv_id"
    }'::jsonb,
    -- Postback rules: triggers on charge success (conversion event)
    '{
        "conversion": {
            "mobplus": {
                "method": "GET",
                "url": "http://m.mobplus.net/c/p/YOUR_MOBPLUS_CAMPAIGN_KEY?txid={click_id}&pub_id={pub_id}"
            }
        }
    }'::jsonb,
    '{"per_msisdn_per_day": 3, "per_ip_per_day": 10}'::jsonb,
    true
) ON CONFLICT (slug) WHERE tenant_id IS NULL DO UPDATE SET
    attribution_mapping = EXCLUDED.attribution_mapping,
    postback_rules = EXCLUDED.postback_rules,
    updated_at = CURRENT_TIMESTAMP;

-- Example 2: Generic Test Campaign (for development/testing)
INSERT INTO campaigns (
    slug, language, country, operator,
    offer_product_id, pricepoint_id, partner_role_id,
    flow_type, short_code, sms_keyword,
    price, billing_cycle,
    terms_url, consent_required, consent_version,
    attribution_mapping, postback_rules,
    throttles, enabled
) VALUES (
    'test-campaign',
    'en',
    'GH',
    'AirtelTigo',
    8509,
    NULL,
    2117,
    'OTP',
    NULL,
    NULL,
    5.00,
    'daily',
    'https://example.com/terms',
    true,
    '1.0',
    '{
        "click_id": "click_id",
        "txid": "click_id",
        "sub1": "sub1",
        "sub2": "sub2",
        "sub3": "sub3"
    }'::jsonb,
    -- Generic postback (uses httpbin for testing)
    '{
        "conversion": {
            "generic": {
                "method": "GET",
                "url": "https://httpbin.org/get?click_id={click_id}&event={event}&campaign={campaign_slug}"
            }
        }
    }'::jsonb,
    '{"per_msisdn_per_day": 3, "per_ip_per_day": 10}'::jsonb,
    true
) ON CONFLICT (slug) WHERE tenant_id IS NULL DO UPDATE SET
    attribution_mapping = EXCLUDED.attribution_mapping,
    postback_rules = EXCLUDED.postback_rules,
    updated_at = CURRENT_TIMESTAMP;

-- Example 3: PIN flow campaign (no OTP required)
INSERT INTO campaigns (
    slug, language, country, operator,
    offer_product_id, pricepoint_id, partner_role_id,
    flow_type, short_code, sms_keyword,
    price, billing_cycle,
    terms_url, consent_required, consent_version,
    attribution_mapping, postback_rules,
    throttles, enabled
) VALUES (
    'gh-tigo-pin-daily',
    'en',
    'GH',
    'AirtelTigo',
    8509,
    NULL,
    2117,
    'PIN',  -- PIN flow - no OTP verification
    NULL,
    NULL,
    5.00,
    'daily',
    'https://your-domain.com/terms/gh-tigo',
    true,
    '1.0',
    '{
        "click_id": "click_id",
        "txid": "click_id",
        "sub1": "sub1"
    }'::jsonb,
    '{
        "conversion": {
            "mobplus": {
                "method": "GET",
                "url": "http://m.mobplus.net/c/p/YOUR_MOBPLUS_CAMPAIGN_KEY?txid={click_id}&pub_id={pub_id}"
            }
        }
    }'::jsonb,
    '{"per_msisdn_per_day": 5, "per_ip_per_day": 20}'::jsonb,
    false  -- Disabled by default - enable when ready
) ON CONFLICT (slug) WHERE tenant_id IS NULL DO UPDATE SET
    attribution_mapping = EXCLUDED.attribution_mapping,
    postback_rules = EXCLUDED.postback_rules,
    flow_type = EXCLUDED.flow_type,
    updated_at = CURRENT_TIMESTAMP;
