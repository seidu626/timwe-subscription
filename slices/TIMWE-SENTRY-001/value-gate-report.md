# TIMWE-SENTRY-001 — Value Gate Report

Verdict: PASS (manual production validation pending)

## Acceptance Criteria Coverage

- Landing page app includes Sentry runtime config files:
  - `services/landing-web/sentry.client.config.ts`
  - `services/landing-web/sentry.server.config.ts`
  - `services/landing-web/sentry.edge.config.ts`
- `services/landing-web/next.config.ts` wraps config with `withSentryConfig`.
- `services/landing-web/package.json` includes `@sentry/nextjs`.
- `frontend/webspa-admin/package.json` includes `@sentry/angular-ivy`.
- Angular env wiring references:
  - `sentryDsn`, `sentryEnvironment`, `sentryRelease` in `frontend/webspa-admin/src/environments/environment.ts`
  - same keys in `frontend/webspa-admin/src/environments/environment.prod.ts`
- Angular bootstrap has:
  - Sentry runtime init in `frontend/webspa-admin/src/main.ts`
  - `SentryModule.forRoot(...)` in `frontend/webspa-admin/src/app/app.module.ts`
  - `SentryErrorHandler` provider registration in `frontend/webspa-admin/src/app/app.module.ts`
- CI injects release metadata for Next build:
  - `SENTRY_RELEASE: ${{ github.sha }}`

## Failure-Mode Coverage

- Build-time release verification against a live Sentry project remains manual (`throw new Error('sentry-test')`) and requires a test DSN.
- No backend Sentry integration was changed in this slice; only Next + webspa-admin frontend paths were implemented.

## Evidence

```text
rg -n "SENTRY_DSN|SENTRY_ENVIRONMENT|SENTRY_RELEASE" \
  services/landing-web frontend/webspa-admin
rg -n "withSentryConfig" services/landing-web/next.config.ts
rg -n "SentryModule|SentryErrorHandler" frontend/webspa-admin/src/app/app.module.ts
rg -n "Sentry.init" frontend/webspa-admin/src/main.ts
rg -n "SENTRY_RELEASE" .github/workflows/ci.yml
```
