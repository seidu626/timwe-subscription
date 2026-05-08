# Value Gate Plan

No implementation has been written in this planning slice, so value-gate cannot honestly PASS or FAIL any feature code yet. This file defines the required gate for every slice before implementation can be called complete.

For each `slices/TMP-*/slice.yaml` after build:

1. Criteria coverage: every happy, failure, edge, and invariant criterion must map to a test file and named test.
2. Failure coverage: at least invalid input, missing required, authorization, dependency failure, and duplicate/conflict where applicable.
3. Invariant preservation: tenant isolation, auditability, credential secrecy, idempotent outbox behavior, and existing single-tenant compatibility must have positive and negative tests.
4. Journey completeness: the actor journey in the slice must be demonstrable through route/service tests or smoke tests.
5. Test quality: no assertion-free tests; no status-only coverage without body/state assertions; mocks must not replace all observable behavior.
6. Contract evidence: public, partner, callback, gateway, and worker entrypoints must include request/response or payload fixtures when the slice changes an external contract.
7. Operational evidence: slices that touch secrets, migrations, workers, callbacks, or charge flows must include runbook, rollback, health, or metric evidence.
8. Named evidence format: each implementation value-gate report must include `criterion_id`, `test_file`, `test_name`, `assertion_type`, and PASS/FAIL for every criterion.
9. Invariant polarity: every invariant must have at least one positive test proving allowed behavior and one negative test proving denied behavior.

Before build starts for each slice:

1. Scope check: the slice must name allowed service/module ownership and explicitly push unrelated UI, migration, or ops breadth into dependent slices.
2. Tenant route check: if the slice touches public routes, callbacks, workers, or reports, tenant resolution and trust boundary must be named.
3. Channel capability check: if the slice touches opt-in, confirm, MT, charge, notification, cadence, postback, or renewal, unsupported capability behavior must be tested.
4. Compatibility check: existing Ghana/TIMWE single-tenant behavior must have a smoke or repository-level compatibility assertion.

Required output after implementation: `slices/<id>/value-gate-report.md` with PASS, FAIL, or CONDITIONAL. A FAIL blocks slice-verify.
