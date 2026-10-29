#!/bin/sh
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# Exercise real PostgreSQL uniqueness and crash rollback on a DISPOSABLE database.
# Uses superuser replica mode solely to synthesize otherwise impossible device evidence;
# does not validate signatures, physical readers, or the full signed workflow.
set -eu
DB="${1:-${PGDATABASE:-}}"
case "$DB" in
  agiparel_v39_test_*|agiparel_dev) ;;
  *) echo 'ERROR: disposable v39 database required' >&2; exit 2 ;;
esac
[ "${AGIPAREL_TEST_DB_DISPOSABLE:-}" = YES ] || {
  echo 'ERROR: AGIPAREL_TEST_DB_DISPOSABLE=YES required' >&2; exit 2;
}
USER_NAME="${PGUSER:-${POSTGRES_USER:-}}"
[ -n "$USER_NAME" ] || { echo 'ERROR: PGUSER/POSTGRES_USER missing' >&2; exit 2; }
DIR="$(mktemp -d)"
PIDS=''
cleanup() {
  for pid in $PIDS; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$DIR"
}
trap cleanup EXIT HUP INT TERM
psql_db() { psql -X -v ON_ERROR_STOP=1 -U "$USER_NAME" -d "$DB" "$@"; }

# Uniqueness is enforced by the real v22 index despite simultaneous transactions
# and even when a privileged administrator bypasses application INSERT guards.
TENANT="F39_RACE_$$"
GARMENT="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
CARTON_A="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
CARTON_B="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
SPEC="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
EVENT_A="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
EVENT_B="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
HASH='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'

