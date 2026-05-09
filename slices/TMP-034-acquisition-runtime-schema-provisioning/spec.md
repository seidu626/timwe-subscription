# TMP-034 Spec

## Objective

Classify and track the approval-gated acquisition-api runtime schema provisioning blocker discovered after TMP-030. Do not change schema, migrations, runtime code, compose files, dependencies, or credentials in this slice.

## Broken Behavior

Acquisition API exits during admin schema bootstrap because add_admin_management_tables.sql expects relation products in the empty compose DB.

## Expected Behavior

The compose DB schema provisioning path creates or migrates products and userbase before add_admin_management_tables.sql runs, so acquisition-api reaches health checks.

## Acceptance Proof

```bash
jq empty slices/manifest.json agent/state/TMP-034.work-order.json agent/state/TMP-034.handoff.json .agent/tasks.json
hvc check agent/backlog/issues/*.md --fail-on block
slice-harness status
slice-harness sync --dry-run
git diff --name-only
```

## Approval Gate

- Schema/migration provisioning is approval-gated by repo risk boundaries.
- The failing relation products/userbase path requires schema ownership and migration-order decision before implementation.
