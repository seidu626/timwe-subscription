# Droplet production review — 2026-09-23

Host: `139.59.135.253` (`debian-s-2vcpu-2gb-90gb-intel-fra1-01`). This is a
service snapshot with the production changes described below. External reachability was tested
from one client; it does not establish who uses each port.

## Completed

- `api.nouveauricheglobalgroup.com` and
  `admin.nouveauricheglobalgroup.com` serve trusted Let's Encrypt certificates
  through 2026-12-22. The existing `certbot.timer` remains active.
- `cert-watch.timer` scans Certbot lineages, nginx TLS hosts and served
  certificates, and host/Docker TLS listeners daily. It records a group-readable
  report at `/var/lib/cert-watch/report.json` and sends success/failure signals
  to Healthchecks.io. The operator confirmed receipt of the original alert.
  The private ping URL was rotated after exposure in a diagnostic response;
  the replacement check received the current failure and the original check
  was removed. The operator confirmed receipt of an alert from the
  replacement check.
- MinIO's admin console moved from public `:9101` to
  `127.0.0.1:9101` in the live Compose file and repository source. The MinIO
  image and `/data` volume were unchanged. Local console returned HTTP 200,
  S3 health on `:9100` passed, `acquisition-api` stayed healthy, and external
  TCP access to `:9101` failed after the change. The previous Compose file is
  `/home/xper626/services/nouveauricheglobalgroup/docker-compose.yml.before-minio-console-20260923`.
- Six NRG application ports (`:3000`, `:8081`–`:8084`, `:8091`) now bind to
  `127.0.0.1` in the live Compose file and repository source. Each container
  was recreated from its existing image and passed its health check. These
  ports remained accessible locally but failed external TCP probes; the public
  API and admin hosts returned HTTP 200. The previous Compose file is
  `/home/xper626/services/nouveauricheglobalgroup/docker-compose.yml.before-app-loopback-20260923`.
- The TIMWE Compose source and live droplet file now require explicit
  `PG_HOST` and `PG_PORT` instead of defaulting to host PostgreSQL. The live
  `.env` classifies as managed PostgreSQL on port 25060 with SSL mode `require`;
  the rendered Compose configuration SHA-256 remained
  `84f215f6e47806f78f40c1d84b47808bf2869d0b06742e24c54cc54a352ea8ee`
  before and after. No containers were recreated. The previous live file is
  `/home/xper626/services/nouveauricheglobalgroup/docker-compose.yml.before-pg-retire-20260923`.
- KrakenD's live template and DO settings render `listen_ip=127.0.0.1`.
  The rendered configuration passed `krakend check`, and the previously
  approved service restart was applied. `ss` now shows only
  `127.0.0.1:8080`; local gateway health and the public API returned HTTP
  200. A direct external TCP handshake to public `:8080` still completed,
  but HTTP returned an empty reply and the host showed no matching established
  connection during a held probe. Direct application access was not observed.

## Open findings

| Priority | Finding | Evidence and next action |
| --- | --- | --- |
| Urgent | `lp` and `landing.nouveauricheglobalgroup.com` still use the Sectigo wildcard, expiring 2026-09-30 23:59 UTC. | Both DNS names returned NXDOMAIN and the certificate audit reported about 7.8 days remaining on 2026-09-23. Add A records to `139.59.135.253`, then run the scoped `nrg-cert-maintenance enroll` action and verify public SNI. |
| High | Host PostgreSQL could reopen public `:5432` after a reboot or manual restart. | Before the stop, `ss` showed `0.0.0.0:5432` and `[::]:5432`, and an external TCP probe connected. The cluster's `/etc/postgresql/15/main/start.conf` still says `auto`, and systemd reports `enabled-runtime`; the approved interface-specific UFW deny for public `:5432` remains unverified. Keep the cluster stopped until a separate change sets startup to `manual` or the firewall rule is confirmed. |
| Completed | Host PostgreSQL stopped after a monitored trial. | The scoped helper audited one application database, `subscription_manager` (7.6 GB), with zero current clients. All seven TIMWE containers use managed PostgreSQL on port 25060 and the same database name. `pg_dumpall` completed to root-only `/var/backups/nrg-pg-retire/pg15-main-20260923T060820Z.sql.gz` (398,635,354 bytes; SHA-256 `6eeb4ecd4a4d3e493af829810bc935593bb3c5d75fc8927a0968d2e027be1fef`); gzip integrity passed. The host cluster was stopped at about 06:09 UTC; after about ten minutes of healthy container, public API/admin, and database-backed product-list checks, the automatic rollback timer was canceled. PostgreSQL is inactive, `pg_lsclusters` reports down, and no `:5432` listener remains. Connection logging was off, so historical session timing is unknown. Rollback: `sudo -n /usr/local/sbin/nrg-pg-retire start`. |
| High | Veritas Quest ClamAV is unhealthy and `clamd` is absent. | Health checks report `Unable to contact server`; `docker top` shows only init, tail, and freshclam. `docker inspect` reports `OOMKilled=true`, 59,804 consecutive failed health checks, and a 1.5 GiB memory limit; freshclam still updates signatures but cannot notify clamd. Production media scanning code rejects/quarantines uploads on scanner failure, so this is an availability problem for media uploads rather than observed malware bypass. [ClamAV's official Docker guide](https://docs.clamav.net/manual/Installing/Docker.html) recommends at least 3 GiB for the scanner and says reloads can briefly need more than 2.4 GiB; the shared droplet has 3.8 GiB total and about 2.3 GiB available after the host PostgreSQL stop. Plan a dedicated scanner host or a host-memory upgrade before restarting/recreating with a viable limit; a restart under the current 1.5 GiB cap may repeat the OOM. |
| High | MinIO S3 `:9100` remains public over HTTP. | It was deliberately left unchanged during console hardening. Confirm whether public asset links depend on it; plan a TLS endpoint and access policy before restricting it. The production Compose source also has `minioadmin` fallback credentials; the live credential value was not inspected. |
| Medium | `mongod.service` is enabled but failed since 2026-08-12. | No MongoDB container is running and `systemctl --failed` lists the host service. Confirm whether any application still uses its data before repair or retirement. |
| Medium | Backup recovery remains unverified. | Scoutsphere's backup container logged daily successful dumps through 2026-09-22. Off-host copies and a restore test were not verified. Host `/var/backups` inspection showed package backups, not evidence of application recovery. |
| Medium | Production update and rollback hygiene needs review. | Nine packages were listed as upgradable, including Docker and `tzdata`; several running images use mutable `latest` tags. Schedule updates and pin image references after a service-by-service rollback review. |

## Host snapshot

- Debian 12, kernel `6.1.0-51-amd64`; 3.8 GiB RAM with 2.1 GiB available,
  2 GiB swap with about 0.5 GiB used, root disk 49% used.
- 21 running containers; one unhealthy (ClamAV). `nginx`, `krakend`,
  Docker, fail2ban, UFW, and unattended upgrades were active. Host PostgreSQL
  15 was active at intake and is now stopped after the monitored trial.
  `cert-watch.service` is failed by design while the reported certificate
  problems persist; `mongod.service` is the other failed unit.
- The Docker host had about 9 GiB of reclaimable images and 4.8 GiB of build
  cache. No prune was run; old images may be needed for rollback.

No credential values, database contents, firewall rules, cloud backups, or
provider-side direct-client records were inspected. The account has scoped
passwordless sudo for certificate maintenance only, so privileged firewall
and PostgreSQL configuration review is pending.
