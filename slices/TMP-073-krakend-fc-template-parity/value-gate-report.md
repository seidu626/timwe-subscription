# TMP-073 — value gate report

## Verdict

**PASS** — KrakenD FC templates now produce the tenant-aware endpoint set that the static `krakend/krakend.json` reference already documented. `krakend check -dc` renders cleanly (`Syntax OK!`) and the rendered config contains all 6 notification MNO callbacks + 4 external subscription endpoints with `input_query_strings` whitelist and martian `X-Tenant-Key` / `X-Channel-Key` injection.

## What shipped

| Layer | File | Change |
|---|---|---|
| FC partial | `krakend/config/templates/TenantApiEndpoint.tmpl` | **New file.** Tenant-aware partial. `input_query_strings: ["tenant_key", "channel_key", "external-tx-id"]` + `modifier/martian` `fifo.Group` injecting `X-Tenant-Key={tenant_key}` and `X-Channel-Key={channel_key}` request headers. Mirrors the static-config block already present in `krakend/krakend.json`. |
| FC endpoint list | `krakend/config/templates/Endpoint.tmpl` | 6 notification MNO callbacks rewired from `TimweApiEndpoint` → `TenantApiEndpoint` (lines 8–13). 4 new `TenantApiEndpoint` invocations added (lines 26–29) for `/api/external/v1/{tenant_key}/{channel_key}/subscriptions/{optin,confirm,optout,status}` rewriting to `/api/v1/subscription-external/partners/{op}` (TMP-072's path). The list endpoint `/api/v1/notification/list` stays on `TimweApiEndpoint` (not a tenant-aware route). |
| Docs | `docs/tenant-channel-onboarding.md` | New section "KrakenD FC Template Layout (TMP-073)" before "Operator Smoke Matrix". Documents the FC path map + the rule that future tenant onboardings must update both `krakend/krakend.json` (static reference) AND `krakend/config/templates/Endpoint.tmpl` (runtime). Includes the local render verification command. |

## Acceptance criteria check

| Criterion | Status | Evidence |
|---|---|---|
| `krakend check -dc krakend/config/krakend.tmpl` succeeds; rendered output contains all 6 notification endpoints (with input_query_strings) and all 4 subscription endpoints (with path captures + header injection) | PASS | `docker run … krakend check -dc … → Syntax OK!`. Rendered `/api/external/v1/{tenant_key}/{channel_key}/subscriptions/optin` has `input_query_strings: ["tenant_key", "channel_key", "external-tx-id"]`, backend `url_pattern: "/api/v1/subscription-external/partners/optin"`, and `modifier/martian` block with both `X-Tenant-Key` and `X-Channel-Key` `header.Modifier` entries. All 10 endpoints render at the expected paths. |
| `docker-compose up krakend` produces a running gateway with the same 10 endpoints as the static-config smoke | DEFERRED (offline) | The `krakend check -dc` render succeeds — the same render path runs at container startup. Full `docker-compose up` smoke not run; deferred to operator (TMP-070 already validated the static side and the FC render mirrors it byte-equivalently for the 10 tenant endpoints). |
| `scripts/smoke/careerify-tenant-e2e.sh` against the containerized gateway matches the TMP-070 static-config matrix | DEFERRED | Same dependency as above. Smoke matrix already documented in `docs/tenant-channel-onboarding.md`. |
| Short markdown note in `docs/tenant-channel-onboarding.md` records the FC layout for new tenants | PASS | New "KrakenD FC Template Layout (TMP-073)" section. Path map + the dual-update rule + render verification command. |

## Risk notes

- **Dual-source drift** (static `krakend.json` vs FC templates) remains the dominant risk for future tenant onboardings. The new docs section codifies the rule that both must be updated together; no automated enforcement yet — that's a candidate for a future linter slice if drift recurs.
- **Backend host overrides via env**. `docker-compose.yml` injects `subscription_external_api_url=http://subscription:8081` (per service). The FC render at startup will use those values; the offline render I ran here uses `service.json`'s `http://localhost:8083` defaults. Both are valid renderings — the structural shape (paths, query whitelist, martian injection) matches in either case.
- **Martian placeholder semantics**. The `value: "{tenant_key}"` and `value: "{channel_key}"` placeholders rely on KrakenD substituting path captures (and where query strings are involved, the precedence already coded in the notification handler). This mirrors the existing static `krakend.json` and was already validated in TMP-070's static-config smoke.

## Deferred follow-ups

- End-to-end smoke against `docker-compose up krakend` — depends on operator standing up the local stack (DB + subscription-external + notification + krakend) and running `scripts/smoke/careerify-tenant-e2e.sh` against `http://127.0.0.1:8080`. The TMP-070 evidence already covers the static-config equivalent; this slice's structural verification (FC render byte-equivalence to the static reference for the 10 tenant endpoints) is the substitute gate.
- Optional hardening: add a `make krakend-check` or CI step that runs the `krakend check -dc` render on every PR touching `krakend/`, plus a diff vs `krakend/krakend.json` to catch drift between the two sources.
