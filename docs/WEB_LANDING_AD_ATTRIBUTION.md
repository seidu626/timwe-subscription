# Web Landing Pages + Ad Attribution Integration

## Overview

This document describes the web-channel acquisition system that enables users to subscribe via landing pages with full ad provider attribution and postback tracking.

## Architecture

The system consists of three main components:

1. **Landing Web** (Next.js): Static-first landing pages served from CDN
2. **Acquisition API** (Go): Centralized subscription logic and transaction management
3. **Postback Dispatcher** (Go Worker): Async delivery of conversion postbacks to ad providers

## Campaign Model

### Campaign Configuration

Campaigns are stored in the `campaigns` table and define:

- **Basic Info**: `slug`, `language`, `country`, `operator`
- **Product Mapping**: `offer_product_id`, `pricepoint_id`, `partner_role_id`
- **Flow Type**: `CLICK_TO_SMS`, `OTP`, `REDIRECT`, or `MIXED`
- **Pricing**: `price`, `billing_cycle`, `trial_flags`
- **Compliance**: `terms_url`, `inline_terms_text`, `consent_required`, `consent_version`
- **Attribution**: `attribution_mapping` (JSON mapping provider params to canonical fields)
- **Postback Rules**: `postback_rules` (JSON mapping events to provider URLs)
- **Throttles**: `throttles` (JSON with `per_msisdn_per_day`, `per_ip_per_day` limits)
- **Access Control**: `allowed_referrers`, `allowed_sources`
- **Landing Page Binding**: `landing_page_urls` (array of absolute URLs; optional; allows one campaign to be bound to multiple LP domains/paths)

### Example Campaign

```sql
INSERT INTO campaigns (
    slug, language, country, offer_product_id,
    flow_type, price, billing_cycle,
    consent_required, attribution_mapping, postback_rules,
    throttles, enabled
) VALUES (
    'summer-promo',
    'en',
    'GH',
    8509,
    'OTP',
    5.00,
    'daily',
    true,
    '{"click_id": "click_id", "txid": "click_id"}'::jsonb,
    '{"subscribed": {"mobplus": "http://m.mobplus.net/c/p/{campaignKey}?txid={click_id}"}}'::jsonb,
    '{"per_msisdn_per_day": 3, "per_ip_per_day": 10}'::jsonb,
    true
);
```

## URL Parameters Mapping

### Landing Page URL Format

```
/lp/{campaignSlug}?provider={provider}&click_id={clickId}&sub1={sub1}&sub2={sub2}
```

### Supported Providers

- **mobplus**: Uses `click_id` or `txid` parameter
- **generic**: Uses `click_id` parameter

### Attribution Normalization

The system normalizes provider-specific parameters to a canonical format:

- `provider`: Ad provider identifier (mobplus, generic, etc.)
- `click_id`: Normalized click identifier
- `sub1`, `sub2`, `sub3`: Additional tracking parameters
- `campaign_slug`: Internal campaign identifier

## API Endpoints

### Campaign Endpoints

- `GET /v1/campaigns/{slug}` - Get public campaign config for rendering
- `GET /v1/campaigns` - List all enabled campaigns

### Transaction Endpoints

- `POST /v1/acquisition/transactions` - Create new subscription transaction
  ```json
  {
    "campaign_slug": "summer-promo",
    "msisdn": "233241234567",
    "provider": "mobplus",
    "click_id": "abc123",
    "attribution_data": {"sub1": "value1"},
    "consent_checked": true
  }
  ```

- `POST /v1/acquisition/transactions/{id}/confirm` - Confirm transaction (OTP flow)
  ```json
  {
    "transaction_id": "uuid",
    "auth_code": "123456"
  }
  ```

- `GET /v1/acquisition/transactions/{id}/status` - Get transaction status

### Callback Endpoints

- `POST /v1/callbacks/{telco}` - Receive telco confirmation callbacks
  ```json
  {
    "msisdn": "233241234567",
    "transaction_id": "timwe-tx-id",
    "status": "DELIVERED"
  }
  ```

## Universal CTA Flow

The Acquisition API returns a `next_action` field indicating the next step:

### Next Actions

1. **OPEN_SMS**: User should open SMS app
   ```json
   {
     "next_action": "OPEN_SMS",
     "payload": {
       "sms_link": "sms:0000?body=OK",
       "short_code": "0000",
       "keyword": "OK",
       "fallback_steps": ["Step 1", "Step 2"]
     }
   }
   ```

2. **OTP**: User needs to enter OTP
   ```json
   {
     "next_action": "OTP",
     "payload": {
       "transaction_id": "uuid",
       "prompt": "Enter the code sent to your phone"
     }
   }
   ```

3. **REDIRECT**: User should be redirected
   ```json
   {
     "next_action": "REDIRECT",
     "payload": {
       "url": "https://..."
     }
   }
   ```

4. **SHOW_INSTRUCTIONS**: Show success/failure message
   ```json
   {
     "next_action": "SHOW_INSTRUCTIONS",
     "payload": {
       "message": "Subscription successful!"
     }
   }
   ```

