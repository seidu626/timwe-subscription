# TMP-042 Domain Brief

## Actor

Platform operator and repo maintainer.

## Business Outcome

The operator can make the minimum set of explicit decisions needed to unblock full-system release verification without inferring approvals from repeated progress reports.

## Domain Invariant

Approval-gated work is not executable until the approval is recorded in a durable artifact. Evidence that a blocker exists is not the same as permission to change schema, dependencies, gitlinks, or branch history.

## Entrypoint

Supervisor blocked queue and the full-system verification matrix.

## Trigger

`agent-supervisor auto-loop --max-rounds 1` reports no ready tasks while full-system verification remains blocked.

## Risk

If the decision packet is mistaken for approval, an agent could mutate schema, dependencies, branch history, or submodule strategy without maintainer intent. TMP-042 explicitly preserves all existing blockers and only consolidates the decision surface.

## Failure Modes

- Missing required decision: a blocked slice remains blocked because no approval artifact exists.
- Ambiguous decision: implementation proceeds with an assumed strategy and hides release risk.
- Scope drift: a decision packet changes runtime files instead of documenting the decision surface.
