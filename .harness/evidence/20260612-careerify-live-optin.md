# Careerify Live Opt-In Evidence

Date: 2026-06-12
Tenant: `careerify`
Channel: `web-gh-airteltigo`
Product: `32535`
MSISDN: `23357*****30`

## Result

The local `subscription-external` partner opt-in route reached TIMWE but failed with provider response `NOT AUTHORIZED TO USE THE API` because that path selected the Careerify `mt_api_key` for `/subscription/optin/2117`.

Evidence:
- `.harness/logs/20260612-careerify-live-optin.summary`
- `.harness/logs/20260612-careerify-live-optin-subext-log-excerpt.redacted.txt`

A direct TIMWE subscription opt-in call was then sent using the Careerify subscription API key from the active runtime credential, with the app-equivalent payload shape and redacted evidence.

Provider result:
- HTTP status: `200`
- Code: `SUCCESS`
- Subscription result: `OPTIN_PREACTIVE_WAIT_CONF`
- Subscription error: `Preactive and Wait Confirmation`

Evidence:
- `.harness/logs/20260612-careerify-direct-optin.summary`
- `.harness/logs/20260612-careerify-direct-optin-WEB.response.redacted.json`

## Follow-Up

Do not run confirm until the confirmation code is available.
The local app route needs a small key-selection fix or credential split before it can run this exact opt-in through `/api/v1/subscription-external/partners/optin` with the subscription API key.
