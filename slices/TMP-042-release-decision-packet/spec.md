# TMP-042 Spec

## Story

As a platform operator, I need the release decision packet refreshed against the latest local full-system verification evidence, so I do not treat old blockers as current or local proof as production release approval.

## Acceptance

- A decision packet exists at `docs/agent/release-decision-packet-2026-05-09.md`.
- The packet states that the previous TMP-021/TMP-026/TMP-034/TMP-035/TMP-036/TMP-037/TMP-038 blocker set is superseded by newer local verification evidence where applicable.
- The packet identifies remaining release-owner decisions that are outside TMP-042, especially production deploy, live provider credentials/flows, release-candidate rerun, and primary checkout integration if required.
- The packet explicitly says it does not approve a production release, deployment, branch merge/reset, schema change, dependency change, compose change, submodule/gitlink change, or source change.
- The change set is limited to docs, slice, state, issue, and run-capsule files.

## Non-Goals

- Do not implement schema provisioning.
- Do not change SQL, compose, service code, package manifests, lockfiles, frontend code, submodules, or branch history.
- Do not record a production release or deploy approval on behalf of the operator.
