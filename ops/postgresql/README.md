# Host PostgreSQL trial retirement

The droplet's PostgreSQL 15 cluster is separate from the DigitalOcean managed
database used by the running TIMWE application containers. Do not remove its
data directory. A stop is reversible with `nrg-pg-retire start`.

## Scope

`nrg-pg-retire` is a root-owned operator helper with five exact sudo actions:

- `audit`: report database names, sizes, cumulative session counts, current
  client count, and backup space. It does not read table contents.
- `backup`: create a root-only compressed `pg_dumpall` under
  `/var/backups/nrg-pg-retire/`, then verify the gzip stream and hash it.
- `stop`: require a backup no older than 24 hours and zero active client
  sessions, schedule an automatic restart after 15 minutes, then stop only
  `postgresql@15-main.service`.
- `confirm`: cancel the automatic restart after the application checks pass.
- `start`: restart the host cluster and check port 5432, including emergency
  rollback during the 15 minute trial.

The helper never drops databases, deletes the data directory, or disables the
systemd unit. A compressed dump is a recovery aid, not proof of a tested
restore. Database and connection history must be reviewed before calling the
cluster unused.

The cluster's `start.conf` still says `auto`; it may start again after a host
reboot. A durable retirement requires a separately reviewed startup-policy
change, and public port 5432 must remain blocked if it is ever restarted.

## Checks before confirming the stop

1. Review `audit` output and confirm any non-template databases are historical.
2. Confirm all seven TIMWE containers still target managed PostgreSQL and are
   healthy; check public API and admin health before and after the trial stop.
3. Review other applications on the shared droplet and scheduled jobs.
4. Keep the host cluster available for immediate rollback until the trial is
   confirmed.

The Compose deployment now requires explicit `PG_HOST` and `PG_PORT`. Its
rendered live configuration was unchanged after that edit.
