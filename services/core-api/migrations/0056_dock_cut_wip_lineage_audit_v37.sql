-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.24 v37: bounded read-only cross-ledger audit of manifest garments, cut
-- consumption, effective part/QC readiness and outstanding WIP/jobwork holds.
-- No shipment posting, raw/trim stock, financial balance or carrier EDI proof.
BEGIN;

CREATE FUNCTION dock_lineage_audit_v37(t text, p text, m uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE AS $$
DECLARE
 cartons bigint := 0;
 units bigint := 0;
 unique_units bigint := 0;
 missing_identity bigint := 0;
 missing_cut_balance bigint := 0;
 invalid_qc_lineage bigint := 0;
 active_piece_holds bigint := 0;
 open_jobwork_parts bigint := 0;
 disputed_jobwork_parts bigint := 0;
 result_ok boolean := false;
 carton_ok boolean := false;
 notes text[] := ARRAY[]::text[];
BEGIN
 -- All queries execute under the caller's tenant RLS and an HTTP-level
 -- read-only REPEATABLE READ transaction. No privileged data access.
 SELECT count(*) INTO cartons FROM pack_manifest_events_v22 x
 WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON';

 SELECT count(*),count(DISTINCT c.garment_id) INTO units,unique_units
 FROM pack_manifest_events_v22 x
 JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
 WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON';

 -- Cap expensive per-piece examinations; a large manifest must be audited in
 -- explicitly bounded chunks in a future release, never silently truncated.
 IF cartons=0 OR units=0 OR cartons>5000 OR units>5000 THEN
  RETURN jsonb_build_object('manifest_id',m::text,'cartons',cartons,'garments',units,
   'lineage_evidence_complete',false,'blockers',ARRAY['MISSING_OR_OVERSIZED_MANIFEST'],
   'scope','CUT_TO_PACKING_EVIDENCE_ONLY','global_inventory_conserved',false,
   'trim_stock_conserved',false,'financial_ledger_conserved',false,
   'asn_network_delivery_proven',false,'shipment_credit',false,
   'authority','READ_ONLY_CROSS_LEDGER_DIAGNOSTIC');
 END IF;

 -- Count exact garment membership and current identity/plant scope.
 SELECT count(*) INTO missing_identity
 FROM pack_manifest_events_v22 x
 JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
 LEFT JOIN gen_garments_v18 g ON g.tenant_id=c.tenant_id AND g.id=c.garment_id
 WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
   AND (g.id IS NULL OR g.plant_id IS DISTINCT FROM p);

 -- A release-level human-classified spread conservation record must exist for
 -- every represented signed cutting release. This does NOT certify the fabric
 -- meter or infer global warehouse stock.
 SELECT count(*) INTO missing_cut_balance FROM (
  SELECT DISTINCT g.release_id FROM pack_manifest_events_v22 x
  JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
  JOIN gen_garments_v18 g ON g.tenant_id=c.tenant_id AND g.id=c.garment_id
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
   AND g.plant_id=p
 ) releases
 WHERE NOT EXISTS (SELECT 1 FROM cut_consumption_reconciliations_v19 b
  WHERE b.tenant_id=t AND b.plant_id=p AND b.release_id=releases.release_id);

 -- Re-evaluate effective accepted part/recut lineage and latest QC using the
 -- same finishing-station rule enforced by the original QC write path.
 SELECT count(*) INTO invalid_qc_lineage FROM (
  SELECT DISTINCT c.garment_id FROM pack_manifest_events_v22 x
  JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
 ) u WHERE NOT EXISTS (
  SELECT 1 FROM qc_garment_events_v21 q
  JOIN qc_garment_specs_v21 s ON s.tenant_id=q.tenant_id AND s.id=q.spec_id
  WHERE q.tenant_id=t AND q.plant_id=p AND q.garment_id=u.garment_id
    AND q.action='QUALITY_RELEASE' AND s.plant_id=p AND s.enabled
    AND q.event_no=(SELECT max(latest.event_no) FROM qc_garment_events_v21 latest
      WHERE latest.tenant_id=q.tenant_id AND latest.garment_id=q.garment_id)
    AND qc_garment_parts_ready_v21(t,u.garment_id,s.finish_station)
    AND NOT garment_needle_safety_blocked_v32(t,u.garment_id)
 );

 SELECT count(*) INTO active_piece_holds FROM (
  SELECT DISTINCT c.garment_id FROM pack_manifest_events_v22 x
  JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
 ) u WHERE wip_piece_active_hold_v26(t,u.garment_id);

 SELECT count(*) FILTER(WHERE jobwork_part_offsite_v28(t,k.id)),
        count(*) FILTER(WHERE jobwork_part_disputed_v29(t,k.id))
 INTO open_jobwork_parts,disputed_jobwork_parts
 FROM pack_manifest_events_v22 x
 JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
 JOIN gen_cut_parts_v18 k ON k.tenant_id=c.tenant_id AND k.garment_id=c.garment_id
 WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON';

 carton_ok:=dock_carton_reconciled_v35(t,p,m);
 IF NOT carton_ok THEN notes:=array_append(notes,'CARTON_SCAN_OR_QC_MISMATCH'); END IF;
 IF units<>unique_units THEN notes:=array_append(notes,'DUPLICATE_GARMENT_MEMBERSHIP'); END IF;
 IF missing_identity<>0 THEN notes:=array_append(notes,'MISSING_GARMENT_IDENTITY'); END IF;
 IF missing_cut_balance<>0 THEN notes:=array_append(notes,'UNRECONCILED_CUT_RELEASE'); END IF;
 IF invalid_qc_lineage<>0 THEN notes:=array_append(notes,'PART_LINEAGE_OR_QC_NOT_CURRENT'); END IF;
 IF active_piece_holds<>0 THEN notes:=array_append(notes,'ACTIVE_PIECE_HOLD'); END IF;
 IF open_jobwork_parts<>0 THEN notes:=array_append(notes,'OFFSITE_JOBWORK_PART'); END IF;
 IF disputed_jobwork_parts<>0 THEN notes:=array_append(notes,'DISPUTED_JOBWORK_PART'); END IF;
 result_ok:=cardinality(notes)=0;
 RETURN jsonb_build_object(
  'manifest_id',m::text,'cartons',cartons,'garments',units,
  'distinct_garments',unique_units,'missing_identity',missing_identity,
  'unreconciled_cut_releases',missing_cut_balance,
  'invalid_part_or_qc_lineage',invalid_qc_lineage,
  'active_piece_holds',active_piece_holds,'offsite_jobwork_parts',open_jobwork_parts,
  'disputed_jobwork_parts',disputed_jobwork_parts,
  'carton_scan_qc_current',carton_ok,'lineage_evidence_complete',result_ok,
  'blockers',to_jsonb(notes),'scope','CUT_TO_PACKING_EVIDENCE_ONLY',
  'global_inventory_conserved',false,'trim_stock_conserved',false,
  'financial_ledger_conserved',false,'asn_network_delivery_proven',false,
  'shipment_credit',false,'authority','READ_ONLY_CROSS_LEDGER_DIAGNOSTIC');
END $$;
REVOKE ALL ON FUNCTION dock_lineage_audit_v37(text,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION dock_lineage_audit_v37(text,text,uuid) TO agiparel_app;
COMMIT;
