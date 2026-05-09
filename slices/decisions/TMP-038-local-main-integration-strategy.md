# TMP-038 Decision Template: Local Main Integration Strategy

Status: proposed

Approval recorded: no

## Context

The primary checkout `main` is clean but diverged from `origin/main`. The 2026-05-09T08:44:16Z evidence snapshot showed `51 ahead / 38 behind`, with `origin/main` at `bad5fe156f876938ff10895a5a330178c95bb8de`; exact behind counts change as `origin/main` receives evidence-only commits.

Earlier isolated merge probing found broad add/add conflicts. Branch resets, conflict resolution, or deletion could discard local-only history, so maintainer direction is required.

## Decision Required

Choose one integration strategy:

- Preserve local-only history and manually integrate conflicts.
- Treat `origin/main` as source of truth and archive/reset local `main` with explicit approval.
- Split local-only commits into reviewed branches before reconciling `main`.

## Decision

Pending maintainer decision.

## Consequences To Review

- Local-only commits and generated artifacts.
- Whether primary checkout should track release source truth or remain a local workbench.
- Conflict resolution ownership.
- How future verification should choose source truth.

## Post-Decision Proof

```bash
git -C /home/xper626/workspace/apps/timwe-subscription status --short --branch --untracked-files=all
git -C /home/xper626/workspace/apps/timwe-subscription rev-list --left-right --count main...origin/main
# plus maintainer-selected integration verification
```

## Slice Impact

- Blocks: `TMP-021`, `TMP-038`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-038.handoff.json`
