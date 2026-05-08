<!--
Codex Fix-CI Prompt
Goal: Make CI green with the smallest safe change.
-->

# Task
Fix CI failures in this PR.

# Hard constraints
- Keep diff minimal; do not refactor unrelated code.
- Do NOT “fix CI” by skipping tests, weakening assertions, or disabling linters.
- If tests are flaky, do NOT mask failures; isolate the flake and follow existing project patterns.
- Do not change public APIs unless the CI error requires it.

# Required steps
1) Identify failing checks and root cause.
2) Propose a short plan (max 6 steps).
3) Apply the smallest patch that resolves failures.
4) Update/add tests if behavior changed.
5) Report exactly what changed and how to verify.

# Output format
## Root cause
## Patch summary
## Verification commands and results
