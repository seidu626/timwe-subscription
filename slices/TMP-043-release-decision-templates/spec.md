# TMP-043 Spec

## Story

As a platform operator, I need pending ADR templates for each release blocker, so I can record approvals without first inventing the document structure.

## Acceptance

- Six pending decision templates exist under `slices/decisions/`.
- Each template states that approval is not recorded.
- Each template lists choices, consequences, proof, and slice impact.
- The slice changes only metadata and evidence files.

## Non-Goals

- Do not approve any decision.
- Do not implement schema provisioning, dependency upgrades, gitlink changes, or branch integration.
- Do not edit runtime source, SQL, compose, package, or submodule files.
