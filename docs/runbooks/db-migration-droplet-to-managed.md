# Production database migration: droplet PostgreSQL to DigitalOcean Managed PostgreSQL

## 1. Overview

This runbook migrates production database `subscription_manager` from PostgreSQL 15.19 on droplet `139.59.135.253` to the existing DigitalOcean Managed PostgreSQL 18.4 instance.

The source stays online during the trial restore.

The seven application containers stop for the final dump and remain stopped until restore and verification pass.

The source PostgreSQL server and database stay running and unchanged so rollback only requires restoring the previous application connection settings.

The unrelated `veritasquest` database shares the managed instance and must not be altered, dropped, restored, or used for migration checks.

```text
Before
======

  acquisition-api             \
  subscription-external        \
  notification-worker           \
  notification-service           +--> host.docker.internal:5432
  cadence-engine                /        PostgreSQL 15.19 on droplet
  subscription-partner         /         database: subscription_manager
  postback-dispatcher          /

  pgAdmin saved server entry ---------> droplet PostgreSQL


After
=====

  acquisition-api             \
  subscription-external        \
  notification-worker           \
  notification-service           +--> TLS required, port 25060
  cadence-engine                /        DigitalOcean Managed PostgreSQL 18.4
  subscription-partner         /         database: subscription_manager
  postback-dispatcher          /          role: sm_admin
       pool per service: max open 6, max idle 2

  pgAdmin saved server entry ---------> managed subscription_manager

  managed instance also contains veritasquest, which is out of scope
```

### Fixed endpoints and credential sources

| Purpose | Value or location |
| --- | --- |
| Droplet | `139.59.135.253`, SSH alias `do-sa` |
| Source PostgreSQL | `host.docker.internal:5432` from containers |
| Source database and role | `subscription_manager` as `sm_admin` |
| Managed PostgreSQL | `db-pgsql-fra1-10481-do-user-2716310-0.m.db.ondigitalocean.com:25060` |
| Managed TLS mode | `require` |
| Managed administrative role | `doadmin` |
| `doadmin` password source | Local workstation file `/home/xper626/workspace/vault/keys`, PostgreSQL block |
| `sm_admin` password source | Droplet file `/home/xper626/services/nouveauricheglobalgroup/.env` |
| Compose file | `/home/xper626/services/nouveauricheglobalgroup/docker-compose.yml` |

Never copy a password into this runbook, a command argument, shell history, a ticket, or a log.

Use only temporary `.pgpass` files with mode `0600`, or an unlogged stdin pipe, and delete temporary credential files immediately after use.

## 2. Preconditions checklist

Do not start Phase A until every checked fact below is still true.

### Source reconnaissance

- [ ] The source is PostgreSQL 15.19 (Debian) on port `5432`.
- [ ] Database `subscription_manager` has an observed size of `7237 MB`.
- [ ] Encoding is `UTF8` and collation is `C.UTF-8`.
- [ ] The only installed extension is `plpgsql`.
- [ ] The source contains 38 tables, 7 views, 20 sequences, 0 large objects, 0 replication slots, and 0 publications.
- [ ] Of the 38 tables, 36 are owned by `sm_admin` and two accident-backup tables are owned by `postgres`.
- [ ] `sm_admin` is the only application role and has no superuser flags.
- [ ] Normal load remains approximately six live connections.
- [ ] `pg_dump` 15.19 is available on the droplet.
- [ ] At least the observed 26 GB of free space remains available on `/` before creating a dump under `/tmp/dbmig`.
- [ ] No cron or systemd database consumers have been introduced.

### Consumer inventory

