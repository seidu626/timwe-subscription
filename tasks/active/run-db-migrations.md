# Run DB migrations (local + remote)

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-17
- Completed: 2026-01-17

## Dependencies
- None

## ExitCriteria
- [x] Migrations applied successfully to local DB
- [x] Migrations applied successfully to remote DB

## Todos
1. run-local - Run `make migrate` against local DB [completed]
2. run-remote - Run `make migrate` against remote DB [completed]

## Notes
- `psql` is not installed locally; used Docker (`docker exec` for local, `docker run postgres:16` for remote) to run migrations.
- Local DB: `subscription_manager_db` container on port 5434 (localhost)
- Remote DB: `139.59.135.253:5432`
- Migration file: `services/subscription-external/migrations/011_message_cadence_engine.sql`
- Tables created on both DBs:
  - `product_message_series`
  - `message_schedule_rules`
  - `message_content_items`
  - `subscription_message_state`
  - `message_outbox`
