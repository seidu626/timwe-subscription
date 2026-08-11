-- Admit the two additional landing-page subscription modes:
--   DOUBLE_OPTIN - provider holds the subscription pre-active and the user
--                  confirms with a button press (confirm carries no auth code)
--   AUTO         - no confirmation step; requires a provider product configured
--                  as single-type opt-in
-- The four pre-existing values are unchanged and remain in live use.

SET lock_timeout = '10s';
SET statement_timeout = '10min';

ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_flow_type_check;

ALTER TABLE campaigns
    ADD CONSTRAINT campaigns_flow_type_check
    CHECK (flow_type IN ('CLICK_TO_SMS', 'OTP', 'REDIRECT', 'MIXED', 'DOUBLE_OPTIN', 'AUTO'));
