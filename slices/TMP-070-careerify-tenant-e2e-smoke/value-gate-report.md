# Value Gate Report — TMP-070: Careerify Tenant E2E Smoke

**Slice:** TMP-070 `careerify-tenant-e2e-smoke`  
**Status:** Scripts authored — awaiting operator run against staging  
**Risk boundary:** Smoke/adversarial scripts + docs only. No production code touched.

---

## Shipped Commit SHAs

| Slice | Description | Commit |
|-------|-------------|--------|
| TMP-066 | Seed careerify tenant + web-gh-airteltigo channel + credentials | `77f9359` |
| TMP-067 | KrakenD notification tenant header propagation | `7e10692` |
| TMP-068 | KrakenD subscription tenant path routing | `3027c86` |
| TMP-069 | Tenant resolver header/query precedence (conflict 409) | `3897e89` |

These are the commits the smoke matrix targets. Run against the staging build that contains all four.

---

## Happy-Path URL Matrix (10 endpoints)

All 10 use `POST`, `Content-Type: application/json`, `TENANT_KEY=careerify`, `CHANNEL_KEY=web-gh-airteltigo`.

| # | Label | URL (template) | Expected HTTP |
|---|-------|----------------|---------------|
| 1 | notification/mo | `POST /api/v1/notification/mo/{partnerRole}?tenant_key=careerify&channel_key=web-gh-airteltigo` | 2xx |
| 2 | notification/mt-dn | `POST /api/v1/notification/mt/dn/{partnerRole}?tenant_key=careerify&channel_key=web-gh-airteltigo` | 2xx |
| 3 | notification/user-optin | `POST /api/v1/notification/user-optin/{partnerRole}?tenant_key=careerify&channel_key=web-gh-airteltigo` | 2xx |
| 4 | notification/user-renewed | `POST /api/v1/notification/user-renewed/{partnerRole}?tenant_key=careerify&channel_key=web-gh-airteltigo` | 2xx |
| 5 | notification/user-optout | `POST /api/v1/notification/user-optout/{partnerRole}?tenant_key=careerify&channel_key=web-gh-airteltigo` | 2xx |
| 6 | notification/charge | `POST /api/v1/notification/charge/{partnerRole}?tenant_key=careerify&channel_key=web-gh-airteltigo` | 2xx |
| 7 | subscription/optin | `POST /api/external/v1/careerify/web-gh-airteltigo/subscriptions/optin` | 2xx |
| 8 | subscription/confirm | `POST /api/external/v1/careerify/web-gh-airteltigo/subscriptions/confirm` | 2xx |
| 9 | subscription/optout | `POST /api/external/v1/careerify/web-gh-airteltigo/subscriptions/optout` | 2xx |
| 10 | subscription/status | `POST /api/external/v1/careerify/web-gh-airteltigo/subscriptions/status` | 2xx |

Default `{partnerRole}` = `airtelgh`. Override with `PARTNER_ROLE=<value>`.

---

## Adversarial Matrix (3 cross-tenant cases)

**PASS = server rejected with expected status. FAIL = server accepted (2xx) — scoping gap.**

| Case | Endpoint | Headers / Query | Expected | Expected Error Code | Gap Owner if FAIL |
|------|----------|----------------|----------|---------------------|-------------------|
| A — conflict | `POST /api/external/v1/careerify/web-gh-airteltigo/subscriptions/optin?tenant_key=other-tenant&channel_key=web-gh-airteltigo` | `X-Tenant-Key: careerify` (header disagrees with query) | **409** | `TENANT_CONTEXT_REQUIRED` | TMP-069 |
| B — foreign tenant | `POST /api/v1/notification/mo/{partnerRole}?tenant_key=evil-tenant&channel_key=web-gh-airteltigo` | none beyond Content-Type | **4xx** (400 or 404) | tenant resolution failure | TMP-066 or TMP-069 |
| C — missing channel | `POST /api/v1/notification/mo/{partnerRole}?tenant_key=careerify` (no channel_key) | none beyond Content-Type | **4xx** (400) | `TENANT_CHANNEL_REQUIRED` | TMP-067 or TMP-069 |

