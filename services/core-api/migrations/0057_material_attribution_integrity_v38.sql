-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.25 v38: manifest-bounded material attribution checks and unified operator
-- evidence report. READ ONLY. Human-reported millimeters are not sensor proof;
-- no global roll/trim balance, shipment posting, EDI delivery or finance credit.
BEGIN;

CREATE FUNCTION dock_material_trace_v38(t text, p text, m uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE AS $$
DECLARE
 staged bigint := 0;
 units bigint := 0;
 release_count bigint := 0;
 invalid_release_count bigint := 0;
 unbalanced_spreads bigint := 0;
 unsafe_issue_links bigint := 0;
 overallocated_issues bigint := 0;
 reported_mm numeric := 0;
 classified_mm numeric := 0;
 notes text[] := ARRAY[]::text[];
BEGIN
 SELECT count(*) INTO staged FROM pack_manifest_events_v22
 WHERE tenant_id=t AND plant_id=p AND manifest_id=m AND action='STAGE';
 SELECT count(*) INTO units FROM pack_manifest_events_v22 x
 JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
 WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON';
 -- A bounded scope is a hard failure, never a truncated positive report.
 IF staged<>1 OR units=0 OR units>5000 THEN
  RETURN jsonb_build_object('manifest_id',m::text,'staged',staged=1,
   'garments',units,'material_attribution_complete',false,
   'blockers',ARRAY['MISSING_OR_OVERSIZED_STAGED_MANIFEST'],
   'scope','HUMAN_CLASSIFIED_CUT_MATERIAL_ATTRIBUTION_ONLY',
   'global_fabric_stock_conserved',false,'trim_stock_conserved',false,
   'financial_ledger_conserved',false,'asn_network_delivery_proven',false,
   'shipment_credit',false,'authority','READ_ONLY_UNCERTIFIED_DIAGNOSTIC');
 END IF;

 WITH represented AS MATERIALIZED (
  SELECT DISTINCT g.release_id FROM pack_manifest_events_v22 x
  JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
  LEFT JOIN gen_garments_v18 g ON g.tenant_id=c.tenant_id AND g.id=c.garment_id AND g.plant_id=p
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
 ) SELECT count(*) INTO release_count FROM represented;
 IF release_count=0 OR release_count>500 THEN
  RETURN jsonb_build_object('manifest_id',m::text,'garments',units,
   'represented_releases',release_count,'material_attribution_complete',false,
   'blockers',ARRAY['MISSING_OR_OVERSIZED_CUT_RELEASE_SCOPE'],
   'scope','HUMAN_CLASSIFIED_CUT_MATERIAL_ATTRIBUTION_ONLY',
   'global_fabric_stock_conserved',false,'trim_stock_conserved',false,
   'financial_ledger_conserved',false,'asn_network_delivery_proven',false,
   'shipment_credit',false,'authority','READ_ONLY_UNCERTIFIED_DIAGNOSTIC');
 END IF;

 WITH represented AS MATERIALIZED (
  SELECT DISTINCT g.release_id FROM pack_manifest_events_v22 x
  JOIN pack_carton_events_v22 c ON c.tenant_id=x.tenant_id AND c.plant_id=x.plant_id
   AND c.carton_id=x.carton_id AND c.action='ADD'
  LEFT JOIN gen_garments_v18 g ON g.tenant_id=c.tenant_id AND g.id=c.garment_id AND g.plant_id=p
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
 ), checked AS (
  SELECT rr.release_id,
   r.id AS actual_release_id, sp.id AS report_id, b.id AS balance_id,
   sp.reported_consumption_mm AS reported,
   (b.used_part_mm + b.trim_waste_mm + b.damaged_mm) AS classified,
   (SELECT coalesce(sum(a.allocated_mm),0) FROM cut_spread_issue_allocations_v8 a
    WHERE a.tenant_id=t AND a.spread_report_id=r.spread_report_id) AS attributed,
   (SELECT count(*) FROM cut_spread_issue_allocations_v8 a
     LEFT JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id
     LEFT JOIN store_rolls s ON s.tenant_id=i.tenant_id AND s.id=i.roll_id
     WHERE a.tenant_id=t AND a.spread_report_id=r.spread_report_id
       AND (i.id IS NULL OR s.id IS NULL OR s.plant_id<>p)) AS invalid_issues,
   (SELECT count(*) FROM cut_spread_issue_allocations_v8 a
     JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id
     WHERE a.tenant_id=t AND a.spread_report_id=r.spread_report_id
       AND (SELECT coalesce(sum(other.allocated_mm),0) FROM cut_spread_issue_allocations_v8 other
         WHERE other.tenant_id=a.tenant_id AND other.issue_id=a.issue_id)>i.length_mm) AS overallocated
  FROM represented rr
  LEFT JOIN cut_sewing_releases_v8 r ON r.tenant_id=t AND r.plant_id=p AND r.id=rr.release_id
  LEFT JOIN cut_spread_reports_v8 sp ON sp.tenant_id=t AND sp.id=r.spread_report_id AND sp.lay_draft_id=r.lay_draft_id
  LEFT JOIN cut_consumption_reconciliations_v19 b ON b.tenant_id=t AND b.plant_id=p
    AND b.release_id=r.id AND b.spread_report_id=r.spread_report_id
 )
 SELECT count(*) FILTER (WHERE actual_release_id IS NULL OR report_id IS NULL OR balance_id IS NULL),
  count(*) FILTER (WHERE reported IS NULL OR classified IS NULL OR attributed<>reported
    OR classified<>reported),
  coalesce(sum(invalid_issues),0), coalesce(sum(overallocated),0),
  coalesce(sum(reported),0),coalesce(sum(classified),0)
 INTO invalid_release_count,unbalanced_spreads,unsafe_issue_links,overallocated_issues,
  reported_mm,classified_mm FROM checked;

 IF invalid_release_count<>0 THEN notes:=array_append(notes,'MISSING_SIGNED_RELEASE_OR_DUAL_CLASSIFICATION'); END IF;
 IF unbalanced_spreads<>0 THEN notes:=array_append(notes,'CUT_SPREAD_ALLOCATION_OR_CLASSIFICATION_MISMATCH'); END IF;
 IF unsafe_issue_links<>0 THEN notes:=array_append(notes,'MATERIAL_ISSUE_PLANT_OR_IDENTITY_MISMATCH'); END IF;
 IF overallocated_issues<>0 THEN notes:=array_append(notes,'MATERIAL_ISSUE_ATTRIBUTED_ABOVE_SIGNED_ISSUE'); END IF;
 IF coalesce((dock_lineage_audit_v37(t,p,m)->>'lineage_evidence_complete')::boolean,false) IS NOT TRUE THEN
  notes:=array_append(notes,'UPSTREAM_CUT_WIP_QC_LINEAGE_INCOMPLETE');
 END IF;
 RETURN jsonb_build_object('manifest_id',m::text,'garments',units,
  'represented_releases',release_count,'missing_release_or_classification',invalid_release_count,
  'unbalanced_spread_count',unbalanced_spreads,'cross_plant_or_missing_issues',unsafe_issue_links,
  'overallocated_issue_links',overallocated_issues,
  'reported_consumption_mm',reported_mm,'human_classified_consumption_mm',classified_mm,
  'material_attribution_complete',cardinality(notes)=0,'blockers',to_jsonb(notes),
  'scope','HUMAN_CLASSIFIED_CUT_MATERIAL_ATTRIBUTION_ONLY',
  'global_fabric_stock_conserved',false,'trim_stock_conserved',false,
  'financial_ledger_conserved',false,'asn_network_delivery_proven',false,
  'shipment_credit',false,'authority','READ_ONLY_UNCERTIFIED_DIAGNOSTIC');
END $$;

CREATE FUNCTION dock_integrity_report_v38(t text,p text,m uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE AS $$
DECLARE d jsonb; l jsonb; a jsonb; complete boolean;
BEGIN
 d := dock_diagnostics_v36(t,p,m);
 l := dock_lineage_audit_v37(t,p,m);
 a := dock_material_trace_v38(t,p,m);
 complete := coalesce((d->>'external_evidence_current')::boolean,false)
   AND coalesce((l->>'lineage_evidence_complete')::boolean,false)
   AND coalesce((a->>'material_attribution_complete')::boolean,false);
 RETURN jsonb_build_object('manifest_id',m::text,
  'evidence_review_complete',complete,'dock',d,'lineage',l,'material',a,
  'global_fabric_stock_conserved',false,'trim_stock_conserved',false,
  'financial_ledger_conserved',false,'asn_network_delivery_proven',false,
  'shipment_credit',false,'authority','READ_ONLY_OPERATOR_EVIDENCE_NOT_SHIPMENT_RELEASE');
END $$;

REVOKE ALL ON FUNCTION dock_material_trace_v38(text,text,uuid),
 dock_integrity_report_v38(text,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION dock_material_trace_v38(text,text,uuid),
 dock_integrity_report_v38(text,text,uuid) TO agiparel_app;
COMMIT;
