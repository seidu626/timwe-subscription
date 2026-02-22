-- Migration: Add lp_copy configuration to campaigns
-- Stores landing-page copy per campaign (EN required, AR optional)

ALTER TABLE campaigns
ADD COLUMN IF NOT EXISTS lp_copy JSONB;

UPDATE campaigns
SET lp_copy = '{
  "en": {
    "heroTitle": "Subscribe to unlock premium content.",
    "heDescription": "To continue, tap Subscribe.",
    "heCta": "Subscribe",
    "heModalTitle": "Almost there. Please confirm to continue.",
    "heModalConfirm": "Confirm",
    "msisdnDescription": "Enter your mobile number to receive your PIN code.",
    "msisdnPlaceholder": "Mobile number (9 digits)",
    "msisdnCta": "Subscribe",
    "otpDescription": "Enter the 4-digit PIN sent to your phone.",
    "otpPlaceholder": "4-digit PIN",
    "otpCta": "Confirm",
    "successTitle": "Subscription successful",
    "successBody": "You will receive a text message with your access details.",
    "consentPrefix": "I agree to the",
    "consentTerms": "Terms and Conditions",
    "termsHeading": "Terms and Conditions",
    "legal": "Your subscription renews automatically until cancelled. You must be 18+ years old or have parental permission to use this service.",
    "phoneRequired": "Phone number is required.",
    "phoneInvalid": "Enter a valid 9-digit mobile number.",
    "otpInvalid": "PIN must be exactly 4 digits.",
    "consentRequired": "You must accept terms to continue."
  }
}'::jsonb
WHERE lp_copy IS NULL;

ALTER TABLE campaigns
ALTER COLUMN lp_copy SET DEFAULT '{
  "en": {
    "heroTitle": "Subscribe to unlock premium content.",
    "heDescription": "To continue, tap Subscribe.",
    "heCta": "Subscribe",
    "heModalTitle": "Almost there. Please confirm to continue.",
    "heModalConfirm": "Confirm",
    "msisdnDescription": "Enter your mobile number to receive your PIN code.",
    "msisdnPlaceholder": "Mobile number (9 digits)",
    "msisdnCta": "Subscribe",
    "otpDescription": "Enter the 4-digit PIN sent to your phone.",
    "otpPlaceholder": "4-digit PIN",
    "otpCta": "Confirm",
    "successTitle": "Subscription successful",
    "successBody": "You will receive a text message with your access details.",
    "consentPrefix": "I agree to the",
    "consentTerms": "Terms and Conditions",
    "termsHeading": "Terms and Conditions",
    "legal": "Your subscription renews automatically until cancelled. You must be 18+ years old or have parental permission to use this service.",
    "phoneRequired": "Phone number is required.",
    "phoneInvalid": "Enter a valid 9-digit mobile number.",
    "otpInvalid": "PIN must be exactly 4 digits.",
    "consentRequired": "You must accept terms to continue."
  }
}'::jsonb,
ALTER COLUMN lp_copy SET NOT NULL;

COMMENT ON COLUMN campaigns.lp_copy IS 'Landing-page copy blob. Expected shape: {"en": {...}, "ar": {...optional...}}';

CREATE INDEX IF NOT EXISTS idx_campaigns_lp_copy
ON campaigns USING GIN (lp_copy);
