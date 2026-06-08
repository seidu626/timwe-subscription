# Claude Code + Cursor Integration Guide

A reference guide for setting up AI-assisted development workflows with Claude Code and Cursor IDE.

## Overview

This guide covers:
1. Claude Code project memory and modular rules
2. Claude Code custom slash commands
3. Claude Code permissions configuration
4. Cursor project rules with proper frontmatter

---

## 1. Claude Code Integration

### 1.1 Project Memory (`CLAUDE.md`)

Claude Code automatically loads `CLAUDE.md` from the repo root. This file defines the AI's "prime directive" for the project.

**Location:** `./CLAUDE.md` or `./.claude/CLAUDE.md`

**Example:**

```markdown
# Project workflow (AI-assisted)

## Prime directive
- Keep diffs minimal; do not refactor unrelated code.
- Preserve public APIs unless explicitly required.
- If behavior changes, add/adjust tests (fail-before/pass-after).

## Commands (use these)
- Tests: `just test`
- Gateway config: `just krakend-query-forwarding-check`
- Command catalog: `just --list`

## Review bundle
- When a change is non-trivial, generate a review bundle:
  - `python tools/review_bundle.py --base origin/main --cmd "just test" --cmd "just krakend-query-forwarding-check"`

## Codex handoff format
- Preferred handoff artifact is a git diff + test logs (use review bundle output).
- When asked to prepare PR instructions for Codex, produce a paste-ready `@codex ...` comment.

## Imported repo standards
- See @AGENTS.md
```

**Key features:**
- The `@path/to/file` syntax imports other files (e.g., `@AGENTS.md`)
- Verify loaded memory with `/memory` command in Claude Code
- In monorepos, Claude Code loads `CLAUDE.md` recursively up the directory tree

### 1.2 Modular Rules (`.claude/rules/`)

For topic-based rules that Claude Code loads automatically.

**Location:** `.claude/rules/*.md`

**Example: `.claude/rules/minimal-diff.md`**

```markdown
# Minimal diffs rule

- Prefer the smallest diff that satisfies requirements.
- Do not rename symbols or reformat unrelated code.
- Keep changes localized to the requested area.
- Explain any unavoidable behavior changes explicitly.
```

**Example: `.claude/rules/tests.md`**

```markdown
# Tests rule

- For behavior changes, add/adjust tests that fail-before/pass-after.
- Add edge cases (invalid inputs, boundaries, error handling).
- Avoid flaky timing assertions; prefer deterministic checks.
```

### 1.3 Custom Slash Commands (`.claude/commands/`)

Create custom `/project:<name>` commands by adding markdown files.

**Location:** `.claude/commands/<name>.md`

The filename becomes the command name: `review-bundle.md` creates `/project:review-bundle`

**Example: `.claude/commands/review-bundle.md`**

```markdown
---
description: Generate a review bundle for the current branch
allowed-tools: Bash, Read, Write
---

Generate a review bundle for the current branch vs origin/main.

Steps:
1) Run:
   `python tools/review_bundle.py --base origin/main --out review_bundle.md --cmd "just test" --cmd "just krakend-query-forwarding-check"`
2) Summarize:
   - What changed (top 5 bullets)
   - Any test failures and likely root cause
3) Output a paste-ready "review bundle" section I can paste into ChatGPT/Codex.
```

**Example: `.claude/commands/codex-task.md`**

```markdown
---
description: Prepare a paste-ready PR comment for Codex
allowed-tools: Read, Write
---

You are preparing a paste-ready PR comment for Codex.

Input: The review bundle (diff + test output) + constraints.

Output:
- A short prioritized fix list (P0/P1/P2)
- A paste-ready `@codex` comment that:
  - forbids unrelated refactors
  - lists fixes in order
  - requests verification commands/results
```

**Command features:**
- `$ARGUMENTS` - captures all arguments passed to the command
- `$1`, `$2`, etc. - positional arguments
- Subdirectories create namespaced commands: `.claude/commands/frontend/component.md` → `/project:frontend:component`

### 1.4 Permissions (`.claude/settings.json`)

Configure which commands Claude Code can run without asking permission.

**Location:** `.claude/settings.json` (team-shared) or `~/.claude.json` (personal)

**Example:**

```json
{
  "permissions": {
    "allow": [
      "Bash(python tools/review_bundle.py*)",
      "Bash(git diff*)",
      "Bash(git status*)",
      "Bash(git log*)",
      "Bash(just --list)",
      "Bash(just test)",
      "Bash(just krakend-query-forwarding-check)"
    ],
    "deny": []
  }
}
```

**Important:** Tool names must start with uppercase (e.g., `Bash(...)`, not `bash(...)`).

**Common tool patterns:**
- `Bash(command*)` - shell commands with wildcards
- `Read` - file reading
- `Write` - file writing
- `Edit` - file editing

---

## 2. Cursor Integration

### 2.1 Project Rules (`.cursor/rules/`)

Cursor supports `.mdc` files with YAML frontmatter for conditional rule injection.

**Location:** `.cursor/rules/*.mdc`

**Example: `.cursor/rules/00-global-minimal-diff.mdc`**

```yaml
---
description: "Always apply: keep diffs minimal, preserve APIs, and require tests for behavior changes."
globs: []
alwaysApply: true
---

- Keep diffs minimal; do not refactor unrelated code.
- Preserve public APIs unless explicitly requested.
- If behavior changes, add/adjust tests (fail-before/pass-after).
- Avoid renames or formatting-only churn unless explicitly requested.
```

**Example: `.cursor/rules/10-tests-only.mdc`**

