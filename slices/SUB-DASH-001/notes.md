# Build notes

The existing acquisition Reports handler, route, and Angular Operations component remain canonical. The provider import needs new files because no report collector, provider daily store, or migration currently exists.

prune: approve services/acquisition-api/internal/worker/timwe_report_sync.go provider CSV fetch and periodic sync owner
prune: approve services/acquisition-api/internal/worker/timwe_report_sync_test.go parser and retry behavior proof
prune: approve services/acquisition-api/internal/repository/timwe_report_repository.go atomic daily replacement and tenant-scoped report query
prune: approve services/acquisition-api/internal/repository/timwe_report_repository_test.go persistence and tenant isolation proof
prune: approve services/acquisition-api/migrations/timwe_report_daily.sql provider daily aggregate schema and sync state
prune: approve docs/timwe-subscription-dashboard.md operator config, rollout, rollback and data meanings

Frontend owner: Claude worker, limited to existing Operations component files and `subscription-ops.service.ts`. Backend owner: Codex. Shared source tree: neither owner reverts the other's edits.
