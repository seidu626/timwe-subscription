# Value-Gate Report: TMP-069

Run-id: `slice-TMP-069-build-2026-05-13T2025Z`
Date: 2026-05-13

## Outcome: PASS

## Changes delivered

| File | Change |
|---|---|
| `common/auth/tenantctx/trusted_service.go` | Added `HeaderChannelKey = "X-Channel-Key"` constant |
| `common/auth/tenantctx/resolver.go` | New — implements `ResolveKeyPair` with 4-rule precedence |
| `common/auth/tenantctx/resolver_test.go` | New — 6 tests covering all 4 precedence cases |
| `services/notification/internal/handler/http.go` | Updated `tenantIDForAdminRead` to call canonical resolver; added `fasthttpHeaderGetter` and `firstNonBlank` |
| `services/subscription-external/internal/handler/partner_handler.go` | Updated `tenantRouteFromRequest` to call `ResolveKeyPair` for channel key; `tenantRouteStatus` returns 409 on `ErrTenantKeyConflict` |
| `services/notification/vendor/.../trusted_service.go` | Synced `HeaderChannelKey` constant to notification vendor |
| `docs/tenant-channel-onboarding.md` | Appended "Tenant / Channel Key Resolver Precedence" section |

## Test counts

- `common/auth/tenantctx`: 16 passed (6 new resolver tests + 10 existing)
- `services/notification`: 18 passed
- `services/subscription-external`: 92 passed, 1 pre-existing failure (`TestNetworkResilientClientIntegration` — unrelated nil panic, present on HEAD before this slice)

## Acceptance criteria status

- [x] Header wins when both header and query agree
- [x] Conflict between header and query returns refusal with ErrTenantKeyConflict (HTTP 409)
- [x] Query alone accepted only when header absent AND gateway trusted
- [x] Mixed-case keys normalised before comparison
- [x] Precedence rule documented in docs/tenant-channel-onboarding.md
- [x] go test ./common/auth/tenantctx/... covers all 4 cases
- [x] Notification handler uses canonical resolver
- [x] Partner handler uses canonical resolver; conflict returns HTTP 409
