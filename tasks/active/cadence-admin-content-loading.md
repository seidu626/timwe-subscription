# Cadence Admin + Content Loading (CSV)

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-17
- Completed: 2026-01-17

## Dependencies
- Message Cadence Engine (completed)

## ExitCriteria
- [x] Cadence-engine exposes admin API for series/rules/content + CSV import (dry-run + apply)
- [x] Web admin UI can manage series/rules/content and import CSV
- [x] Tests cover CSV parsing + validation + bulk upsert behavior

## Todos
1. cadence-admin-task - Create new task record under /tasks and link it in TaskIndex Now [completed]
2. cadence-admin-api - Add cadence-engine admin HTTP server with token auth + CORS; CRUD endpoints for series/rules/content + CSV import dry-run/apply [completed]
3. cadence-admin-repo - Extend cadence-engine repository with queries for list/create/update and bulk content upsert + deactivate-missing [completed]
4. cadence-admin-ui - Add webspa-admin Cadence screens (series/rules/content/CSV import) and hook auth token [completed]
5. cadence-admin-tests - Add unit/integration tests for CSV parsing + validation + repository upsert behavior [completed]

## Notes
- Phase 1 runtime stays “send final text from DB”: `message_content_items.message_text` is authoritative at send time.
- CSV import must be audit-friendly: upsert + mark missing inactive (no deletes).
- Implementing cadence-engine admin HTTP server (net/http) with X-Admin-Token auth + CORS; endpoints next.
- Wired admin HTTP server into `services/cadence-engine/cmd/cadence-engine/main.go` (listens on `CADENCE_ADMIN_HTTP_ADDR`, default `:8091`).
- Ran `go mod tidy` for `services/cadence-engine` to fix missing go.sum entries; build now succeeds.
- Implemented cadence-engine admin API endpoints for series/rules/content and CSV import (dryRun/apply) in `services/cadence-engine/internal/adminhttp/server.go`.
- Ran `go fmt ./...` and `go test ./...` in `services/cadence-engine` (tests pass).
- Started webspa-admin Cadence UI: new `/cadence` route + nav item + module/component + API service + env config.
- Verified `webspa-admin` typecheck (`npx tsc --noEmit`) and `cadence-engine` tests (`go test ./...`) after UI/API alignment changes.
- Exposed cadence-engine admin HTTP port 8091 in compose and added cadence-engine Service/port + admin env vars in k8s for UI access.
