# TMP-026 Domain Brief

- Actor: platform-operator
- Business outcome: Admin frontend verification has a precise submodule checkout decision instead of a stale missing-metadata blocker.
- Domain invariant: full-system verification must not claim admin UI readiness when the tracked `frontend/webspa-admin` gitlink cannot be initialized from the configured remote.
- Entrypoint: `frontend/webspa-admin` gitlink
- Trigger: Operator runs full-system verification and admin UI checks.
- Risk: Repointing, replacing, or editing the admin frontend is a repository ownership decision; this slice may only record checkout evidence and blocker cause.

## Story Craft

The story is concrete and testable: run submodule metadata and checkout commands, then record whether the pinned commit can be fetched. If the commit is unavailable, the slice remains blocked with the exact remote and commit.

## Roadmap To Slices

TMP-026 is a blocked operational verification slice under TMP-021. A future implementation requires an operator decision to publish, repoint, or replace the admin frontend gitlink.
