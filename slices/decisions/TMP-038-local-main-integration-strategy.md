# TMP-038 Decision Template: Local Main Integration Strategy

Status: accepted

Approval recorded: yes

## Context

The primary checkout `main` is clean but diverged from `origin/main`. The 2026-05-09T08:44:16Z evidence snapshot showed `51 ahead / 38 behind`, with `origin/main` at `bad5fe156f876938ff10895a5a330178c95bb8de`; exact behind counts change as `origin/main` receives evidence-only commits.

Earlier isolated merge probing found broad add/add conflicts. Branch resets, conflict resolution, or deletion could discard local-only history, so maintainer direction is required.

## Decision Required

Choose one integration strategy:

- Preserve local-only history and manually integrate conflicts.
- Treat `origin/main` as source of truth and archive/reset local `main` with explicit approval.
- Split local-only commits into reviewed branches before reconciling `main`.

## Decision

Preserve the primary local `main` history and do not run destructive branch operations, conflict-heavy merges, or reset from this agent. Treat the origin/main-derived isolated worktree branch `agent/codex/fullsystem-20260510-045911` as the current release verification surface. Primary local `main` reconciliation remains a separate maintainer-owned integration activity.

Approval source: operator auto-proceed directive in this Codex session on 2026-05-10.

## Consequences To Review

- Local-only commits and generated artifacts are preserved.
- The primary checkout remains a local workbench until maintainer reconciliation.
- Conflict resolution ownership stays with the maintainer, not the autonomous release verifier.
- Current release verification chooses the isolated origin/main-derived worktree branch as source truth.

## Post-Decision Proof

```bash
git -C /home/xper626/workspace/apps/timwe-subscription status --short --branch --untracked-files=all
git -C /home/xper626/workspace/apps/timwe-subscription rev-list --left-right --count main...origin/main
# plus maintainer-selected integration verification
```

## Slice Impact

- Blocks: `TMP-021`, `TMP-038`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-038.handoff.json`
