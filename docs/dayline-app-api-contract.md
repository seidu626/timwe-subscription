# Dayline App API Contract (v1)

Contract for the Dayline subscriber mobile app. All three lanes (mobile, auth/catalog, feed/push) build against THIS document. Do not rename paths or fields unilaterally; if a change is unavoidable, record it in the lane result capsule.

Gateway prefix: all routes exposed via KrakenD under `/v1/app/*`.
Owner services: auth/catalog/subscriptions = acquisition-api; feed/devices/prefs = subscription-external; push delivery = notification.

## Conventions
- JSON bodies, `snake_case` fields.
- Errors: `{"error": {"code": "STRING_CODE", "message": "human readable"}}` with proper HTTP status. Codes: `INVALID_MSISDN`, `OTP_INVALID`, `OTP_EXPIRED`, `RATE_LIMITED`, `UNAUTHORIZED`, `NOT_FOUND`, `CONFLICT`, `PROVIDER_ERROR`, `VALIDATION`.
- MSISDN format: E.164 without plus, e.g. `233241234567`.
- Auth: `Authorization: Bearer <jwt>` on every route except otp request/verify and catalog browse.

## JWT
- HS256, secret env `DAYLINE_APP_JWT_SECRET` (shared by acquisition-api and subscription-external; fail closed if unset).
- Claims: `sub` = msisdn, `tenant` = tenant_key, `iss` = "dayline-app", `exp` (24h), `iat`.
- Login OTP is a DISTINCT credential from the TIMWE billing opt-in PIN. Never reuse tables/copy between them.

## Auth (acquisition-api)
- `POST /v1/app/auth/otp/request` body `{"msisdn": "...", "tenant": "careerify"}` -> 204. Generates 6-digit code, 5 min TTL, max 3 active requests/msisdn/hour (else 429 RATE_LIMITED). Delivery: direct send through the tenant's `sms_api` gateway credential (see docs/tenant-channel-onboarding.md, "SMS Gateway Credential"); the platform's TIMWE partner MT path cannot carry free text, so the outbox route was dropped.
- `POST /v1/app/auth/otp/verify` body `{"msisdn": "...", "tenant": "...", "code": "123456"}` -> `{"token": "...", "expires_in": 86400}`. 5 attempts/code then OTP_INVALID + invalidate.
- Storage: new table `app_login_otps` (msisdn, tenant_key, code_hash, expires_at, attempts, consumed_at).

## Catalog (acquisition-api)
- `GET /v1/app/catalog?tenant=careerify&country=GH` (no auth) -> `{"products": [{"slug", "name", "tagline", "description", "category", "artwork_url", "sample_content", "price", "currency", "billing_cycle", "flow_type", "subscriber_count", "featured"}]}`. `subscriber_count` is computed live from active rows in the `subscriptions` table (tenant_id + product_id, status=active). `featured` is true when the console assigned an `app_featured_rank`; featured products sort first within each tenant section (rank ascending, unranked last). Every product also carries `tenant` (tenant_key) and `tenant_name` so marketplace clients can attribute it.
- `GET /v1/app/catalog` without a `tenant` param (marketplace mode, no auth) -> `{"tenants": [{"tenant_key": "...", "tenant_name": "...", "products": [...]}]}`: one storefront section per tenant that has at least one enabled app-visible campaign, sections ordered by tenant_key, products within a section ordered featured-rank first then slug. Product objects are the same shape as the tenant-filtered response. `country` still filters when provided.
- Backed by `campaigns` plus new nullable columns `app_name`, `app_tagline`, `app_description`, `app_category`, `app_artwork_url`, `app_sample_content`, `app_featured_rank` (additive migrations only). Fall back to lp_copy fields where app_* is null. Only campaigns with `enabled=true` (existing semantics) are listed.

