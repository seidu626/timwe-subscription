# TMP-077: Real merchandising signals (Dayline slice 3)

## Goal
Make the app catalog's persuasion signals real:
1. subscriber_count computed from active rows in the subscriptions table
   (scalar subquery per campaign: tenant_id + offer_product_id, LOWER(status)='active').
2. next_charge_hint populated for ACTIVE app subscriptions from charged_at
   (fallback started_at) advanced by billing_cycle to the next future date.
3. app_featured_rank (nullable INTEGER, additive migration) on campaigns:
   featured products sort first in catalog sections and carry featured=true
   on the wire; console App Presence section gains a Featured rank field;
   Discover renders a featured hero row.

## Verification
- cd services/acquisition-api && go build ./... && go test ./...
- cd frontend/webspa-admin && npm run build
- cd frontend/dayline-mobile && npx tsc --noEmit

## Poteto notes
Additive migration only; empty rank = not featured (no behavior change for
existing rows). Fail-before/pass-after tests for count, hint math, ordering.
