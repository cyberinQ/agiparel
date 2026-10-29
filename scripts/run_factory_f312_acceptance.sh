#!/usr/bin/env bash
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Staging-only observation verification. NEVER synthesizes a gateway scan.
# Usage: F312_FILE=... F312_PUBLIC_KEY_HEX=... F312_TENANT=... F312_PLANT=...
#        F312_GATEWAY=... F312_STATION=... [F312_DIR=...] bash scripts/run_factory_f312_acceptance.sh verify|audit
set -euo pipefail
MODE="${1:-}"
if [[ "$MODE" != verify && "$MODE" != audit ]]; then
  echo 'Usage: ... run_factory_f312_acceptance.sh verify|audit' >&2
  exit 2
fi
: "${F312_FILE:?Use genuine scanner-signed JSON file; never a simulated event}"
: "${F312_PUBLIC_KEY_HEX:?Provide independently provisioned gateway public key in hex}"
: "${F312_TENANT:?Expected tenant}"
: "${F312_PLANT:?Expected plant}"
: "${F312_GATEWAY:?Expected gateway}"
: "${F312_STATION:?Expected station}"

RELAY="${F312_RELAY:-$HOME/bin/agiparel-scanrelay}"
if [[ ! -x "$RELAY" ]]; then
  echo "ERROR: Build F3.12 with: go build -o \"$RELAY\" ./services/edge-daemon/cmd/scanrelay" >&2
  exit 2
fi
ARGS=("$MODE" --file "$F312_FILE" --public-key-hex "$F312_PUBLIC_KEY_HEX" \
  --tenant "$F312_TENANT" --plant "$F312_PLANT" --gateway "$F312_GATEWAY" --station "$F312_STATION")
if [[ "$MODE" == audit ]]; then
  : "${F312_DIR:?Audit requires the durable F3.11 outbox directory}"
  ARGS+=(--dir "$F312_DIR")
fi
EVIDENCE="$("$RELAY" "${ARGS[@]}")"
printf '%s\n' "$EVIDENCE"
if [[ "$MODE" == verify ]]; then
  echo 'Local signature VALID against supplied public key; run enrollment preflight and actual gateway relay next.'
  echo 'No PostgreSQL receipt, physical RF performance, or custody certified.'
  exit 0
fi

# Parse one strict JSON field at a time without eval, generated shell commands,
# or passing the signed scanner envelope through a shell variable.
get_field() {
  printf '%s\n' "$EVIDENCE" | python3 -c \
    'import json,sys; d=json.load(sys.stdin); v=d[sys.argv[1]]; assert isinstance(v,str) and v; print(v)' "$1"
}
SCAN_ID="$(get_field scan_id)"
EVENT_ID="$(get_field event_id)"
PART_ID="$(get_field part_id)"
CARRIER_ID="$(get_field carrier_id)"
EPC_HEX="$(get_field epc_hex)"
EVENT_HASH="$(get_field event_sha256)"
SIGNATURE_HASH="$(get_field signature_sha256)"
KEY_HEX="$(get_field public_key_hex)"

# PostgreSQL 16 app role is forced inside the read-only SQL transaction.
# SQL performs exact row and key comparisons under tenant RLS.
docker compose -f compose/docker-compose.dev.yml \
  exec -T postgres sh -c '
    exec psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
      -v "tenant_id=$1" -v "plant_id=$2" -v "gateway_id=$3" -v "station_id=$4" \
      -v "event_id=$5" -v "part_id=$6" -v "carrier_id=$7" -v "epc_hex=$8" \
      -v "scan_id=$9" -v "event_sha256=${10}" -v "signature_sha256=${11}" -v "public_key_hex=${12}"
  ' sh "$F312_TENANT" "$F312_PLANT" "$F312_GATEWAY" "$F312_STATION" \
  "$EVENT_ID" "$PART_ID" "$CARRIER_ID" "$EPC_HEX" "$SCAN_ID" \
  "$EVENT_HASH" "$SIGNATURE_HASH" "$KEY_HEX" \
  < scripts/test_wip_f312_live_pg.sql

echo 'F3.12 SIGNED-CHAIN AUDIT PASSED. Physical-reader certification remains pending a witnessed factory test.'
