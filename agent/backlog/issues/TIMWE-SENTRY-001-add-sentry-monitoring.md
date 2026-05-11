---
id: TIMWE-SENTRY-001
title: "Add Sentry error monitoring to landing-web and webspa-admin"
class: bounded_enabler
status: queued
parent_vertical_slice_id: TMP-007
consumed_by:
  - TMP-007
scope_limit: "Add Sentry wiring for landing-web and webspa-admin; do not change backend runtime behavior."
merge_policy: "Merge only after grep-based wiring checks, targeted smoke validation, and harness/slice verification."
evidence_required:
  - "grep -r SENTRY_DSN services/landing-web/ frontend/webspa-admin/"
  - "grep withSentryConfig services/landing-web/next.config.ts"
  - "grep SentryModule frontend/webspa-admin/src/app.module.ts"
acceptance_tests:
  - "SENTRY_DSN is present in frontend and Next configuration in a non-secret way."
  - "SENTRY_RELEASE uses `${{ github.sha }}` from the CI workflow."
  - "A manual sentry-test throw surfaces as an event in the configured Sentry project."
actor: platform-observability
outcome: "Runtime errors from landing-web and webspa-admin are surfaced in Sentry with release and environment context."
entrypoint: "services/landing-web, frontend/webspa-admin"
trigger: "Runtime errors in landing-web or webspa-admin are difficult to observe in production."
broken_outcome: "UI/service errors remain in console logs without centralized alerting and release context."
expected_behavior: "Errors in landing-web and webspa-admin publish Sentry events with release and environment tags."
reproduction:
  command: "Configure test DSN, trigger manual sentry-test error, and verify event appears in Sentry."
  observed: "No event appears or release tag is missing."
  expected: "Sentry receives event with expected environment and release metadata."
system_path:
  - "services/landing-web"
  - "frontend/webspa-admin"
verification_layers:
  - frontend
  - automation
  - harness
blocked_by:
  - "TIMWE-CI-001"
blocks: []
parallel_group: observability-web
non_goals:
  - "Do not add backend Sentry integration in this slice."
  - "Do not add production secret rotation."
file_scope:
  allowed:
    - "agent/backlog/issues/TIMWE-SENTRY-001-add-sentry-monitoring.md"
    - "agent/state/TIMWE-SENTRY-001.work-order.json"
    - "agent/state/TIMWE-SENTRY-001.handoff.json"
    - "services/landing-web/**"
    - "frontend/webspa-admin/**"
    - "slices/TIMWE-SENTRY-001/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "ops/**"
---

## Operator Story

As a platform operator, I can see runtime errors from landing-web and webspa-admin surfaced in Sentry with environment and release tags.
