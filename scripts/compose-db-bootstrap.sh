#!/bin/sh
set -eu

workspace="${COMPOSE_DB_BOOTSTRAP_WORKSPACE:-/workspace}"

required_env="PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE"
for key in $required_env; do
    eval "value=\${$key:-}"
    if [ -z "$value" ]; then
        echo "compose-db-bootstrap: missing required environment variable $key" >&2
        exit 2
    fi
done

echo "compose-db-bootstrap: waiting for PostgreSQL at $PGHOST:$PGPORT/$PGDATABASE"
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE"; do
    sleep 1
done

apply_sql() {
    file="$workspace/$1"
    if [ ! -f "$file" ]; then
        echo "compose-db-bootstrap: missing SQL file $file" >&2
        exit 3
    fi
    echo "compose-db-bootstrap: applying $1"
    psql -v ON_ERROR_STOP=1 -f "$file"
}

apply_sql "ops/db/bootstrap/001_runtime_base.sql"
apply_sql "services/acquisition-api/migrations/add_admin_management_tables.sql"
apply_sql "services/acquisition-api/migrations/add_tenant_channels.sql"
apply_sql "services/acquisition-api/migrations/add_tenant_channel_credentials.sql"
apply_sql "services/acquisition-api/migrations/add_tenant_z_campaign_binding.sql"
apply_sql "services/acquisition-api/migrations/add_tenant_zz_acquisition_flow.sql"
apply_sql "services/acquisition-api/migrations/create_postback_tables.sql"
apply_sql "services/acquisition-api/migrations/add_tenant_postback_routing.sql"
apply_sql "services/subscription-external/migrations/011_message_cadence_engine.sql"
apply_sql "services/subscription-external/migrations/017_tenant_notification_cadence_routing.sql"

echo "compose-db-bootstrap: complete"