- [ ] The consumers remain `acquisition-api`, `subscription-external`, `notification-worker`, `notification-service`, `cadence-engine`, `subscription-partner`, and `postback-dispatcher`.
- [ ] Each consumer still obtains its host through `APP_DATABASE_POSTGRESQL_HOST=${PG_HOST:-host.docker.internal}` in the Compose file.
- [ ] `PG_USER`, `PG_PASSWORD`, `PG_DB`, `PG_SSL_MODE`, and `PG_PORT` still come from the adjacent `.env` file.
- [ ] Source defaults remain SSL mode `disable` and port `5432` when the corresponding values are unset.
- [ ] The separate pgAdmin Compose deployment is not an application writer.
- [ ] Any open pgAdmin query sessions will be closed before the final dump.

### Managed target

- [ ] The target endpoint is `db-pgsql-fra1-10481-do-user-2716310-0.m.db.ondigitalocean.com:25060` with `sslmode=require`.
- [ ] The target reports PostgreSQL 18.4 and `max_connections=50`.
- [ ] `doadmin` still has `rolcreatedb` and `rolcreaterole`, and is not a superuser.
- [ ] `sm_admin` exists with the same password as the source role.
- [ ] Membership `GRANT sm_admin TO doadmin` remains in place.
- [ ] `subscription_manager` exists with owner `sm_admin`, as created on 2026-08-15.
- [ ] The droplet can establish a TLS-required connection to the target.
- [ ] `veritasquest` is present and excluded from every destructive command.

### Cutover controls

- [ ] A maintenance window is open for stopping all seven application containers.
- [ ] The exact pre-cutover states of `PG_HOST`, `PG_PORT`, and `PG_SSL_MODE` are recorded without copying any credential.
- [ ] An operator is assigned to run the existing production health check for each service after restart.
- [ ] Rollback authority is available until all post-cutover verification checks pass.

## 3. Credential-safe working setup

Run database client commands from a droplet shell unless a step explicitly says to use the local workstation.

Start the droplet shell and create a private temporary directory.

```bash
ssh do-sa
install -d -m 700 /tmp/dbmig
cd /tmp/dbmig

export SOURCE_HOST=127.0.0.1
export SOURCE_PORT=5432
export TARGET_HOST=db-pgsql-fra1-10481-do-user-2716310-0.m.db.ondigitalocean.com
export TARGET_PORT=25060
export MIGRATION_DB=subscription_manager
export SOURCE_PGPASS=/tmp/dbmig/source.pgpass
export MANAGED_PGPASS=/tmp/dbmig/managed.pgpass

umask 077
```

Read the `sm_admin` password only from the droplet `.env` file.

Enter it through a silent prompt, escape `.pgpass` delimiters, create source and managed-role records, and clear the shell variables.

```bash
read -rsp "Enter sm_admin password from the droplet .env file: " SM_ADMIN_PASSWORD
printf '\n'

pgpass_escape() {
  local value=$1
  value=${value//\\/\\\\}
  value=${value//:/\\:}
  printf '%s' "$value"
}

SM_ADMIN_PGPASS_VALUE=$(pgpass_escape "$SM_ADMIN_PASSWORD")
printf '%s:%s:%s:%s:%s\n' \
  "$SOURCE_HOST" "$SOURCE_PORT" "$MIGRATION_DB" sm_admin "$SM_ADMIN_PGPASS_VALUE" \
  > "$SOURCE_PGPASS"
printf '%s:%s:%s:%s:%s\n' \
  "$TARGET_HOST" "$TARGET_PORT" "$MIGRATION_DB" sm_admin "$SM_ADMIN_PGPASS_VALUE" \
  > "$MANAGED_PGPASS"
chmod 600 "$SOURCE_PGPASS" "$MANAGED_PGPASS"
unset SM_ADMIN_PASSWORD SM_ADMIN_PGPASS_VALUE
```

In a second shell on the local workstation, read the `doadmin` password only from `/home/xper626/workspace/vault/keys`, PostgreSQL block.

Create a temporary `.pgpass` fragment locally, transfer it to the droplet, append it to the managed credential file, and delete the fragment at both ends.

No password appears in an argument or log in this flow.

