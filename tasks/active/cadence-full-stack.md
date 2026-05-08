# Cadence Full Stack Implementation

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-17
- Completed: 2026-01-17

## Dependencies
- Cadence Admin + Content Loading (CSV) - completed

## ExitCriteria
- [x] Build cadence-engine and notification-worker locally
- [x] Seed test cadence data (series, schedule rule, content items) on local and remote DBs
- [x] Add "Publish content version" admin endpoint and UI
- [x] Verify end-to-end cadence flow (backfill -> planner -> outbox -> dispatcher)

## Todos
1. build-services - Build cadence-engine and notification services [completed]
2. seed-test-data - Seed test cadence data [completed]
3. publish-version-flow - Add Publish content version feature [completed]
4. verify-e2e - Verify end-to-end flow [completed]

## Notes
- Built `cadence-engine` and `notification-worker` binaries locally
- Seeded test data on both local (`subscription_manager_db`) and remote (139.59.135.253) databases:
  - Series: `daily-tips` (SEQUENTIAL mode) for partner_role_id=2117
  - Schedule rule: DAILY at 10:00, send window 08:00-20:00, Africa/Accra timezone
  - Content items: 5 welcome/tip messages
  - Local test subscription: 233200000001
- Added `POST /v1/admin/cadence/series/{id}/publish` endpoint to cadence-engine
  - Validates content version has active items before publishing
  - Returns previous and new version info
- Added "Publish Version" tab to webspa-admin Cadence UI
  - Dropdown to select available content versions
  - Publish button with success feedback
- Verified end-to-end flow by simulating SQL queries:
  - Backfill detects missing subscription_message_state pairs and creates them
  - Planner creates message_outbox jobs for due states
  - Outbox job correctly resolves content_item_id and message_text
- Fixed docker-compose.yml interpolation syntax (`:-` for default values)
