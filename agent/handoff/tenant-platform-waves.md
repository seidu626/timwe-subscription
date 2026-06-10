# Tenant Platform — Remaining Waves Handoff

**Written:** 2026-06-10 (end of tenant-config session)
**For:** next session (Claude/Codex/Gemini) to execute to completion
**Main HEAD at handoff:** `e4c5323 fix(auth0jwt): make JWKS acquisition lazy and self-healing`
**Droplet:** `do-sa` (139.59.135.253), compose `/home/xper626/services/nouveauricheglobalgroup`, images `xper626/*:latest` on Docker Hub. Deploy via `just deploy-<svc>` (build->push->ssh deploy.sh). DB same host, db `subscription_manager`.

---

## Ground truth: deployed vs merged-but-undeployed

| Service | Droplet image | This session's code deployed? |
|---|---|---|
| acquisition-api | 2026-06-10 | YES - credential write path + master key live |
| subscription-external | 2026-06-10 | YES - secret:// resolver + per-tenant fields + JOIN fix live |
| notification-service | 2026-05-15 | NO - pre-session (tenant-enforcement NOT deployed) |
| subscription-partner | 2026-05-15 | NO - pre-session (tenant resolution NOT deployed) |
| krakend | not redeployed | NO - new routes / auth / Auth0-param NOT deployed |
| cadence-engine | 2026-05-15 | NO |

**On main but on NO droplet:** `e4c5323` (JWKS self-heal, all 4 services), krakend admin-auth + tenant-routes, notification 422 enforcement, subscription-partner tenancy, batch auth guard. Held back last session to protect nrg live traffic. Deploying them = Wave 0; sequence carefully.

**Live tenants:** `nrg` (1.65M subs, real traffic, TIMWE 200) and `careerify` (onboarded, dev placeholder keys -> TIMWE rejects; needs real keys). Both have `secret://` ACTIVE credentials.
**Master key:** `TENANT_SECRET_MASTER_KEY` in droplet `.env` only. NOT backed up. Loss = unrecoverable secrets.
**careerify channel id:** `e5cb0f18-ae69-40d6-8ea9-796607a4ec6b` | **nrg channel id:** `7d151c7e-1393-457d-9798-91f6b356c6ce`

---

## How to run each wave
Each thread = one Agent dispatch. USE SUPERVISOR-PREPARED WORKTREES, not isolation:worktree (last session: harness worktrees came off stale base 4/6 times):
```
git worktree add /tmp/<name> -b <branch> main    # supervisor runs first
# dispatch agent at /tmp/<name>; brief: "verify git log --oneline -1 == main HEAD before editing"
```
Go: `go mod vendor` after import changes (vendor/ gitignored except subscription-partner which tracks it). Verify real `go build ./...` per module + tests. Merge sequentially, re-verify, then deploy. Threads within a wave have disjoint file scopes (parallel-safe); waves are sequential.

---

## WAVE 0 - Deploy what's merged (FIRST, gated, NOT parallel)
- **0.1 JWKS self-heal -> 4 services** (acquisition-api, cadence-engine, notification, subscription-external). Low risk. Verify each /health 200 + admin auth works after.
- **0.2 notification tenant-enforcement.** RISK: NOTIFICATION_REQUIRE_TENANT_CONTEXT defaults TRUE -> tenantless TIMWE callbacks 422. GATE: confirm TIMWE callback URLs for nrg carry tenant_key+channel_key first; else deploy flag FALSE, flip after URLs updated.
- **0.3 subscription-partner tenancy.** RISK: routes 422 without tenant context (incl legacy /api/v1/webhooks/timwe/notification). Same gate as 0.2.
- **0.4 krakend changes** (krakend-sync + recreate): 8 admin-ops routes, auth/validator on 6 previously-open admin endpoints, Auth0-from-settings, tenant-path MT/charge. Verify nrg public partner routes resolve after.
- **0.5 Set JWT_SECRET (trusted-service HMAC) on subscription-external** so admin manual-subscription routes work (today 400 "trusted service secret is not configured"). Wire into compose env like TENANT_SECRET_MASTER_KEY.
Closeout: smoke nrg optin+status+callback after each step. Rollback tags :rollback-20260610 exist for acquisition-api + subscription-external.

---

## WAVE 1 - Complete admin-managed config surface (3 parallel threads)
- **1.1 webspa-admin credential secret_value form** (frontend only). Dialog (`features/tenant/...`, `tenant.service.ts`, `tenant.model.ts` ChannelCredentialPayload) only sends secret_ref. Add per-field form for full blob (base_url,api_key,mt_api_key,psk,partner_service_id,partner_role_id,realm,mcc,mnc,large_account,service_name,free_mt_pricepoint_id,mo_pricepoint_ids,billing_pricepoint_ids,he_iv_param_spec_key) POSTing secret_value (stringified JSON). Mask display; show version history. `npm run build`.
- **1.2 Channel update endpoint** (acquisition-api + frontend). Only enable/disable exists. Add PATCH /v1/admin/channels/{id} (operator/capabilities/country) + repo + tests + portal edit form.
- **1.3 Credential revoke+purge** (acquisition-api + frontend). No crypto-erasure today. Add DELETE /v1/admin/channels/{id}/credentials/{credential_id}: mark REVOKED + purge tenant_channel_secrets ciphertext, keep purge audit record. Portal button w/ confirm.

---