```bash
umask 077
DOADMIN_FRAGMENT=$(mktemp)
read -rsp "Enter doadmin password from the local vault file: " DOADMIN_PASSWORD
printf '\n'

pgpass_escape() {
  local value=$1
  value=${value//\\/\\\\}
  value=${value//:/\\:}
  printf '%s' "$value"
}

DOADMIN_PGPASS_VALUE=$(pgpass_escape "$DOADMIN_PASSWORD")
printf '%s:%s:%s:%s:%s\n' \
  db-pgsql-fra1-10481-do-user-2716310-0.m.db.ondigitalocean.com \
  25060 postgres doadmin "$DOADMIN_PGPASS_VALUE" \
  > "$DOADMIN_FRAGMENT"
printf '%s:%s:%s:%s:%s\n' \
  db-pgsql-fra1-10481-do-user-2716310-0.m.db.ondigitalocean.com \
  25060 subscription_manager doadmin "$DOADMIN_PGPASS_VALUE" \
  >> "$DOADMIN_FRAGMENT"
chmod 600 "$DOADMIN_FRAGMENT"
unset DOADMIN_PASSWORD DOADMIN_PGPASS_VALUE

scp "$DOADMIN_FRAGMENT" do-sa:/tmp/dbmig/doadmin.pgpass
rm -f "$DOADMIN_FRAGMENT"
unset DOADMIN_FRAGMENT

ssh do-sa 'umask 077; cat /tmp/dbmig/doadmin.pgpass >> /tmp/dbmig/managed.pgpass; chmod 600 /tmp/dbmig/managed.pgpass; rm -f /tmp/dbmig/doadmin.pgpass'
```

Back in the droplet shell, confirm modes and client versions without printing file contents.

Both `pg_dump` and `pg_restore` must report 15.19 for the documented primary path.

```bash
stat -c '%a %n' "$SOURCE_PGPASS" "$MANAGED_PGPASS"
pg_dump --version
pg_restore --version
df -h /
```

Expected credential file mode is `600`.

## 4. Phase A: trial restore while production runs

Phase A measures dump and restore time, exercises the PostgreSQL 15 to 18 path, and provides a count spot check without stopping production.

Source row counts can change while this phase runs, so a trial count difference is not by itself evidence of restore loss.

Use the exact final cutover comparison in Phase B as the data-integrity gate.

### 4.1 Confirm target scope

Connect only to `subscription_manager` and inspect its user-object count.

Do not run any destructive statement against `veritasquest`.

```bash
PGSSLMODE=require PGPASSFILE="$MANAGED_PGPASS" psql -X \
  --host="$TARGET_HOST" \
  --port="$TARGET_PORT" \
  --username=doadmin \
  --dbname="$MIGRATION_DB" \
  --set=ON_ERROR_STOP=1 \
  --command="SELECT count(*) AS user_tables FROM pg_tables WHERE schemaname NOT IN ('pg_catalog', 'information_schema');"
```

If `subscription_manager` contains objects from an earlier trial, reset only that database.

```bash
PGSSLMODE=require PGPASSFILE="$MANAGED_PGPASS" psql -X \
  --host="$TARGET_HOST" \
  --port="$TARGET_PORT" \
  --username=doadmin \
  --dbname=postgres \
  --set=ON_ERROR_STOP=1 <<'SQL'
SET ROLE sm_admin;
DROP DATABASE subscription_manager WITH (FORCE);
RESET ROLE;
CREATE DATABASE subscription_manager OWNER sm_admin;
SQL
```

### 4.2 Create and restore the custom-format trial dump

Record wall-clock timing separately from command output.

