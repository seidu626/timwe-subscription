# TIMWE-CI-001 — Value Gate Report

Verdict: PASS (with pre-existing hard blockers tracked)

## Acceptance Criteria Coverage

- **Pipeline exists and is multi-lane**: `.github/workflows/ci.yml` now defines:
  - `angular` lane (webspa-admin)
  - `nextjs` lane (landing-web)
  - `go-services` lane (matrix over acquisition-api, billing, cadence-engine, notification, postback-dispatcher, subscription-external, subscription-partner)
- **Angular lane runs**:
  - `npm ci --legacy-peer-deps`
  - test (`npm run ng -- test --watch=false --browsers=ChromeHeadlessNoSandbox`)
  - build (`npm run ng -- build --configuration=production`)
- **Next.js lane runs**:
  - `npm ci`
  - `tsc --noEmit`
  - `npm run build`
- **Go lane runs**:
  - matrix `go vet ./...`
  - matrix `go test ./...`

## Failure-Mode Coverage

- Angular lint step is currently skipped in CI because this app has no Angular CLI lint target configured.
- Next.js `next lint` is unavailable in the installed Next version (16.2.6), so the lint step is safely skipped with a clear log message.
- `go vet` is red only for `services/subscription-external` with three vet findings:
  - lock copy in `internal/monitoring/system_health.go`
  - unreachable code in `internal/utils/batch_processor.go`
  - context cancel/use path issues in `internal/service/subscription.go` and `internal/handler/subscription_handler.go`
- All other services pass `go vet` and all seven services pass `go test ./...` in CI-equivalent commands.

## Evidence

```text
## Angular lane
cd frontend/webspa-admin
npm ci --legacy-peer-deps
npm run ng -- test --watch=false --browsers=ChromeHeadlessNoSandbox --progress=false
npm run ng -- build --configuration=production

## Next.js lane
cd services/landing-web
npm ci
npm run lint
npx tsc --noEmit
npm run build

## Go lane
cd services/acquisition-api && go vet ./... && go test ./...
cd services/billing && go vet ./... && go test ./...
cd services/cadence-engine && go vet ./... && go test ./...
cd services/notification && go vet ./... && go test ./...
cd services/postback-dispatcher && go vet ./... && go test ./...
cd services/subscription-external && go vet ./... && go test ./...
cd services/subscription-partner && go vet ./... && go test ./...
```

```text
cd /home/xper626/workspace/apps/timwe-subscription && \
rg -n \"cannot find \\\"lint\\\" target for the specified project|Invalid project directory provided\" \
  frontend/webspa-admin/. | sed -n '1,120p'
```
