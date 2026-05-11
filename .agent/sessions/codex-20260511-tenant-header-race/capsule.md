# Session Capsule: codex-20260511-tenant-header-race

Task: `TMP-060`
Status: `done`

## Summary

Fixed the frontend tenant header race so report/KPI calls are not sent until tenant workspace state resolves and X-Tenant-Key can be attached.

## Completed Work

- Created TMP-060 for the tenant_context_required report race.
- Traced KPI 403s to TenantWorkspaceInterceptor taking the initial loading workspace emission.
- Changed the interceptor to wait until tenant workspace loading is false before forwarding workspace API requests.
- Added a focused unit test proving delayed tenant resolution still attaches X-Tenant-Key.
- Restarted the admin dev server from the patched worktree.

## Unfinished Work


## Next Tasks

