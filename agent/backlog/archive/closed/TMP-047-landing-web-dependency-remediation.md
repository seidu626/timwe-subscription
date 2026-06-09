---
id: TMP-047
title: "Landing web dependency remediation"
class: vertical_defect_slice
status: in_progress
scope_limit: "Upgrade services/landing-web dependencies only as required to remediate the Next/PostCSS audit blocker after TMP-037 approval. Preserve landing page and HE simulation behavior."
merge_policy: "Merge only after npm audit, build, bounded runtime smoke, HVC, supervisor preflight, and value-gate evidence pass."
evidence_required:
  - "cd services/landing-web && npm audit --audit-level=moderate"
  - "cd services/landing-web && npm run build"
  - "bounded landing-web runtime smoke returns HTTP 200 for /"
  - "slices/TMP-047-landing-web-dependency-remediation/value-gate-report.md"
acceptance_tests:
  - "cd services/landing-web && npm audit --audit-level=moderate"
  - "cd services/landing-web && npm run build"
  - "bounded landing-web runtime smoke returns HTTP 200 for /"
  - "jq empty slices/manifest.json agent/state/TMP-047.work-order.json agent/state/TMP-047.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
actor: platform-operator
outcome: "Landing-web no longer ships with the Next/PostCSS vulnerability blocker and still builds and serves its landing entrypoint."
entrypoint: "services/landing-web/package.json"
trigger: "Verifier reruns npm audit after TMP-037 approval."
broken_outcome: "npm audit reports Next/PostCSS advisories and npm audit fix proposes a breaking upgrade."
expected_behavior: "The landing-web dependency graph audits cleanly at moderate severity or higher, the Next app builds, and the landing page runtime responds."
reproduction:
  command: "cd services/landing-web && npm audit --audit-level=moderate"
  observed: "npm audit reports high-severity Next.js advisories and a moderate PostCSS advisory; npm audit fix proposes a breaking Next 16 upgrade."
  expected: "npm audit exits 0 with no moderate-or-higher advisories after the remediation."
system_path:
  - "Platform operator approves the dependency remediation gate in TMP-037."
  - "Agent updates landing-web dependency metadata and lockfile."
  - "Verifier runs audit, build, and runtime smoke."
  - "Release matrix can retire the dependency blocker."
change_layers:
  - dependency
  - frontend-runtime
  - evidence
verification_layers:
  - dependency-audit
  - build
  - runtime-smoke
blocked_by:
  - "TMP-037"
blocks:
  - "TMP-021"
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - "services/landing-web/package.json"
    - "services/landing-web/package-lock.json"
    - "services/landing-web/next.config.js"
    - "services/landing-web/middleware.ts"
    - "services/landing-web/next-env.d.ts"
    - "services/landing-web/tsconfig.json"
    - "services/landing-web/app/api/campaigns/[tenant]/route.ts"
    - "services/landing-web/app/api/campaigns/[tenant]/[slug]/route.ts"
    - "services/landing-web/app/api/transactions/[id]/confirm/route.ts"
    - "services/landing-web/app/c/[slug]/page.tsx"
    - "services/landing-web/app/lp/[tenant]/page.tsx"
    - "services/landing-web/app/lp/[tenant]/[slug]/page.tsx"
    - "agent/backlog/issues/TMP-047-landing-web-dependency-remediation.md"
    - "agent/state/TMP-047.work-order.json"
    - "agent/state/TMP-047.handoff.json"
    - "slices/manifest.json"
    - "slices/TMP-047-landing-web-dependency-remediation/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/landing-web/app/components/**"
    - "services/landing-web/app/lib/**"
    - "services/landing-web/lib/**"
    - "services/**/migrations/**"
    - "common/**"
    - "frontend/**"
    - "ops/**"
    - "docker-compose*.yml"
    - "Makefile"
    - "go.mod"
    - "go.sum"
---

## Operator Story

As a platform-operator, I can run landing-web dependency verification without Next/PostCSS advisories blocking release, while still proving the landing page builds and responds at runtime.

## Acceptance Criteria

- `npm audit --audit-level=moderate` passes in `services/landing-web`.
- `npm run build` passes in `services/landing-web`.
- A bounded runtime smoke starts the built landing-web app and receives HTTP 200 from `/`.
- The remediation stays scoped to landing-web package/config files and slice evidence.
- Next 16 route/page params compatibility is updated only in the dynamic landing-web routes needed for the build.
