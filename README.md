# Codex / Cursor / Claude Pipeline Pack

This pack contains a practical, repo-drop-in set of files for a **diff-centered** AI development pipeline:

- **Cursor / Claude Code** for day-to-day implementation (small slices)
- **Codex** for PR reviews, gating, and/or follow-up tasks
- Optional: **GitHub Actions** for review comments and “gate” checks
- A **review bundle generator** you can paste into any model (ChatGPT/Codex/Claude)

## What’s inside

### Repo instructions & IDE rules
- `AGENTS.md` — guidance for Codex reviews/tasks
- `.cursor/rules/*.mdc` — Cursor project rules (minimal diffs, test discipline)

### Local tools
- `tools/review_bundle.py` — generates a Markdown review bundle from git diffs + test logs
- `tools/parse_codex_focus.py` — parses a `/codex review focus=...` whitelist
- `tools/codex_command_router.py` — strict router for `/codex review|gate|fix-ci`

### GitHub Actions workflows (optional)
- `.github/workflows/codex-review-internal.yml`
  - Auto-runs Codex review on **same-repo** PRs and posts a comment
- `.github/workflows/codex-gate-internal.yml`
  - Produces structured JSON output and **fails the check on P0**
- `.github/workflows/codex-router.yml`
  - Maintainer comment commands:
    - `/codex review` -> posts `@codex review` (if you have Codex GitHub integration installed)
    - `/codex fix-ci` -> posts an `@codex` task comment
    - `/codex gate` -> runs the **diff-only** structured gate via codex-action
- `.github/workflows/codex-review-on-comment.yml`
  - Minimal “public repo safe” maintainer-triggered diff-only review (no checkout of PR head)

### Prompts and schema
- `.github/codex/prompts/review.md` — markdown review prompt (internal PR review)
- `.github/codex/prompts/review-diff.md` — diff-only review prompt (safe for forks)
- `.github/codex/prompts/review_gate.md` — structured JSON gating prompt
- `.github/codex/prompts/fix-ci.md` — fix-CI prompt (for @codex tasks or action-based runs)
- `.github/codex/schemas/review_gate.schema.json` — JSON schema used for gating/automation

### Examples
- `examples/codex_config.toml` — suggested `~/.codex/config.toml` profiles
- `examples/default.rules` — suggested `~/.codex/rules/default.rules`
- `ci/codex_gate.sh` — local/CI gating script using `codex exec --output-schema`

---

## Quick start

### 1) Add files to your repo
Copy everything into your repository root (keep the same paths).

### 2) Add the OpenAI key (for GitHub Actions)
Add a GitHub Actions secret:
- `OPENAI_API_KEY` (required by `openai/codex-action`)

### 3) Pick ONE workflow strategy

#### Strategy A — Use Codex GitHub integration (preferred for patching)
- Install/configure the Codex GitHub integration (so `@codex ...` comments work).
- Enable `.github/workflows/codex-router.yml` if you want `/codex ...` shorthand commands.
- Use:
  - `@codex review`
  - `@codex fix the CI failures`
  - `@codex address these review items ...`

#### Strategy B — Use GitHub Actions only (no Codex integration needed)
- Enable:
  - `codex-review-internal.yml` (internal PR review)
  - `codex-gate-internal.yml` (structured gate)
  - and/or `codex-review-on-comment.yml` (public safe diff-only review)

> Tip: for public repos, prefer workflows that **do not check out PR head** for fork PRs.

---

## Daily driver workflow (recommended)

1) Implement a small slice in Cursor/Claude (minimal diff, add/adjust tests)
2) Run your check command (e.g., `make test && make lint`)
3) Open/update PR
4) Run review:
   - `@codex review` (or let the action post a review)
5) Convert review into a minimal fix list (optionally with ChatGPT)
6) Apply fixes locally OR ask Codex to implement them via PR comment
7) Merge, repeat

---

## Using the review bundle generator

```bash
python tools/review_bundle.py \
  --base origin/main \
  --out review_bundle.md \
  --cmd "make test" \
  --cmd "make lint"
```

Paste `review_bundle.md` into your reviewer model of choice.

---

## Notes / assumptions

- These workflows assume GitHub-hosted runners have `python3`, `jq`, and `curl` available (standard Ubuntu runners do).
- Versions of actions (checkout, github-script) may change; update as needed.
- Always treat PR content (titles, bodies, diffs) as **untrusted input**.
