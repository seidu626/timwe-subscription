# SUB-DASH-001 verification

Verdict: **Implementation ready for a staged rollout; live activation unverified.**

## Verified

- The live TIMWE Charges Consolidated report exposed a Power BI Report Server CSV export with explicit date, shortcode, and numeric product ID filters. The sampled report showed `REPORT_DATE`, `PARTNER_ID`, `SHORT_CODE`, `PRODUCT_ID`, `QTY_CHARGES`, and `TOTAL_INCOME`; the current-day total changed during observation. No subscriber identifiers were present.
- The importer validates the fixed HTTPS host, Basic Auth configuration, report dates and rows; limits response size and time; blocks redirects; and replaces each source/window in a database transaction. Focused tests cover CSV shape, malformed rows, unsafe URL, unavailable provider totals, and rollback after a failed row insert.
- `GET /v1/admin/reports/subscription-health` is under the existing admin authorization gate, resolves a single tenant, rejects channel and platform aggregate filters, and scopes every query by tenant. The UI keeps local subscription counts distinct from provider billings and does not invent a currency.
- `go test ./...` and `go build ./...` in `services/acquisition-api` passed. The Angular `npm run build` passed after Claude's frontend changes. `docker compose config --quiet` passed for all three compose variants; `git diff --check` passed.
- A local Angular dev server served `/operations`, then the browser routed the unauthenticated session to `/#/login`. This proves the auth boundary was active, but it did not exercise the authenticated Operations tab.
- The production Angular build passed. A Docker-format nginx runtime image assembled from those assets served HTTP 200 and passed its healthcheck after the Dockerfile probe was corrected to `127.0.0.1` (the prior `localhost` probe failed inside the container). The acquisition API image built and contained the binary and new migration.

## Remaining live gates

- A target test or production database was not named, so the migration was not run against a real database. The SQL is reviewed and staged, not empirically applied.
- No authenticated local admin session or deployed build was available for a browser journey of the new tab and tenant isolation. Angular compilation is static evidence only.
- No provider credentials were saved in the repository or deployment. Production activation requires secret rotation/configuration, the database migration, image rollout, and a comparison of the first imported totals with TIMWE's report.
- Current production preflight: the public API `/health` and admin site returned 200, while the new report route returned 404. Both configured droplet SSH identities were rejected, the production PostgreSQL port did not answer from this machine, and local Podman has no Docker Hub login. No live migration or deploy has occurred.
- The report is polled, not streamed. Its daily figures can be revised; the UI marks the current day provisional.

## Release order

1. Confirm source-to-tenant mapping and rotated provider credentials outside Git.
2. Back up the chosen database, apply `services/acquisition-api/migrations/timwe_report_daily.sql`, and verify both new tables.
3. Deploy gateway, acquisition API, and admin frontend; enable the source mapping after the migration.
4. Watch import success/failure logs, compare the first daily totals with TIMWE, and perform an authenticated `/operations` browser journey for the intended tenant and a second tenant.
