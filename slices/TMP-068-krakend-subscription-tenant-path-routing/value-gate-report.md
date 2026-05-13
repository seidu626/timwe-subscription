# Value Gate Report — TMP-068: krakend-subscription-tenant-path-routing

**Date:** 2026-05-13  
**Slice:** TMP-068  
**Status:** Ready for operator risk-boundary gate (commit pending approval)

---

## What Changed

### `krakend/krakend.json`

Added 4 new POST endpoints at the end of the `endpoints` array. Each endpoint:

- Accepts `POST /api/external/v1/{tenant_key}/{channel_key}/subscriptions/{op}`
- Path-rewrites to `backend[0].url_pattern = /api/v1/subscription-external/admin/{op}` (Option A — KrakenD controls the upstream path via `url_pattern`)
- Injects `X-Tenant-Key` and `X-Channel-Key` headers via `modifier/martian` `fifo.Group` in `backend[0].extra_config`, with values substituted from path captures `{tenant_key}` and `{channel_key}` at request time
- Uses backend host `http://127.0.0.1:8083` (same as existing admin endpoints)
- Passes through `external-tx-id`, `x-admin-request-id`, `x-requestid` headers

**No Go code modified.** The `partner_handler.go` already reads `X-Tenant-Key` / `X-Channel-Key` headers (lines 262–293).

### `scripts/smoke/krakend-subscription-tenant.sh`

New smoke script (executable). POSTs to all 4 gateway URLs with `TENANT_KEY=careerify CHANNEL_KEY=web-gh-airteltigo`. Mirrors the structure of `krakend-notification-tenant.sh`. Returns exit 0 only if all 4 produce 2xx.

---

## Sample JSON Snippet (optin endpoint)

```json
{
  "endpoint": "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/optin",
  "method": "POST",
  "output_encoding": "no-op",
  "extra_config": {
    "github.com/devopsfaith/krakend/http": { "return_error_details": true }
  },
  "headers_to_pass": ["external-tx-id", "x-admin-request-id", "x-requestid"],
  "backend": [
    {
      "url_pattern": "/api/v1/subscription-external/admin/optin",
      "encoding": "json",
      "sd": "static",
      "method": "POST",
      "extra_config": {
        "modifier/martian": {
          "fifo.Group": {
            "scope": ["request"],
            "aggregateErrors": true,
            "modifiers": [
              {"header.Modifier": {"scope": ["request"], "name": "X-Tenant-Key", "value": "{tenant_key}"}},
              {"header.Modifier": {"scope": ["request"], "name": "X-Channel-Key", "value": "{channel_key}"}}
            ]
          }
        }
      },
      "host": ["http://127.0.0.1:8083"],
      "disable_host_sanitize": true,
      "headers_to_pass": ["external-tx-id", "x-admin-request-id", "x-requestid"]
    }
  ]
}
```

---

## Acceptance Checks

| # | Criterion | Result |
|---|-----------|--------|
| 1 | `krakend/krakend.json` has 4 new POST endpoints | PASS |
| 2 | Each rewrites upstream path to `/api/v1/subscription-external/admin/{op}` | PASS |
| 3 | Each injects X-Tenant-Key and X-Channel-Key via martian fifo.Group | PASS |
| 4 | `scripts/smoke/krakend-subscription-tenant.sh` exists and is executable | PASS |
| 5 | `jq empty krakend/krakend.json` returns 0 (valid JSON) | PASS |

### `jq` verification output

```
$ jq '.endpoints[] | select(.endpoint | startswith("/api/external/v1/{tenant_key}")) | {endpoint, method, "backend_url": .backend[0].url_pattern}' krakend/krakend.json

{
  "endpoint": "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/optin",
  "method": "POST",
  "backend_url": "/api/v1/subscription-external/admin/optin"
}
{
  "endpoint": "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/confirm",
  "method": "POST",
  "backend_url": "/api/v1/subscription-external/admin/confirm"
}
{
  "endpoint": "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/optout",
  "method": "POST",
  "backend_url": "/api/v1/subscription-external/admin/optout"
}
{
  "endpoint": "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/status",
  "method": "POST",
  "backend_url": "/api/v1/subscription-external/admin/status"
}
```

---

## Risk Notes

- Gateway config change — operator confirmation required before commit (per context pack `risk_boundary`).
- No Go code changes; downstream handler is unchanged.
- Backend host `http://127.0.0.1:8083` is unchanged from existing admin endpoints — no new host introduced.
- Smoke script will fail (exit 1) until the full stack is running with a seeded `careerify` tenant — expected; TMP-070 covers the live run.
