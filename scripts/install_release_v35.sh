#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# One-shot F3.22 deployment: inflate -> backup/migrate -> scratch restore/tests
# -> build -> deploy/health -> exact-file git commit/push. Stop on any failure.
set -Eeuo pipefail

ZIP="${1:-}"
EXPECTED_SHA="${2:-}"
BASELINE='90dff56'
MIGRATION='services/core-api/migrations/0054_external_acknowledgement_and_dock_acceptance_v35.sql'
COMPOSE='compose/docker-compose.dev.yml'

fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
[ -n "$ZIP" ] && [ -n "$EXPECTED_SHA" ] || fail 'Provide ZIP path and SHA256 as the first two arguments'
[[ "$EXPECTED_SHA" =~ ^[0-9a-f]{64}$ ]] || fail 'Invalid SHA256 format'
[ -f "$ZIP" ] || fail "ZIP not found: $ZIP"
[ -f 'go.mod' ] && [ -f "$COMPOSE" ] || fail 'Run from the agiparel repository root'
[ "$(git branch --show-current)" = 'main' ] || fail 'Switch to main before installation'
[ "$(git rev-parse --short HEAD)" = "$BASELINE" ] || fail "Expected Git baseline $BASELINE"
[ -z "$(git status --porcelain)" ] || fail 'Working tree is not clean; preserve/commit changes first'

printf '%s  %s\n' "$EXPECTED_SHA" "$ZIP" | shasum -a 256 -c -
unzip -tq "$ZIP"

# Reject traversal, absolute names, non-file items and untracked collisions.
while IFS= read -r path; do
  [ -n "$path" ] || continue
  case "$path" in
    /*|../*|*/../*|*/..|.|./*) fail "Unsafe path in ZIP: $path" ;;
  esac
  [[ "$path" =~ ^[A-Za-z0-9_.+/-]+$ ]] || fail "Invalid path in ZIP: $path"
  [[ "$path" != */ ]] || fail "Directories not permitted in patch ZIP: $path"
  if [ -e "$path" ] && ! git ls-files --error-unmatch -- "$path" >/dev/null 2>&1; then
    fail "Untracked path conflict: $path"
  fi
done < <(unzip -Z1 "$ZIP")

echo '=== INFLATE v35 PATCH ==='
unzip -oq "$ZIP" -d .
git diff --check

DC=(docker compose -f "$COMPOSE")
"${DC[@]}" up -d postgres

SCHEMA_STATE=$("${DC[@]}" exec -T postgres sh -c \
 'psql -X -At -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
SELECT CASE WHEN to_regclass('public.dock_asn_drafts_v34') IS NOT NULL
  AND to_regclass('public.dock_external_assertions_v35') IS NULL
  THEN 'ready' ELSE 'not-ready' END;
SQL
)
[ "$SCHEMA_STATE" = 'ready' ] || fail 'Requires migration 0053 applied and 0054 absent; inspect DB before retrying'

umask 077
mkdir -p "$HOME/agiparel-db-backups"
STAMP=$(date +%Y%m%d_%H%M%S)
PRE="$HOME/agiparel-db-backups/pre_v35_${STAMP}.dump"
POST="$HOME/agiparel-db-backups/post_v35_${STAMP}.dump"
SCRATCH="agiparel_v35_drill_${STAMP}"
SCRATCH_CREATED=0
cleanup() {
  if [ "$SCRATCH_CREATED" -eq 1 ]; then
    "${DC[@]}" exec -T postgres sh -c \
      'dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$SCRATCH" || true
  fi
}
trap cleanup EXIT

echo '=== BACKUP ==='
"${DC[@]}" exec -T postgres sh -c \
 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$PRE"
[ -s "$PRE" ] || fail 'Empty pre-migration backup'
"${DC[@]}" exec -T postgres pg_restore --list < "$PRE" >/dev/null
printf 'Pre-migration backup: %s\n' "$PRE"

echo '=== MIGRATE 0054 ==='
"${DC[@]}" exec -T postgres sh -c \
 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$MIGRATION"

echo '=== BACKUP POST MIGRATION; RESTORE DISPOSABLE DB ==='
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
 scripts/test_factory_v35_tenant_isolation_pg.sql; do
  echo "=== PostgreSQL test $script ==="
  "${DC[@]}" exec -T postgres sh -c \
   'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$SCRATCH" < "$script"
done
cleanup
SCRATCH_CREATED=0

echo '=== GO TEST / RACE / VET ==='
go version
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...

echo '=== JAVASCRIPT CONTRACT REGRESSIONS ==='
for script in scripts/test_*.cjs; do node "$script"; done

echo '=== FORMAT + BUILD ==='
[ -z "$(find services -name '*.go' -exec gofmt -l {} +)" ] || fail 'Unformatted Go source'
git diff --check
go build ./...
"${DC[@]}" build plant-edge-os

echo '=== DEPLOY + HEALTH CHECK ==='
"${DC[@]}" up -d --no-deps plant-edge-os
curl --fail --silent --show-error --retry 5 --retry-delay 1 --retry-connrefused \
 http://127.0.0.1:18530/healthz
printf '\n'
"${DC[@]}" ps
"${DC[@]}" logs --tail=30 plant-edge-os

echo '=== COMMIT PATCH FILES + PUSH ==='
while IFS= read -r path; do [ -n "$path" ] && git add -- "$path"; done < <(unzip -Z1 "$ZIP")
git diff --cached --check
git diff --cached --stat
[ -n "$(git diff --cached --name-only)" ] || fail 'No staged changes'
git commit -m 'feat(factory): add externally signed ASN/buyer assertions and dock reconciliation'
git push origin main

echo '=== RELEASE COMPLETED ==='
git status --short
git log -2 --oneline
printf 'Backups: %s %s\n' "$PRE" "$POST"
printf '%s\n' 'External signing routes remain disabled unless explicitly provisioned and enabled.'
