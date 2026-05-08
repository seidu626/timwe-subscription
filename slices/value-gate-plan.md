# Value Gate Plan

No implementation has been written in this planning slice, so value-gate cannot honestly PASS or FAIL any feature code yet. This file defines the required gate for every slice before implementation can be called complete.

For each `slices/TMP-*/slice.yaml` after build:

1. Criteria coverage: every happy, failure, edge, and invariant criterion must map to a test file and named test.
2. Failure coverage: at least invalid input, missing required, authorization, dependency failure, and duplicate/conflict where applicable.
3. Invariant preservation: tenant isolation, auditability, credential secrecy, idempotent outbox behavior, and existing single-tenant compatibility must have positive and negative tests.
4. Journey completeness: the actor journey in the slice must be demonstrable through route/service tests or smoke tests.
5. Test quality: no assertion-free tests; no status-only coverage without body/state assertions; mocks must not replace all observable behavior.

Required output after implementation: `slices/<id>/value-gate-report.md` with PASS, FAIL, or CONDITIONAL. A FAIL blocks slice-verify.