```bash
rm -f /tmp/dbmig/trial.dump /tmp/dbmig/trial-dump.time /tmp/dbmig/trial-restore.time

/usr/bin/time -p -o /tmp/dbmig/trial-dump.time \
  env PGPASSFILE="$SOURCE_PGPASS" \
  pg_dump \
    --host="$SOURCE_HOST" \
    --port="$SOURCE_PORT" \
    --username=sm_admin \
    --dbname="$MIGRATION_DB" \
    --format=custom \
    --file=/tmp/dbmig/trial.dump

/usr/bin/time -p -o /tmp/dbmig/trial-restore.time \
  env PGPASSFILE="$MANAGED_PGPASS" \
  PGSSLMODE=require \
  pg_restore \
    --host="$TARGET_HOST" \
    --port="$TARGET_PORT" \
    --username=doadmin \
    --dbname="$MIGRATION_DB" \
    --jobs=2 \
    --no-owner \
    --role=sm_admin \
    --exit-on-error \
    /tmp/dbmig/trial.dump

ls -lh /tmp/dbmig/trial.dump
cat /tmp/dbmig/trial-dump.time
cat /tmp/dbmig/trial-restore.time
```

If `pg_restore` 15 reports an incompatibility with PostgreSQL 18, do not ignore the error or continue with a partial restore.

Reset only `subscription_manager` with the preceding reset command, then use this plain-format fallback.

The fallback pipes the dump to `psql` without putting a password in the pipeline or process arguments.

```bash
set -o pipefail
{
  printf 'SET ROLE sm_admin;\n'
  PGPASSFILE="$SOURCE_PGPASS" pg_dump \
    --host="$SOURCE_HOST" \
    --port="$SOURCE_PORT" \
    --username=sm_admin \
    --dbname="$MIGRATION_DB" \
    --format=plain \
    --no-owner
} | PGSSLMODE=require PGPASSFILE="$MANAGED_PGPASS" psql -X \
      --host="$TARGET_HOST" \
      --port="$TARGET_PORT" \
      --username=doadmin \
      --dbname="$MIGRATION_DB" \
      --set=ON_ERROR_STOP=1
```

Record which restore path was used and its elapsed time.

Load the Appendix A helpers and capture sorted source and target counts.

Review at least three corresponding table rows and record any difference as expected live-write drift or as an investigated anomaly.

```bash
capture_table_counts "$SOURCE_PGPASS" "$SOURCE_HOST" "$SOURCE_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/trial-source-table-counts.tsv
PGSSLMODE=require capture_table_counts "$MANAGED_PGPASS" "$TARGET_HOST" "$TARGET_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/trial-target-table-counts.tsv

wc -l /tmp/dbmig/trial-source-table-counts.tsv /tmp/dbmig/trial-target-table-counts.tsv
paste \
  <(sed -n '1p;19p;38p' /tmp/dbmig/trial-source-table-counts.tsv) \
  <(sed -n '1p;19p;38p' /tmp/dbmig/trial-target-table-counts.tsv)
```

The trial passes only when restore exits zero, target inventory contains 38 tables, 7 views, and 20 sequences, and sampled count differences are explained or investigated.

If `sm_admin` cannot read either postgres-owned accident-backup table, stop instead of excluding it from the dump or count checks.

No verified privileged source credential path is documented here, so resolving that permission failure requires an approved source access method before cutover.

## 5. Phase B: production cutover

### 5.1 Stop every application database consumer

Run on the droplet from the Compose project directory.

```bash
cd /home/xper626/services/nouveauricheglobalgroup

docker compose stop \
  acquisition-api \
  subscription-external \
  notification-worker \
  notification-service \
  cadence-engine \
  subscription-partner \
  postback-dispatcher

docker compose ps --all \
  acquisition-api \
  subscription-external \
  notification-worker \
  notification-service \
  cadence-engine \
  subscription-partner \
  postback-dispatcher
```

All seven containers must show a stopped state before the final dump begins.

Close any active pgAdmin query session against the source database.

Use Appendix C and confirm there is no remaining application activity against `subscription_manager`.

The source PostgreSQL server itself stays running.

### 5.2 Capture the final source state and dump

Return to `/tmp/dbmig` in the prepared droplet shell and load the Appendix A helpers if this is a new shell.

