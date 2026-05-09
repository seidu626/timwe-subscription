---
id: TMP-022
title: "Landing web dynamic route build failure"
class: vertical_defect_slice
status: done
scope_limit: "Fix the Next.js dynamic segment naming conflict that prevents the landing-web production build. Preserve existing public URL shapes and do not change acquisition API behavior."
merge_policy: "Merge only after HVC, landing-web build, and value-gate evidence pass."
evidence_required:
  - "npm run build"
  - "slices/TMP-022-landing-web-dynamic-route-build/value-gate-report.md"
acceptance_tests:
  - "cd services/landing-web && npm run build"
  - "test -f slices/TMP-022-landing-web-dynamic-route-build/value-gate-report.md"
non_goals:
  - "No Next.js dependency upgrade."
  - "No visual redesign."
  - "No acquisition API contract change."
actor: end-subscriber
outcome: "Landing pages can be built for production while preserving legacy single-slug and tenant-qualified campaign URLs."
entrypoint: "services/landing-web/app/lp routes and services/landing-web/app/api/campaigns routes"
trigger: "Operator runs `npm run build` for landing-web"
broken_outcome: "`npm run build` fails before compiling the landing app because Next.js rejects sibling dynamic folders with different names at the same route level."
expected_behavior: "`npm run build` succeeds while preserving `/lp/:slug`, `/lp/:tenant/:slug`, `/api/campaigns/:slug`, and `/api/campaigns/:tenant/:slug` URL shapes."
reproduction: "In services/landing-web after `npm ci`, run `npm run build`; Next.js reports `You cannot use different slug names for the same dynamic path ('slug' !== 'tenant')`."
system_path:
  - "Next.js route sorting validates dynamic segment names."
  - "Legacy single-segment landing and campaign API routes keep their URL shape."
  - "Tenant-qualified landing and campaign API routes remain available."
change_layers:
  - frontend
verification_layers:
  - build
blocked_by: []
blocks: []
parallel_group: tenant-platform-defects
file_scope:
  allowed:
    - "services/landing-web/app/api/campaigns/**"
    - "services/landing-web/app/lp/**"
    - "slices/TMP-022-landing-web-dynamic-route-build/**"
    - "slices/manifest.json"
    - "agent/backlog/issues/TMP-022-landing-web-dynamic-route-build.md"
    - "agent/state/TMP-022.work-order.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/landing-web/package.json"
    - "services/landing-web/package-lock.json"
    - "common/**"
    - "services/**/go.mod"
    - "services/**/go.sum"
---

## Operator story

As an end-subscriber, I can reach campaign landing pages after a production build, whether the URL is legacy single-slug or tenant-qualified.

## Acceptance criteria

- `npm run build` in `services/landing-web` no longer fails with the dynamic segment name conflict.
- Public URL shapes `/lp/:slug`, `/lp/:tenant/:slug`, `/api/campaigns/:slug`, and `/api/campaigns/:tenant/:slug` remain represented by routes.
- Legacy single-segment route code continues to treat the one segment as the campaign slug.
- Tenant-qualified route code continues to pass tenant and slug separately.
