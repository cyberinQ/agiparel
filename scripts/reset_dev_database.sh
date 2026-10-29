#!/usr/bin/env bash
# Author: Sunil Jain, 2026-10-29.
# Explicitly rebuild ONLY the Agiparel dev DB from v42 canonical schema.
set -euo pipefail
[[ "${1:-}" == '--reset-agiparel-dev' && "${2:-}" == '--confirm-data-loss' && "$#" == 2 ]] || {
 echo 'Usage: bash scripts/reset_dev_database.sh --reset-agiparel-dev --confirm-data-loss'; exit 2;
}
[[ "$(git rev-parse --show-toplevel)" == "$(pwd -P)" ]] || { echo 'ERROR: wrong working directory'; exit 1; }
[[ -s services/core-api/migrations/0001_v42_baseline.sql ]] || { echo 'ERROR: baseline is missing'; exit 1; }
DC=(docker compose -f compose/docker-compose.dev.yml)
ACTUAL="$(docker inspect agiparel-postgres --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^POSTGRES_DB=//p' | head -1)"
[[ "$ACTUAL" == agiparel_dev ]] || { echo 'ERROR: only agiparel_dev may be reset'; exit 1; }
[[ "${POSTGRES_DB:-agiparel_dev}" == agiparel_dev ]] || { echo 'ERROR: DB override is unsafe'; exit 1; }
[[ -f .env ]] || { echo 'ERROR: .env credentials missing'; exit 1; }
APP_PASS="$(sed -n 's/^AGIPAREL_APP_DB_PASSWORD=//p' .env | tail -1)"
DOC_PASS="$(sed -n 's/^AGIPAREL_DOC_DB_PASSWORD=//p' .env | tail -1)"
[[ "$APP_PASS" =~ ^[a-f0-9]{48}$ && "$DOC_PASS" =~ ^[a-f0-9]{48}$ ]] || { echo 'ERROR: invalid dev credentials'; exit 1; }
export AGIPAREL_APP_DB_PASSWORD="$APP_PASS" AGIPAREL_DOC_DB_PASSWORD="$DOC_PASS"
umask 077; mkdir -p "$HOME/agiparel-db-backups"
BACKUP="$HOME/agiparel-db-backups/pre_v42_manual_reset_$(date +%Y%m%d_%H%M%S).dump"
"${DC[@]}" exec -T postgres sh -c 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$BACKUP"
[[ -s "$BACKUP" ]] || { echo 'ERROR: backup failed'; exit 1; }
"${DC[@]}" stop plant-edge-os document-intake
"${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "DROP DATABASE IF EXISTS agiparel_dev WITH (FORCE)"'
"${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE agiparel_dev OWNER agiparel_admin TEMPLATE template0"'
"${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d agiparel_dev -v app_password="$1" -v doc_password="$2"' sh "$APP_PASS" "$DOC_PASS" < scripts/bootstrap_roles_v42.sql
"${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d agiparel_dev' < services/core-api/migrations/0001_v42_baseline.sql > /dev/null
# Existing WAL files are not cleared: only use reset on a disposable dev environment.
echo "v42 dev database rebuilt; backup at $BACKUP. Review WAL continuity before restarting services."
