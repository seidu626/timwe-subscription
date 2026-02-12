# Message Cadence Engine

## Status
- Owner: agent
- Status: completed
- Started: 2026-01-17
- Completed: 2026-01-17

## Dependencies
- None

## ExitCriteria
- [x] Cadence tables and indexes migrated
- [x] `services/cadence-engine` plans + advances message state
- [x] `services/notification` worker dispatches outbox jobs
- [x] Compose/K8s + Makefile wiring in place

## Todos
1. task-records - Create task record + log entry [completed]
2. db-migration - Add cadence DB migration [completed]
3. cadence-engine-service - Add cadence-engine service [completed]
4. notification-worker - Add notification worker [completed]
5. ops-wiring - Wire compose/k8s + Makefile [completed]

## Notes
- Planner uses `FOR UPDATE SKIP LOCKED` + idempotency keys.
- Dispatcher will call subscription-external Partner MT endpoint in phase 1.
