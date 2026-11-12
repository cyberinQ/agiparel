#!/usr/bin/env bash
# Author: Sunil Jain, 2026-11-11 — Fail-closed SOP-2 unit, contract, build, migration and staging deployment checks.
set -Eeuo pipefail
cd "$(dirname "$0")/.."
MODE="${1:---test}"
[[ "$MODE" == '--test' || "$MODE" == '--migrate' || "$MODE" == '--deploy' ]] || { echo 'Usage: scripts/run_cut_sop2_v46_checks.sh [--test|--migrate|--deploy]' >&2; exit 2; }
command -v go >/dev/null && command -v node >/dev/null && command -v gofmt >/dev/null || { echo 'Go, gofmt and Node 18+ are required' >&2; exit 1; }
node --check services/edge-daemon/internal/web/static/cutting_sop2.js
node scripts/test_cut_sop2_contract.cjs
formatting="$(gofmt -l services/edge-daemon/cmd/server/cutting_sop2*.go services/edge-daemon/cmd/server/cutting_lay_drafts.go services/edge-daemon/cmd/server/main.go services/edge-daemon/internal/web/assets.go services/edge-daemon/pkg/cutting/sop2/*.go)"
[[ -z "$formatting" ]] || { echo "gofmt discrepancies: $formatting" >&2; exit 1; }
# The application module declares go 1.26: do not downgrade it or skip its tests.
GOFLAGS=-mod=readonly go test -count=1 ./services/edge-daemon/pkg/cutting/sop2 ./services/edge-daemon/cmd/server ./services/edge-daemon/internal/web
BUILD_BINARY="$(mktemp -u /tmp/agiparel-sop2-server.XXXXXX)"
trap 'rm -f "$BUILD_BINARY"' EXIT
GOFLAGS=-mod=readonly go build -o "$BUILD_BINARY" ./services/edge-daemon/cmd/server
if [[ "$MODE" == '--test' ]]; then echo 'PASS: SOP-2 source and application build checks'; exit 0; fi
command -v docker >/dev/null || { echo 'Docker Compose required for migration/deployment' >&2; exit 1; }
[[ -f .env ]] || { echo 'Create local .env before applying migration' >&2; exit 1; }
if grep -Eq '^ENVIRONMENT=production' .env; then echo 'PRODUCTION BLOCKED: review factory authentication and change approval first' >&2; exit 1; fi
COMPOSE=(docker compose --env-file .env -f compose/docker-compose.dev.yml)
"${COMPOSE[@]}" up -d postgres
BACKUP_DIR="${HOME}/agiparel-db-backups"
mkdir -p "$BACKUP_DIR"
STAMP="$(date +%Y%m%d_%H%M%S)"
BACKUP="$BACKUP_DIR/pre_sop2_v46_${STAMP}_$$.dump"
"${COMPOSE[@]}" exec -T postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$BACKUP"
[[ -s "$BACKUP" ]] || { echo 'Database backup empty; abort' >&2; exit 1; }
echo "Database backup: $BACKUP"
"${COMPOSE[@]}" exec -T postgres sh -c 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' < services/core-api/migrations/0005_cut_sop2_control_plane_v46.sql
"${COMPOSE[@]}" exec -T postgres sh -c 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' < scripts/test_cut_sop2_v46_schema_pg.sql
if [[ "$MODE" == '--migrate' ]]; then echo 'PASS: additive SOP-2 schema migration; server not restarted'; exit 0; fi
"${COMPOSE[@]}" up -d --build plant-edge-os
curl -fsS --retry 10 --retry-delay 2 --retry-connrefused http://127.0.0.1:18530/healthz >/dev/null
# Basic operator authentication may return 401 or 200 depending on dev config.
code="$(curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:18530/cutting/sop2)"
[[ "$code" == '200' || "$code" == '401' ]] || { echo "SOP-2 page status $code; expected protected 200/401" >&2; exit 1; }
echo "PASS: staging SOP-2 deployment; /cutting/sop2 returned HTTP $code"
echo 'Default target writes remain OFF. For development only: set AGIPAREL_CUT_SOP2_WRITES_ENABLED=true and restart plant-edge-os.'
