# TMP-047 Spec: Landing Web Dependency Remediation

## Story

As a platform operator, I want the landing-web dependency chain upgraded so release verification is not blocked by known Next/PostCSS advisories.

## Scope

In scope:
- Update `services/landing-web/package.json` and `package-lock.json` to remediate the Next/PostCSS audit blocker.
- Apply minimal Next configuration or middleware compatibility changes only if the upgraded framework requires them.
- Apply minimal dynamic route/page params compatibility changes only where Next 16 type-checking requires them.
- Prove dependency audit, build, and a bounded runtime smoke.

Out of scope:
- Landing page redesign or route behavior changes.
- HE simulation feature changes.
- Admin frontend dependency remediation.
- Compose, schema, or Go service changes.

## Acceptance Criteria

1. `cd services/landing-web && npm audit --audit-level=moderate` exits 0.
2. `cd services/landing-web && npm run build` exits 0.
3. A bounded local runtime smoke starts the built app and receives HTTP 200 from `/`.
4. The change stays within landing-web package/config files, required dynamic route/page params compatibility edits, and slice evidence.

## Architecture Notes

This is a dependency-bound defect slice, not a broad frontend migration. Context7 Next.js documentation for v16 reports a Node.js minimum of 20.9.0 and React/React DOM 19 requirements; the current runtime is Node 24, so the slice can update the landing-web package set and verify behavior locally.