```bash
cd /tmp/dbmig

capture_table_counts "$SOURCE_PGPASS" "$SOURCE_HOST" "$SOURCE_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/final-source-table-counts.tsv
capture_sequence_values "$SOURCE_PGPASS" "$SOURCE_HOST" "$SOURCE_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/final-source-sequences.tsv

test "$(wc -l < /tmp/dbmig/final-source-table-counts.tsv)" -eq 38
test "$(wc -l < /tmp/dbmig/final-source-sequences.tsv)" -eq 20

rm -f /tmp/dbmig/final.dump /tmp/dbmig/final-dump.time /tmp/dbmig/final-restore.time

/usr/bin/time -p -o /tmp/dbmig/final-dump.time \
  env PGPASSFILE="$SOURCE_PGPASS" \
  pg_dump \
    --host="$SOURCE_HOST" \
    --port="$SOURCE_PORT" \
    --username=sm_admin \
    --dbname="$MIGRATION_DB" \
    --format=custom \
    --file=/tmp/dbmig/final.dump

ls -lh /tmp/dbmig/final.dump
cat /tmp/dbmig/final-dump.time
```

If an assertion or dump fails, keep applications stopped and resolve the failure or execute rollback.

### 5.3 Drop and recreate only the target migration database

This step erases the trial restore from `subscription_manager`.

It must not reference `veritasquest` in any statement.

```bash
PGSSLMODE=require PGPASSFILE="$MANAGED_PGPASS" psql -X \
  --host="$TARGET_HOST" \
  --port="$TARGET_PORT" \
  --username=doadmin \
  --dbname=postgres \
  --set=ON_ERROR_STOP=1 <<'SQL'
SET ROLE sm_admin;
DROP DATABASE subscription_manager WITH (FORCE);
RESET ROLE;
CREATE DATABASE subscription_manager OWNER sm_admin;
SQL
```

### 5.4 Restore the final dump

Use the restore path that passed Phase A.

For the primary custom-format path, run:

```bash
/usr/bin/time -p -o /tmp/dbmig/final-restore.time \
  env PGPASSFILE="$MANAGED_PGPASS" \
  PGSSLMODE=require \
  pg_restore \
    --host="$TARGET_HOST" \
    --port="$TARGET_PORT" \
    --username=doadmin \
    --dbname="$MIGRATION_DB" \
    --jobs=2 \
    --no-owner \
    --role=sm_admin \
    --exit-on-error \
    /tmp/dbmig/final.dump

cat /tmp/dbmig/final-restore.time
```

If Phase A required the plain-format fallback, repeat that tested fallback against the freshly recreated database instead of retrying the incompatible path.

After either restore path, capture the source again and prove it did not change between the pre-dump snapshot and completion of the restore.

This stability gate also covers the fresh source stream used by the plain-format fallback.

```bash
capture_table_counts "$SOURCE_PGPASS" "$SOURCE_HOST" "$SOURCE_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/final-source-postrestore-table-counts.tsv
capture_sequence_values "$SOURCE_PGPASS" "$SOURCE_HOST" "$SOURCE_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/final-source-postrestore-sequences.tsv

diff -u \
  /tmp/dbmig/final-source-table-counts.tsv \
  /tmp/dbmig/final-source-postrestore-table-counts.tsv
diff -u \
  /tmp/dbmig/final-source-sequences.tsv \
  /tmp/dbmig/final-source-postrestore-sequences.tsv
```

Both stability comparisons must exit zero before target data verification begins.

If either comparison differs, do not start applications because the target is not proven to match the final source state.

### 5.5 Require exact data and object verification

Capture target state with the same helpers and require exact file comparisons.

