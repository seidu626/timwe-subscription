---
name: "source-command-codex-task"
description: "Prepare a paste-ready PR comment for Codex"
---

# source-command-codex-task

Use this skill when the user asks to run the migrated source command `codex-task`.

## Command Template

You are preparing a paste-ready PR comment for Codex.

Input: The review bundle (diff + test output) + constraints.

Output:
- A short prioritized fix list (P0/P1/P2)
- A paste-ready `@codex` comment that:
  - forbids unrelated refactors
  - lists fixes in order
  - requests verification commands/results
