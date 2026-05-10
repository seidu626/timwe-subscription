# TMP-046 Domain Brief: webspa-admin Reproducible Source

## Actors

- Platform operator: runs full-system verification and needs the admin frontend to build/test from a clean checkout. Source: `agent/backlog/issues/TMP-026-webspa-submodule-verification.md`.
- Tenant admin: uses the admin frontend tenant workspace, route guards, product/userbase/campaign/report/postback surfaces. Source: `frontend/webspa-admin/src/app/app.routes.ts`.
- Agent verifier: checks repository metadata and frontend commands without relying on local-only nested repositories. Source: `slices/TMP-026-webspa-submodule-verification/value-gate-report.md`.

## Ubiquitous Language

- Gitlink: superproject entry that points at a submodule commit instead of tracked source. Source: `git ls-tree HEAD frontend/webspa-admin`.
- Pinned admin checkout: local `frontend/webspa-admin` commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a` that contains tenant workspace guardrails. Source: local nested checkout log.
- Tracked source strategy: keeping the admin source in this superproject so package install, build, and tests do not need submodule fetch.
- Tenant workspace guardrails: admin route guards, tenant context service, interceptor, and denial UI that were added in the pinned admin commit. Source: `frontend/webspa-admin/src/app/core/guards/tenant-workspace.guard.ts`.

## Domain Invariants

- The tenant admin source that passed build/test must be preserved; replacing it with upstream CoreUI main would lose tenant workspace behavior.
- A clean superproject checkout must contain `frontend/webspa-admin/package.json` without running `git submodule update`.
- Admin verification must be executable from tracked source with the lockfile.
- Source-control metadata must have a single canonical path: either submodule or tracked source, not both.

## Failure Modes

- Missing required: `.gitmodules` points at a commit not available from the public remote, so clean checkout verification fails.
- Duplicate/conflict: keeping both a gitlink and tracked files would create ambiguous ownership.
- Dependency failure: npm install can complete with vulnerability/engine warnings; those are recorded but not remediated by this slice.
- Regression: tenant workspace guardrail files disappear if repointed to CoreUI upstream; this slice avoids that by materializing the pinned checkout.

## User Journey

1. Platform operator checks out the superproject.
2. `frontend/webspa-admin/package.json` exists immediately as tracked source.
3. Verifier runs `npm ci`, `npm run build`, and ChromeHeadless tests from the admin directory.
4. Tenant admin UI verification is no longer blocked by `upload-pack: not our ref`.

Failure journeys:

1. Admin source is missing -> `test -f frontend/webspa-admin/package.json` fails.
2. Source-control keeps the old submodule -> clean clone may still attempt the unreachable gitlink.
3. Build/test regress -> value gate fails before TMP-021 can close.

## Open Questions

- A later architecture cleanup should decide whether the admin frontend deserves its own first-party repository again once there is a reachable remote under project control.
