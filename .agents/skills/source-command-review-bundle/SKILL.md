---
name: "source-command-review-bundle"
description: "Generate a review bundle for the current branch"
---

# source-command-review-bundle

Use this skill when the user asks to run the migrated source command `review-bundle`.

## Command Template

Generate a review bundle for the current branch vs origin/main.

Steps:
1) Run:
   `python tools/review_bundle.py --base origin/main --out review_bundle.md --cmd "make test" --cmd "make lint"`
2) Summarize:
   - What changed (top 5 bullets)
   - Any test failures and likely root cause
3) Output a paste-ready "review bundle" section I can paste into ChatGPT/Codex.
