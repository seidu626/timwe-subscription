# TMP-047 Domain Brief

## Actors

- Platform operator: runs release verification and needs landing-web dependency security gates to pass before release. Source: `agent/backlog/issues/TMP-047-landing-web-dependency-remediation.md`.
- End subscriber: reaches the public landing entrypoint and HE simulation/landing APIs through the Next app. Source: `services/landing-web/app/page.tsx`, `services/landing-web/middleware.ts`.

## Ubiquitous Language

- Landing web: the Next.js app under `services/landing-web` that serves the public landing entrypoint. Source: `services/landing-web/package.json`.
- HE identity: validated headers/cookies resolved in middleware and passed to API routes. Source: `services/landing-web/middleware.ts`, `services/landing-web/lib/he-types.ts`.
- Dependency audit: `npm audit --audit-level=moderate` release gate for Next/PostCSS advisories. Source: `agent/backlog/issues/TMP-037-landing-web-dependency-remediation-approval.md`.
- Runtime smoke: bounded local start of the built Next app with an HTTP check against `/`. Source: `agent/backlog/issues/TMP-047-landing-web-dependency-remediation.md`.

## Domain Invariants

- Release verification must not claim readiness while moderate or high landing-web dependency advisories remain. Source: `slices/TMP-021-full-system-verification/value-gate-report.md`.
- Landing-web remediation must preserve a buildable Next app. Source: `services/landing-web/package.json` `build` script.
- Landing-web remediation must preserve a reachable public root route. Source: `services/landing-web/app/page.tsx`.
- HE identity middleware must remain present while upgrading Next, because API routes depend on propagated HE headers. Source: `services/landing-web/middleware.ts`.

## Failure Modes

- Audit still reports Next/PostCSS advisories: release remains blocked and the value gate fails.
- Build fails after dependency upgrade: package remediation is not shippable and must be reverted or adjusted.
- Runtime start fails or `/` does not return HTTP 200: dependency upgrade regressed the public landing entrypoint.
- Next 16 middleware compatibility changes affect request handling: preserve middleware behavior unless the verifier proves a required migration path.

## User Journey

1. Platform operator approves dependency remediation through TMP-037.
2. Agent upgrades landing-web dependency metadata and lockfile.
3. Platform operator or verifier runs `npm audit --audit-level=moderate`.
4. Verifier runs `npm run build`.
5. Verifier starts the built app and checks `/` returns HTTP 200.

## Failure Journeys

1. Audit still reports advisories -> TMP-047 value gate fails and TMP-021 stays blocked.
2. Build fails -> implementation is incomplete and no release readiness claim is made.
3. Runtime smoke fails -> remediation is treated as a behavioral regression.

## Open Questions

- No app-level test suite exists in `services/landing-web`; this slice uses audit, build, and runtime smoke as the bounded proof.
