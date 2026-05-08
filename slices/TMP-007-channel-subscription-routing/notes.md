# TMP-007 Build Notes

## Build Log

Completed at: 2026-05-08T06:30:28Z

Layers completed:

- Manifest/build state: moved TMP-007 from `specced` to `building` on branch `agent/codex/tenant-platform-roadmap-20260508-014156`.
- Schema/storage: added nullable tenant/channel ownership columns and tenant-scoped indexes for subscriptions, notifications, and admin subscription action logs while preserving legacy rows.
- Routing module: added the canonical tenant provider routing module in subscription-external for active channel lookup, capability gating, credential reference resolution, provider config construction, and env-backed local credential dereferencing.
- Service/API: wired tenant provider routing into partner MT, charge, status, optout, confirm, admin actions, and acquisition opt-in propagation.
- Observability/redaction: redacted provider auth headers in admin action captures and removed API key logging from status checks.
- Tests: added focused tests for tenant context enforcement, env credential resolution, capability gating, redaction, and acquisition signed tenant/channel propagation.

Commits:

- `f181842` slice(TMP-007): manifest - start build
- `6b4d082` slice(TMP-007): db - add tenant channel ownership
- `6439f08` slice(TMP-007): service - route provider calls by tenant channel
- `946c80d` slice(TMP-007): tests - cover tenant routing gates

Validation:

- `cd services/subscription-external && go test ./...`
- `cd services/acquisition-api && go test ./...`
- `git diff --check`

Discovered during build:

- `docs/SLICE-METHODOLOGY.md` and the cross-agent completion docs named by the generic slice skill are not present in this repository package; the active audit contract is under `slices/`.
- TMP-007 keeps tenant/channel columns nullable to preserve legacy unscoped rows. TMP-011 remains responsible for default-tenant migration and hard enforcement.
- TMP-007 ships an `env://` credential resolver for local/test provider config. TMP-015 remains responsible for production secret backend hardening.
