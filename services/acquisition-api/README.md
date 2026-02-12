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

### Admin configuration (environment variables)

- `ADMIN_AUTH0_DOMAIN` (required): Auth0 tenant domain used to validate JWTs.
- `ADMIN_AUTH0_AUDIENCE` (required): Auth0 API audience expected in the JWT `aud` claim.
- `ACQUISITION_ADMIN_CORS_ORIGINS` (optional): comma-separated allowed origins for admin CORS (default: `http://localhost:4200`).

## Running

```bash
go run cmd/main.go
```

Or with Docker:
```bash
docker build -t acquisition-api .
docker run -p 8084:8084 acquisition-api
```
