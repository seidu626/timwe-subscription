# Web Landing Pages + Ad Attribution Implementation Summary

## Overview

Successfully implemented a complete web-channel acquisition system with landing pages, ad provider attribution, and postback tracking. The system follows the recommended two-part architecture: **Landing Runtime (Next.js)** + **Acquisition API (Go)**.

## Components Implemented

### 1. Acquisition API (`services/acquisition-api/`)

**Core Features:**
- Campaign configuration management
- Transaction lifecycle management (PENDING → ACTION_REQUIRED → CONFIRM_REQUIRED → SUBSCRIBED/FAILED)
- Universal CTA decisioning (OPEN_SMS, OTP, REDIRECT, SHOW_INSTRUCTIONS)
- Ad provider attribution (Mobplus + generic, extensible)
- Throttling and compliance checks
- Postback queuing

**Key Files:**
- `internal/domain/` - Campaign, Transaction, Postback domain models
- `internal/service/` - Transaction service, Campaign service, Ad provider registry
- `internal/repository/` - Campaign, Transaction, Postback repositories
- `internal/handler/` - HTTP handlers for campaigns, transactions, callbacks
- `internal/transport/router.go` - FastHTTP routing
- `cmd/main.go` - Service entry point

**API Endpoints:**
- `GET /v1/campaigns/{slug}` - Get campaign config
- `POST /v1/acquisition/transactions` - Create transaction
- `POST /v1/acquisition/transactions/{id}/confirm` - Confirm (OTP)
- `GET /v1/acquisition/transactions/{id}/status` - Get status
- `POST /v1/callbacks/{telco}` - Telco callbacks

### 2. Landing Web (`services/landing-web/`)

**Core Features:**
- Next.js 14 with App Router
- Campaign config-driven rendering
- Universal CTA flow support
- Static-first, CDN-friendly
- Minimal JavaScript

**Key Files:**
- `app/lp/[slug]/page.tsx` - Dynamic landing page route
- `app/layout.tsx` - Root layout
- `next.config.js` - Next.js configuration

**Routes:**
- `GET /lp/{slug}` - Render landing page for campaign

### 3. Postback Dispatcher (`services/postback-dispatcher/`)

**Core Features:**
- Async postback delivery from outbox queue
- Exponential backoff retry (2^attempt seconds, max 1 hour)
- Circuit breaker (opens after 10 consecutive failures)
- Comprehensive attempt logging
- DLQ for failed postbacks (after max attempts)

**Key Files:**
- `cmd/main.go` - Worker entry point with polling loop

### 4. Database Schema

**Migration:** `services/subscription-external/migrations/006_web_acquisition_campaigns.sql`

**Tables Created:**
- `campaigns` - Campaign configuration
- `landing_versions` - Landing page versioning (compliance)
- `acquisition_transactions` - Transaction lifecycle tracking
- `consents` - Immutable consent records
- `postback_outbox` - Queued postbacks
- `postback_attempts` - Postback delivery history

### 5. Ad Provider System

**Provider Interface:**
- `AdProvider.Normalize()` - Convert provider params to canonical format
- `AdProvider.BuildPostback()` - Construct postback HTTP request

**Built-in Providers:**
- **Mobplus**: Maps `click_id`/`txid` → canonical, POST postback format
- **Generic**: Fallback provider for testing

**Extensibility:** New providers can be added by implementing the interface and registering in `ProviderRegistry`.

### 6. KrakenD Gateway Integration

**Templates Added:**
- `LandingWebEndpoint.tmpl` - Landing page routes
- `AcquisitionApiEndpoint.tmpl` - Acquisition API routes

**Routes Added:**
- `/lp/{slug}` → landing-web:3000
- `/v1/campaigns/*` → acquisition-api:8084
- `/v1/acquisition/*` → acquisition-api:8084
- `/v1/callbacks/*` → acquisition-api:8084

### 7. Docker Compose Integration

**Services Added:**
- `acquisition-api` (port 8084)
- `landing-web` (port 3000)
- `postback-dispatcher` (background worker)

**Dockerfiles Created:**
- `services/acquisition-api/Dockerfile`
- `services/postback-dispatcher/Dockerfile`
- `services/landing-web/Dockerfile`

### 8. Documentation

**Files Created:**
- `docs/WEB_LANDING_AD_ATTRIBUTION.md` - Complete integration guide
- `services/acquisition-api/README.md`
- `services/postback-dispatcher/README.md`
- `services/landing-web/README.md`

## Key Design Decisions

1. **Provider-Agnostic Architecture**: Mobplus is treated as one ad provider among many, not a special case
2. **Universal CTA Flow**: Single flow supports multiple mechanisms (SMS, OTP, Redirect, Instructions)
3. **Async Postback Delivery**: Postbacks queued in DB outbox, processed by background worker
4. **Compliance Built-In**: Consent tracking, throttling, source controls at campaign level
5. **Static-First Landing Pages**: Next.js with minimal JS for CDN deployment

## Next Steps

1. **Run Database Migration:**
   ```bash
   psql -d subscription_manager -f services/subscription-external/migrations/006_web_acquisition_campaigns.sql
   ```

2. **Seed Sample Campaign:**
   ```bash
   psql -d subscription_manager -f services/acquisition-api/migrations/seed_campaign.sql
   ```

3. **Build Services:**
   ```bash
   make build-acquisition-api
   make build-postback-dispatcher
   cd services/landing-web && npm install && npm run build
   ```

4. **Start Services:**
   ```bash
   docker-compose up acquisition-api landing-web postback-dispatcher
   ```

5. **Test Landing Page:**
   ```
   http://localhost:8080/lp/test-campaign?provider=mobplus&click_id=test123
   ```

## Integration Notes

- **TIMWE Client**: Currently a stub implementation. Replace `TIMWEClientImpl` in `services/acquisition-api/internal/service/timwe_client.go` with actual TIMWE API integration (can reuse code from `subscription-external` service).
- **Campaign Configuration**: Campaigns are stored in DB. Consider adding admin UI in `frontend/webspa-admin` for campaign management.
- **Postback Templates**: Mobplus postback URL includes hardcoded campaign key. Should be configurable per campaign in `postback_rules` JSON.

## Testing Checklist

- [ ] Database migration runs successfully
- [ ] Campaign seed creates test campaign
- [ ] Acquisition API starts and responds to health check
- [ ] Landing web serves pages at `/lp/{slug}`
- [ ] Transaction creation works end-to-end
- [ ] OTP confirmation flow works
- [ ] Postback dispatcher processes queue
- [ ] KrakenD routes work correctly
- [ ] Mobplus attribution normalization works
- [ ] Throttling prevents duplicate requests
