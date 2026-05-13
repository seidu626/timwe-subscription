# TMP-069 — tenant-resolver-header-query-precedence

Skeleton spec. Authored by `/slice-plan` 2026-05-13. To be expanded by `/slice-spec`.

## User story

As an integration engineer, the canonical tenant resolver in `common/auth/tenantctx` enforces deterministic precedence between header and query sources of `tenant_key`/`channel_key`, refuses conflicting pairs, and is documented so backends and gateway authors share one rule.

## Demo

Unit tests in `common/auth/tenantctx` and the two consuming services assert:

1. Header wins when both header and query agree.
2. Conflict between header and query returns refusal (no silent override).
3. Query alone is accepted only when header absent **and** request originated via the gateway trust boundary (verified by network policy or shared header secret).
4. Mixed-case keys are normalized to lowercase before comparison.

## Scope (files in)

- `common/auth/tenantctx/identity.go` and sibling files.
- `common/auth/tenantctx/*_test.go` — new tests covering the 4 precedence cases.
- `services/notification/internal/handler/http.go` if the resolver call site needs to be updated.
- `services/subscription-external/internal/handler/partner_handler.go` if same.
- `docs/tenant-channel-onboarding.md` or a sibling doc — record the rule.
- `slices/TMP-069-tenant-resolver-header-query-precedence/value-gate-report.md`.

## Scope (files out)

- `krakend/` — gateway is the source of headers, not consumer of this rule.
- `ops/nginx/` — out of scope.

## Acceptance

- Precedence rule documented in a markdown file under `docs/`.
- `go test ./common/auth/tenantctx/...` covers the 4 cases above.
- Notification and subscription-external handlers use the canonical resolver (no inline tenant parsing).
- Conflict case returns HTTP 4xx with a structured error code, not silent acceptance.

## Verification

See manifest `verification.automated`.

## Notes

Defence-in-depth slice. Without it, TMP-067/068 still function as long as KrakenD config is correct; with it, a gateway misconfig produces a loud failure instead of cross-tenant routing.
