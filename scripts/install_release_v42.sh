#!/usr/bin/env bash
# Author: Sunil Jain, 2026-10-29.
# V42: verify -> back up -> derive/equivalence-check baseline -> compact -> rebuild -> test -> deploy -> push.
set -euo pipefail
ZIP="${1:?Usage: install_release_v42.sh PATCH.zip SHA256}"
SHA="${2:?Provide SHA256}"
[[ "${AGIPAREL_V42_RESET_DEV:-}" == YES ]] || { echo 'ERROR: Set AGIPAREL_V42_RESET_DEV=YES to authorize destructive DEV reset'; exit 2; }
[[ "$(git rev-parse --show-toplevel)" == "$(pwd -P)" ]] || { echo 'ERROR: run from repository root'; exit 2; }
[[ "$(git rev-parse --short HEAD)" == '96e6add' ]] || { echo 'ERROR: baseline must be v41 96e6add'; exit 2; }
[[ "$(git branch --show-current)" == main ]] || { echo 'ERROR: expected main'; exit 2; }
[[ -z "$(git status --porcelain)" ]] || { echo 'ERROR: working tree not clean'; exit 2; }
command -v docker >/dev/null
command -v node >/dev/null
command -v go >/dev/null
command -v openssl >/dev/null
printf '%s  %s\n' "$SHA" "$ZIP" | shasum -a 256 -c -
unzip -tq "$ZIP"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
unzip -q "$ZIP" -d "$STAGE"
DC=(docker compose -f compose/docker-compose.dev.yml)
# Verify the source database BEFORE installing the new Compose file or touching data.
ACTUAL="$(docker inspect agiparel-postgres --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^POSTGRES_DB=//p' | head -1)"
[[ "$ACTUAL" == agiparel_dev ]] || { echo "ERROR: only agiparel_dev may be reset (found $ACTUAL)"; exit 2; }
if [[ -f .env ]] && grep -q '^POSTGRES_DB=' .env; then
  [[ "$(sed -n 's/^POSTGRES_DB=//p' .env | tail -1)" == agiparel_dev ]] || { echo 'ERROR: .env DB mismatch'; exit 2; }
