# TMP-022 Spec

## Objective

Fix the landing-web production build failure caused by conflicting dynamic segment names while preserving public URL shapes.

## Broken Behavior

`npm run build` in `services/landing-web` fails with:

```text
You cannot use different slug names for the same dynamic path ('slug' !== 'tenant')
```

The conflicting route families are:

- `app/lp/[slug]` and `app/lp/[tenant]/[slug]`
- `app/api/campaigns/[slug]` and `app/api/campaigns/[tenant]/[slug]`

## Expected Behavior

- `/lp/:slug` remains the legacy single-segment landing route.
- `/lp/:tenant/:slug` remains the tenant-qualified landing route.
- `/api/campaigns/:slug` remains the legacy single-segment campaign API route.
- `/api/campaigns/:tenant/:slug` remains the tenant-qualified campaign API route.
- `npm run build` passes without changing package versions.

## Implementation Constraint

Use a parameter-name compatibility fix only. Do not change static route segments, path depth, package files, or acquisition API behavior.

## Acceptance Proof

```bash
cd services/landing-web && npm run build
```
