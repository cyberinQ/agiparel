-- Author: Sunil Jain, 2026-10-29.
-- Read-only schema preflight for dock replay/immutability/tenant isolation.
DO $$
DECLARE constraint_name text;
BEGIN
 IF NOT EXISTS (
  SELECT 1 FROM pg_constraint WHERE conrelid='dock_carton_scans_v34'::regclass
   AND contype='u' AND pg_get_constraintdef(oid) LIKE '%(tenant_id, manifest_id, carton_id)%'
 ) THEN RAISE EXCEPTION 'F40_MISSING_ONE_DOCK_SCAN_PER_CARTON'; END IF;
 IF NOT EXISTS (
  SELECT 1 FROM pg_constraint WHERE conrelid='dock_carton_scans_v34'::regclass
   AND contype='u' AND pg_get_constraintdef(oid) LIKE '%(tenant_id, gateway_id, event_id)%'
 ) THEN RAISE EXCEPTION 'F40_MISSING_GATEWAY_REPLAY_BOUNDARY'; END IF;
 IF NOT EXISTS (
  SELECT 1 FROM pg_constraint WHERE conrelid='dock_external_assertions_v35'::regclass
   AND contype='u' AND pg_get_constraintdef(oid) LIKE '%(tenant_id, plant_id, manifest_id, assertion_kind)%'
 ) THEN RAISE EXCEPTION 'F40_MISSING_SINGLE_EXTERNAL_DECISION_BOUNDARY'; END IF;
 IF NOT EXISTS (
  SELECT 1 FROM pg_constraint WHERE conrelid='dock_external_assertions_v35'::regclass
   AND contype='u' AND pg_get_constraintdef(oid) LIKE '%(tenant_id, plant_id, signer_id, event_id)%'
 ) THEN RAISE EXCEPTION 'F40_MISSING_EXTERNAL_SIGNER_REPLAY_BOUNDARY'; END IF;
 IF NOT EXISTS (
  SELECT 1 FROM pg_trigger WHERE tgrelid='dock_carton_scans_v34'::regclass
    AND tgname='dock_immutable_v34' AND tgenabled='O'
 ) OR NOT EXISTS (
  SELECT 1 FROM pg_trigger WHERE tgrelid='dock_external_assertions_v35'::regclass
    AND tgname='dock_external_immutable_v35' AND tgenabled='O'
 ) THEN RAISE EXCEPTION 'F40_IMMUTABILITY_TRIGGER_MISSING'; END IF;
 IF has_table_privilege('agiparel_app','dock_gateways_v34','INSERT') OR
    has_table_privilege('agiparel_app','dock_external_signers_v35','INSERT') OR
    has_table_privilege('agiparel_app','dock_external_signers_v35','UPDATE') THEN
    RAISE EXCEPTION 'F40_TRUST_ROOTS_NOT_PROVISIONER_ONLY';
 END IF;
 RAISE NOTICE 'PASS v40: dock replay constraints, external uniqueness, immutable evidence, provisioner-only signers';
END $$;
