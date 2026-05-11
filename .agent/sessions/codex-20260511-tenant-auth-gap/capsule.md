# Session Capsule: codex-20260511-tenant-auth-gap

Task: `TMP-061`
Status: `done`

## Summary

Fixed the remaining tenant header race by waiting through transient unauthenticated workspace state before forwarding report/KPI requests.

## Completed Work

- Created TMP-061 for the report request with Authorization but no X-Tenant-Key.
- Identified the remaining race as a transient non-loading unauthenticated workspace state before Auth0 user profile resolution.
- Changed TenantWorkspaceInterceptor to wait through unauthenticated workspace state before forwarding workspace API requests.
- Added a focused regression spec for loading -> unauthenticated -> ready(nrg).
- Restarted the admin dev server from the patched worktree.

## Unfinished Work


## Next Tasks