## Subscriptions (acquisition-api)
- `POST /v1/app/subscriptions` auth; body `{"campaign_slug": "..."}` -> `{"subscription_ref": "<transaction_id>", "next_action": "OTP"|"CONFIRM"|"SUBSCRIBED", "message": "..."}`. Thin wrapper over the existing transaction create path (same service methods the LP uses; msisdn/tenant from JWT). Respect campaign flow_type exactly as the LP does (OTP, DOUBLE_OPTIN, AUTO).
- `POST /v1/app/subscriptions/{ref}/confirm` auth; body `{"pin": "1234"}` (pin optional for DOUBLE_OPTIN) -> `{"status": "ACTIVE"|"PENDING"|"FAILED"}`. Wrapper over existing confirm.
- `GET /v1/app/subscriptions` auth -> `{"subscriptions": [{"ref", "product_slug", "product_name", "status", "price", "currency", "billing_cycle", "next_charge_hint", "started_at"}]}`. `status` is `"ACTIVE"|"PENDING"|"CANCELLED"|"FAILED"`. `next_charge_hint` (e.g. "Renews 25 Aug") is populated only for ACTIVE subscriptions: the last charge (`charged_at`, falling back to the opt-in date) advanced by whole billing cycles (daily/weekly/biweekly/monthly) past now; omitted for unknown cycles.
- `DELETE /v1/app/subscriptions/{ref}` auth -> 202. Triggers the existing opt-out path in subscription-external (direct in-cluster call with gateway-trust headers, same pattern acquisition-api already uses for opt-in) and marks the transaction CANCELLED on success. PENDING/FAILED entries never activated at the provider, so cancel skips the provider opt-out and just marks the row CANCELLED; cancelling an already-CANCELLED entry is an idempotent no-op.

## Feed (subscription-external)
- `GET /v1/app/feed` auth -> `{"items": [{"id", "product_slug", "product_name", "title", "body", "published_at", "read": bool}]}`. Items = content already DELIVERED to this msisdn (from message_outbox history) plus today's due item if sent. Order: published_at desc, max 50.
- `GET /v1/app/feed/items/{id}` auth -> single item.
- `POST /v1/app/feed/items/{id}/read` auth -> 204.
- Rich content: migration adds nullable `title` to `message_content_items`; `body` = existing `message_text`. Derive title fallback = first 60 chars of message_text.
- Read state: new table `app_feed_read_state` (msisdn, content_item_id, read_at).

## Devices & notification prefs (subscription-external)
- `POST /v1/app/devices` auth; body `{"fcm_token": "...", "platform": "android"|"ios"}` -> 204. Upsert into new `app_devices` (msisdn, tenant_key, fcm_token unique, platform, updated_at). On iOS the value is the raw APNs token unless the Firebase iOS config is compiled into the build; operator setup and the token-shape caveat are in docs/dayline-push-setup.md.
- `PUT /v1/app/notification-prefs` auth; body `{"product_slug": "...", "channel": "PUSH"|"SMS"|"BOTH"}` -> 204. New table `app_notification_prefs`.
- `GET /v1/app/notification-prefs` auth -> 200 `{"prefs": [{"product_slug": "...", "channel": "PUSH"|"SMS"|"BOTH"}]}`. One entry per stored row, ordered by product_slug; products with no stored row are absent and the app treats them as its default (BOTH).

## Push delivery (notification)
- `message_outbox` gains nullable `channel` column, default 'SMS'. Cadence planner unchanged this round; a small router in notification dispatcher: jobs for msisdns with channel pref PUSH and a registered device go to the FCM sender, else SMS (existing path). Same idempotency keys.
- FCM: HTTP v1 API, service-account JSON via env `FCM_CREDENTIALS_JSON_PATH`. If unset, PUSH jobs fail over to SMS and a warning logs once - never drop content.

## Non-negotiables (all lanes)
- No mocks/stubs in shipped paths; env-gated real clients only.
- Migrations additive only; follow each service's existing migration numbering.
- Tests: fail-before/pass-after for new behavior; deterministic.
- Match surrounding code style; no reformat-only diffs.
