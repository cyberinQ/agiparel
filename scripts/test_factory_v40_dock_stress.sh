#!/bin/sh
# Author: Sunil Jain, 2026-10-29.
# Test dock uniqueness, tenant scope and crash rollback under real PostgreSQL
# concurrency. Synthetic rows bypass signatures/TRIGGERS ONLY on disposable DB.
set -eu
DB="${1:-${PGDATABASE:-}}"
case "$DB" in
  agiparel_v40_test_*|agiparel_v39_test_*) ;;
  agiparel_dev) [ "${CI:-}" = true ] || { echo 'CI-only agiparel_dev database' >&2; exit 2; } ;;
  *) echo 'Refusing non-disposable PostgreSQL database' >&2; exit 2 ;;
esac
[ "${AGIPAREL_TEST_DB_DISPOSABLE:-}" = YES ] || { echo 'Disposable-db opt-in required' >&2; exit 2; }
DBUSER="${PGUSER:-${POSTGRES_USER:-}}"
[ -n "$DBUSER" ] || { echo 'PGUSER/POSTGRES_USER is required' >&2; exit 2; }
TMP="$(mktemp -d)"; PIDS=''
cleanup() {
  for pid in $PIDS; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$TMP"
}
trap cleanup EXIT HUP INT TERM
psql_db() { psql -X -v ON_ERROR_STOP=1 -U "$DBUSER" -d "$DB" "$@"; }
UUID="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
MANIFEST="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
CARTON="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
CARTON2="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
EXT_MANIFEST="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
CRASH_MANIFEST="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
TENANT="F40_STRESS_$$"
OTHER="F40_OTHER_$$"
HASH='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
N="${AGIPAREL_V40_STRESS_WORKERS:-12}"
case "$N" in ''|*[!0-9]*) echo 'Invalid worker count' >&2; exit 2;; esac
[ "$N" -ge 4 ] && [ "$N" -le 48 ] || { echo 'Worker count must be 4..48' >&2; exit 2; }
# Distinct physical gateway event identifiers but same logical carton: exactly
# one transaction may commit regardless of database scheduling.
i=1
while [ "$i" -le "$N" ]; do
 (
 psql_db > "$TMP/worker_$i.out" 2>&1 <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
SET LOCAL statement_timeout = '10000ms';
INSERT INTO dock_carton_scans_v34
 (tenant_id, plant_id, manifest_id, carton_id, gateway_id, event_id, carton_code,
  observed_at, event_sha256, gateway_key_sha256, signature_sha256)
 VALUES ('$TENANT','F40_PLANT','$MANIFEST','$CARTON','GATEWAY_$(printf '%03d' "$i")',
 'F40_DOCK_EVENT_$(printf '%03d' "$i")','F40_CARTON',clock_timestamp(),'$HASH','$HASH','$HASH');
COMMIT;
SQL
 ) &
 PIDS="$PIDS $!"
 i=$((i + 1))
done
for pid in $PIDS; do wait "$pid" || true; done
PIDS=''
COUNT="$(psql_db -At -c "SELECT count(*) FROM dock_carton_scans_v34 WHERE tenant_id='$TENANT'")"
[ "$COUNT" -eq 1 ] || { echo "Expected exactly one committed scan under $N concurrent requests; got $COUNT" >&2; exit 1; }
# Replay the committed gateway's event ID with a different carton: the
# independent gateway-event uniqueness key must reject this payload.
GATEWAY="$(psql_db -At -c "SELECT gateway_id FROM dock_carton_scans_v34 WHERE tenant_id='$TENANT'")"
EVENT="$(psql_db -At -c "SELECT event_id FROM dock_carton_scans_v34 WHERE tenant_id='$TENANT'")"
if psql_db > "$TMP/replay.out" 2>&1 <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
INSERT INTO dock_carton_scans_v34
 (tenant_id,plant_id,manifest_id,carton_id,gateway_id,event_id,carton_code,
 observed_at,event_sha256,gateway_key_sha256,signature_sha256)
 VALUES('$TENANT','F40_PLANT','$UUID','$CARTON2','$GATEWAY','$EVENT',
 'F40_CARTON_2',clock_timestamp(),'$HASH','$HASH','$HASH');
COMMIT;
SQL
then
 echo 'Gateway-event replay was admitted for a different carton' >&2; exit 1
fi
# Same logical carton in a DIFFERENT tenant may coexist. Tenant-scoped RLS
# must conceal it from the first tenant's application role.
psql_db >/dev/null <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
INSERT INTO dock_carton_scans_v34
 (tenant_id,plant_id,manifest_id,carton_id,gateway_id,event_id,carton_code,
 observed_at,event_sha256,gateway_key_sha256,signature_sha256)
 VALUES('$OTHER','F40_PLANT','$MANIFEST','$CARTON','GATEWAY_OTHER',
 'F40_DOCK_OTHER_001','F40_CARTON',clock_timestamp(),'$HASH','$HASH','$HASH');