## WAVE 2 - Wire stored-but-unused per-tenant fields (2 parallel threads)
Fields added last session as store-through only (in TenantProviderConfig, not yet consumed).
- **2.1 Pricepoint + large-account in TIMWE requests** (subscription-external service). MT/optin uses product.PricePointId/ShortCode today. Make providerCfg.FreeMTPricepointID/MOPricepointIDs/BillingPricepointIDs/LargeAccount the documented fallback (decide product-vs-tenant precedence). Tests prove each reaches outbound payload; remove the TODOs.
- **2.2 acquisition-api MCC/MNC + LargeAccount per-tenant** (acquisition-api). timwe_client.go uses global c.config.MCC/MNC, never sets LargeAccount. Thread per-tenant values through the acq->subscription-external optin call (already forwards X-Tenant-Channel-Id). Coordinate forwarded fields with 2.1.
Dep: end-to-end test needs Wave 0.4 deployed; code can be written on Wave 1 state.

---

## WAVE 3 - Deferred security/hardening (4 parallel threads)
- **3.1 Cryptographic gateway trust marker** = WO-CRYPTOGRAPHIC-GATEWAY-TRUST-MARKER-FOR-TENANT-CO (in work-orders/review/, classified vertical_slice). KrakenD injects signed X-Gateway-Trust; services verify before honoring query-param tenant context. Stops direct service-port spoofing. Scope: krakend + common/auth/tenantctx + 3 partner services.
- **3.2 Module-wide MSISDN log masking** (subscription-external + common). 160+ zap.String("msisdn",...) raw-PII sites. Promote maskMSISDN to common helper, sweep all layers.
- **3.3 Gateway CORS + per-IP callback throttle + 404 log redaction** (krakend + ops/nginx + notification). Tighten allow_headers:['*']; per-IP limit on notification callbacks; redact Authorization/HE headers from error logs.
- **3.4 Resolver real-DB integration test** (subscription-external test infra). The JOIN-prefix bug shipped because the fake driver ignores SQL. Add testcontainers/dockertest (or sqlmock-regex) for the scoped secret lookup.

---

## WAVE 4 - Operator data + key management
- **4.1 careerify real TIMWE keys** - operator provides; re-POST secret_value to careerify channel.
- **4.2 nrg distinct identifiers** - confirm nrg == global (role 2117/svc 2170/MCC 620/MNC 03) or supply nrg's own; re-bind if different.
- **4.3 Master key backup + rotation** - back up TENANT_SECRET_MASTER_KEY to a secret manager; implement key_version-aware decrypt + re-encrypt migration (column exists, decrypt always uses current key). Scope: common/secretcrypto + acquisition-api + ops.
- **4.4 Real secret backend (optional)** - only secret:// + env:// resolvers exist; add Vault/AWS-SM resolver + write path if moving off DB-encrypted. Decision-gated.
- **4.5 HE IV param spec key consumption** - stored, no consumer (HE uses plaintext MNO headers). Spike if HE-decryption is a real requirement.

---

## WAVE 5 - Control-plane + tooling debt (low priority, anytime)
From `sia list`: vnext schemas fail-open on loop_mode/loop_budget + no ceiling (fix in agent-hub canonical, re-sync); `agent-hub evidence record` missing lineage-record.schema.json; vendored-schema drift detection; `just dev` blocks before binding when Postgres unreachable; start-binary port-shift; dev-launcher 5s wait too short under dev-all.

---

## Order
Wave 0 (gated deploy) -> Wave 1 (portal) and Wave 3 (hardening) overlap -> Wave 2 (field wiring, needs 0.4) -> Wave 4 (operator/keys) -> Wave 5 (anytime).

## Status update — 2026-06-10 (second session)

**DONE on main (code-complete, NOT deployed):** Waves 1 (1.1/1.2/1.3), 2 (2.1/2.2), 3 (3.1/3.2/3.3/3.4), plus an independent Opus review of the whole range and a fix branch for its findings (CORS DELETE method, legacy partner handlers wired into checkGatewayTrust, tenant/channel-scoped ciphertext reads). All module suites green: acquisition-api 275, subscription-external 310, notification 75, subscription-partner 24, common 52. WO-CRYPTOGRAPHIC-GATEWAY-TRUST-MARKER advanced to review with evidence recorded; lineage-record schema synced (evidence ledger unblocked).

**Trust-marker deploy notes (now part of Wave 0):**
- KrakenD CE martian can only inject a STATIC X-Gateway-Trust token (no per-request HMAC). Replace `__REPLACE_WITH_GATEWAY_TRUST_TOKEN__` in krakend.json using `tools/gateway-trust-token` before krakend deploy.
- `GATEWAY_TRUST_SECRET` + `GATEWAY_TRUST_REQUIRED` env stanzas added to docker-compose.prod-do.yml for the 3 services; enforcement defaults OFF (permissive, log-only). Flip per service only after krakend injects the header.
- Notification per-IP callback limiter is OFF by default (`CALLBACK_RATE_LIMIT_PER_MIN=0`).

**REMAINING:** Wave 0 (gated deploy, unchanged — operator must confirm TIMWE callback URLs carry tenant_key+channel_key before 0.2/0.3); Wave 4 (operator data + key management; 4.3 rotation code implementable but secret-handling gated); Wave 5 minus the lineage-record schema item (done).

## Done = whole effort
All tenant config add/update/delete portal-managed; per-tenant credentials drive every TIMWE request field for every tenant; notification+partner+krakend tenant code deployed and nrg verified intact; gateway trust marker live; PII masked; master key backed up + rotatable; careerify on real keys.
