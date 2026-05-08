# Acquisition API

The Acquisition API is the central service for web-channel subscription acquisition. It handles campaign configuration, transaction creation, attribution tracking, and postback management.

## Features

- Campaign configuration management
- Transaction lifecycle management
- Universal CTA decisioning (OPEN_SMS, OTP, REDIRECT, SHOW_INSTRUCTIONS)
- Ad provider attribution (Mobplus, generic, extensible)
- Throttling and compliance checks
- Postback queuing for async delivery

## API Endpoints

### Public Endpoints

- `GET /v1/campaigns/{slug}` - Get campaign config
- `GET /v1/campaigns` - List enabled campaigns (public-safe)
- `POST /v1/acquisition/transactions` - Create transaction
- `POST /v1/acquisition/transactions/{id}/confirm` - Confirm transaction (OTP)
- `GET /v1/acquisition/transactions/{id}/status` - Get transaction status
- `POST /v1/callbacks/{telco}` - Receive telco callbacks

### Analytics Endpoints (public)

- `POST /v1/analytics/landing/events` - Record landing page events for funnel reporting

**Request body:**
```json
{
  "event_type": "landing_view",  // landing_view | landing_click | form_submit
  "campaign_slug": "gh-tigo-daily",
  "click_id": "abc123",          // optional
  "ad_provider": "mobplus",      // optional
  "session_id": "session123",    // optional (for deduplication)
  "referrer_domain": "google.com" // optional
}
```

### Admin Endpoints (JWT-protected)

These endpoints require `Authorization: Bearer <access_token>` (see env vars below):

#### Campaign Management

- `GET /v1/admin/campaigns` - List campaigns (enabled + disabled; supports `?enabled=true|false` and `?country=GH`)
- `GET /v1/admin/campaigns/{slug}` - Get full campaign (admin view)
- `POST /v1/admin/campaigns` - Create campaign
- `PUT /v1/admin/campaigns/{slug}` - Update campaign (slug immutable)
- `PATCH /v1/admin/campaigns/{slug}/enabled` - Enable/disable campaign
- `POST /v1/admin/campaign-assets/background/presign` - Generate presigned upload URL for campaign background image

#### Reporting Endpoints

All reporting endpoints support the following query parameters:
- `startDate` (YYYY-MM-DD) - Start date for the report (default: 30 days ago)
- `endDate` (YYYY-MM-DD) - End date for the report (default: today)
- `campaignSlug` (string) - Filter by specific campaign
- `country` (string) - Filter by country

**KPIs:**
- `GET /v1/admin/reports/kpis` - Get aggregated KPIs (views, transactions, subscribed, charged, revenue, conversion rates)

**Acquisition Funnel:**
- `GET /v1/admin/reports/acquisition-funnel` - Get funnel stages with dropoff percentages

**Campaign Performance:**
- `GET /v1/admin/reports/campaign-performance` - Get per-campaign metrics table

**Time Series:**
- `GET /v1/admin/reports/timeseries` - Get time series data for charts
  - Additional param: `interval` (daily | hourly, default: daily)

## Configuration

See `config.yaml` for database and application settings.

### Pending transaction reuse TTL

- `ACQUISITION_PENDING_TRANSACTION_TTL` (optional): Go duration override for reusing `CONFIRM_REQUIRED`/`ACTION_REQUIRED` transactions (default: `10m`).

### Acquisition transaction schema prerequisites

`GET /v1/admin/transactions` selects charge-tracking and HE columns from `acquisition_transactions`.
Before running admin transaction endpoints against a database, ensure these migrations have been applied:

- `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`
- `services/subscription-external/migrations/007_add_charge_tracking_columns.sql`
- `services/subscription-external/migrations/013_he_tracking.sql`

You can verify required columns with:

```sql
SELECT column_name
FROM information_schema.columns
WHERE table_name = 'acquisition_transactions'
  AND column_name IN (
    'charged_at',
    'charge_payout',
    'conversion_postback_sent',
    'he_source',
    'he_msisdn',
    'he_operator'
  )
ORDER BY column_name;
```

### Admin management schema prerequisites

Admin management endpoints rely on:

- `admin_activity_logs`
- `userbase_import_jobs`
- `userbase_import_errors`

On startup, acquisition-api now executes `migrations/add_admin_management_tables.sql` and verifies
these relations exist. If bootstrap fails (for example, DB role lacks `CREATE TABLE`/`CREATE INDEX`),
service startup fails fast.

For pre-provisioned environments, include this migration in your DB rollout:

- `services/acquisition-api/migrations/add_admin_management_tables.sql`

### Admin configuration (environment variables)

- `ADMIN_AUTH0_DOMAIN` (required): Auth0 tenant domain used to validate JWTs.
- `ADMIN_AUTH0_AUDIENCE` (required): Auth0 API audience expected in the JWT `aud` claim.
- `ACQUISITION_ADMIN_CORS_ORIGINS` (optional): comma-separated allowed origins for admin CORS (default: `http://localhost:4200`).

### Campaign background asset storage (optional)

Set these to enable admin background-image uploads with presigned URLs:

- `CAMPAIGN_ASSET_STORAGE_ENABLED=true`
- `CAMPAIGN_ASSET_STORAGE_ENDPOINT` (S3-compatible endpoint, host or URL)
- `CAMPAIGN_ASSET_STORAGE_BUCKET`
- `CAMPAIGN_ASSET_STORAGE_ACCESS_KEY_ID`
- `CAMPAIGN_ASSET_STORAGE_SECRET_ACCESS_KEY`
- `CAMPAIGN_ASSET_STORAGE_USE_SSL` (optional, default `true`)
- `CAMPAIGN_ASSET_STORAGE_REGION` (optional)
- `CAMPAIGN_ASSET_STORAGE_PUBLIC_BASE_URL` (optional CDN/public base URL used for returned `asset_url`)
- `CAMPAIGN_ASSET_STORAGE_KEY_PREFIX` (optional, default `campaign-backgrounds`)
- `CAMPAIGN_ASSET_STORAGE_MAX_UPLOAD_BYTES` (optional, default `2097152`)
- `CAMPAIGN_ASSET_STORAGE_PRESIGN_EXPIRY` (optional Go duration, default `10m`)

## Running

```bash
go run cmd/main.go
```

Or with Docker:
```bash
docker build -t acquisition-api .
docker run -p 8084:8084 acquisition-api
```
