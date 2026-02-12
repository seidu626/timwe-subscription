#!/usr/bin/env python3

"""
parse_codex_focus.py

Parse a PR comment body and emit a sanitized focus list for Codex reviews.

Security model:
- Treat the entire comment as untrusted input.
- Only accept focus values from a strict whitelist.
- Output is suitable for GitHub Actions step outputs (key=value lines).

Usage:
  python tools/parse_codex_focus.py "<comment body>"

In GitHub Actions:
  python tools/parse_codex_focus.py "${{ github.event.comment.body }}" >> "$GITHUB_OUTPUT"
"""

from __future__ import annotations

import re
import sys
from typing import List

ALLOWED = {"security", "correctness", "perf", "tests"}
DEFAULT_FOCUS = ["security", "correctness"]


def extract_focus_tokens(body: str) -> List[str]:
    """
    Extract focus tokens from a string like:
      "/codex review focus=security,perf"
    Returns a sanitized list of tokens from ALLOWED (deduped, max 3).
    """
    if not body.strip().lower().startswith("/codex"):
        return DEFAULT_FOCUS

    m = re.search(r"\bfocus=([a-zA-Z0-9,_-]+)\b", body)
    if not m:
        return DEFAULT_FOCUS

    raw = m.group(1).lower()
    tokens = [t.strip() for t in raw.split(",") if t.strip()]

    out: List[str] = []
    for t in tokens:
        if t in ALLOWED and t not in out:
            out.append(t)

    return out[:3] if out else DEFAULT_FOCUS


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("Usage: parse_codex_focus.py '<comment body>'")

    focus = extract_focus_tokens(sys.argv[1])
    print(f"focus_csv={','.join(focus)}")
    print(f"focus_md={', '.join(focus)}")


if __name__ == "__main__":
    main()