```bash
PGSSLMODE=require capture_table_counts "$MANAGED_PGPASS" "$TARGET_HOST" "$TARGET_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/final-target-table-counts.tsv
PGSSLMODE=require capture_sequence_values "$MANAGED_PGPASS" "$TARGET_HOST" "$TARGET_PORT" sm_admin "$MIGRATION_DB" \
  /tmp/dbmig/final-target-sequences.tsv

test "$(wc -l < /tmp/dbmig/final-target-table-counts.tsv)" -eq 38
test "$(wc -l < /tmp/dbmig/final-target-sequences.tsv)" -eq 20

diff -u \
  /tmp/dbmig/final-source-table-counts.tsv \
  /tmp/dbmig/final-target-table-counts.tsv
diff -u \
  /tmp/dbmig/final-source-sequences.tsv \
  /tmp/dbmig/final-target-sequences.tsv
```

Both `diff` commands must exit zero and produce no differences.

Run Appendix B and require 38 tables, 7 views, 20 sequences, and 0 large objects.

Confirm that only `plpgsql` is installed.

Confirm all restored user objects are owned by `sm_admin`.

The two source accident-backup tables were owned by `postgres`, but target ownership is intentionally normalized to `sm_admin` by `--no-owner --role=sm_admin`.

### 5.6 Change endpoint and pool configuration

Edit the droplet `.env` file.

Set only these endpoint values and leave `PG_USER`, `PG_PASSWORD`, and `PG_DB` unchanged.

```bash
PG_HOST=db-pgsql-fra1-10481-do-user-2716310-0.m.db.ondigitalocean.com
PG_PORT=25060
PG_SSL_MODE=require
```

Each Go service otherwise defaults to `MaxOpenConns=50` in `common/postgres/database.go`.

Seven services at that default can overwhelm the managed `max_connections=50` limit.

Add both variables to the `environment` block of each of the seven services in the Compose file.

The variables are read directly through `os.Getenv`, so they must be present in every service container environment.

```yaml
environment:
  PGMAX_OPEN_CONNS: "6"
  PGMAX_IDLE_CONNS: "2"
```

The aggregate configured application maximum is 42 open connections before `veritasquest`, pgAdmin, administrators, or other clients are counted.

The managed cap applies across the instance, not only to `subscription_manager`.

Validate syntax without rendering resolved environment values.

```bash
cd /home/xper626/services/nouveauricheglobalgroup
docker compose config -q
```

Do not place resolved Compose output in a log because Compose consumes credentials from `.env`.

### 5.7 Start services and verify health

```bash
cd /home/xper626/services/nouveauricheglobalgroup
docker compose up -d

for service in \
  acquisition-api \
  subscription-external \
  notification-worker \
  notification-service \
  cadence-engine \
  subscription-partner \
  postback-dispatcher
do
  container_id=$(docker compose ps -q "$service")
  test -n "$container_id"
  docker inspect \
    --format '{{.Name}} status={{.State.Status}}{{if .State.Health}} health={{.State.Health.Status}}{{else}} health=not-configured{{end}}' \
    "$container_id"
done
```

Require container state `running` for every service.

Where a Docker health check exists, require state `healthy`.

For a service without a Docker health check, run its established production health verification and record the result.

No service-specific health endpoint is asserted because none is part of the verified migration facts.

Do not invent an endpoint or treat container state alone as application-health proof.

Use Appendix C to confirm application connections now land on managed `subscription_manager`.

Run the instance-wide cap query and record the result.

```sql
SELECT
  current_setting('max_connections')::integer AS max_connections,
  count(*) AS current_client_connections,
  current_setting('max_connections')::integer - count(*) AS nominal_headroom,
  count(*) FILTER (WHERE datname = 'subscription_manager') AS subscription_manager_connections,
  count(*) FILTER (WHERE datname = 'veritasquest') AS veritasquest_connections
FROM pg_stat_activity
WHERE backend_type = 'client backend';
```

Do not accept cutover while the total is at or near the cap or while any service health check is failing.

## 6. Rollback

The source PostgreSQL server and source `subscription_manager` remain available because they were left running and untouched.

