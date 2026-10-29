#!/bin/sh
# Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
# v41: concurrent restricted-role RLS reads, enrollment revocation visibility.
# ONLY on disposable PostgreSQL 16. No signature or live hardware claims.
set -eu
DB="${1:-${PGDATABASE:-}}"
case "$DB" in
  agiparel_v39_test_*|agiparel_v41_test_*) ;;
  agiparel_dev) [ "${CI:-}" = true ] || { echo 'CI-only agiparel_dev' >&2; exit 2; } ;;
  *) echo 'Refusing non-disposable database' >&2; exit 2 ;;
esac
[ "${AGIPAREL_TEST_DB_DISPOSABLE:-}" = YES ] || { echo 'Disposable-db opt-in required' >&2; exit 2; }
DBUSER="${PGUSER:-${POSTGRES_USER:-}}"
[ -n "$DBUSER" ] || { echo 'PGUSER or POSTGRES_USER required' >&2; exit 2; }
case "${AGIPAREL_V41_WORKERS:-12}" in
  ''|*[!0-9]*) echo 'Invalid worker count' >&2; exit 2 ;;
esac
N="${AGIPAREL_V41_WORKERS:-12}"
[ "$N" -ge 4 ] && [ "$N" -le 32 ] || { echo 'Worker count must be 4..32' >&2; exit 2; }
WORK="$(mktemp -d)"; PIDS=''; PREFIX="F41_$$_"
cleanup() {
  for pid in $PIDS; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$WORK"
}
trap cleanup EXIT HUP INT TERM
psql_db() { psql -X -v ON_ERROR_STOP=1 -U "$DBUSER" -d "$DB" "$@"; }
A="${PREFIX}A"; B="${PREFIX}B"
# Real provisioner writes for distinct tenants with intentionally identical
# gateway IDs; no synthetic evidence rows or trigger bypass are involved.
psql_db > /dev/null <<SQL
INSERT INTO tenants(id,name) VALUES ('$A','v41 A'),('$B','v41 B');
INSERT INTO dock_gateways_v34(tenant_id,plant_id,gateway_id,public_key,enabled,approved_by)
VALUES ('$A','F41_PLANT_A','F41-GATEWAY',decode(repeat('ab',32),'hex'),true,'F41_PROVISIONER'),
       ('$B','F41_PLANT_B','F41-GATEWAY',decode(repeat('cd',32),'hex'),true,'F41_PROVISIONER');
INSERT INTO dock_external_signers_v35(tenant_id,plant_id,signer_id,party_kind,party_code,public_key,enabled,approved_by)
VALUES ('$A','F41_PLANT_A','F41-SIGNER','BUYER','F41-BUYER-A',decode(repeat('ef',32),'hex'),true,'F41_PROVISIONER'),
       ('$B','F41_PLANT_B','F41-SIGNER','BUYER','F41-BUYER-B',decode(repeat('01',32),'hex'),true,'F41_PROVISIONER');
SQL
# Each worker checks 2 trust-root tables as agiparel_app with a separate
# transaction-local tenant GUC. Detect wrong-tenant rows and empty-tenant leaks.
i=1
while [ "$i" -le "$N" ]; do
  if [ $((i % 2)) -eq 0 ]; then OWN="$A"; OTHER="$B"; else OWN="$B"; OTHER="$A"; fi
  (
    psql_db > "$WORK/read_$i.log" 2>&1 <<SQL
BEGIN READ ONLY;
SET LOCAL ROLE agiparel_app;
SET LOCAL statement_timeout='5000ms';
SELECT set_config('app.current_tenant_id','$OWN',true);
DO \$\$ DECLARE n integer; BEGIN
  SELECT count(*) INTO n FROM dock_gateways_v34 WHERE enabled;
  IF n<>1 THEN RAISE EXCEPTION 'F41_GATEWAY_RLS_%: %','$i',n; END IF;
  SELECT count(*) INTO n FROM dock_external_signers_v35 WHERE enabled;
  IF n<>1 THEN RAISE EXCEPTION 'F41_SIGNER_RLS_%: %','$i',n; END IF;
  IF EXISTS(SELECT 1 FROM dock_gateways_v34 WHERE tenant_id='$OTHER') OR
     EXISTS(SELECT 1 FROM dock_external_signers_v35 WHERE tenant_id='$OTHER') THEN
    RAISE EXCEPTION 'F41_CROSS_TENANT_LEAK_%','$i';
  END IF;
END \$\$;
SELECT set_config('app.current_tenant_id','',true);
DO \$\$ BEGIN
 IF EXISTS(SELECT 1 FROM dock_gateways_v34) OR EXISTS(SELECT 1 FROM dock_external_signers_v35)
 THEN RAISE EXCEPTION 'F41_UNSCOPED_LEAK_%','$i'; END IF;
END \$\$;
COMMIT;
SQL
  ) &
  PIDS="$PIDS $!"
  i=$((i + 1))
done
FAIL=0
for pid in $PIDS; do wait "$pid" || FAIL=1; done
PIDS=''
if [ "$FAIL" -ne 0 ]; then
  for f in "$WORK"/read_*.log; do grep -q 'ERROR:' "$f" && cat "$f" >&2 || true; done
  echo 'Concurrent app-role tenant reads failed' >&2; exit 1
fi
# Revoke one root, verify its tenant sees revocation and the other remains
# unaffected. These are fresh transaction snapshots, not live device probes.
psql_db > /dev/null -c "UPDATE dock_gateways_v34 SET enabled=false WHERE tenant_id='$A' AND gateway_id='F41-GATEWAY'"
psql_db > /dev/null <<SQL
BEGIN READ ONLY;
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id','$A',true);
DO \$\$ BEGIN
 IF (SELECT count(*) FROM dock_gateways_v34 WHERE enabled)<>0 THEN
 RAISE EXCEPTION 'F41_REVOKED_GATEWAY_STILL_CURRENT'; END IF;
END \$\$;
SELECT set_config('app.current_tenant_id','$B',true);
DO \$\$ BEGIN
 IF (SELECT count(*) FROM dock_gateways_v34 WHERE enabled)<>1 THEN
 RAISE EXCEPTION 'F41_OTHER_TENANT_REVOKED'; END IF;
END \$\$;
COMMIT;
SQL
# The installer drops this disposable database. No live tenants are modified.
printf 'PASS v41: %s concurrent app-role scoped reads, null-tenant denial, key revocation scoped to one tenant\n' "$N"
