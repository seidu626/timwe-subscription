# TMP-070 — careerify-tenant-e2e-smoke

Skeleton spec. Authored by `/slice-plan` 2026-05-13. To be expanded by `/slice-spec`.

## User story

As a platform operator, I run a smoke matrix that exercises the 10 inbound URLs (6 notification + 4 subscription) for `careerify`+`web-gh-airteltigo` through nginx → KrakenD → backend, confirms tenant scoping end-to-end, and verifies cross-tenant injection attempts are rejected.

## Demo

`scripts/smoke/careerify-tenant-e2e.sh` runs all 10 sample URLs, captures logs proving `tenant_id=careerify` on each, executes 3 cross-tenant injection cases (mismatched header vs query, foreign tenant key, missing channel), and prints a green pass matrix.

## Scope (files in)

- `scripts/smoke/careerify-tenant-e2e.sh` — runs 10 happy-path URLs.
- `scripts/smoke/careerify-tenant-cross-tenant-refusal.sh` — runs adversarial cases.
- `slices/TMP-070-careerify-tenant-e2e-smoke/value-gate-report.md` — final evidence: smoke transcript + DB row proofs + adversarial outcomes.
- `docs/tenant-channel-onboarding.md` — append careerify onboarding evidence section.

## Scope (files out)

- Any production code change. This slice is verification only. If a gap is found, bounce to TMP-066/067/068/069.

## Acceptance

- 10/10 happy-path URLs return 2xx and resolve `tenant_id` to careerify's UUID.
- 3/3 adversarial URLs are refused with appropriate 4xx + structured error.
- `value-gate-report.md` includes commit SHAs of TMP-066..069 it was run against, smoke transcript, and DB query results.
- A clear pass matrix table is checked in.

## Verification

See manifest `verification.automated` and `manual_smoke`.

## Notes

Closing slice for careerify onboarding. Functions as the adversarial gate per the original mythos-agent-orchestrator packet. On any failure, halt and route the gap to the upstream slice owner — do not patch the smoke script to mask the issue.
