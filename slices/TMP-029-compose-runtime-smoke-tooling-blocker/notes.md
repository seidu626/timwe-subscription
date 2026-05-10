# TMP-029 Notes

## Domain Grounding

- Actor: verification-agent.
- Business outcome: operators can see the next concrete compose-runtime unblock is local Docker registry auth/tooling before app health evidence can be collected.
- Domain invariant: config render is not runtime verification, and pre-start tooling failures are not app runtime failures.
- Entrypoint: `docs/agent/full-system-verification-2026-05-09.md`.
- Risk: overstating runtime readiness or mutating Docker credentials while trying to fix a local auth failure.

## Story Craft

The story is concrete and testable: a bounded compose smoke was attempted with temporary local-only scaffolding, failed before app containers started, and the exact blocker plus cleanup evidence is recorded.

## Value Gate

Pass criteria:
- HVC allows the slice.
- Release evidence records the compose smoke attempt and blocker.
- Direct image pull reproduction is recorded.
- Cleanup checks show the temporary network and override file are gone.
- File-scope check shows no forbidden source, compose, dependency, frontend, package manifest, lockfile, or vendor files changed.

## Claude Critique

Claude delegation was attempted for read-only TMP-029 critique but did not return output and was killed after hanging. The local critique narrowed the slice from broad full-system language to a bounded compose-smoke evidence update, which made HVC assignment `allow`.

