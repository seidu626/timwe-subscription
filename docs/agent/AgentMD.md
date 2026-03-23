# AgentMD Notes

## 2026-02-25 - TIMWE confirm SUCCESS semantics + null message sharp edge
- Expected: `code: "SUCCESS"` from `/subscription/optin/confirm` would always be non-final unless explicit terminal status was present.
- Observed: In production flow, OTP confirm can be truly successful while upstream still returns `code: "SUCCESS"` without terminal `responseData` markers.
- Impact: Acquisition marked confirm as not successful, then LP showed a literal `"null"` error string and reset flow despite successful subscription.
- Fix/workaround:
  - Treat confirm `SUCCESS` as final unless explicit pending indicators are present (`OPTIN_WAITING`, `OPTIN_PIN_WAITING`, etc.) or message clearly indicates pending.
  - Sanitize upstream/provider messages so `"null"`/`"nil"` are treated as empty and fallback copy is used.
  - Add LP-side message normalization to avoid rendering `"null"` to users.
- Prevent recurrence: Keep tests for both cases (pending SUCCESS and final SUCCESS) and preserve message sanitization in both backend and LP parser.

## 2026-03-23 - HE edge config ships in discovery mode, so port-80 noise is expected
- Expected: HTTP HE capture would only trust real operator proxy CIDRs at the NGINX edge.
- Observed: `ops/nginx/conf.d/he-bootstrap.conf` currently has `geo $he_trusted_proxy { default 1; }`, which means discovery mode trusts all sources unless the config is tightened for production.
- Impact: Internet scan traffic can reach the HE bootstrap locations and produce misleading HE/campaign noise, while application-layer HE extraction still rejects identity unless `HE_TRUSTED_PROXY_CIDRS` is configured.
- Fix/workaround:
  - Replace discovery mode with explicit operator CIDRs before treating HE logs as signal.
  - Separate non-HTTP scanner noise from genuine `/c/:slug` campaign fetches when debugging HE.
- Prevent recurrence: Validate both NGINX trusted-proxy config and `HE_TRUSTED_PROXY_CIDRS` together during rollout; otherwise the edge and app trust models diverge.

## 2026-03-23 - Podman build path requires fully qualified upstream image names
- Expected: `make docker-build-all` would resolve base images like `krakend:latest` and `golang:1.24-alpine` the same way Docker often does.
- Observed: this environment uses Podman's Docker CLI emulation with no `unqualified-search registries` configured, so unqualified image names fail immediately during `docker build` and `docker run`.
- Impact: Dockerfile stages and KrakenD validation targets fail before service code is built, starting with `krakend/Dockerfile`.
- Fix/workaround:
  - Use fully qualified Docker Hub references such as `docker.io/library/krakend:latest`, `docker.io/library/golang:1.24-alpine`, `docker.io/library/node:20-alpine`, `docker.io/library/nginx:alpine`, and `docker.io/library/alpine:latest`.
  - Apply the same qualification to direct `docker run ... krakend` invocations in the Makefile.
- Prevent recurrence: treat short image names as environment-sensitive; qualify upstream images in repo-owned Dockerfiles and validation scripts rather than relying on host registry search config.

## 2026-03-23 - Docker push failures were auth/namespace preflight issues, not blob reuse bugs
- Expected: `make docker-push-subscription-external` and `make docker-push-acquisition-api` would either push successfully or fail with a clear login/namespace error before contacting Docker Hub.
- Observed: Podman attempted the push, then failed deep in blob reuse with `requested access to the resource is denied`, while `podman login --get-login docker.io` showed no active Docker Hub login. The Makefile also hardcoded `DOCKER_USER = xper626`, which prevented easy namespace override.
- Impact: push failures looked like registry corruption or blob issues even though the real problem was missing Docker Hub auth or a mismatched target namespace.
- Fix/workaround:
  - Make `DOCKER_USER` overrideable and add a `docker-login-check` preflight before all push targets.
  - Tag the image with an explicit `docker.io/...` destination before pushing so Podman does not rely on implicit registry resolution.
  - If pushing to another namespace, run `make DOCKER_USER=<namespace> docker-push-...` after `docker login docker.io`.
- Prevent recurrence: treat push auth as a first-class precondition in repo-owned automation; fail before the registry push starts so the operator sees the real issue immediately.

## 2026-03-23 - Docker Hub token exchange can fail intermittently during Podman blob reuse
- Expected: once Docker Hub login is valid, repeated `podman push` or `docker push` of the same image should behave consistently.
- Observed: identical pushes alternated between success and `invalid username/password ... must log in with a Personal Access Token (PAT)` while reusing blobs, then succeeded on the next retry without any credential change. The successful debug trace showed the error happens during Docker Hub token exchange for cross-repo blob mounts, not at initial local auth lookup.
- Impact: `make docker-push-all` can fail nondeterministically on a single image even when credentials are valid.
- Fix/workaround:
  - Retry the full `docker push` command in the Makefile push macro instead of failing on the first registry-token error.
  - Keep the explicit `docker.io/...` destination tags so the push path is deterministic.