fi
if [[ -n "${POSTGRES_DB:-}" && "$POSTGRES_DB" != agiparel_dev ]]; then echo 'ERROR: POSTGRES_DB mismatch'; exit 2; fi
[[ -f "$STAGE/scripts/fixtures/v41_legacy_migrations.tar.gz" ]]
"${DC[@]}" up -d postgres
umask 077
mkdir -p "$HOME/agiparel-db-backups"
BACKUP="$HOME/agiparel-db-backups/pre_v42_$(date +%Y%m%d_%H%M%S)_$$.dump"
echo '=== 1 BACKUP ORIGINAL DEV DB ==='
"${DC[@]}" exec -T postgres sh -c 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$BACKUP"
[[ -s "$BACKUP" ]] || { echo 'ERROR: empty backup'; exit 1; }
"${DC[@]}" exec -T postgres pg_restore --list < "$BACKUP" > /dev/null
echo "Verified backup: $BACKUP"
# Use old Compose and old source migrations to prove the new baseline first.
mkdir -p scripts/fixtures
cp "$STAGE/scripts/fixtures/v41_legacy_migrations.tar.gz" scripts/fixtures/
cp "$STAGE/scripts/generate_baseline_v42.sh" scripts/
echo '=== 2 GENERATE BASELINE FROM TWO ISOLATED PG16 DATABASES ==='
bash scripts/generate_baseline_v42.sh
[[ -s services/core-api/migrations/0001_v42_baseline.sql ]]
echo '=== 3 INSTALL COMPACTED SOURCE AND BOOTSTRAP ==='
while IFS= read -r path; do
  [[ -n "$path" ]] || continue
  [[ "$path" != /* && "$path" != *'..'* ]] || { echo 'ERROR: invalid ZIP path'; exit 1; }
  mkdir -p "$(dirname "$path")"
  cp "$STAGE/$path" "$path"
done < <(cd "$STAGE" && find . -type f | sed 's#^./##' | sort)
# Remove only reviewed historical files; scripts/tests now use immutable archived fixtures.
while IFS= read -r path; do
  [[ -n "$path" ]] || continue
  [[ -f "$path" ]] || { echo "ERROR: deletion target absent: $path"; exit 1; }
  git rm -q -- "$path"
done < scripts/v42_delete_manifest.txt
# These are URL-safe random development-only database passwords.
if [[ ! -f .env ]]; then touch .env; chmod 600 .env; fi
for key in AGIPAREL_APP_DB_PASSWORD AGIPAREL_DOC_DB_PASSWORD; do
  if ! grep -q "^${key}=" .env; then
    printf '%s=%s\n' "$key" "$(openssl rand -hex 24)" >> .env
  fi
done
APP_PASS="$(sed -n 's/^AGIPAREL_APP_DB_PASSWORD=//p' .env | tail -1)"
DOC_PASS="$(sed -n 's/^AGIPAREL_DOC_DB_PASSWORD=//p' .env | tail -1)"
[[ "$APP_PASS" =~ ^[a-f0-9]{48}$ && "$DOC_PASS" =~ ^[a-f0-9]{48}$ ]] || { echo 'ERROR: .env database passwords must be 48 lowercase hex characters'; exit 1; }
export AGIPAREL_APP_DB_PASSWORD="$APP_PASS" AGIPAREL_DOC_DB_PASSWORD="$DOC_PASS"
"${DC[@]}" config --quiet
# A new WAL volume is used to prevent replaying v41 events against reset SQL data.
echo '=== 4 REBUILD ONLY agiparel_dev (EXISTING WAL VOLUME RETAINED) ==='
"${DC[@]}" stop plant-edge-os document-intake || true
"${DC[@]}" up -d postgres
"${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "DROP DATABASE IF EXISTS agiparel_dev WITH (FORCE)"'
"${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE agiparel_dev OWNER agiparel_admin TEMPLATE template0"'
"${DC[@]}" exec -T postgres sh -c \
  'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d agiparel_dev -v app_password="$1" -v doc_password="$2"' \
  sh "$APP_PASS" "$DOC_PASS" < scripts/bootstrap_roles_v42.sql
"${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d agiparel_dev' \
  < services/core-api/migrations/0001_v42_baseline.sql > /dev/null
echo '=== 5 ISOLATED SQL / CONCURRENCY / GO / JS TESTS ==='
# Old-schema archive is restored only to prove the original backup can be read.
OLD_DRILL="agiparel_v42_backup_$(date +%s)_$$"
"${DC[@]}" exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" --template=template0 "$1"' sh "$OLD_DRILL"
"${DC[@]}" exec -T postgres sh -c 'pg_restore --exit-on-error -U "$POSTGRES_USER" -d "$1"' sh "$OLD_DRILL" < "$BACKUP" >/dev/null || {
 echo 'ERROR: original backup restore drill failed; preserve backup and examine state'; exit 1;
}
"${DC[@]}" exec -T postgres sh -c 'dropdb -U "$POSTGRES_USER" "$1"' sh "$OLD_DRILL"
DRILL="agiparel_v39_test_$(date +%s)_$$"
"${DC[@]}" exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" --template=template0 "$1"' sh "$DRILL"
"${DC[@]}" exec -T postgres sh -c 'pg_dump -Fc -U "$POSTGRES_USER" -d agiparel_dev' > "$STAGE/current_v42.dump"
"${DC[@]}" exec -T postgres sh -c 'pg_restore --exit-on-error -U "$POSTGRES_USER" -d "$1"' sh "$DRILL" < "$STAGE/current_v42.dump"
for sql in \
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
  echo "PostgreSQL: $sql"
  "${DC[@]}" exec -T postgres sh -c 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$DRILL" < "$sql"
done
for stress in scripts/test_factory_v39_concurrency.sh scripts/test_factory_v40_dock_stress.sh scripts/test_factory_v41_multitenant_load.sh; do
  echo "Disposable concurrency/recovery drill: $stress"
  "${DC[@]}" exec -T -e AGIPAREL_TEST_DB_DISPOSABLE=YES postgres sh -s -- "$DRILL" < "$stress"
done
"${DC[@]}" exec -T postgres sh -c 'dropdb -U "$POSTGRES_USER" "$1"' sh "$DRILL"
for f in scripts/test_*.cjs; do node "$f"; done
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
test -z "$(gofmt -l $(git ls-files '*.go'))"
git diff --check
echo '=== 6 BUILD + DEPLOY ==='
"${DC[@]}" build plant-edge-os document-intake
"${DC[@]}" up -d --no-deps plant-edge-os document-intake
HEALTHY=0
for attempt in $(seq 1 20); do
  if curl --fail --silent --show-error http://127.0.0.1:18530/healthz; then HEALTHY=1; break; fi
  sleep 1
done
[[ "$HEALTHY" == 1 ]] || { echo 'ERROR: deployment health check failed'; exit 1; }
printf '\n'
"${DC[@]}" ps
# No automatic production switch enabled.
echo '=== 7 GITHUB COMMIT + PUSH ==='
git add -A -- .github README.md compose docs scripts services/core-api/migrations
git add -f scripts/fixtures/v41_legacy_migrations.tar.gz docs/history/v34_v41_release_notes.tar.gz
git diff --cached --check
git diff --cached --stat
git commit -m 'refactor(factory): canonical v42 database baseline and audited repository cleanup'
git push origin main
git status --short
echo '=== v42 COMPLETE: DATABASE REBUILT, TESTED AND PUSHED ==='
echo "Pre-reset backup: $BACKUP"
echo 'Old dev WAL remains in its previous Docker volume; v42 uses a new volume.'