COMMIT;
SQL
psql_db >/dev/null <<SQL
BEGIN;
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id','$TENANT',true);
DO \$\$ DECLARE own_count int; other_count int; BEGIN
 SELECT count(*) INTO own_count FROM dock_carton_scans_v34 WHERE tenant_id='$TENANT';
 SELECT count(*) INTO other_count FROM dock_carton_scans_v34 WHERE tenant_id='$OTHER';
 IF own_count <> 1 OR other_count <> 0 THEN
  RAISE EXCEPTION 'F40_RLS_BROKEN: own %, other %',own_count,other_count;
 END IF;
END \$\$;
COMMIT;
SQL
# Independent external counterparties racing to acknowledge the SAME manifest
# must not create multiple carrier assertions. This tests the real unique index,
# not the ingress Ed25519 verifier (which is exercised in Go separately).
i=1; PIDS=''
while [ "$i" -le "$N" ]; do
 (
 psql_db > "$TMP/external_$i.out" 2>&1 <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
SET LOCAL statement_timeout = '10000ms';
INSERT INTO dock_external_assertions_v35
 (tenant_id,plant_id,manifest_id,asn_draft_id,signer_id,party_kind,party_code,
 assertion_kind,decision,event_id,external_reference,asn_reference,payload_sha256,
 issued_at,event_sha256,signer_key_sha256,signature_sha256)
 VALUES('$TENANT','F40_PLANT','$EXT_MANIFEST','$UUID','F40_SIGNER_$(printf '%03d' "$i")',
 'CARRIER','F40_CARRIER','CARRIER_ASN_ACK','ACKNOWLEDGED',
 'F40_EXT_EVENT_$(printf '%03d' "$i")','F40-EXT-ACK-$(printf '%03d' "$i")',
 'F40-ASN-001','$HASH',clock_timestamp(),'$HASH','$HASH','$HASH');
COMMIT;
SQL
 ) &
 PIDS="$PIDS $!"
 i=$((i + 1))
done
for pid in $PIDS; do wait "$pid" || true; done
PIDS=''
EXTERNAL_COUNT="$(psql_db -At -c "SELECT count(*) FROM dock_external_assertions_v35 WHERE tenant_id='$TENANT' AND manifest_id='$EXT_MANIFEST'")"
[ "$EXTERNAL_COUNT" -eq 1 ] || { echo "Expected one carrier acknowledgement, got $EXTERNAL_COUNT" >&2; exit 1; }

# A backend death AFTER its write but BEFORE COMMIT must leave no assertion.
CRASH="F40_BACKEND_$$"
psql_db > "$TMP/crash.out" 2>&1 <<SQL &
BEGIN;
SET LOCAL session_replication_role = replica;
SET LOCAL application_name = '$CRASH';
INSERT INTO dock_external_assertions_v35
 (tenant_id,plant_id,manifest_id,asn_draft_id,signer_id,party_kind,party_code,
 assertion_kind,decision,event_id,external_reference,asn_reference,payload_sha256,
 issued_at,event_sha256,signer_key_sha256,signature_sha256)
 VALUES('$TENANT','F40_PLANT','$CRASH_MANIFEST','$UUID','F40_SIGNER','CARRIER','F40_CARRIER',
 'CARRIER_ASN_ACK','ACKNOWLEDGED','F40_CRASH_EVENT_001','F40-CRASH-ACK',
 'F40-ASN-001','$HASH',clock_timestamp(),'$HASH','$HASH','$HASH');
SELECT pg_sleep(30);
COMMIT;
SQL
PID=$!; PIDS="$PID"; BACKEND=''; i=0
while [ "$i" -lt 120 ]; do
 BACKEND="$(psql_db -At -c "SELECT pid FROM pg_stat_activity WHERE datname='$DB' AND application_name='$CRASH' AND state='active' AND query LIKE '%pg_sleep(30)%' LIMIT 1")"
 [ -n "$BACKEND" ] && break
 kill -0 "$PID" 2>/dev/null || break
 sleep 0.05
 i=$((i + 1))
done
[ -n "$BACKEND" ] || { cat "$TMP/crash.out" >&2; echo 'Crash barrier not reached' >&2; exit 1; }
[ "$(psql_db -At -c "SELECT pg_terminate_backend($BACKEND)")" = t ] || { echo 'Backend termination failed' >&2; exit 1; }
if wait "$PID"; then echo 'Terminated writer committed unexpectedly' >&2; exit 1; fi
PIDS=''
GHOST="$(psql_db -At -c "SELECT count(*) FROM dock_external_assertions_v35 WHERE tenant_id='$TENANT' AND manifest_id='$CRASH_MANIFEST'")"
[ "$GHOST" -eq 0 ] || { echo 'Uncommitted carrier assertion survived backend death' >&2; exit 1; }
# All synthetic evidence belongs to the disposable DB, but also clean it up.
psql_db >/dev/null <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
DELETE FROM dock_carton_scans_v34 WHERE tenant_id IN ('$TENANT','$OTHER');
DELETE FROM dock_external_assertions_v35 WHERE tenant_id='$TENANT';
COMMIT;
SQL
printf 'PASS v40: %s concurrent gateway + carrier writes, replay protection, RLS, crash rollback\n' "$N"
