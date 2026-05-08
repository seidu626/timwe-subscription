## Auth0 Unauthorized debugging + UI message cleanup

**Status**: completed
**Owner**: platform
**Created**: 2026-01-22

### Goal
Resolve production `401 Unauthorized` for admin API calls after Auth0 login, and remove legacy "admin token" messaging from the WebSPA UI.

### Constraints
- Do not log JWTs or sensitive headers.
- Keep error messages user-friendly and actionable.

### Dependencies
- Production droplet has `ADMIN_AUTH0_DOMAIN` and `ADMIN_AUTH0_AUDIENCE` set for `acquisition-api` and `cadence-engine`.
- KrakenD `/v1/admin/*` endpoints are deployed with JWT validation.

### ExitCriteria
- WebSPA does not show "admin token in Settings" anywhere.
- On 401, UI instructs user to re-login with Auth0 (not set token).
- Backend logs provide enough signal to identify whether 401 is due to issuer/audience mismatch or JWKS/key lookup issues (without exposing tokens).

### Notes
- 2026-01-22: Created task after observing prod 401 + legacy UI messaging.
- 2026-01-22: Updated WebSPA error messages to reference Auth0 login (removed "admin token" wording).
- 2026-01-22: Added safe backend logging + richer validation errors (issuer/audience mismatch) without logging tokens.

