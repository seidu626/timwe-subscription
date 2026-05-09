---
id: TMP-030
title: "Acquisition API compose build context"
class: vertical_defect_slice
status: ready
scope_limit: "Fix acquisition-api Docker Compose image build so the Dockerfile can resolve the repo-local common module without dependency, vendor, package, or service source changes."
merge_policy: "Merge only after acquisition-api Docker image build, HVC, JSON validation, slice-harness, supervisor preflight, and source-scope checks pass."
evidence_required:
  - "DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker compose --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml build acquisition-api"
  - "slices/TMP-030-acquisition-compose-build-context/value-gate-report.md"
acceptance_tests:
  - "DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker compose --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml build acquisition-api"
  - "jq empty slices/manifest.json agent/state/TMP-030.work-order.json agent/state/TMP-030.handoff.json .agent/tasks.json"
  - "hvc check agent/backlog/issues/*.md --fail-on block"
  - "slice-harness status && slice-harness sync --dry-run"
actor: verification-agent
outcome: "Acquisition API image builds in the local compose runtime path without requiring a stale service-local vendor tree."
entrypoint: "docker-compose.yml acquisition-api build"
trigger: "Verifier runs bounded compose runtime smoke."
broken_outcome: "Compose image build fails before acquisition-api startup because the Dockerfile forces -mod=vendor while the service has no vendor directory and cannot see ../../common from a service-only build context."
expected_behavior: "The acquisition-api Dockerfile builds from a repo-root context, copies the service plus common module into matching paths, and uses readonly module resolution without mutating dependencies."
reproduction:
  command: "DOCKER_CONFIG=<tmp> REGISTRY_AUTH_FILE=<tmp> docker compose --env-file .env.example -f docker-compose.yml -f /tmp/timwe-compose-auth-probe-override.yml up -d --build ... acquisition-api ..."
  observed: "Build fails in services/acquisition-api Dockerfile at `go build -mod=vendor` with inconsistent vendoring because the service has no vendor directory."
  expected: "acquisition-api image build completes and can proceed to container startup."
system_path:
  - "Verification agent runs compose build for acquisition-api."
  - "Docker build context includes services/acquisition-api and common."
  - "go build resolves the local common replacement at ../../common."
  - "Image build completes without vendor or dependency metadata changes."
change_layers:
  - compose-build-config
verification_layers:
  - docker-build
  - harness
blocked_by: []
blocks: ["TMP-021"]
parallel_group: release-verification-blockers
file_scope:
  allowed:
    - "docker-compose.yml"
    - "services/acquisition-api/Dockerfile"
    - "docs/agent/full-system-verification-2026-05-09.md"
    - "slices/manifest.json"
    - "slices/TMP-021-full-system-verification/value-gate-report.md"
    - "slices/TMP-030-acquisition-compose-build-context/**"
    - "agent/backlog/issues/TMP-030-acquisition-compose-build-context.md"
    - "agent/state/TMP-030.work-order.json"
    - "agent/state/TMP-030.handoff.json"
    - "agent/state/TMP-021.handoff.json"
    - ".agent/**"
    - ".harness/**"
  forbidden:
    - "common/**"
    - "services/acquisition-api/go.mod"
    - "services/acquisition-api/go.sum"
    - "services/acquisition-api/vendor/**"
    - "services/acquisition-api/internal/**"
    - "services/acquisition-api/cmd/**"
    - "services/*/go.mod"
    - "services/*/go.sum"
    - "services/*/vendor/**"
    - "frontend/**"
    - "package.json"
    - "package-lock.json"
    - "vendor/**"
---

## Operator Story

As a verification agent, I can build the acquisition-api compose image from the repo-local module graph, so the full compose smoke can progress beyond a stale vendor/build-context failure.

## Acceptance Criteria

- `docker-compose.yml` points the acquisition-api build at a context that includes both `services/acquisition-api` and `common`.
- `services/acquisition-api/Dockerfile` copies only required build inputs and builds with readonly module resolution.
- Acquisition API image build succeeds with temporary isolated Docker auth.
- No Go source, dependency metadata, vendor, frontend, package manifest, or lockfile files are changed.
