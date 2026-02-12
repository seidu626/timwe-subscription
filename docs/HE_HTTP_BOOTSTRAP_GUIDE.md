# HTTP Header Enrichment Bootstrap Guide

This guide explains how to handle HTTP-only operator Header Enrichment (HE) when your application runs on HTTPS.

## Problem Statement

Ghana MNOs inject `X-MSISDN` / `X-UP-CALLING-LINE-ID` headers **only on HTTP** requests. However, our application stack redirects all HTTP traffic to HTTPS via NGINX:

```
HTTP request (with HE headers) → NGINX 301 → HTTPS request (HE headers LOST)
```

The browser's follow-up HTTPS request is a **fresh request** that does not carry the operator-injected headers.

## Operator-Mapped Hostnames

Operator HE is currently mapped to **three HTTP hostnames**:

| HTTP Host | HTTPS Redirect Target | Backend |
|-----------|----------------------|---------|
| `http://api.nouveauricheglobalgroup.com` | `https://api.nouveauricheglobalgroup.com` | landing-web for `/lp/*`, `/c/*`, KrakenD for API |
| `http://landing.nouveauricheglobalgroup.com` | `https://landing.nouveauricheglobalgroup.com` | landing-web |
| `http://lp.nouveauricheglobalgroup.com` | `https://lp.nouveauricheglobalgroup.com` | landing-web |

**Same-host redirect**: After HE capture, the browser is redirected to HTTPS on the **same host** as the incoming request. This ensures the user journey stays consistent with where they started.

## Canonical vs Alias Paths

- **Canonical landing path**: `/lp/:slug` (served by `landing-web`)
- **Alias path**: `/c/:slug` → redirects to `/lp/:slug` (backward compatibility)

HTTP HE capture uses `/c/:slug` as the entry point. After capturing HE headers and minting a token, the bootstrap handler redirects to the **canonical path** `/lp/:slug?he_token=...`.

## Solution: HTTP Bootstrap + Token Handoff

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Mobile Browser (on operator data network)                                   │
└─────────────────────────────────────────────────────────────────────────────┘
                │
                │ 1. User clicks ad link: http://{api|landing|lp}.*/c/campaign
                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Operator Proxy (adds X-MSISDN header)                                       │
└─────────────────────────────────────────────────────────────────────────────┘
                │
                │ 2. Request with HE headers reaches NGINX port 80
                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  NGINX (port 80) - for api.*, landing.*, or lp.*                            │
│  ├─ /he/bootstrap → proxy to acquisition-api                                │
│  ├─ /c/{slug}     → proxy to acquisition-api (campaign bootstrap)           │
│  └─ everything else → 301 redirect to HTTPS (same host)                     │
└─────────────────────────────────────────────────────────────────────────────┘
                │
                │ 3. HE bootstrap captures MSISDN, mints token
                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  acquisition-api: HE Bootstrap Handler                                       │
