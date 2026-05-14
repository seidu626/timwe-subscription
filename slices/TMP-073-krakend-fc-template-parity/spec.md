# TMP-073 — krakend-fc-template-parity

## User story
As a release engineer, the KrakenD container built from `docker-compose.yml` serves the same set of endpoints as the static `krakend/krakend.json` reference, so TMP-067/TMP-068 routing for careerify (and future tenants) works in production, not only when running KrakenD directly against the static config.

## Background
TMP-067 and TMP-068 added the tenant-aware notification and subscription endpoints to `krakend/krakend.json` (4373 lines, the rendered reference). However, the runtime image built by `krakend/Dockerfile` ignores `krakend.json` — its entrypoint is:

```
CMD ["krakend", "run", "-dc", "/etc/krakend/config/krakend.tmpl"]
```

The `-dc` flag is KrakenD Flexible Configuration. The image renders the live config from `krakend/config/krakend.tmpl` + `partials/` + `templates/` at startup and writes the output to `/tmp/krakend.json`. None of TMP-067 or TMP-068's new endpoints appear in any FC template:

```
$ grep -l "external/v1/{tenant_key}" krakend/config/templates/*.tmpl
(no matches)
$ grep -c "subscriptions/optin" krakend/config/templates/Endpoint.tmpl
0
```

This means the production gateway runtime does NOT serve `/api/v1/notification/...` with `input_query_strings` for tenant_key/channel_key, and does NOT serve `/api/external/v1/{tenant_key}/{channel_key}/subscriptions/{op}` at all. TMP-067/068 are effectively "shipped to the static reference" but "not deployed to the FC runtime".

Evidence: `slices/TMP-070-careerify-tenant-e2e-smoke/value-gate-report.md` "Verdict and ownership of gaps" row 6.

## Scope
- `krakend/config/templates/Endpoint.tmpl` — add the 6 notification endpoint partials with `input_query_strings: ["tenant_key", "channel_key", "external-tx-id"]` and the martian `header.Modifier` block injecting `X-Tenant-Key`/`X-Channel-Key`.
- `krakend/config/templates/Endpoint.tmpl` (or a new dedicated `SubscriptionTenantEndpoint.tmpl` invoked from it) — add the 4 subscription endpoints with path-param capture and the path rewrite to `/api/v1/subscription-external/admin/{op}` (or whatever path TMP-072 lands on).
- Verify by rendering: `FC_ENABLE=1 FC_SETTINGS=krakend/config/settings FC_PARTIALS=krakend/config/partials FC_TEMPLATES=krakend/config/templates FC_OUT=/tmp/rendered.json krakend check -dc krakend/config/krakend.tmpl --debug` and diff `/tmp/rendered.json` against `krakend/krakend.json` for the 10 new endpoints.
- `scripts/smoke/careerify-tenant-e2e.sh` — run against a containerized KrakenD built from `docker-compose up krakend`, not against the static config.

## Out of scope
- Notification handler enforcement (TMP-071).
- Subscription gateway auth (TMP-072) — this slice's smoke run will still surface that gap, but TMP-072 closes it independently.
- Changes to legacy endpoints not affected by tenant routing.

## Acceptance criteria
- `FC_OUT=/tmp/rendered.json krakend check -dc krakend/config/krakend.tmpl` succeeds and the rendered output contains all 6 notification endpoints (with input_query_strings) and all 4 subscription endpoints (with path captures + header injection).
- `docker-compose up krakend` produces a running gateway that serves the same 10 endpoints as the static-config smoke from TMP-070.
- `bash scripts/smoke/careerify-tenant-e2e.sh` against the containerized gateway matches the TMP-070 static-config matrix (notification 6/6; subscription pending TMP-072).
- A short markdown note in `docs/tenant-channel-onboarding.md` records the FC layout for new tenants so future onboardings update both the static reference and the templates together.

## Dependencies
- TMP-067, TMP-068 (shipped — to mirror their static-config endpoints into the FC layer).
- TMP-072 is independent — the FC parity work doesn't depend on which auth approach TMP-072 lands; if TMP-072 changes the upstream `url_pattern`, refresh the template.

## Risk
Low–medium. Pure gateway config templating; no Go code, no DB. Risk is misaligning FC partials with the static reference and silently breaking other endpoints. Mitigate by diffing the full rendered output against `krakend/krakend.json` before merging.

## Verification
```
# Render the FC config and diff against the static reference
docker run --rm -v $(pwd)/krakend:/etc/krakend:ro,Z \
  docker.io/library/krakend:latest check -dc /etc/krakend/config/krakend.tmpl

# Bring up gateway via compose and re-run smoke
docker-compose up -d krakend
HOST=http://127.0.0.1:8090 bash scripts/smoke/careerify-tenant-e2e.sh
```
