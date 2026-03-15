#!/bin/sh
set -e

if [ -f /home/vscode/.env ]; then
	set -a
	. /home/vscode/.env
	set +a
fi

PGHOST="${DB_HOST:-localhost}"
PGPORT="${DB_PORT:-15432}"
PGDB="${DB_NAME:-app}"
PGUSER="${DB_USER:-app}"

export PGPASSWORD="$DB_PASS"

psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "SELECT pg_advisory_lock(hashtext('homelab-migrations'))"

# run migra or psql apply here
migra --from "postgresql://$PGUSER:$PGPASSWORD@${PGHOST}:${PGPORT}/${PGDB}" --to "postgresql://...

# release
psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "SELECT pg_advisory_unlock(hashtext('homelab-migrations'))"
