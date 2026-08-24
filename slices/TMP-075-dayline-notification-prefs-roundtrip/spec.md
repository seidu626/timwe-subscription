# TMP-075 dayline-notification-prefs-roundtrip

## Problem

The Dayline app contract has PUT /v1/app/notification-prefs but no GET, so the
Notifications screen holds preferences in local component state only. Every
app launch shows BOTH for every product regardless of what the user saved.

## Decision

Additive GET /v1/app/notification-prefs on subscription-external (the service
that owns app_notification_prefs). No migration needed; the table exists.

Response shape:

    {"prefs": [{"product_slug": "careerify-tips", "channel": "PUSH"}]}

Products with no stored row are absent; the app treats absence as its default
(BOTH). Rows ordered by product_slug for a stable response.

## Scope

- repository: ListNotificationPrefs(ctx, msisdn)
- handler: GetNotificationPrefs + AppFeedRepo interface method
- router: GET wired next to the existing PUT
- krakend: Endpoint.tmpl GET line mirroring the PUT
- contract doc: new GET section
- mobile: api/devices.ts getNotificationPrefs, types, queryKeys,
  useNotificationPrefs hook, notifications.tsx hydration (server base,
  local optimistic override)

## Verification

- go build ./... && go test ./... in services/subscription-external
  (new handler tests: 401 fail-closed, 200 happy path, 500 repo error;
  new repo test via sqlmock)
- npx tsc --noEmit in frontend/dayline-mobile
