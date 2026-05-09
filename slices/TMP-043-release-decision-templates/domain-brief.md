# TMP-043 Domain Brief

## Actor

Platform operator and repo maintainer.

## Business Outcome

Approvals required by the full-system verification blockers can be recorded in a consistent, durable ADR format.

## Domain Invariant

A proposed ADR template is not an accepted decision. Implementation remains blocked until the ADR status and decision are explicitly changed by an authorized operator or maintainer.

## Entrypoint

`docs/agent/release-decision-packet-2026-05-09.md`.

## Trigger

The decision packet identifies approval artifacts as the minimum gate for the remaining blocked implementation slices.

## Risk

Templates could be misread as approvals. Every new template therefore states `Status: proposed` and `Approval recorded: no`.
