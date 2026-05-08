#!/usr/bin/env bash
# ci/codex_gate.sh
#
# Purpose:
# - Run a structured Codex review against the current git diff
# - Fail CI if P0 findings exist
#
# Requirements:
# - jq installed
# - Codex CLI installed (`npm i -g @openai/codex`)
#
# Notes:
# - Codex CLI examples commonly use CODEX_API_KEY; if you only have OPENAI_API_KEY,
#   this script will copy it into CODEX_API_KEY for convenience.

set -euo pipefail

BASE_REF="${1:-origin/main}"
SCHEMA_PATH=".github/codex/schemas/review_gate.schema.json"
PROMPT_FILE=".github/codex/prompts/review_gate.md"
OUT_JSON="codex_review.json"

# Make CODEX_API_KEY available if only OPENAI_API_KEY is set.
if [[ -z "${CODEX_API_KEY:-}" && -n "${OPENAI_API_KEY:-}" ]]; then
  export CODEX_API_KEY="${OPENAI_API_KEY}"
fi

if [[ -z "${CODEX_API_KEY:-}" ]]; then
  echo "ERROR: CODEX_API_KEY (or OPENAI_API_KEY) is not set." >&2
  exit 2
fi

# Generate the diff to review
git fetch -q origin || true
git diff --no-color "${BASE_REF}...HEAD" > pr.diff || true

# Run Codex in a constrained way:
# - read-only sandbox is safest for review
# - output is forced to match the schema
codex exec \
  --sandbox read-only \
  --ask-for-approval never \
  --output-schema "${SCHEMA_PATH}" \
  -o "${OUT_JSON}" \
  "Review the diff in pr.diff using the guidance in ${PROMPT_FILE}. Output JSON only."

# Gate: fail if P0 exists or decision is block
DECISION="$(jq -r '.decision' "${OUT_JSON}")"
P0="$(jq -r '.p0_count' "${OUT_JSON}")"

echo "Codex decision=${DECISION}, p0=${P0}"

if [[ "${DECISION}" == "block" || "${P0}" -gt 0 ]]; then
  echo "Blocking: P0 issues detected."
  exit 1
fi

echo "Gate passed."
