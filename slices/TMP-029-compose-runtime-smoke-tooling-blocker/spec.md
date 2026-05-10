# TMP-029 Spec

## Outcome

Compose smoke evidence distinguishes config readiness from local Docker auth/tooling failure before app containers start.

## Acceptance

- Release matrix records the bounded smoke attempt after TMP-028.
- Evidence records the temporary Redis host-port override and temporary `shared-network` creation.
- Evidence records that compose failed before app startup on the Go builder image pull.
- Direct image pull reproduction is recorded.
- Cleanup evidence records no remaining smoke containers, temporary override file, or temporary network.
- No source, compose, dependency, vendor, package manifest, lockfile, or frontend files change.

## Non-Goals

- No Docker auth repair or registry credential handling.
- No dependency update or base-image change.
- No `docker-compose.yml` edit.
- No runtime/live-provider claim.