psql_db > "$DIR/winner.log" 2>&1 <<SQL &
BEGIN;
SET LOCAL session_replication_role = replica;
SET LOCAL application_name = 'F39_WIN_$TENANT';
INSERT INTO pack_carton_events_v22
 (tenant_id,plant_id,carton_id,event_no,action,spec_id,garment_id,note,
  command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
 VALUES ('$TENANT','TEST_PLANT','$CARTON_A',1,'ADD','$SPEC','$GARMENT',
  'Synthetic race test only','F39_RACE_CMD_A_123456','$HASH','test_a',
  'F39_RACE_PROOF_A_$TENANT','$HASH');
SELECT pg_sleep(3);
COMMIT;
SQL
WINNER_PID=$!
PIDS="$WINNER_PID"
ready=0
n=0
while [ "$n" -lt 80 ]; do
  STATE="$(psql_db -At -c "SELECT count(*) FROM pg_stat_activity WHERE datname='$DB' AND application_name='F39_WIN_$TENANT' AND state='active' AND query LIKE '%pg_sleep(3)%'")"
  if [ "$STATE" = 1 ]; then ready=1; break; fi
  kill -0 "$WINNER_PID" 2>/dev/null || break
  sleep 0.05
  n=$((n+1))
done
[ "$ready" -eq 1 ] || { cat "$DIR/winner.log" >&2; echo 'ERROR: first transaction did not insert' >&2; exit 1; }

if psql_db > "$DIR/loser.log" 2>&1 <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
SET LOCAL statement_timeout = '6000ms';
INSERT INTO pack_carton_events_v22
 (tenant_id,plant_id,carton_id,event_no,action,spec_id,garment_id,note,
  command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
 VALUES ('$TENANT','TEST_PLANT','$CARTON_B',1,'ADD','$SPEC','$GARMENT',
  'Synthetic race test only','F39_RACE_CMD_B_123456','$HASH','test_b',
  'F39_RACE_PROOF_B_$TENANT','$HASH');
COMMIT;
SQL
then
  echo 'ERROR: concurrent second carton insertion succeeded' >&2; exit 1
fi
if ! grep -q 'pack_one_carton_per_garment_v22' "$DIR/loser.log"; then
  cat "$DIR/loser.log" >&2
  echo 'ERROR: conflict was not rejected by physical garment uniqueness' >&2
  exit 1
fi
wait "$WINNER_PID"
PIDS=''
COUNT="$(psql_db -At -c "SELECT count(*) FROM pack_carton_events_v22 WHERE tenant_id='$TENANT' AND garment_id='$GARMENT'")"
[ "$COUNT" = 1 ] || { echo "ERROR: expected one committed packing row, got $COUNT" >&2; exit 1; }

# Connection death after a successful INSERT but before COMMIT must not
# persist a phantom garment. Test the actual table under PostgreSQL WAL.
CRASH_GARMENT="$(psql_db -At -c 'SELECT uuid_generate_v4()')"
psql_db -At > "$DIR/crash.log" 2>&1 <<SQL &
BEGIN;
SET LOCAL session_replication_role = replica;
SET LOCAL application_name = 'F39_CRASH_$TENANT';
INSERT INTO pack_carton_events_v22
 (tenant_id,plant_id,carton_id,event_no,action,spec_id,garment_id,note,
  command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
 VALUES ('$TENANT','TEST_PLANT','$(psql_db -At -c 'SELECT uuid_generate_v4()')',1,
  'ADD','$SPEC','$CRASH_GARMENT','Synthetic rollback drill only',
  'F39_CRASH_CMD_123456','$HASH','test_crash','F39_CRASH_PROOF_$TENANT','$HASH');
SELECT pg_sleep(30);
COMMIT;
SQL
CRASH_PID=$!
PIDS="$CRASH_PID"
BACKEND=''
n=0
while [ "$n" -lt 100 ]; do
  BACKEND="$(psql_db -At -c "SELECT pid FROM pg_stat_activity WHERE datname='$DB' AND application_name='F39_CRASH_$TENANT' AND state='active' AND query LIKE '%pg_sleep(30)%' LIMIT 1")"
  [ -n "$BACKEND" ] && break
  kill -0 "$CRASH_PID" 2>/dev/null || break
  sleep 0.05
  n=$((n+1))
done
[ -n "$BACKEND" ] || { cat "$DIR/crash.log" >&2; echo 'ERROR: transaction never reached crash barrier' >&2; exit 1; }
TERMINATED="$(psql_db -At -c "SELECT pg_terminate_backend($BACKEND)")"
[ "$TERMINATED" = t ] || { echo 'ERROR: PostgreSQL backend was not terminated' >&2; exit 1; }
if wait "$CRASH_PID"; then
  echo 'ERROR: terminated connection appeared to commit' >&2; exit 1
fi
PIDS=''
GHOST="$(psql_db -At -c "SELECT count(*) FROM pack_carton_events_v22 WHERE tenant_id='$TENANT' AND garment_id='$CRASH_GARMENT'")"
[ "$GHOST" = 0 ] || { echo "ERROR: $GHOST uncommitted rows survived backend death" >&2; exit 1; }

# Cross-tenant identical identity must not collide in the UNIQUE key.
psql_db >/dev/null <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
INSERT INTO pack_carton_events_v22
 (tenant_id,plant_id,carton_id,event_no,action,spec_id,garment_id,note,
  command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
 VALUES ('${TENANT}_OTHER','TEST_PLANT','$CARTON_B',1,'ADD','$SPEC','$GARMENT',
  'Synthetic tenant test only','F39_TENANT_CMD_123456','$HASH','test_other',
  'F39_TENANT_PROOF_$TENANT','$HASH');
COMMIT;
SQL
CROSS="$(psql_db -At -c "SELECT count(*) FROM pack_carton_events_v22 WHERE garment_id='$GARMENT' AND tenant_id IN ('$TENANT','${TENANT}_OTHER')")"
[ "$CROSS" = 2 ] || { echo "ERROR: cross-tenant index unexpectedly collides ($CROSS)" >&2; exit 1; }

# Never retain synthetic proof records, even in the disposable copy.
psql_db >/dev/null <<SQL
BEGIN;
SET LOCAL session_replication_role = replica;
DELETE FROM pack_carton_events_v22 WHERE tenant_id IN ('$TENANT','${TENANT}_OTHER');
COMMIT;
SQL
printf '%s\n' 'PASS v39: real partial unique index race, backend-death rollback, and tenant-scoped uniqueness'
