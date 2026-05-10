# TMP-046 Spec: webspa-admin Reproducible Source

## Story

As a platform operator, I want `frontend/webspa-admin` to be present as reproducible tracked source, so full-system verification can build and test the tenant admin UI without fetching an unreachable submodule commit.

## Scope

In scope:
- Remove the broken `.gitmodules` webspa-admin submodule metadata.
- Replace the gitlink with the exact source tree from pinned local commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- Run `npm ci`, `npm run build`, and ChromeHeadless tests from the tracked source checkout.
- Reconcile TMP-026 evidence from blocked submodule status to passed tracked-source status.

Out of scope:
- Dependency vulnerability remediation.
- Angular/Node version upgrade.
- UI redesign or feature change beyond preserving pinned source.

## Acceptance Criteria

1. `.gitmodules` is absent and no longer declares `frontend/webspa-admin`.
2. `frontend/webspa-admin/package.json` exists as tracked source.
3. `npm ci` completes from `frontend/webspa-admin`.
4. `npm run build` passes from `frontend/webspa-admin`.
5. `CHROME_BIN=/usr/bin/google-chrome-stable npm test -- --watch=false --browsers=ChromeHeadless --progress=false` reports `TOTAL: 84 SUCCESS`.
6. TMP-026 value-gate report is updated with the replacement strategy and current evidence.

## Architecture Notes

This applies the `/prune` single-canonical-path rule in cleanup mode: the previous codebase had a submodule path whose source truth was local-only and unreproducible. The tracked source strategy collapses the dual path into one canonical module interface for the verifier: `frontend/webspa-admin` is just a normal source directory in the superproject.