A configuration-only rollback is data-safe only while no production write has committed to the managed database.

Once managed production writes exist, redirecting applications to the unchanged source would omit those target-only writes.

No reverse synchronization or target-to-source reconciliation procedure is part of the verified facts for this runbook.

If any managed write may have committed, stop all seven containers and do not redirect them until an approved reconciliation procedure preserves those writes.

Stop all seven application containers before redirecting them to avoid concurrent writes to both databases.

Restore the exact pre-cutover `PG_HOST`, `PG_PORT`, and `PG_SSL_MODE` state in the droplet `.env` file.

The effective source values are `host.docker.internal`, port `5432`, and SSL mode `disable`, whether explicit or supplied by the documented defaults.

Do not change `PG_USER`, `PG_PASSWORD`, or `PG_DB`.

The lower `PGMAX_OPEN_CONNS` and `PGMAX_IDLE_CONNS` values can remain during emergency rollback.

```bash
cd /home/xper626/services/nouveauricheglobalgroup

docker compose stop \
  acquisition-api \
  subscription-external \
  notification-worker \
  notification-service \
  cadence-engine \
  subscription-partner \
  postback-dispatcher

# Restore the recorded pre-cutover PG_HOST, PG_PORT, and PG_SSL_MODE state in .env.
docker compose config -q
docker compose up -d
```

Repeat service-by-service health verification and verify application sessions have returned to the source.

Do not drop the managed copy during rollback because it preserves cutover evidence for investigation.

## 7. Post-migration actions

- [ ] Manually repoint the pgAdmin UI-saved entry for `subscription_manager` to the managed host, port `25060`, and SSL mode `require`.
- [ ] Use the `sm_admin` credential from the droplet `.env` when pgAdmin requests it, without copying it into notes or logs.
- [ ] Do not change `veritasquest` or its saved server configuration.
- [ ] Monitor instance-wide client connections against the cap of 50, including `veritasquest` and administrative tools.
- [ ] Confirm all seven services retain `PGMAX_OPEN_CONNS=6` and `PGMAX_IDLE_CONNS=2` after later Compose deployments.
- [ ] Record whether locale-sensitive text ordering differs from the source.
- [ ] Move the automated-backup expectation from droplet PostgreSQL backups to DigitalOcean managed backups.
- [ ] Confirm managed backups meet the production recovery requirement before scheduling source decommission.
- [ ] Schedule droplet PostgreSQL decommission only after an approved soak period completes without rollback criteria.
- [ ] Delete temporary credential files immediately after the migration decision is final.

```bash
rm -f \
  /tmp/dbmig/source.pgpass \
  /tmp/dbmig/managed.pgpass \
  /tmp/dbmig/doadmin.pgpass
unset SOURCE_PGPASS MANAGED_PGPASS
```

The source used collation `C.UTF-8`, while the managed database was created with managed defaults.

All indexes are rebuilt from data during restore, so the collation difference creates no index-corruption risk.

Locale-sensitive text comparisons can produce a different `ORDER BY` result and must be observed during the soak period.

## Appendix A: exact table-count and sequence capture helpers

Define these functions in the droplet shell before capture commands.

They write sorted, tab-separated evidence without printing credentials.

