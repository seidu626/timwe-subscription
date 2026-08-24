# TMP-078: Dayline app hardening

From the completeness review (slice 4 of the recommended sequence).

## Goals

1. Global 401 sign-out: any authenticated /v1/app/* call returning 401 clears the
   stored session and returns the user to the sign-in screen instead of leaving
   dead screens behind an expired 24h token.
2. Cancel PENDING subscriptions: a subscriber stuck in PENDING (opt-in never
   confirmed by the carrier) can remove the entry, not just ACTIVE ones. Backend
   must accept cancel for PENDING states; mobile shows the cancel action for
   PENDING rows.
3. Pull-to-refresh on Today, Discover, and Subscriptions so users can refetch
   without killing the app.

## Non-goals

- iOS push (slice 5) and hygiene items (slice 6).

## Verification

- cd services/acquisition-api && go build ./... && go test ./...
- cd frontend/dayline-mobile && npx tsc --noEmit
