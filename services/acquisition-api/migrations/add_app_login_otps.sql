-- Dayline app login OTP storage.
--
-- This is a DISTINCT credential from the TIMWE billing opt-in PIN
-- (acquisition_transactions.transaction_auth_code / TIMWE confirm flow).
-- It authenticates the Dayline mobile app session (POST /v1/app/auth/otp/*)
-- and must never share tables or copy with the billing PIN flow.
--
-- code_hash is SHA-256(code_salt || code); the plaintext code is never stored.
CREATE TABLE IF NOT EXISTS app_login_otps (
    id          SERIAL PRIMARY KEY,
    msisdn      VARCHAR(20) NOT NULL,
    tenant_key  VARCHAR(120) NOT NULL,
    code_hash   VARCHAR(64) NOT NULL,
    code_salt   VARCHAR(32) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    attempts    INT NOT NULL DEFAULT 0,
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_login_otps_lookup
    ON app_login_otps (msisdn, tenant_key, created_at DESC);
