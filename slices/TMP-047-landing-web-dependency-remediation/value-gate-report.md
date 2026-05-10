# TMP-047 Value Gate Report

- Timestamp: 2026-05-10T05:30:00Z
- Agent: Codex
- Verdict: PASS
- Outcome code: outcome:verified

## Audit 1: Acceptance Criteria Coverage

- Dependency audit clean: COVERED by `cd services/landing-web && npm audit --audit-level=moderate` returning `found 0 vulnerabilities`.
- Landing-web build passes: COVERED by `cd services/landing-web && npm run build` returning exit 0 on Next 16.2.6.
- Runtime smoke returns HTTP 200: COVERED by standalone server on port 3138 and `curl` returning `200`.
- Scope stays bounded: COVERED by changed files limited to landing-web package metadata, Next-required dynamic route params compatibility, tsconfig, and slice evidence.

Audit 1 result: PASS.

## Audit 2: Failure Mode Coverage

- Audit remains vulnerable: COVERED by rerun audit; it exits 0 after remediation.
- Build regression: COVERED by Next production build; it exits 0 after dynamic params compatibility updates.
- Runtime regression: COVERED by standalone runtime smoke; `/` returns 200.

Audit 2 result: PASS.

## Audit 3: Domain Invariants

- Release verification must not claim readiness with moderate-or-higher landing-web advisories: PRESERVED by audit gate.
- Landing-web must remain buildable: PRESERVED by Next build gate.
- Public landing entrypoint remains reachable: PRESERVED by standalone runtime smoke.
- HE identity middleware remains present: PRESERVED; middleware file remains in place and build reports it as proxy/middleware.

Audit 3 result: PASS.

## Audit 4: User Journey

- Platform operator approves dependency remediation through TMP-037: COMPLETE, decision accepted.
- Agent upgrades dependency metadata and lockfile: COMPLETE, Next 16.2.6, React 19.2.6, and PostCSS 8.5.14 override are installed.
- Verifier runs dependency audit: COMPLETE.
- Verifier runs build: COMPLETE.
- Verifier checks `/`: COMPLETE.

Audit 4 result: PASS.

## Audit 5: Test Quality

No dedicated landing-web test suite exists in this service. This defect slice uses the executable release gates that exposed the blocker: dependency audit, production build, and runtime smoke.

Audit 5 result: PASS for the available release-gate proof.

## Commands

```bash
cd services/landing-web && npm audit --audit-level=moderate
cd services/landing-web && npm run build
cd services/landing-web && PORT=3138 node .next/standalone/workspace/apps/worktrees/codex-fullsystem-20260510-045911/services/landing-web/server.js
curl -sS -o /tmp/timwe-landing-root-standalone.html -w '%{http_code}\n' http://127.0.0.1:3138/
npm ls postcss next react react-dom
hvc check agent/backlog/issues/*.md --fail-on block
```

## Notes

- `next start` returned HTTP 200 during smoke, but Next warns that standalone output should use `node .next/standalone/.../server.js`; the value gate uses the standalone server as the canonical runtime proof.
- Next 16 still warns that `middleware.ts` is deprecated in favor of proxy. The warning does not fail build or smoke and no middleware behavior change was required for this slice.
