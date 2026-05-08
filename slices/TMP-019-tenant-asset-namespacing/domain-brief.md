# TMP-019 Domain Brief: Tenant Asset Namespacing

## Actors

- tenant-admin: requests a presigned upload URL for a campaign background from a trusted tenant admin session.
- campaign asset service: validates file metadata, builds object-storage keys, and returns upload/public asset URLs.
- object storage backend: creates presigned PUT URLs for tenant-scoped campaign assets.
- landing runtime: later reads public campaign assets after campaign rows are tenant-bound.

## Ubiquitous Language

- Tenant namespace: sanitized tenant id or tenant key used as the storage isolation segment.
- Campaign segment: sanitized campaign slug used below the tenant namespace.
- Object key: storage path under `campaign-backgrounds/tenants/{tenant}/{campaign}/...`.
- Public asset URL: URL constructed by the server from the object key and configured CDN/storage base.
- Legacy asset: pre-tenant campaign media that does not yet carry a tenant namespace.

## Domain Invariants

- Presigned campaign background uploads require a trusted tenant identity.
- Object keys include a tenant namespace before the campaign segment.
- File names are treated as hints only; path separators, traversal, and control characters are rejected.
- Public asset URLs are constructed from server-owned object keys, not accepted as arbitrary upload output.
- Storage backend failures surface as a generic dependency error and do not echo credentials or backend internals.
- Two tenants using the same campaign slug cannot produce the same object key.

## Failure Modes

- Missing tenant context: admin presign endpoint returns 403 before parsing storage credentials or issuing a presign.
- Invalid file name: traversal, nested paths, Windows separators, and control characters return 400.
- Invalid content type or size: service rejects unsupported image types and oversized uploads before storage calls.
- Storage unavailable: presign errors are wrapped with `ErrCampaignAssetStorageUnavailable`; the handler returns generic 503.
- Legacy global asset path: full public-serving enforcement waits for TMP-005 because campaign rows do not yet have tenant ownership.

## User Journey

1. Tenant admin calls `POST /v1/admin/campaign-assets/background/presign` from a tenant-scoped admin session.
2. Handler requires tenant identity and passes the tenant namespace into the asset service.
3. Service validates campaign slug, file name, content type, and size.
4. Service builds `campaign-backgrounds/tenants/{tenant}/{campaign}/{timestamp}-{uuid}.{ext}`.
5. Service asks object storage for a presigned PUT URL and returns it with a public URL derived from the object key.

Failure journeys:

1. Caller has no tenant identity -> 403, no object key generated.
2. Caller sends `../background.png` -> 400, no storage call.
3. Object storage rejects presign -> 503 with a generic dependency message.

## Review Amendments Applied

- Claude critique required tenant namespaces in object keys, filename validation before extension override, and generic backend-failure responses.
- Parallel review confirmed this slice should not invent campaign ownership checks before TMP-005 adds tenant-bound campaign rows.
- Roadmap dependency remains: TMP-005 must connect campaign rows to tenants before public campaign DTOs can reject cross-tenant stored references.

## Open Questions

- Legacy global assets need a default-tenant migration path when TMP-005 adds campaign tenant ownership.
- Public campaign DTO filtering should move from URL-string validation to server-owned asset references when campaign asset metadata exists.
