# TMP-026 Spec

## Outcome

Admin frontend verification has a precise submodule checkout decision instead of a stale missing-metadata blocker.

## Entry Point

`frontend/webspa-admin` gitlink in the superproject.

## Acceptance

- Verify `.gitmodules` maps `frontend/webspa-admin`.
- Run `git submodule update --init --recursive frontend/webspa-admin`.
- Record the resulting submodule status and any fetch failure.
- Do not edit files inside `frontend/webspa-admin`.
- Reconcile TMP-021 evidence so the full-system blocker names the real cause.

## Non-Goals

- No dependency, vendor, or package-lock changes.
- No replacement of the admin frontend source.
- No submodule pointer change.
- No push to the configured CoreUI remote.

