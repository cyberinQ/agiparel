#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# v38: verified/resumable unzip -> backup/migrate -> tests -> build -> deploy -> GitHub.
# Must be executed from a FILE, never `unzip -p ... | bash`.
set -Eeuo pipefail

ZIP="${1:-}"
EXPECTED_SHA="${2:-}"
BASELINE='67690c7'
MIGRATION='services/core-api/migrations/0057_material_attribution_integrity_v38.sql'
COMPOSE='compose/docker-compose.dev.yml'
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
trap 'printf "FAILED v38 at line %s; no Git push unless release completes.\n" "$LINENO" >&2' ERR
[[ "$EXPECTED_SHA" =~ ^[0-9a-f]{64}$ ]] || fail 'ZIP and exact SHA-256 required'
[ -f "$ZIP" ] && [ -f go.mod ] && [ -f "$COMPOSE" ] || fail 'Run from agiparel repo root; ZIP must exist'
[ "$(git branch --show-current)" = 'main' ] || fail 'Requires main branch'
[ "$(git rev-parse --short HEAD)" = "$BASELINE" ] || fail "Requires baseline $BASELINE"
# Also permit a safe resume of an identical already-inflated patch. Only
# exact packaged file bytes or an unchanged HEAD file may be overwritten.
for file in $(git status --porcelain | cut -c4-); do
  if ! unzip -Z1 "$ZIP" | grep -Fqx -- "$file"; then
    fail "Unrelated dirty working-tree path: $file"
  fi
done
printf '%s  %s\n' "$EXPECTED_SHA" "$ZIP" | shasum -a 256 -c -
unzip -tq "$ZIP"
# Before extracting, ensure no unsafe paths, traversal or untracked overwrite.
while IFS= read -r path; do
 [ -n "$path" ] || continue
 case "$path" in /*|../*|*/../*|*/..|.|./*|*'//'*) fail "Unsafe ZIP path: $path";; esac
 [[ "$path" =~ ^[A-Za-z0-9_.+/-]+$ ]] || fail "Invalid ZIP path: $path"
 [[ "$path" != */ ]] || fail "Unexpected directory: $path"
 [ ! -L "$path" ] || fail "Symlink conflict: $path"
 if git ls-files --error-unmatch -- "$path" >/dev/null 2>&1 &&
    ! git diff --quiet HEAD -- "$path" &&
    ! cmp -s "$path" <(unzip -p "$ZIP" "$path"); then
   fail "Tracked file has unrelated edits: $path"
 fi
 if [ -e "$path" ] && ! git ls-files --error-unmatch -- "$path" >/dev/null 2>&1; then
  if ! cmp -s "$path" <(unzip -p "$ZIP" "$path"); then
   fail "Untracked path exists with different bytes: $path"
  fi
 fi
done < <(unzip -Z1 "$ZIP")

echo '=== 1 INFLATE ==='
unzip -oq "$ZIP" -d .
git diff --check
DC=(docker compose -f "$COMPOSE")
"${DC[@]}" up -d postgres
# Schema-state check enables safe resume after a partially completed validation.
STATE=$("${DC[@]}" exec -T postgres sh -c \
 'psql -X -At -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
SELECT CASE
 WHEN to_regprocedure('public.dock_integrity_report_v38(text,text,uuid)') IS NOT NULL
  AND to_regprocedure('public.dock_material_trace_v38(text,text,uuid)') IS NOT NULL THEN 'already'
 WHEN to_regprocedure('public.dock_lineage_audit_v37(text,text,uuid)') IS NOT NULL
  AND to_regprocedure('public.dock_integrity_report_v38(text,text,uuid)') IS NULL
  AND to_regprocedure('public.dock_material_trace_v38(text,text,uuid)') IS NULL THEN 'ready'
 ELSE 'invalid' END;
SQL
)
[[ "$STATE" == ready || "$STATE" == already ]] || fail 'Requires migration 0056 and a clean 0057 state. Inspect DB.'
umask 077
mkdir -p "$HOME/agiparel-db-backups"
STAMP="$(date +%Y%m%d_%H%M%S)_$$"
PRE="$HOME/agiparel-db-backups/pre_v38_${STAMP}.dump"
POST="$HOME/agiparel-db-backups/post_v38_${STAMP}.dump"
SCRATCH="agiparel_v38_test_${STAMP}"
SCRATCH_CREATED=0
cleanup() {
 if [ "$SCRATCH_CREATED" -eq 1 ]; then
  "${DC[@]}" exec -T postgres sh -c \
   'dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$SCRATCH" || true
 fi
}
trap cleanup EXIT

echo '=== 2 BACKUP + MIGRATE 0057 ==='
"${DC[@]}" exec -T postgres sh -c \
 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$PRE"
[ -s "$PRE" ] || fail 'Empty pre-migration backup'
"${DC[@]}" exec -T postgres pg_restore --list < "$PRE" >/dev/null
printf 'Backup: %s\n' "$PRE"
if [[ "$STATE" == ready ]]; then
 "${DC[@]}" exec -T postgres sh -c \
  'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$MIGRATION"
else
 echo 'Migration 0057 already applied; resuming validation (no duplicate DDL).'
fi

echo '=== 3 SCRATCH RESTORE + DB / GO / JS TESTS ==='
"${DC[@]}" exec -T postgres sh -c \
 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$POST"
[ -s "$POST" ] || fail 'Empty post-migration backup'
"${DC[@]}" exec -T postgres sh -c \
 'createdb -U "$POSTGRES_USER" --template=template0 "$1"' sh "$SCRATCH"
SCRATCH_CREATED=1
"${DC[@]}" exec -T postgres sh -c \
 'pg_restore --exit-on-error -U "$POSTGRES_USER" -d "$1"' sh "$SCRATCH" < "$POST"
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
 scripts/test_factory_v38_tenant_isolation_pg.sql; do
 echo "PostgreSQL: $script"
 "${DC[@]}" exec -T postgres sh -c \
  'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$SCRATCH" < "$script"
done
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
[[ "$HEALTH" == *'"status":"UP"'* ]] || fail 'UP health status missing'
[[ "$HEALTH" == *'"wal_synced":true'* ]] || fail 'WAL sync health missing'
"${DC[@]}" ps
"${DC[@]}" logs --tail=30 plant-edge-os

echo '=== 6 GIT COMMIT / PUSH ==='
while IFS= read -r path; do [ -n "$path" ] && git add -- "$path"; done < <(unzip -Z1 "$ZIP")
git diff --cached --check
[ -n "$(git diff --cached --name-only)" ] || fail 'No staged changes'
git diff --cached --stat
git commit -m 'feat(factory): add material attribution and unified integrity diagnostics v38'
git push origin main
git status --short
git log -2 --oneline
printf 'v38 complete. Backups: %s %s\n' "$PRE" "$POST"
printf '%s\n' 'Material classification trace only: no shipment-credit, global stock credit or carrier delivery enabled.'
