#!/usr/bin/env python3

"""
review_bundle.py

Generate a Markdown "review bundle" from a git repo:
- Branch + SHAs
- Changed files + stats
- Truncated unified diff
- Optional command outputs (tests, lint, etc.)

Why:
- Makes handoffs between Cursor/Claude/Codex/ChatGPT consistent.
- Prevents "context thrash" across multiple agents.

Usage examples:
  python tools/review_bundle.py --base origin/main --out review_bundle.md
  python tools/review_bundle.py --base main --cmd "make test" --cmd "make lint"
  python tools/review_bundle.py --base origin/main --max-diff-lines 2500
"""

from __future__ import annotations

import argparse
import subprocess
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import List, Tuple


@dataclass(frozen=True)
class CmdResult:
    """Result of executing a shell command."""
    cmd: str
    exit_code: int
    stdout: str
    stderr: str


def run(cmd: List[str]) -> Tuple[int, str, str]:
    """
    Run a command and return (exit_code, stdout, stderr).
    Uses text mode and captures output for embedding into Markdown.
    """
    proc = subprocess.run(cmd, text=True, capture_output=True)
    return proc.returncode, proc.stdout, proc.stderr


def require_git_repo() -> None:
    """Exit early if we're not inside a git repository."""
    code, out, _ = run(["git", "rev-parse", "--is-inside-work-tree"])
    if code != 0 or out.strip() != "true":
        raise SystemExit("ERROR: Not inside a git repository.")


def detect_head() -> str:
    """Return HEAD SHA."""
    _, out, _ = run(["git", "rev-parse", "HEAD"])
    return out.strip()


def detect_branch() -> str:
    """Return current branch name (or detached HEAD)."""
    _, out, _ = run(["git", "rev-parse", "--abbrev-ref", "HEAD"])
    return out.strip()


def merge_base(base_ref: str, head_ref: str = "HEAD") -> str:
    """Compute merge-base between base_ref and head_ref."""
    code, out, err = run(["git", "merge-base", base_ref, head_ref])
    if code != 0:
        raise SystemExit(f"ERROR: git merge-base failed for {base_ref}..{head_ref}\n{err}")
    return out.strip()


def git_diff_name_status(base: str, head: str = "HEAD") -> str:
    """Return file list with change type (A/M/D/R)."""
    _, out, _ = run(["git", "diff", "--name-status", f"{base}...{head}"])
    return out.strip()


def git_diff_stat(base: str, head: str = "HEAD") -> str:
    """Return diffstat summary."""
    _, out, _ = run(["git", "diff", "--stat", f"{base}...{head}"])
    return out.strip()


def git_diff_unified(base: str, head: str = "HEAD") -> str:
    """Return unified diff text."""
    _, out, _ = run(["git", "diff", f"{base}...{head}"])
    return out


def truncate_lines(text: str, max_lines: int) -> Tuple[str, bool]:
    """Truncate text to max_lines; return (truncated_text, was_truncated)."""
    lines = text.splitlines()
    if len(lines) <= max_lines:
        return text, False
    truncated = "\n".join(lines[:max_lines]) + "\n\n[TRUNCATED OUTPUT]"
    return truncated, True


def run_shell_command(cmd: str) -> CmdResult:
    """
    Run a shell command string.
    Uses `bash -lc` so your shell environment behaves as expected.
    """
    code, out, err = run(["bash", "-lc", cmd])
    return CmdResult(cmd=cmd, exit_code=code, stdout=out, stderr=err)


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Generate a Markdown review bundle from git diffs and optional command outputs."
    )
    parser.add_argument("--base", required=True, help="Base ref to diff against (e.g. origin/main, main, develop).")
    parser.add_argument("--out", default="review_bundle.md", help="Output Markdown file path.")
    parser.add_argument("--max-diff-lines", type=int, default=2000, help="Max diff lines to embed before truncating.")
    parser.add_argument("--cmd", action="append", default=[], help="Command(s) to run and include output (repeatable).")
    args = parser.parse_args()

    require_git_repo()

    branch = detect_branch()
    head_sha = detect_head()
    base_sha = merge_base(args.base)

    name_status = git_diff_name_status(args.base)
    diff_stat = git_diff_stat(args.base)

    diff_text = git_diff_unified(args.base)
    diff_text_trunc, diff_was_trunc = truncate_lines(diff_text, args.max_diff_lines)

    cmd_results: List[CmdResult] = []
    for c in args.cmd:
        cmd_results.append(run_shell_command(c))

    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")

    md: List[str] = []
    md.append("# Review Bundle\n")
    md.append(f"- Generated: `{now}`\n")
    md.append(f"- Branch: `{branch}`\n")
    md.append(f"- Base ref: `{args.base}` (merge-base `{base_sha}`)\n")
    md.append(f"- HEAD: `{head_sha}`\n")

    md.append("\n## Files changed (name-status)\n")
    md.append("```text\n" + (name_status or "(no changes)") + "\n```\n")

    md.append("\n## Diff stat\n")
    md.append("```text\n" + (diff_stat or "(no diff stat)") + "\n```\n")

    md.append("\n## Unified diff (truncated)\n")
    md.append("```diff\n" + (diff_text_trunc or "(empty diff)") + "\n```\n")
    if diff_was_trunc:
        md.append(f"\n> NOTE: Diff was truncated to {args.max_diff_lines} lines. Consider splitting the PR.\n")

    if cmd_results:
        md.append("\n## Command outputs\n")
        for r in cmd_results:
            md.append(f"\n### `{r.cmd}`\n")
            md.append(f"- Exit code: `{r.exit_code}`\n")
            md.append("\n#### stdout\n")
            md.append("```text\n" + (r.stdout.strip() or "(empty)") + "\n```\n")
            md.append("\n#### stderr\n")
            md.append("```text\n" + (r.stderr.strip() or "(empty)") + "\n```\n")

    md.append("\n## What I want from you (copy/paste)\n")
    md.append(
        "Please review the change for:\n"
        "- P0 security/correctness issues\n"
        "- P1 missing tests or likely regressions\n"
        "Then produce a minimal fix checklist and a paste-ready `@codex` comment to implement it.\n"
    )

    with open(args.out, "w", encoding="utf-8") as f:
        f.write("\n".join(md))

    print(f"Wrote {args.out}")


if __name__ == "__main__":
    main()
