#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# v41: verified inflate -> no migration -> backup and disposable DB drill ->
# Go/JS/DB/dock stress tests -> build -> deploy -> GitHub, stopping on any error.
# Always execute from a file, never `unzip -p ... | bash`.
set -Eeuo pipefail
ZIP="${1:-}"
EXPECTED_SHA="${2:-}"
BASELINE='4831c15'
COMPOSE='compose/docker-compose.dev.yml'
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
trap 'printf "FAILED v41 near line %s; no automatic Git push.\n" "$LINENO" >&2' ERR
[[ "$EXPECTED_SHA" =~ ^[0-9a-f]{64}$ ]] || fail 'Exact ZIP SHA-256 required'
[ -f "$ZIP" ] && [ -f go.mod ] && [ -f "$COMPOSE" ] || fail 'Run from agiparel root and download ZIP'
[ "$(git branch --show-current)" = main ] || fail 'Requires main branch'
[ "$(git rev-parse --short HEAD)" = "$BASELINE" ] || fail "Requires commit $BASELINE"
# Clean tree or byte-identical partially inflated patch only.
while IFS= read -r path; do
 [ -n "$path" ] || continue
 unzip -Z1 "$ZIP" | grep -Fqx -- "$path" || fail "Unrelated dirty working-tree path: $path"
done < <(git status --porcelain | cut -c4-)
printf '%s  %s\n' "$EXPECTED_SHA" "$ZIP" | shasum -a 256 -c -
unzip -tq "$ZIP"
while IFS= read -r path; do
 [ -n "$path" ] || continue
 case "$path" in /*|../*|*/../*|*/..|.|./*|*'//'*) fail "Unsafe ZIP path: $path";; esac
 [[ "$path" =~ ^[A-Za-z0-9_.+/-]+$ ]] || fail "Invalid ZIP path: $path"
 [[ "$path" != */ ]] || fail "Unexpected directory: $path"
 [ ! -L "$path" ] || fail "Symlink conflict: $path"
 if git ls-files --error-unmatch -- "$path" >/dev/null 2>&1 &&
    ! git diff --quiet HEAD -- "$path" &&
    ! cmp -s "$path" <(unzip -p "$ZIP" "$path"); then
  fail "Tracked file with unrelated edits: $path"
 fi
 if [ -e "$path" ] && ! git ls-files --error-unmatch -- "$path" >/dev/null 2>&1 &&
    ! cmp -s "$path" <(unzip -p "$ZIP" "$path"); then
  fail "Untracked path with unrelated bytes: $path"
 fi
done < <(unzip -Z1 "$ZIP")

echo '=== 1 INFLATE ==='
unzip -oq "$ZIP" -d .
git diff --check

echo '=== 2 MIGRATION: NONE (v38 migration 0057 remains current) ==='
DC=(docker compose -f "$COMPOSE")
"${DC[@]}" up -d postgres
STATE=$("${DC[@]}" exec -T postgres sh -c \
 'psql -X -At -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
SELECT CASE WHEN to_regprocedure('public.dock_integrity_report_v38(text,text,uuid)') IS NOT NULL
 AND to_regclass('public.pack_carton_events_v22') IS NOT NULL
 AND to_regclass('public.pack_one_carton_per_garment_v22') IS NOT NULL
 THEN 'ready' ELSE 'invalid' END;
SQL
)
[ "$STATE" = ready ] || fail 'Requires completed v38 migration 0057 and packing uniqueness'
umask 077
mkdir -p "$HOME/agiparel-db-backups"
STAMP="$(date +%Y%m%d_%H%M%S)_$$"
BACKUP="$HOME/agiparel-db-backups/pre_v41_${STAMP}.dump"
SCRATCH="agiparel_v39_test_${STAMP}"
SCRATCH_CREATED=0
cleanup() {
 if [ "$SCRATCH_CREATED" -eq 1 ]; then
  "${DC[@]}" exec -T postgres sh -c \
   'dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$SCRATCH" || true
 fi
}
trap cleanup EXIT

echo '=== 3 BACKUP + DISPOSABLE RESTORE + DB/CONCURRENCY/GO/JS TESTS ==='
"${DC[@]}" exec -T postgres sh -c \
 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$BACKUP"
