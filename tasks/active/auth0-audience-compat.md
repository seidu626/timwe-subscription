## Auth0 audience compatibility (prod unauthorized)

**Status**: in_progress
**Owner**: platform
**Created**: 2026-01-22

### Goal
Stop production `401 Unauthorized` caused by Auth0 `aud` (audience) mismatches between tokens issued to the SPA and what backend services validate.

### Constraints
- Must not weaken signature/issuer/expiry validation.
- Must not log JWTs or sensitive headers.
- Keep backward compatible behavior for a single configured audience.

### Dependencies
- Auth0 access tokens contain `aud` claim (string or array).
- Services use `ADMIN_AUTH0_DOMAIN` and `ADMIN_AUTH0_AUDIENCE`.

### ExitCriteria
- Validator supports **comma-separated audiences** in `ADMIN_AUTH0_AUDIENCE` (e.g. `aud1,aud2`) and accepts any match.
- Production can be configured to accept both legacy audience (`https://dev-chliep5q.auth0.com/api/v2/`) and desired API identifier (e.g. `https://api.nouveauricheglobalgroup.com`) during migration.
- `acquisition-api` and `cadence-engine` still pass tests.

### Notes
- 2026-01-22: Created task after confirming direct `curl` to `localhost:8084` works but WebAdmin calls return 401.
- 2026-01-22: Updated shared validator to accept comma-separated audiences in `ADMIN_AUTH0_AUDIENCE` and match any `aud` in token.
- 2026-01-22: Documented comma-separated audience support in `docs/environment-variables.md`.