- Prevent recurrence: treat Docker Hub token exchange as flaky under Podman blob reuse and add bounded retries at the automation layer.

## 2026-03-23 - Admin postback status list returns `items`, not `postbacks`
- Expected: the Angular admin client could bind `GET /v1/admin/postbacks/status/{status}` directly into a `{ postbacks: [...] }` response shape like the transaction lookup endpoint.
- Observed: the backend status-list handler returns `{ status, count, items }`, while the admin UI initially consumed `response.postbacks`, which leaves the status/DLQ table empty even though the API responds with rows.
- Impact: postback stats may render, but DLQ/status management can look unimplemented because the table stays empty.
- Fix/workaround:
  - Normalize the response in the admin frontend service so it accepts either `items` or `postbacks`.
  - Treat future status-management UI work as contract-sensitive because the lookup and status-list endpoints do not return the same top-level keys.
- Prevent recurrence: add a frontend regression test around the service mapping before changing status-management UI again.

## 2026-03-23 - Global HTTP retry retried admin POST actions
- Expected: a transient failure on admin `POST` actions such as trigger-postback, retry, or bulk requeue would surface once to the operator without automatically replaying the mutation.
- Observed: `frontend/webspa-admin/src/app/core/http-interceptors/http-error.interceptor.ts` applied `retry(1)` to every HTTP request, including non-idempotent admin mutations.
- Impact: operators could accidentally enqueue duplicate postbacks or replay recovery actions after a single failing click.
- Fix/workaround:
  - Restrict automatic retries to idempotent methods (`GET`, `HEAD`, `OPTIONS`) only.
  - Keep mutation errors visible in the UI so operators can decide whether to retry manually.
- Prevent recurrence: maintain a regression test that proves `POST` requests are not retried automatically.

## 2026-03-23 - Admin postback bulk requeue also needs to refresh active transaction lookup
- Expected: after bulk requeueing DLQ items from the postback admin page, any active transaction-specific lookup on the same page would reflect the new statuses immediately.
- Observed: the bulk requeue handler refreshed only the status list and stats cards; the transaction lookup result stayed stale until a manual search rerun.
- Impact: admins can think bulk requeue did not affect the selected transaction even though the backend succeeded.
- Fix/workaround:
  - After bulk requeue succeeds, rerun the transaction lookup when a `transactionId` is active.
- Prevent recurrence: keep a component-level test covering bulk requeue plus active search state.

## 2026-03-23 - Clipboard actions need failure handling in admin tables
- Expected: copy buttons in the admin UI would either copy successfully or show a clear user-facing error.
- Observed: both transaction and postback admin screens assumed `navigator.clipboard.writeText(...)` always succeeds; denied permission or insecure-origin cases surface as unhandled promise rejections.
- Impact: copy actions can fail silently or throw noisy runtime errors outside local happy-path testing.
- Fix/workaround:
  - Catch clipboard promise rejections and show an error snackbar.
- Prevent recurrence: keep at least one unit test around clipboard rejection handling for admin action components.

## 2026-03-23 - Transaction stats and table defaulted to different date windows
- Expected: the admin transaction stats cards and the transactions table would use the same default date range so the counts match the rows being reviewed.
- Observed: the stats request defaulted to the last 7 days when filter dates were blank, while the table request sent no default `start_date` or `end_date`, so the cards could show a much smaller total than the visible list.
- Impact: operators can misread queue health and campaign performance because the summary cards and row set are describing different time windows.
- Fix/workaround:
  - Initialize the transaction list filters with the same default start/end dates used by the stats request and restore those defaults on Clear.
- Prevent recurrence: keep a lightweight unit test around the initial filter state so future UI changes do not reintroduce the mismatch.

## 2026-03-23 - DLQ list and bulk requeue must share the same page slice
- Expected: when admins bulk requeue DLQ rows from the postback screen, the action should target the same rows currently visible in the DLQ table.
- Observed: the status list originally returned a newest-first limited slice, while the bulk requeue endpoint reset oldest-first rows with no `offset`, so the UI could look unchanged after a successful requeue.
- Impact: operators can assume the action failed and repeat the mutation, even though the backend changed different hidden rows.
- Fix/workaround:
  - Add `offset` support to both the status-list and bulk-requeue admin endpoints.
  - Return the total matching row count and page metadata to the frontend so the DLQ table can paginate and requeue the current visible page.
- Prevent recurrence: treat status-management actions as contract-sensitive and verify list ordering plus mutation targeting together whenever this admin surface changes.
