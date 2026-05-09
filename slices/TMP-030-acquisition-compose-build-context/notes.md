# TMP-030 Notes

- The original compose build used `context: ./services/acquisition-api`, so the Dockerfile could not resolve the repo-local `../../common` replacement.
- The original Dockerfile also used `go build -mod=vendor`, but the service does not have a vendor directory.
- The fix moves the compose build context to the repo root and keeps the Dockerfile path explicit.
- The Dockerfile copies `common` to `/common` and service files to `/build`, preserving the existing relative replacement target.
- Temporary empty Docker auth files allowed anonymous builder-image pulls without changing local credential configuration.
- After the image build passed, the compose smoke reached application startup. Several services responded to health checks, but acquisition API exited while bootstrapping admin schema because `migrations/add_admin_management_tables.sql` expects relation `products`.
