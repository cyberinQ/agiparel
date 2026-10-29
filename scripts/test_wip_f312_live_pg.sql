-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.12 read-only, tenant-RLS-checked inspection of an ACTUAL signed gateway event.
-- All parameters MUST originate from scanrelay audit of a genuine source envelope.
-- This SQL never inserts a scan, movement, carrier binding, incident, or QC event.
\set ON_ERROR_STOP on
BEGIN READ ONLY;
SET LOCAL statement_timeout = '10s';
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id', :'tenant_id', true);

WITH facts AS (
 SELECT
  current_user = 'agiparel_app' AS expected_role,
  (SELECT count(*) FROM sew_prod_gateways_v10 g
     WHERE g.tenant_id=:'tenant_id' AND g.plant_id=:'plant_id'
       AND g.gateway_id=:'gateway_id' AND g.station_id=:'station_id'
       AND g.enabled AND encode(g.public_key,'hex')=:'public_key_hex') = 1 AS enrolled_key_match,
  (SELECT count(*) FROM wip_gateway_scans_v20 s
     WHERE s.tenant_id=:'tenant_id' AND s.plant_id=:'plant_id'
       AND s.gateway_id=:'gateway_id' AND s.station_id=:'station_id'
       AND s.event_id=:'event_id' AND s.id::text=:'scan_id'
       AND s.part_id::text=:'part_id' AND s.carrier_id::text=:'carrier_id'
       AND s.epc_hex=:'epc_hex'
       AND s.event_sha256=:'event_sha256'
       AND s.signature_sha256=:'signature_sha256'
       AND s.evidence='SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF') = 1 AS immutable_scan_match,
  (SELECT count(*) FROM wip_gateway_scans_v20 s
     WHERE s.tenant_id=:'tenant_id' AND s.gateway_id=:'gateway_id'
       AND s.event_id=:'event_id') = 1 AS unique_replay_identity,
  (SELECT count(*) FROM wip_movements_v20 m
     WHERE m.tenant_id=:'tenant_id' AND m.scan_id::text=:'scan_id') AS signed_custody_movements,
  (SELECT count(*) FROM wip_exception_cases_v23 c
     WHERE c.tenant_id=:'tenant_id' AND c.part_id::text=:'part_id'
       AND NOT EXISTS (SELECT 1 FROM wip_exception_closures_v23 x
         WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id)) AS open_part_exceptions
)
SELECT expected_role, enrolled_key_match, immutable_scan_match,
       unique_replay_identity, signed_custody_movements, open_part_exceptions,
       'SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF' AS evidence_authority,
       false AS physically_certified
FROM facts;

WITH facts AS (
 SELECT
  current_user = 'agiparel_app' AS role_ok,
  EXISTS (SELECT 1 FROM sew_prod_gateways_v10 g WHERE g.tenant_id=:'tenant_id'
       AND g.plant_id=:'plant_id' AND g.gateway_id=:'gateway_id'
       AND g.station_id=:'station_id' AND g.enabled
       AND encode(g.public_key,'hex')=:'public_key_hex') AS key_ok,
  (SELECT count(*) FROM wip_gateway_scans_v20 s WHERE s.tenant_id=:'tenant_id'
       AND s.plant_id=:'plant_id' AND s.station_id=:'station_id'
       AND s.gateway_id=:'gateway_id' AND s.event_id=:'event_id'
       AND s.id::text=:'scan_id' AND s.part_id::text=:'part_id'
       AND s.carrier_id::text=:'carrier_id' AND s.epc_hex=:'epc_hex'
       AND s.event_sha256=:'event_sha256'
       AND s.signature_sha256=:'signature_sha256'
       AND s.evidence='SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF')=1 AS scan_ok,
  (SELECT count(*) FROM wip_gateway_scans_v20 s
     WHERE s.tenant_id=:'tenant_id' AND s.gateway_id=:'gateway_id'
       AND s.event_id=:'event_id')=1 AS unique_ok
)
SELECT (role_ok AND key_ok AND scan_ok AND unique_ok) AS f312_pass FROM facts \gset
\if :f312_pass
  \echo 'PASS F3.12: enrolled public key, signed event, receipt scan UUID and immutable PostgreSQL record match under tenant RLS; NOT PHYSICAL RF CERTIFICATION'
\else
  \echo 'FAIL F3.12: key enrollment or immutable event evidence mismatch; stop commissioning'
  SELECT 1/0 AS f312_audit_failure;
\endif
ROLLBACK;
