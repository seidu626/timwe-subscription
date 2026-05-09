# Release Decision Packet

Generated: 2026-05-09T06:25:00Z

Purpose: consolidate the exact operator decisions required before the blocked full-system verification slices can move from evidence-only tracking to implementation.

This packet does not approve any change. It is an evidence artifact for the maintainer/operator to record approvals elsewhere, such as an ADR under `slices/decisions/`, a maintainer comment, or an updated issue decision field.

## Current Gate

| Area | Blocked slices | Current blocker | Minimum approval artifact |
|---|---|---|---|
| Release matrix | TMP-021 | Release verification cannot pass while child blockers remain. | Approval artifacts for the child blockers, then rerun the full-system matrix. |
| Admin frontend source reproducibility | TMP-026 | `frontend/webspa-admin` pins `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`, but the configured CoreUI remote does not provide that commit. | Decision to publish the pinned commit, repoint the gitlink, or replace the gitlink strategy. |
| Acquisition runtime schema | TMP-034 | Acquisition API exits during admin schema bootstrap because `products`/`userbase` base schema is missing before `add_admin_management_tables.sql`. | Decision naming the canonical products/userbase provisioning path and migration order. |
| Notification outbox schema | TMP-035 | Notification worker starts, then dispatcher logs missing `message_outbox`. | Decision naming how compose/runtime applies `services/subscription-external/migrations/011_message_cadence_engine.sql` before worker polling. |
| Postback outbox schema | TMP-036 | Postback dispatcher starts, then polling logs missing `postback_outbox`; two SQL sources define the table. | Decision naming canonical `postback_outbox` owner and migration order. |
| Landing dependency remediation | TMP-037 | `npm audit` remediation requires a breaking Next/PostCSS upgrade path. | Explicit approval for dependency upgrade scope and required UI regression proof. |
| Local main integration | TMP-038 | Primary `main` is clean but diverged from `origin/main`; prior isolated merge probe found broad add/add conflicts. | Maintainer strategy: preserve local history, reset/reseed, or manually integrate. |

## Decision Options

### TMP-026: webspa-admin source reproducibility

Allowed choices:
- Publish or move commit `2ad95b18ecff4d8b23e5d1b7152975c477d5137a` to an accessible remote and keep the gitlink.
- Repoint `frontend/webspa-admin` to a commit available from the configured remote after reviewing feature loss or replacement risk.
- Replace the gitlink strategy with tracked source or a different repository strategy.

Required proof after approval:
- `git submodule update --init --recursive frontend/webspa-admin`
- `git submodule status --recursive frontend/webspa-admin`
- Admin build/test command from the initialized checkout.

### TMP-034/TMP-035/TMP-036: compose runtime schema provisioning

Allowed choices:
- Add a reviewed compose/runtime migration runner that applies the existing SQL sources in explicit order.
- Add a documented operator runbook and verification command for provisioning the local compose database before service startup.
- Create a canonical migration source for duplicated schema only after an owner is chosen.

Required proof after approval:
- Targeted acquisition-api compose smoke reaches `/health`.
- Targeted notification-worker smoke runs without `message_outbox` missing-relation logs.
- Targeted postback-dispatcher smoke runs without `postback_outbox` missing-relation logs.
- Full compose smoke with real local env/provider values is rerun.

### TMP-037: landing-web dependency remediation

Allowed choices:
- Approve `npm audit fix --force` or equivalent breaking Next/PostCSS upgrade and require UI regression proof.
- Defer remediation with a documented risk acceptance and revisit date.
- Choose a narrower patched dependency path if available and verified.

Required proof after approval:
- `npm audit --audit-level=moderate`
- `npm run build`
- Browser/runtime smoke for landing pages and campaign API routes.

### TMP-038: local main integration strategy

Allowed choices:
- Preserve local-only history and manually integrate conflicts.
- Treat `origin/main` as source of truth and archive/reset local `main` only with explicit maintainer approval.
- Split local-only commits into reviewed branches before reconciling `main`.

Required proof after approval:
- Primary checkout status is clean.
- `main` and `origin/main` have an agreed relationship.
- No local-only work is silently discarded.

## Non-Decisions

- This packet does not approve schema, migration, compose, dependency, submodule, or branch changes.
- This packet does not select a canonical migration owner.
- This packet does not reset, merge, or rewrite any branch.
- This packet does not change package manifests or lockfiles.

## Evidence Sources

- `docs/agent/full-system-verification-2026-05-09.md`
- `agent/state/TMP-026.handoff.json`
- `agent/state/TMP-034.handoff.json`
- `agent/state/TMP-035.handoff.json`
- `agent/state/TMP-036.handoff.json`
- `agent/state/TMP-037.handoff.json`
- `agent/state/TMP-038.handoff.json`
- `slices/TMP-041-runtime-schema-source-inventory/value-gate-report.md`
