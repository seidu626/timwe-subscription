# DigitalOcean Service Deploy Evidence

Date: 2026-06-12
Mode: service-level deploy
Commit: `8bb8b0b`
Services:

- `subscription-external`
- `webspa-admin`

## Purpose

Deploy the Careerify multi-tenant opt-in key-selection fix and the tenant workspace selection fix to the DigitalOcean droplet without restarting unrelated services.

## Local Preflight

- `git fetch origin --prune`
- `git status --short`
- `git rev-list --left-right --count origin/main...HEAD`
- `docker compose -f docker-compose.prod-do.yml config --quiet`
- `just krakend-check-do`

The KrakenD and compose checks passed. `docker` on this host is Podman-backed, so image build output included the expected OCI healthcheck warning.

## Local Verification Before Deploy

Subscription external:

- `go test ./internal/service`
- `go test ./internal/...`
- `go test ./cmd`
- `go build -o subscription-external ./cmd`
- local `GET /health` returned HTTP 200

Web admin:

- focused tenant workspace service test passed under `ChromeHeadlessNoSandbox`
- focused 403, guard, and interceptor tests passed under `ChromeHeadlessNoSandbox`
- `npm run build:prod` passed

The frontend production build retained existing warnings:

- `campaign-form.component.scss` exceeded the component style budget by `2.47 kB`
- one CSS selector warning for `.form-floating>~label`

## Git Integration

- Commit created: `8bb8b0b fix(tenant): repair Careerify opt-in and workspace selection`
- `git push origin main` completed successfully.
- Post-push state: `origin/main...HEAD` reported `0 0`.

## Remote Preflight

Remote host:

- `debian-s-2vcpu-2gb-90gb-intel-fra1-01`
- deploy directory: `~/services/nouveauricheglobalgroup`

Verified before mutation:

- `nginx` active as a host systemd service
- `krakend` active as a host systemd service
- app stack running through Docker Compose in the deploy directory
- remote deploy directory is not a git checkout, so image push plus `deploy.sh <service>` is the correct deploy path

Rollback baseline before deploy:

- previous `subscription-external` image ID: `22eb5e3c13f8`
- previous `webspa-admin` image ID: `b78e707c1d8a`

## Deploy Commands

- `just deploy-subscription-external`
- `just deploy-webspa-admin`

Both commands built, pushed, and restarted only their target service through the remote deploy script.

New remote images:

- `subscription-external`: `xper626/subscription-external:latest`, image ID `698f44751fc9`
- `webspa-admin`: `xper626/nr-subscription-webspa-admin:latest`, image ID `57ae640f32e0`

## Post-Deploy Verification

Remote compose status:

- `subscription-external`: `Up`, `healthy`, port `8083`
- `webspa-admin`: `Up`, `healthy`, host-bound port `127.0.0.1:4200`

Host services:

- `nginx`: active
- `krakend`: active

Public route checks:

- `https://admin.nouveauricheglobalgroup.com/` returned HTTP 200.
- `https://api.nouveauricheglobalgroup.com/health` returned HTTP 200 with body `healthy`.

Deployed frontend asset check:

- public admin HTML references `main-3KG2VIFI.js`
- local production build produced `main-3KG2VIFI.js`

Filtered post-deploy logs:

- `subscription-external` reported healthy system checks and successful monitor data sync.
- no filtered `error`, `warn`, `panic`, or `fatal` lines were observed in the checked post-deploy slice for the changed services.

## Secrets Handling

No `.env` values, Auth0 secrets, JWT secrets, TIMWE credentials, provider PSKs, bearer tokens, or raw subscriber numbers were included in this evidence.

## Residual Risk

The authenticated admin workspace browser flow was not replayed with the user's live browser session during deploy verification. The deployed bundle matches the locally tested production build that contains the tenant workspace fix, and the public admin route is serving successfully.

## Rollback Handle

If rollback is needed, redeploy the previous images captured before the deploy:

- `subscription-external`: image ID `22eb5e3c13f8`
- `webspa-admin`: image ID `b78e707c1d8a`

The source rollback point before this deployment is the parent of commit `8bb8b0b`.
