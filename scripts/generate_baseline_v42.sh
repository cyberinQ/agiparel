#!/usr/bin/env bash
# Author: Sunil Jain, 2026-10-29.
# Replay historical migrations in isolated databases; prove equivalence before destructive reset.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
DC=(docker compose -f compose/docker-compose.dev.yml)
OUT='services/core-api/migrations/0001_v42_baseline.sql'
FIXTURES='scripts/fixtures/v41_legacy_migrations.tar.gz'
[[ -f "$FIXTURES" ]] || { echo 'ERROR: missing legacy archive'; exit 1; }
[[ ! -e "$OUT" ]] || { echo 'ERROR: baseline already exists; refusing regeneration'; exit 1; }
WORK="$(mktemp -d)"
SUFFIX="$(date +%s)_$$"
OLD="agiparel_v42_legacy_${SUFFIX}"
NEW="agiparel_v42_baseline_${SUFFIX}"
NORM_OLD="agiparel_v42_normalized_legacy_${SUFFIX}"
NORM_NEW="agiparel_v42_normalized_baseline_${SUFFIX}"
cleanup() {
  "${DC[@]}" exec -T postgres sh -c 'dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$NORM_OLD" >/dev/null 2>&1 || true
  "${DC[@]}" exec -T postgres sh -c 'dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$NORM_NEW" >/dev/null 2>&1 || true
  "${DC[@]}" exec -T postgres sh -c 'dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$NEW" >/dev/null 2>&1 || true
  "${DC[@]}" exec -T postgres sh -c 'dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$OLD" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT
mkdir -p "$WORK/migrations"
tar -xzf "$FIXTURES" -C "$WORK/migrations"
N=0
for migration in "$WORK"/migrations/[0-9][0-9][0-9][0-9]_*.sql; do
  N=$((N+1))
  [[ "$(basename "$migration" | cut -c1-4)" == "$(printf '%04d' "$N")" ]] || { echo 'ERROR: historical sequence gap'; exit 1; }
done
[[ "$N" -eq 57 ]] || { echo "ERROR: expected 57 migrations, found $N"; exit 1; }
# Reject archives differing from the actual checked-out old migration sources.
for f in "$WORK"/migrations/[0-9][0-9][0-9][0-9]_*.sql; do
  cmp -s "$f" "services/core-api/migrations/$(basename "$f")" || { echo "ERROR: legacy archive mismatch: $f"; exit 1; }
done
cmp -s "$WORK/migrations/s01_identity_foundation.sql" "services/s01-identity/migrations/0001_s01_s02_foundation.sql" || {
  echo 'ERROR: identity migration archive mismatch'; exit 1;
}
"${DC[@]}" exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" --template=template0 "$1"' sh "$OLD"
"${DC[@]}" exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" --template=template0 "$1"' sh "$NEW"
echo '=== REPLAY 57 LEGACY MIGRATIONS IN DISPOSABLE DATABASE ==='
for f in "$WORK"/migrations/[0-9][0-9][0-9][0-9]_*.sql; do
  "${DC[@]}" exec -T postgres sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$OLD" < "$f" > /dev/null
done
# Identity migration was previously separate: incorporate it explicitly in v42.
"${DC[@]}" exec -T postgres sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$OLD" \
  < "$WORK/migrations/s01_identity_foundation.sql" > /dev/null
SCHEMA="$WORK/schema.sql"
SEEDS="$WORK/seeds.sql"
"${DC[@]}" exec -T postgres sh -c 'pg_dump --schema-only --no-owner -U "$POSTGRES_USER" -d "$1"' sh "$OLD" > "$SCHEMA"
"${DC[@]}" exec -T postgres sh -c 'pg_dump --data-only --inserts --no-owner -U "$POSTGRES_USER" -d "$1"' sh "$OLD" > "$SEEDS"
{
  printf '%s\n' '-- Author: Sunil Jain, 2026-10-29.' '-- Agiparel v42 canonical baseline: PostgreSQL 16 generated from historical migrations.' '-- Role passwords are provisioned separately; this file is not a migration concatenation.'
  cat "$SCHEMA"
  cat "$SEEDS"
} > "$WORK/baseline.sql"
echo '=== REAPPLY CANONICAL BASELINE TO SECOND FRESH DATABASE ==='
"${DC[@]}" exec -T postgres sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$NEW" < "$WORK/baseline.sql" > /dev/null
# Keep object/ACL/seed comparisons, but do not compare first-generation pg_dump
# textual expressions directly: PostgreSQL normalizes e.g. varchar ANY arrays
# during restore without changing semantics.
strip_dump_tokens() { sed '/^\\restrict /d; /^\\unrestrict /d'; }
for db in "$OLD" "$NEW"; do
  "${DC[@]}" exec -T postgres sh -c 'pg_dump --schema-only --no-owner -U "$POSTGRES_USER" -d "$1"' sh "$db" \
    | strip_dump_tokens > "$WORK/${db}.schema"
  "${DC[@]}" exec -T postgres sh -c 'pg_dump --data-only --inserts --no-owner -U "$POSTGRES_USER" -d "$1"' sh "$db" \
    | strip_dump_tokens > "$WORK/${db}.data"
done
# Exact reference data, including setval sequence states, must be unchanged.
if ! cmp -s "$WORK/${OLD}.data" "$WORK/${NEW}.data"; then
  diff -u "$WORK/${OLD}.data" "$WORK/${NEW}.data" | sed -n '1,100p' || true
  echo 'ERROR: reference-data baseline mismatch'
  exit 1
fi
# Perform one *identical* PG16 parser/serializer round-trip on each side.
# This normalizes equivalent CHECK/partial-index expressions through PostgreSQL
# itself, rather than using regex/whitespace deletion that could mask defects.
for entry in "$OLD:$NORM_OLD" "$NEW:$NORM_NEW"; do
  src="${entry%%:*}"
  dst="${entry#*:}"
  "${DC[@]}" exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" --template=template0 "$1"' sh "$dst"
  "${DC[@]}" exec -T postgres sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$dst" \
    < "$WORK/${src}.schema" > /dev/null
  "${DC[@]}" exec -T postgres sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "$dst" \
    < "$WORK/${src}.data" > /dev/null
  "${DC[@]}" exec -T postgres sh -c 'pg_dump --schema-only --no-owner -U "$POSTGRES_USER" -d "$1"' sh "$dst" \
    | strip_dump_tokens > "$WORK/${dst}.schema"
  "${DC[@]}" exec -T postgres sh -c 'pg_dump --data-only --inserts --no-owner -U "$POSTGRES_USER" -d "$1"' sh "$dst" \
    | strip_dump_tokens > "$WORK/${dst}.data"
done
if ! cmp -s "$WORK/${NORM_OLD}.schema" "$WORK/${NORM_NEW}.schema"; then
  diff -u "$WORK/${NORM_OLD}.schema" "$WORK/${NORM_NEW}.schema" | sed -n '1,100p' || true
  echo 'ERROR: normalized PostgreSQL schema or ACL mismatch'
  exit 1
fi
if ! cmp -s "$WORK/${NORM_OLD}.data" "$WORK/${NORM_NEW}.data"; then
  diff -u "$WORK/${NORM_OLD}.data" "$WORK/${NORM_NEW}.data" | sed -n '1,100p' || true
  echo 'ERROR: normalized reference-data mismatch'
  exit 1
fi
cp "$WORK/baseline.sql" "$OUT"
echo "PASS: equivalent schema AND seeded data; baseline: $OUT"