## Ad Provider Integration

### Mobplus Provider

**Normalization**:
- Maps `click_id` or `txid` → canonical `click_id`

**Postback Format**:
```
POST http://m.mobplus.net/c/p/{campaignKey}?txid={clickId}
```

### Generic Provider

**Normalization**:
- Maps `click_id` → canonical `click_id`

**Postback Format**:
```
GET https://example.com/postback?click_id={click_id}&event={event}
```

### Adding New Providers

1. Implement `AdProvider` interface in `internal/service/ad_provider.go`
2. Register in `ProviderRegistry`
3. Configure in campaign `postback_rules`

## Postback Delivery

Postbacks are queued in `postback_outbox` and processed asynchronously by the `postback-dispatcher` worker.

### Retry Logic

- Exponential backoff: 2^attempt seconds (max 1 hour)
- Max attempts: 5 (configurable per outbox entry)
- Circuit breaker: Opens after 10 consecutive failures

### Postback Events

- `subscribed`: User successfully subscribed
- `failed`: Subscription failed
- `cancelled`: Subscription cancelled

## Transaction Lifecycle

1. **PENDING**: Transaction created, awaiting action
2. **ACTION_REQUIRED**: User action needed (e.g., send SMS)
3. **CONFIRM_REQUIRED**: OTP confirmation needed
4. **SUBSCRIBED**: Successfully subscribed
5. **FAILED**: Subscription failed
6. **CANCELLED**: Subscription cancelled

## Compliance & Throttling

### Consent Tracking

- `consent_required`: Campaign requires consent
- `consent_checked`: User checked consent box
- `consent_version`: Version of terms accepted
- `consent_timestamp`: When consent was given

### Throttling

Configured per campaign in `throttles` JSON:

```json
{
  "per_msisdn_per_day": 3,
  "per_ip_per_day": 10
}
```

### Source Controls

- `allowed_referrers`: Array of allowed referrer domains
- `allowed_sources`: Array of allowed traffic sources

## Deployment

### Services

1. **acquisition-api**: Port 8084
2. **landing-web**: Port 3000 (Next.js)
3. **postback-dispatcher**: Background worker

### KrakenD Routes

All routes are exposed through KrakenD gateway:

- `/lp/{slug}` → landing-web
- `/v1/campaigns/*` → acquisition-api
- `/v1/acquisition/*` → acquisition-api
- `/v1/callbacks/*` → acquisition-api

### Database Migration

Run migration:
```bash
psql -d subscription_manager -f services/subscription-external/migrations/006_web_acquisition_campaigns.sql
```

Seed sample campaign:
```bash
psql -d subscription_manager -f services/acquisition-api/migrations/seed_campaign.sql
```

## Testing

### Test Campaign URL

```
http://localhost:8080/lp/test-campaign?provider=mobplus&click_id=test123
```

### Test Transaction Creation

```bash
curl -X POST http://localhost:8080/v1/acquisition/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_slug": "test-campaign",
    "msisdn": "233241234567",
    "provider": "mobplus",
    "click_id": "test123",
    "consent_checked": true
  }'
```

## Operational Runbook

### Monitoring

- Check `acquisition_transactions` table for transaction status
- Check `postback_outbox` for pending postbacks
- Check `postback_attempts` for delivery history

### Troubleshooting

1. **Postbacks not sending**: Check `postback_outbox.status` and `postback_attempts`
2. **Transactions stuck**: Check `acquisition_transactions.status` and TIMWE integration
3. **Throttling issues**: Review `throttles` config and transaction counts

### Manual Postback Retry

```sql
UPDATE postback_outbox
SET status = 'PENDING', next_retry_at = CURRENT_TIMESTAMP
WHERE id = 'outbox-id';
```

## Outbound Click-ID Redirect (click-out)

Some partners expect *us* to generate the click_id before we redirect traffic to them, so they can later send conversion postbacks referencing the same click_id.

### Endpoint

- `GET /v1/click/out` (public)

### Behavior

- Validates `dest` against an allowlist (prevents open redirect abuse)
- Generates a server-side `click_id` (UUID)
- Persists a row in `outbound_clicks`
- Sets first-party cookies: `click_id`, `click_partner`
- Redirects (302) to the allowlisted destination with the click id appended using the destination’s configured `click_id_param`

### Configuration

Destinations are configured via environment variable `CLICKOUT_DESTINATIONS_JSON` as a JSON map:

```json
{
  "partnerA": {
    "base_url": "https://partner.example.com/click",
    "click_id_param": "click_id",
    "passthrough_params": ["sub1", "sub2"]
  },
  "landing_web": {
    "base_url": "https://landing.example.com/lp/test-campaign",
    "click_id_param": "click_id",
    "passthrough_params": ["utm_source", "utm_medium", "utm_campaign"]
  }
}
```

Ad provider normalization is enabled via `AD_PROVIDERS` (comma-separated list; default: `generic`).