│  - Validates source IP ∈ operator CIDRs                                      │
│  - Extracts MSISDN from X-MSISDN / X-UP-CALLING-LINE-ID                      │
│  - Creates single-use token (stored in Redis, TTL 60s)                       │
│  - Returns 302 redirect to /lp/:slug?he_token=... (same host, canonical path)│
└─────────────────────────────────────────────────────────────────────────────┘
                │
                │ 4. Browser redirects to HTTPS /lp/:slug?he_token=...
                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  NGINX (port 443) → landing-web for /lp/*, /c/*, /_next/*; KrakenD for API  │
└─────────────────────────────────────────────────────────────────────────────┘
                │
                │ 5. Landing page captures he_token, exchanges server-side
                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  landing-web: Transaction Creation                                           │
│  - Captures he_token from URL (stores in sessionStorage)                     │
│  - Sends X-HE-Bootstrap-Token header to /api/transactions                    │
│  - Server-side: exchanges token for HE identity via acquisition-api         │
│  - Attaches X-He-* headers to backend transaction call                       │
│  - Subscription flow continues with HE identity                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Special Routing for `api.*` HTTPS

When the HE bootstrap redirects to `https://api.nouveauricheglobalgroup.com/lp/...`:

- `/lp/*` routes → `landing-web` (canonical campaign landing pages)
- `/c/*` routes → `landing-web` (alias, redirects to `/lp/*`)
- `/_next/*` routes → `landing-web` (Next.js static assets)
- `/favicon.ico`, `/robots.txt` → `landing-web`
- All other routes → KrakenD (API gateway, as usual)

This allows the API host to serve landing pages for HE flows while maintaining its primary API gateway function.

---

## 1. Operator Proxy CIDR Configuration (REQUIRED)

### What You Need From the Operator

Contact each Ghana MNO and request:

1. **Source IP ranges (CIDRs)** of their HE proxy servers
2. **Header names** they inject (confirm: `X-MSISDN`, `X-UP-CALLING-LINE-ID`, others?)
3. **IP allowlisting requirements** (some operators only enrich traffic to allowlisted destinations)

### Expected Format

| Operator | CIDRs | Header Names | Notes |
|----------|-------|--------------|-------|
| MTN Ghana (620-01) | `TBD` | X-MSISDN, X-UP-CALLING-LINE-ID | Request from MTN integration team |
| Telecel Ghana (620-02) | `TBD` | X-MSISDN, X-UP-CALLING-LINE-ID | Request from Telecel/Vodafone team |
| AT Ghana (620-03, 620-06) | `TBD` | X-MSISDN, X-UP-CALLING-LINE-ID | Request from AirtelTigo/AT team |

### Configuration

Once obtained, configure in environment:

```bash
# Comma-separated list of operator proxy CIDRs
# Example (placeholder - replace with real values from operators):
HE_TRUSTED_PROXY_CIDRS=41.223.128.0/18,197.210.0.0/16,102.176.0.0/16

# For NGINX geo module (ops/nginx/conf.d/he-bootstrap.conf)
# Each CIDR needs to be added to the geo block
```

### Temporary: Logging Mode for Discovery

If CIDRs are unknown, deploy in "logging mode" to discover them:

1. Set `HE_BOOTSTRAP_LOG_ONLY=true`
2. Allow all IPs temporarily (development only)
3. Test with real devices on each operator network
4. Capture `X-Real-IP` / `X-Forwarded-For` from requests that have HE headers
5. Work with operators to confirm these are their proxy ranges

---

## 2. Environment Variables

Add to your deployment environment:

```bash
# ============================================
# Header Enrichment Bootstrap Configuration
# ============================================

# Trusted operator proxy CIDRs (comma-separated)
# SECURITY: Only requests from these IPs will have HE headers trusted
HE_TRUSTED_PROXY_CIDRS=

# Bootstrap token configuration
HE_BOOTSTRAP_TOKEN_TTL=60       # seconds, default 60
HE_BOOTSTRAP_TOKEN_SECRET=      # 32+ byte random secret for signing

# Redis configuration (for token storage)
# Uses existing REDIS_HOST, REDIS_PORT from main config

# Logging mode (for CIDR discovery)
HE_BOOTSTRAP_LOG_ONLY=false     # Set true to log without enforcing CIDRs
```

---

## 3. Security Requirements (Non-Negotiable)

### Edge (NGINX) Requirements

1. **IP Allowlist**: Only proxy HE bootstrap path for requests from operator CIDRs
2. **Strip Headers**: Remove `X-MSISDN`, `X-UP-CALLING-LINE-ID` from all other traffic
3. **Rate Limit**: Limit bootstrap endpoint to prevent abuse

### Application Requirements

1. **Double-Check Source IP**: Even if NGINX allows, verify IP at application layer
2. **Single-Use Tokens**: Delete token immediately after exchange
3. **Short TTL**: 30-120 seconds maximum
4. **No MSISDN in URLs/Logs**: Only use opaque tokens; log MSISDN as hash

### What NOT to Do

- ❌ Do not enable HSTS preload while using HTTP-only HE
- ❌ Do not trust HE headers from non-operator IPs
- ❌ Do not store MSISDN in URL query parameters
- ❌ Do not log raw MSISDN values

---

## 4. Testing Checklist

### Pre-Production

- [ ] Obtain operator CIDRs from MTN, Telecel, AT Ghana
- [ ] Configure `HE_TRUSTED_PROXY_CIDRS` environment variable
- [ ] Update NGINX `he-bootstrap.conf` with operator geo blocks
- [ ] Generate and configure `HE_BOOTSTRAP_TOKEN_SECRET`
- [ ] Verify Redis connectivity for token storage

### Production Verification

- [ ] Test with real device on MTN mobile data
- [ ] Test with real device on Telecel mobile data  
- [ ] Test with real device on AT Ghana mobile data
- [ ] Verify spoofing attempt (send X-MSISDN from public IP) is rejected
- [ ] Verify token replay is rejected
- [ ] Verify expired token is rejected
- [ ] Check logs for proper MSISDN hashing

---

## 5. Troubleshooting

### HE Headers Not Appearing

1. Ensure device is on **mobile data** (not WiFi)
2. Ensure request is **HTTP** (not HTTPS) - check ad link
3. Some operators require IP allowlisting - contact operator
4. Check NGINX logs for incoming headers

### Token Exchange Failing

1. Check token TTL hasn't expired (default 60s)
2. Verify Redis connectivity
3. Check for token replay (already used)
4. Verify `HE_BOOTSTRAP_TOKEN_SECRET` matches between services

### Spoofed Headers Being Accepted

1. Verify `HE_TRUSTED_PROXY_CIDRS` is correctly configured
2. Check NGINX geo module is properly stripping headers from unknown IPs
3. Review application-level IP validation

---

## 6. Testing

### Automated Tests

Run the test script to verify the bootstrap flow:

```bash
# Test against local environment
./scripts/test_he_bootstrap.sh local

# Test against staging
./scripts/test_he_bootstrap.sh staging
```

### Manual Mobile Testing

Real HE testing requires physical devices on operator mobile data networks:

1. **MTN Ghana Test**
   - Device with MTN SIM on mobile data (not WiFi)
   - Navigate to: `http://landing.nouveauricheglobalgroup.com/c/test-campaign`
   - Expected: Redirect to HTTPS with `he_token` param
   - Exchange token and verify MSISDN starts with `233`

2. **Telecel Ghana Test**
   - Device with Telecel SIM on mobile data
   - Same flow as above

3. **AT Ghana Test**
   - Device with AT Ghana SIM on mobile data
   - Same flow as above

4. **WiFi Test (negative)**
   - Device on WiFi (no HE)
   - Navigate to same URL
   - Expected: Redirect to HTTPS **without** `he_token` (OTP flow)

5. **Spoof Test (negative)**
   - From any device, attempt to send X-MSISDN header on HTTPS:
   ```bash
   curl -H 'X-MSISDN: 233999999999' \
     https://api.nouveauricheglobalgroup.com/v1/acquisition/transactions \
     -d '{"msisdn":"233999999999"}'
   ```
   - Expected: MSISDN should NOT be in request context (stripped by NGINX)

---

## 7. Long-Term: Request Secure HE from Operators

The HTTP bootstrap is a workaround. The proper solution is **secure HE on HTTPS**:

- Ask each operator if they support HTTPS header enrichment
- Some operators use encrypted/tokenized MSISDNs that work over HTTPS
- This eliminates the HTTP bootstrap complexity entirely

---

## Related Documentation

- [Ghana HE Parameters](./ghana-header-enrichment-parameters.md) - MCC/MNC values and header names
- [HE Simulation Guide](./timwe-he-simulation-e2e.md) - Testing without real MNO network
