# TMP-037 Decision Template: Landing Web Dependency Remediation

Status: proposed

Approval recorded: no

## Context

`npm audit` reports Next/PostCSS advisories for `services/landing-web`. The available npm remediation proposes a breaking upgrade to `next@16.2.6`.

Dependency changes are approval-gated because they can alter runtime behavior and require UI regression proof.

## Decision Required

Choose one path before implementation:

- Approve breaking Next/PostCSS upgrade and required regression proof.
- Defer remediation with documented risk acceptance and revisit date.
- Choose a narrower patched dependency path if available and verified.

## Decision

Pending operator decision.

## Consequences To Review

- Compatibility with current Next routes and APIs.
- Required browser/runtime smoke coverage.
- Security risk if deferred.
- CI and lockfile behavior.

## Post-Decision Proof

```bash
cd services/landing-web && npm audit --audit-level=moderate
cd services/landing-web && npm run build
# browser/runtime smoke for landing pages and campaign API routes
```

## Slice Impact

- Blocks: `TMP-021`, `TMP-037`
- Evidence: `docs/agent/release-decision-packet-2026-05-09.md`, `agent/state/TMP-037.handoff.json`
