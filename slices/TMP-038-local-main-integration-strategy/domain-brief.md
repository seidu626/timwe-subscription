# TMP-038 Domain Brief

- Actor: repo-maintainer
- Business outcome: The divergent primary local main branch is reconciled with origin/main through an explicit integration strategy instead of accidental merge conflict resolution.
- Domain invariant: full-system verification must not claim end-to-end readiness while this blocker remains unresolved.
- Entrypoint: /home/xper626/workspace/apps/timwe-subscription main branch
- Trigger: Verifier compares primary checkout main against origin/main during full-system verification.
- Risk: Destructive or broad conflict-resolution branch operations require explicit maintainer direction. Primary main contains local-only history that must not be discarded by an agent.

## Story Craft

The story is concrete and testable: Primary local main is diverged from origin/main. The 2026-05-09T08:44:16Z evidence refresh showed 51 ahead / 38 behind, and an isolated merge probe produced broad add/add conflicts; exact behind counts change as origin/main receives evidence-only commits. The expected outcome is: A maintainer chooses whether to preserve local-only history, reset to remote, or manually integrate the divergent histories before treating primary main as verified.

## Roadmap To Slices

This is a blocked follow-up slice under TMP-021. It records the smallest independently verifiable blocker without implementing approval-gated changes.
