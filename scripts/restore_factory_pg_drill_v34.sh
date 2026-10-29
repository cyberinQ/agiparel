#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Restore a custom archive ONLY into a new, explicitly named scratch database.
set -euo pipefail
if [[ "$#" -ne 4 || "${1:-}" != '--backup' || "${3:-}" != '--scratch-db' ]]; then
  echo 'Usage: PGHOST=... PGPORT=... PGUSER=... PGDATABASE=... bash scripts/restore_factory_pg_drill_v34.sh --backup /secure/backup.dump --scratch-db agiparel_restore_drill_YYYYMMDD' >&2
  exit 2
fi
for v in PGHOST PGPORT PGUSER PGDATABASE; do
  if [[ -z "${!v:-}" ]]; then echo "ERROR: $v must be configured" >&2; exit 2; fi
done
archive="$2"; scratch="$4"
[[ "$scratch" =~ ^agiparel_restore_drill_[A-Za-z0-9_]{4,32}$ ]] || { echo 'ERROR: scratch name must be agiparel_restore_drill_*' >&2; exit 2; }
[[ "$scratch" != "$PGDATABASE" && -s "$archive" ]] || { echo 'ERROR: missing archive or unsafe database' >&2; exit 2; }
for tool in createdb pg_restore psql; do command -v "$tool" >/dev/null || { echo "ERROR: $tool unavailable" >&2; exit 2; }; done
pg_restore --list "$archive" >/dev/null
actual="$(psql -X -A -t -v ON_ERROR_STOP=1 -c 'SELECT current_database()')"
[[ "$actual" == "$PGDATABASE" ]] || { echo 'ERROR: source database identity mismatch' >&2; exit 1; }
exists="$(psql -X -A -t -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM pg_database WHERE datname='$scratch'")"
[[ "$exists" == 0 ]] || { echo 'ERROR: scratch database already exists; refusing overwrite' >&2; exit 1; }
createdb --template=template0 "$scratch"
# A failed restore deliberately leaves the scratch database for investigation.
pg_restore --exit-on-error --dbname "$scratch" "$archive"
PGDATABASE="$scratch" psql -X -v ON_ERROR_STOP=1 -c "SELECT count(*) AS restored_tenants FROM tenants"
PGDATABASE="$scratch" psql -X -v ON_ERROR_STOP=1 -c "SELECT count(*) AS restored_dock_scans FROM dock_carton_scans_v34"
echo "RESTORE DRILL COMPLETE: $scratch. Verify application behavior and audit logs before asserting RTO/RPO."
echo 'Scratch database was intentionally retained for inspection; delete it separately after approval.'
