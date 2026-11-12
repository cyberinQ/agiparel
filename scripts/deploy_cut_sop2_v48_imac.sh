#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-12.
# Rebuild the existing iMac development-backed Docker origin in place, with PG backup and rollback metadata.
# Does not touch local MacBook state, public proxy, database volumes, or signed release history.
set -Eeuo pipefail
umask 077
cd "$(dirname "$0")/.."
MODE="${1:---check}"
[[ "$MODE" == --check || "$MODE" == --deploy ]] || { echo 'Usage: bash scripts/deploy_cut_sop2_v48_imac.sh [--check|--deploy]' >&2; exit 2; }
[[ -d .git && -f .env ]] || { echo 'STOP: checkout + existing .env required; no fresh database initialization' >&2; exit 1; }
[[ "$(git branch --show-current)" == main ]] || { echo 'STOP: expected main checkout' >&2; exit 1; }
[[ -z "$(git status --porcelain --untracked-files=no)" ]] || { echo 'STOP: tracked source files modified on this iMac. Do not overwrite.' >&2; exit 1; }
[[ "$(git rev-parse HEAD)" == "$(git rev-parse refs/remotes/origin/main)" ]] || { echo 'STOP: fetched origin/main and HEAD mismatch' >&2; exit 1; }
[[ -z "$(git ls-files -- .env)" ]] || { echo 'STOP: production credentials tracked in Git' >&2; exit 1; }
command -v docker >/dev/null && command -v curl >/dev/null || { echo 'Docker and curl are required' >&2; exit 1; }
COMPOSE=(docker compose --env-file .env -f compose/docker-compose.dev.yml)
"${COMPOSE[@]}" config --quiet
if grep -Eq '^ENVIRONMENT=production([[:space:]]|$)' .env; then
  echo 'STOP: existing stack declares production; do not deploy the development compose file.' >&2; exit 1
fi
EDGE_CID="$("${COMPOSE[@]}" ps -q plant-edge-os)"
PG_CID="$("${COMPOSE[@]}" ps -q postgres)"
[[ -n "$EDGE_CID" && -n "$PG_CID" ]] || {
  echo 'STOP: expected existing edge + Postgres containers not found in dev compose. Identify the actual iMac stack before deploying.' >&2; exit 1;
}
EDGE_TAG="$(docker inspect --format '{{.Config.Image}}' "$EDGE_CID")"
EDGE_ID="$(docker inspect --format '{{.Image}}' "$EDGE_CID")"
DOC_CID="$("${COMPOSE[@]}" ps -q document-intake)"
DOC_TAG=''; DOC_ID=''
if [[ -n "$DOC_CID" ]]; then
  DOC_TAG="$(docker inspect --format '{{.Config.Image}}' "$DOC_CID")"
  DOC_ID="$(docker inspect --format '{{.Image}}' "$DOC_CID")"
fi
printf 'Source commit: %s\nExisting app image: %s\nExisting DB container: %s\n' "$(git rev-parse --short HEAD)" "$EDGE_ID" "$PG_CID"
if command -v node >/dev/null; then
  node scripts/test_cutter_mode_v47.cjs >/dev/null
  node scripts/test_public_cutter_v48.cjs
fi
if command -v go >/dev/null; then
  bash scripts/run_cut_sop2_v46_checks.sh --test
fi
if [[ "$MODE" == --check ]]; then
  echo 'PASS: iMac checkout, existing Docker stack, release source and safety checks.'
  echo 'Set AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED=true in the iMac .env to show /demo/cutting publicly.'
  echo 'To deploy: AGIPAREL_CONFIRM_IMAC_DEPLOY=YES bash scripts/deploy_cut_sop2_v48_imac.sh --deploy'
  exit 0
fi
[[ "${AGIPAREL_CONFIRM_IMAC_DEPLOY:-}" == YES ]] || { echo 'STOP: set AGIPAREL_CONFIRM_IMAC_DEPLOY=YES to authorize in-place rebuild' >&2; exit 1; }

# No service restarted until both images are compiled and database snapshot verified.
echo '=== Build images without restarting services ==='
"${COMPOSE[@]}" build plant-edge-os document-intake
BACKUP_ROOT="$HOME/agiparel-db-backups"
mkdir -p "$BACKUP_ROOT"
STAMP="$(date +%Y%m%d_%H%M%S)"
BACKUP="$BACKUP_ROOT/pre_cut_sop2_v48_${STAMP}_$$.dump"
echo '=== Snapshot existing PostgreSQL, then verify archive integrity ==='
"${COMPOSE[@]}" exec -T postgres sh -c 'exec pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$BACKUP"
[[ -s "$BACKUP" ]] || { echo 'STOP: database dump empty' >&2; exit 1; }
"${COMPOSE[@]}" exec -T postgres pg_restore --list < "$BACKUP" > /dev/null
printf 'Verified DB backup: %s\n' "$BACKUP"