### Case A conflict response shape (TMP-069, `partner_handler.go writeError`):

```json
{
  "code": "TENANT_CONTEXT_REQUIRED",
  "message": "tenant key conflict: header and query parameter disagree: X-Tenant-Key header=\"careerify\" query=\"other-tenant\"",
  "inError": true,
  "responseData": {}
}
```

---

## How to Run Against Staging

### Prerequisites

- `curl` available on the operator machine.
- Staging stack running all four commits above (TMP-066 through TMP-069).
- Network access from the operator machine to the staging KrakenD gateway.

### Environment variables

| Variable | Default | Override |
|----------|---------|---------|
| `HOST` | `http://127.0.0.1:8080` | Set to staging KrakenD base URL, e.g. `https://staging-gw.example.com` |
| `TENANT_KEY` | `careerify` | Leave as default for this matrix |
| `CHANNEL_KEY` | `web-gh-airteltigo` | Leave as default for this matrix |
| `PARTNER_ROLE` | `airtelgh` | Override if staging uses a different partner role slug |
| `MSISDN` | `233572503330` | Override with a staging-safe MSISDN if required |

### Step 1 — Happy-path matrix

```bash
HOST=https://staging-gw.example.com \
  ./scripts/smoke/careerify-tenant-e2e.sh
```

**Successful output looks like:**

```
=== Careerify tenant e2e smoke test (10 happy-path URLs) ===
  Gateway    : https://staging-gw.example.com
  Tenant     : careerify / web-gh-airteltigo
  ...

  [PASS] notification/mo  HTTP 200
  [PASS] notification/mt-dn  HTTP 200
  [PASS] notification/user-optin  HTTP 200
  [PASS] notification/user-renewed  HTTP 200
  [PASS] notification/user-optout  HTTP 200
  [PASS] notification/charge  HTTP 200
  [PASS] subscription/optin  HTTP 200
  [PASS] subscription/confirm  HTTP 200
  [PASS] subscription/optout  HTTP 200
  [PASS] subscription/status  HTTP 200

=== Results: 10/10 PASS  0/10 FAIL ===

All 10 careerify tenant endpoints returned 2xx. Tenant scoping end-to-end VERIFIED.
```

Exit code 0 = gate passes.

### Step 2 — Adversarial refusal matrix

```bash
HOST=https://staging-gw.example.com \
  ./scripts/smoke/careerify-tenant-cross-tenant-refusal.sh
```

**Successful output looks like:**

```
=== Careerify cross-tenant refusal smoke test (3 adversarial cases) ===
  ...
  Case  : A) header/query conflict (tenant key mismatch)
  Result: [PASS] server refused with 409 as expected

  Case  : B) foreign tenant key (unknown tenant)
  Result: [PASS] server refused with 4xx (404) as expected

  Case  : C) missing channel_key (tenant only, no channel)
  Result: [PASS] server refused with 4xx (400) as expected

=== Results: 3/3 PASS  3/3 FAIL ===

All 3 adversarial cross-tenant injection attempts were correctly refused.
```

Exit code 0 = gate passes.

### Failure modes and ownership

| Failure | Likely cause | Owning slice |
|---------|-------------|--------------|
| notification/* return 404 or 502 | KrakenD route missing or backend unreachable | TMP-067 |
| notification/* return 400 (tenant not found) | `careerify` tenant not seeded in DB | TMP-066 |
| subscription/* return 404 or 502 | KrakenD subscription path not routed | TMP-068 |
| subscription/* return 400 (tenant not found) | `careerify` or `web-gh-airteltigo` channel missing | TMP-066 |
| Case A returns 200 (not 409) | Header/query conflict not detected | TMP-069 |
| Case B returns 200 (not 4xx) | Unknown tenant accepted | TMP-066 or TMP-069 |
| Case C returns 200 (not 4xx) | Missing channel_key not enforced | TMP-067 or TMP-069 |

---

## Agent Note

Scripts are not run automatically here — no live stack is present in the agent worktree.
Operator validates against staging and records results in this document.

Gate: **operator sign-off** on this value-gate-report.md after a clean run of both scripts
against the staging environment containing commits 77f9359 through 3897e89.
