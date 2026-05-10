# TMP-030 Domain Brief

## Slice

Acquisition API compose build context

## Actor

Verification agent running the full-system compose smoke.

## Outcome

The acquisition API image builds through the local compose path without relying on a missing service-local vendor tree.

## System Path

1. The verifier runs compose with `.env.example` and temporary local overrides.
2. Compose invokes the `acquisition-api` image build.
3. The build context includes both `services/acquisition-api` and the repo-local `common` module.
4. The Dockerfile copies module metadata first, downloads dependencies without mutating repo files, copies source, and builds with readonly module resolution.
5. The image build completes so compose can proceed to application startup checks.

## Invariants

- No Go source, dependency metadata, vendor tree, frontend, package manifest, or lockfile changes.
- The local `../../common` replacement remains valid inside the build image.
- Runtime failures discovered after image build are recorded as downstream defects, not hidden inside the build-context slice.

## Downstream Runtime Finding

After the image build fix, the acquisition API container reaches database connection and exits during admin schema bootstrap because `migrations/add_admin_management_tables.sql` expects a `products` table. That is a runtime schema-order defect candidate, outside the TMP-030 build-context fix.
