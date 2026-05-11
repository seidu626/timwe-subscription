# TIMWE-SENTRY-001 — Add Sentry error monitoring to landing-web and webspa-admin

**Class**: bounded_enabler
**State**: planned
**Layers**: observability, frontend
**Depends on**: TIMWE-CI-001

---

## ACCEPTANCE CRITERIA

- **services/landing-web**: `@sentry/nextjs` installed; `sentry.client.config.ts`, `sentry.server.config.ts`, `sentry.edge.config.ts` present; `next.config.ts` wrapped with `withSentryConfig`.
- **frontend/webspa-admin**: `@sentry/angular-ivy` installed; `SentryModule.forRoot()` in `app.module.ts`; `SentryErrorHandler` registered.
- Both apps read `SENTRY_DSN`, `SENTRY_ENVIRONMENT`, `SENTRY_RELEASE` from env at runtime.
- Manually thrown test error in each app (dev mode, test DSN) routes to the Sentry dashboard.
- CI lane from TIMWE-CI-001 injects `SENTRY_RELEASE` as `${{ github.sha }}` during build.

---

## FILES TO TOUCH

| File | Action |
|------|--------|
| `services/landing-web/sentry.client.config.ts` | new |
| `services/landing-web/sentry.server.config.ts` | new |
| `services/landing-web/sentry.edge.config.ts` | new |
| `services/landing-web/next.config.ts` | wrap with `withSentryConfig` |
| `services/landing-web/package.json` | add `@sentry/nextjs` |
| `frontend/webspa-admin/src/app.module.ts` | add `SentryModule.forRoot()`, register `SentryErrorHandler` |
| `frontend/webspa-admin/src/main.ts` | Sentry init call |
| `frontend/webspa-admin/package.json` | add `@sentry/angular-ivy` |
| `.github/workflows/ci.yml` | amend — inject `SENTRY_RELEASE: ${{ github.sha }}` env in build steps |

---

## OUT OF SCOPE

- Go service Sentry integration (follow-up slice).
- Source-map upload automation (follow-up slice).
- Production DSN rotation / secrets management beyond `.env.local`.

---

## DEMO

1. Set `SENTRY_DSN=<test DSN>` in `.env.local` (landing-web) and `environment.ts` (webspa-admin).
2. Trigger `throw new Error('sentry-test')` in each app (dev mode).
3. Confirm event appears in Sentry dashboard with correct `environment` and `release` tag.

---

## DEPENDS ON

**TIMWE-CI-001** — the CI lane is the host for `SENTRY_RELEASE` injection via `${{ github.sha }}`.
