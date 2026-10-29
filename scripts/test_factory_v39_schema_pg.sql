-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Verifies deployed constraints and least-privilege boundaries. No new migration.
DO $$
DECLARE definition text; tbl text;
BEGIN
 SELECT pg_get_indexdef('public.pack_one_carton_per_garment_v22'::regclass) INTO definition;
 IF definition NOT ILIKE '%UNIQUE INDEX%' OR definition NOT ILIKE '%(tenant_id, garment_id)%'
 OR (position('WHERE' IN definition)=0 OR position('ADD' IN definition)=0) THEN
  RAISE EXCEPTION 'ERR_F39_PACKING_CROSS_CARTON_DUPLICATE_INDEX_MISSING: %',definition;
 END IF;
 FOREACH tbl IN ARRAY ARRAY[
  'pack_carton_events_v22','pack_manifest_events_v22',
  'dock_carton_scans_v34','dock_container_seals_v34',
  'dock_carrier_receipts_v34','dock_asn_drafts_v34',
  'dock_external_assertions_v35'] LOOP
  IF NOT EXISTS (SELECT 1 FROM pg_class WHERE oid=to_regclass('public.'||tbl)
      AND relrowsecurity AND relforcerowsecurity) THEN
   RAISE EXCEPTION 'ERR_F39_MISSING_FORCED_RLS: %',tbl;
  END IF;
  IF has_table_privilege('agiparel_app',tbl,'UPDATE') OR
     has_table_privilege('agiparel_app',tbl,'DELETE') OR
     EXISTS(SELECT 1 FROM pg_class c CROSS JOIN LATERAL
       aclexplode(coalesce(c.relacl,acldefault('r',c.relowner))) acl
       WHERE c.oid=to_regclass('public.'||tbl) AND acl.grantee=0
        AND acl.privilege_type='INSERT') THEN
   RAISE EXCEPTION 'ERR_F39_DOCK_EVIDENCE_MUTABLE_OR_PUBLIC: %',tbl;
  END IF;
 END LOOP;
 FOREACH tbl IN ARRAY ARRAY['dock_gateways_v34','dock_carrier_authorizations_v34',
   'dock_station_policies_v34','dock_external_signers_v35','dock_buyer_spec_bindings_v36'] LOOP
  IF has_table_privilege('agiparel_app',tbl,'INSERT') OR
     has_table_privilege('agiparel_app',tbl,'UPDATE') OR
     has_table_privilege('agiparel_app',tbl,'DELETE') THEN
   RAISE EXCEPTION 'ERR_F39_PROVISIONER_AUTHORITY_EXPOSED: %',tbl;
  END IF;
 END LOOP;
 RAISE NOTICE 'PASS v39 RLS + immutable evidence + provisioner-only trust roots + DB uniqueness';
END $$;
