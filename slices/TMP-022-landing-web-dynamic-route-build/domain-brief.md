# TMP-022 Domain Brief

## Actors

- End-subscriber: opens campaign landing URLs and completes acquisition flow through landing-web. Source: `services/landing-web/README.md`.
- Platform operator: runs production build before deployment. Source: `Makefile` and `services/landing-web/package.json`.

## Ubiquitous Language

- Campaign slug: single public identifier in legacy landing URLs. Source: `services/landing-web/app/lp/[slug]/page.tsx`.
- Tenant-qualified route: URL form that includes tenant and campaign slug. Source: `services/landing-web/app/lp/[tenant]/[slug]/page.tsx`.
- Campaign API route: local Next.js API proxy for campaign lookup. Source: `services/landing-web/app/api/campaigns/**/route.ts`.

## Domain Invariants

- Public URL shape is contract-sensitive: `/lp/:slug` and `/lp/:tenant/:slug` must remain valid route shapes.
- Legacy single-segment URL uses its only segment as campaign slug.
- Tenant-qualified URL must keep tenant and campaign slug distinct.

## Failure Modes

- Build-time route conflict: Next.js rejects sibling dynamic segment names under the same path level.
- Legacy route parameter drift: renaming folders for Next.js compatibility could accidentally change the variable treated as slug.
- Tenant-qualified route regression: a compatibility rename could collapse the tenant and slug distinction.

## User Journey

1. End-subscriber opens `/lp/summer-campaign`; landing-web treats `summer-campaign` as the campaign slug.
2. End-subscriber opens `/lp/tenant-a/summer-campaign`; landing-web treats `tenant-a` as tenant and `summer-campaign` as slug.
3. Operator runs `npm run build`; route tree compiles for deployment.

## Open Questions

- No automated route-level tests currently cover both URL shapes; `npm run build` is the minimum regression proof for this defect.