# The v46 CREATE POLICY is not idempotent. Apply only to a proven zero-table baseline.
COUNT="$("${COMPOSE[@]}" exec -T postgres sh -c 'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='\''public'\'' AND c.relname LIKE '\''cut_sop2_%_v46'\'' AND c.relkind='\''r'\''"' | tr -d '[:space:]')"
[[ "$COUNT" =~ ^[0-9]+$ ]] || { echo 'STOP: cannot determine SOP-2 schema state' >&2; exit 1; }
if [[ "$COUNT" == 0 ]]; then
  echo '=== Apply additive SOP-2 v46 schema to EXISTING DB ==='
  "${COMPOSE[@]}" exec -T postgres sh -c 'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < services/core-api/migrations/0005_cut_sop2_control_plane_v46.sql
elif [[ "$COUNT" == 10 ]]; then
  echo 'SOP-2 tables already present; skipping the non-idempotent migration.'
else
  echo "STOP: partial SOP-2 schema ($COUNT / 10). No writes or service restart performed." >&2
  exit 1
fi
"${COMPOSE[@]}" exec -T postgres sh -c 'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < scripts/test_cut_sop2_v46_schema_pg.sql

cat > "$BACKUP_ROOT/pre_cut_sop2_v48_${STAMP}_rollback.txt" <<EOF
Author: Sunil Jain, 2026-11-12
AGIPAREL source commit: $(git rev-parse HEAD)
Original edge image reference: $EDGE_TAG
Original edge image ID: $EDGE_ID
Original document-intake image reference: $DOC_TAG
Original document-intake image ID: $DOC_ID
Database backup (retain): $BACKUP
Rollback: retag the old image IDs to the reference tags above, then run docker compose up -d --no-deps --force-recreate plant-edge-os document-intake.
Do not automatically restore the database: migration is additive, and a restore could destroy new signed transactions.
EOF

# New containers reuse existing PostgreSQL, Redis, WAL and document volumes.
echo '=== Restart only application and document worker ==='
"${COMPOSE[@]}" up -d --no-deps plant-edge-os document-intake

for i in {1..30}; do
  if curl --silent --fail --max-time 3 http://127.0.0.1:18530/healthz >/dev/null; then break; fi
  sleep 2
done
curl --fail --silent --show-error http://127.0.0.1:18530/healthz >/dev/null || {
  echo 'FAIL: edge health. Original image IDs and verified PG backup are recorded above. Roll back the application, not the DB.' >&2
  exit 1
}
HOME_HTTP="$(curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:18530/)"
[[ "$HOME_HTTP" == 200 ]] || { echo "FAIL: public homepage localhost HTTP $HOME_HTTP" >&2; exit 1; }

# The real control tower stays private. 401 is expected behind the dev credential gate.
CONTROL_HTTP="$(curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:18530/cutting/sop2)"
[[ "$CONTROL_HTTP" == 200 || "$CONTROL_HTTP" == 401 ]] || {
  echo "FAIL: control tower HTTP $CONTROL_HTTP (expected 401 private or 200 authenticated)" >&2; exit 1;
}

PUBLIC_FLAG="$(grep -E '^AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED=' .env | tail -1 | cut -d= -f2- || true)"
PUBLIC_HTTP="$(curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:18530/demo/cutting)"
if [[ "$PUBLIC_FLAG" == true ]]; then
  [[ "$PUBLIC_HTTP" == 200 ]] || { echo "FAIL: public training requested but HTTP $PUBLIC_HTTP" >&2; exit 1; }
  curl --fail --silent --show-error http://127.0.0.1:18530/demo-assets/cutter.js > /dev/null
else
  [[ "$PUBLIC_HTTP" == 404 ]] || { echo "FAIL: public training not opted in but HTTP $PUBLIC_HTTP" >&2; exit 1; }
fi

echo "PASS: iMac app rebuilt; local home $HOME_HTTP; private SOP-2 $CONTROL_HTTP; training $PUBLIC_HTTP; database verified."
echo "DB backup: $BACKUP"
echo '=== Public TLS origin verification (external gateway may restrict requests) ==='
if curl --fail --silent --show-error --max-time 12 https://agiparel.empowerai.us/ > /dev/null; then
  echo 'PASS: public TLS homepage reachable from this iMac.'
  if [[ "$PUBLIC_FLAG" == true ]]; then
    if curl --fail --silent --show-error --max-time 12 https://agiparel.empowerai.us/demo/cutting > /dev/null; then
      echo 'PASS: public Cutter Mode training reachable over HTTPS.'
    else
      echo 'WARNING: training works locally but public gateway route not confirmed. Inspect proxy/tunnel/Access policy.'
    fi
  fi
else
  echo 'WARNING: public gateway not confirmed. Check existing reverse proxy/tunnel configuration separately; do not replace DB or volumes.'
fi
