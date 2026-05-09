# TMP-042 Spec

## Story

As a platform operator, I need one packet that lists every remaining full-system verification decision, so I can approve or reject the next implementation work deliberately.

## Acceptance

- A decision packet exists at `docs/agent/release-decision-packet-2026-05-09.md`.
- The packet covers TMP-021, TMP-026, TMP-034, TMP-035, TMP-036, TMP-037, and TMP-038.
- The packet names concrete decision options and post-approval verification proof.
- The packet explicitly says it does not approve any change.
- The change set is limited to harness, slice, and evidence files.

## Non-Goals

- Do not implement schema provisioning.
- Do not change SQL, compose, service code, package manifests, lockfiles, frontend code, submodules, or branch history.
- Do not record an approval on behalf of the operator.
