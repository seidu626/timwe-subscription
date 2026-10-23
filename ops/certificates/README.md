# Droplet certificate maintenance

`certbot.timer` renews every Certbot lineage when due. It is enabled on the
droplet and runs twice daily. The NRG lineage has a deploy hook at
`/etc/letsencrypt/renewal-hooks/deploy/90-nrg-api-admin-nginx-reload` to test
and reload nginx after renewal. Other nginx lineages use Certbot's nginx
installer.

`cert-watch.timer` runs daily at 04:30 UTC, with up to 30 minutes of random
delay. Its service discovers:

- every Certbot renewal lineage;
- every loaded nginx TLS server block and its configured certificate;
- the certificate nginx actually serves for each named host;
- TLS listeners on host and Docker exposed TCP ports;
- PostgreSQL and local SMTP STARTTLS endpoints.

It writes `/var/lib/cert-watch/report.json` and exits with failure when a
certificate is expired, a Certbot certificate has at most 14 days remaining,
an unmanaged certificate has at most 30 days remaining, a named nginx host
has missing DNS or a mismatched certificate, or discovery fails. Exim's
generated short lived loopback SMTP certificate is recorded and only flagged
when expired. The service failure is visible in systemd and the issue list is
in the report.

## Email alerting through Healthchecks.io

The droplet's Exim is configured for local mail only, and DigitalOcean blocks
outbound SMTP ports 25, 465, and 587. The alert integration uses a
Healthchecks.io check over HTTPS. The live check has a one day period, a two
hour grace period, and email notifications to the operator. Email delivery from
the original check was confirmed on 2026-09-23. Treat its unique ping URL as a
secret. To replace it,
put the new URL in `~/cert-watch-stage/healthchecks-url` on the droplet with
mode `0600`; do not put it in this repository or a chat message.

The root owned `nrg-cert-maintenance install` command validates the URL,
copies it to `/etc/cert-watch/healthchecks-url` with mode `0600`, deletes the
staged copy, installs the ping script and systemd units, and sends a success
ping. Successful daily audits send a success ping; failed audits send `/fail`.
Missed runs become overdue after the provider's configured grace period.
The ping contains no report details; read the local JSON report to see why the
audit failed. On 2026-09-23, the exposed original check was replaced; the new
check received the audit failure, and the operator confirmed receipt of its
email alert. If the URL is exposed, create a new check, install its URL,
verify that it receives the audit result, then remove the exposed check.

The checker does not issue certificates for arbitrary services. That requires
the service owner's renewal mechanism and credentials. Public nginx hosts
using Certbot are renewed by the existing Certbot timer; internal self signed
certificates remain monitored for planned replacement.

## Operator checks

```sh
systemctl status certbot.timer cert-watch.timer cert-watch.service --no-pager
cat /var/lib/cert-watch/report.json
sudo certbot certificates
sudo certbot renew --dry-run
```

The root owned report directory should have mode `2750` and group `xper626`,
with the report at mode `0640` and the same group, for operator access.
Certbot's staging service returned `rateLimited: Service busy; retry later`
on the first NRG dry run on 2026-09-23, so a successful dry run remains
unverified.

## NRG DNS and certificate expansion

`lp.nouveauricheglobalgroup.com` and
`landing.nouveauricheglobalgroup.com` still need Namecheap hosting DNS A
records pointing to `139.59.135.253`. Once they resolve publicly, the scoped
`sudo /usr/local/sbin/nrg-cert-maintenance enroll` command expands the
`nrg-api-admin` Certbot lineage to all four NRG names, changes only the two
remaining Sectigo nginx hosts, tests nginx, reloads it, and checks the served
names. The command creates an nginx backup in `/var/backups/` and restores it
if nginx testing, reload, or local served certificate checks fail.

After enrollment, verify public SNI and the full chain for each NRG host,
then update the repository copy at `ops/nginx/conf.d/ssl.conf` to match the
live file. The existing Sectigo wildcard can be removed only after no
service references it.

The audit also currently flags `ussd.talentandtrivia.app`: nginx lists it in
`/etc/nginx/sites-enabled/talentrivia-shortcode`, but it has no DNS record and
the site's Certbot certificate does not cover it. The nginx file itself says
to add DNS and extend the certificate before using that name over HTTPS.
Decide whether that name is still needed before changing this separate
TalentTrivia virtual host.

## Files and rollback

The root owned installer is `/usr/local/sbin/nrg-cert-maintenance`; its
source is `install-on-droplet.sh`. It verifies fixed SHA-256 hashes of staged
files before installing them. Its sudo rule permits only `install` and
`enroll` arguments. To disable the audit without affecting renewal:

```sh
sudo systemctl disable --now cert-watch.timer
```

Do not disable `certbot.timer` while Certbot lineages are active.