```bash
capture_table_counts() {
  local pgpass_file=$1
  local host=$2
  local port=$3
  local user=$4
  local database=$5
  local output=$6

  PGPASSFILE="$pgpass_file" psql -X -qAt -F $'\t' \
    --host="$host" \
    --port="$port" \
    --username="$user" \
    --dbname="$database" \
    --set=ON_ERROR_STOP=1 \
    > "$output" <<'SQL'
CREATE TEMP TABLE migration_table_counts (
  schema_name text NOT NULL,
  table_name text NOT NULL,
  row_count bigint NOT NULL
);

DO $$
DECLARE
  table_record record;
  exact_count bigint;
BEGIN
  FOR table_record IN
    SELECT schemaname, tablename
    FROM pg_tables
    WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
    ORDER BY schemaname, tablename
  LOOP
    EXECUTE format(
      'SELECT count(*) FROM %I.%I',
      table_record.schemaname,
      table_record.tablename
    ) INTO exact_count;

    INSERT INTO migration_table_counts (schema_name, table_name, row_count)
    VALUES (table_record.schemaname, table_record.tablename, exact_count);
  END LOOP;
END
$$;

SELECT schema_name, table_name, row_count
FROM migration_table_counts
ORDER BY schema_name, table_name;
SQL
}

capture_sequence_values() {
  local pgpass_file=$1
  local host=$2
  local port=$3
  local user=$4
  local database=$5
  local output=$6

  PGPASSFILE="$pgpass_file" psql -X -qAt -F $'\t' \
    --host="$host" \
    --port="$port" \
    --username="$user" \
    --dbname="$database" \
    --set=ON_ERROR_STOP=1 \
    --command="SELECT schemaname, sequencename, last_value FROM pg_sequences WHERE schemaname NOT IN ('pg_catalog', 'information_schema') ORDER BY schemaname, sequencename;" \
    > "$output"
}
```

## Appendix B: object and ownership verification queries

Run these against target `subscription_manager` with TLS required through the connection parameters.

```sql
SELECT
  (SELECT count(*) FROM pg_tables
   WHERE schemaname NOT IN ('pg_catalog', 'information_schema')) AS tables,
  (SELECT count(*) FROM pg_views
   WHERE schemaname NOT IN ('pg_catalog', 'information_schema')) AS views,
  (SELECT count(*) FROM pg_sequences
   WHERE schemaname NOT IN ('pg_catalog', 'information_schema')) AS sequences,
  (SELECT count(*) FROM pg_largeobject_metadata) AS large_objects;

SELECT extname
FROM pg_extension
ORDER BY extname;

SELECT tableowner, count(*) AS table_count
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
GROUP BY tableowner
ORDER BY tableowner;

SELECT
  n.nspname AS schema_name,
  c.relname AS object_name,
  c.relkind AS object_kind,
  pg_get_userbyid(c.relowner) AS owner
FROM pg_class AS c
JOIN pg_namespace AS n ON n.oid = c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND c.relkind IN ('r', 'p', 'S', 'v', 'm')
  AND pg_get_userbyid(c.relowner) <> 'sm_admin'
ORDER BY n.nspname, c.relname;
```

The final ownership query must return no rows.

## Appendix C: connection verification queries

Run the source version before the final dump and the managed version after application restart.

`application_name` is reported as `<unset>` when a client does not supply one.

If names are unset, correlate client address, user, state, and established health evidence without assigning an invented service identity.

```sql
SELECT
  datname,
  usename,
  COALESCE(NULLIF(application_name, ''), '<unset>') AS application_name,
  client_addr,
  state,
  count(*) AS connections
FROM pg_stat_activity
WHERE backend_type = 'client backend'
GROUP BY datname, usename, application_name, client_addr, state
ORDER BY datname, usename, application_name, client_addr, state;

SELECT
  COALESCE(NULLIF(application_name, ''), '<unset>') AS application_name,
  client_addr,
  state,
  count(*) AS connections
FROM pg_stat_activity
WHERE backend_type = 'client backend'
  AND datname = 'subscription_manager'
GROUP BY application_name, client_addr, state
ORDER BY application_name, client_addr, state;

SELECT
  current_setting('max_connections')::integer AS max_connections,
  count(*) AS current_client_connections,
  current_setting('max_connections')::integer - count(*) AS nominal_headroom,
  count(*) FILTER (WHERE datname = 'subscription_manager') AS subscription_manager_connections,
  count(*) FILTER (WHERE datname = 'veritasquest') AS veritasquest_connections
FROM pg_stat_activity
WHERE backend_type = 'client backend';
```
