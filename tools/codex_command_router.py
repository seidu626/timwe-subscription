#!/usr/bin/env python3

"""
codex_command_router.py

Parse maintainer PR comments for a small, safe command language.
Outputs GitHub Actions outputs (key=value lines) for routing.

Security posture:
- Treat comment body as untrusted input.
- Allow only a strict command set and whitelisted focus tags.
- Do NOT pass arbitrary trailing text into prompts.

Supported:
  /codex review [focus=security,perf]
  /codex gate  [focus=security,correctness]
  /codex fix-ci

Outputs:
  action = "review" | "gate" | "fix-ci" | "none"
  focus_csv = "security,correctness"
"""

from __future__ import annotations

import re
import sys
from typing import List, Tuple

ALLOWED_ACTIONS = {"review", "gate", "fix-ci"}
ALLOWED_FOCUS = {"security", "correctness", "perf", "tests"}
DEFAULT_FOCUS = ["security", "correctness"]


def parse_command(body: str) -> Tuple[str, List[str]]:
    """
    Parse the command from a comment body.
    Returns (action, focus_list).
    """
    text = body.strip()

    # Strict prefix; ignore anything that doesn't start with /codex
    if not text.lower().startswith("/codex"):
        return "none", DEFAULT_FOCUS

    m = re.match(r"^/codex\s+([a-zA-Z0-9_-]+)\b", text)
    if not m:
        return "none", DEFAULT_FOCUS

    action = m.group(1).lower()
    if action not in ALLOWED_ACTIONS:
        return "none", DEFAULT_FOCUS

    focus = DEFAULT_FOCUS
    if action in {"review", "gate"}:
        mf = re.search(r"\bfocus=([a-zA-Z0-9,_-]+)\b", text)
        if mf:
            raw = mf.group(1).lower()
            tokens = [t.strip() for t in raw.split(",") if t.strip()]
            out: List[str] = []
            for t in tokens:
                if t in ALLOWED_FOCUS and t not in out:
                    out.append(t)
            focus = out[:3] if out else DEFAULT_FOCUS

    return action, focus


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("Usage: codex_command_router.py '<comment body>'")

    action, focus = parse_command(sys.argv[1])
    print(f"action={action}")
    print(f"focus_csv={','.join(focus)}")


if __name__ == "__main__":
    main()