[ -s "$BACKUP" ] || fail 'Empty database backup'
"${DC[@]}" exec -T postgres pg_restore --list < "$BACKUP" >/dev/null
printf 'Backup: %s\n' "$BACKUP"
"${DC[@]}" exec -T postgres sh -c \
 'createdb -U "$POSTGRES_USER" --template=template0 "$1"' sh "$SCRATCH"
SCRATCH_CREATED=1
"${DC[@]}" exec -T postgres sh -c \
 'pg_restore --exit-on-error -U "$POSTGRES_USER" -d "$1"' sh "$SCRATCH" < "$BACKUP"
for script in \
 scripts/test_factory_f319c_f319d_f320a_schema_pg.sql \
 scripts/test_factory_f320b_f320d_schema_pg.sql \
 scripts/test_factory_v34_tenant_isolation_pg.sql \
 scripts/test_factory_v35_schema_pg.sql \
 scripts/test_factory_v35_tenant_isolation_pg.sql \
 scripts/test_factory_v36_schema_pg.sql \
 scripts/test_factory_v36_tenant_isolation_pg.sql \
 scripts/test_factory_v37_schema_pg.sql \
 scripts/test_factory_v37_isolation_pg.sql \
 scripts/test_factory_v38_schema_pg.sql \
 scripts/test_factory_v38_tenant_isolation_pg.sql \
 scripts/test_factory_v39_schema_pg.sql \
 scripts/test_factory_v40_schema_pg.sql \
 scripts/test_factory_v41_security_pg.sql; do
 echo "PostgreSQL: $script"
 "${DC[@]}" exec -T postgres sh -c \
  'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$SCRATCH" < "$script"
done
echo '=== CONCURRENCY + CONNECTION FAILURE DRILL (SCRATCH ONLY) ==='
"${DC[@]}" exec -T -e AGIPAREL_TEST_DB_DISPOSABLE=YES postgres sh -s -- "$SCRATCH" \
 < scripts/test_factory_v39_concurrency.sh
echo '=== DOCK INGEST LOAD + REPLAY + CRASH DRILL (SCRATCH ONLY) ==='
"${DC[@]}" exec -T -e AGIPAREL_TEST_DB_DISPOSABLE=YES postgres sh -s -- "$SCRATCH"  < scripts/test_factory_v40_dock_stress.sh
echo '=== TENANT ISOLATION + TRUST REVOCATION LOAD (SCRATCH ONLY) ==='
"${DC[@]}" exec -T -e AGIPAREL_TEST_DB_DISPOSABLE=YES postgres sh -s -- "$SCRATCH" \
 < scripts/test_factory_v41_multitenant_load.sh
cleanup
SCRATCH_CREATED=0

go version
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
for script in scripts/test_*.cjs; do node "$script"; done
[ -z "$(find services -name '*.go' -exec gofmt -l {} +)" ] || fail 'gofmt check failed'
git diff --check

echo '=== 4 BUILD ==='
go build ./...
"${DC[@]}" build plant-edge-os

echo '=== 5 DEPLOY + HEALTH ==='
"${DC[@]}" up -d --no-deps plant-edge-os
HEALTH=$(curl --fail --silent --show-error --retry 5 --retry-delay 1 --retry-connrefused \
 http://127.0.0.1:18530/healthz)
printf 'Health: %s\n' "$HEALTH"
[[ "$HEALTH" == *'"status":"UP"'* ]] || fail 'UP health missing'
[[ "$HEALTH" == *'"wal_synced":true'* ]] || fail 'WAL synced health missing'
"${DC[@]}" ps
"${DC[@]}" logs --tail=30 plant-edge-os

echo '=== 6 GIT COMMIT / PUSH ==='
while IFS= read -r path; do [ -n "$path" ] && git add -- "$path"; done < <(unzip -Z1 "$ZIP")
git diff --cached --check
[ -n "$(git diff --cached --name-only)" ] || fail 'No release changes staged'
git diff --cached --stat
git commit -m 'test(factory): harden production opt-in and verify parallel trust isolation v41'
git push origin main
git status --short
git log -2 --oneline
printf 'v41 complete. Backup: %s\n' "$BACKUP"
printf '%s\n' 'No migration or shipment-credit / carrier-delivery changes; production sewing is opt-in.'
