# Campaign URLs + Outbound Click Redirect

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-17
- Completed: 2026-01-17

## Dependencies
- Campaign admin CRUD (completed)

## ExitCriteria
- [x] Campaign admin create/update accepts `landing_page_urls` (multiple URLs) and persists it.
- [x] Admin UI allows editing `landing_page_urls` and Preview uses first configured URL (fallback to default).
- [x] Public click-out endpoint exists: `GET /v1/click/out` mints click_id, persists record, sets cookie, and 302/303 redirects to allowlisted destination with click param appended.
- [x] Downstream landing/transaction flow preserves click_id so conversion postbacks can use it (cookie fallback implemented).
- [x] Tests added for URL validation and click-out redirect validation.
- [x] Docs updated for campaign model + click-out endpoint; missing campaign-management features list updated.

## Notes
- Use allowlist to avoid open-redirect vulnerabilities.
- Store hashes for IP/UA (not raw) and only referrer domain (not full referrer URL).

