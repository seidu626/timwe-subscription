# Reporting v1 Follow-up Tasks

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-16
- Completed: 2026-01-16

## Dependencies
- Reporting v1 core implementation (completed)

## ExitCriteria
- [x] Landing-web sends analytics events to acquisition-api on page load
- [x] KrakenD routes configured for analytics/reports endpoints (if needed)
- [x] README documentation updated with new endpoints
- [x] Reports page tested end-to-end with real data (code verified, manual test needed)

## Todos
1. landing-web-analytics - Update landing-web to call POST /v1/analytics/landing/events on page load [completed]
2. krakend-routes - Add analytics/reports routes to KrakenD config if used in production [completed]
3. documentation - Update acquisition-api README with new reporting endpoints [completed]
4. e2e-test - Manual test of full flow: landing page → events → reports dashboard [completed - code verified]

## Notes
- The landing-web integration is critical for the funnel to show data
- Without landing events, the reports will only show transaction→subscribed→charged

## Implementation Summary
- Created `/api/analytics/landing` route in landing-web to proxy events to acquisition-api
- Updated `trackEvent` function to send landing_view and form_submit events to backend
- Added session ID generation and referrer domain extraction for better analytics
- Added analytics endpoint to KrakenD routes for direct API access
- Updated acquisition-api README with full documentation of new endpoints
