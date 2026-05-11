# Session Capsule: codex-20260511-021956

Task: `TMP-057`
Status: `done`

## Summary

Fixed acquisition-api admin reporting scope resolution so tenant-key-only identities resolve to a canonical tenant_id before report queries run, while preserving the platform-only all_tenants guard.

## Completed Work

- Created TMP-057 defect slice and work order.
- Updated reports filter parsing to resolve tenant keys through the tenant catalog.
- Added handler tests for tenant-scoped tenant key resolution and unknown tenant key rejection.
- Rebuilt and restarted live acquisition-api on port 8084 with the backend bootstrap admin emails set.

## Unfinished Work


## Next Tasks

