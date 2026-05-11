---
id: TMP-058
title: "Bootstrap admin email without email_verified claim"
class: vertical_defect_slice
status: queued
parent_vertical_slice_id: TMP-010
scope_limit: "Allow configured bootstrap admin emails to receive platform tenant workspaces when the token omits email_verified, while still blocking an explicit unverified email claim."
merge_policy: "Merge only after HVC, focused Auth0/admin transport tests, focused admin SPA tenant workspace tests, and live acquisition-api restart verification pass."
evidence_required:
  - "Auth0/admin transport tests show missing email_verified bootstraps and explicit false does not."
  - "Admin SPA tenant workspace tests show missing email_verified bootstraps and explicit false does not."
  - "hvc check agent/backlog/issues/TMP-058-bootstrap-email-without-email-verified.md --fail-on block"
acceptance_tests:
  - "A configured bootstrap email with no email_verified claim receives platform scope and selected tenant context."
  - "A configured bootstrap email with email_verified=false remains unscoped."
  - "The admin SPA maps configured bootstrap emails without email_verified to configured tenant workspaces."
actor: platform-bootstrap-admin
outcome: "bootstrap admins can access configured tenant workspaces when Auth0 omits email_verified from the token."
entrypoint: "GET /v1/admin/* and admin SPA tenant workspace resolution"
trigger: "Admin loads the local SPA with an Auth0 identity containing email but no email_verified claim."
broken_outcome: "The bootstrap email allowlist is ignored because the backend and SPA treat an absent email_verified claim as false."
expected_behavior: "Absent email_verified is treated as unknown and bootstrap email matching may proceed; explicit false remains blocked."
reproduction:
  command: "Open http://localhost:4200 with an Auth0 token that contains email but omits email_verified."
  observed: "Configured bootstrap email does not get platform tenant context and admin APIs can return Forbidden."
  expected: "Configured bootstrap email gets platform tenant context unless email_verified is explicitly false."
system_path:
  - "Auth0 token is decoded into tenant identity."
  - "acquisition-api applies bootstrap platform email authorization."
  - "webspa-admin resolves tenant workspace options from Auth0 user claims."
change_layers:
  - backend-authz
  - frontend-tenant-workspace
  - tests
verification_layers:
  - auth-claim-tests
  - transport-tests
  - frontend-service-tests
  - harness
  - runtime-smoke
blocked_by: []
blocks:
  - "TMP-021"
parallel_group: acquisition-auth-bootstrap
non_goals:
  - "Do not grant bootstrap scope for emails outside the configured allowlist."
  - "Do not ignore an explicit email_verified=false claim."
  - "Do not change Auth0 tenant mappings or add dependencies."
file_scope:
  allowed:
    - "agent/backlog/issues/TMP-058-bootstrap-email-without-email-verified.md"
    - "agent/state/TMP-058.work-order.json"
    - "agent/state/TMP-058.handoff.json"
    - "common/auth/auth0jwt/claims.go"
    - "common/auth/auth0jwt/claims_test.go"
    - "common/auth/tenantctx/identity.go"
    - "services/acquisition-api/internal/transport/admin.go"
    - "services/acquisition-api/internal/transport/admin_test.go"
    - "frontend/webspa-admin/src/app/core/services/tenant-workspace.service.ts"
    - "frontend/webspa-admin/src/app/core/services/tenant-workspace.service.spec.ts"
    - "slices/manifest.json"
    - "slices/TMP-058-bootstrap-email-without-email-verified/**"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "services/**/go.mod"
    - "services/**/go.sum"
    - "frontend/**/package.json"
    - "frontend/**/package-lock.json"
    - "docker-compose*.yml"
---

## Operator Story

As a configured bootstrap admin, I can use my email allowlist entry even when Auth0 does not emit an `email_verified` claim.

## Acceptance Criteria

- Missing `email_verified` does not block a configured bootstrap admin email.
- Explicit `email_verified=false` still blocks bootstrap platform scope.
- Backend and admin SPA tenant workspace bootstrap behavior match.
