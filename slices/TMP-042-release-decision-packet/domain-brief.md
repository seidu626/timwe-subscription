# TMP-042 Domain Brief

## Actor

Platform operator and repo maintainer.

## Business Outcome

The operator can distinguish previous local full-system verification blockers from release-owner decisions that still require explicit production, provider, credential, deploy, or branch-integration approval.

## Domain Invariant

Approval-gated work is not executable until the approval is recorded in a durable artifact. Evidence that a blocker exists is not the same as permission to change schema, dependencies, gitlinks, or branch history.

## Entrypoint

Supervisor blocked queue, the full-system verification matrix, and the current release-decision packet.

## Trigger

Parent batch requests a TMP-042 refresh after newer local full-system verification evidence exists.

## Risk

If the decision packet is mistaken for approval, an agent could deploy, mutate schema, change dependencies, rewrite branch history, or alter submodule strategy without maintainer intent. TMP-042 explicitly avoids making production release or deploy decisions.

## Failure Modes

- Missing required decision: production/live-provider proof remains absent because no release owner has scoped it.
- Ambiguous decision: release proceeds from a local verification packet without deploy or credential proof.
- Scope drift: a decision packet changes runtime files instead of documenting the decision surface.
