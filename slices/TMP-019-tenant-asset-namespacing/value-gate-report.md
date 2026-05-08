# TMP-019 Value Gate Report

Verdict: PASS

## Slice

Tenant admin presigned campaign background uploads are isolated by tenant namespace and reject unsafe asset file inputs.

## Review Gates Applied

- Claude critique applied: object keys now include tenant namespace, filename traversal is rejected before extension inference, and storage failures map through a generic sentinel.
- Parallel domain review applied: public campaign DTO tenant enforcement is deferred to TMP-005 because campaigns are not tenant-owned yet.
- Story craft applied: actor, entrypoint, outcome, failure journeys, and invariants are captured in `slice.yaml` and `domain-brief.md`.
- Roadmap-to-slices applied: TMP-019 remains a dependency for TMP-005 and narrows its shipped boundary to storage key isolation.

## Acceptance Coverage

- Tenant asset key generated: `buildBackgroundObjectKey` emits `campaign-backgrounds/tenants/{tenant}/{campaign}/...`.
- Public asset URL is tenant-safe: response asset URL is constructed from the server-generated tenant object key.
- Cross-tenant key collision: two tenants using the same campaign slug and asset id produce distinct keys.
- Path traversal: handler and service tests reject traversal, nested paths, backslashes, and control characters with 400/service validation errors.
- Missing tenant: admin presign endpoint returns 403 before storage calls.
- Storage backend failure: MinIO/S3 errors are wrapped as `ErrCampaignAssetStorageUnavailable`; handler returns generic 503.

## Verification

Commands run:

```bash
cd services/acquisition-api && go test ./internal/service ./internal/handler ./internal/transport
```

Results:

- Focused acquisition-api service, handler, and transport tests passed.

## Gaps Deferred

- Campaign ownership checks and public campaign DTO stripping require TMP-005 to add `tenant_id` to campaign rows.
- Legacy global asset compatibility requires the campaign tenant migration path introduced by TMP-005 or the migration isolation slice.