```yaml
---
description: "Use when editing tests: add meaningful edge cases and avoid flaky assertions."
globs: ["**/*test*.*", "**/*spec*.*"]
alwaysApply: false
---

- Add edge cases (invalid inputs, boundary values, error handling).
- Avoid timing-based flake; prefer deterministic assertions.
- Match existing test style and helpers.
```

### 2.2 Frontmatter Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Helps the model decide when to apply the rule |
| `globs` | array | File patterns that trigger the rule (e.g., `["**/*.ts"]`) |
| `alwaysApply` | boolean | If `true`, rule is always injected regardless of context |

**Common gotchas:**
- `globs` must be an array: `globs: ["**/*.ts"]` not `globs: "**/*.ts"`
- Empty globs for global rules: `globs: []` not `globs:`
- If rules don't appear, try "Reload Window" in Cursor

### 2.3 Verify Rules in Cursor

1. Go to **Cursor Settings → Rules → Project Rules**
2. Confirm your rules appear with correct activation settings
3. Test by editing a file matching your glob pattern

---

## 3. AGENTS.md (Codex/AI Review Rubric)

A shared rubric file that can be imported into `CLAUDE.md` and used across tools.

**Location:** `./AGENTS.md`

**Example:**

```markdown
# AGENTS.md

## Setup & verification commands (use these exact commands)
- Test: `just test`
- Gateway config: `just krakend-query-forwarding-check`
- Command catalog: `just --list`

## Review rubric (P0/P1 only)
### P0 (must fix)
- Auth/authz bypass (missing middleware, object-level auth failures)
- Injection risks (SQL/command injection, SSRF), unsafe deserialization
- Secrets/PII leakage in logs or telemetry
- Data loss / irreversible migrations without rollback
- Broken builds / failing CI

### P1 (strongly recommended)
- Missing tests for behavior changes
- Likely correctness regressions / unhandled edge cases
- Major perf regressions (N+1 queries, unbounded loops)

## Change discipline
- Prefer minimal diffs (no unrelated refactors).
- Preserve public APIs unless explicitly required.
- If behavior changes, add tests that fail-before/pass-after.
```

---

## 4. Review Bundle Script

A Python script to generate consistent handoff artifacts.

**Location:** `tools/review_bundle.py`

**Usage:**

```bash
# Basic usage
python tools/review_bundle.py --base origin/main --out review_bundle.md

# With test and lint output
python tools/review_bundle.py --base origin/main --cmd "just test" --cmd "just krakend-query-forwarding-check"

# Custom diff line limit
python tools/review_bundle.py --base origin/main --max-diff-lines 2500
```

**Output includes:**
- Branch and SHA information
- Changed files list
- Truncated unified diff
- Command outputs (tests, lint)
- Paste-ready review request template

---

## 5. Directory Structure Summary

```
your-repo/
├── CLAUDE.md                    # Claude Code project memory
├── AGENTS.md                    # Shared AI review rubric
├── .claude/
│   ├── commands/
│   │   ├── review-bundle.md     # /project:review-bundle
│   │   └── codex-task.md        # /project:codex-task
│   ├── rules/
│   │   ├── minimal-diff.md      # Modular rule
│   │   └── tests.md             # Modular rule
│   └── settings.json            # Permissions allowlist
├── .cursor/
│   └── rules/
│       ├── 00-global-minimal-diff.mdc
│       └── 10-tests-only.mdc
└── tools/
    └── review_bundle.py         # Review bundle generator
```

---

## 6. Verification Checklist

### Claude Code
- [ ] Run `/memory` to confirm `CLAUDE.md` and rules are loaded
- [ ] Test `/project:review-bundle` command
- [ ] Test `/project:codex-task` command
- [ ] Verify allowed commands run without permission prompts

### Cursor
- [ ] Settings → Rules → Project Rules shows your rules
- [ ] Global rule (alwaysApply: true) activates on any file
- [ ] Test rule activates when editing `*test*` files
- [ ] Reload Window if rules don't appear

---

## 7. Workflow Integration

### Recommended Loop

1. **Implement** in Cursor (Agent/Composer mode)
2. **Generate bundle**: `python tools/review_bundle.py --base origin/main --cmd "just test" --cmd "just krakend-query-forwarding-check"`
3. **Review** by pasting `review_bundle.md` into Claude Code (`/project:codex-task`)
4. **Apply fixes** in Cursor
5. **Push PR**

### Prompts

**For Cursor (writer role):**

```
Implement <goal> with a minimal diff.

Constraints:
- No unrelated refactors, renames, or formatting-only edits.
- Preserve public APIs unless required.
- Add/adjust tests (fail-before/pass-after).

After coding:
- Run: just test && just krakend-query-forwarding-check
- Summarize changes + verification results.
```

**For Claude Code (reviewer role):**

```
Use the review bundle in review_bundle.md.

1) Identify P0/P1 issues.
2) Produce a minimal fix checklist.
3) Output a paste-ready @codex comment to implement the fixes.
4) Include exact verification commands.
```

---

## 8. Troubleshooting

| Issue | Solution |
|-------|----------|
| Claude Code doesn't load rules | Restart Claude Code, check `/memory` |
| Cursor rules don't appear | Reload Window, verify frontmatter syntax |
| `globs` not matching | Use array format: `["**/*.ts"]` not string |
| Permission errors in Claude Code | Tool names must start with uppercase: `Bash(...)` |
| `/project:*` command not found | Restart Claude Code after adding command files |
| Rules duplicated | Check for both `.mdc` files and `RULE.md` folders |
