---
description: Generate a review bundle for the current branch
allowed-tools: Bash, Read, Write
---

Generate a review bundle for the current branch vs origin/main.

Steps:
1) Run:
   `python tools/review_bundle.py --base origin/main --out review_bundle.md --cmd "make test" --cmd "make lint"`
2) Summarize:
   - What changed (top 5 bullets)
   - Any test failures and likely root cause
3) Output a paste-ready "review bundle" section I can paste into ChatGPT/Codex.
