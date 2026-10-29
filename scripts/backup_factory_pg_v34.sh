#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Create and integrity-check a PostgreSQL custom-format backup without plaintext credentials.
set -euo pipefail
umask 077
if [[ "$#" -ne 2 || "${1:-}" != '--output' || -z "${2:-}" ]]; then
  echo 'Usage: PGHOST=... PGUSER=... PGDATABASE=... bash scripts/backup_factory_pg_v34.sh --output /secure/path/backup.dump' >&2
  exit 2
fi
for v in PGHOST PGPORT PGUSER PGDATABASE; do
  if [[ -z "${!v:-}" ]]; then echo "ERROR: $v must be explicitly configured" >&2; exit 2; fi
done
if [[ "$PGDATABASE" == 'postgres' || "$PGDATABASE" == 'template0' || "$PGDATABASE" == 'template1' ]]; then
  echo 'ERROR: PostgreSQL system database not permitted' >&2; exit 2
fi
for tool in pg_dump pg_restore psql; do command -v "$tool" >/dev/null || { echo "ERROR: $tool unavailable" >&2; exit 2; }; done
actual="$(psql -X -A -t -v ON_ERROR_STOP=1 -c 'SELECT current_database()')"
[[ "$actual" == "$PGDATABASE" ]] || { echo 'ERROR: database identity mismatch' >&2; exit 1; }
out="$2"
[[ ! -e "$out" ]] || { echo 'ERROR: destination exists; backup will not be overwritten' >&2; exit 1; }
mkdir -p "$(dirname "$out")"
tmp="$(mktemp "${out}.part.XXXXXXXX")"
trap 'rm -f -- "$tmp"' EXIT
pg_dump -Fc --file "$tmp"
[[ -s "$tmp" ]] || { echo 'ERROR: empty backup' >&2; exit 1; }
pg_restore --list "$tmp" >/dev/null
chmod 600 "$tmp"
ln "$tmp" "$out"
rm -f -- "$tmp"
[[ -s "$out" ]] || { echo 'ERROR: backup not committed' >&2; exit 1; }
if command -v shasum >/dev/null; then shasum -a 256 "$out"; else sha256sum "$out"; fi
printf 'Backup created and format-checked: %s\n' "$out"
printf '%s\n' 'NOTE: this is NOT a restore test or evidence of point-in-time recovery.'
