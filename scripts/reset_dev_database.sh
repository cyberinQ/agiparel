#!/usr/bin/env bash
# Agiparel *development-only* database reset, without Docker volume removal.
# Destructively recreates ONLY database agiparel_dev inside agiparel-postgres.
# Does NOT reset any other databases, Redis, models, containers or applications.
set -euo pipefail
[[ "${1:-}" == '--reset-agiparel-dev' && "$#" == 1 ]] || {
  echo 'Usage: bash scripts/reset_dev_database.sh --reset-agiparel-dev';
  echo 'DESTROYS all data in PostgreSQL database agiparel_dev; rebuilds schema 0001-0025.';
  exit 2;
}
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[[ -n "$ROOT" && "$(pwd -P)" == "$(cd "$ROOT" && pwd -P)" ]] || {
  echo 'ERROR: run from the Agiparel repository root'; exit 1;
}
[[ -f compose/docker-compose.dev.yml && -f services/core-api/migrations/0025_factory_intelligence_snapshots_v12.sql ]] || {
  echo 'ERROR: wrong repo or incomplete migrations'; exit 1;
}
command -v docker >/dev/null || { echo 'ERROR: docker unavailable'; exit 1; }
# Enforce expected dev database identity; never interpolate a user-supplied DB.
ACTUAL_DB="$(docker inspect agiparel-postgres --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^POSTGRES_DB=//p' | head -1)"
[[ "$ACTUAL_DB" == 'agiparel_dev' ]] || {
  echo "ERROR: agiparel-postgres reports POSTGRES_DB=$ACTUAL_DB; refusing to reset"; exit 1;
}
# If an override is configured for Compose, reject rather than change targets.
if [[ -n "${POSTGRES_DB:-}" && "$POSTGRES_DB" != 'agiparel_dev' ]]; then
  echo 'ERROR: current POSTGRES_DB differs from agiparel_dev'; exit 1
fi
if [[ -f .env ]] && grep -Eq '^POSTGRES_DB=' .env; then
  ENV_DB="$(sed -n 's/^POSTGRES_DB=//p' .env | tail -1 | tr -d '\r\"\047')"
  [[ "$ENV_DB" == 'agiparel_dev' ]] || { echo 'ERROR: .env POSTGRES_DB mismatch'; exit 1; }
fi

PSQL=(docker exec -i agiparel-postgres psql -X -U agiparel_admin -v ON_ERROR_STOP=1)
echo 'Stopping ONLY Agiparel application processes (PostgreSQL container remains running).'
docker compose -f compose/docker-compose.dev.yml stop plant-edge-os document-intake

echo 'Dropping and recreating ONLY agiparel_dev; no pgdata volume is removed.'
"${PSQL[@]}" -d postgres -c 'DROP DATABASE IF EXISTS agiparel_dev WITH (FORCE);'
"${PSQL[@]}" -d postgres -c 'CREATE DATABASE agiparel_dev OWNER agiparel_admin TEMPLATE template0;'

echo 'Reapplying ALL migrations 0001 through 0025 in numerical order.'
count=0
for f in services/core-api/migrations/[0-9][0-9][0-9][0-9]_*.sql; do
  [[ -f "$f" ]] || { echo 'ERROR: missing migrations'; exit 1; }
  v="$(basename "$f" | cut -c1-4)"
  expected="$(printf '%04d' "$((count+1))")"
  [[ "$v" == "$expected" ]] || {
    echo "ERROR: migration sequence gap: expected $expected but found $f"; exit 1;
  }
  echo "  [$v] $f"
  "${PSQL[@]}" -d agiparel_dev < "$f" >/dev/null || {
    echo "ERROR: migration $f failed. Services remain stopped. Fix and repeat reset."; exit 1;
  }
  count=$((count+1))
done
expected=$(ls services/core-api/migrations/0*.sql | wc -l | tr -d ' ')
[[ "$count" -eq "$expected" ]] || { echo "ERROR: expected $expected migrations, got $count"; exit 1; }

# A clean database has zero application rows. Raise an error if migrations seed data.
"${PSQL[@]}" -d agiparel_dev <<'SQL'
DO $$
DECLARE r record; n bigint;
BEGIN
  FOR r IN SELECT tablename FROM pg_tables WHERE schemaname='public' LOOP
    EXECUTE format('SELECT count(*) FROM public.%I', r.tablename) INTO n;
    IF n <> 0 THEN RAISE EXCEPTION 'Unexpected seeded rows in %: %', r.tablename, n; END IF;
  END LOOP;
END $$;
SQL

echo 'Empty Agiparel schema verified. Starting only the Agiparel services.'
docker compose -f compose/docker-compose.dev.yml up -d --no-deps plant-edge-os document-intake
echo 'RESET COMPLETE: database agiparel_dev has migrations 0001-0025 and zero app rows.'
