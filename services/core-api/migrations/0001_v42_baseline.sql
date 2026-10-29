-- Author: Sunil Jain, 2026-10-29.
-- Agiparel v42 canonical baseline: PostgreSQL 16 generated from historical migrations.
-- Role passwords are provisioned separately; this file is not a migration concatenation.
--
-- PostgreSQL database dump
--

\restrict ofxAhRyiDpoZ8iRglI0hFFSEaR8dxFWzFry44RcDEffpQl6KH9fk0OTklVvhxDA

-- Dumped from database version 16.15
-- Dumped by pg_dump version 16.15

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: s01_identity; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA s01_identity;


--
-- Name: s02_device; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA s02_device;


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: apply_gen_carrier_event_v18(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.apply_gen_carrier_event_v18() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
DECLARE c public.gen_carriers_v18%ROWTYPE; active public.gen_carrier_active_v18%ROWTYPE; actual_plant text;
BEGIN
 -- Shares the original roll/bundle EPC namespace lock; both old and new
 -- writers participate, preventing a concurrent cross-table double binding.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT * INTO c FROM public.gen_carriers_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.carrier_id FOR UPDATE;
 IF NOT FOUND OR (NEW.action='BIND' AND NOT c.enabled) THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_NOT_ENROLLED'; END IF;
 CASE NEW.target_type
  WHEN 'GARMENT' THEN SELECT plant_id INTO actual_plant FROM public.gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.target_id;
  WHEN 'CUT_PART' THEN SELECT g.plant_id INTO actual_plant FROM public.gen_cut_parts_v18 p JOIN public.gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.target_id;
  WHEN 'BUNDLE' THEN SELECT plant_id INTO actual_plant FROM public.gen_bundles_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.target_id;
 END CASE;
 IF actual_plant IS NULL OR actual_plant<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_TARGET_SCOPE'; END IF;
 SELECT * INTO active FROM public.gen_carrier_active_v18 WHERE tenant_id=NEW.tenant_id AND carrier_id=NEW.carrier_id;
 IF NEW.action='BIND' THEN
  IF active.carrier_id IS NOT NULL OR
   EXISTS (SELECT 1 FROM public.cut_bundle_scan_witnesses_v8 w WHERE w.tenant_id=NEW.tenant_id AND w.epc_hex=c.epc_hex) OR
   EXISTS (SELECT 1 FROM public.store_roll_rfid_bindings_v7 r WHERE r.tenant_id=NEW.tenant_id AND r.epc_hex=c.epc_hex)
  THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_ALREADY_ASSIGNED_OR_LEGACY'; END IF;
  INSERT INTO public.gen_carrier_active_v18(tenant_id,carrier_id,plant_id,target_type,target_id,bound_event_id)
   VALUES(NEW.tenant_id,NEW.carrier_id,NEW.plant_id,NEW.target_type,NEW.target_id,NEW.id);
 ELSE
  IF active.carrier_id IS NULL OR active.target_type<>NEW.target_type OR active.target_id<>NEW.target_id OR active.plant_id<>NEW.plant_id THEN
   RAISE EXCEPTION 'ERR_GEN_CARRIER_RETIRE_MISMATCH'; END IF;
  DELETE FROM public.gen_carrier_active_v18 WHERE tenant_id=NEW.tenant_id AND carrier_id=NEW.carrier_id;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: check_cut_spread_coverage_v8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.check_cut_spread_coverage_v8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE covered_mm bigint;
BEGIN
 SELECT COALESCE(sum(a.allocated_mm),0) INTO covered_mm
 FROM cut_spread_issue_allocations_v8 a
 WHERE a.tenant_id=NEW.tenant_id AND a.spread_report_id=NEW.id;
 IF covered_mm<>NEW.reported_consumption_mm THEN
   RAISE EXCEPTION 'ERR_CUT_SPREAD_CONSUMPTION_NOT_COVERED';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: check_gen_part_coverage_v18(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.check_gen_part_coverage_v18() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE expected integer; actual integer;
BEGIN
 SELECT COALESCE(sum(b.pieces_per_garment),0)::integer INTO expected FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position
    AND NEW.ply_number BETWEEN b.ply_start AND b.ply_end;
 SELECT count(*)::integer INTO actual FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.id;
 IF expected=0 OR actual<>expected THEN RAISE EXCEPTION 'ERR_GEN_PART_COVERAGE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: check_material_reservation_v5(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.check_material_reservation_v5() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    roll_remaining BIGINT;
    roll_owner VARCHAR(64);
    roll_state TEXT;
    roll_quality TEXT;
    open_mm BIGINT;
BEGIN
    SELECT ROUND(remaining_length_m*1000)::BIGINT, reserved_order_id, status, inspection_status
    INTO roll_remaining,roll_owner,roll_state,roll_quality
    FROM store_rolls WHERE id=NEW.roll_id AND tenant_id=NEW.tenant_id FOR UPDATE;
    IF NOT FOUND OR roll_quality <> 'PASS' OR roll_state IN ('QUARANTINE','DEPLETED') THEN
        RAISE EXCEPTION 'ERR_MATERIAL_NOT_ELIGIBLE';
    END IF;
    IF roll_owner IS NOT NULL AND roll_owner <> NEW.order_id THEN
        RAISE EXCEPTION 'ERR_SACROSANCT_RESERVATION';
    END IF;
    SELECT COALESCE(SUM(reserved_mm-issued_mm-released_mm),0)
      INTO open_mm FROM material_reservations_v5
      WHERE tenant_id=NEW.tenant_id AND roll_id=NEW.roll_id;
    IF open_mm > roll_remaining THEN
        RAISE EXCEPTION 'ERR_MATERIAL_OVERRESERVED';
    END IF;
    RETURN NEW;
END $$;


--
-- Name: deny_trim_ledger_mutation_v24(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.deny_trim_ledger_mutation_v24() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'ERR_F314_APPEND_ONLY'; END $$;


--
-- Name: dock_buyer_binding_guard_v36(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_buyer_binding_guard_v36() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF TG_OP='UPDATE' THEN
  IF (OLD.tenant_id,OLD.plant_id,OLD.spec_id,OLD.buyer_code,OLD.contract_reference)
    IS DISTINCT FROM
    (NEW.tenant_id,NEW.plant_id,NEW.spec_id,NEW.buyer_code,NEW.contract_reference)
  THEN RAISE EXCEPTION 'ERR_F36_BUYER_BINDING_IDENTITY_IMMUTABLE'; END IF;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pack_carton_specs_v22 s
  WHERE s.tenant_id=NEW.tenant_id AND s.plant_id=NEW.plant_id AND s.id=NEW.spec_id)
 THEN RAISE EXCEPTION 'ERR_F36_BUYER_SPEC_PLANT_MISMATCH'; END IF;
 RETURN NEW;
END $$;


--
-- Name: dock_buyer_scope_current_v36(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_buyer_scope_current_v36(t text, p text, m uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT EXISTS (
  SELECT 1 FROM pack_manifest_events_v22 o
  JOIN dock_buyer_spec_bindings_v36 b ON b.tenant_id=o.tenant_id
    AND b.plant_id=o.plant_id AND b.spec_id=o.spec_id AND b.enabled
  JOIN dock_external_assertions_v35 x ON x.tenant_id=o.tenant_id
    AND x.plant_id=o.plant_id AND x.manifest_id=o.manifest_id
    AND x.assertion_kind='BUYER_LOT_REVIEW' AND x.decision='REVIEWED_ACCEPT'
    AND x.party_kind='BUYER' AND x.party_code=b.buyer_code
  JOIN dock_external_signers_v35 s ON s.tenant_id=x.tenant_id
    AND s.plant_id=x.plant_id AND s.signer_id=x.signer_id
    AND s.enabled AND s.party_kind='BUYER' AND s.party_code=b.buyer_code
    AND encode(digest(s.public_key,'sha256'),'hex')=x.signer_key_sha256
  WHERE o.tenant_id=t AND o.plant_id=p AND o.manifest_id=m AND o.action='OPEN'
 )
$$;


--
-- Name: dock_carton_reconciled_v35(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_carton_reconciled_v35(t text, p text, m uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT
 (SELECT count(*) FROM pack_manifest_events_v22
   WHERE tenant_id=t AND plant_id=p AND manifest_id=m AND action='STAGE')=1
 AND (SELECT count(*) FROM pack_manifest_events_v22
   WHERE tenant_id=t AND plant_id=p AND manifest_id=m AND action='OPEN')=1
 AND EXISTS (SELECT 1 FROM pack_manifest_events_v22
   WHERE tenant_id=t AND plant_id=p AND manifest_id=m AND action='ADD_CARTON')
 AND NOT EXISTS (
  SELECT 1 FROM pack_manifest_events_v22 a
  WHERE a.tenant_id=t AND a.plant_id=p AND a.manifest_id=m AND a.action='ADD_CARTON'
  AND (
    (SELECT count(*) FROM pack_carton_events_v22 c WHERE c.tenant_id=t AND c.plant_id=p
      AND c.carton_id=a.carton_id AND c.action='OPEN')<>1
    OR (SELECT count(*) FROM pack_carton_events_v22 c WHERE c.tenant_id=t AND c.plant_id=p
      AND c.carton_id=a.carton_id AND c.action='SEAL')<>1
    OR NOT EXISTS (SELECT 1 FROM pack_carton_events_v22 c WHERE c.tenant_id=t AND c.plant_id=p
      AND c.carton_id=a.carton_id AND c.action='ADD')
    OR EXISTS (SELECT 1 FROM pack_carton_events_v22 c WHERE c.tenant_id=t AND c.plant_id=p
      AND c.carton_id=a.carton_id AND c.action='ADD'
      AND ((SELECT q.action FROM qc_garment_events_v21 q
        WHERE q.tenant_id=c.tenant_id AND q.plant_id=c.plant_id AND q.garment_id=c.garment_id
        ORDER BY q.event_no DESC LIMIT 1) IS DISTINCT FROM 'QUALITY_RELEASE'
        OR garment_needle_safety_blocked_v32(c.tenant_id,c.garment_id)))
    OR NOT EXISTS (
     SELECT 1 FROM dock_carton_scans_v34 d JOIN dock_gateways_v34 g
       ON g.tenant_id=d.tenant_id AND g.plant_id=d.plant_id
       AND g.gateway_id=d.gateway_id AND g.enabled
       AND encode(digest(g.public_key,'sha256'),'hex')=d.gateway_key_sha256
     JOIN pack_carton_events_v22 opener ON opener.tenant_id=d.tenant_id
       AND opener.plant_id=d.plant_id AND opener.carton_id=d.carton_id
       AND opener.action='OPEN' AND opener.carton_code=d.carton_code
     WHERE d.tenant_id=t AND d.plant_id=p AND d.manifest_id=m AND d.carton_id=a.carton_id
    )
  )
 )
 AND (SELECT count(*) FROM dock_carton_scans_v34 d
     WHERE d.tenant_id=t AND d.plant_id=p AND d.manifest_id=m)
     = (SELECT count(*) FROM pack_manifest_events_v22 a
     WHERE a.tenant_id=t AND a.plant_id=p AND a.manifest_id=m AND a.action='ADD_CARTON')
$$;


--
-- Name: dock_diagnostics_v36(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_diagnostics_v36(t text, p text, m uuid) RETURNS jsonb
    LANGUAGE sql
    AS $$
 SELECT jsonb_build_object(
  'manifest_id',m::text,
  'staged', EXISTS(SELECT 1 FROM pack_manifest_events_v22 x WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='STAGE'),
  'cartons_expected',(SELECT count(*) FROM pack_manifest_events_v22 x WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'),
  'carton_scans_recorded',(SELECT count(*) FROM dock_carton_scans_v34 x WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m),
  'current_safety_aql_qc', dock_manifest_safe_v34(t,p,m),
  'carton_identity_conserved',dock_carton_reconciled_v35(t,p,m),
  'internal_custody_current',dock_evidence_current_v34(t,p,m),
  'contract_bound_buyer_current',dock_buyer_scope_current_v36(t,p,m),
  'external_evidence_current',dock_external_current_v35(t,p,m),
  'blockers', to_jsonb(array_remove(ARRAY[
    CASE WHEN NOT dock_manifest_safe_v34(t,p,m) THEN 'SAFETY_AQL_QC_OR_MANIFEST'::text END,
    CASE WHEN NOT dock_carton_reconciled_v35(t,p,m) THEN 'CARTON_OR_READER_RECONCILIATION'::text END,
    CASE WHEN NOT dock_evidence_current_v34(t,p,m) THEN 'DOCK_CUSTODY_INCOMPLETE_OR_REVOKED'::text END,
    CASE WHEN NOT dock_buyer_scope_current_v36(t,p,m) THEN 'BUYER_CONTRACT_SCOPE_OR_REVIEW_MISSING'::text END,
    CASE WHEN NOT dock_external_current_v35(t,p,m) THEN 'EXTERNAL_ASSERTIONS_INCOMPLETE_OR_REVOKED'::text END
  ],NULL)),
  'asn_network_delivery_proven',false,
  'global_inventory_conserved',false,
  'shipment_credit',false,
  'authority','READ_ONLY_DIAGNOSTICS_NOT_SHIPMENT_RELEASE'
 )
$$;


--
-- Name: dock_evidence_current_v34(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_evidence_current_v34(t text, p text, m uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT dock_manifest_safe_v34(t,p,m)
 AND EXISTS(SELECT 1 FROM dock_station_policies_v34 x
  WHERE x.tenant_id=t AND x.plant_id=p AND x.enabled)
 AND EXISTS(SELECT 1 FROM dock_container_seals_v34 s
  WHERE s.tenant_id=t AND s.plant_id=p AND s.manifest_id=m)
 AND EXISTS(SELECT 1 FROM dock_asn_drafts_v34 a
  JOIN dock_carrier_receipts_v34 r ON r.tenant_id=a.tenant_id AND r.id=a.carrier_receipt_id
  JOIN dock_carrier_authorizations_v34 auth ON auth.tenant_id=r.tenant_id AND auth.plant_id=r.plant_id
   AND auth.carrier_code=r.carrier_code AND auth.idp_issuer=r.actor_issuer AND auth.idp_subject=r.actor_subject
  WHERE a.tenant_id=t AND a.plant_id=p AND a.manifest_id=m AND auth.enabled)
 AND (SELECT count(*) FROM dock_carton_scans_v34 x
  JOIN dock_gateways_v34 g ON g.tenant_id=x.tenant_id AND g.gateway_id=x.gateway_id
   AND g.plant_id=x.plant_id AND g.enabled
   AND encode(digest(g.public_key,'sha256'),'hex')=x.gateway_key_sha256
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m)=
  (SELECT count(*) FROM pack_manifest_events_v22 e
   WHERE e.tenant_id=t AND e.plant_id=p AND e.manifest_id=m AND e.action='ADD_CARTON')
$$;


--
-- Name: dock_external_current_v35(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_external_current_v35(t text, p text, m uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT dock_evidence_current_v34(t,p,m) AND dock_carton_reconciled_v35(t,p,m)
 AND EXISTS (
  SELECT 1 FROM dock_external_assertions_v35 x JOIN dock_external_signers_v35 s
   ON s.tenant_id=x.tenant_id AND s.plant_id=x.plant_id AND s.signer_id=x.signer_id
   AND s.enabled AND s.party_kind='CARRIER' AND s.party_code=x.party_code
   AND encode(digest(s.public_key,'sha256'),'hex')=x.signer_key_sha256
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m
   AND x.assertion_kind='CARRIER_ASN_ACK' AND x.decision='ACKNOWLEDGED'
 )
 AND dock_buyer_scope_current_v36(t,p,m)
$$;


--
-- Name: dock_integrity_report_v38(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_integrity_report_v38(t text, p text, m uuid) RETURNS jsonb
    LANGUAGE plpgsql
    AS $$
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


--
-- Name: dock_lineage_audit_v37(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_lineage_audit_v37(t text, p text, m uuid) RETURNS jsonb
    LANGUAGE plpgsql
    AS $$
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


--
-- Name: dock_manifest_safe_v34(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_manifest_safe_v34(t text, p text, m uuid) RETURNS boolean
    LANGUAGE plpgsql
    AS $$
DECLARE staged bigint; cartons bigint; invalid bigint; plans bigint; approved bigint;
BEGIN
 SELECT count(*) FILTER(WHERE action='STAGE'),count(*) FILTER(WHERE action='ADD_CARTON')
 INTO staged,cartons FROM pack_manifest_events_v22
 WHERE tenant_id=t AND plant_id=p AND manifest_id=m;
 IF staged<>1 OR cartons=0 OR NOT EXISTS(
   SELECT 1 FROM pack_manifest_events_v22 e WHERE e.tenant_id=t AND e.plant_id=p
    AND e.manifest_id=m AND e.action='OPEN') THEN RETURN false; END IF;
 SELECT count(*) INTO invalid FROM pack_manifest_events_v22 x
 WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
 AND (NOT EXISTS (SELECT 1 FROM pack_carton_events_v22 c
      WHERE c.tenant_id=t AND c.plant_id=p AND c.carton_id=x.carton_id AND c.action='SEAL')
   OR NOT EXISTS (SELECT 1 FROM pack_carton_events_v22 c
      WHERE c.tenant_id=t AND c.plant_id=p AND c.carton_id=x.carton_id AND c.action='ADD')
   OR EXISTS (SELECT 1 FROM pack_carton_events_v22 c
      WHERE c.tenant_id=t AND c.plant_id=p AND c.carton_id=x.carton_id AND c.action='ADD'
       AND (garment_needle_safety_blocked_v32(c.tenant_id,c.garment_id)
        OR (SELECT q.action FROM qc_garment_events_v21 q
            WHERE q.tenant_id=t AND q.plant_id=p AND q.garment_id=c.garment_id
            ORDER BY q.event_no DESC LIMIT 1) IS DISTINCT FROM 'QUALITY_RELEASE')));
 IF invalid<>0 THEN RETURN false; END IF;
 -- Require a current buyer-plan review, not merely an old/stale recommendation.
 SELECT count(*),count(*) FILTER(WHERE EXISTS(
    SELECT 1 FROM buyer_aql_reviews_v33 r
    JOIN buyer_aql_review_rules_v33 v ON v.tenant_id=r.tenant_id AND v.plan_id=r.plan_id AND v.enabled
    JOIN documents d ON d.tenant_id=v.tenant_id AND d.id=v.source_document_id
    WHERE r.tenant_id=b.tenant_id AND r.plan_id=b.id AND r.decision='INTERNAL_RECOMMEND_ACCEPT'
     AND d.status='APPROVED' AND upper(d.issuer)='CUSTOMER' AND d.file_checksum=v.source_sha256))
 INTO plans,approved
 FROM buyer_aql_plans_v32 b JOIN pack_carton_specs_v22 s
 ON s.tenant_id=b.tenant_id AND s.lay_draft_id=b.lay_draft_id
 WHERE s.tenant_id=t AND s.plant_id=p AND s.enabled AND s.id=(
  SELECT e.spec_id FROM pack_manifest_events_v22 e WHERE e.tenant_id=t AND e.plant_id=p
   AND e.manifest_id=m AND e.action='OPEN' LIMIT 1)
 AND b.plant_id=p AND b.enabled
 AND b.revision=(SELECT max(z.revision) FROM buyer_aql_plans_v32 z
 WHERE z.tenant_id=b.tenant_id AND z.plant_id=b.plant_id AND z.lot_code=b.lot_code);
 RETURN plans>0 AND approved=plans;
END $$;


--
-- Name: dock_material_trace_v38(text, text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.dock_material_trace_v38(t text, p text, m uuid) RETURNS jsonb
    LANGUAGE plpgsql
    AS $$
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


--
-- Name: enforce_roll_single_order_v7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.enforce_roll_single_order_v7() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM 1 FROM store_rolls WHERE tenant_id=NEW.tenant_id AND id=NEW.roll_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'ERR_ROLL_NOT_FOUND'; END IF;
 IF EXISTS(SELECT 1 FROM material_reservations_v5
       WHERE tenant_id=NEW.tenant_id AND roll_id=NEW.roll_id AND order_id<>NEW.order_id)
    OR EXISTS(SELECT 1 FROM material_issues_v5
       WHERE tenant_id=NEW.tenant_id AND roll_id=NEW.roll_id AND order_id<>NEW.order_id) THEN
  RAISE EXCEPTION 'ERR_SACROSANCT_ROLL_ORDER';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: garment_needle_safety_blocked_v32(text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.garment_needle_safety_blocked_v32(t text, g uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT EXISTS(SELECT 1 FROM needle_safety_events_v32 e
 WHERE e.tenant_id=t AND e.garment_id=g AND e.event_kind IN('NEEDLE_BREAK','DETECTOR_CHALLENGE_FAIL','DETECTOR_REJECT')
 AND NOT EXISTS(SELECT 1 FROM needle_safety_clearances_v33 c WHERE c.tenant_id=e.tenant_id AND c.incident_id=e.id))
$$;


--
-- Name: guard_buyer_aql_plan_v32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_buyer_aql_plan_v32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE d documents%ROWTYPE; n integer; prior integer;
BEGIN
 SELECT * INTO d FROM documents WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF d.id IS NULL OR d.status<>'APPROVED' OR upper(d.issuer)<>'CUSTOMER' OR
  d.file_checksum<>NEW.source_sha256 OR
  d.document_type NOT IN ('TECH_PACK','BUYER_SPEC','BUYER_AQL_PLAN','AQL_INSPECTION_PLAN')
 THEN RAISE EXCEPTION 'ERR_F319A_BUYER_SOURCE'; END IF;
 SELECT count(DISTINCT sampled.garment_id) INTO n FROM unnest(NEW.sample_garment_ids) AS sampled(garment_id);
 IF n<>cardinality(NEW.sample_garment_ids) OR EXISTS (
   SELECT 1 FROM unnest(NEW.sample_garment_ids) AS sampled(garment_id) LEFT JOIN gen_garments_v18 g
    ON g.tenant_id=NEW.tenant_id AND g.id=sampled.garment_id
    WHERE g.id IS NULL OR g.plant_id<>NEW.plant_id OR g.lay_draft_id<>NEW.lay_draft_id)
 THEN RAISE EXCEPTION 'ERR_F319A_SAMPLE_GENEALOGY'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F319A:PLAN:'||NEW.plant_id||':'||NEW.lot_code));
 SELECT max(revision) INTO prior FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id
  AND plant_id=NEW.plant_id AND lot_code=NEW.lot_code;
 IF (prior IS NULL AND NEW.revision<>1) OR (prior IS NOT NULL AND NEW.revision<>prior+1)
 THEN RAISE EXCEPTION 'ERR_F319A_REVISION'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_buyer_aql_review_v33(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_buyer_aql_review_v33() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE p buyer_aql_plans_v32%ROWTYPE; r buyer_aql_review_rules_v33%ROWTYPE;
 pass_count bigint; minor_count bigint; major_count bigint; critical_count bigint; total bigint; latest integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F319D:AQL:'||NEW.plan_id::text));
 SELECT * INTO p FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.plan_id;
 SELECT * INTO r FROM buyer_aql_review_rules_v33 WHERE tenant_id=NEW.tenant_id AND plan_id=NEW.plan_id;
 SELECT max(revision) INTO latest FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND plant_id=p.plant_id AND lot_code=p.lot_code;
 SELECT count(*),count(*) FILTER(WHERE finding='PASS'),count(*) FILTER(WHERE finding='MINOR'),
 count(*) FILTER(WHERE finding='MAJOR'),count(*) FILTER(WHERE finding='CRITICAL')
 INTO total,pass_count,minor_count,major_count,critical_count FROM buyer_aql_sample_events_v32
 WHERE tenant_id=NEW.tenant_id AND plan_id=NEW.plan_id;
 IF p.id IS NULL OR NOT p.enabled OR p.revision<>latest OR p.plant_id<>NEW.plant_id
 OR r.plan_id IS NULL OR NOT r.enabled OR total<>cardinality(p.sample_garment_ids)
 OR NOT EXISTS(SELECT 1 FROM documents d WHERE d.tenant_id=p.tenant_id AND d.id=p.source_document_id
  AND d.status='APPROVED' AND upper(d.issuer)='CUSTOMER' AND d.file_checksum=p.source_sha256)
 OR EXISTS(SELECT 1 FROM buyer_aql_sample_events_v32 e WHERE e.tenant_id=NEW.tenant_id
  AND e.plan_id=NEW.plan_id AND e.inspector_subject=NEW.reviewer_subject)
 OR NEW.pass_count<>pass_count OR NEW.minor_count<>minor_count OR NEW.major_count<>major_count
 OR NEW.critical_count<>critical_count
 OR (NEW.decision='INTERNAL_RECOMMEND_ACCEPT' AND
   (minor_count>r.max_minor OR major_count>r.max_major OR critical_count>r.max_critical))
 THEN RAISE EXCEPTION 'ERR_F319D_REVIEW_INCOMPLETE_OR_UNAUTHORIZED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_buyer_aql_rules_v33(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_buyer_aql_rules_v33() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE p buyer_aql_plans_v32%ROWTYPE; d documents%ROWTYPE;
BEGIN
 SELECT * INTO p FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.plan_id;
 SELECT * INTO d FROM documents WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF p.id IS NULL OR p.plant_id<>NEW.plant_id OR p.source_document_id<>NEW.source_document_id
 OR p.source_sha256<>NEW.source_sha256 OR d.id IS NULL OR d.status<>'APPROVED'
 OR upper(d.issuer)<>'CUSTOMER' OR d.file_checksum<>NEW.source_sha256
 OR NEW.max_minor+NEW.max_major>=cardinality(p.sample_garment_ids)
 THEN RAISE EXCEPTION 'ERR_F319D_AQL_RULES_SOURCE_OR_LIMITS'; END IF;
 NEW.created_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_buyer_aql_sample_v32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_buyer_aql_sample_v32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE p buyer_aql_plans_v32%ROWTYPE; last_rev integer;
BEGIN
 SELECT * INTO p FROM buyer_aql_plans_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.plan_id;
 SELECT max(revision) INTO last_rev FROM buyer_aql_plans_v32
  WHERE tenant_id=NEW.tenant_id AND plant_id=p.plant_id AND lot_code=p.lot_code;
 IF p.id IS NULL OR NOT p.enabled OR p.revision<>last_rev OR p.plant_id<>NEW.plant_id
 OR NOT (NEW.garment_id=ANY(p.sample_garment_ids)) THEN
  RAISE EXCEPTION 'ERR_F319A_SAMPLE_NOT_SELECTED'; END IF;
 IF NOT EXISTS (SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.plant_id=NEW.plant_id AND q.garment_id=NEW.garment_id AND q.action='QUALITY_RELEASE')
 THEN RAISE EXCEPTION 'ERR_F319A_PER_GARMENT_QC_RELEASE_REQUIRED'; END IF;
 IF NEW.defect_code IS NOT NULL AND NOT EXISTS(SELECT 1 FROM qc_garment_defects_v21 d
  WHERE d.tenant_id=NEW.tenant_id AND d.plant_id=NEW.plant_id AND d.defect_code=NEW.defect_code AND d.enabled)
 THEN RAISE EXCEPTION 'ERR_F319A_UNKNOWN_DEFECT'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_cut_consumption_v19(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_cut_consumption_v19() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE actual_mm bigint; allocated_mm bigint; r cut_sewing_releases_v8%ROWTYPE;
BEGIN
 SELECT * INTO r FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR r.plant_id<>NEW.plant_id OR r.spread_report_id<>NEW.spread_report_id THEN
  RAISE EXCEPTION 'ERR_F32_RELEASE_SCOPE'; END IF;
 SELECT reported_consumption_mm INTO actual_mm FROM cut_spread_reports_v8
  WHERE tenant_id=NEW.tenant_id AND id=NEW.spread_report_id AND lay_draft_id=r.lay_draft_id;
 SELECT COALESCE(sum(allocated_mm),0) INTO allocated_mm FROM cut_spread_issue_allocations_v8
  WHERE tenant_id=NEW.tenant_id AND spread_report_id=NEW.spread_report_id;
 IF actual_mm IS NULL OR allocated_mm<>actual_mm OR
  NEW.used_part_mm>actual_mm OR NEW.trim_waste_mm>actual_mm-NEW.used_part_mm OR
  NEW.damaged_mm<>actual_mm-NEW.used_part_mm-NEW.trim_waste_mm THEN
   RAISE EXCEPTION 'ERR_F32_MATERIAL_BALANCE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_spread_issue_allocations_v8 a
  JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id
  JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
  WHERE a.tenant_id=NEW.tenant_id AND a.spread_report_id=NEW.spread_report_id AND sr.plant_id=NEW.plant_id) THEN
   RAISE EXCEPTION 'ERR_F32_NO_SIGNED_ISSUE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_cut_part_outcome_v19(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_cut_part_outcome_v19() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE actual_plant text; actual_release uuid;
BEGIN
 SELECT g.plant_id,g.release_id INTO actual_plant,actual_release FROM gen_cut_parts_v18 p
  JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF actual_plant IS NULL OR actual_plant<>NEW.plant_id OR actual_release<>NEW.release_id THEN
  RAISE EXCEPTION 'ERR_F32_PART_SCOPE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_consumption_reconciliations_v19 c WHERE c.tenant_id=NEW.tenant_id
  AND c.release_id=NEW.release_id AND c.plant_id=NEW.plant_id) THEN
  RAISE EXCEPTION 'ERR_F32_CONSUMPTION_NOT_RECONCILED'; END IF;
 -- Serialize late reject against recut-link creation for this replacement.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PART:'||NEW.part_id::text));
 IF NEW.disposition='REJECTED' AND EXISTS (SELECT 1 FROM cut_recut_links_v19 l
  WHERE l.tenant_id=NEW.tenant_id AND l.replacement_part_id=NEW.part_id) THEN
  RAISE EXCEPTION 'ERR_F32_LINKED_RECUT_CANNOT_BE_REJECTED'; END IF;
 IF NEW.disposition='REJECTED' AND NOT EXISTS (
  SELECT 1 FROM cut_defect_codes_v19 d WHERE d.tenant_id=NEW.tenant_id AND d.plant_id=NEW.plant_id
  AND d.code=NEW.defect_code AND d.enabled) THEN RAISE EXCEPTION 'ERR_F32_DEFECT_NOT_APPROVED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_cut_recut_v19(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_cut_recut_v19() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE oldpart record; newpart record;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PART:'||NEW.replacement_part_id::text));
 SELECT p.part_code,p.part_slot,g.release_id,g.plant_id,g.lay_draft_id,d.intake_id,d.fabric_sku,d.shade_band,d.source_order_sha256,
  pos.size_code,pos.color_code INTO oldpart
 FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
 JOIN cut_lay_drafts_v8 d ON d.tenant_id=g.tenant_id AND d.id=g.lay_draft_id
 JOIN cut_lay_positions_v8 pos ON pos.tenant_id=g.tenant_id AND pos.lay_draft_id=g.lay_draft_id AND pos.marker_position=g.marker_position
 WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.original_part_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'ERR_F32_ORIGINAL_NOT_FOUND'; END IF;
 SELECT p.part_code,p.part_slot,g.release_id,g.plant_id,g.lay_draft_id,d.intake_id,d.fabric_sku,d.shade_band,d.source_order_sha256,
  pos.size_code,pos.color_code INTO newpart
 FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
 JOIN cut_lay_drafts_v8 d ON d.tenant_id=g.tenant_id AND d.id=g.lay_draft_id
 JOIN cut_lay_positions_v8 pos ON pos.tenant_id=g.tenant_id AND pos.lay_draft_id=g.lay_draft_id AND pos.marker_position=g.marker_position
 WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.replacement_part_id;
 IF NOT FOUND OR oldpart.plant_id<>NEW.plant_id OR newpart.plant_id<>NEW.plant_id OR
  oldpart.release_id<>NEW.release_id OR oldpart.lay_draft_id=newpart.lay_draft_id OR oldpart.intake_id<>newpart.intake_id OR
  oldpart.part_code<>newpart.part_code OR oldpart.part_slot<>newpart.part_slot OR
  oldpart.size_code<>newpart.size_code OR oldpart.color_code<>newpart.color_code OR
  oldpart.fabric_sku<>newpart.fabric_sku OR oldpart.shade_band<>newpart.shade_band OR
  oldpart.source_order_sha256<>newpart.source_order_sha256 THEN
   RAISE EXCEPTION 'ERR_F32_RECUT_LINEAGE'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 d WHERE d.tenant_id=NEW.tenant_id
  AND d.part_id=NEW.original_part_id AND d.disposition='REJECTED') THEN
   RAISE EXCEPTION 'ERR_F32_ORIGINAL_NOT_REJECTED'; END IF;
 IF EXISTS(SELECT 1 FROM cut_part_outcomes_v19 d WHERE d.tenant_id=NEW.tenant_id
  AND d.part_id=NEW.replacement_part_id AND d.disposition='REJECTED') THEN
   RAISE EXCEPTION 'ERR_F32_REPLACEMENT_REJECTED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_dock_asn_v34(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_dock_asn_v34() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE receipt dock_carrier_receipts_v34%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:MANIFEST:'||NEW.manifest_id::text));
 SELECT * INTO receipt FROM dock_carrier_receipts_v34
 WHERE tenant_id=NEW.tenant_id AND id=NEW.carrier_receipt_id;
 IF receipt.id IS NULL OR receipt.plant_id<>NEW.plant_id OR receipt.manifest_id<>NEW.manifest_id
 OR receipt.actor_subject=NEW.actor_subject
 OR NOT dock_manifest_safe_v34(NEW.tenant_id,NEW.plant_id,NEW.manifest_id)
 THEN RAISE EXCEPTION 'ERR_F320D_ASN_DRAFT_NOT_AUTHORIZED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_dock_carrier_v34(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_dock_carrier_v34() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE seal dock_container_seals_v34%ROWTYPE; staged_actor text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:MANIFEST:'||NEW.manifest_id::text));
 SELECT * INTO seal FROM dock_container_seals_v34 WHERE tenant_id=NEW.tenant_id AND id=NEW.seal_id;
 SELECT actor_subject INTO staged_actor FROM pack_manifest_events_v22
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND manifest_id=NEW.manifest_id AND action='STAGE';
 IF seal.id IS NULL OR seal.plant_id<>NEW.plant_id OR seal.manifest_id<>NEW.manifest_id
 OR seal.actor_subject=NEW.actor_subject OR staged_actor=NEW.actor_subject
 OR NOT EXISTS(SELECT 1 FROM dock_carrier_authorizations_v34 c WHERE c.tenant_id=NEW.tenant_id
    AND c.plant_id=NEW.plant_id AND c.carrier_code=NEW.carrier_code AND c.enabled
    AND c.idp_issuer=NEW.actor_issuer AND c.idp_subject=NEW.actor_subject)
 OR NOT dock_manifest_safe_v34(NEW.tenant_id,NEW.plant_id,NEW.manifest_id)
 THEN RAISE EXCEPTION 'ERR_F320C_CARRIER_CUSTODY_NOT_VERIFIED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_dock_external_v35(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_dock_external_v35() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE a dock_asn_drafts_v34%ROWTYPE;
        signer dock_external_signers_v35%ROWTYPE;
        carrier_code text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:MANIFEST:'||NEW.manifest_id::text));
 SELECT * INTO signer FROM dock_external_signers_v35
  WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND signer_id=NEW.signer_id;
 SELECT * INTO a FROM dock_asn_drafts_v34 WHERE tenant_id=NEW.tenant_id AND id=NEW.asn_draft_id;
 IF signer.signer_id IS NULL OR NOT signer.enabled OR signer.party_kind<>NEW.party_kind
 OR signer.party_code<>NEW.party_code
 OR encode(digest(signer.public_key,'sha256'),'hex')<>NEW.signer_key_sha256
 OR a.id IS NULL OR a.plant_id<>NEW.plant_id OR a.manifest_id<>NEW.manifest_id
 OR a.asn_reference<>NEW.asn_reference OR a.payload_sha256<>NEW.payload_sha256
 OR NEW.issued_at<a.recorded_at - interval '2 minutes'
 OR NEW.issued_at<clock_timestamp()-interval '24 hours'
 OR NEW.issued_at>clock_timestamp()+interval '2 minutes'
 OR NOT dock_evidence_current_v34(NEW.tenant_id,NEW.plant_id,NEW.manifest_id)
 THEN RAISE EXCEPTION 'ERR_F35_EXTERNAL_ASSERTION_NOT_CURRENT'; END IF;
 IF NEW.assertion_kind='CARRIER_ASN_ACK' THEN
  SELECT r.carrier_code INTO carrier_code FROM dock_carrier_receipts_v34 r
   WHERE r.tenant_id=NEW.tenant_id AND r.id=a.carrier_receipt_id;
  IF carrier_code IS DISTINCT FROM signer.party_code THEN
   RAISE EXCEPTION 'ERR_F35_CARRIER_MISMATCH'; END IF;
 ELSIF NEW.assertion_kind='BUYER_LOT_REVIEW' THEN
  IF NOT EXISTS (SELECT 1 FROM pack_manifest_events_v22 o
   JOIN dock_buyer_spec_bindings_v36 b ON b.tenant_id=o.tenant_id
     AND b.plant_id=o.plant_id AND b.spec_id=o.spec_id
     AND b.enabled AND b.buyer_code=signer.party_code
   WHERE o.tenant_id=NEW.tenant_id AND o.plant_id=NEW.plant_id
     AND o.manifest_id=NEW.manifest_id AND o.action='OPEN')
  THEN RAISE EXCEPTION 'ERR_F36_BUYER_NOT_AUTHORIZED_FOR_SPEC'; END IF;
 ELSE RAISE EXCEPTION 'ERR_F36_ASSERTION_KIND_UNSUPPORTED'; END IF;
 NEW.received_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_dock_proof_v34(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_dock_proof_v34() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PROOF:'||NEW.proof_id));
 IF EXISTS(SELECT 1 FROM needle_safety_events_v32 WHERE tenant_id=NEW.tenant_id AND proof_id=NEW.proof_id)
 OR EXISTS(SELECT 1 FROM buyer_aql_sample_events_v32 WHERE tenant_id=NEW.tenant_id AND inspector_proof_id=NEW.proof_id)
 OR EXISTS(SELECT 1 FROM fabric_calibration_links_v32 WHERE tenant_id=NEW.tenant_id AND inspector_proof_id=NEW.proof_id)
 OR EXISTS(SELECT 1 FROM needle_safety_clearances_v33 WHERE tenant_id=NEW.tenant_id AND reviewer_proof_id=NEW.proof_id)
 OR EXISTS(SELECT 1 FROM buyer_aql_reviews_v33 WHERE tenant_id=NEW.tenant_id AND reviewer_proof_id=NEW.proof_id)
 OR EXISTS(SELECT 1 FROM dock_container_seals_v34 WHERE tenant_id=NEW.tenant_id AND proof_id=NEW.proof_id)
 OR EXISTS(SELECT 1 FROM dock_carrier_receipts_v34 WHERE tenant_id=NEW.tenant_id AND proof_id=NEW.proof_id)
 OR EXISTS(SELECT 1 FROM dock_asn_drafts_v34 WHERE tenant_id=NEW.tenant_id AND proof_id=NEW.proof_id)
 THEN RAISE EXCEPTION 'ERR_F34_SIGNED_PROOF_REUSED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_dock_scan_v34(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_dock_scan_v34() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE staged_at timestamptz; code text; enrolled_key bytea;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:MANIFEST:'||NEW.manifest_id::text));
 SELECT public_key INTO enrolled_key FROM dock_gateways_v34 g WHERE g.tenant_id=NEW.tenant_id
  AND g.plant_id=NEW.plant_id AND g.gateway_id=NEW.gateway_id AND g.enabled;
 IF enrolled_key IS NULL OR encode(digest(enrolled_key,'sha256'),'hex')<>NEW.gateway_key_sha256
 OR NOT EXISTS(SELECT 1 FROM dock_station_policies_v34 s WHERE s.tenant_id=NEW.tenant_id
  AND s.plant_id=NEW.plant_id AND s.enabled)
 THEN RAISE EXCEPTION 'ERR_F320B_GATEWAY_OR_POLICY_NOT_APPROVED'; END IF;
 SELECT recorded_at INTO staged_at FROM pack_manifest_events_v22
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND manifest_id=NEW.manifest_id AND action='STAGE';
 SELECT carton_code INTO code FROM pack_carton_events_v22
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND carton_id=NEW.carton_id AND action='OPEN';
 IF staged_at IS NULL OR code IS NULL OR code<>NEW.carton_code
 OR NOT EXISTS(SELECT 1 FROM pack_manifest_events_v22 e WHERE e.tenant_id=NEW.tenant_id
  AND e.plant_id=NEW.plant_id AND e.manifest_id=NEW.manifest_id AND e.carton_id=NEW.carton_id AND e.action='ADD_CARTON')
 OR NOT dock_manifest_safe_v34(NEW.tenant_id,NEW.plant_id,NEW.manifest_id)
 OR NEW.observed_at<staged_at OR NEW.observed_at>clock_timestamp()+interval '2 minutes'
 THEN RAISE EXCEPTION 'ERR_F320B_DOCK_SCAN_PRECONDITIONS'; END IF;
 NEW.received_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_dock_seal_v34(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_dock_seal_v34() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE total bigint; scanned bigint; age_minutes integer; staged_actor text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:MANIFEST:'||NEW.manifest_id::text));
 SELECT count(*) INTO total FROM pack_manifest_events_v22
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND manifest_id=NEW.manifest_id AND action='ADD_CARTON';
 SELECT max_scan_age_minutes INTO age_minutes FROM dock_station_policies_v34
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND enabled;
 SELECT actor_subject INTO staged_actor FROM pack_manifest_events_v22
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND manifest_id=NEW.manifest_id AND action='STAGE';
 SELECT count(*) INTO scanned FROM dock_carton_scans_v34
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND manifest_id=NEW.manifest_id
 AND received_at >=clock_timestamp()-make_interval(mins=>COALESCE(age_minutes,0));
 IF total=0 OR age_minutes IS NULL OR scanned<>total OR NEW.scanned_cartons<>total
 OR staged_actor IS NULL OR staged_actor=NEW.actor_subject
 OR NOT dock_manifest_safe_v34(NEW.tenant_id,NEW.plant_id,NEW.manifest_id)
 THEN RAISE EXCEPTION 'ERR_F320C_UNSCANNED_UNSAFE_OR_NONINDEPENDENT_SEAL'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_fabric_calibration_link_v32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_fabric_calibration_link_v32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE o fabric_shrinkage_observations_v30%ROWTYPE; c factory_instrument_calibrations_v32%ROWTYPE; newest integer;
BEGIN
 SELECT * INTO o FROM fabric_shrinkage_observations_v30 WHERE tenant_id=NEW.tenant_id AND id=NEW.observation_id;
 SELECT * INTO c FROM factory_instrument_calibrations_v32 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id
  AND instrument_id=NEW.instrument_id AND revision=NEW.calibration_revision;
 SELECT max(revision) INTO newest FROM factory_instrument_calibrations_v32
  WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND instrument_id=NEW.instrument_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR c.instrument_id IS NULL OR c.instrument_kind<>'DIMENSION'
 OR NOT c.enabled OR c.revision<>newest OR o.recorded_at NOT BETWEEN c.valid_from AND c.valid_until
 OR c.valid_until<clock_timestamp() OR o.tester_subject=NEW.inspector_subject THEN
  RAISE EXCEPTION 'ERR_F318C_INVALID_CALIBRATION_OR_INDEPENDENCE'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_fabric_shrinkage_spec_v31(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_fabric_shrinkage_spec_v31() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE src documents%ROWTYPE;
BEGIN
 SELECT * INTO src FROM documents WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF src.id IS NULL OR src.status<>'APPROVED' OR upper(src.issuer)<>'CUSTOMER'
    OR src.file_checksum<>NEW.source_sha256
    OR src.document_type NOT IN ('TECH_PACK','BUYER_SPEC','FABRIC_SHRINKAGE_SPEC','FABRIC_TEST_SPEC')
 THEN RAISE EXCEPTION 'ERR_F318B_APPROVED_CUSTOMER_SPEC_REQUIRED'; END IF;
 NEW.approved_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_fabric_shrinkage_v30(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_fabric_shrinkage_v30() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE sku varchar(128); pl varchar(64); doc_hash char(64);
BEGIN
 SELECT plant_id,factory_sku INTO pl,sku FROM store_rolls
  WHERE tenant_id=NEW.tenant_id AND id=NEW.roll_id;
 IF NOT FOUND OR pl<>NEW.plant_id OR sku<>NEW.material_sku
 THEN RAISE EXCEPTION 'ERR_F318A_ROLL_SCOPE'; END IF;
 SELECT sha256 INTO doc_hash FROM document_sources_v65
  WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF doc_hash IS NULL OR doc_hash<>NEW.source_sha256
 THEN RAISE EXCEPTION 'ERR_F318A_SOURCE_HASH'; END IF;
 IF abs(NEW.warp_before_mm-NEW.warp_after_mm)*100 > NEW.warp_before_mm*30
 OR abs(NEW.weft_before_mm-NEW.weft_after_mm)*100 > NEW.weft_before_mm*30
 THEN RAISE EXCEPTION 'ERR_F318A_OUT_OF_BOUNDS'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_factory_proof_v32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_factory_proof_v32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE proof text;
BEGIN
 IF TG_TABLE_NAME='needle_safety_events_v32' THEN proof:=NEW.proof_id;
 ELSE proof:=NEW.inspector_proof_id; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PROOF:'||proof));
 IF EXISTS(SELECT 1 FROM fabric_calibration_links_v32 x WHERE x.tenant_id=NEW.tenant_id AND x.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM buyer_aql_sample_events_v32 x WHERE x.tenant_id=NEW.tenant_id AND x.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM needle_safety_events_v32 x WHERE x.tenant_id=NEW.tenant_id AND x.proof_id=proof)
 OR EXISTS(SELECT 1 FROM needle_safety_clearances_v33 x WHERE x.tenant_id=NEW.tenant_id AND x.reviewer_proof_id=proof)
 OR EXISTS(SELECT 1 FROM buyer_aql_reviews_v33 x WHERE x.tenant_id=NEW.tenant_id AND x.reviewer_proof_id=proof)
 OR EXISTS(SELECT 1 FROM dock_container_seals_v34 x WHERE x.tenant_id=NEW.tenant_id AND x.proof_id=proof)
 OR EXISTS(SELECT 1 FROM dock_carrier_receipts_v34 x WHERE x.tenant_id=NEW.tenant_id AND x.proof_id=proof)
 OR EXISTS(SELECT 1 FROM dock_asn_drafts_v34 x WHERE x.tenant_id=NEW.tenant_id AND x.proof_id=proof)
 THEN RAISE EXCEPTION 'ERR_F32_PROOF_REUSED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_factory_review_proof_v33(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_factory_review_proof_v33() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE proof text;
BEGIN
 proof:=NEW.reviewer_proof_id;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F32:PROOF:'||proof));
 IF EXISTS(SELECT 1 FROM needle_safety_events_v32 e WHERE e.tenant_id=NEW.tenant_id AND e.proof_id=proof)
 OR EXISTS(SELECT 1 FROM buyer_aql_sample_events_v32 e WHERE e.tenant_id=NEW.tenant_id AND e.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM fabric_calibration_links_v32 e WHERE e.tenant_id=NEW.tenant_id AND e.inspector_proof_id=proof)
 OR EXISTS(SELECT 1 FROM needle_safety_clearances_v33 e WHERE e.tenant_id=NEW.tenant_id AND e.reviewer_proof_id=proof)
 OR EXISTS(SELECT 1 FROM buyer_aql_reviews_v33 e WHERE e.tenant_id=NEW.tenant_id AND e.reviewer_proof_id=proof)
 OR EXISTS(SELECT 1 FROM dock_container_seals_v34 e WHERE e.tenant_id=NEW.tenant_id AND e.proof_id=proof)
 OR EXISTS(SELECT 1 FROM dock_carrier_receipts_v34 e WHERE e.tenant_id=NEW.tenant_id AND e.proof_id=proof)
 OR EXISTS(SELECT 1 FROM dock_asn_drafts_v34 e WHERE e.tenant_id=NEW.tenant_id AND e.proof_id=proof)
 THEN RAISE EXCEPTION 'ERR_F33_ASSERTION_REUSED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_gen_bundle_v18(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_gen_bundle_v18() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE r cut_sewing_releases_v8%ROWTYPE;
BEGIN
 SELECT * INTO r FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR r.plant_id<>NEW.plant_id OR r.lay_draft_id<>NEW.lay_draft_id OR
  NOT EXISTS (SELECT 1 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.label_code=NEW.label_code) THEN
  RAISE EXCEPTION 'ERR_GEN_BUNDLE_RELEASE_SCOPE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_gen_garment_v18(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_gen_garment_v18() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE r cut_sewing_releases_v8%ROWTYPE; overlapping integer;
BEGIN
 SELECT * INTO r FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR r.plant_id<>NEW.plant_id OR r.lay_draft_id<>NEW.lay_draft_id OR
  NOT EXISTS (SELECT 1 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position AND NEW.ply_number BETWEEN b.ply_start AND b.ply_end) THEN
  RAISE EXCEPTION 'ERR_GEN_GARMENT_CUT_SCOPE'; END IF;
 SELECT count(*)-count(DISTINCT part_code) INTO overlapping FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position
    AND NEW.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF overlapping<>0 THEN RAISE EXCEPTION 'ERR_GEN_AMBIGUOUS_PLY_PART'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_gen_part_v18(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_gen_part_v18() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE g gen_garments_v18%ROWTYPE; b gen_bundles_v18%ROWTYPE; bd cut_bundle_drafts_v8%ROWTYPE;
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 SELECT * INTO b FROM gen_bundles_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.bundle_id;
 SELECT * INTO bd FROM cut_bundle_drafts_v8 WHERE tenant_id=NEW.tenant_id AND lay_draft_id=NEW.lay_draft_id AND label_code=NEW.label_code;
 IF NOT FOUND OR g.id IS NULL OR b.id IS NULL OR b.release_id<>g.release_id OR b.plant_id<>g.plant_id OR
  b.lay_draft_id<>g.lay_draft_id OR b.label_code<>NEW.label_code OR NEW.lay_draft_id<>g.lay_draft_id OR
  bd.marker_position<>g.marker_position OR NEW.part_code<>bd.part_code OR
  g.ply_number NOT BETWEEN bd.ply_start AND bd.ply_end OR NEW.part_slot>bd.pieces_per_garment THEN
  RAISE EXCEPTION 'ERR_GEN_PART_LINEAGE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_instrument_calibration_v32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_instrument_calibration_v32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE h char(64); last_rev integer;
BEGIN
 SELECT sha256 INTO h FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF h IS NULL OR h<>NEW.source_sha256 THEN RAISE EXCEPTION 'ERR_F318C_CALIBRATION_SOURCE_HASH'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F318C:CAL:'||NEW.plant_id||':'||NEW.instrument_id));
 SELECT max(revision) INTO last_rev FROM factory_instrument_calibrations_v32
 WHERE tenant_id=NEW.tenant_id AND plant_id=NEW.plant_id AND instrument_id=NEW.instrument_id;
 IF last_rev IS NOT NULL AND NEW.revision<>last_rev+1 OR last_rev IS NULL AND NEW.revision<>1 THEN
  RAISE EXCEPTION 'ERR_F318C_REVISION_SEQUENCE'; END IF;
 NEW.registered_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_jobwork_discrepancy_v29(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_jobwork_discrepancy_v29() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE o jobwork_outwards_v28%ROWTYPE; g uuid; lay uuid; pos integer; grp integer;
        inward_actor varchar(128);
BEGIN
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR o.part_id<>NEW.part_id
 THEN RAISE EXCEPTION 'ERR_F317B_OUTWARD_SCOPE'; END IF;
 SELECT p.garment_id,garm.lay_draft_id,garm.marker_position INTO g,lay,pos
  FROM gen_cut_parts_v18 p JOIN gen_garments_v18 garm
   ON garm.tenant_id=p.tenant_id AND garm.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 SELECT min(b.ply_group) INTO grp FROM cut_bundle_drafts_v8 b JOIN gen_garments_v18 x
  ON x.tenant_id=b.tenant_id AND x.lay_draft_id=b.lay_draft_id
  WHERE x.tenant_id=NEW.tenant_id AND x.id=g
   AND b.marker_position=pos AND x.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF g IS NULL OR grp IS NULL THEN RAISE EXCEPTION 'ERR_F317B_PART_GENEALOGY'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||pos||':'||grp));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 -- Do not backdate a disputed state into already released or packed work.
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q
   WHERE q.tenant_id=NEW.tenant_id AND q.garment_id=g AND q.action='QUALITY_RELEASE')
 OR EXISTS(SELECT 1 FROM pack_carton_events_v22 pk
   WHERE pk.tenant_id=NEW.tenant_id AND pk.garment_id=g AND pk.action='ADD')
 THEN RAISE EXCEPTION 'ERR_F317B_ALREADY_RELEASED'; END IF;
 IF o.sender_subject=NEW.reporter_subject THEN
  RAISE EXCEPTION 'ERR_F317B_REPORTER_NOT_INDEPENDENT'; END IF;
 SELECT receiver_subject INTO inward_actor FROM jobwork_inwards_v28
  WHERE tenant_id=NEW.tenant_id AND outward_id=NEW.outward_id;
 IF (NEW.category='MISSING_AT_RECONCILIATION' AND inward_actor IS NOT NULL)
 OR (NEW.category='DAMAGED_ON_RETURN' AND inward_actor IS NULL)
 THEN RAISE EXCEPTION 'ERR_F317B_DISCREPANCY_CONTRADICTS_CUSTODY'; END IF;
 IF inward_actor IS NOT NULL AND inward_actor=NEW.reporter_subject THEN
  RAISE EXCEPTION 'ERR_F317B_REPORTER_NOT_INDEPENDENT'; END IF;
 IF EXISTS(SELECT 1 FROM jobwork_outwards_v28 later
  WHERE later.tenant_id=NEW.tenant_id AND later.part_id=NEW.part_id
   AND (later.recorded_at,later.id)>(o.recorded_at,o.id))
 THEN RAISE EXCEPTION 'ERR_F317B_NOT_LATEST_CYCLE'; END IF;
 -- Single-use evidence across the job-work family, regardless of action.
 IF EXISTS(SELECT 1 FROM jobwork_used_proofs_v28 p
   WHERE p.tenant_id=NEW.tenant_id AND p.proof_id=NEW.reporter_proof_id)
 THEN RAISE EXCEPTION 'ERR_F317B_REUSED_PROOF'; END IF;
 NEW.reported_at := clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_jobwork_disposition_v31(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_jobwork_disposition_v31() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE d jobwork_discrepancies_v29%ROWTYPE;
        o jobwork_outwards_v28%ROWTYPE;
        ret jobwork_inwards_v28%ROWTYPE;
        garment uuid; lay uuid; pos integer; grp integer;
BEGIN
 SELECT * INTO d FROM jobwork_discrepancies_v29 WHERE tenant_id=NEW.tenant_id AND id=NEW.discrepancy_id;
 IF d.id IS NULL OR d.outward_id<>NEW.outward_id OR d.part_id<>NEW.part_id OR d.plant_id<>NEW.plant_id
 THEN RAISE EXCEPTION 'ERR_F317D_DISCREPANCY_SCOPE'; END IF;
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 IF o.id IS NULL OR o.part_id<>NEW.part_id OR o.plant_id<>NEW.plant_id
 THEN RAISE EXCEPTION 'ERR_F317D_OUTWARD_SCOPE'; END IF;
 SELECT p.garment_id,g.lay_draft_id,g.marker_position INTO garment,lay,pos
  FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 SELECT min(b.ply_group) INTO grp FROM cut_bundle_drafts_v8 b JOIN gen_garments_v18 g
  ON g.tenant_id=b.tenant_id AND g.lay_draft_id=b.lay_draft_id
  WHERE g.tenant_id=NEW.tenant_id AND g.id=garment AND b.marker_position=pos
    AND g.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF garment IS NULL OR grp IS NULL THEN RAISE EXCEPTION 'ERR_F317D_GENEALOGY'; END IF;
 -- Same serialization boundary as earlier piece holds and job-work transactions.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||pos||':'||grp));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF EXISTS(SELECT 1 FROM jobwork_discrepancy_resolutions_v31 r
   WHERE r.tenant_id=NEW.tenant_id AND r.discrepancy_id=NEW.discrepancy_id)
 THEN RAISE EXCEPTION 'ERR_F317D_ALREADY_DISPOSED'; END IF;
 SELECT * INTO ret FROM jobwork_inwards_v28 WHERE tenant_id=NEW.tenant_id AND outward_id=NEW.outward_id;
 IF NEW.supervisor_subject IN (d.reporter_subject,o.sender_subject)
    OR (ret.id IS NOT NULL AND NEW.supervisor_subject=ret.receiver_subject)
 THEN RAISE EXCEPTION 'ERR_F317D_INDEPENDENCE'; END IF;
 IF EXISTS(SELECT 1 FROM jobwork_used_proofs_v28 u WHERE u.tenant_id=NEW.tenant_id AND u.proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 x WHERE x.tenant_id=NEW.tenant_id AND x.reporter_proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM jobwork_vendor_acks_v30 a WHERE a.tenant_id=NEW.tenant_id AND a.recorder_proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM wip_piece_holds_v26 h WHERE h.tenant_id=NEW.tenant_id AND h.reporter_proof_id=NEW.supervisor_proof_id)
 OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 h WHERE h.tenant_id=NEW.tenant_id AND h.supervisor_proof_id=NEW.supervisor_proof_id)
 THEN RAISE EXCEPTION 'ERR_F317D_PROOF_SPENT'; END IF;
 IF NEW.resolution_kind='RETURNED_TO_PIECE_HOLD' THEN
  -- Recorded inward must exist. A missing-item report requires return AFTER report;
  -- damaged-on-return has its own prior inward. The active hold is a SECOND ledger.
  IF ret.id IS NULL OR (d.category='MISSING_AT_RECONCILIATION' AND ret.recorded_at<=d.reported_at)
    OR NOT EXISTS(SELECT 1 FROM wip_piece_holds_v26 h
       WHERE h.tenant_id=NEW.tenant_id AND h.garment_id=garment AND h.opened_at>=d.reported_at
        AND NOT EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=h.tenant_id AND r.hold_id=h.id))
  THEN RAISE EXCEPTION 'ERR_F317D_PHYSICAL_RETURN_AND_ACTIVE_HOLD_REQUIRED'; END IF;
 ELSIF NEW.resolution_kind='SHORTAGE_REPORTED_REMAINS_BLOCKED' THEN
  IF d.category<>'MISSING_AT_RECONCILIATION' OR ret.id IS NOT NULL
  THEN RAISE EXCEPTION 'ERR_F317D_NOT_OUTSTANDING'; END IF;
 ELSIF NEW.resolution_kind='DAMAGE_REPORTED_REMAINS_BLOCKED' THEN
  IF d.category<>'DAMAGED_ON_RETURN' OR ret.id IS NULL
  THEN RAISE EXCEPTION 'ERR_F317D_NO_DAMAGE_RETURN'; END IF;
 END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_jobwork_disputed_progress_v29(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_jobwork_disputed_progress_v29() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE garment uuid; part uuid; lay uuid;
BEGIN
 IF TG_TABLE_NAME IN ('jobwork_outwards_v28','wip_movements_v20') THEN
  part:=NEW.part_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||part::text));
  IF jobwork_part_disputed_v29(NEW.tenant_id,part) THEN
   RAISE EXCEPTION 'ERR_F317B_DISPUTED_PART_BLOCKED'; END IF;
 ELSIF TG_TABLE_NAME='sew_prod_completions_v10' THEN
  SELECT r.lay_draft_id INTO lay FROM sew_prod_routes_v10 r
   WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.route_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||NEW.marker_position||':'||NEW.ply_group));
  IF EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 d
    JOIN gen_cut_parts_v18 p ON p.tenant_id=d.tenant_id AND p.id=d.part_id
    JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
    JOIN cut_bundle_drafts_v8 b ON b.tenant_id=g.tenant_id AND b.lay_draft_id=g.lay_draft_id
     AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
    WHERE d.tenant_id=NEW.tenant_id AND g.lay_draft_id=lay
      AND g.marker_position=NEW.marker_position AND b.ply_group=NEW.ply_group
      AND jobwork_part_disputed_v29(d.tenant_id,d.part_id))
  THEN RAISE EXCEPTION 'ERR_F317B_DISPUTED_GROUP_BLOCKED'; END IF;
 ELSIF TG_TABLE_NAME IN ('sew_piece_completions_v27','qc_garment_events_v21','pack_carton_events_v22') THEN
  IF TG_TABLE_NAME='pack_carton_events_v22' AND NEW.action<>'ADD' THEN RETURN NEW; END IF;
  IF TG_TABLE_NAME='qc_garment_events_v21' AND NEW.action IN ('INSPECT_HOLD','REINSPECT_HOLD','REPAIR_COMPLETE') THEN RETURN NEW; END IF;
  garment:=NEW.garment_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
  IF EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=garment
    AND jobwork_part_disputed_v29(p.tenant_id,p.id))
  THEN RAISE EXCEPTION 'ERR_F317B_DISPUTED_GARMENT_BLOCKED'; END IF;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_jobwork_inward_v28(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_jobwork_inward_v28() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE o jobwork_outwards_v28%ROWTYPE; g uuid; group_no integer; lay uuid; pos integer;
BEGIN
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR o.part_id<>NEW.part_id OR o.station_id<>NEW.station_id
   OR o.sender_subject=NEW.receiver_subject THEN RAISE EXCEPTION 'ERR_F317_OUTWARD_OR_INDEPENDENCE'; END IF;
 SELECT p.garment_id,garm.lay_draft_id,garm.marker_position INTO g,lay,pos
  FROM gen_cut_parts_v18 p JOIN gen_garments_v18 garm ON garm.tenant_id=p.tenant_id AND garm.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 SELECT min(b.ply_group) INTO group_no FROM cut_bundle_drafts_v8 b JOIN gen_garments_v18 garm
  ON garm.tenant_id=b.tenant_id AND garm.lay_draft_id=b.lay_draft_id
  WHERE garm.tenant_id=NEW.tenant_id AND garm.id=g
  AND b.marker_position=pos AND garm.ply_number BETWEEN b.ply_start AND b.ply_end;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||pos||':'||group_no));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF NOT jobwork_part_offsite_v28(NEW.tenant_id,NEW.part_id) OR
  EXISTS(SELECT 1 FROM jobwork_inwards_v28 i WHERE i.tenant_id=NEW.tenant_id AND i.outward_id=NEW.outward_id)
 THEN RAISE EXCEPTION 'ERR_F317_OUTWARD_NOT_OPEN'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.receiver_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F317_RECEIVER_NOT_ENROLLED'; END IF;
 PERFORM jobwork_require_scan_v28(NEW.tenant_id,NEW.plant_id,NEW.station_id,NEW.part_id,NEW.scan_id);
 IF NOT EXISTS(SELECT 1 FROM wip_gateway_scans_v20 x WHERE x.tenant_id=NEW.tenant_id
  AND x.id=NEW.scan_id AND x.observed_at>=o.recorded_at AND x.received_at>=o.recorded_at)
 THEN RAISE EXCEPTION 'ERR_F317_RETURN_SCAN_PRECEDES_OUTWARD'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_jobwork_offsite_v28(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_jobwork_offsite_v28() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE garment uuid; part uuid; lay uuid; pos int; group_no int;
BEGIN
 IF TG_TABLE_NAME='wip_movements_v20' THEN
  part:=NEW.part_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||part::text));
  IF jobwork_part_offsite_v28(NEW.tenant_id,part) THEN RAISE EXCEPTION 'ERR_F317_OFFSITE_NO_WIP_MOVE'; END IF;
 ELSIF TG_TABLE_NAME='sew_prod_completions_v10' THEN
  SELECT r.lay_draft_id INTO lay FROM sew_prod_routes_v10 r WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.route_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||lay::text||':'||NEW.marker_position||':'||NEW.ply_group));
  IF EXISTS(SELECT 1 FROM jobwork_outwards_v28 o JOIN gen_cut_parts_v18 p ON p.tenant_id=o.tenant_id AND p.id=o.part_id
    JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
    JOIN cut_bundle_drafts_v8 b ON b.tenant_id=g.tenant_id AND b.lay_draft_id=g.lay_draft_id
     AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
    WHERE o.tenant_id=NEW.tenant_id AND g.lay_draft_id=lay AND g.marker_position=NEW.marker_position
     AND b.ply_group=NEW.ply_group AND jobwork_part_offsite_v28(o.tenant_id,o.part_id))
  THEN RAISE EXCEPTION 'ERR_F317_OFFSITE_NO_GROUP_SEWING'; END IF;
 ELSIF TG_TABLE_NAME='sew_piece_completions_v27' OR TG_TABLE_NAME='qc_garment_events_v21'
 OR TG_TABLE_NAME='pack_carton_events_v22' THEN
  IF TG_TABLE_NAME='pack_carton_events_v22' THEN
   IF NEW.action<>'ADD' THEN RETURN NEW; END IF;
  END IF;
  garment:=NEW.garment_id;
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
  IF EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=garment
   AND jobwork_part_offsite_v28(p.tenant_id,p.id))
  THEN RAISE EXCEPTION 'ERR_F317_OFFSITE_NO_SEWING_QC_PACKING'; END IF;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_jobwork_outward_v28(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_jobwork_outward_v28() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE g gen_garments_v18%ROWTYPE; part gen_cut_parts_v18%ROWTYPE;
 prior wip_movements_v20%ROWTYPE; group_no integer;
BEGIN
 SELECT * INTO part FROM gen_cut_parts_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.part_id;
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=part.garment_id;
 IF part.id IS NULL OR g.id IS NULL OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_F317_PART_SCOPE'; END IF;
 SELECT min(b.ply_group) INTO group_no FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id
  AND b.lay_draft_id=g.lay_draft_id AND b.marker_position=g.marker_position
  AND g.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF group_no IS NULL THEN RAISE EXCEPTION 'ERR_F317_GROUP_MISSING'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||g.lay_draft_id::text||':'||g.marker_position||':'||group_no));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g.id::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF NOT EXISTS(SELECT 1 FROM jobwork_vendors_v28 v WHERE v.tenant_id=NEW.tenant_id
   AND v.plant_id=NEW.plant_id AND v.vendor_id=NEW.vendor_id AND v.enabled
   AND NEW.process_code=ANY(v.allowed_processes))
 OR NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
   AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
   AND w.worker_subject=NEW.sender_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F317_VENDOR_OR_SENDER_NOT_ENROLLED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=NEW.tenant_id
  AND o.part_id=NEW.part_id AND o.disposition='ACCEPTED')
 OR EXISTS(SELECT 1 FROM cut_recut_links_v19 l WHERE l.tenant_id=NEW.tenant_id AND l.original_part_id=NEW.part_id)
 OR wip_piece_active_hold_v26(NEW.tenant_id,g.id)
 OR EXISTS(SELECT 1 FROM wip_exception_cases_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.part_id=NEW.part_id
   AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 cl WHERE cl.tenant_id=x.tenant_id AND cl.case_id=x.id))
 OR EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id AND q.garment_id=g.id AND q.action='QUALITY_RELEASE')
 OR EXISTS(SELECT 1 FROM pack_carton_events_v22 pk WHERE pk.tenant_id=NEW.tenant_id AND pk.garment_id=g.id AND pk.action='ADD')
 THEN RAISE EXCEPTION 'ERR_F317_PART_HELD_REPLACED_OR_FINISHED'; END IF;
 IF jobwork_part_offsite_v28(NEW.tenant_id,NEW.part_id) THEN RAISE EXCEPTION 'ERR_F317_ALREADY_OFFSITE'; END IF;
 SELECT * INTO prior FROM wip_movements_v20 WHERE tenant_id=NEW.tenant_id AND part_id=NEW.part_id ORDER BY event_no DESC LIMIT 1;
 IF prior.id IS NULL OR prior.action<>'ARRIVE' OR prior.station_id<>NEW.station_id
 THEN RAISE EXCEPTION 'ERR_F317_STATION_CUSTODY_REQUIRED'; END IF;
 PERFORM jobwork_require_scan_v28(NEW.tenant_id,NEW.plant_id,NEW.station_id,NEW.part_id,NEW.scan_id);
 IF NOT EXISTS(SELECT 1 FROM wip_gateway_scans_v20 x WHERE x.tenant_id=NEW.tenant_id
  AND x.id=NEW.scan_id AND x.observed_at>=prior.recorded_at AND x.received_at>=prior.recorded_at)
 THEN RAISE EXCEPTION 'ERR_F317_SCAN_PRECEDES_STATION_CUSTODY'; END IF;
 IF NEW.sender_proof_id IS NULL OR length(btrim(NEW.sender_proof_id))=0 THEN RAISE EXCEPTION 'ERR_F317_PROOF_REQUIRED'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_jobwork_vendor_ack_v30(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_jobwork_vendor_ack_v30() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE o jobwork_outwards_v28%ROWTYPE; k jobwork_vendor_keys_v30%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F317:OUTWARD:'||NEW.outward_id::text));
 SELECT * INTO o FROM jobwork_outwards_v28 WHERE tenant_id=NEW.tenant_id AND id=NEW.outward_id;
 SELECT * INTO k FROM jobwork_vendor_keys_v30 WHERE tenant_id=NEW.tenant_id
  AND plant_id=NEW.plant_id AND vendor_id=NEW.vendor_id AND key_id=NEW.key_id;
 IF o.id IS NULL OR o.plant_id<>NEW.plant_id OR o.vendor_id<>NEW.vendor_id
  OR o.part_id<>NEW.part_id OR o.process_code<>NEW.process_code OR o.job_reference<>NEW.job_reference
  OR k.key_id IS NULL OR NOT k.enabled OR
  NOT EXISTS(SELECT 1 FROM jobwork_vendors_v28 v WHERE v.tenant_id=NEW.tenant_id
   AND v.plant_id=NEW.plant_id AND v.vendor_id=NEW.vendor_id AND v.enabled)
 THEN RAISE EXCEPTION 'ERR_F317C_VENDOR_NOT_ENROLLED_OR_SCOPE'; END IF;
 IF NEW.key_sha256<>encode(digest(decode(k.public_key_hex,'hex'),'sha256'),'hex')
 THEN RAISE EXCEPTION 'ERR_F317C_KEY_HASH_MISMATCH'; END IF;
 IF NEW.recorder_subject=o.sender_subject OR EXISTS(SELECT 1 FROM jobwork_used_proofs_v28 p
  WHERE p.tenant_id=NEW.tenant_id AND p.proof_id=NEW.recorder_proof_id)
  OR EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 d
   WHERE d.tenant_id=NEW.tenant_id AND d.reporter_proof_id=NEW.recorder_proof_id)
 THEN RAISE EXCEPTION 'ERR_F317C_RECORDER_NOT_INDEPENDENT_OR_PROOF_SPENT'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_legacy_carrier_collision_v18(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_legacy_carrier_collision_v18() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 IF EXISTS (
  SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 c
   ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
  WHERE a.tenant_id=NEW.tenant_id AND c.epc_hex=NEW.epc_hex
 ) THEN RAISE EXCEPTION 'ERR_GEN_CARRIER_ALREADY_ACTIVE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_needle_safety_clearance_v33(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_needle_safety_clearance_v33() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE incident needle_safety_events_v32%ROWTYPE; followup needle_safety_events_v32%ROWTYPE; h char(64);
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 SELECT * INTO incident FROM needle_safety_events_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.incident_id;
 SELECT * INTO followup FROM needle_safety_events_v32 WHERE tenant_id=NEW.tenant_id AND id=NEW.followup_pass_id;
 SELECT sha256 INTO h FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.evidence_document_id;
 IF incident.id IS NULL OR incident.garment_id<>NEW.garment_id OR incident.plant_id<>NEW.plant_id
 OR incident.event_kind NOT IN('NEEDLE_BREAK','DETECTOR_CHALLENGE_FAIL','DETECTOR_REJECT')
 OR incident.actor_subject=NEW.reviewer_subject OR h IS NULL OR h<>NEW.evidence_sha256
 OR followup.id IS NULL OR followup.garment_id<>NEW.garment_id OR followup.plant_id<>NEW.plant_id
 OR followup.event_kind<>'DETECTOR_CHALLENGE_PASS' OR followup.recorded_at<=incident.recorded_at
 OR followup.actor_subject=NEW.reviewer_subject OR followup.actor_subject=incident.actor_subject
 OR NOT EXISTS(SELECT 1 FROM gen_garments_v18 g WHERE g.tenant_id=NEW.tenant_id AND g.id=NEW.garment_id AND g.plant_id=NEW.plant_id)
 OR (incident.event_kind='NEEDLE_BREAK' AND NEW.accounted_fragments<>incident.expected_fragments)
 OR (incident.event_kind<>'NEEDLE_BREAK' AND NEW.accounted_fragments<>0)
 THEN RAISE EXCEPTION 'ERR_F319C_INDEPENDENT_VERIFIED_FOLLOWUP_REQUIRED'; END IF;
 -- No reliance on client-side evidence or an arbitrary PASS. The PASS was itself
 -- admitted under the current calibrated-detector gate in F3.19B.
 IF EXISTS(SELECT 1 FROM needle_safety_clearances_v33 c WHERE c.tenant_id=NEW.tenant_id
   AND c.followup_pass_id=NEW.followup_pass_id AND c.incident_id<>NEW.incident_id)
 THEN RAISE EXCEPTION 'ERR_F319C_FOLLOWUP_ALREADY_CONSUMED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_needle_safety_event_v32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_needle_safety_event_v32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE g gen_garments_v18%ROWTYPE; h char(64);
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 SELECT sha256 INTO h FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.evidence_document_id;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF g.id IS NULL OR g.plant_id<>NEW.plant_id OR h IS NULL OR h<>NEW.evidence_sha256
 THEN RAISE EXCEPTION 'ERR_F319B_GARMENT_OR_EVIDENCE'; END IF;
 IF NEW.event_kind LIKE 'DETECTOR_%' AND (NEW.instrument_id IS NULL OR NOT EXISTS(
  SELECT 1 FROM factory_instrument_calibrations_v32 c WHERE c.tenant_id=NEW.tenant_id
   AND c.plant_id=NEW.plant_id AND c.instrument_id=NEW.instrument_id AND c.instrument_kind='METAL_DETECTOR'
   AND c.revision=(SELECT max(revision) FROM factory_instrument_calibrations_v32 x
    WHERE x.tenant_id=c.tenant_id AND x.plant_id=c.plant_id AND x.instrument_id=c.instrument_id)
   AND c.enabled AND clock_timestamp() BETWEEN c.valid_from AND c.valid_until)) THEN
  RAISE EXCEPTION 'ERR_F319B_DETECTOR_CALIBRATION_REQUIRED'; END IF;
 NEW.recorded_at:=clock_timestamp(); RETURN NEW;
END $$;


--
-- Name: guard_needle_safety_progress_v32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_needle_safety_progress_v32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF (TG_TABLE_NAME='qc_garment_events_v21' AND NEW.action<>'QUALITY_RELEASE') OR
    (TG_TABLE_NAME='pack_carton_events_v22' AND NEW.action<>'ADD') THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF garment_needle_safety_blocked_v32(NEW.tenant_id,NEW.garment_id) THEN
  RAISE EXCEPTION 'ERR_F319B_UNRESOLVED_GARMENT_SAFETY_HOLD'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_pack_carton_event_v22(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_pack_carton_event_v22() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE prior pack_carton_events_v22%ROWTYPE; opener pack_carton_events_v22%ROWTYPE;
 spec pack_carton_specs_v22%ROWTYPE; garment gen_garments_v18%ROWTYPE;
 qty bigint; net_expected bigint; last_quality text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:CARTON:'||NEW.carton_id::text));
 NEW.event_no:=(SELECT COALESCE(max(event_no),0)+1 FROM pack_carton_events_v22
   WHERE tenant_id=NEW.tenant_id AND carton_id=NEW.carton_id);
 SELECT * INTO prior FROM pack_carton_events_v22 WHERE tenant_id=NEW.tenant_id AND carton_id=NEW.carton_id
  ORDER BY event_no DESC LIMIT 1;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM pack_carton_specs_v22 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF spec.id IS NULL OR spec.plant_id<>NEW.plant_id OR NOT spec.enabled THEN RAISE EXCEPTION 'ERR_F35_PACK_SPEC_SCOPE_OR_REVOKED'; END IF;
 IF NEW.action='OPEN' THEN
  IF prior.id IS NOT NULL OR NOT spec.enabled OR length(btrim(NEW.carton_code))<3 THEN
   RAISE EXCEPTION 'ERR_F35_OPEN_REQUIRES_APPROVED_PLAN'; END IF;
 ELSE
  SELECT * INTO opener FROM pack_carton_events_v22 WHERE tenant_id=NEW.tenant_id
   AND carton_id=NEW.carton_id AND action='OPEN';
  IF prior.id IS NULL OR opener.id IS NULL OR opener.spec_id<>NEW.spec_id OR prior.action='SEAL'
   THEN RAISE EXCEPTION 'ERR_F35_CARTON_MISSING_SEALED_OR_WRONG_SPEC'; END IF;
  IF NEW.action='ADD' THEN
   SELECT count(*) INTO qty FROM pack_carton_events_v22 WHERE tenant_id=NEW.tenant_id
      AND carton_id=NEW.carton_id AND action='ADD';
   IF qty>=spec.max_units THEN RAISE EXCEPTION 'ERR_F35_CARTON_MAX_UNITS'; END IF;
   SELECT * INTO garment FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
   IF garment.id IS NULL OR garment.plant_id<>NEW.plant_id OR garment.lay_draft_id<>spec.lay_draft_id THEN
    RAISE EXCEPTION 'ERR_F35_GARMENT_SCOPE_OR_LAY'; END IF;
   SELECT q.action INTO last_quality FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
     AND q.garment_id=NEW.garment_id ORDER BY q.event_no DESC LIMIT 1;
   IF last_quality IS DISTINCT FROM 'QUALITY_RELEASE' THEN RAISE EXCEPTION 'ERR_F35_GARMENT_NOT_RELEASED'; END IF;
   IF NOT EXISTS(SELECT 1 FROM pack_size_weights_v22 w JOIN cut_lay_positions_v8 p
     ON p.tenant_id=w.tenant_id AND p.size_code=w.size_code
     WHERE w.tenant_id=NEW.tenant_id AND w.spec_id=NEW.spec_id
       AND p.lay_draft_id=garment.lay_draft_id AND p.marker_position=garment.marker_position)
     THEN RAISE EXCEPTION 'ERR_F35_SIZE_NOT_SPECIFIED'; END IF;
  ELSIF NEW.action='SEAL' THEN
   SELECT count(*),COALESCE(sum(w.expected_g),0) INTO qty,net_expected
    FROM pack_carton_events_v22 e JOIN gen_garments_v18 g ON g.tenant_id=e.tenant_id AND g.id=e.garment_id
    JOIN cut_lay_positions_v8 p ON p.tenant_id=g.tenant_id AND p.lay_draft_id=g.lay_draft_id AND p.marker_position=g.marker_position
    JOIN pack_size_weights_v22 w ON w.tenant_id=e.tenant_id AND w.spec_id=e.spec_id AND w.size_code=p.size_code
    WHERE e.tenant_id=NEW.tenant_id AND e.carton_id=NEW.carton_id AND e.action='ADD';
   IF qty<spec.min_units OR qty>spec.max_units OR prior.action<>'ADD'
    THEN RAISE EXCEPTION 'ERR_F35_INCOMPLETE_CARTON'; END IF;
   net_expected:=net_expected+spec.tare_g;
   IF NEW.expected_g<>net_expected OR abs(NEW.observed_g-net_expected)>spec.tolerance_g
     THEN RAISE EXCEPTION 'ERR_F35_WEIGHT_OUT_OF_TOLERANCE'; END IF;
   IF EXISTS(SELECT 1 FROM pack_carton_events_v22 e WHERE e.tenant_id=NEW.tenant_id
      AND e.carton_id=NEW.carton_id AND e.actor_subject=NEW.actor_subject) OR length(btrim(NEW.scale_reference))<3
     THEN RAISE EXCEPTION 'ERR_F35_INDEPENDENT_SEAL_OR_SCALE_REF'; END IF;
  END IF;
 END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_pack_manifest_event_v22(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_pack_manifest_event_v22() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE prior pack_manifest_events_v22%ROWTYPE; opener pack_manifest_events_v22%ROWTYPE;
 carton_open pack_carton_events_v22%ROWTYPE; carton_last pack_carton_events_v22%ROWTYPE;
 qty bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:MANIFEST:'||NEW.manifest_id::text));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:SPEC:'||NEW.spec_id::text));
 IF NOT EXISTS(SELECT 1 FROM pack_carton_specs_v22 s WHERE s.tenant_id=NEW.tenant_id
   AND s.id=NEW.spec_id AND s.plant_id=NEW.plant_id AND s.enabled) THEN
   RAISE EXCEPTION 'ERR_F35_MANIFEST_SPEC_REVOKED_OR_SCOPE'; END IF;
 NEW.event_no:=(SELECT COALESCE(max(event_no),0)+1 FROM pack_manifest_events_v22 WHERE tenant_id=NEW.tenant_id AND manifest_id=NEW.manifest_id);
 SELECT * INTO prior FROM pack_manifest_events_v22 WHERE tenant_id=NEW.tenant_id AND manifest_id=NEW.manifest_id ORDER BY event_no DESC LIMIT 1;
 IF NEW.action='OPEN' THEN
  IF prior.id IS NOT NULL OR length(btrim(NEW.manifest_ref))<3 OR length(btrim(NEW.destination_ref))<3
    OR NOT EXISTS(SELECT 1 FROM pack_carton_specs_v22 s WHERE s.tenant_id=NEW.tenant_id
      AND s.id=NEW.spec_id AND s.plant_id=NEW.plant_id AND s.enabled) THEN
   RAISE EXCEPTION 'ERR_F35_MANIFEST_REQUIRES_APPROVED_SPEC'; END IF;
 ELSE
  SELECT * INTO opener FROM pack_manifest_events_v22 WHERE tenant_id=NEW.tenant_id AND manifest_id=NEW.manifest_id AND action='OPEN';
  IF prior.id IS NULL OR opener.id IS NULL OR opener.spec_id<>NEW.spec_id OR opener.plant_id<>NEW.plant_id OR prior.action='STAGE' THEN
   RAISE EXCEPTION 'ERR_F35_MANIFEST_NOT_OPEN'; END IF;
  IF NEW.action='ADD_CARTON' THEN
   SELECT * INTO carton_open FROM pack_carton_events_v22 e WHERE e.tenant_id=NEW.tenant_id
    AND e.carton_id=NEW.carton_id AND e.action='OPEN';
   SELECT * INTO carton_last FROM pack_carton_events_v22 e WHERE e.tenant_id=NEW.tenant_id
    AND e.carton_id=NEW.carton_id ORDER BY event_no DESC LIMIT 1;
   IF carton_open.id IS NULL OR carton_open.spec_id<>NEW.spec_id OR carton_open.plant_id<>NEW.plant_id
    OR carton_last.action<>'SEAL' THEN RAISE EXCEPTION 'ERR_F35_UNSEALED_OR_WRONG_CARTON'; END IF;
  ELSIF NEW.action='STAGE' THEN
   SELECT count(*) INTO qty FROM pack_manifest_events_v22 e WHERE e.tenant_id=NEW.tenant_id
    AND e.manifest_id=NEW.manifest_id AND e.action='ADD_CARTON';
   IF qty=0 OR EXISTS(SELECT 1 FROM pack_manifest_events_v22 e WHERE e.tenant_id=NEW.tenant_id
      AND e.manifest_id=NEW.manifest_id AND e.actor_subject=NEW.actor_subject) THEN
    RAISE EXCEPTION 'ERR_F35_EMPTY_OR_NOT_INDEPENDENT_STAGING'; END IF;
  END IF;
 END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_pack_prod_hold_v12(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_pack_prod_hold_v12() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE src sew_prod_completions_v10%ROWTYPE; qc qc_prod_group_events_v11%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('QC11:'||NEW.completion_id::text));
 SELECT * INTO src FROM sew_prod_completions_v10 WHERE tenant_id=NEW.tenant_id AND id=NEW.completion_id;
 IF NOT FOUND OR src.plant_id<>NEW.plant_id THEN
  RAISE EXCEPTION 'ERR_PACK_PROD_SEWING_LINEAGE';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pack_prod_assignments_v12 a WHERE a.tenant_id=NEW.tenant_id
  AND a.plant_id=NEW.plant_id AND a.station_id=NEW.station_id
  AND a.worker_subject=NEW.actor_subject AND a.role='packing_operator' AND a.enabled) THEN
  RAISE EXCEPTION 'ERR_PACK_PROD_OPERATOR_ASSIGNMENT';
 END IF;
 SELECT * INTO qc FROM qc_prod_group_events_v11 WHERE tenant_id=NEW.tenant_id
  AND completion_id=NEW.completion_id ORDER BY revision DESC LIMIT 1;
 IF NOT FOUND OR qc.next_state<>'REVIEWED' OR qc.revision<>NEW.qc_revision THEN
  RAISE EXCEPTION 'ERR_PACK_PROD_QC_NOT_CURRENTLY_REVIEWED';
 END IF;
 NEW.qc_event_id:=qc.id;
 NEW.nominal_group_units:=src.nominal_ply_count;
 RETURN NEW;
END $$;


--
-- Name: guard_piece_review_scan_reuse_v26(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_piece_review_scan_reuse_v26() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||NEW.scan_id::text));
 IF EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=NEW.tenant_id AND r.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F316_REVIEW_SCAN_ALREADY_USED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_qc_exception_hold_v23(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_qc_exception_hold_v23() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF NEW.action='QUALITY_RELEASE' AND EXISTS(
  SELECT 1 FROM gen_cut_parts_v18 p JOIN wip_exception_cases_v23 c
   ON c.tenant_id=p.tenant_id AND c.part_id=p.id
  WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
   AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
     WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F310_UNRESOLVED_EXCEPTION_QC_HOLD'; END IF;
 IF NEW.action='QUALITY_RELEASE' AND EXISTS (
  SELECT 1 FROM gen_cut_parts_v18 original
   JOIN cut_recut_links_v19 link ON link.tenant_id=original.tenant_id AND link.original_part_id=original.id
   JOIN wip_exception_cases_v23 c ON c.tenant_id=link.tenant_id AND c.part_id=link.replacement_part_id
   WHERE original.tenant_id=NEW.tenant_id AND original.garment_id=NEW.garment_id
    AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F310_UNRESOLVED_RECUT_EXCEPTION_QC_HOLD'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_qc_garment_event_v21(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_qc_garment_event_v21() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$
DECLARE spec qc_garment_specs_v21%ROWTYPE; g gen_garments_v18%ROWTYPE;
 prior qc_garment_events_v21%ROWTYPE; prior_inspector varchar(128);
 size_text varchar(32); rule record; val_text text; measured bigint;
 required_count bigint:=0; provided_count bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 NEW.event_no := (SELECT COALESCE(max(event_no),0)+1 FROM qc_garment_events_v21
  WHERE tenant_id=NEW.tenant_id AND garment_id=NEW.garment_id);
 NEW.recorded_at := clock_timestamp();
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM qc_garment_specs_v21 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF g.id IS NULL OR spec.id IS NULL OR NOT spec.enabled OR g.plant_id<>NEW.plant_id
    OR spec.plant_id<>NEW.plant_id OR spec.lay_draft_id<>g.lay_draft_id THEN
  RAISE EXCEPTION 'ERR_F34_SPEC_SCOPE_OR_REVISION'; END IF;
 SELECT p.size_code INTO size_text FROM cut_lay_positions_v8 p
  WHERE p.tenant_id=NEW.tenant_id AND p.lay_draft_id=g.lay_draft_id AND p.marker_position=g.marker_position;
 IF size_text IS NULL THEN RAISE EXCEPTION 'ERR_F34_SIZE_MISSING'; END IF;
 SELECT * INTO prior FROM qc_garment_events_v21
  WHERE tenant_id=NEW.tenant_id AND garment_id=NEW.garment_id ORDER BY event_no DESC LIMIT 1;
 IF prior.id IS NOT NULL AND prior.spec_id<>NEW.spec_id THEN RAISE EXCEPTION 'ERR_F34_MIXED_SPEC_REVISIONS'; END IF;
 IF NEW.action='INSPECT_PASS' OR NEW.action='INSPECT_HOLD' THEN
  IF prior.id IS NOT NULL THEN RAISE EXCEPTION 'ERR_F34_ALREADY_INSPECTED'; END IF;
 ELSIF NEW.action='REPAIR_COMPLETE' THEN
  IF prior.id IS NULL OR prior.action NOT IN ('INSPECT_HOLD','REINSPECT_HOLD')
     OR prior.actor_subject=NEW.actor_subject THEN RAISE EXCEPTION 'ERR_F34_REPAIR_SEQUENCE'; END IF;
 ELSIF NEW.action IN ('REINSPECT_PASS','REINSPECT_HOLD') THEN
  IF prior.id IS NULL OR prior.action<>'REPAIR_COMPLETE' OR prior.actor_subject=NEW.actor_subject THEN
   RAISE EXCEPTION 'ERR_F34_REINSPECTION_SEQUENCE'; END IF;
 ELSIF NEW.action='FINISHING_COMPLETE' THEN
  IF prior.id IS NULL OR prior.action NOT IN ('INSPECT_PASS','REINSPECT_PASS')
     OR prior.actor_subject=NEW.actor_subject THEN RAISE EXCEPTION 'ERR_F34_FINISHING_SEQUENCE'; END IF;
 ELSIF NEW.action='QUALITY_RELEASE' THEN
  IF prior.id IS NULL OR prior.action<>'FINISHING_COMPLETE' OR prior.actor_subject=NEW.actor_subject THEN
   RAISE EXCEPTION 'ERR_F34_RELEASE_SEQUENCE'; END IF;
 ELSE RAISE EXCEPTION 'ERR_F34_UNKNOWN_ACTION'; END IF;
 IF NEW.action IN ('INSPECT_PASS','INSPECT_HOLD','REINSPECT_PASS','REINSPECT_HOLD','FINISHING_COMPLETE','QUALITY_RELEASE')
    AND NOT qc_garment_parts_ready_v21(NEW.tenant_id,NEW.garment_id,spec.finish_station) THEN
   RAISE EXCEPTION 'ERR_F34_PARTS_NOT_AT_FINISH_STATION'; END IF;
 IF NEW.action IN ('INSPECT_HOLD','REINSPECT_HOLD') AND NOT EXISTS (
  SELECT 1 FROM qc_garment_defects_v21 d WHERE d.tenant_id=NEW.tenant_id
   AND d.plant_id=NEW.plant_id AND d.defect_code=NEW.defect_code AND d.enabled) THEN
   RAISE EXCEPTION 'ERR_F34_DEFECT_CODE_NOT_APPROVED'; END IF;
 IF NEW.action IN ('INSPECT_PASS','REINSPECT_PASS') THEN
  SELECT count(*) INTO provided_count FROM jsonb_object_keys(NEW.measurements_mm);
  FOR rule IN SELECT measure_code,nominal_mm,lower_tolerance_mm,upper_tolerance_mm
   FROM qc_garment_measure_rules_v21 WHERE tenant_id=NEW.tenant_id
   AND spec_id=NEW.spec_id AND size_code=size_text LOOP
   required_count:=required_count+1;
   val_text:=NEW.measurements_mm->>rule.measure_code;
   IF val_text IS NULL OR val_text !~ '^[0-9]{1,12}$' THEN
    RAISE EXCEPTION 'ERR_F34_MISSING_OR_NONINTEGER_MEASUREMENT'; END IF;
   measured:=val_text::bigint;
   IF measured<rule.nominal_mm-rule.lower_tolerance_mm OR measured>rule.nominal_mm+rule.upper_tolerance_mm THEN
    RAISE EXCEPTION 'ERR_F34_MEASUREMENT_OUT_OF_TOLERANCE'; END IF;
  END LOOP;
  IF required_count=0 OR provided_count<>required_count THEN RAISE EXCEPTION 'ERR_F34_UNGOVERNED_MEASUREMENTS'; END IF;
 ELSE
  IF NEW.measurements_mm<>'{}'::jsonb THEN RAISE EXCEPTION 'ERR_F34_MEASUREMENTS_ON_NONPASS'; END IF;
 END IF;
 RETURN NEW;
END $_$;


--
-- Name: guard_qc_piece_hold_v26(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_qc_piece_hold_v26() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF NEW.action IN ('INSPECT_PASS','REINSPECT_PASS','FINISHING_COMPLETE','QUALITY_RELEASE') AND wip_piece_active_hold_v26(NEW.tenant_id,NEW.garment_id)
 THEN RAISE EXCEPTION 'ERR_F316_PIECE_HOLD_QC'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_qc_prod_group_event_v11(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_qc_prod_group_event_v11() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE source_row sew_prod_completions_v10%ROWTYPE;
        previous qc_prod_group_events_v11%ROWTYPE;
        bad_actor text;
        needed_role text;
        next_value text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('QC11:'||NEW.completion_id::text));
 SELECT * INTO source_row FROM sew_prod_completions_v10
 WHERE tenant_id=NEW.tenant_id AND id=NEW.completion_id;
 IF NOT FOUND OR source_row.plant_id<>NEW.plant_id THEN
  RAISE EXCEPTION 'ERR_QC_PROD_SEWING_LINEAGE';
 END IF;
 IF NEW.action='REPAIR_COMPLETE' THEN needed_role:='quality_repairer';
 ELSE needed_role:='quality_inspector'; END IF;
 IF NOT EXISTS(SELECT 1 FROM qc_prod_assignments_v11 a
  WHERE a.tenant_id=NEW.tenant_id AND a.plant_id=NEW.plant_id
   AND a.station_id=NEW.station_id AND a.worker_subject=NEW.actor_subject
   AND a.role=needed_role AND a.enabled) THEN
  RAISE EXCEPTION 'ERR_QC_PROD_ASSIGNMENT';
 END IF;
 SELECT * INTO previous FROM qc_prod_group_events_v11
  WHERE tenant_id=NEW.tenant_id AND completion_id=NEW.completion_id
  ORDER BY revision DESC LIMIT 1;
 IF FOUND THEN
  NEW.prior_state:=previous.next_state;
  NEW.revision:=previous.revision+1;
 ELSE
  NEW.prior_state:='PENDING'; NEW.revision:=1;
 END IF;
 IF NEW.action IN ('INSPECT_PASS','REINSPECT_PASS') THEN
  IF NEW.inspected_units<>source_row.nominal_ply_count OR NEW.defective_units<>0 OR NEW.defect_code<>'' THEN
   RAISE EXCEPTION 'ERR_QC_PROD_FULL_INSPECTION_REQUIRED';
  END IF;
 ELSIF NEW.action IN ('INSPECT_HOLD','REINSPECT_HOLD') THEN
  IF NEW.inspected_units<1 OR NEW.inspected_units>source_row.nominal_ply_count OR
    NEW.defective_units<1 OR NEW.defective_units>NEW.inspected_units OR NEW.defect_code='' OR
    NOT EXISTS(SELECT 1 FROM qc_prod_defect_catalog_v11 c WHERE
       c.tenant_id=NEW.tenant_id AND c.plant_id=NEW.plant_id
       AND c.code=NEW.defect_code AND c.active) THEN
   RAISE EXCEPTION 'ERR_QC_PROD_DEFECT_OR_COUNTS';
  END IF;
 ELSE
  IF NEW.inspected_units<>0 OR NEW.defective_units<>0 OR NEW.defect_code<>'' OR length(trim(NEW.note))=0 THEN
   RAISE EXCEPTION 'ERR_QC_PROD_REPAIR_EVIDENCE';
  END IF;
 END IF;
 IF NEW.prior_state='PENDING' AND NEW.action IN ('INSPECT_PASS','INSPECT_HOLD') THEN
  IF NEW.action='INSPECT_PASS' THEN next_value:='REVIEWED'; ELSE next_value:='HOLD'; END IF;
 ELSIF NEW.prior_state='REVIEWED' AND NEW.action='INSPECT_HOLD' THEN
  next_value:='HOLD';
 ELSIF NEW.prior_state='HOLD' AND NEW.action='REPAIR_COMPLETE' AND NEW.actor_subject<>previous.actor_subject THEN
  next_value:='REINSPECT';
 ELSIF NEW.prior_state='REINSPECT' AND NEW.action IN ('REINSPECT_PASS','REINSPECT_HOLD') THEN
  SELECT e.actor_subject INTO bad_actor FROM qc_prod_group_events_v11 e
  WHERE e.tenant_id=NEW.tenant_id AND e.completion_id=NEW.completion_id
   AND e.revision=previous.revision-1 AND e.next_state='HOLD';
  IF NEW.actor_subject=previous.actor_subject OR NEW.actor_subject=bad_actor OR bad_actor IS NULL THEN
   RAISE EXCEPTION 'ERR_QC_PROD_INDEPENDENT_REINSPECTION';
  END IF;
  IF NEW.action='REINSPECT_PASS' THEN next_value:='REVIEWED'; ELSE next_value:='HOLD'; END IF;
 ELSE
  RAISE EXCEPTION 'ERR_QC_PROD_STATE_TRANSITION';
 END IF;
 NEW.next_state:=next_value;
 RETURN NEW;
END $$;


--
-- Name: guard_qc_spec_v21(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_qc_spec_v21() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE doc_hash char(64);
BEGIN
 IF NEW.enabled THEN RAISE EXCEPTION 'ERR_F34_SPEC_ACTIVATE_AFTER_RULES'; END IF;
 SELECT sha256 INTO doc_hash FROM document_sources_v65
 WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF doc_hash IS NULL OR doc_hash<>NEW.source_sha256 THEN
  RAISE EXCEPTION 'ERR_F34_SPEC_SOURCE_MISMATCH'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_sewing_releases_v8 r WHERE r.tenant_id=NEW.tenant_id
  AND r.lay_draft_id=NEW.lay_draft_id AND r.plant_id=NEW.plant_id) THEN
  RAISE EXCEPTION 'ERR_F34_SPEC_SIGNED_LAY_REQUIRED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_sew_completion_piece_hold_v26(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_sew_completion_piece_hold_v26() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||NEW.lay_draft_id::text||':'||NEW.marker_position||':'||NEW.ply_group));
 IF EXISTS(SELECT 1 FROM gen_garments_v18 g JOIN wip_piece_holds_v26 h
   ON h.tenant_id=g.tenant_id AND h.garment_id=g.id
  WHERE g.tenant_id=NEW.tenant_id AND g.lay_draft_id=NEW.lay_draft_id
   AND g.marker_position=NEW.marker_position AND h.plant_id=NEW.plant_id
   AND NOT EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=h.tenant_id AND r.hold_id=h.id)
   AND EXISTS(SELECT 1 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=g.tenant_id
    AND b.lay_draft_id=g.lay_draft_id AND b.marker_position=g.marker_position
    AND b.ply_group=NEW.ply_group AND g.ply_number BETWEEN b.ply_start AND b.ply_end))
 THEN RAISE EXCEPTION 'ERR_F316_NO_FULL_BUNDLE_COMPLETION_DURING_PIECE_HOLD'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_sew_piece_completion_v27(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_sew_piece_completion_v27() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE g gen_garments_v18%ROWTYPE; route sew_prod_routes_v10%ROWTYPE;
        scan wip_gateway_scans_v20%ROWTYPE;
        sid uuid; pid uuid; group_no integer; n_group integer;
        code varchar(64); slot_no integer; part_owner uuid;
        accepted boolean; replacement boolean; used_codes text[] := ARRAY[]::text[];
        prev_code text; arrival_action text; arrival_station text; arrival_at timestamptz;
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id OR g.lay_draft_id<>NEW.lay_draft_id
  OR g.release_id<>NEW.release_id OR g.marker_position<>NEW.marker_position
 THEN RAISE EXCEPTION 'ERR_F316B_GARMENT_SCOPE'; END IF;
 SELECT * INTO route FROM sew_prod_routes_v10 WHERE tenant_id=NEW.tenant_id AND id=NEW.route_id;
 IF NOT FOUND OR NOT route.enabled OR route.lay_draft_id<>NEW.lay_draft_id
  OR route.plant_id<>NEW.plant_id OR route.station_id<>NEW.station_id
 THEN RAISE EXCEPTION 'ERR_F316B_ROUTE_SCOPE'; END IF;
 IF cardinality(route.required_parts)<>cardinality(NEW.scan_ids) OR
   (SELECT count(DISTINCT x) FROM unnest(route.required_parts) x)<>cardinality(route.required_parts) OR
   (SELECT count(DISTINCT x) FROM unnest(NEW.scan_ids) x)<>cardinality(NEW.scan_ids)
 THEN RAISE EXCEPTION 'ERR_F316B_REQUIRED_SCAN_SET'; END IF;

 FOR pid IN SELECT part_id FROM (
  SELECT p.id part_id FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
  UNION
  SELECT link.replacement_part_id FROM cut_recut_links_v19 link
   JOIN gen_cut_parts_v18 p ON p.tenant_id=link.tenant_id AND p.id=link.original_part_id
   WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
 ) parts ORDER BY part_id LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||pid::text));
 END LOOP;
 -- Share the RFID bind/retire serialization lock with F3.1/F3.3.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT count(DISTINCT b.ply_group),min(b.ply_group) INTO n_group,group_no
 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=NEW.tenant_id
  AND b.lay_draft_id=NEW.lay_draft_id AND b.marker_position=NEW.marker_position
  AND g.ply_number BETWEEN b.ply_start AND b.ply_end;
 IF n_group<>1 OR group_no<>NEW.ply_group THEN RAISE EXCEPTION 'ERR_F316B_PLY_GROUP'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||NEW.lay_draft_id::text||':'||NEW.marker_position||':'||NEW.ply_group));
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF wip_piece_active_hold_v26(NEW.tenant_id,NEW.garment_id) THEN
  RAISE EXCEPTION 'ERR_F316B_PIECE_HELD'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.worker_subject AND w.enabled) THEN
  RAISE EXCEPTION 'ERR_F316B_WORKER_NOT_ASSIGNED'; END IF;
 -- Operation codes, not only route IDs: a published revision cannot grant
 -- the same garment a second completion for the same logical operation.
 IF EXISTS(SELECT 1 FROM sew_piece_completions_v27 prior
  JOIN sew_prod_routes_v10 pr ON pr.tenant_id=prior.tenant_id AND pr.id=prior.route_id
  WHERE prior.tenant_id=NEW.tenant_id AND prior.garment_id=NEW.garment_id
   AND pr.lay_draft_id=NEW.lay_draft_id AND pr.plant_id=NEW.plant_id
   AND pr.operation_code=route.operation_code)
 THEN RAISE EXCEPTION 'ERR_F316B_OPERATION_ALREADY_COMPLETED'; END IF;
 IF EXISTS(SELECT 1 FROM sew_prod_completions_v10 whole
  JOIN sew_prod_routes_v10 prior ON prior.tenant_id=whole.tenant_id AND prior.id=whole.route_id
  WHERE whole.tenant_id=NEW.tenant_id AND whole.marker_position=NEW.marker_position
   AND whole.ply_group=NEW.ply_group AND prior.plant_id=NEW.plant_id
   AND prior.lay_draft_id=NEW.lay_draft_id AND prior.operation_code=route.operation_code)
 THEN RAISE EXCEPTION 'ERR_F316B_WHOLE_GROUP_ALREADY_COMPLETED'; END IF;
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.garment_id=NEW.garment_id AND q.action='QUALITY_RELEASE')
 THEN RAISE EXCEPTION 'ERR_F316B_ALREADY_QC_RELEASED'; END IF;

 FOR prev_code IN SELECT DISTINCT unnest(route.predecessors) LOOP
  IF NOT EXISTS(SELECT 1 FROM sew_piece_completions_v27 c
   JOIN sew_prod_routes_v10 prior ON prior.tenant_id=c.tenant_id AND prior.id=c.route_id
   WHERE c.tenant_id=NEW.tenant_id AND c.garment_id=NEW.garment_id
    AND prior.plant_id=NEW.plant_id AND prior.lay_draft_id=NEW.lay_draft_id
    AND prior.operation_code=prev_code AND prior.enabled)
  AND NOT EXISTS(SELECT 1 FROM sew_prod_completions_v10 c
   JOIN sew_prod_routes_v10 prior ON prior.tenant_id=c.tenant_id AND prior.id=c.route_id
   WHERE c.tenant_id=NEW.tenant_id AND c.marker_position=NEW.marker_position AND c.ply_group=NEW.ply_group
    AND prior.plant_id=NEW.plant_id AND prior.lay_draft_id=NEW.lay_draft_id
    AND prior.operation_code=prev_code AND prior.enabled)
  THEN RAISE EXCEPTION 'ERR_F316B_PREDECESSOR'; END IF;
 END LOOP;

 FOR sid IN SELECT unnest(NEW.scan_ids) ORDER BY 1 LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||sid::text));
  SELECT * INTO scan FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=sid;
  IF NOT FOUND OR scan.plant_id<>NEW.plant_id OR scan.station_id<>NEW.station_id
   OR scan.observed_at<clock_timestamp()-interval '15 minutes'
   OR scan.received_at<clock_timestamp()-interval '15 minutes'
   OR scan.observed_at>clock_timestamp()+interval '2 minutes'
   OR scan.received_at>clock_timestamp()
   OR scan.observed_at<route.created_at-interval '2 minutes'
  THEN RAISE EXCEPTION 'ERR_F316B_FRESH_SCOPED_SCAN_REQUIRED'; END IF;
  IF NOT EXISTS(SELECT 1 FROM sew_prod_gateways_v10 gw
   WHERE gw.tenant_id=NEW.tenant_id AND gw.gateway_id=scan.gateway_id
    AND gw.plant_id=NEW.plant_id AND gw.station_id=NEW.station_id AND gw.enabled)
  THEN RAISE EXCEPTION 'ERR_F316B_GATEWAY_DISABLED'; END IF;
  IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a
    JOIN gen_carriers_v18 carrier ON carrier.tenant_id=a.tenant_id AND carrier.id=a.carrier_id
   WHERE a.tenant_id=NEW.tenant_id AND a.target_type='CUT_PART' AND a.target_id=scan.part_id
    AND a.plant_id=NEW.plant_id AND a.carrier_id=scan.carrier_id
    AND a.bound_event_id=scan.binding_event_id AND carrier.enabled AND carrier.epc_hex=scan.epc_hex)
  THEN RAISE EXCEPTION 'ERR_F316B_STALE_CARRIER'; END IF;

  -- The accepted part is either original and not replaced, or the single
  -- linked replacement of an original part belonging to this garment.
  SELECT p.part_code,p.part_slot,p.garment_id,
    EXISTS(SELECT 1 FROM cut_part_outcomes_v19 out WHERE out.tenant_id=p.tenant_id
     AND out.part_id=p.id AND out.disposition='ACCEPTED'),
    EXISTS(SELECT 1 FROM cut_recut_links_v19 link WHERE link.tenant_id=p.tenant_id AND link.replacement_part_id=p.id)
   INTO code,slot_no,part_owner,accepted,replacement
   FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.id=scan.part_id;
  IF NOT FOUND OR NOT accepted OR NOT (code=ANY(route.required_parts)) THEN
   RAISE EXCEPTION 'ERR_F316B_PART_NOT_ACCEPTED'; END IF;
  IF replacement THEN
   IF NOT EXISTS(SELECT 1 FROM cut_recut_links_v19 link JOIN gen_cut_parts_v18 original
      ON original.tenant_id=link.tenant_id AND original.id=link.original_part_id
     WHERE link.tenant_id=NEW.tenant_id AND link.replacement_part_id=scan.part_id
      AND original.garment_id=NEW.garment_id AND original.part_code=code AND original.part_slot=slot_no)
   THEN RAISE EXCEPTION 'ERR_F316B_RECUT_LINEAGE'; END IF;
  ELSIF part_owner<>NEW.garment_id OR EXISTS(SELECT 1 FROM cut_recut_links_v19 link
     WHERE link.tenant_id=NEW.tenant_id AND link.original_part_id=scan.part_id)
  THEN RAISE EXCEPTION 'ERR_F316B_ORIGINAL_REPLACED'; END IF;
  IF code=ANY(used_codes) THEN RAISE EXCEPTION 'ERR_F316B_DUPLICATE_PART_CODE'; END IF;
  used_codes:=array_append(used_codes,code);
  SELECT move.action,move.station_id,move.recorded_at INTO arrival_action,arrival_station,arrival_at
   FROM wip_movements_v20 move WHERE move.tenant_id=NEW.tenant_id AND move.part_id=scan.part_id
   ORDER BY move.event_no DESC LIMIT 1;
  IF NOT FOUND OR arrival_action<>'ARRIVE' OR arrival_station<>NEW.station_id
    OR scan.observed_at<arrival_at OR scan.received_at<arrival_at
  THEN RAISE EXCEPTION 'ERR_F316B_STATION_CUSTODY_REQUIRED'; END IF;
  IF EXISTS(SELECT 1 FROM wip_movements_v20 move WHERE move.tenant_id=NEW.tenant_id AND move.scan_id=sid)
   OR EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.scan_id=sid)
   OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 review WHERE review.tenant_id=NEW.tenant_id AND review.scan_id=sid)
   OR EXISTS(SELECT 1 FROM sew_piece_used_scans_v27 used WHERE used.tenant_id=NEW.tenant_id AND used.scan_id=sid)
  THEN RAISE EXCEPTION 'ERR_F316B_SCAN_ALREADY_USED'; END IF;
  IF EXISTS(SELECT 1 FROM wip_exception_cases_v23 x WHERE x.tenant_id=NEW.tenant_id
    AND x.part_id=scan.part_id AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 cl
       WHERE cl.tenant_id=x.tenant_id AND cl.case_id=x.id))
  THEN RAISE EXCEPTION 'ERR_F316B_WIP_EXCEPTION_OPEN'; END IF;
 END LOOP;
 IF cardinality(used_codes)<>cardinality(route.required_parts)
 THEN RAISE EXCEPTION 'ERR_F316B_MISSING_REQUIRED_PART'; END IF;
 NEW.recorded_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_sew_prod_completion_v10(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_sew_prod_completion_v10() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE r sew_prod_routes_v10%ROWTYPE; rel cut_sewing_releases_v8%ROWTYPE;
    seen integer; unique_parts integer; starts integer; ends integer; colors integer; sizes integer;
    low_ply integer; high_ply integer; unmet integer; obs uuid;
BEGIN
 SELECT * INTO r FROM sew_prod_routes_v10 WHERE tenant_id=NEW.tenant_id AND id=NEW.route_id;
 IF NOT FOUND OR NOT r.enabled OR r.plant_id<>NEW.plant_id OR r.station_id<>NEW.station_id OR r.lay_draft_id<>NEW.lay_draft_id THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_ROUTE_SCOPE';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM sew_prod_worker_stations_v10 ws
   WHERE ws.tenant_id=NEW.tenant_id AND ws.plant_id=NEW.plant_id
     AND ws.station_id=NEW.station_id AND ws.worker_subject=NEW.worker_subject AND ws.enabled) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_WORKER_NOT_ASSIGNED';
 END IF;
 SELECT * INTO rel FROM cut_sewing_releases_v8 WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR rel.plant_id<>NEW.plant_id OR rel.lay_draft_id<>NEW.lay_draft_id THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_SIGNED_RELEASE';
 END IF;
 IF (SELECT count(DISTINCT x) FROM unnest(r.required_parts) x)<>cardinality(r.required_parts) OR
    (SELECT count(DISTINCT x) FROM unnest(NEW.observation_ids) x)<>cardinality(NEW.observation_ids) OR
    cardinality(NEW.observation_ids)<>cardinality(r.required_parts) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_PARTS';
 END IF;
 SELECT count(*)::int,count(DISTINCT o.part_code)::int,count(DISTINCT b.ply_start)::int,
   count(DISTINCT b.ply_end)::int,count(DISTINCT b.color_code)::int,count(DISTINCT b.size_code)::int,
   min(b.ply_start),max(b.ply_end)
 INTO seen,unique_parts,starts,ends,colors,sizes,low_ply,high_ply
 FROM unnest(NEW.observation_ids) ids(obs_id)
 JOIN sew_prod_observations_v10 o ON o.tenant_id=NEW.tenant_id AND o.id=ids.obs_id
 JOIN cut_bundle_drafts_v8 b ON b.tenant_id=o.tenant_id AND b.lay_draft_id=o.lay_draft_id AND b.label_code=o.label_code
 JOIN sew_prod_gateways_v10 g ON g.tenant_id=o.tenant_id AND g.gateway_id=o.gateway_id AND g.enabled
 WHERE o.lay_draft_id=NEW.lay_draft_id AND o.release_id=NEW.release_id AND
       o.plant_id=NEW.plant_id AND o.station_id=NEW.station_id AND
       o.marker_position=NEW.marker_position AND o.ply_group=NEW.ply_group AND
       o.part_code=ANY(r.required_parts) AND g.plant_id=NEW.plant_id AND g.station_id=NEW.station_id
       AND o.observed_at >= r.created_at - interval '2 minutes';
 IF seen<>cardinality(r.required_parts) OR unique_parts<>seen OR starts<>1 OR ends<>1 OR
    colors<>1 OR sizes<>1 OR NEW.nominal_ply_count<>high_ply-low_ply+1 THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_INCOMPLETE_GENEALOGY';
 END IF;
 SELECT count(*)::int INTO unmet FROM unnest(r.predecessors) p(code)
 WHERE NOT EXISTS (
   SELECT 1 FROM sew_prod_routes_v10 pr JOIN sew_prod_completions_v10 c
     ON c.tenant_id=pr.tenant_id AND c.route_id=pr.id
   WHERE pr.tenant_id=NEW.tenant_id AND pr.lay_draft_id=NEW.lay_draft_id
     AND pr.plant_id=NEW.plant_id AND pr.operation_code=p.code AND pr.enabled
     AND c.marker_position=NEW.marker_position AND c.ply_group=NEW.ply_group);
 IF unmet>0 THEN RAISE EXCEPTION 'ERR_SEW_PROD_PREDECESSOR'; END IF;
 -- An RF observation is evidence for one operation at one station, not a
 -- reusable voucher. Deterministic per-observation locks avoid race/double use
 -- even when two different routes complete at the same time.
 FOR obs IN SELECT unnest(NEW.observation_ids) ORDER BY 1 LOOP
   PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id), hashtext('SEW_OBS:'||obs::text));
 END LOOP;
 IF EXISTS (SELECT 1 FROM sew_prod_completions_v10 existing
   WHERE existing.tenant_id=NEW.tenant_id AND existing.observation_ids && NEW.observation_ids) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_OBSERVATION_ALREADY_CONSUMED';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_sew_prod_observation_v10(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_sew_prod_observation_v10() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE rel cut_sewing_releases_v8%ROWTYPE; matched integer;
BEGIN
 IF NOT EXISTS (
  SELECT 1 FROM sew_prod_gateways_v10 g
  WHERE g.tenant_id=NEW.tenant_id AND g.gateway_id=NEW.gateway_id
    AND g.enabled AND g.plant_id=NEW.plant_id AND g.station_id=NEW.station_id) THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_GATEWAY_SCOPE';
 END IF;
 SELECT * INTO rel FROM cut_sewing_releases_v8
  WHERE tenant_id=NEW.tenant_id AND id=NEW.release_id;
 IF NOT FOUND OR rel.plant_id<>NEW.plant_id OR rel.lay_draft_id<>NEW.lay_draft_id OR
    NEW.observed_at < rel.created_at - interval '2 minutes' THEN
   RAISE EXCEPTION 'ERR_SEW_PROD_CUT_RELEASE';
 END IF;
 SELECT count(*)::int INTO matched FROM cut_bundle_drafts_v8 b
 JOIN cut_bundle_scan_witnesses_v8 w
   ON w.tenant_id=b.tenant_id AND w.lay_draft_id=b.lay_draft_id AND w.label_code=b.label_code
 WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id
  AND b.label_code=NEW.label_code AND w.epc_hex=NEW.epc_hex
  AND w.spread_report_id=rel.spread_report_id AND w.plant_id=NEW.plant_id
  AND b.marker_position=NEW.marker_position AND b.ply_group=NEW.ply_group AND b.part_code=NEW.part_code;
 IF matched<>1 THEN RAISE EXCEPTION 'ERR_SEW_PROD_BUNDLE_LINEAGE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_sew_sim_cycle_transition_v9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_sew_sim_cycle_transition_v9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'ERR_SEW_SIM_CYCLE_DELETE'; END IF;
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.id<>OLD.id OR
    NEW.claim_id<>OLD.claim_id OR NEW.claim_sha256<>OLD.claim_sha256 OR
    NEW.operation_id<>OLD.operation_id OR NEW.lay_draft_id<>OLD.lay_draft_id OR
    NEW.marker_position<>OLD.marker_position OR NEW.ply_group<>OLD.ply_group OR
    NEW.plant_id<>OLD.plant_id OR NEW.station_id<>OLD.station_id OR NEW.worker_id<>OLD.worker_id OR
    NEW.machine_class<>OLD.machine_class OR NEW.claimed_at<>OLD.claimed_at OR
    NEW.active_ms<OLD.active_ms OR NEW.paused_ms<OLD.paused_ms THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CYCLE_IDENTITY_OR_DURATION_TAMPER';
 END IF;
 IF NOT ((OLD.state='CLAIMED' AND NEW.state IN('PAUSED','COMPLETED','REVIEW_REQUIRED'))
   OR (OLD.state='PAUSED' AND NEW.state IN('CLAIMED','REVIEW_REQUIRED'))) THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_INVALID_STATE_TRANSITION';
 END IF;
 IF (NEW.state='CLAIMED' AND (NEW.active_since IS NULL OR NEW.paused_since IS NOT NULL)) OR
    (NEW.state='PAUSED' AND (NEW.active_since IS NOT NULL OR NEW.paused_since IS NULL)) OR
    (NEW.state IN('COMPLETED','REVIEW_REQUIRED') AND (NEW.active_since IS NOT NULL OR NEW.paused_since IS NOT NULL)) OR
    (NEW.state='COMPLETED' AND NEW.finished_at IS NULL) THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CYCLE_CLOCK_STATE';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_trim_admission_v25(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_trim_admission_v25() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
DECLARE source_status text; source_hash text; source_type text; manager_ok bool; inspector_ok bool;
BEGIN
 IF NEW.admitted_at < clock_timestamp()-interval '5 minutes' OR
    NEW.admitted_at > clock_timestamp()+interval '60 seconds' THEN
   RAISE EXCEPTION 'ERR_F314B_ADMISSION_TIME';
 END IF;
 SELECT d.status,d.file_checksum,d.document_type INTO source_status,source_hash,source_type
 FROM public.documents d WHERE d.tenant_id=NEW.tenant_id AND d.id=NEW.source_document_id;
 IF source_status IS DISTINCT FROM 'APPROVED' OR source_hash IS DISTINCT FROM NEW.source_sha256 OR
    source_type NOT IN ('TRIM_RECEIPT','DELIVERY_CHALLAN','SUPPLIER_INVOICE','PACKING_LIST') THEN
   RAISE EXCEPTION 'ERR_F314B_APPROVED_SOURCE_REQUIRED';
 END IF;
 SELECT EXISTS(SELECT 1 FROM public.workforce_identity_grants_v15 g
   JOIN public.workforce_chapter_actions_v17 p ON p.permission=g.permission AND p.chapter_id=4 AND p.authority='SIGNED_WRITE'
   WHERE g.tenant_id=NEW.tenant_id AND g.plant_id=NEW.plant_id
     AND g.idp_subject=NEW.manager_subject AND g.idp_issuer=NEW.manager_issuer
     AND g.permission='stores.trims.admit' AND g.enabled) INTO manager_ok;
 SELECT EXISTS(SELECT 1 FROM public.workforce_identity_grants_v15 g
   JOIN public.workforce_chapter_actions_v17 p ON p.permission=g.permission AND p.chapter_id=4 AND p.authority='SIGNED_WRITE'
   WHERE g.tenant_id=NEW.tenant_id AND g.plant_id=NEW.plant_id
     AND g.idp_subject=NEW.inspector_subject AND g.idp_issuer=NEW.inspector_issuer
     AND g.permission='stores.trims.inspect' AND g.enabled) INTO inspector_ok;
 IF NOT manager_ok OR NOT inspector_ok THEN
   RAISE EXCEPTION 'ERR_F314B_WORKFORCE_GRANTS_REQUIRED';
 END IF;
 IF EXISTS (SELECT 1 FROM public.trim_issuance_proof_uses_v24 u
       WHERE u.tenant_id=NEW.tenant_id AND u.proof_id IN(NEW.manager_proof_id,NEW.inspector_proof_id)) THEN
   RAISE EXCEPTION 'ERR_F314B_ISSUANCE_PROOF_REUSE';
 END IF;
 INSERT INTO public.trim_admission_proof_uses_v25(tenant_id,proof_id,admission_id,role)
 VALUES(NEW.tenant_id,NEW.manager_proof_id,NEW.id,'MANAGER'),
       (NEW.tenant_id,NEW.inspector_proof_id,NEW.id,'INSPECTOR');
 -- New lot only; no adjustment/top-up and no implied stock receipt on conflict.
 INSERT INTO public.store_trims_v13(tenant_id,id,plant_id,trim_code,description,supplier_lot,
    color_code,size_code,uom,received_qty,remaining_qty,status,bin_location)
 VALUES(NEW.tenant_id,NEW.stock_id,NEW.plant_id,NEW.trim_code,NEW.description,
    NEW.supplier_lot,NEW.color_code,NEW.size_code,NEW.uom,NEW.received_qty,NEW.received_qty,'AVAILABLE',NEW.bin_location);
 RETURN NULL;
END $$;


--
-- Name: guard_trim_issuance_v24(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_trim_issuance_v24() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $_$
DECLARE current_balance numeric(12,3); current_status text; current_plant text;
BEGIN
 IF NEW.plant_id IS NULL OR btrim(NEW.plant_id)='' OR
    NEW.command_id IS NULL OR NEW.command_id !~ '^[A-Za-z0-9_-]{12,80}$' OR
    NEW.issuer_proof_id IS NULL OR btrim(NEW.issuer_proof_id)='' OR
    NEW.recipient_proof_id IS NULL OR btrim(NEW.recipient_proof_id)='' OR
    NEW.issuer_proof_id=NEW.recipient_proof_id OR NEW.issuer_id=NEW.recipient_id OR
    NEW.challenge_sha256 IS NULL OR NEW.challenge_sha256 !~ '^[0-9a-f]{64}$' OR
    NEW.issuer_assertion_sha256 IS NULL OR NEW.issuer_assertion_sha256 !~ '^[0-9a-f]{64}$' OR
    NEW.recipient_assertion_sha256 IS NULL OR NEW.recipient_assertion_sha256 !~ '^[0-9a-f]{64}$' THEN
  RAISE EXCEPTION 'ERR_F314_VERIFIED_WITNESSES_REQUIRED';
 END IF;
 IF NEW.issued_qty <= 0 THEN RAISE EXCEPTION 'ERR_F314_INVALID_QTY'; END IF;
 IF NEW.issued_at < clock_timestamp() - interval '5 minutes' OR
    NEW.issued_at > clock_timestamp() + interval '60 seconds' THEN
  RAISE EXCEPTION 'ERR_F314_ISSUANCE_TIME';
 END IF;
 SELECT s.remaining_qty,s.status,s.plant_id INTO current_balance,current_status,current_plant
 FROM public.store_trims_v13 s WHERE s.tenant_id=NEW.tenant_id AND s.id=NEW.trim_stock_id FOR UPDATE;
 IF NOT FOUND OR current_plant IS DISTINCT FROM NEW.plant_id THEN
  RAISE EXCEPTION 'ERR_F314_STOCK_PLANT_SCOPE';
 END IF;
 IF current_status <> 'AVAILABLE' THEN
  RAISE EXCEPTION 'ERR_F314_STOCK_NOT_AVAILABLE';
 END IF;
 IF current_balance < NEW.issued_qty THEN
  RAISE EXCEPTION 'ERR_F314_INSUFFICIENT_STOCK';
 END IF;
 -- A proposal or unrelated line must never consume authenticated factory stock.
 IF NOT EXISTS(SELECT 1 FROM public.dynamic_work_orders w
     WHERE w.tenant_id=NEW.tenant_id AND w.plant_id=NEW.plant_id AND
           w.id=NEW.order_id AND w.line_id=NEW.line_id AND
           w.status IN ('LOCKED','IN_PROGRESS')) THEN
  RAISE EXCEPTION 'ERR_F314_RELEASED_ORDER_LINE_REQUIRED';
 END IF;
 -- Unique proof IDs apply ACROSS issuer and recipient roles, even concurrently.
 INSERT INTO public.trim_issuance_proof_uses_v24(tenant_id,proof_id,issuance_id,witness_role)
 VALUES(NEW.tenant_id,NEW.issuer_proof_id,NEW.id,'ISSUER'),
       (NEW.tenant_id,NEW.recipient_proof_id,NEW.id,'RECIPIENT');
 UPDATE public.store_trims_v13 SET remaining_qty=remaining_qty-NEW.issued_qty,
 status=CASE WHEN remaining_qty-NEW.issued_qty=0 THEN 'DEPLETED' ELSE 'AVAILABLE' END
 WHERE tenant_id=NEW.tenant_id AND id=NEW.trim_stock_id;
 RETURN NULL;
END $_$;


--
-- Name: guard_wip_exception_close_v23(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_exception_close_v23() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE c wip_exception_cases_v23%ROWTYPE; s wip_gateway_scans_v20%ROWTYPE;
BEGIN
 SELECT * INTO c FROM wip_exception_cases_v23 WHERE tenant_id=NEW.tenant_id AND id=NEW.case_id;
 IF NOT FOUND OR c.plant_id<>NEW.plant_id OR c.part_id<>NEW.part_id OR c.station_id<>NEW.station_id THEN RAISE EXCEPTION 'ERR_F310_CASE_SCOPE'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||c.part_id::text));
 IF NEW.supervisor_subject=c.reporter_subject THEN RAISE EXCEPTION 'ERR_F310_INDEPENDENT_SUPERVISOR'; END IF;
 IF EXISTS (SELECT 1 FROM wip_exception_cases_v23 prev WHERE prev.tenant_id=NEW.tenant_id AND prev.reporter_proof_id=NEW.supervisor_proof_id)
 THEN RAISE EXCEPTION 'ERR_F310_CROSS_EVENT_PROOF_REPLAY'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=c.station_id AND w.worker_subject=NEW.supervisor_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F310_SUPERVISOR_STATION'; END IF;
 SELECT * INTO s FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.scan_id;
 IF NOT FOUND OR s.plant_id<>NEW.plant_id OR s.part_id<>c.part_id OR s.station_id<>c.station_id
  OR s.received_at<=c.opened_at OR s.observed_at<=c.opened_at OR s.received_at>clock_timestamp()
  OR s.received_at<clock_timestamp()-interval '15 minutes' OR s.observed_at<clock_timestamp()-interval '15 minutes'
 THEN RAISE EXCEPTION 'ERR_F310_NEW_SIGNED_SCAN_REQUIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 carrier
   ON carrier.tenant_id=a.tenant_id AND carrier.id=a.carrier_id
  WHERE a.tenant_id=NEW.tenant_id AND a.plant_id=NEW.plant_id AND a.target_type='CUT_PART'
   AND a.target_id=c.part_id AND a.carrier_id=s.carrier_id AND a.bound_event_id=s.binding_event_id
   AND carrier.enabled AND carrier.epc_hex=s.epc_hex)
 THEN RAISE EXCEPTION 'ERR_F310_CARRIER_NOT_CURRENT'; END IF;
 IF c.issue_type='LOST_CARRIER' AND NOT EXISTS (SELECT 1 FROM gen_carrier_active_v18 a
  WHERE a.tenant_id=NEW.tenant_id AND a.target_type='CUT_PART' AND a.target_id=c.part_id
   AND a.bound_at>c.opened_at AND a.bound_event_id=s.binding_event_id)
 THEN RAISE EXCEPTION 'ERR_F310_REPLACEMENT_BINDING_REQUIRED'; END IF;
 IF EXISTS(SELECT 1 FROM wip_movements_v20 m WHERE m.tenant_id=NEW.tenant_id AND m.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F310_RECOVERY_SCAN_ALREADY_CONSUMED'; END IF;
 NEW.closed_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_wip_exception_hold_v23(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_exception_hold_v23() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF EXISTS (SELECT 1 FROM wip_exception_cases_v23 c WHERE c.tenant_id=NEW.tenant_id
  AND c.plant_id=NEW.plant_id AND c.part_id=NEW.part_id
  AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
  WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F310_UNRESOLVED_EXCEPTION_WIP_HOLD'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_wip_exception_open_v23(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_exception_open_v23() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE g gen_garments_v18%ROWTYPE; original_garment uuid;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 SELECT g.* INTO g FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g
  ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_F310_PART_SCOPE'; END IF;
 -- Serialize with quality-release transitions; a late incident cannot place an
 -- already released garment under an inconsistent new open hold.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||g.id::text));
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id AND w.worker_subject=NEW.reporter_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F310_REPORTER_STATION'; END IF;
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.garment_id=g.id AND q.action='QUALITY_RELEASE') THEN RAISE EXCEPTION 'ERR_F310_ALREADY_RELEASED'; END IF;
 -- Replacement cut parts also participate in their original garment's QC.
 SELECT original.garment_id INTO original_garment FROM cut_recut_links_v19 link
  JOIN gen_cut_parts_v18 original ON original.tenant_id=link.tenant_id AND original.id=link.original_part_id
  WHERE link.tenant_id=NEW.tenant_id AND link.replacement_part_id=NEW.part_id;
 IF original_garment IS NOT NULL THEN
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
  IF EXISTS (SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
   AND q.garment_id=original_garment AND q.action='QUALITY_RELEASE')
  THEN RAISE EXCEPTION 'ERR_F310_RECUT_ORIGINAL_ALREADY_RELEASED'; END IF;
 END IF;
 IF EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.supervisor_proof_id=NEW.reporter_proof_id)
 THEN RAISE EXCEPTION 'ERR_F310_CROSS_EVENT_PROOF_REPLAY'; END IF;
 IF EXISTS(SELECT 1 FROM wip_exception_cases_v23 c WHERE c.tenant_id=NEW.tenant_id
  AND c.part_id=NEW.part_id AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
  WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id)) THEN RAISE EXCEPTION 'ERR_F310_ALREADY_OPEN'; END IF;
 NEW.opened_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_wip_move_v20(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_move_v20() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE route wip_transfer_routes_v20%ROWTYPE; scan wip_gateway_scans_v20%ROWTYPE;
 prior wip_movements_v20%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 -- Sequence and receipt time are allocated only AFTER the per-part lock.
 -- Global identity sequences assigned before lock would reorder concurrent moves.
 NEW.event_no := (SELECT COALESCE(max(event_no),0)+1 FROM wip_movements_v20
  WHERE tenant_id=NEW.tenant_id AND part_id=NEW.part_id);
 NEW.recorded_at := clock_timestamp();
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT * INTO route FROM wip_transfer_routes_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.route_id;
 IF NOT FOUND OR NOT route.enabled OR route.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_WIP_ROUTE'; END IF;
 SELECT * INTO scan FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.scan_id;
 IF NOT FOUND OR scan.part_id<>NEW.part_id OR scan.plant_id<>NEW.plant_id OR scan.station_id<>NEW.station_id
    OR scan.received_at>NEW.recorded_at OR scan.observed_at>NEW.recorded_at+interval '2 minutes' THEN
   RAISE EXCEPTION 'ERR_WIP_SCAN_MISMATCH'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=NEW.tenant_id AND o.part_id=NEW.part_id AND o.disposition='ACCEPTED') THEN RAISE EXCEPTION 'ERR_WIP_PART_NOT_ACCEPTED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a WHERE a.tenant_id=NEW.tenant_id
   AND a.carrier_id=scan.carrier_id AND a.bound_event_id=scan.binding_event_id
   AND a.target_type='CUT_PART' AND a.target_id=NEW.part_id
   AND a.plant_id=NEW.plant_id) THEN RAISE EXCEPTION 'ERR_WIP_CARRIER_RETIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
   AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
   AND w.worker_subject=NEW.worker_subject AND w.enabled) THEN RAISE EXCEPTION 'ERR_WIP_WORKER_STATION'; END IF;
 SELECT * INTO prior FROM wip_movements_v20 WHERE tenant_id=NEW.tenant_id AND part_id=NEW.part_id
  ORDER BY event_no DESC LIMIT 1;
 IF NEW.action='DEPART' THEN
  IF NEW.station_id<>route.from_station OR (prior.id IS NOT NULL AND
      (prior.action<>'ARRIVE' OR prior.station_id<>route.from_station)) THEN
      RAISE EXCEPTION 'ERR_WIP_DEPART_STATE'; END IF;
 ELSE
  IF NEW.station_id<>route.to_station OR prior.id IS NULL OR prior.action<>'DEPART' OR
     prior.route_id<>NEW.route_id OR prior.station_id<>route.from_station OR
     prior.worker_subject=NEW.worker_subject OR NEW.recorded_at<prior.recorded_at THEN
     RAISE EXCEPTION 'ERR_WIP_ARRIVE_STATE'; END IF;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_wip_piece_hold_v26(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_piece_hold_v26() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE g gen_garments_v18%ROWTYPE; pid uuid; ply_group_no integer;
BEGIN
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=NEW.garment_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_F316_GARMENT_SCOPE'; END IF;
 -- Use exactly the WIP part locks used by F3.3/F3.10, including replacement parts.
 FOR pid IN SELECT part_id FROM (
  SELECT p.id part_id FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
  UNION SELECT link.replacement_part_id FROM cut_recut_links_v19 link
   JOIN gen_cut_parts_v18 p ON p.tenant_id=link.tenant_id AND p.id=link.original_part_id
   WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=NEW.garment_id
 ) parts ORDER BY part_id LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||pid::text));
 END LOOP;
 FOR ply_group_no IN SELECT DISTINCT b.ply_group FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=g.lay_draft_id
  AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
  ORDER BY b.ply_group LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||g.lay_draft_id::text||':'||g.marker_position||':'||ply_group_no));
 END LOOP;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.reporter_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F316_REPORTER_STATION'; END IF;
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
  AND q.garment_id=NEW.garment_id AND q.action='QUALITY_RELEASE')
 THEN RAISE EXCEPTION 'ERR_F316_ALREADY_QC_RELEASED'; END IF;
 IF EXISTS(SELECT 1 FROM wip_piece_reviews_v26 x WHERE x.tenant_id=NEW.tenant_id AND x.command_id=NEW.command_id)
 THEN RAISE EXCEPTION 'ERR_F316_COMMAND_REUSE'; END IF;
 IF wip_piece_active_hold_v26(NEW.tenant_id,NEW.garment_id)
 THEN RAISE EXCEPTION 'ERR_F316_ALREADY_HELD'; END IF;
 IF EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r WHERE r.tenant_id=NEW.tenant_id
   AND r.supervisor_proof_id=NEW.reporter_proof_id)
 THEN RAISE EXCEPTION 'ERR_F316_CROSS_PROOF_REPLAY'; END IF;
 NEW.opened_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_wip_piece_move_hold_v26(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_piece_move_hold_v26() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||NEW.part_id::text));
 IF EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id
  AND p.id=NEW.part_id AND wip_piece_active_hold_v26(NEW.tenant_id,p.garment_id))
 OR EXISTS(SELECT 1 FROM cut_recut_links_v19 l JOIN gen_cut_parts_v18 orig
  ON orig.tenant_id=l.tenant_id AND orig.id=l.original_part_id
  WHERE l.tenant_id=NEW.tenant_id AND l.replacement_part_id=NEW.part_id
  AND wip_piece_active_hold_v26(NEW.tenant_id,orig.garment_id))
 THEN RAISE EXCEPTION 'ERR_F316_PIECE_HOLD_MOVEMENT'; END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_wip_piece_review_v26(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_piece_review_v26() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE h wip_piece_holds_v26%ROWTYPE; s wip_gateway_scans_v20%ROWTYPE; pid uuid; g gen_garments_v18%ROWTYPE; ply_group_no integer;
BEGIN
 SELECT * INTO h FROM wip_piece_holds_v26 WHERE tenant_id=NEW.tenant_id AND id=NEW.hold_id;
 IF NOT FOUND OR h.plant_id<>NEW.plant_id OR h.garment_id<>NEW.garment_id
  OR h.station_id<>NEW.station_id THEN RAISE EXCEPTION 'ERR_F316_HOLD_SCOPE'; END IF;
 FOR pid IN SELECT part_id FROM (
  SELECT p.id part_id FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=h.garment_id
  UNION SELECT link.replacement_part_id FROM cut_recut_links_v19 link
   JOIN gen_cut_parts_v18 p ON p.tenant_id=link.tenant_id AND p.id=link.original_part_id
   WHERE p.tenant_id=NEW.tenant_id AND p.garment_id=h.garment_id
 ) parts ORDER BY part_id LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:PART:'||pid::text));
 END LOOP;
 SELECT * INTO g FROM gen_garments_v18 WHERE tenant_id=NEW.tenant_id AND id=h.garment_id;
 FOR ply_group_no IN SELECT DISTINCT b.ply_group FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=g.lay_draft_id
   AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
  ORDER BY b.ply_group LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F316:GROUP:'||g.lay_draft_id::text||':'||g.marker_position||':'||ply_group_no));
 END LOOP;
 IF EXISTS(SELECT 1 FROM wip_piece_holds_v26 x WHERE x.tenant_id=NEW.tenant_id AND x.command_id=NEW.command_id)
 THEN RAISE EXCEPTION 'ERR_F316_COMMAND_REUSE'; END IF;
 IF NEW.supervisor_subject=h.reporter_subject THEN RAISE EXCEPTION 'ERR_F316_INDEPENDENT_REVIEW'; END IF;
 IF EXISTS(SELECT 1 FROM wip_piece_holds_v26 o WHERE o.tenant_id=NEW.tenant_id
   AND o.reporter_proof_id=NEW.supervisor_proof_id)
 THEN RAISE EXCEPTION 'ERR_F316_CROSS_PROOF_REPLAY'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE w.tenant_id=NEW.tenant_id
  AND w.plant_id=NEW.plant_id AND w.station_id=NEW.station_id
  AND w.worker_subject=NEW.supervisor_subject AND w.enabled)
 THEN RAISE EXCEPTION 'ERR_F316_SUPERVISOR_STATION'; END IF;
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||NEW.scan_id::text));
 SELECT * INTO s FROM wip_gateway_scans_v20 WHERE tenant_id=NEW.tenant_id AND id=NEW.scan_id;
 IF NOT FOUND OR s.plant_id<>NEW.plant_id OR s.station_id<>NEW.station_id
  OR s.received_at<=h.opened_at OR s.observed_at<=h.opened_at
  OR s.received_at>clock_timestamp() OR s.observed_at<clock_timestamp()-interval '15 minutes'
  OR s.received_at<clock_timestamp()-interval '15 minutes'
 THEN RAISE EXCEPTION 'ERR_F316_FRESH_SIGNED_SCAN_REQUIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_cut_parts_v18 p WHERE p.tenant_id=NEW.tenant_id
  AND p.garment_id=h.garment_id AND p.id=s.part_id)
 AND NOT EXISTS(SELECT 1 FROM cut_recut_links_v19 link JOIN gen_cut_parts_v18 orig
  ON orig.tenant_id=link.tenant_id AND orig.id=link.original_part_id
  WHERE orig.tenant_id=NEW.tenant_id AND orig.garment_id=h.garment_id
  AND link.replacement_part_id=s.part_id)
 THEN RAISE EXCEPTION 'ERR_F316_SCAN_NOT_IN_GARMENT'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 c
  ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
  WHERE a.tenant_id=NEW.tenant_id AND a.plant_id=NEW.plant_id AND a.target_type='CUT_PART'
  AND a.target_id=s.part_id AND a.carrier_id=s.carrier_id
  AND a.bound_event_id=s.binding_event_id AND c.enabled AND c.epc_hex=s.epc_hex)
 THEN RAISE EXCEPTION 'ERR_F316_CARRIER_NOT_CURRENT'; END IF;
 IF EXISTS(SELECT 1 FROM wip_movements_v20 m WHERE m.tenant_id=NEW.tenant_id AND m.scan_id=NEW.scan_id)
 OR EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=NEW.tenant_id AND x.scan_id=NEW.scan_id)
 OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 old WHERE old.tenant_id=NEW.tenant_id AND old.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F316_SCAN_ALREADY_USED'; END IF;
 IF EXISTS(SELECT 1 FROM wip_exception_cases_v23 c WHERE c.tenant_id=NEW.tenant_id
  AND c.part_id=s.part_id AND NOT EXISTS(SELECT 1 FROM wip_exception_closures_v23 x
  WHERE x.tenant_id=c.tenant_id AND x.case_id=c.id))
 THEN RAISE EXCEPTION 'ERR_F316_UNRESOLVED_WIP_EXCEPTION'; END IF;
 NEW.reviewed_at:=clock_timestamp();
 RETURN NEW;
END $$;


--
-- Name: guard_wip_quality_release_v21(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_quality_release_v21() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE garment uuid; original_garment uuid;
BEGIN
 SELECT p.garment_id INTO garment FROM gen_cut_parts_v18 p
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF garment IS NULL THEN RAISE EXCEPTION 'ERR_F34_WIP_GARMENT_MISSING'; END IF;
 -- A recut part may belong to another reserved garment identity; also check
 -- the original garment whose missing part it replaces.
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||garment::text));
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
   AND q.garment_id=garment AND q.action='QUALITY_RELEASE') THEN
  RAISE EXCEPTION 'ERR_F34_RELEASED_GARMENT_WIP_LOCKED'; END IF;
 SELECT original.garment_id INTO original_garment FROM cut_recut_links_v19 l
    JOIN gen_cut_parts_v18 original ON original.tenant_id=l.tenant_id AND original.id=l.original_part_id
    WHERE l.tenant_id=NEW.tenant_id AND l.replacement_part_id=NEW.part_id;
 IF original_garment IS NOT NULL THEN
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
  IF EXISTS (SELECT 1 FROM qc_garment_events_v21 q WHERE q.tenant_id=NEW.tenant_id
    AND q.garment_id=original_garment AND q.action='QUALITY_RELEASE') THEN
   RAISE EXCEPTION 'ERR_F34_RELEASED_RECUT_WIP_LOCKED'; END IF;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: guard_wip_scan_v20(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.guard_wip_scan_v20() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE g gen_garments_v18%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 SELECT garment.* INTO g FROM gen_cut_parts_v18 p JOIN gen_garments_v18 garment
  ON garment.tenant_id=p.tenant_id AND garment.id=p.garment_id
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.part_id;
 IF NOT FOUND OR g.plant_id<>NEW.plant_id THEN RAISE EXCEPTION 'ERR_WIP_PART_PLANT'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=NEW.tenant_id
    AND o.part_id=NEW.part_id AND o.disposition='ACCEPTED')
 OR EXISTS(SELECT 1 FROM cut_recut_links_v19 l WHERE l.tenant_id=NEW.tenant_id AND l.original_part_id=NEW.part_id)
 THEN RAISE EXCEPTION 'ERR_WIP_PART_NOT_ACCEPTED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM sew_prod_gateways_v10 gw WHERE gw.tenant_id=NEW.tenant_id
  AND gw.gateway_id=NEW.gateway_id AND gw.plant_id=NEW.plant_id
  AND gw.station_id=NEW.station_id AND gw.enabled) THEN
  RAISE EXCEPTION 'ERR_WIP_GATEWAY_NOT_ENROLLED'; END IF;
 SELECT a.bound_event_id INTO NEW.binding_event_id FROM gen_carriers_v18 c JOIN gen_carrier_active_v18 a
   ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
   WHERE c.tenant_id=NEW.tenant_id AND c.id=NEW.carrier_id
   AND c.enabled AND c.epc_hex=NEW.epc_hex AND a.plant_id=NEW.plant_id
   AND a.target_type='CUT_PART' AND a.target_id=NEW.part_id;
 IF NEW.binding_event_id IS NULL THEN RAISE EXCEPTION 'ERR_WIP_CARRIER_NOT_BOUND'; END IF;
 IF NEW.observed_at>clock_timestamp()+interval '2 minutes' THEN
   RAISE EXCEPTION 'ERR_WIP_SCAN_FUTURE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: intel_sim_snapshot_no_edit_v12(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.intel_sim_snapshot_no_edit_v12() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'ERR_INTEL_SIM_SNAPSHOT_IMMUTABLE'; END $$;


--
-- Name: jobwork_part_disputed_v29(text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.jobwork_part_disputed_v29(t text, p uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT EXISTS(SELECT 1 FROM jobwork_discrepancies_v29 d
  WHERE d.tenant_id=t AND d.part_id=p AND NOT EXISTS(
    SELECT 1 FROM jobwork_discrepancy_resolutions_v31 r
    WHERE r.tenant_id=d.tenant_id AND r.discrepancy_id=d.id
      AND r.resolution_kind='RETURNED_TO_PIECE_HOLD'))
$$;


--
-- Name: jobwork_part_offsite_v28(text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.jobwork_part_offsite_v28(t text, p uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT EXISTS(SELECT 1 FROM jobwork_outwards_v28 o WHERE o.tenant_id=t AND o.part_id=p
  AND NOT EXISTS(SELECT 1 FROM jobwork_inwards_v28 i WHERE i.tenant_id=o.tenant_id AND i.outward_id=o.id))
$$;


--
-- Name: jobwork_require_scan_v28(text, text, text, uuid, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.jobwork_require_scan_v28(t text, pl text, st text, part uuid, scanid uuid) RETURNS void
    LANGUAGE plpgsql
    AS $$
DECLARE s wip_gateway_scans_v20%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(t),hashtext('WIP:SCAN:'||scanid::text));
 SELECT * INTO s FROM wip_gateway_scans_v20 WHERE tenant_id=t AND id=scanid;
 IF NOT FOUND OR s.part_id<>part OR s.plant_id<>pl OR s.station_id<>st
    OR s.received_at<clock_timestamp()-interval '15 minutes'
    OR s.observed_at<clock_timestamp()-interval '15 minutes'
    OR s.observed_at>clock_timestamp()+interval '2 minutes'
    OR s.received_at>clock_timestamp() THEN RAISE EXCEPTION 'ERR_F317_FRESH_SCAN_REQUIRED'; END IF;
 IF NOT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a JOIN gen_carriers_v18 c
    ON c.tenant_id=a.tenant_id AND c.id=a.carrier_id
    WHERE a.tenant_id=t AND a.carrier_id=s.carrier_id AND a.bound_event_id=s.binding_event_id
      AND a.target_type='CUT_PART' AND a.target_id=part AND a.plant_id=pl
      AND c.enabled AND c.epc_hex=s.epc_hex)
 OR NOT EXISTS(SELECT 1 FROM sew_prod_gateways_v10 gw WHERE gw.tenant_id=t
   AND gw.gateway_id=s.gateway_id AND gw.plant_id=pl AND gw.station_id=st AND gw.enabled)
 OR jobwork_scan_spent_v28(t,scanid) THEN RAISE EXCEPTION 'ERR_F317_SCAN_UNTRUSTED_OR_SPENT'; END IF;
END $$;


--
-- Name: jobwork_reserve_evidence_v28(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.jobwork_reserve_evidence_v28() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
DECLARE kind text; proof text;
BEGIN
 IF TG_TABLE_NAME='jobwork_outwards_v28' THEN kind:='OUTWARD';proof:=NEW.sender_proof_id;
 ELSE kind:='INWARD';proof:=NEW.receiver_proof_id; END IF;
 INSERT INTO public.jobwork_used_scans_v28(tenant_id,scan_id,kind,entry_id) VALUES(NEW.tenant_id,NEW.scan_id,kind,NEW.id);
 INSERT INTO public.jobwork_used_proofs_v28(tenant_id,proof_id,kind,entry_id) VALUES(NEW.tenant_id,proof,kind,NEW.id);
 RETURN NEW;
END $$;


--
-- Name: jobwork_scan_spent_v28(text, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.jobwork_scan_spent_v28(t text, s uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT EXISTS(SELECT 1 FROM wip_movements_v20 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM wip_exception_closures_v23 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM wip_piece_reviews_v26 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM sew_piece_used_scans_v27 x WHERE x.tenant_id=t AND x.scan_id=s)
 OR EXISTS(SELECT 1 FROM jobwork_used_scans_v28 x WHERE x.tenant_id=t AND x.scan_id=s)
$$;


--
-- Name: material_scope_read_locked_v5(character varying, character varying, character varying, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.material_scope_read_locked_v5(p_tenant character varying, p_order character varying, p_plant character varying, p_holder text) RETURNS TABLE(mode text, legacy_writer_fenced boolean)
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
    IF p_tenant IS NULL OR p_tenant = ''
       OR p_tenant IS DISTINCT FROM
          NULLIF(current_setting('app.current_tenant_id', true), '')
       OR p_order IS NULL OR p_order = ''
       OR p_plant IS NULL OR p_plant = ''
       OR p_holder IS NULL OR p_holder = '' THEN
        RETURN;
    END IF;

    RETURN QUERY
    SELECT s.mode, s.legacy_writer_fenced
    FROM public.material_write_scopes_v5 AS s
    WHERE s.tenant_id = p_tenant
      AND s.order_id = p_order
      AND s.plant_id = p_plant
      AND s.lease_holder = p_holder
      AND s.lease_expires_at > clock_timestamp()
    FOR SHARE;
END;
$$;


--
-- Name: pack_sim_carton_state_v11(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_sim_carton_state_v11() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.carton_code<>OLD.carton_code OR NEW.lay_draft_id<>OLD.lay_draft_id OR NEW.plant_id<>OLD.plant_id OR
  OLD.state<>'OPEN' OR NEW.state<>'SEALED' OR NEW.sealed_at IS NULL THEN RAISE EXCEPTION 'ERR_PACK_SIM_CARTON_TRANSITION'; END IF;
 RETURN NEW;
END $$;


--
-- Name: pack_sim_immutable_v11(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_sim_immutable_v11() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN RAISE EXCEPTION 'ERR_PACK_SIM_IMMUTABLE'; END $$;


--
-- Name: pack_sim_item_delete_guard_v11(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_sim_item_delete_guard_v11() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE state_now varchar(16);
BEGIN
 SELECT state INTO state_now FROM pack_sim_cartons_v11 WHERE tenant_id=OLD.tenant_id AND carton_code=OLD.carton_code FOR UPDATE;
 IF state_now<>'OPEN' THEN RAISE EXCEPTION 'ERR_PACK_SIM_SEALED_CARTON_IMMUTABLE'; END IF;
 RETURN OLD;
END $$;


--
-- Name: pack_sim_item_guard_v11(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_sim_item_guard_v11() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE u qc_sim_units_v10%ROWTYPE; b qc_sim_buyer_bindings_v10%ROWTYPE; c pack_sim_cartons_v11%ROWTYPE; steps text[];
BEGIN
 SELECT * INTO c FROM pack_sim_cartons_v11 WHERE tenant_id=NEW.tenant_id AND carton_code=NEW.carton_code FOR UPDATE;
 SELECT * INTO u FROM qc_sim_units_v10 WHERE tenant_id=NEW.tenant_id AND serial=NEW.serial FOR SHARE;
 SELECT * INTO b FROM qc_sim_buyer_bindings_v10 WHERE tenant_id=NEW.tenant_id AND serial=NEW.serial;
 SELECT r.steps INTO steps FROM qc_sim_routes_v10 r WHERE r.tenant_id=u.tenant_id AND r.lay_draft_id=u.lay_draft_id AND r.plant_id=u.plant_id;
 IF c.state<>'OPEN' OR u.qc_state<>'ACCEPTED' OR steps IS NULL OR u.finishing_count<>cardinality(steps) OR
    b.buyer_code IS NULL OR b.buyer_code<>NEW.buyer_code OR c.lay_draft_id<>u.lay_draft_id OR c.plant_id<>u.plant_id THEN
    RAISE EXCEPTION 'ERR_PACK_SIM_GARMENT_NOT_ELIGIBLE';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: pack_sim_manifest_guard_v11(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_sim_manifest_guard_v11() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE c pack_sim_cartons_v11%ROWTYPE; m pack_sim_manifests_v11%ROWTYPE; actual integer;
BEGIN
 SELECT * INTO c FROM pack_sim_cartons_v11 WHERE tenant_id=NEW.tenant_id AND carton_code=NEW.carton_code FOR UPDATE;
 SELECT * INTO m FROM pack_sim_manifests_v11 WHERE tenant_id=NEW.tenant_id AND manifest_code=NEW.manifest_code FOR UPDATE;
 SELECT count(*)::integer INTO actual FROM pack_sim_items_v11 WHERE tenant_id=NEW.tenant_id AND carton_code=NEW.carton_code;
 IF c.state<>'SEALED' OR m.state<>'DRAFT' OR c.plant_id<>m.plant_id OR actual<>NEW.item_count OR actual=0 THEN
   RAISE EXCEPTION 'ERR_PACK_SIM_MANIFEST_CONSERVATION';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: pack_sim_manifest_state_v11(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_sim_manifest_state_v11() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.manifest_code<>OLD.manifest_code OR NEW.plant_id<>OLD.plant_id OR
  OLD.state<>'DRAFT' OR NEW.state<>'DISPATCH_REHEARSAL' OR NEW.dispatched_at IS NULL THEN RAISE EXCEPTION 'ERR_PACK_SIM_MANIFEST_TRANSITION'; END IF;
 RETURN NEW;
END $$;


--
-- Name: pack_size_rule_guard_v22(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_size_rule_guard_v22() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE spec pack_carton_specs_v22%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F35:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM pack_carton_specs_v22 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF spec.id IS NULL OR spec.enabled OR NOT EXISTS(SELECT 1 FROM cut_lay_positions_v8 p
   WHERE p.tenant_id=spec.tenant_id AND p.lay_draft_id=spec.lay_draft_id AND p.size_code=NEW.size_code) THEN
  RAISE EXCEPTION 'ERR_F35_RULE_REQUIRES_DRAFT_AND_VALID_SIZE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: pack_spec_insert_guard_v22(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_spec_insert_guard_v22() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE digest char(64);
BEGIN
 IF NEW.enabled THEN RAISE EXCEPTION 'ERR_F35_SPEC_ACTIVATE_AFTER_WEIGHTS'; END IF;
 SELECT sha256 INTO digest FROM document_sources_v65 WHERE tenant_id=NEW.tenant_id AND id=NEW.source_document_id;
 IF digest IS NULL OR digest<>NEW.source_sha256 THEN RAISE EXCEPTION 'ERR_F35_SOURCE_MISMATCH'; END IF;
 IF NOT EXISTS(SELECT 1 FROM cut_sewing_releases_v8 r WHERE r.tenant_id=NEW.tenant_id
   AND r.plant_id=NEW.plant_id AND r.lay_draft_id=NEW.lay_draft_id) THEN
  RAISE EXCEPTION 'ERR_F35_SIGNED_CUT_LAY_REQUIRED'; END IF;
 RETURN NEW;
END $$;


--
-- Name: pack_spec_update_guard_v22(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.pack_spec_update_guard_v22() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE missing_sizes bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(OLD.tenant_id),hashtext('F35:SPEC:'||OLD.id::text));
 IF TG_OP='DELETE' OR (to_jsonb(OLD)-'enabled')<>(to_jsonb(NEW)-'enabled') THEN
  RAISE EXCEPTION 'ERR_F35_SPEC_IMMUTABLE'; END IF;
 IF OLD.enabled AND NOT NEW.enabled THEN RETURN NEW; END IF;
 IF NOT OLD.enabled AND NEW.enabled THEN
  SELECT count(*) INTO missing_sizes FROM (
   SELECT DISTINCT size_code FROM cut_lay_positions_v8 WHERE tenant_id=OLD.tenant_id AND lay_draft_id=OLD.lay_draft_id
  ) sizes WHERE NOT EXISTS(SELECT 1 FROM pack_size_weights_v22 w WHERE w.tenant_id=OLD.tenant_id
   AND w.spec_id=OLD.id AND w.size_code=sizes.size_code);
  IF missing_sizes<>0 OR NOT EXISTS(SELECT 1 FROM pack_size_weights_v22 w WHERE w.tenant_id=OLD.tenant_id AND w.spec_id=OLD.id)
    THEN RAISE EXCEPTION 'ERR_F35_SIZE_WEIGHT_RULES_INCOMPLETE'; END IF;
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'ERR_F35_SPEC_IMMUTABLE';
END $$;


--
-- Name: prevent_cross_type_epc_v8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.prevent_cross_type_epc_v8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('RFID_EPC_GLOBAL_V8'));
 IF TG_TABLE_NAME='store_roll_rfid_bindings_v7' THEN
   IF EXISTS(SELECT 1 FROM cut_bundle_scan_witnesses_v8 WHERE tenant_id=NEW.tenant_id AND epc_hex=NEW.epc_hex) THEN
     RAISE EXCEPTION 'ERR_EPC_ALREADY_BOUND_TO_CUT_BUNDLE';
   END IF;
 ELSE
   IF EXISTS(SELECT 1 FROM store_roll_rfid_bindings_v7 WHERE tenant_id=NEW.tenant_id AND epc_hex=NEW.epc_hex) THEN
     RAISE EXCEPTION 'ERR_EPC_ALREADY_BOUND_TO_FABRIC_ROLL';
   END IF;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: protect_cut_draft_v8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.protect_cut_draft_v8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'ERR_CUT_DRAFT_IMMUTABLE'; END $$;


--
-- Name: protect_material_variance_v5(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.protect_material_variance_v5() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF OLD.status <> 'PENDING_CEO' THEN
   RAISE EXCEPTION 'ERR_VARIANCE_IMMUTABLE';
 END IF;
 IF ROW(NEW.tenant_id,NEW.plant_id,NEW.reservation_id,NEW.roll_id,NEW.order_id,NEW.cut_no,
         NEW.requested_additional_mm,NEW.actual_story,NEW.reason_code,NEW.requested_by,NEW.request_key,NEW.created_at)
    IS DISTINCT FROM
    ROW(OLD.tenant_id,OLD.plant_id,OLD.reservation_id,OLD.roll_id,OLD.order_id,OLD.cut_no,
         OLD.requested_additional_mm,OLD.actual_story,OLD.reason_code,OLD.requested_by,OLD.request_key,OLD.created_at) THEN
   RAISE EXCEPTION 'ERR_VARIANCE_SOURCE_IMMUTABLE';
 END IF;
 IF NEW.challenge_hash <> OLD.challenge_hash AND OLD.challenge_hash <> repeat('0',64) THEN
   RAISE EXCEPTION 'ERR_VARIANCE_CHALLENGE_IMMUTABLE';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: protect_stores_admission_v7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.protect_stores_admission_v7() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'ERR_STORES_ADMISSION_IMMUTABLE'; END $$;


--
-- Name: protect_stores_pick_proposal_v7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.protect_stores_pick_proposal_v7() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'ERR_PICK_PROPOSAL_IMMUTABLE'; END $$;


--
-- Name: qc_freeze_garment_parts_v21(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_freeze_garment_parts_v21() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||NEW.garment_id::text));
 IF EXISTS(SELECT 1 FROM qc_garment_events_v21 e WHERE e.tenant_id=NEW.tenant_id
   AND e.garment_id=NEW.garment_id) THEN
  RAISE EXCEPTION 'ERR_F34_GENEALOGY_FROZEN_AFTER_QC'; END IF;
 RETURN NEW;
END $$;


--
-- Name: qc_freeze_recut_v21(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_freeze_recut_v21() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE original_garment uuid; replacement_garment uuid;
BEGIN
 SELECT p.garment_id INTO original_garment FROM gen_cut_parts_v18 p
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.original_part_id;
 SELECT p.garment_id INTO replacement_garment FROM gen_cut_parts_v18 p
  WHERE p.tenant_id=NEW.tenant_id AND p.id=NEW.replacement_part_id;
 IF original_garment IS NULL OR replacement_garment IS NULL THEN
  RAISE EXCEPTION 'ERR_F34_RECUT_PART_UNKNOWN'; END IF;
 -- Locks are acquired in stable lexical order to avoid cross-garment cycles.
 IF original_garment::text < replacement_garment::text THEN
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||replacement_garment::text));
 ELSE
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||replacement_garment::text));
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:GARMENT:'||original_garment::text));
 END IF;
 IF EXISTS (SELECT 1 FROM qc_garment_events_v21 e WHERE e.tenant_id=NEW.tenant_id
   AND e.garment_id IN(original_garment,replacement_garment)) THEN
  RAISE EXCEPTION 'ERR_F34_RECUT_AFTER_INSPECTION'; END IF;
 RETURN NEW;
END $$;


--
-- Name: qc_garment_parts_ready_v21(character varying, uuid, character varying); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_garment_parts_ready_v21(t character varying, gid uuid, finish character varying) RETURNS boolean
    LANGUAGE sql STABLE
    AS $$
 WITH effective_parts AS (
  SELECT p.id original_id,
   CASE WHEN o.disposition='ACCEPTED' AND l.id IS NULL AND NOT EXISTS (
          SELECT 1 FROM cut_recut_links_v19 donor WHERE donor.tenant_id=p.tenant_id
            AND donor.replacement_part_id=p.id) THEN p.id
        WHEN o.disposition='REJECTED' THEN l.replacement_part_id END AS effective_id
  FROM gen_cut_parts_v18 p
  LEFT JOIN cut_part_outcomes_v19 o ON o.tenant_id=p.tenant_id AND o.part_id=p.id
  LEFT JOIN cut_recut_links_v19 l ON l.tenant_id=p.tenant_id AND l.original_part_id=p.id
  WHERE p.tenant_id=t AND p.garment_id=gid
 ), checked_parts AS (
  SELECT ep.original_id, ep.effective_id,
   (SELECT m.action='ARRIVE' AND m.station_id=finish
    FROM wip_movements_v20 m WHERE m.tenant_id=t AND m.part_id=ep.effective_id
    ORDER BY m.event_no DESC LIMIT 1) AS at_finish,
   EXISTS (SELECT 1 FROM cut_part_outcomes_v19 o WHERE o.tenant_id=t
      AND o.part_id=ep.effective_id AND o.disposition='ACCEPTED') AS accepted
  FROM effective_parts ep
 )
 SELECT count(*)>0 AND count(*)=count(effective_id)
  AND bool_and(COALESCE(at_finish,false) AND accepted)
 FROM checked_parts
$$;


--
-- Name: qc_rule_insert_guard_v21(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_rule_insert_guard_v21() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE spec qc_garment_specs_v21%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('F34:SPEC:'||NEW.spec_id::text));
 SELECT * INTO spec FROM qc_garment_specs_v21 WHERE tenant_id=NEW.tenant_id AND id=NEW.spec_id;
 IF spec.id IS NULL OR spec.enabled OR NOT EXISTS (
  SELECT 1 FROM cut_lay_positions_v8 p WHERE p.tenant_id=spec.tenant_id
    AND p.lay_draft_id=spec.lay_draft_id AND p.size_code=NEW.size_code) THEN
   RAISE EXCEPTION 'ERR_F34_RULE_REQUIRES_DRAFT_SPEC_AND_VALID_SIZE'; END IF;
 RETURN NEW;
END $$;


--
-- Name: qc_sim_buyer_gate_v10(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_sim_buyer_gate_v10() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE u qc_sim_units_v10%ROWTYPE; route_steps text[]; original_style varchar(64); valid_sku boolean;
BEGIN
 SELECT * INTO u FROM qc_sim_units_v10 WHERE tenant_id=NEW.tenant_id AND serial=NEW.serial FOR SHARE;
 IF NOT FOUND OR u.qc_state<>'ACCEPTED' THEN RAISE EXCEPTION 'ERR_QC_SIM_NOT_ACCEPTED'; END IF;
 SELECT steps INTO route_steps FROM qc_sim_routes_v10 WHERE tenant_id=NEW.tenant_id AND lay_draft_id=u.lay_draft_id AND plant_id=u.plant_id;
 IF NOT FOUND OR u.finishing_count<>cardinality(route_steps) THEN RAISE EXCEPTION 'ERR_QC_SIM_NOT_FINISHED'; END IF;
 SELECT o.style_code,EXISTS(SELECT 1 FROM order_intake_skus_v5 sk WHERE sk.tenant_id=o.tenant_id AND sk.intake_id=o.id AND sk.color_code=u.color_code AND sk.size_code=u.size_code)
   INTO original_style,valid_sku FROM cut_lay_drafts_v8 d JOIN order_intakes_v5 o ON o.tenant_id=d.tenant_id AND o.id=d.intake_id
   WHERE d.tenant_id=NEW.tenant_id AND d.id=u.lay_draft_id;
 IF NOT valid_sku OR NEW.style_code<>original_style OR NEW.size_code<>u.size_code THEN RAISE EXCEPTION 'ERR_QC_SIM_BUYER_SKU_MISMATCH'; END IF;
 RETURN NEW;
END $$;


--
-- Name: qc_sim_no_edit_v10(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_sim_no_edit_v10() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'ERR_QC_SIM_IMMUTABLE_RECORD'; END $$;


--
-- Name: qc_sim_source_guard_v10(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_sim_source_guard_v10() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE matched boolean;
BEGIN
 SELECT EXISTS(
   SELECT 1 FROM sew_sim_credits_v9 cr
   JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=cr.tenant_id AND cy.id=cr.cycle_id AND cy.state='COMPLETED'
   JOIN cut_lay_positions_v8 lp ON lp.tenant_id=cr.tenant_id AND lp.lay_draft_id=cr.lay_draft_id AND lp.marker_position=cr.marker_position
   WHERE cr.tenant_id=NEW.tenant_id AND NEW.serial=ANY(cr.serials)
     AND cr.lay_draft_id=NEW.lay_draft_id AND cr.marker_position=NEW.marker_position
     AND cy.plant_id=NEW.plant_id AND lp.color_code=NEW.color_code AND lp.size_code=NEW.size_code
 ) INTO matched;
 IF NOT matched OR NEW.qc_state<>'PENDING' OR NEW.finishing_count<>0 OR NEW.revision<>0 THEN
  RAISE EXCEPTION 'ERR_QC_SIM_NO_SOURCE_GENEALOGY';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: qc_sim_unit_guard_v10(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_sim_unit_guard_v10() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'ERR_QC_SIM_UNIT_DELETE'; END IF;
 IF NEW.tenant_id<>OLD.tenant_id OR NEW.serial<>OLD.serial OR NEW.lay_draft_id<>OLD.lay_draft_id OR
    NEW.plant_id<>OLD.plant_id OR NEW.marker_position<>OLD.marker_position OR
    NEW.color_code<>OLD.color_code OR NEW.size_code<>OLD.size_code OR NEW.source_state<>OLD.source_state OR
    NEW.revision<>OLD.revision+1 OR NEW.finishing_count<OLD.finishing_count OR
    NEW.finishing_count>OLD.finishing_count+1 THEN
  RAISE EXCEPTION 'ERR_QC_SIM_GENEALOGY_IMMUTABLE';
 END IF;
 IF NOT ((OLD.qc_state='PENDING' AND NEW.qc_state IN ('ACCEPTED','HOLD')) OR
  (OLD.qc_state='HOLD' AND NEW.qc_state='REINSPECT') OR
  (OLD.qc_state='REINSPECT' AND NEW.qc_state IN ('ACCEPTED','HOLD')) OR
  (OLD.qc_state='ACCEPTED' AND NEW.qc_state='ACCEPTED' AND NEW.finishing_count=OLD.finishing_count+1)) THEN
  RAISE EXCEPTION 'ERR_QC_SIM_INVALID_TRANSITION';
 END IF;
 IF OLD.qc_state<>'ACCEPTED' AND NEW.finishing_count<>OLD.finishing_count THEN
  RAISE EXCEPTION 'ERR_QC_SIM_FINISH_BEFORE_ACCEPT';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: qc_spec_immutable_v21(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.qc_spec_immutable_v21() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE missing_sizes bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(OLD.tenant_id),hashtext('F34:SPEC:'||OLD.id::text));
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'ERR_F34_IMMUTABLE_SPEC'; END IF;
 IF (to_jsonb(OLD)-'enabled')<>(to_jsonb(NEW)-'enabled') THEN
  RAISE EXCEPTION 'ERR_F34_IMMUTABLE_SPEC'; END IF;
 IF OLD.enabled AND NOT NEW.enabled THEN RETURN NEW; END IF;
 IF NOT OLD.enabled AND NEW.enabled THEN
  IF EXISTS (SELECT 1 FROM qc_garment_events_v21 e WHERE e.tenant_id=OLD.tenant_id AND e.spec_id=OLD.id) THEN
   RAISE EXCEPTION 'ERR_F34_ALREADY_USED_SPEC'; END IF;
  SELECT count(*) INTO missing_sizes FROM (
    SELECT DISTINCT size_code FROM cut_lay_positions_v8 WHERE tenant_id=OLD.tenant_id AND lay_draft_id=OLD.lay_draft_id
  ) sizes WHERE NOT EXISTS (
    SELECT 1 FROM qc_garment_measure_rules_v21 m WHERE m.tenant_id=OLD.tenant_id
      AND m.spec_id=OLD.id AND m.size_code=sizes.size_code);
  IF missing_sizes<>0 OR NOT EXISTS (SELECT 1 FROM qc_garment_measure_rules_v21 m
    WHERE m.tenant_id=OLD.tenant_id AND m.spec_id=OLD.id) THEN
   RAISE EXCEPTION 'ERR_F34_MEASUREMENT_RULES_INCOMPLETE'; END IF;
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'ERR_F34_IMMUTABLE_SPEC';
END $$;


--
-- Name: reject_jobwork_scan_reuse_v28(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.reject_jobwork_scan_reuse_v28() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE ids uuid[]; sid uuid;
BEGIN
 IF TG_TABLE_NAME='sew_piece_completions_v27' THEN ids:=NEW.scan_ids;
 ELSE ids:=ARRAY[NEW.scan_id]; END IF;
 FOREACH sid IN ARRAY ids LOOP
  PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||sid::text));
  IF EXISTS(SELECT 1 FROM jobwork_used_scans_v28 j WHERE j.tenant_id=NEW.tenant_id AND j.scan_id=sid)
  THEN RAISE EXCEPTION 'ERR_F317_SCAN_ALREADY_USED_FOR_JOBWORK'; END IF;
 END LOOP;
 RETURN NEW;
END $$;


--
-- Name: reject_reused_sew_piece_scan_v27(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.reject_reused_sew_piece_scan_v27() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext('WIP:SCAN:'||NEW.scan_id::text));
 IF EXISTS(SELECT 1 FROM sew_piece_used_scans_v27 used
  WHERE used.tenant_id=NEW.tenant_id AND used.scan_id=NEW.scan_id)
 THEN RAISE EXCEPTION 'ERR_F316B_SCAN_SPENT_BY_SEWING'; END IF;
 RETURN NEW;
END $$;


--
-- Name: reject_whole_sew_when_piece_v27(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.reject_whole_sew_when_piece_v27() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM sew_piece_completions_v27 c
  JOIN gen_garments_v18 g ON g.tenant_id=c.tenant_id AND g.id=c.garment_id
  JOIN sew_prod_routes_v10 pr ON pr.tenant_id=c.tenant_id AND pr.id=c.route_id
  JOIN sew_prod_routes_v10 newroute ON newroute.tenant_id=NEW.tenant_id AND newroute.id=NEW.route_id
  WHERE c.tenant_id=NEW.tenant_id AND g.marker_position=NEW.marker_position
   AND c.ply_group=NEW.ply_group AND pr.lay_draft_id=newroute.lay_draft_id
   AND pr.operation_code=newroute.operation_code AND pr.plant_id=newroute.plant_id)
 THEN RAISE EXCEPTION 'ERR_F316B_MIXED_WHOLE_AND_PIECE_CREDIT'; END IF;
 RETURN NEW;
END $$;


--
-- Name: reserve_sew_piece_scans_v27(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.reserve_sew_piece_scans_v27() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
BEGIN
 INSERT INTO public.sew_piece_used_scans_v27(tenant_id,scan_id,completion_id)
 SELECT NEW.tenant_id,x,NEW.id FROM unnest(NEW.scan_ids) x;
 RETURN NEW;
END $$;


--
-- Name: reserve_sew_prod_observations_v10(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.reserve_sew_prod_observations_v10() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 INSERT INTO sew_prod_used_observations_v10(tenant_id,observation_id,completion_id)
 SELECT NEW.tenant_id,x,NEW.id FROM unnest(NEW.observation_ids) x;
 RETURN NEW;
END $$;


--
-- Name: sew_sim_immutable_v9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sew_sim_immutable_v9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN RAISE EXCEPTION 'ERR_SEW_SIM_EVIDENCE_IMMUTABLE'; END $$;


--
-- Name: trim_cross_ledger_proof_guard_v25(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trim_cross_ledger_proof_guard_v25() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
DECLARE a text; b text; first_proof text; second_proof text;
BEGIN
 IF TG_TABLE_NAME='trim_stock_admissions_v25' THEN
   a:=NEW.manager_proof_id; b:=NEW.inspector_proof_id;
 ELSE
   a:=NEW.issuer_proof_id; b:=NEW.recipient_proof_id;
 END IF;
 IF a IS NULL OR b IS NULL THEN RETURN NEW; END IF;
 first_proof:=least(a,b); second_proof:=greatest(a,b);
 PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext(first_proof));
 IF first_proof<>second_proof THEN
   PERFORM pg_advisory_xact_lock(hashtext(NEW.tenant_id),hashtext(second_proof));
 END IF;
 IF TG_TABLE_NAME='trim_stock_admissions_v25' THEN
   IF EXISTS (SELECT 1 FROM public.trim_issuance_proof_uses_v24 u
       WHERE u.tenant_id=NEW.tenant_id AND u.proof_id IN(a,b)) THEN
     RAISE EXCEPTION 'ERR_F314B_ISSUANCE_PROOF_REUSE';
   END IF;
 ELSE
   IF EXISTS (SELECT 1 FROM public.trim_admission_proof_uses_v25 u
       WHERE u.tenant_id=NEW.tenant_id AND u.proof_id IN(a,b)) THEN
     RAISE EXCEPTION 'ERR_F314B_ADMISSION_PROOF_REUSE';
   END IF;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: validate_cut_issue_attribution_v8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.validate_cut_issue_attribution_v8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE signed_mm bigint; already_mm bigint; okay boolean;
BEGIN
 SELECT i.length_mm,
  (i.cut_no=d.cut_no AND (i.order_id=d.intake_id OR
     (i.order_id=o.po_number AND
       (SELECT count(*) FROM order_intakes_v5 oo WHERE oo.tenant_id=d.tenant_id AND oo.po_number=o.po_number)=1))
    AND sr.factory_sku=d.fabric_sku AND sr.shade_band=d.shade_band)
 INTO signed_mm,okay
 FROM material_issues_v5 i
 JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
 JOIN cut_spread_reports_v8 s ON s.tenant_id=NEW.tenant_id AND s.id=NEW.spread_report_id
 JOIN cut_lay_drafts_v8 d ON d.tenant_id=s.tenant_id AND d.id=s.lay_draft_id
 JOIN order_intakes_v5 o ON o.tenant_id=d.tenant_id AND o.id=d.intake_id
 WHERE i.tenant_id=NEW.tenant_id AND i.id=NEW.issue_id FOR UPDATE OF i;
 IF NOT FOUND OR NOT okay THEN RAISE EXCEPTION 'ERR_CUT_ISSUE_SCOPE_MISMATCH'; END IF;
 SELECT COALESCE(sum(allocated_mm),0) INTO already_mm
 FROM cut_spread_issue_allocations_v8 WHERE tenant_id=NEW.tenant_id AND issue_id=NEW.issue_id;
 IF already_mm>signed_mm OR NEW.allocated_mm>signed_mm-already_mm THEN
  RAISE EXCEPTION 'ERR_CUT_ISSUE_DOUBLE_ATTRIBUTION';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: validate_cut_release_v8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.validate_cut_release_v8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE expected integer; humans integer; machines integer; plant_issues integer; issued_total bigint; reported bigint;
BEGIN
 SELECT count(*)::int INTO expected FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id;
 SELECT count(*)::int INTO humans FROM cut_bundle_scan_witnesses_v8 w
  WHERE w.tenant_id=NEW.tenant_id AND w.spread_report_id=NEW.spread_report_id AND w.plant_id=NEW.plant_id;
 SELECT count(*)::int INTO machines FROM cut_reader_events_v8 e
  JOIN cut_bundle_scan_witnesses_v8 w ON w.tenant_id=e.tenant_id AND w.spread_report_id=e.spread_report_id
   AND w.label_code=e.label_code AND w.epc_hex=e.epc_hex AND w.plant_id=NEW.plant_id
  JOIN cut_reader_gateways_v8 device ON device.tenant_id=e.tenant_id AND device.gateway_id=e.gateway_id
   AND device.enabled AND device.plant_id=NEW.plant_id
  WHERE e.tenant_id=NEW.tenant_id AND e.spread_report_id=NEW.spread_report_id AND e.plant_id=NEW.plant_id;
 SELECT s.reported_consumption_mm INTO reported FROM cut_spread_reports_v8 s WHERE s.tenant_id=NEW.tenant_id AND s.id=NEW.spread_report_id;
 SELECT COALESCE(sum(allocated_mm),0)::bigint INTO issued_total FROM cut_spread_issue_allocations_v8
  WHERE tenant_id=NEW.tenant_id AND spread_report_id=NEW.spread_report_id;
 SELECT count(*)::int INTO plant_issues FROM cut_spread_issue_allocations_v8 a
  JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id
  JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
  WHERE a.tenant_id=NEW.tenant_id AND a.spread_report_id=NEW.spread_report_id AND sr.plant_id<>NEW.plant_id;
 IF expected=0 OR expected<>NEW.bundle_count OR humans<>expected OR machines<>expected OR plant_issues<>0 OR issued_total<>reported THEN
  RAISE EXCEPTION 'ERR_CUT_RELEASE_INCOMPLETE_EVIDENCE';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: validate_sew_sim_credit_v9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.validate_sew_sim_credit_v9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE c sew_sim_cycles_v9%ROWTYPE; op sew_sim_operations_v9%ROWTYPE;
        counted integer; first_ply integer; last_ply integer; bad integer;
        ply_starts integer; ply_ends integer; colors integer; sizes integer; expected_serials text[];
BEGIN
 SELECT * INTO c FROM sew_sim_cycles_v9 WHERE tenant_id=NEW.tenant_id AND id=NEW.cycle_id;
 IF NOT FOUND OR c.state<>'COMPLETED' OR c.operation_id<>NEW.operation_id OR
   c.lay_draft_id<>NEW.lay_draft_id OR c.marker_position<>NEW.marker_position OR
   c.ply_group<>NEW.ply_group OR c.active_ms<>NEW.active_ms THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CREDIT_WITHOUT_COMPLETION';
 END IF;
 SELECT * INTO op FROM sew_sim_operations_v9 WHERE tenant_id=NEW.tenant_id AND id=NEW.operation_id;
 SELECT count(*)::int, min(b.ply_start),max(b.ply_end),
   count(DISTINCT b.ply_start)::int,count(DISTINCT b.ply_end)::int,
   count(DISTINCT b.color_code)::int,count(DISTINCT b.size_code)::int,
   count(*) FILTER (WHERE NOT (b.part_code=ANY(op.required_parts)) OR
       b.lay_draft_id<>NEW.lay_draft_id OR b.marker_position<>NEW.marker_position OR
       b.ply_group<>NEW.ply_group)::int
 INTO counted,first_ply,last_ply,ply_starts,ply_ends,colors,sizes,bad
 FROM sew_sim_cycle_inputs_v9 i
 JOIN cut_bundle_drafts_v8 b ON b.tenant_id=i.tenant_id AND b.label_code=i.label_code
 WHERE i.tenant_id=NEW.tenant_id AND i.cycle_id=NEW.cycle_id;
 IF counted<>cardinality(op.required_parts) OR bad<>0 OR
    ply_starts<>1 OR ply_ends<>1 OR colors<>1 OR sizes<>1 OR
   NEW.garment_count<>(last_ply-first_ply+1) THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CREDIT_INPUT_INVARIANT';
 END IF;
 SELECT array_agg(format('SIM-G-%s-%s-%s',NEW.lay_draft_id,NEW.marker_position,g.n) ORDER BY g.n)
 INTO expected_serials FROM generate_series(first_ply,last_ply) g(n);
 IF NEW.serials IS DISTINCT FROM expected_serials THEN
  RAISE EXCEPTION 'ERR_SEW_SIM_CREDIT_SERIAL_FORGERY';
 END IF;
 RETURN NEW;
END $$;


--
-- Name: wip_piece_active_hold_v26(character varying, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.wip_piece_active_hold_v26(t character varying, gid uuid) RETURNS boolean
    LANGUAGE sql
    AS $$
 SELECT EXISTS(SELECT 1 FROM wip_piece_holds_v26 h
  WHERE h.tenant_id=t AND h.garment_id=gid
  AND NOT EXISTS(SELECT 1 FROM wip_piece_reviews_v26 r
   WHERE r.tenant_id=h.tenant_id AND r.hold_id=h.id))
$$;


--
-- Name: workforce_oidc_begin_v16(bytea, bytea, text, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.workforce_oidc_begin_v16(p_state bytea, p_browser bytea, p_nonce text, p_pkce text) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
BEGIN
 IF octet_length(p_state)<>32 OR octet_length(p_browser)<>32 OR length(p_nonce) NOT BETWEEN 32 AND 128 OR length(p_pkce) NOT BETWEEN 43 AND 128 THEN RAISE EXCEPTION 'invalid login input'; END IF;
 DELETE FROM public.workforce_oidc_requests_v16 WHERE expires_at<now();
 INSERT INTO public.workforce_oidc_requests_v16(state_hash,browser_hash,nonce,pkce_verifier,expires_at)
 VALUES(p_state,p_browser,p_nonce,p_pkce,now()+interval '5 minutes');
END $$;


--
-- Name: workforce_oidc_consume_v16(bytea, bytea); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.workforce_oidc_consume_v16(p_state bytea, p_browser bytea) RETURNS TABLE(nonce text, pkce_verifier text)
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
BEGIN
 RETURN QUERY DELETE FROM public.workforce_oidc_requests_v16 q
 WHERE q.state_hash=p_state AND q.browser_hash=p_browser AND q.expires_at>now()
 RETURNING q.nonce,q.pkce_verifier;
END $$;


--
-- Name: workforce_session_create_v16(bytea, bytea, text, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.workforce_session_create_v16(p_session bytea, p_csrf bytea, p_issuer text, p_subject text) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
BEGIN
 IF octet_length(p_session)<>32 OR octet_length(p_csrf)<>32 OR length(p_issuer) NOT BETWEEN 9 AND 512 OR p_issuer NOT LIKE 'https://%' OR length(p_subject) NOT BETWEEN 1 AND 128 THEN RAISE EXCEPTION 'invalid session input'; END IF;
 INSERT INTO public.workforce_browser_sessions_v16(session_hash,csrf_hash,idp_issuer,idp_subject,expires_at)
 VALUES(p_session,p_csrf,p_issuer,p_subject,now()+interval '8 hours');
END $$;


--
-- Name: workforce_session_csrf_rotate_v16(bytea, bytea); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.workforce_session_csrf_rotate_v16(p_session bytea, p_csrf bytea) RETURNS boolean
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
DECLARE changed integer;
BEGIN
 IF octet_length(p_session)<>32 OR octet_length(p_csrf)<>32 THEN RETURN false; END IF;
 UPDATE public.workforce_browser_sessions_v16 SET csrf_hash=p_csrf WHERE session_hash=p_session
 AND revoked_at IS NULL AND expires_at>now() AND last_seen_at>now()-interval '20 minutes';
 GET DIAGNOSTICS changed=ROW_COUNT;
 RETURN changed=1;
END $$;


--
-- Name: workforce_session_get_v16(bytea); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.workforce_session_get_v16(p_session bytea) RETURNS TABLE(idp_issuer text, idp_subject text, csrf_hash bytea)
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
BEGIN
 RETURN QUERY UPDATE public.workforce_browser_sessions_v16 s
 SET last_seen_at=now() WHERE s.session_hash=p_session AND s.revoked_at IS NULL
 AND s.expires_at>now() AND s.last_seen_at>now()-interval '20 minutes'
 RETURNING s.idp_issuer,s.idp_subject::text,s.csrf_hash;
END $$;


--
-- Name: workforce_session_revoke_v16(bytea); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.workforce_session_revoke_v16(p_session bytea) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'public'
    AS $$
BEGIN
 UPDATE public.workforce_browser_sessions_v16 SET revoked_at=now() WHERE session_hash=p_session AND revoked_at IS NULL;
END $$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: attendance_register; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.attendance_register (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    worker_id character varying(64) NOT NULL,
    shift_date date DEFAULT CURRENT_DATE NOT NULL,
    clock_in_time timestamp with time zone,
    status character varying(32) NOT NULL,
    assigned_line_id character varying(64) DEFAULT ''::character varying NOT NULL,
    assigned_station_id character varying(64) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT attendance_register_status_check CHECK (((status)::text = ANY ((ARRAY['PRESENT'::character varying, 'ABSENT'::character varying, 'LATE'::character varying, 'ON_LEAVE'::character varying])::text[])))
);

ALTER TABLE ONLY public.attendance_register FORCE ROW LEVEL SECURITY;


--
-- Name: bom_trim_requirements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bom_trim_requirements (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    style_id character varying(64) NOT NULL,
    trim_code character varying(64) NOT NULL,
    trim_description character varying(255) NOT NULL,
    supplier_lot character varying(128),
    required_qty_per_garment numeric(10,3) NOT NULL,
    uom character varying(32) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT bom_trim_requirements_required_qty_per_garment_check CHECK ((required_qty_per_garment > (0)::numeric))
);

ALTER TABLE ONLY public.bom_trim_requirements FORCE ROW LEVEL SECURITY;


--
-- Name: bundle_registry; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bundle_registry (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) DEFAULT 'PLANT_DEFAULT'::character varying NOT NULL,
    order_id character varying(64) NOT NULL,
    cut_no integer NOT NULL,
    sequential_bundle_no integer NOT NULL,
    pant_position integer NOT NULL,
    part_code character varying(64) NOT NULL,
    sub_bundle_id integer DEFAULT 1 NOT NULL,
    ssb_ply_group integer NOT NULL,
    part_description character varying(128) NOT NULL,
    ply_start integer NOT NULL,
    ply_end integer NOT NULL,
    piece_count integer NOT NULL,
    garment_serial_start integer NOT NULL,
    garment_serial_end integer NOT NULL,
    active_nfc_uid character varying(128),
    mapped_by_helper_id character varying(64),
    mapped_at timestamp with time zone,
    aggregate_version bigint DEFAULT 0 NOT NULL,
    status character varying(32) NOT NULL,
    CONSTRAINT bundle_registry_check CHECK ((ply_end >= ply_start)),
    CONSTRAINT bundle_registry_check1 CHECK ((garment_serial_end >= garment_serial_start)),
    CONSTRAINT bundle_registry_cut_no_check CHECK ((cut_no > 0)),
    CONSTRAINT bundle_registry_garment_serial_start_check CHECK ((garment_serial_start > 0)),
    CONSTRAINT bundle_registry_pant_position_check CHECK ((pant_position > 0)),
    CONSTRAINT bundle_registry_piece_count_check CHECK ((piece_count > 0)),
    CONSTRAINT bundle_registry_ply_start_check CHECK ((ply_start > 0)),
    CONSTRAINT bundle_registry_sequential_bundle_no_check CHECK ((sequential_bundle_no > 0)),
    CONSTRAINT bundle_registry_ssb_ply_group_check CHECK ((ssb_ply_group > 0)),
    CONSTRAINT bundle_registry_status_check CHECK (((status)::text = ANY ((ARRAY['PENDING_MAP'::character varying, 'IN_SEWING'::character varying, 'COMPLETED'::character varying, 'SCRAPPED'::character varying])::text[]))),
    CONSTRAINT bundle_registry_sub_bundle_id_check CHECK ((sub_bundle_id > 0))
);

ALTER TABLE ONLY public.bundle_registry FORCE ROW LEVEL SECURITY;


--
-- Name: buyer_aql_plans_v32; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.buyer_aql_plans_v32 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    lot_code character varying(80) NOT NULL,
    revision integer NOT NULL,
    sample_garment_ids uuid[] NOT NULL,
    source_document_id character varying(128) NOT NULL,
    source_sha256 character(64) NOT NULL,
    sampling_method character varying(80) NOT NULL,
    provisioned_by character varying(128) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT buyer_aql_plans_v32_lot_code_check CHECK (((lot_code)::text ~ '^[A-Za-z0-9_.-]{4,80}$'::text)),
    CONSTRAINT buyer_aql_plans_v32_provisioned_by_check CHECK ((length(btrim((provisioned_by)::text)) > 0)),
    CONSTRAINT buyer_aql_plans_v32_revision_check CHECK (((revision >= 1) AND (revision <= 100000))),
    CONSTRAINT buyer_aql_plans_v32_sample_garment_ids_check CHECK (((array_ndims(sample_garment_ids) = 1) AND ((cardinality(sample_garment_ids) >= 1) AND (cardinality(sample_garment_ids) <= 500)))),
    CONSTRAINT buyer_aql_plans_v32_sampling_method_check CHECK ((length(btrim((sampling_method)::text)) >= 8)),
    CONSTRAINT buyer_aql_plans_v32_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.buyer_aql_plans_v32 FORCE ROW LEVEL SECURITY;


--
-- Name: buyer_aql_review_rules_v33; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.buyer_aql_review_rules_v33 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    plan_id uuid NOT NULL,
    max_minor integer NOT NULL,
    max_major integer NOT NULL,
    max_critical integer DEFAULT 0 NOT NULL,
    source_document_id character varying(128) NOT NULL,
    source_sha256 character(64) NOT NULL,
    provisioned_by character varying(128) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT buyer_aql_review_rules_v33_max_critical_check CHECK ((max_critical = 0)),
    CONSTRAINT buyer_aql_review_rules_v33_max_major_check CHECK ((max_major >= 0)),
    CONSTRAINT buyer_aql_review_rules_v33_max_minor_check CHECK ((max_minor >= 0)),
    CONSTRAINT buyer_aql_review_rules_v33_provisioned_by_check CHECK ((length(btrim((provisioned_by)::text)) > 0)),
    CONSTRAINT buyer_aql_review_rules_v33_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.buyer_aql_review_rules_v33 FORCE ROW LEVEL SECURITY;


--
-- Name: buyer_aql_reviews_v33; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.buyer_aql_reviews_v33 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    plan_id uuid NOT NULL,
    decision character varying(30) NOT NULL,
    pass_count integer NOT NULL,
    minor_count integer NOT NULL,
    major_count integer NOT NULL,
    critical_count integer NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    reviewer_subject character varying(128) NOT NULL,
    reviewer_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'INTERNAL_AQL_REVIEW_NOT_BUYER_LOT_ACCEPTANCE_OR_SHIPMENT'::text NOT NULL,
    CONSTRAINT buyer_aql_reviews_v33_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT buyer_aql_reviews_v33_authority_check CHECK ((authority = 'INTERNAL_AQL_REVIEW_NOT_BUYER_LOT_ACCEPTANCE_OR_SHIPMENT'::text)),
    CONSTRAINT buyer_aql_reviews_v33_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT buyer_aql_reviews_v33_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT buyer_aql_reviews_v33_decision_check CHECK (((decision)::text = ANY ((ARRAY['INTERNAL_RECOMMEND_ACCEPT'::character varying, 'HOLD_FOR_BUYER_REVIEW'::character varying])::text[]))),
    CONSTRAINT buyer_aql_reviews_v33_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 800)))
);

ALTER TABLE ONLY public.buyer_aql_reviews_v33 FORCE ROW LEVEL SECURITY;


--
-- Name: buyer_aql_sample_events_v32; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.buyer_aql_sample_events_v32 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    plan_id uuid NOT NULL,
    garment_id uuid NOT NULL,
    finding character varying(16) NOT NULL,
    defect_code character varying(64),
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    inspector_subject character varying(128) NOT NULL,
    inspector_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_AQL_SAMPLE_FINDING_NOT_LOT_ACCEPTANCE'::text NOT NULL,
    CONSTRAINT buyer_aql_sample_events_v32_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT buyer_aql_sample_events_v32_authority_check CHECK ((authority = 'SIGNED_AQL_SAMPLE_FINDING_NOT_LOT_ACCEPTANCE'::text)),
    CONSTRAINT buyer_aql_sample_events_v32_check CHECK (((((finding)::text = 'PASS'::text) AND (defect_code IS NULL)) OR (((finding)::text <> 'PASS'::text) AND (defect_code IS NOT NULL)))),
    CONSTRAINT buyer_aql_sample_events_v32_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT buyer_aql_sample_events_v32_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT buyer_aql_sample_events_v32_finding_check CHECK (((finding)::text = ANY ((ARRAY['PASS'::character varying, 'MINOR'::character varying, 'MAJOR'::character varying, 'CRITICAL'::character varying])::text[]))),
    CONSTRAINT buyer_aql_sample_events_v32_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 800)))
);

ALTER TABLE ONLY public.buyer_aql_sample_events_v32 FORCE ROW LEVEL SECURITY;


--
-- Name: buyer_order_handoffs_v65c1; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.buyer_order_handoffs_v65c1 (
    id bigint NOT NULL,
    tenant_id character varying(64) NOT NULL,
    intake_id character varying(40) NOT NULL,
    po_document_id character varying(36) NOT NULL,
    bom_document_id character varying(36) NOT NULL,
    source_digest character(64) NOT NULL,
    reviewed_by character varying(64) NOT NULL,
    review_note character varying(1000) NOT NULL,
    mapping jsonb NOT NULL,
    captured_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT buyer_order_handoffs_v65c1_review_note_check CHECK ((char_length((review_note)::text) >= 8))
);

ALTER TABLE ONLY public.buyer_order_handoffs_v65c1 FORCE ROW LEVEL SECURITY;


--
-- Name: buyer_order_handoffs_v65c1_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.buyer_order_handoffs_v65c1 ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.buyer_order_handoffs_v65c1_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: carton_pack_manifests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.carton_pack_manifests (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) DEFAULT 'PLANT_DEFAULT'::character varying NOT NULL,
    order_id character varying(64) NOT NULL,
    box_range_start integer NOT NULL,
    box_range_end integer NOT NULL,
    pack_type character varying(32) NOT NULL,
    color_code character varying(64) DEFAULT ''::character varying NOT NULL,
    slot_rules_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    expected_gross_weight_kg numeric(6,3) NOT NULL,
    weight_tolerance_kg numeric(4,3) DEFAULT 0.250 NOT NULL,
    aggregate_version bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT carton_pack_manifests_box_range_start_check CHECK ((box_range_start > 0)),
    CONSTRAINT carton_pack_manifests_check CHECK ((box_range_end >= box_range_start)),
    CONSTRAINT carton_pack_manifests_expected_gross_weight_kg_check CHECK ((expected_gross_weight_kg > (0)::numeric)),
    CONSTRAINT carton_pack_manifests_pack_type_check CHECK (((pack_type)::text = ANY ((ARRAY['SOLID'::character varying, 'RATIO'::character varying])::text[])))
);

ALTER TABLE ONLY public.carton_pack_manifests FORCE ROW LEVEL SECURITY;


--
-- Name: cut_bundle_drafts_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_bundle_drafts_v8 (
    tenant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    label_code character varying(29) NOT NULL,
    marker_position integer NOT NULL,
    color_code character varying(64) NOT NULL,
    size_code character varying(32) NOT NULL,
    part_code character varying(64) NOT NULL,
    pieces_per_garment integer NOT NULL,
    ply_group integer NOT NULL,
    ply_start integer NOT NULL,
    ply_end integer NOT NULL,
    piece_count integer NOT NULL,
    rfid_state character varying(32) DEFAULT 'UNBOUND_DRAFT_LABEL'::character varying NOT NULL,
    CONSTRAINT cut_bundle_drafts_v8_check CHECK (((ply_end >= ply_start) AND (ply_end <= 210))),
    CONSTRAINT cut_bundle_drafts_v8_check1 CHECK ((piece_count = (((ply_end - ply_start) + 1) * pieces_per_garment))),
    CONSTRAINT cut_bundle_drafts_v8_label_code_check CHECK (((label_code)::text ~ '^CB08-[0-9A-F]{24}$'::text)),
    CONSTRAINT cut_bundle_drafts_v8_pieces_per_garment_check CHECK (((pieces_per_garment >= 1) AND (pieces_per_garment <= 8))),
    CONSTRAINT cut_bundle_drafts_v8_ply_group_check CHECK (((ply_group >= 1) AND (ply_group <= 210))),
    CONSTRAINT cut_bundle_drafts_v8_ply_start_check CHECK (((ply_start >= 1) AND (ply_start <= 210))),
    CONSTRAINT cut_bundle_drafts_v8_rfid_state_check CHECK (((rfid_state)::text = 'UNBOUND_DRAFT_LABEL'::text))
);

ALTER TABLE ONLY public.cut_bundle_drafts_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_bundle_scan_witnesses_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_bundle_scan_witnesses_v8 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    spread_report_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    label_code character varying(29) NOT NULL,
    epc_hex character varying(96) NOT NULL,
    scanner_subject character varying(128) NOT NULL,
    scanner_proof_id character varying(255) NOT NULL,
    scanner_assertion_hash character(64) NOT NULL,
    signed_challenge_hash character(64) NOT NULL,
    witness_state character varying(48) DEFAULT 'SIGNED_HUMAN_WITNESS_NOT_HARDWARE_ATTESTED'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_bundle_scan_witnesses_v8_epc_hex_check CHECK ((((epc_hex)::text ~ '^[0-9A-F]{24,96}$'::text) AND ((length((epc_hex)::text) % 2) = 0))),
    CONSTRAINT cut_bundle_scan_witnesses_v8_scanner_assertion_hash_check CHECK ((scanner_assertion_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_bundle_scan_witnesses_v8_signed_challenge_hash_check CHECK ((signed_challenge_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_bundle_scan_witnesses_v8_witness_state_check CHECK (((witness_state)::text = 'SIGNED_HUMAN_WITNESS_NOT_HARDWARE_ATTESTED'::text))
);

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_consumption_reconciliations_v19; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_consumption_reconciliations_v19 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    release_id uuid NOT NULL,
    spread_report_id uuid NOT NULL,
    used_part_mm bigint NOT NULL,
    trim_waste_mm bigint NOT NULL,
    damaged_mm bigint NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    supervisor_subject character varying(128) NOT NULL,
    quality_subject character varying(128) NOT NULL,
    supervisor_proof_id character varying(255) NOT NULL,
    quality_proof_id character varying(255) NOT NULL,
    supervisor_assertion_sha256 character(64) NOT NULL,
    quality_assertion_sha256 character(64) NOT NULL,
    note text NOT NULL,
    evidence text DEFAULT 'DUAL_HUMAN_CLASSIFICATION_NOT_SENSOR_MEASUREMENT'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_consumption_reconciliations_v19_check CHECK ((((supervisor_subject)::text <> (quality_subject)::text) AND ((supervisor_proof_id)::text <> (quality_proof_id)::text))),
    CONSTRAINT cut_consumption_reconciliations_v19_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT cut_consumption_reconciliations_v19_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_consumption_reconciliations_v19_damaged_mm_check CHECK ((damaged_mm >= 0)),
    CONSTRAINT cut_consumption_reconciliations_v19_evidence_check CHECK ((evidence = 'DUAL_HUMAN_CLASSIFICATION_NOT_SENSOR_MEASUREMENT'::text)),
    CONSTRAINT cut_consumption_reconciliations_v19_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 1000))),
    CONSTRAINT cut_consumption_reconciliations_v19_trim_waste_mm_check CHECK ((trim_waste_mm >= 0)),
    CONSTRAINT cut_consumption_reconciliations_v19_used_part_mm_check CHECK ((used_part_mm >= 0))
);

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_defect_codes_v19; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_defect_codes_v19 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    code character varying(64) NOT NULL,
    description character varying(512) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    CONSTRAINT cut_defect_codes_v19_code_check CHECK (((code)::text ~ '^[A-Za-z0-9_-]{2,64}$'::text)),
    CONSTRAINT cut_defect_codes_v19_description_check CHECK ((length(btrim((description)::text)) > 0))
);

ALTER TABLE ONLY public.cut_defect_codes_v19 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_lay_drafts_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_lay_drafts_v8 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    intake_id character varying(40) NOT NULL,
    cut_no integer NOT NULL,
    revision integer NOT NULL,
    request_key character varying(128) NOT NULL,
    input_sha256 character(64) NOT NULL,
    source_order_sha256 character(64) NOT NULL,
    marker_code character varying(64) NOT NULL,
    fabric_sku character varying(128) NOT NULL,
    shade_band character varying(32) NOT NULL,
    marker_length_mm bigint NOT NULL,
    usable_width_mm integer NOT NULL,
    reported_plies integer NOT NULL,
    bundle_ply_group_size integer NOT NULL,
    planned_garments integer NOT NULL,
    review_note text NOT NULL,
    state character varying(32) DEFAULT 'PROPOSED_HOLD'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_lay_drafts_v8_bundle_ply_group_size_check CHECK (((bundle_ply_group_size >= 1) AND (bundle_ply_group_size <= 210))),
    CONSTRAINT cut_lay_drafts_v8_cut_no_check CHECK (((cut_no >= 1) AND (cut_no <= 100000))),
    CONSTRAINT cut_lay_drafts_v8_input_sha256_check CHECK ((input_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_lay_drafts_v8_marker_length_mm_check CHECK (((marker_length_mm >= 100) AND (marker_length_mm <= 100000000))),
    CONSTRAINT cut_lay_drafts_v8_planned_garments_check CHECK (((planned_garments >= 1) AND (planned_garments <= 16800))),
    CONSTRAINT cut_lay_drafts_v8_reported_plies_check CHECK (((reported_plies >= 1) AND (reported_plies <= 210))),
    CONSTRAINT cut_lay_drafts_v8_request_key_check CHECK (((request_key)::text ~ '^[A-Za-z0-9_-]{8,128}$'::text)),
    CONSTRAINT cut_lay_drafts_v8_review_note_check CHECK (((length(btrim(review_note)) >= 12) AND (length(btrim(review_note)) <= 1000))),
    CONSTRAINT cut_lay_drafts_v8_revision_check CHECK (((revision >= 1) AND (revision <= 10001))),
    CONSTRAINT cut_lay_drafts_v8_source_order_sha256_check CHECK ((source_order_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_lay_drafts_v8_state_check CHECK (((state)::text = 'PROPOSED_HOLD'::text)),
    CONSTRAINT cut_lay_drafts_v8_usable_width_mm_check CHECK (((usable_width_mm >= 100) AND (usable_width_mm <= 4000)))
);

ALTER TABLE ONLY public.cut_lay_drafts_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_lay_positions_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_lay_positions_v8 (
    tenant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    marker_position integer NOT NULL,
    color_code character varying(64) NOT NULL,
    size_code character varying(32) NOT NULL,
    CONSTRAINT cut_lay_positions_v8_marker_position_check CHECK (((marker_position >= 1) AND (marker_position <= 80)))
);

ALTER TABLE ONLY public.cut_lay_positions_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_lay_rolls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_lay_rolls (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    cut_no integer NOT NULL,
    lay_start integer NOT NULL,
    lay_end integer NOT NULL,
    plies_count integer NOT NULL,
    end_bit_length_m numeric(10,2) NOT NULL,
    end_bit_action character varying(32) NOT NULL,
    recorded_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT cut_lay_rolls_check CHECK ((lay_end >= lay_start)),
    CONSTRAINT cut_lay_rolls_cut_no_check CHECK ((cut_no > 0)),
    CONSTRAINT cut_lay_rolls_end_bit_action_check CHECK (((end_bit_action)::text = ANY ((ARRAY['SCRAP'::character varying, 'RECUT_HOLD'::character varying, 'STORE_RETURN'::character varying])::text[]))),
    CONSTRAINT cut_lay_rolls_end_bit_length_m_check CHECK ((end_bit_length_m >= (0)::numeric)),
    CONSTRAINT cut_lay_rolls_lay_start_check CHECK ((lay_start > 0)),
    CONSTRAINT cut_lay_rolls_plies_count_check CHECK (((plies_count >= 1) AND (plies_count <= 210)))
);

ALTER TABLE ONLY public.cut_lay_rolls FORCE ROW LEVEL SECURITY;


--
-- Name: cut_lay_snapshots_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_lay_snapshots_v8 (
    tenant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    snapshot jsonb NOT NULL,
    snapshot_sha256 character(64) NOT NULL,
    CONSTRAINT cut_lay_snapshots_v8_snapshot_check CHECK ((jsonb_typeof(snapshot) = 'object'::text)),
    CONSTRAINT cut_lay_snapshots_v8_snapshot_sha256_check CHECK ((snapshot_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.cut_lay_snapshots_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_part_outcomes_v19; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_part_outcomes_v19 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    part_id uuid NOT NULL,
    release_id uuid NOT NULL,
    disposition character varying(12) NOT NULL,
    defect_code character varying(64),
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    supervisor_subject character varying(128) NOT NULL,
    quality_subject character varying(128) NOT NULL,
    supervisor_proof_id character varying(255) NOT NULL,
    quality_proof_id character varying(255) NOT NULL,
    supervisor_assertion_sha256 character(64) NOT NULL,
    quality_assertion_sha256 character(64) NOT NULL,
    note text NOT NULL,
    evidence text DEFAULT 'DUAL_HUMAN_PART_DISPOSITION_NOT_SENSOR_VERIFIED'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_part_outcomes_v19_check CHECK ((((supervisor_subject)::text <> (quality_subject)::text) AND ((supervisor_proof_id)::text <> (quality_proof_id)::text))),
    CONSTRAINT cut_part_outcomes_v19_check1 CHECK (((((disposition)::text = 'ACCEPTED'::text) AND (defect_code IS NULL)) OR (((disposition)::text = 'REJECTED'::text) AND (defect_code IS NOT NULL)))),
    CONSTRAINT cut_part_outcomes_v19_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT cut_part_outcomes_v19_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_part_outcomes_v19_disposition_check CHECK (((disposition)::text = ANY ((ARRAY['ACCEPTED'::character varying, 'REJECTED'::character varying])::text[]))),
    CONSTRAINT cut_part_outcomes_v19_evidence_check CHECK ((evidence = 'DUAL_HUMAN_PART_DISPOSITION_NOT_SENSOR_VERIFIED'::text)),
    CONSTRAINT cut_part_outcomes_v19_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 1000)))
);

ALTER TABLE ONLY public.cut_part_outcomes_v19 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_proof_consumptions_v19; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_proof_consumptions_v19 (
    tenant_id character varying(64) NOT NULL,
    proof_id character varying(255) NOT NULL,
    entity_type character varying(16) NOT NULL,
    entity_id uuid NOT NULL,
    actor_subject character varying(128) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_proof_consumptions_v19_entity_type_check CHECK (((entity_type)::text = ANY ((ARRAY['CONSUMPTION'::character varying, 'PART'::character varying, 'RECUT'::character varying])::text[])))
);

ALTER TABLE ONLY public.cut_proof_consumptions_v19 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_reader_events_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_reader_events_v8 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    gateway_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    event_id character varying(64) NOT NULL,
    observed_at timestamp with time zone NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    lay_draft_id uuid NOT NULL,
    spread_report_id uuid NOT NULL,
    label_code character varying(29) NOT NULL,
    epc_hex character varying(96) NOT NULL,
    antenna_port integer NOT NULL,
    event_sha256 character(64) NOT NULL,
    signature_sha256 character(64) NOT NULL,
    state character varying(40) DEFAULT 'SIGNED_GATEWAY_REPORT_UNVERIFIED_RF'::character varying NOT NULL,
    CONSTRAINT cut_reader_events_v8_antenna_port_check CHECK (((antenna_port >= 1) AND (antenna_port <= 64))),
    CONSTRAINT cut_reader_events_v8_epc_hex_check CHECK ((((epc_hex)::text ~ '^[0-9A-F]{24,96}$'::text) AND ((length((epc_hex)::text) % 2) = 0))),
    CONSTRAINT cut_reader_events_v8_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,64}$'::text)),
    CONSTRAINT cut_reader_events_v8_event_sha256_check CHECK ((event_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_reader_events_v8_signature_sha256_check CHECK ((signature_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_reader_events_v8_state_check CHECK (((state)::text = 'SIGNED_GATEWAY_REPORT_UNVERIFIED_RF'::text))
);

ALTER TABLE ONLY public.cut_reader_events_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_reader_gateways_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_reader_gateways_v8 (
    tenant_id character varying(64) NOT NULL,
    gateway_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    public_key bytea NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_reader_gateways_v8_gateway_id_check CHECK (((gateway_id)::text ~ '^[A-Za-z0-9_-]{3,64}$'::text)),
    CONSTRAINT cut_reader_gateways_v8_public_key_check CHECK ((octet_length(public_key) = 32)),
    CONSTRAINT cut_reader_gateways_v8_station_id_check CHECK (((length((station_id)::text) >= 2) AND (length((station_id)::text) <= 64)))
);

ALTER TABLE ONLY public.cut_reader_gateways_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_recut_links_v19; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_recut_links_v19 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    original_part_id uuid NOT NULL,
    replacement_part_id uuid NOT NULL,
    release_id uuid NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    supervisor_subject character varying(128) NOT NULL,
    quality_subject character varying(128) NOT NULL,
    supervisor_proof_id character varying(255) NOT NULL,
    quality_proof_id character varying(255) NOT NULL,
    supervisor_assertion_sha256 character(64) NOT NULL,
    quality_assertion_sha256 character(64) NOT NULL,
    note text NOT NULL,
    evidence text DEFAULT 'IDENTIFIED_RECUT_NOT_ACCEPTED_OR_CONSUMPTION_CREDIT'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_recut_links_v19_check CHECK (((original_part_id <> replacement_part_id) AND ((supervisor_subject)::text <> (quality_subject)::text) AND ((supervisor_proof_id)::text <> (quality_proof_id)::text))),
    CONSTRAINT cut_recut_links_v19_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT cut_recut_links_v19_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_recut_links_v19_evidence_check CHECK ((evidence = 'IDENTIFIED_RECUT_NOT_ACCEPTED_OR_CONSUMPTION_CREDIT'::text)),
    CONSTRAINT cut_recut_links_v19_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 1000)))
);

ALTER TABLE ONLY public.cut_recut_links_v19 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_sewing_releases_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_sewing_releases_v8 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    spread_report_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    evidence_sha256 character(64) NOT NULL,
    supervisor_subject character varying(128) NOT NULL,
    quality_subject character varying(128) NOT NULL,
    supervisor_proof_id character varying(255) NOT NULL,
    quality_proof_id character varying(255) NOT NULL,
    supervisor_assertion_sha256 character(64) NOT NULL,
    quality_assertion_sha256 character(64) NOT NULL,
    bundle_count integer NOT NULL,
    state character varying(48) DEFAULT 'RELEASED_TO_SEWING_SIGNED'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_sewing_releases_v8_bundle_count_check CHECK ((bundle_count > 0)),
    CONSTRAINT cut_sewing_releases_v8_check CHECK (((supervisor_subject)::text <> (quality_subject)::text)),
    CONSTRAINT cut_sewing_releases_v8_check1 CHECK (((supervisor_proof_id)::text <> (quality_proof_id)::text)),
    CONSTRAINT cut_sewing_releases_v8_evidence_sha256_check CHECK ((evidence_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_sewing_releases_v8_state_check CHECK (((state)::text = 'RELEASED_TO_SEWING_SIGNED'::text))
);

ALTER TABLE ONLY public.cut_sewing_releases_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_spread_issue_allocations_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_spread_issue_allocations_v8 (
    tenant_id character varying(64) NOT NULL,
    spread_report_id uuid NOT NULL,
    issue_id uuid NOT NULL,
    allocated_mm bigint NOT NULL,
    CONSTRAINT cut_spread_issue_allocations_v8_allocated_mm_check CHECK ((allocated_mm > 0))
);

ALTER TABLE ONLY public.cut_spread_issue_allocations_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: cut_spread_reports_v8; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cut_spread_reports_v8 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    request_key character varying(128) NOT NULL,
    input_sha256 character(64) NOT NULL,
    draft_snapshot_sha256 character(64) NOT NULL,
    reported_actual_plies integer NOT NULL,
    measured_marker_length_mm bigint NOT NULL,
    reported_consumption_mm bigint NOT NULL,
    evidence_note text NOT NULL,
    state character varying(48) DEFAULT 'REPORTED_PENDING_INDEPENDENT_VERIFICATION'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cut_spread_reports_v8_draft_snapshot_sha256_check CHECK ((draft_snapshot_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_spread_reports_v8_evidence_note_check CHECK (((length(btrim(evidence_note)) >= 12) AND (length(btrim(evidence_note)) <= 1000))),
    CONSTRAINT cut_spread_reports_v8_input_sha256_check CHECK ((input_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT cut_spread_reports_v8_measured_marker_length_mm_check CHECK (((measured_marker_length_mm >= 100) AND (measured_marker_length_mm <= 100000000))),
    CONSTRAINT cut_spread_reports_v8_reported_actual_plies_check CHECK (((reported_actual_plies >= 1) AND (reported_actual_plies <= 210))),
    CONSTRAINT cut_spread_reports_v8_reported_consumption_mm_check CHECK ((reported_consumption_mm > 0)),
    CONSTRAINT cut_spread_reports_v8_request_key_check CHECK (((request_key)::text ~ '^[A-Za-z0-9_-]{8,128}$'::text)),
    CONSTRAINT cut_spread_reports_v8_state_check CHECK (((state)::text = 'REPORTED_PENDING_INDEPENDENT_VERIFICATION'::text))
);

ALTER TABLE ONLY public.cut_spread_reports_v8 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_asn_drafts_v34; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_asn_drafts_v34 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    manifest_id uuid NOT NULL,
    carrier_receipt_id uuid NOT NULL,
    asn_reference character varying(128) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'INTERNAL_ASN_DRAFT_NOT_TRANSMITTED_OR_ACKNOWLEDGED'::text NOT NULL,
    CONSTRAINT dock_asn_drafts_v34_asn_reference_check CHECK ((length(btrim((asn_reference)::text)) >= 6)),
    CONSTRAINT dock_asn_drafts_v34_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_asn_drafts_v34_authority_check CHECK ((authority = 'INTERNAL_ASN_DRAFT_NOT_TRANSMITTED_OR_ACKNOWLEDGED'::text)),
    CONSTRAINT dock_asn_drafts_v34_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT dock_asn_drafts_v34_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_asn_drafts_v34_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 800))),
    CONSTRAINT dock_asn_drafts_v34_payload_sha256_check CHECK ((payload_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.dock_asn_drafts_v34 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_buyer_spec_bindings_v36; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_buyer_spec_bindings_v36 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    spec_id uuid NOT NULL,
    buyer_code character varying(64) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    contract_reference character varying(128) NOT NULL,
    approved_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT dock_buyer_spec_bindings_v36_approved_by_check CHECK ((length(btrim((approved_by)::text)) > 0)),
    CONSTRAINT dock_buyer_spec_bindings_v36_buyer_code_check CHECK (((buyer_code)::text ~ '^[A-Za-z0-9_-]{3,64}$'::text)),
    CONSTRAINT dock_buyer_spec_bindings_v36_contract_reference_check CHECK (((length(btrim((contract_reference)::text)) >= 6) AND (length(btrim((contract_reference)::text)) <= 128)))
);

ALTER TABLE ONLY public.dock_buyer_spec_bindings_v36 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_carrier_authorizations_v34; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_carrier_authorizations_v34 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    carrier_code character varying(64) NOT NULL,
    idp_issuer character varying(512) NOT NULL,
    idp_subject character varying(128) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    CONSTRAINT dock_carrier_authorizations_v34_carrier_code_check CHECK (((carrier_code)::text ~ '^[A-Za-z0-9_-]{3,64}$'::text)),
    CONSTRAINT dock_carrier_authorizations_v34_idp_issuer_check CHECK (((idp_issuer)::text ~~ 'https://%'::text)),
    CONSTRAINT dock_carrier_authorizations_v34_idp_subject_check CHECK ((length(btrim((idp_subject)::text)) > 0))
);

ALTER TABLE ONLY public.dock_carrier_authorizations_v34 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_carrier_receipts_v34; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_carrier_receipts_v34 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    manifest_id uuid NOT NULL,
    seal_id uuid NOT NULL,
    carrier_code character varying(64) NOT NULL,
    receipt_reference character varying(128) NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    actor_issuer character varying(512) NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'ONBOARDED_CARRIER_PERSONNEL_ATTESTATION_NOT_EXTERNAL_API_RECEIPT'::text NOT NULL,
    CONSTRAINT dock_carrier_receipts_v34_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_carrier_receipts_v34_authority_check CHECK ((authority = 'ONBOARDED_CARRIER_PERSONNEL_ATTESTATION_NOT_EXTERNAL_API_RECEIPT'::text)),
    CONSTRAINT dock_carrier_receipts_v34_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT dock_carrier_receipts_v34_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_carrier_receipts_v34_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 800))),
    CONSTRAINT dock_carrier_receipts_v34_receipt_reference_check CHECK ((length(btrim((receipt_reference)::text)) >= 6))
);

ALTER TABLE ONLY public.dock_carrier_receipts_v34 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_carton_scans_v34; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_carton_scans_v34 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    manifest_id uuid NOT NULL,
    carton_id uuid NOT NULL,
    gateway_id character varying(64) NOT NULL,
    event_id character varying(80) NOT NULL,
    carton_code character varying(80) NOT NULL,
    observed_at timestamp with time zone NOT NULL,
    received_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    event_sha256 character(64) NOT NULL,
    gateway_key_sha256 character(64) NOT NULL,
    signature_sha256 character(64) NOT NULL,
    authority text DEFAULT 'ENROLLED_GATEWAY_SCAN_NOT_INDEPENDENT_RF_CERTIFICATION'::text NOT NULL,
    CONSTRAINT dock_carton_scans_v34_authority_check CHECK ((authority = 'ENROLLED_GATEWAY_SCAN_NOT_INDEPENDENT_RF_CERTIFICATION'::text)),
    CONSTRAINT dock_carton_scans_v34_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT dock_carton_scans_v34_event_sha256_check CHECK ((event_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_carton_scans_v34_gateway_key_sha256_check CHECK ((gateway_key_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_carton_scans_v34_signature_sha256_check CHECK ((signature_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.dock_carton_scans_v34 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_container_seals_v34; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_container_seals_v34 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    manifest_id uuid NOT NULL,
    container_ref character varying(80) NOT NULL,
    seal_code character varying(80) NOT NULL,
    scanned_cartons integer NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SUPERVISOR_SEALED_CONTAINER_NOT_CARRIER_HANDOVER'::text NOT NULL,
    CONSTRAINT dock_container_seals_v34_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_container_seals_v34_authority_check CHECK ((authority = 'SUPERVISOR_SEALED_CONTAINER_NOT_CARRIER_HANDOVER'::text)),
    CONSTRAINT dock_container_seals_v34_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT dock_container_seals_v34_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_container_seals_v34_container_ref_check CHECK ((length(btrim((container_ref)::text)) >= 3)),
    CONSTRAINT dock_container_seals_v34_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 800))),
    CONSTRAINT dock_container_seals_v34_scanned_cartons_check CHECK ((scanned_cartons > 0)),
    CONSTRAINT dock_container_seals_v34_seal_code_check CHECK ((length(btrim((seal_code)::text)) >= 3))
);

ALTER TABLE ONLY public.dock_container_seals_v34 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_external_assertions_v35; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_external_assertions_v35 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    manifest_id uuid NOT NULL,
    asn_draft_id uuid NOT NULL,
    signer_id character varying(64) NOT NULL,
    party_kind text NOT NULL,
    party_code character varying(64) NOT NULL,
    assertion_kind text NOT NULL,
    decision text NOT NULL,
    event_id character varying(80) NOT NULL,
    external_reference character varying(128) NOT NULL,
    asn_reference character varying(128) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    issued_at timestamp with time zone NOT NULL,
    received_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    event_sha256 character(64) NOT NULL,
    signer_key_sha256 character(64) NOT NULL,
    signature_sha256 character(64) NOT NULL,
    authority text DEFAULT 'EXTERNAL_SIGNED_ASSERTION_NOT_INDEPENDENT_DELIVERY_OR_LEGAL_ACCEPTANCE'::text NOT NULL,
    CONSTRAINT dock_external_assertions_v35_assertion_kind_check CHECK ((assertion_kind = ANY (ARRAY['CARRIER_ASN_ACK'::text, 'BUYER_LOT_REVIEW'::text]))),
    CONSTRAINT dock_external_assertions_v35_authority_check CHECK ((authority = 'EXTERNAL_SIGNED_ASSERTION_NOT_INDEPENDENT_DELIVERY_OR_LEGAL_ACCEPTANCE'::text)),
    CONSTRAINT dock_external_assertions_v35_check CHECK ((((assertion_kind = 'CARRIER_ASN_ACK'::text) AND (party_kind = 'CARRIER'::text) AND (decision = 'ACKNOWLEDGED'::text)) OR ((assertion_kind = 'BUYER_LOT_REVIEW'::text) AND (party_kind = 'BUYER'::text) AND (decision = ANY (ARRAY['REVIEWED_ACCEPT'::text, 'REVIEWED_REJECT'::text]))))),
    CONSTRAINT dock_external_assertions_v35_decision_check CHECK ((decision = ANY (ARRAY['ACKNOWLEDGED'::text, 'REVIEWED_ACCEPT'::text, 'REVIEWED_REJECT'::text]))),
    CONSTRAINT dock_external_assertions_v35_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT dock_external_assertions_v35_event_sha256_check CHECK ((event_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_external_assertions_v35_external_reference_check CHECK (((length(btrim((external_reference)::text)) >= 6) AND (length(btrim((external_reference)::text)) <= 128))),
    CONSTRAINT dock_external_assertions_v35_party_kind_check CHECK ((party_kind = ANY (ARRAY['CARRIER'::text, 'BUYER'::text]))),
    CONSTRAINT dock_external_assertions_v35_payload_sha256_check CHECK ((payload_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_external_assertions_v35_signature_sha256_check CHECK ((signature_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT dock_external_assertions_v35_signer_key_sha256_check CHECK ((signer_key_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.dock_external_assertions_v35 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_external_signers_v35; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_external_signers_v35 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    signer_id character varying(64) NOT NULL,
    party_kind text NOT NULL,
    party_code character varying(64) NOT NULL,
    public_key bytea NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    CONSTRAINT dock_external_signers_v35_approved_by_check CHECK ((length(btrim((approved_by)::text)) > 0)),
    CONSTRAINT dock_external_signers_v35_party_code_check CHECK (((party_code)::text ~ '^[A-Za-z0-9_-]{3,64}$'::text)),
    CONSTRAINT dock_external_signers_v35_party_kind_check CHECK ((party_kind = ANY (ARRAY['CARRIER'::text, 'BUYER'::text]))),
    CONSTRAINT dock_external_signers_v35_public_key_check CHECK ((octet_length(public_key) = 32)),
    CONSTRAINT dock_external_signers_v35_signer_id_check CHECK (((signer_id)::text ~ '^[A-Za-z0-9_-]{3,64}$'::text))
);

ALTER TABLE ONLY public.dock_external_signers_v35 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_gateways_v34; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_gateways_v34 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    gateway_id character varying(64) NOT NULL,
    public_key bytea NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    CONSTRAINT dock_gateways_v34_gateway_id_check CHECK (((gateway_id)::text ~ '^[A-Za-z0-9_-]{3,64}$'::text)),
    CONSTRAINT dock_gateways_v34_public_key_check CHECK ((octet_length(public_key) = 32))
);

ALTER TABLE ONLY public.dock_gateways_v34 FORCE ROW LEVEL SECURITY;


--
-- Name: dock_station_policies_v34; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dock_station_policies_v34 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    max_scan_age_minutes integer NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    CONSTRAINT dock_station_policies_v34_max_scan_age_minutes_check CHECK (((max_scan_age_minutes >= 5) AND (max_scan_age_minutes <= 1440)))
);

ALTER TABLE ONLY public.dock_station_policies_v34 FORCE ROW LEVEL SECURITY;


--
-- Name: document_assertions_v65; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.document_assertions_v65 (
    tenant_id character varying(64) NOT NULL,
    document_id character varying(36) NOT NULL,
    assertion_id character varying(36) NOT NULL,
    field_path text NOT NULL,
    value_json jsonb NOT NULL,
    source_ordinal integer NOT NULL,
    review_state character varying(20) DEFAULT 'UNREVIEWED'::character varying NOT NULL,
    CONSTRAINT document_assertions_v65_review_state_check CHECK (((review_state)::text = 'UNREVIEWED'::text))
);

ALTER TABLE ONLY public.document_assertions_v65 FORCE ROW LEVEL SECURITY;


--
-- Name: document_jobs_v65; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.document_jobs_v65 (
    tenant_id character varying(64) NOT NULL,
    document_id character varying(36) NOT NULL,
    status character varying(24) DEFAULT 'QUEUED'::character varying NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    outcome text DEFAULT ''::text NOT NULL,
    segment_count integer DEFAULT 0 NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT document_jobs_v65_attempts_check CHECK (((attempts >= 0) AND (attempts <= 3))),
    CONSTRAINT document_jobs_v65_segment_count_check CHECK ((segment_count >= 0)),
    CONSTRAINT document_jobs_v65_status_check CHECK (((status)::text = ANY ((ARRAY['QUEUED'::character varying, 'PROCESSING'::character varying, 'EXTRACTED'::character varying, 'PARTIAL'::character varying, 'UNSUPPORTED'::character varying, 'ERROR'::character varying])::text[])))
);

ALTER TABLE ONLY public.document_jobs_v65 FORCE ROW LEVEL SECURITY;


--
-- Name: document_segments_v65; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.document_segments_v65 (
    tenant_id character varying(64) NOT NULL,
    document_id character varying(36) NOT NULL,
    ordinal integer NOT NULL,
    kind character varying(32) NOT NULL,
    locator jsonb NOT NULL,
    content text NOT NULL,
    extracted_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT document_segments_v65_content_check CHECK ((octet_length(content) <= 16000)),
    CONSTRAINT document_segments_v65_ordinal_check CHECK (((ordinal >= 1) AND (ordinal <= 10000)))
);

ALTER TABLE ONLY public.document_segments_v65 FORCE ROW LEVEL SECURITY;


--
-- Name: document_sources_v65; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.document_sources_v65 (
    tenant_id character varying(64) NOT NULL,
    id character varying(36) NOT NULL,
    filename character varying(255) NOT NULL,
    media_type character varying(100) NOT NULL,
    format character varying(24) NOT NULL,
    byte_length integer NOT NULL,
    sha256 character(64) NOT NULL,
    original bytea NOT NULL,
    evidence_state character varying(20) DEFAULT 'UNREVIEWED'::character varying NOT NULL,
    captured_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT document_sources_v65_byte_length_check CHECK (((byte_length >= 1) AND (byte_length <= 20971520))),
    CONSTRAINT document_sources_v65_evidence_state_check CHECK (((evidence_state)::text = 'UNREVIEWED'::text))
);

ALTER TABLE ONLY public.document_sources_v65 FORCE ROW LEVEL SECURITY;


--
-- Name: documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.documents (
    id character varying(128) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    filename character varying(255) NOT NULL,
    document_type character varying(64) DEFAULT 'WORKBOOK'::character varying NOT NULL,
    issuer character varying(128) DEFAULT 'CUSTOMER'::character varying NOT NULL,
    revision integer DEFAULT 1 NOT NULL,
    file_checksum character varying(128) NOT NULL,
    source_mode character varying(64) DEFAULT 'DETERMINISTIC'::character varying NOT NULL,
    status character varying(32) DEFAULT 'PROPOSED'::character varying NOT NULL,
    conflicts_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    supersedes_document_id character varying(128),
    imported_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT documents_status_check CHECK (((status)::text = ANY ((ARRAY['PROPOSED'::character varying, 'APPROVED'::character varying, 'SUPERSEDED'::character varying])::text[])))
);

ALTER TABLE ONLY public.documents FORCE ROW LEVEL SECURITY;


--
-- Name: dynamic_work_orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dynamic_work_orders (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) DEFAULT 'PLANT_DEFAULT'::character varying NOT NULL,
    po_number character varying(64) NOT NULL,
    style_id character varying(64) NOT NULL,
    line_id character varying(64) NOT NULL,
    supervisor_id character varying(64) NOT NULL,
    ordered_qty integer NOT NULL,
    color_matrix jsonb DEFAULT '{}'::jsonb NOT NULL,
    size_matrix jsonb DEFAULT '{}'::jsonb NOT NULL,
    packaging_bulletin jsonb DEFAULT '{}'::jsonb NOT NULL,
    shipping_spec jsonb DEFAULT '{}'::jsonb NOT NULL,
    aggregate_version bigint DEFAULT 0 NOT NULL,
    status character varying(32) NOT NULL,
    started_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT dynamic_work_orders_ordered_qty_check CHECK ((ordered_qty > 0)),
    CONSTRAINT dynamic_work_orders_status_check CHECK (((status)::text = ANY ((ARRAY['PROPOSED'::character varying, 'PLANNED'::character varying, 'LOCKED'::character varying, 'IN_PROGRESS'::character varying, 'COMPLETED'::character varying, 'HOLD'::character varying])::text[])))
);

ALTER TABLE ONLY public.dynamic_work_orders FORCE ROW LEVEL SECURITY;


--
-- Name: fabric_calibration_links_v32; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fabric_calibration_links_v32 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    observation_id uuid NOT NULL,
    instrument_id character varying(80) NOT NULL,
    calibration_revision integer NOT NULL,
    method_note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    inspector_subject character varying(128) NOT NULL,
    inspector_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'HUMAN_LINKED_CALIBRATION_RECORD_NOT_DEVICE_SIGNED_READING'::text NOT NULL,
    CONSTRAINT fabric_calibration_links_v32_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT fabric_calibration_links_v32_authority_check CHECK ((authority = 'HUMAN_LINKED_CALIBRATION_RECORD_NOT_DEVICE_SIGNED_READING'::text)),
    CONSTRAINT fabric_calibration_links_v32_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT fabric_calibration_links_v32_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT fabric_calibration_links_v32_method_note_check CHECK (((length(btrim(method_note)) >= 20) AND (length(btrim(method_note)) <= 800)))
);

ALTER TABLE ONLY public.fabric_calibration_links_v32 FORCE ROW LEVEL SECURITY;


--
-- Name: fabric_shrinkage_observations_v30; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fabric_shrinkage_observations_v30 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    material_sku character varying(128) NOT NULL,
    sample_id character varying(80) NOT NULL,
    treatment_code character varying(32) NOT NULL,
    warp_before_mm integer NOT NULL,
    warp_after_mm integer NOT NULL,
    weft_before_mm integer NOT NULL,
    weft_after_mm integer NOT NULL,
    warp_change_bp integer GENERATED ALWAYS AS ((round(((((warp_before_mm - warp_after_mm))::numeric * (10000)::numeric) / (warp_before_mm)::numeric)))::integer) STORED,
    weft_change_bp integer GENERATED ALWAYS AS ((round(((((weft_before_mm - weft_after_mm))::numeric * (10000)::numeric) / (weft_before_mm)::numeric)))::integer) STORED,
    source_document_id character varying(36) NOT NULL,
    source_sha256 character(64) NOT NULL,
    method_note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    tester_subject character varying(128) NOT NULL,
    tester_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_HUMAN_REPORTED_SHRINKAGE_NOT_CALIBRATED_OR_BUYER_APPROVED'::text NOT NULL,
    CONSTRAINT fabric_shrinkage_observations_v30_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT fabric_shrinkage_observations_v30_authority_check CHECK ((authority = 'SIGNED_HUMAN_REPORTED_SHRINKAGE_NOT_CALIBRATED_OR_BUYER_APPROVED'::text)),
    CONSTRAINT fabric_shrinkage_observations_v30_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT fabric_shrinkage_observations_v30_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT fabric_shrinkage_observations_v30_method_note_check CHECK (((length(btrim(method_note)) >= 20) AND (length(btrim(method_note)) <= 800))),
    CONSTRAINT fabric_shrinkage_observations_v30_sample_id_check CHECK (((sample_id)::text ~ '^[A-Za-z0-9_.-]{4,80}$'::text)),
    CONSTRAINT fabric_shrinkage_observations_v30_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT fabric_shrinkage_observations_v30_tester_proof_id_check CHECK ((length(btrim((tester_proof_id)::text)) > 0)),
    CONSTRAINT fabric_shrinkage_observations_v30_tester_subject_check CHECK ((length(btrim((tester_subject)::text)) > 0)),
    CONSTRAINT fabric_shrinkage_observations_v30_treatment_code_check CHECK (((treatment_code)::text = ANY ((ARRAY['WASH'::character varying, 'STEAM'::character varying, 'RELAXATION'::character varying, 'OTHER'::character varying])::text[]))),
    CONSTRAINT fabric_shrinkage_observations_v30_warp_after_mm_check CHECK (((warp_after_mm >= 25) AND (warp_after_mm <= 15000))),
    CONSTRAINT fabric_shrinkage_observations_v30_warp_before_mm_check CHECK (((warp_before_mm >= 50) AND (warp_before_mm <= 10000))),
    CONSTRAINT fabric_shrinkage_observations_v30_weft_after_mm_check CHECK (((weft_after_mm >= 25) AND (weft_after_mm <= 15000))),
    CONSTRAINT fabric_shrinkage_observations_v30_weft_before_mm_check CHECK (((weft_before_mm >= 50) AND (weft_before_mm <= 10000)))
);

ALTER TABLE ONLY public.fabric_shrinkage_observations_v30 FORCE ROW LEVEL SECURITY;


--
-- Name: fabric_shrinkage_specs_v31; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fabric_shrinkage_specs_v31 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    material_sku character varying(128) NOT NULL,
    treatment_code character varying(32) NOT NULL,
    revision integer NOT NULL,
    warp_min_bp integer NOT NULL,
    warp_max_bp integer NOT NULL,
    weft_min_bp integer NOT NULL,
    weft_max_bp integer NOT NULL,
    minimum_samples integer NOT NULL,
    source_document_id character varying(128) NOT NULL,
    source_sha256 character(64) NOT NULL,
    provisioned_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    CONSTRAINT fabric_shrinkage_specs_v31_check CHECK (((warp_min_bp <= warp_max_bp) AND (weft_min_bp <= weft_max_bp))),
    CONSTRAINT fabric_shrinkage_specs_v31_minimum_samples_check CHECK (((minimum_samples >= 1) AND (minimum_samples <= 32))),
    CONSTRAINT fabric_shrinkage_specs_v31_provisioned_by_check CHECK ((length(btrim((provisioned_by)::text)) > 0)),
    CONSTRAINT fabric_shrinkage_specs_v31_revision_check CHECK (((revision >= 1) AND (revision <= 100000))),
    CONSTRAINT fabric_shrinkage_specs_v31_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT fabric_shrinkage_specs_v31_treatment_code_check CHECK (((treatment_code)::text = ANY ((ARRAY['WASH'::character varying, 'STEAM'::character varying, 'RELAXATION'::character varying, 'OTHER'::character varying])::text[]))),
    CONSTRAINT fabric_shrinkage_specs_v31_warp_max_bp_check CHECK (((warp_max_bp >= '-3000'::integer) AND (warp_max_bp <= 3000))),
    CONSTRAINT fabric_shrinkage_specs_v31_warp_min_bp_check CHECK (((warp_min_bp >= '-3000'::integer) AND (warp_min_bp <= 3000))),
    CONSTRAINT fabric_shrinkage_specs_v31_weft_max_bp_check CHECK (((weft_max_bp >= '-3000'::integer) AND (weft_max_bp <= 3000))),
    CONSTRAINT fabric_shrinkage_specs_v31_weft_min_bp_check CHECK (((weft_min_bp >= '-3000'::integer) AND (weft_min_bp <= 3000)))
);

ALTER TABLE ONLY public.fabric_shrinkage_specs_v31 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_approved_reference_versions_v65b5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_approved_reference_versions_v65b5 (
    tenant_id character varying(64) NOT NULL,
    entity_type character varying(16) NOT NULL,
    entity_key character varying(256) NOT NULL,
    revision integer NOT NULL,
    candidate_id character varying(35) NOT NULL,
    document_id character varying(36) NOT NULL,
    review_revision integer NOT NULL,
    attributes jsonb NOT NULL,
    approved_by character varying(64) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_approved_reference_versions_v65b5_attributes_check CHECK ((jsonb_typeof(attributes) = 'object'::text)),
    CONSTRAINT factory_approved_reference_versions_v65b5_entity_type_check CHECK (((entity_type)::text = ANY ((ARRAY['MACHINE'::character varying, 'SKILL'::character varying, 'MATERIAL'::character varying, 'RATE'::character varying, 'FIELD'::character varying])::text[]))),
    CONSTRAINT factory_approved_reference_versions_v65b5_revision_check CHECK ((revision > 0))
);

ALTER TABLE ONLY public.factory_approved_reference_versions_v65b5 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_candidate_review_versions_v65b5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_candidate_review_versions_v65b5 (
    tenant_id character varying(64) NOT NULL,
    candidate_id character varying(35) NOT NULL,
    revision integer NOT NULL,
    action character varying(8) NOT NULL,
    attributes jsonb NOT NULL,
    checked_fields jsonb NOT NULL,
    reviewer character varying(64) NOT NULL,
    reviewed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_candidate_review_versions_v65b5_action_check CHECK (((action)::text = ANY ((ARRAY['KEEP'::character varying, 'SKIP'::character varying])::text[]))),
    CONSTRAINT factory_candidate_review_versions_v65b5_attributes_check CHECK ((jsonb_typeof(attributes) = 'object'::text)),
    CONSTRAINT factory_candidate_review_versions_v65b5_checked_fields_check CHECK ((jsonb_typeof(checked_fields) = 'array'::text)),
    CONSTRAINT factory_candidate_review_versions_v65b5_revision_check CHECK ((revision > 0))
);

ALTER TABLE ONLY public.factory_candidate_review_versions_v65b5 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_candidate_reviews_v65b2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_candidate_reviews_v65b2 (
    tenant_id character varying(64) NOT NULL,
    candidate_id character varying(35) NOT NULL,
    decision character varying(24) NOT NULL,
    review_note text DEFAULT ''::text NOT NULL,
    reviewed_by character varying(64) NOT NULL,
    reviewed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_candidate_reviews_v65b2_decision_check CHECK (((decision)::text = ANY ((ARRAY['ACCEPT_PROVISIONAL'::character varying, 'REJECT'::character varying])::text[]))),
    CONSTRAINT factory_candidate_reviews_v65b2_review_note_check CHECK ((char_length(review_note) <= 1000))
);

ALTER TABLE ONLY public.factory_candidate_reviews_v65b2 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_document_candidates_v65b2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_document_candidates_v65b2 (
    tenant_id character varying(64) NOT NULL,
    candidate_id character varying(35) NOT NULL,
    document_id character varying(36) NOT NULL,
    source_ordinal integer NOT NULL,
    source_locator jsonb NOT NULL,
    entity_type character varying(16) NOT NULL,
    entity_key character varying(256) NOT NULL,
    field_key character varying(80) DEFAULT ''::character varying NOT NULL,
    proposed_value text NOT NULL,
    attributes jsonb NOT NULL,
    explanation text NOT NULL,
    fingerprint character(64) NOT NULL,
    captured_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_document_candidates_v65b2_attributes_check CHECK ((jsonb_typeof(attributes) = 'object'::text)),
    CONSTRAINT factory_document_candidates_v65b2_candidate_id_check CHECK (((candidate_id)::text ~ '^FC-[0-9a-f]{32}$'::text)),
    CONSTRAINT factory_document_candidates_v65b2_entity_type_check CHECK (((entity_type)::text = ANY ((ARRAY['MACHINE'::character varying, 'SKILL'::character varying, 'MATERIAL'::character varying, 'RATE'::character varying, 'FIELD'::character varying])::text[]))),
    CONSTRAINT factory_document_candidates_v65b2_explanation_check CHECK (((char_length(explanation) >= 1) AND (char_length(explanation) <= 500))),
    CONSTRAINT factory_document_candidates_v65b2_proposed_value_check CHECK (((char_length(proposed_value) >= 1) AND (char_length(proposed_value) <= 1000))),
    CONSTRAINT factory_document_candidates_v65b2_source_locator_check CHECK ((jsonb_typeof(source_locator) = 'object'::text))
);

ALTER TABLE ONLY public.factory_document_candidates_v65b2 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_document_submissions_v65b5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_document_submissions_v65b5 (
    tenant_id character varying(64) NOT NULL,
    document_id character varying(36) NOT NULL,
    submission_id character varying(35) NOT NULL,
    approved_count integer NOT NULL,
    skipped_count integer NOT NULL,
    reviewer character varying(64) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_document_submissions_v65b5_approved_count_check CHECK ((approved_count >= 0)),
    CONSTRAINT factory_document_submissions_v65b5_skipped_count_check CHECK ((skipped_count >= 0))
);

ALTER TABLE ONLY public.factory_document_submissions_v65b5 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_draft_decisions_v65b3; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_draft_decisions_v65b3 (
    tenant_id character varying(64) NOT NULL,
    entity_type character varying(16) NOT NULL,
    entity_key character varying(256) NOT NULL,
    revision integer NOT NULL,
    candidate_id character varying(35) NOT NULL,
    decision character varying(24) NOT NULL,
    review_note text NOT NULL,
    reviewed_by character varying(64) NOT NULL,
    reviewed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_draft_decisions_v65b3_decision_check CHECK (((decision)::text = ANY ((ARRAY['VERIFY_DRAFT'::character varying, 'NEEDS_CLARIFICATION'::character varying])::text[]))),
    CONSTRAINT factory_draft_decisions_v65b3_entity_type_check CHECK (((entity_type)::text = ANY ((ARRAY['MACHINE'::character varying, 'SKILL'::character varying, 'MATERIAL'::character varying, 'RATE'::character varying])::text[]))),
    CONSTRAINT factory_draft_decisions_v65b3_review_note_check CHECK ((char_length(review_note) <= 1000)),
    CONSTRAINT factory_draft_decisions_v65b3_revision_check CHECK ((revision > 0))
);

ALTER TABLE ONLY public.factory_draft_decisions_v65b3 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_fact_events_v65b; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_fact_events_v65b (
    tenant_id character varying(64) NOT NULL,
    field_key character varying(80) NOT NULL,
    revision integer NOT NULL,
    value_kind character varying(16) NOT NULL,
    value_text text NOT NULL,
    normalized_value jsonb NOT NULL,
    review_state character varying(16) NOT NULL,
    source_document_id character varying(36),
    review_note text DEFAULT ''::text NOT NULL,
    recorded_by character varying(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_fact_events_v65b_normalized_value_check CHECK ((jsonb_typeof(normalized_value) = 'object'::text)),
    CONSTRAINT factory_fact_events_v65b_review_note_check CHECK ((char_length(review_note) <= 1000)),
    CONSTRAINT factory_fact_events_v65b_review_state_check CHECK (((review_state)::text = ANY ((ARRAY['PROVISIONAL'::character varying, 'CONFIRMED'::character varying])::text[]))),
    CONSTRAINT factory_fact_events_v65b_revision_check CHECK (((revision >= 1) AND (revision <= 2147483647))),
    CONSTRAINT factory_fact_events_v65b_value_kind_check CHECK (((value_kind)::text = ANY ((ARRAY['TEXT'::character varying, 'INTEGER'::character varying, 'DECIMAL'::character varying, 'CURRENCY'::character varying, 'CHOICE'::character varying])::text[]))),
    CONSTRAINT factory_fact_events_v65b_value_text_check CHECK (((char_length(value_text) >= 1) AND (char_length(value_text) <= 2000)))
);

ALTER TABLE ONLY public.factory_fact_events_v65b FORCE ROW LEVEL SECURITY;


--
-- Name: factory_instrument_calibrations_v32; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_instrument_calibrations_v32 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    instrument_id character varying(80) NOT NULL,
    revision integer NOT NULL,
    instrument_kind character varying(32) NOT NULL,
    valid_from timestamp with time zone NOT NULL,
    valid_until timestamp with time zone NOT NULL,
    source_document_id character varying(36) NOT NULL,
    source_sha256 character(64) NOT NULL,
    registered_by character varying(128) NOT NULL,
    registered_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    CONSTRAINT factory_instrument_calibrations_v32_check CHECK (((valid_until > valid_from) AND (valid_until <= (valid_from + '5 years'::interval)))),
    CONSTRAINT factory_instrument_calibrations_v32_instrument_id_check CHECK (((instrument_id)::text ~ '^[A-Za-z0-9_.-]{4,80}$'::text)),
    CONSTRAINT factory_instrument_calibrations_v32_instrument_kind_check CHECK (((instrument_kind)::text = ANY ((ARRAY['DIMENSION'::character varying, 'SCALE'::character varying, 'METAL_DETECTOR'::character varying])::text[]))),
    CONSTRAINT factory_instrument_calibrations_v32_registered_by_check CHECK ((length(btrim((registered_by)::text)) > 0)),
    CONSTRAINT factory_instrument_calibrations_v32_revision_check CHECK (((revision >= 1) AND (revision <= 100000))),
    CONSTRAINT factory_instrument_calibrations_v32_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.factory_instrument_calibrations_v32 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_proposed_register_v65b2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_proposed_register_v65b2 (
    tenant_id character varying(64) NOT NULL,
    candidate_id character varying(35) NOT NULL,
    entity_type character varying(16) NOT NULL,
    entity_key character varying(256) NOT NULL,
    attributes jsonb NOT NULL,
    review_state character varying(16) DEFAULT 'PROVISIONAL'::character varying NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_proposed_register_v65b2_entity_type_check CHECK (((entity_type)::text = ANY ((ARRAY['MACHINE'::character varying, 'SKILL'::character varying, 'MATERIAL'::character varying, 'RATE'::character varying])::text[]))),
    CONSTRAINT factory_proposed_register_v65b2_review_state_check CHECK (((review_state)::text = 'PROVISIONAL'::text))
);

ALTER TABLE ONLY public.factory_proposed_register_v65b2 FORCE ROW LEVEL SECURITY;


--
-- Name: factory_reference_versions_v65b4; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.factory_reference_versions_v65b4 (
    tenant_id character varying(64) NOT NULL,
    entity_type character varying(16) NOT NULL,
    entity_key character varying(256) NOT NULL,
    revision integer NOT NULL,
    action character varying(12) NOT NULL,
    candidate_id character varying(35) NOT NULL,
    draft_revision integer NOT NULL,
    attributes jsonb NOT NULL,
    rate_basis character varying(12),
    reviewer_note text NOT NULL,
    reviewed_by character varying(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT factory_reference_versions_v65b4_action_check CHECK (((action)::text = ANY ((ARRAY['PUBLISH'::character varying, 'WITHDRAW'::character varying])::text[]))),
    CONSTRAINT factory_reference_versions_v65b4_attributes_check CHECK ((jsonb_typeof(attributes) = 'object'::text)),
    CONSTRAINT factory_reference_versions_v65b4_draft_revision_check CHECK ((draft_revision > 0)),
    CONSTRAINT factory_reference_versions_v65b4_entity_key_check CHECK (((char_length(TRIM(BOTH FROM entity_key)) >= 1) AND (char_length(TRIM(BOTH FROM entity_key)) <= 256))),
    CONSTRAINT factory_reference_versions_v65b4_entity_type_check CHECK (((entity_type)::text = ANY ((ARRAY['MACHINE'::character varying, 'SKILL'::character varying, 'RATE'::character varying])::text[]))),
    CONSTRAINT factory_reference_versions_v65b4_rate_basis_check CHECK (((rate_basis IS NULL) OR ((rate_basis)::text = ANY ((ARRAY['HOUR'::character varying, 'PIECE'::character varying, 'METER'::character varying, 'YARD'::character varying, 'KG'::character varying, 'DAY'::character varying, 'UNIT'::character varying])::text[])))),
    CONSTRAINT factory_reference_versions_v65b4_reviewer_note_check CHECK (((char_length(TRIM(BOTH FROM reviewer_note)) >= 12) AND (char_length(TRIM(BOTH FROM reviewer_note)) <= 1000))),
    CONSTRAINT factory_reference_versions_v65b4_revision_check CHECK ((revision > 0)),
    CONSTRAINT rate_basis_required CHECK (((((entity_type)::text = 'RATE'::text) AND (rate_basis IS NOT NULL)) OR (((entity_type)::text <> 'RATE'::text) AND (rate_basis IS NULL))))
);

ALTER TABLE ONLY public.factory_reference_versions_v65b4 FORCE ROW LEVEL SECURITY;


--
-- Name: floor_issues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.floor_issues (
    id character varying(128) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    issue_type character varying(64) NOT NULL,
    severity character varying(32) NOT NULL,
    plant_id character varying(128) DEFAULT 'PLANT_DEFAULT'::character varying NOT NULL,
    line_id character varying(64) NOT NULL,
    station_id character varying(64),
    machine_id character varying(64),
    operator_id character varying(64),
    title text NOT NULL,
    started_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    resolved_at timestamp with time zone,
    owner_id character varying(64) DEFAULT 'SUPERVISOR'::character varying NOT NULL,
    status character varying(32) DEFAULT 'open'::character varying NOT NULL,
    CONSTRAINT floor_issues_severity_check CHECK (((severity)::text = ANY ((ARRAY['info'::character varying, 'attention'::character varying, 'critical'::character varying])::text[]))),
    CONSTRAINT floor_issues_status_check CHECK (((status)::text = ANY ((ARRAY['open'::character varying, 'in_progress'::character varying, 'resolved'::character varying])::text[])))
);

ALTER TABLE ONLY public.floor_issues FORCE ROW LEVEL SECURITY;


--
-- Name: gen_bundles_v18; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_bundles_v18 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    release_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    label_code character varying(29) NOT NULL,
    state text DEFAULT 'RELEASED_CUT_BUNDLE_REFERENCE_NOT_UNIT_ACCEPTANCE'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT gen_bundles_v18_state_check CHECK ((state = 'RELEASED_CUT_BUNDLE_REFERENCE_NOT_UNIT_ACCEPTANCE'::text))
);

ALTER TABLE ONLY public.gen_bundles_v18 FORCE ROW LEVEL SECURITY;


--
-- Name: gen_carrier_active_v18; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_carrier_active_v18 (
    tenant_id character varying(64) NOT NULL,
    carrier_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    target_type character varying(12) NOT NULL,
    target_id uuid NOT NULL,
    bound_event_id uuid NOT NULL,
    bound_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY public.gen_carrier_active_v18 FORCE ROW LEVEL SECURITY;


--
-- Name: gen_carrier_events_v18; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_carrier_events_v18 (
    event_no bigint NOT NULL,
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    carrier_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    action character varying(12) NOT NULL,
    target_type character varying(12) NOT NULL,
    target_id uuid NOT NULL,
    worker_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    evidence text DEFAULT 'SIGNED_HUMAN_CARRIER_CLAIM_NOT_RF_PROOF'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT gen_carrier_events_v18_action_check CHECK (((action)::text = ANY ((ARRAY['BIND'::character varying, 'RETIRE'::character varying])::text[]))),
    CONSTRAINT gen_carrier_events_v18_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT gen_carrier_events_v18_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT gen_carrier_events_v18_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT gen_carrier_events_v18_evidence_check CHECK ((evidence = 'SIGNED_HUMAN_CARRIER_CLAIM_NOT_RF_PROOF'::text)),
    CONSTRAINT gen_carrier_events_v18_target_type_check CHECK (((target_type)::text = ANY ((ARRAY['GARMENT'::character varying, 'CUT_PART'::character varying, 'BUNDLE'::character varying])::text[])))
);

ALTER TABLE ONLY public.gen_carrier_events_v18 FORCE ROW LEVEL SECURITY;


--
-- Name: gen_carrier_events_v18_event_no_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.gen_carrier_events_v18 ALTER COLUMN event_no ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.gen_carrier_events_v18_event_no_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: gen_carriers_v18; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_carriers_v18 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    epc_hex character varying(96) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    enrolled_by character varying(128) NOT NULL,
    enrolled_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT gen_carriers_v18_epc_hex_check CHECK ((((epc_hex)::text ~ '^[0-9A-F]{24,96}$'::text) AND ((length((epc_hex)::text) % 2) = 0)))
);

ALTER TABLE ONLY public.gen_carriers_v18 FORCE ROW LEVEL SECURITY;


--
-- Name: gen_cut_parts_v18; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_cut_parts_v18 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    garment_id uuid NOT NULL,
    bundle_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    label_code character varying(29) NOT NULL,
    part_code character varying(64) NOT NULL,
    part_slot integer NOT NULL,
    state text DEFAULT 'CUT_PART_IDENTITY_REFERENCE_NOT_ACCEPTED_PIECE'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT gen_cut_parts_v18_part_slot_check CHECK ((part_slot > 0)),
    CONSTRAINT gen_cut_parts_v18_state_check CHECK ((state = 'CUT_PART_IDENTITY_REFERENCE_NOT_ACCEPTED_PIECE'::text))
);

ALTER TABLE ONLY public.gen_cut_parts_v18 FORCE ROW LEVEL SECURITY;


--
-- Name: gen_garments_v18; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_garments_v18 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    release_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    marker_position integer NOT NULL,
    ply_number integer NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    reserved_by character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    state text DEFAULT 'IDENTITY_RESERVED_NOT_PHYSICALLY_VERIFIED'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT gen_garments_v18_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT gen_garments_v18_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT gen_garments_v18_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT gen_garments_v18_marker_position_check CHECK ((marker_position > 0)),
    CONSTRAINT gen_garments_v18_ply_number_check CHECK ((ply_number > 0)),
    CONSTRAINT gen_garments_v18_state_check CHECK ((state = 'IDENTITY_RESERVED_NOT_PHYSICALLY_VERIFIED'::text))
);

ALTER TABLE ONLY public.gen_garments_v18 FORCE ROW LEVEL SECURITY;


--
-- Name: hangtag_binds; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.hangtag_binds (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    garment_serial integer NOT NULL,
    upc character varying(64) NOT NULL,
    epc_sgtin96 character varying(64) NOT NULL,
    bound_by_worker_id character varying(64),
    bound_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.hangtag_binds FORCE ROW LEVEL SECURITY;


--
-- Name: hourly_line_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.hourly_line_metrics (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    line_id character varying(64) NOT NULL,
    shift_date date DEFAULT CURRENT_DATE NOT NULL,
    hour_window integer NOT NULL,
    target_units integer DEFAULT 75 NOT NULL,
    actual_units integer DEFAULT 0 NOT NULL,
    defect_units integer DEFAULT 0 NOT NULL,
    andon_downtime_minutes integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT hourly_line_metrics_actual_units_check CHECK ((actual_units >= 0)),
    CONSTRAINT hourly_line_metrics_andon_downtime_minutes_check CHECK ((andon_downtime_minutes >= 0)),
    CONSTRAINT hourly_line_metrics_defect_units_check CHECK ((defect_units >= 0)),
    CONSTRAINT hourly_line_metrics_hour_window_check CHECK (((hour_window >= 1) AND (hour_window <= 24))),
    CONSTRAINT hourly_line_metrics_target_units_check CHECK ((target_units >= 0))
);

ALTER TABLE ONLY public.hourly_line_metrics FORCE ROW LEVEL SECURITY;


--
-- Name: intake_document_capture; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.intake_document_capture (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    filename text NOT NULL,
    content_sha256 character(64) NOT NULL,
    source_mode text NOT NULL,
    original_bytes bytea NOT NULL,
    extraction_json jsonb NOT NULL,
    status text DEFAULT 'PROPOSED'::text NOT NULL,
    captured_at timestamp with time zone DEFAULT now() NOT NULL,
    synced_at timestamp with time zone,
    CONSTRAINT intake_document_capture_status_check CHECK ((status = ANY (ARRAY['PROPOSED'::text, 'SYNCED'::text])))
);

ALTER TABLE ONLY public.intake_document_capture FORCE ROW LEVEL SECURITY;


--
-- Name: intake_proposed_fact; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.intake_proposed_fact (
    capture_id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    json_pointer text NOT NULL,
    value_json jsonb NOT NULL
);

ALTER TABLE ONLY public.intake_proposed_fact FORCE ROW LEVEL SECURITY;


--
-- Name: intel_sim_snapshots_v12; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.intel_sim_snapshots_v12 (
    tenant_id character varying(64) NOT NULL,
    capture_id character varying(80) NOT NULL,
    request_sha256 character(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    factory_timezone character varying(64) NOT NULL,
    report jsonb NOT NULL,
    report_sha256 character(64) NOT NULL,
    captured_at timestamp with time zone DEFAULT now() NOT NULL,
    provenance character varying(48) DEFAULT 'DERIVED_FROM_SIMULATED_STAGE_EVENTS'::character varying NOT NULL,
    CONSTRAINT intel_sim_snapshots_v12_capture_id_check CHECK (((capture_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT intel_sim_snapshots_v12_provenance_check CHECK (((provenance)::text = 'DERIVED_FROM_SIMULATED_STAGE_EVENTS'::text)),
    CONSTRAINT intel_sim_snapshots_v12_report_check CHECK (((jsonb_typeof(report) = 'object'::text) AND ((report ->> 'authority'::text) = 'REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE'::text))),
    CONSTRAINT intel_sim_snapshots_v12_report_sha256_check CHECK ((report_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT intel_sim_snapshots_v12_request_sha256_check CHECK ((request_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.intel_sim_snapshots_v12 FORCE ROW LEVEL SECURITY;


--
-- Name: intelligence_prod_readers_v13; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.intelligence_prod_readers_v13 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    worker_subject character varying(128) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY public.intelligence_prod_readers_v13 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_discrepancies_v29; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_discrepancies_v29 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    outward_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    part_id uuid NOT NULL,
    category character varying(32) NOT NULL,
    note text NOT NULL,
    evidence_ref character varying(180) NOT NULL,
    evidence_sha256 character(64) NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    reporter_subject character varying(128) NOT NULL,
    reporter_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    reported_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_HUMAN_DISCREPANCY_REPORT_NOT_VENDOR_CERTIFIED_OR_QC_RELEASED'::text NOT NULL,
    CONSTRAINT jobwork_discrepancies_v29_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_discrepancies_v29_authority_check CHECK ((authority = 'SIGNED_HUMAN_DISCREPANCY_REPORT_NOT_VENDOR_CERTIFIED_OR_QC_RELEASED'::text)),
    CONSTRAINT jobwork_discrepancies_v29_category_check CHECK (((category)::text = ANY ((ARRAY['MISSING_AT_RECONCILIATION'::character varying, 'DAMAGED_ON_RETURN'::character varying])::text[]))),
    CONSTRAINT jobwork_discrepancies_v29_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT jobwork_discrepancies_v29_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_discrepancies_v29_evidence_ref_check CHECK (((length(btrim((evidence_ref)::text)) >= 8) AND (length(btrim((evidence_ref)::text)) <= 180))),
    CONSTRAINT jobwork_discrepancies_v29_evidence_sha256_check CHECK ((evidence_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_discrepancies_v29_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 1000))),
    CONSTRAINT jobwork_discrepancies_v29_reporter_proof_id_check CHECK ((length(btrim((reporter_proof_id)::text)) > 0)),
    CONSTRAINT jobwork_discrepancies_v29_reporter_subject_check CHECK ((length(btrim((reporter_subject)::text)) > 0))
);

ALTER TABLE ONLY public.jobwork_discrepancies_v29 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_discrepancy_resolutions_v31; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_discrepancy_resolutions_v31 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    discrepancy_id uuid NOT NULL,
    outward_id uuid NOT NULL,
    part_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    resolution_kind character varying(40) NOT NULL,
    evidence_ref character varying(180) NOT NULL,
    evidence_sha256 character(64) NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    supervisor_subject character varying(128) NOT NULL,
    supervisor_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'INDEPENDENT_DISCREPANCY_DISPOSITION_NOT_QC_OR_SETTLEMENT'::text NOT NULL,
    CONSTRAINT jobwork_discrepancy_resolutions_v31_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_authority_check CHECK ((authority = 'INDEPENDENT_DISCREPANCY_DISPOSITION_NOT_QC_OR_SETTLEMENT'::text)),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_evidence_ref_check CHECK (((length(btrim((evidence_ref)::text)) >= 8) AND (length(btrim((evidence_ref)::text)) <= 180))),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_evidence_sha256_check CHECK ((evidence_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 1000))),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_resolution_kind_check CHECK (((resolution_kind)::text = ANY ((ARRAY['RETURNED_TO_PIECE_HOLD'::character varying, 'SHORTAGE_REPORTED_REMAINS_BLOCKED'::character varying, 'DAMAGE_REPORTED_REMAINS_BLOCKED'::character varying])::text[]))),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_supervisor_proof_id_check CHECK ((length(btrim((supervisor_proof_id)::text)) > 0)),
    CONSTRAINT jobwork_discrepancy_resolutions_v31_supervisor_subject_check CHECK ((length(btrim((supervisor_subject)::text)) > 0))
);

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_inwards_v28; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_inwards_v28 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    outward_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    part_id uuid NOT NULL,
    scan_id uuid NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    receiver_subject character varying(128) NOT NULL,
    receiver_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_INTERNAL_RETURN_PENDING_QC_NOT_VENDOR_CERTIFIED'::text NOT NULL,
    CONSTRAINT jobwork_inwards_v28_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_inwards_v28_authority_check CHECK ((authority = 'SIGNED_INTERNAL_RETURN_PENDING_QC_NOT_VENDOR_CERTIFIED'::text)),
    CONSTRAINT jobwork_inwards_v28_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT jobwork_inwards_v28_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_inwards_v28_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 500))),
    CONSTRAINT jobwork_inwards_v28_receiver_proof_id_check CHECK ((length(btrim((receiver_proof_id)::text)) > 0)),
    CONSTRAINT jobwork_inwards_v28_receiver_subject_check CHECK ((length(btrim((receiver_subject)::text)) > 0))
);

ALTER TABLE ONLY public.jobwork_inwards_v28 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_outwards_v28; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_outwards_v28 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    part_id uuid NOT NULL,
    scan_id uuid NOT NULL,
    vendor_id character varying(64) NOT NULL,
    job_reference character varying(64) NOT NULL,
    process_code character varying(32) NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    sender_subject character varying(128) NOT NULL,
    sender_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_INTERNAL_OUTWARD_NOT_VENDOR_RECEIPT'::text NOT NULL,
    CONSTRAINT jobwork_outwards_v28_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_outwards_v28_authority_check CHECK ((authority = 'SIGNED_INTERNAL_OUTWARD_NOT_VENDOR_RECEIPT'::text)),
    CONSTRAINT jobwork_outwards_v28_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT jobwork_outwards_v28_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_outwards_v28_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 500))),
    CONSTRAINT jobwork_outwards_v28_process_code_check CHECK (((process_code)::text = ANY ((ARRAY['EMBROIDERY'::character varying, 'PRINTING'::character varying, 'WASHING'::character varying, 'DYEING'::character varying, 'OTHER_FINISHING'::character varying])::text[]))),
    CONSTRAINT jobwork_outwards_v28_sender_proof_id_check CHECK ((length(btrim((sender_proof_id)::text)) > 0)),
    CONSTRAINT jobwork_outwards_v28_sender_subject_check CHECK ((length(btrim((sender_subject)::text)) > 0))
);

ALTER TABLE ONLY public.jobwork_outwards_v28 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_used_proofs_v28; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_used_proofs_v28 (
    tenant_id character varying(64) NOT NULL,
    proof_id character varying(255) NOT NULL,
    kind text NOT NULL,
    entry_id uuid NOT NULL,
    CONSTRAINT jobwork_used_proofs_v28_kind_check CHECK ((kind = ANY (ARRAY['OUTWARD'::text, 'INWARD'::text])))
);

ALTER TABLE ONLY public.jobwork_used_proofs_v28 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_used_scans_v28; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_used_scans_v28 (
    tenant_id character varying(64) NOT NULL,
    scan_id uuid NOT NULL,
    kind text NOT NULL,
    entry_id uuid NOT NULL,
    CONSTRAINT jobwork_used_scans_v28_kind_check CHECK ((kind = ANY (ARRAY['OUTWARD'::text, 'INWARD'::text])))
);

ALTER TABLE ONLY public.jobwork_used_scans_v28 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_vendor_acks_v30; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_vendor_acks_v30 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    outward_id uuid NOT NULL,
    part_id uuid NOT NULL,
    job_reference character varying(64) NOT NULL,
    process_code character varying(32) NOT NULL,
    vendor_id character varying(64) NOT NULL,
    key_id character varying(64) NOT NULL,
    ack_kind character varying(32) NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    vendor_signature_hex character(128) NOT NULL,
    key_sha256 character(64) NOT NULL,
    recorder_subject character varying(128) NOT NULL,
    recorder_proof_id character varying(255) NOT NULL,
    recorder_assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_VENDOR_ACKNOWLEDGEMENT_NOT_RF_PROOF_QC_OR_SETTLEMENT'::text NOT NULL,
    CONSTRAINT jobwork_vendor_acks_v30_ack_kind_check CHECK (((ack_kind)::text = 'OUTWARD_REPORTED_RECEIVED'::text)),
    CONSTRAINT jobwork_vendor_acks_v30_authority_check CHECK ((authority = 'SIGNED_VENDOR_ACKNOWLEDGEMENT_NOT_RF_PROOF_QC_OR_SETTLEMENT'::text)),
    CONSTRAINT jobwork_vendor_acks_v30_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT jobwork_vendor_acks_v30_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_vendor_acks_v30_key_sha256_check CHECK ((key_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_vendor_acks_v30_process_code_check CHECK (((process_code)::text = ANY ((ARRAY['EMBROIDERY'::character varying, 'PRINTING'::character varying, 'WASHING'::character varying, 'DYEING'::character varying, 'OTHER_FINISHING'::character varying])::text[]))),
    CONSTRAINT jobwork_vendor_acks_v30_recorder_assertion_sha256_check CHECK ((recorder_assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_vendor_acks_v30_recorder_proof_id_check CHECK ((length(btrim((recorder_proof_id)::text)) > 0)),
    CONSTRAINT jobwork_vendor_acks_v30_recorder_subject_check CHECK ((length(btrim((recorder_subject)::text)) > 0)),
    CONSTRAINT jobwork_vendor_acks_v30_vendor_signature_hex_check CHECK ((vendor_signature_hex ~ '^[0-9a-f]{128}$'::text))
);

ALTER TABLE ONLY public.jobwork_vendor_acks_v30 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_vendor_keys_v30; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_vendor_keys_v30 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    vendor_id character varying(64) NOT NULL,
    key_id character varying(64) NOT NULL,
    public_key_hex character(64) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    registered_by character varying(128) NOT NULL,
    registered_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT jobwork_vendor_keys_v30_key_id_check CHECK (((key_id)::text ~ '^[A-Za-z0-9_-]{4,64}$'::text)),
    CONSTRAINT jobwork_vendor_keys_v30_public_key_hex_check CHECK ((public_key_hex ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT jobwork_vendor_keys_v30_registered_by_check CHECK ((length(btrim((registered_by)::text)) > 0))
);

ALTER TABLE ONLY public.jobwork_vendor_keys_v30 FORCE ROW LEVEL SECURITY;


--
-- Name: jobwork_vendors_v28; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobwork_vendors_v28 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    vendor_id character varying(64) NOT NULL,
    display_name character varying(180) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    allowed_processes text[] DEFAULT '{}'::text[] NOT NULL,
    approved_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT jobwork_vendors_v28_allowed_processes_check CHECK ((array_position(allowed_processes, NULL::text) IS NULL)),
    CONSTRAINT jobwork_vendors_v28_display_name_check CHECK (((length(btrim((display_name)::text)) >= 3) AND (length(btrim((display_name)::text)) <= 180)))
);

ALTER TABLE ONLY public.jobwork_vendors_v28 FORCE ROW LEVEL SECURITY;


--
-- Name: lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.lines (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    display_name character varying(255) NOT NULL,
    stations_count integer DEFAULT 14 NOT NULL,
    target_daily_capacity integer DEFAULT 600 NOT NULL,
    dwell_floor_pct numeric(4,2) DEFAULT 0.40 NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT lines_dwell_floor_pct_check CHECK (((dwell_floor_pct > (0)::numeric) AND (dwell_floor_pct <= 1.00))),
    CONSTRAINT lines_stations_count_check CHECK ((stations_count >= 0)),
    CONSTRAINT lines_target_daily_capacity_check CHECK ((target_daily_capacity >= 0))
);

ALTER TABLE ONLY public.lines FORCE ROW LEVEL SECURITY;


--
-- Name: machine_roster; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.machine_roster (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    line_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    canonical_class character varying(64) NOT NULL,
    rated_capacity_uph integer DEFAULT 100 NOT NULL,
    status character varying(32) DEFAULT 'ACTIVE'::character varying NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT machine_roster_rated_capacity_uph_check CHECK ((rated_capacity_uph > 0)),
    CONSTRAINT machine_roster_status_check CHECK (((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'IDLE'::character varying, 'BREAKDOWN'::character varying, 'MAINTENANCE'::character varying])::text[])))
);

ALTER TABLE ONLY public.machine_roster FORCE ROW LEVEL SECURITY;


--
-- Name: material_ceo_principals_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_ceo_principals_v5 (
    tenant_id character varying(64) NOT NULL,
    subject text NOT NULL,
    enrolled_by text NOT NULL,
    enrolled_at timestamp with time zone DEFAULT now() NOT NULL,
    active boolean DEFAULT false NOT NULL
);

ALTER TABLE ONLY public.material_ceo_principals_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: material_issues_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_issues_v5 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    reservation_id uuid NOT NULL,
    roll_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    cut_no integer NOT NULL,
    length_mm bigint NOT NULL,
    command_key character varying(128) NOT NULL,
    issuer_subject character varying(128) NOT NULL,
    recipient_subject character varying(128) NOT NULL,
    issuer_proof_id character varying(255) NOT NULL,
    recipient_proof_id character varying(255) NOT NULL,
    issuer_assertion_hash character(64) NOT NULL,
    recipient_assertion_hash character(64) NOT NULL,
    signed_challenge_hash character(64) NOT NULL,
    committed_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT material_issues_v5_check CHECK (((issuer_subject)::text <> (recipient_subject)::text)),
    CONSTRAINT material_issues_v5_check1 CHECK (((issuer_proof_id)::text <> (recipient_proof_id)::text)),
    CONSTRAINT material_issues_v5_cut_no_check CHECK ((cut_no > 0)),
    CONSTRAINT material_issues_v5_length_mm_check CHECK ((length_mm > 0))
);

ALTER TABLE ONLY public.material_issues_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: material_outbox_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_outbox_v5 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    aggregate_kind character varying(64) NOT NULL,
    aggregate_id character varying(128) NOT NULL,
    event_kind character varying(64) NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    delivered_at timestamp with time zone
);

ALTER TABLE ONLY public.material_outbox_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: material_reservations_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_reservations_v5 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    cut_no integer NOT NULL,
    reserved_mm bigint NOT NULL,
    issued_mm bigint DEFAULT 0 NOT NULL,
    released_mm bigint DEFAULT 0 NOT NULL,
    request_key character varying(128) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT material_reservations_v5_check CHECK (((issued_mm + released_mm) <= reserved_mm)),
    CONSTRAINT material_reservations_v5_cut_no_check CHECK ((cut_no > 0)),
    CONSTRAINT material_reservations_v5_issued_mm_check CHECK ((issued_mm >= 0)),
    CONSTRAINT material_reservations_v5_released_mm_check CHECK ((released_mm >= 0)),
    CONSTRAINT material_reservations_v5_reserved_mm_check CHECK ((reserved_mm > 0))
);

ALTER TABLE ONLY public.material_reservations_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: material_scenarios_v6; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_scenarios_v6 (
    id character varying(40) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    intake_id character varying(40) NOT NULL,
    source_sha256 character(64) NOT NULL,
    scenario_sha256 character(64) NOT NULL,
    stock_filename character varying(255) NOT NULL,
    prices_filename character varying(255) NOT NULL,
    stock_original bytea NOT NULL,
    prices_original bytea NOT NULL,
    report_json jsonb NOT NULL,
    status character varying(20) DEFAULT 'PROPOSED'::character varying NOT NULL,
    release_gate character varying(10) DEFAULT 'HOLD'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT material_scenarios_v6_release_gate_check CHECK (((release_gate)::text = 'HOLD'::text)),
    CONSTRAINT material_scenarios_v6_status_check CHECK (((status)::text = 'PROPOSED'::text))
);

ALTER TABLE ONLY public.material_scenarios_v6 FORCE ROW LEVEL SECURITY;


--
-- Name: material_used_proofs_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_used_proofs_v5 (
    tenant_id character varying(64) NOT NULL,
    proof_id character varying(255) NOT NULL,
    issue_id uuid NOT NULL,
    witness_role text NOT NULL,
    recorded_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT material_used_proofs_v5_witness_role_check CHECK ((witness_role = ANY (ARRAY['ISSUER'::text, 'RECIPIENT'::text])))
);

ALTER TABLE ONLY public.material_used_proofs_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: material_variances_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_variances_v5 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    reservation_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    cut_no integer NOT NULL,
    requested_additional_mm bigint NOT NULL,
    actual_story text NOT NULL,
    reason_code text NOT NULL,
    requested_by text NOT NULL,
    request_key character varying(128) NOT NULL,
    challenge_hash character(64) DEFAULT repeat('0'::text, 64) NOT NULL,
    status text DEFAULT 'PENDING_CEO'::text NOT NULL,
    ceo_subject text,
    ceo_assertion_hash character(64),
    ceo_proof_id text,
    approved_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT material_variances_v5_actual_story_check CHECK ((length(TRIM(BOTH FROM actual_story)) >= 20)),
    CONSTRAINT material_variances_v5_check CHECK ((((status = 'APPROVED'::text) AND (ceo_subject IS NOT NULL) AND (ceo_assertion_hash IS NOT NULL) AND (ceo_proof_id IS NOT NULL) AND (approved_at IS NOT NULL)) OR (status <> 'APPROVED'::text))),
    CONSTRAINT material_variances_v5_cut_no_check CHECK ((cut_no > 0)),
    CONSTRAINT material_variances_v5_requested_additional_mm_check CHECK ((requested_additional_mm > 0)),
    CONSTRAINT material_variances_v5_status_check CHECK ((status = ANY (ARRAY['PENDING_CEO'::text, 'APPROVED'::text, 'REJECTED'::text])))
);

ALTER TABLE ONLY public.material_variances_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: material_write_scopes_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_write_scopes_v5 (
    tenant_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    mode text NOT NULL,
    lease_holder text NOT NULL,
    lease_revision bigint NOT NULL,
    legacy_writer_fenced boolean DEFAULT false NOT NULL,
    approved_by text NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL,
    lease_expires_at timestamp with time zone NOT NULL,
    CONSTRAINT material_write_scopes_v5_lease_revision_check CHECK ((lease_revision > 0)),
    CONSTRAINT material_write_scopes_v5_mode_check CHECK ((mode = ANY (ARRAY['M0'::text, 'M1'::text, 'M2'::text, 'M3'::text, 'M4'::text])))
);

ALTER TABLE ONLY public.material_write_scopes_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: needle_safety_clearances_v33; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.needle_safety_clearances_v33 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    incident_id uuid NOT NULL,
    garment_id uuid NOT NULL,
    followup_pass_id uuid NOT NULL,
    accounted_fragments integer NOT NULL,
    evidence_document_id character varying(36) NOT NULL,
    evidence_sha256 character(64) NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    reviewer_subject character varying(128) NOT NULL,
    reviewer_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SUPERVISED_SAFETY_REVIEW_NOT_MACHINE_CERTIFICATION_OR_QC_RELEASE'::text NOT NULL,
    CONSTRAINT needle_safety_clearances_v33_accounted_fragments_check CHECK (((accounted_fragments >= 0) AND (accounted_fragments <= 20))),
    CONSTRAINT needle_safety_clearances_v33_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT needle_safety_clearances_v33_authority_check CHECK ((authority = 'SUPERVISED_SAFETY_REVIEW_NOT_MACHINE_CERTIFICATION_OR_QC_RELEASE'::text)),
    CONSTRAINT needle_safety_clearances_v33_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT needle_safety_clearances_v33_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT needle_safety_clearances_v33_evidence_sha256_check CHECK ((evidence_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT needle_safety_clearances_v33_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 800)))
);

ALTER TABLE ONLY public.needle_safety_clearances_v33 FORCE ROW LEVEL SECURITY;


--
-- Name: needle_safety_events_v32; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.needle_safety_events_v32 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    garment_id uuid NOT NULL,
    instrument_id character varying(80),
    event_kind character varying(32) NOT NULL,
    expected_fragments integer NOT NULL,
    recovered_fragments integer NOT NULL,
    evidence_document_id character varying(36) NOT NULL,
    evidence_sha256 character(64) NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_HUMAN_SAFETY_REPORT_NOT_MACHINE_ATTESTATION_OR_RELEASE'::text NOT NULL,
    CONSTRAINT needle_safety_events_v32_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT needle_safety_events_v32_authority_check CHECK ((authority = 'SIGNED_HUMAN_SAFETY_REPORT_NOT_MACHINE_ATTESTATION_OR_RELEASE'::text)),
    CONSTRAINT needle_safety_events_v32_check CHECK ((recovered_fragments <= expected_fragments)),
    CONSTRAINT needle_safety_events_v32_check1 CHECK ((((event_kind)::text <> 'NEEDLE_BREAK'::text) OR (expected_fragments > 0))),
    CONSTRAINT needle_safety_events_v32_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT needle_safety_events_v32_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT needle_safety_events_v32_event_kind_check CHECK (((event_kind)::text = ANY ((ARRAY['NEEDLE_BREAK'::character varying, 'DETECTOR_CHALLENGE_PASS'::character varying, 'DETECTOR_CHALLENGE_FAIL'::character varying, 'DETECTOR_REJECT'::character varying])::text[]))),
    CONSTRAINT needle_safety_events_v32_evidence_sha256_check CHECK ((evidence_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT needle_safety_events_v32_expected_fragments_check CHECK (((expected_fragments >= 0) AND (expected_fragments <= 20))),
    CONSTRAINT needle_safety_events_v32_note_check CHECK (((length(btrim(note)) >= 20) AND (length(btrim(note)) <= 800))),
    CONSTRAINT needle_safety_events_v32_recovered_fragments_check CHECK (((recovered_fragments >= 0) AND (recovered_fragments <= 20)))
);

ALTER TABLE ONLY public.needle_safety_events_v32 FORCE ROW LEVEL SECURITY;


--
-- Name: ocr_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ocr_records (
    id character varying(128) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    cut_no integer NOT NULL,
    cut_qty integer NOT NULL,
    shipped_qty integer NOT NULL,
    lost_qty integer NOT NULL,
    left_over_qty integer NOT NULL,
    variance integer NOT NULL,
    is_conserved boolean NOT NULL,
    variance_comment text DEFAULT ''::text NOT NULL,
    signatures_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    document_hash character varying(128) NOT NULL,
    cryptographic_proof character varying(128) NOT NULL,
    sealed_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ocr_records_cut_no_check CHECK ((cut_no > 0)),
    CONSTRAINT ocr_records_cut_qty_check CHECK ((cut_qty > 0)),
    CONSTRAINT ocr_records_left_over_qty_check CHECK ((left_over_qty >= 0)),
    CONSTRAINT ocr_records_lost_qty_check CHECK ((lost_qty >= 0)),
    CONSTRAINT ocr_records_shipped_qty_check CHECK ((shipped_qty >= 0))
);

ALTER TABLE ONLY public.ocr_records FORCE ROW LEVEL SECURITY;


--
-- Name: operation_master; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operation_master (
    id character varying(64) NOT NULL,
    operation_name character varying(255) NOT NULL,
    default_sam numeric(6,3) NOT NULL,
    machine_type character varying(64) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT operation_master_default_sam_check CHECK ((default_sam > (0)::numeric))
);


--
-- Name: order_intake_bom_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_intake_bom_v5 (
    tenant_id character varying(64) NOT NULL,
    intake_id character varying(40) NOT NULL,
    line_no integer NOT NULL,
    material_sku character varying(128) NOT NULL,
    category character varying(16) NOT NULL,
    qty_per_garment numeric(14,3) NOT NULL,
    uom character varying(16) NOT NULL,
    CONSTRAINT order_intake_bom_v5_category_check CHECK (((category)::text = ANY ((ARRAY['FABRIC'::character varying, 'TRIM'::character varying])::text[]))),
    CONSTRAINT order_intake_bom_v5_line_no_check CHECK ((line_no > 0)),
    CONSTRAINT order_intake_bom_v5_qty_per_garment_check CHECK ((qty_per_garment > (0)::numeric))
);

ALTER TABLE ONLY public.order_intake_bom_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: order_intake_skus_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_intake_skus_v5 (
    tenant_id character varying(64) NOT NULL,
    intake_id character varying(40) NOT NULL,
    line_no integer NOT NULL,
    color_code character varying(64) NOT NULL,
    size_code character varying(32) NOT NULL,
    upc character varying(64) DEFAULT ''::character varying NOT NULL,
    ordered_qty bigint NOT NULL,
    CONSTRAINT order_intake_skus_v5_line_no_check CHECK ((line_no > 0)),
    CONSTRAINT order_intake_skus_v5_ordered_qty_check CHECK ((ordered_qty > 0))
);

ALTER TABLE ONLY public.order_intake_skus_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: order_intakes_v5; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_intakes_v5 (
    id character varying(40) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    po_number character varying(64) NOT NULL,
    style_code character varying(64) NOT NULL,
    ordered_qty bigint NOT NULL,
    source_sha256 character(64) NOT NULL,
    po_filename character varying(255) NOT NULL,
    bom_filename character varying(255) NOT NULL,
    po_original bytea NOT NULL,
    bom_original bytea NOT NULL,
    status character varying(20) DEFAULT 'PROPOSED'::character varying NOT NULL,
    captured_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT order_intakes_v5_ordered_qty_check CHECK ((ordered_qty > 0)),
    CONSTRAINT order_intakes_v5_status_check CHECK (((status)::text = 'PROPOSED'::text))
);

ALTER TABLE ONLY public.order_intakes_v5 FORCE ROW LEVEL SECURITY;


--
-- Name: order_sku_matrix; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_sku_matrix (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    color_code character varying(64) NOT NULL,
    color_name character varying(128) DEFAULT ''::character varying NOT NULL,
    size_code character varying(32) NOT NULL,
    upc character varying(64) NOT NULL,
    ordered_qty integer NOT NULL,
    overcut_qty integer NOT NULL,
    target_cut_volume integer NOT NULL,
    unit_price_usd numeric(10,2) DEFAULT 0.00 NOT NULL,
    CONSTRAINT order_sku_matrix_ordered_qty_check CHECK ((ordered_qty > 0)),
    CONSTRAINT order_sku_matrix_overcut_qty_check CHECK ((overcut_qty >= 0)),
    CONSTRAINT order_sku_matrix_target_cut_volume_check CHECK ((target_cut_volume > 0))
);

ALTER TABLE ONLY public.order_sku_matrix FORCE ROW LEVEL SECURITY;


--
-- Name: pack_carton_events_v22; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_carton_events_v22 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    carton_id uuid NOT NULL,
    event_no bigint NOT NULL,
    action character varying(12) NOT NULL,
    spec_id uuid NOT NULL,
    carton_code character varying(80),
    garment_id uuid,
    observed_g bigint,
    expected_g bigint,
    scale_reference character varying(128),
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    evidence text DEFAULT 'SIGNED_OPERATOR_PACKING_WEIGHT_NOT_CALIBRATED_NOT_SHIPPED'::text NOT NULL,
    CONSTRAINT pack_carton_events_v22_action_check CHECK (((action)::text = ANY ((ARRAY['OPEN'::character varying, 'ADD'::character varying, 'SEAL'::character varying])::text[]))),
    CONSTRAINT pack_carton_events_v22_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT pack_carton_events_v22_check CHECK (((((action)::text = 'OPEN'::text) AND (carton_code IS NOT NULL) AND (garment_id IS NULL) AND (observed_g IS NULL) AND (expected_g IS NULL) AND (scale_reference IS NULL)) OR (((action)::text = 'ADD'::text) AND (carton_code IS NULL) AND (garment_id IS NOT NULL) AND (observed_g IS NULL) AND (expected_g IS NULL) AND (scale_reference IS NULL)) OR (((action)::text = 'SEAL'::text) AND (carton_code IS NULL) AND (garment_id IS NULL) AND (observed_g > 0) AND (expected_g > 0) AND (scale_reference IS NOT NULL)))),
    CONSTRAINT pack_carton_events_v22_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT pack_carton_events_v22_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT pack_carton_events_v22_event_no_check CHECK ((event_no > 0)),
    CONSTRAINT pack_carton_events_v22_evidence_check CHECK ((evidence = 'SIGNED_OPERATOR_PACKING_WEIGHT_NOT_CALIBRATED_NOT_SHIPPED'::text)),
    CONSTRAINT pack_carton_events_v22_expected_g_check CHECK ((expected_g >= 0)),
    CONSTRAINT pack_carton_events_v22_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 1000))),
    CONSTRAINT pack_carton_events_v22_observed_g_check CHECK ((observed_g >= 0))
);

ALTER TABLE ONLY public.pack_carton_events_v22 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_carton_specs_v22; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_carton_specs_v22 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    revision character varying(64) NOT NULL,
    source_document_id character varying(36) NOT NULL,
    source_sha256 character(64) NOT NULL,
    min_units integer NOT NULL,
    max_units integer NOT NULL,
    tare_g bigint NOT NULL,
    tolerance_g bigint NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT pack_carton_specs_v22_check CHECK ((max_units >= min_units)),
    CONSTRAINT pack_carton_specs_v22_check1 CHECK (((length(btrim((revision)::text)) > 0) AND (length(btrim((approved_by)::text)) > 0))),
    CONSTRAINT pack_carton_specs_v22_min_units_check CHECK ((min_units > 0)),
    CONSTRAINT pack_carton_specs_v22_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT pack_carton_specs_v22_tare_g_check CHECK ((tare_g >= 0)),
    CONSTRAINT pack_carton_specs_v22_tolerance_g_check CHECK ((tolerance_g >= 0))
);

ALTER TABLE ONLY public.pack_carton_specs_v22 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_manifest_events_v22; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_manifest_events_v22 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    manifest_id uuid NOT NULL,
    event_no bigint NOT NULL,
    action character varying(16) NOT NULL,
    spec_id uuid NOT NULL,
    manifest_ref character varying(80),
    destination_ref character varying(128),
    carton_id uuid,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    evidence text DEFAULT 'STAGED_DISPATCH_MANIFEST_NOT_CARRIER_HANDOVER_OR_SHIPMENT'::text NOT NULL,
    CONSTRAINT pack_manifest_events_v22_action_check CHECK (((action)::text = ANY ((ARRAY['OPEN'::character varying, 'ADD_CARTON'::character varying, 'STAGE'::character varying])::text[]))),
    CONSTRAINT pack_manifest_events_v22_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT pack_manifest_events_v22_check CHECK (((((action)::text = 'OPEN'::text) AND (manifest_ref IS NOT NULL) AND (destination_ref IS NOT NULL) AND (carton_id IS NULL)) OR (((action)::text = 'ADD_CARTON'::text) AND (manifest_ref IS NULL) AND (destination_ref IS NULL) AND (carton_id IS NOT NULL)) OR (((action)::text = 'STAGE'::text) AND (manifest_ref IS NULL) AND (destination_ref IS NULL) AND (carton_id IS NULL)))),
    CONSTRAINT pack_manifest_events_v22_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT pack_manifest_events_v22_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT pack_manifest_events_v22_event_no_check CHECK ((event_no > 0)),
    CONSTRAINT pack_manifest_events_v22_evidence_check CHECK ((evidence = 'STAGED_DISPATCH_MANIFEST_NOT_CARRIER_HANDOVER_OR_SHIPMENT'::text)),
    CONSTRAINT pack_manifest_events_v22_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 1000)))
);

ALTER TABLE ONLY public.pack_manifest_events_v22 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_prod_assignments_v12; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_prod_assignments_v12 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    worker_subject character varying(128) NOT NULL,
    role character varying(32) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    CONSTRAINT pack_prod_assignments_v12_role_check CHECK (((role)::text = ANY ((ARRAY['packing_operator'::character varying, 'packing_supervisor'::character varying])::text[])))
);

ALTER TABLE ONLY public.pack_prod_assignments_v12 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_prod_holds_v12; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_prod_holds_v12 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    command_id character varying(80) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    completion_id uuid NOT NULL,
    qc_event_id uuid NOT NULL,
    qc_revision integer NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    nominal_group_units integer NOT NULL,
    note character varying(1000) DEFAULT ''::character varying NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    state character varying(40) DEFAULT 'HOLD_PENDING_UNIT_TRACEABILITY'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT pack_prod_holds_v12_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT pack_prod_holds_v12_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT pack_prod_holds_v12_nominal_group_units_check CHECK ((nominal_group_units > 0)),
    CONSTRAINT pack_prod_holds_v12_payload_sha256_check CHECK ((payload_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT pack_prod_holds_v12_qc_revision_check CHECK ((qc_revision > 0)),
    CONSTRAINT pack_prod_holds_v12_state_check CHECK (((state)::text = 'HOLD_PENDING_UNIT_TRACEABILITY'::text))
);

ALTER TABLE ONLY public.pack_prod_holds_v12 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_sim_cartons_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_sim_cartons_v11 (
    tenant_id character varying(64) NOT NULL,
    carton_code character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    state character varying(16) DEFAULT 'OPEN'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    sealed_at timestamp with time zone,
    provenance character varying(48) DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION'::character varying NOT NULL,
    CONSTRAINT pack_sim_cartons_v11_provenance_check CHECK (((provenance)::text = 'UI_SIMULATED_NO_RF_OBSERVATION'::text)),
    CONSTRAINT pack_sim_cartons_v11_state_check CHECK (((state)::text = ANY ((ARRAY['OPEN'::character varying, 'SEALED'::character varying])::text[])))
);

ALTER TABLE ONLY public.pack_sim_cartons_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_sim_events_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_sim_events_v11 (
    tenant_id character varying(64) NOT NULL,
    event_id character varying(80) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    action character varying(32) NOT NULL,
    carton_code character varying(64) DEFAULT ''::character varying NOT NULL,
    manifest_code character varying(64) DEFAULT ''::character varying NOT NULL,
    serial character varying(128) DEFAULT ''::character varying NOT NULL,
    actor character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    reader_payload jsonb NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT pack_sim_events_v11_action_check CHECK (((action)::text = ANY ((ARRAY['OPEN_CARTON'::character varying, 'ADD_GARMENT'::character varying, 'REMOVE_GARMENT'::character varying, 'SEAL_CARTON'::character varying, 'OPEN_MANIFEST'::character varying, 'LOAD_CARTON'::character varying, 'DISPATCH_REHEARSAL'::character varying])::text[]))),
    CONSTRAINT pack_sim_events_v11_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT pack_sim_events_v11_reader_payload_check CHECK (((jsonb_typeof(reader_payload) = 'object'::text) AND ((reader_payload ->> 'version'::text) = 'PACK_READER_V1'::text) AND ((reader_payload ->> 'source'::text) = 'UI_SIMULATED'::text) AND ((reader_payload ->> 'epc_hex'::text) IS NULL) AND ((reader_payload ->> 'rssi_dbm'::text) IS NULL)))
);

ALTER TABLE ONLY public.pack_sim_events_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_sim_items_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_sim_items_v11 (
    tenant_id character varying(64) NOT NULL,
    serial character varying(128) NOT NULL,
    carton_code character varying(64) NOT NULL,
    buyer_code character varying(96) NOT NULL,
    added_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY public.pack_sim_items_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_sim_manifest_cartons_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_sim_manifest_cartons_v11 (
    tenant_id character varying(64) NOT NULL,
    manifest_code character varying(64) NOT NULL,
    carton_code character varying(64) NOT NULL,
    item_count integer NOT NULL,
    CONSTRAINT pack_sim_manifest_cartons_v11_item_count_check CHECK ((item_count > 0))
);

ALTER TABLE ONLY public.pack_sim_manifest_cartons_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_sim_manifests_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_sim_manifests_v11 (
    tenant_id character varying(64) NOT NULL,
    manifest_code character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    state character varying(24) DEFAULT 'DRAFT'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    dispatched_at timestamp with time zone,
    CONSTRAINT pack_sim_manifests_v11_state_check CHECK (((state)::text = ANY ((ARRAY['DRAFT'::character varying, 'DISPATCH_REHEARSAL'::character varying])::text[])))
);

ALTER TABLE ONLY public.pack_sim_manifests_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: pack_size_weights_v22; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pack_size_weights_v22 (
    tenant_id character varying(64) NOT NULL,
    spec_id uuid NOT NULL,
    size_code character varying(32) NOT NULL,
    expected_g bigint NOT NULL,
    CONSTRAINT pack_size_weights_v22_expected_g_check CHECK ((expected_g > 0))
);

ALTER TABLE ONLY public.pack_size_weights_v22 FORCE ROW LEVEL SECURITY;


--
-- Name: plant_hardware_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.plant_hardware_assets (
    id character varying(128) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) DEFAULT 'PLANT_DEFAULT'::character varying NOT NULL,
    device_type character varying(64) NOT NULL,
    ip_address character varying(64) NOT NULL,
    mac_address character varying(64) NOT NULL,
    zone character varying(64) NOT NULL,
    station_id character varying(64),
    status character varying(32) DEFAULT 'ONLINE'::character varying NOT NULL,
    last_seen timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.plant_hardware_assets FORCE ROW LEVEL SECURITY;


--
-- Name: post_sewing_processes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_sewing_processes (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    process_name character varying(128) NOT NULL,
    facility_unit character varying(128) NOT NULL,
    wash_duration_mins integer NOT NULL,
    target_shrinkage_warp_pct numeric(5,2) DEFAULT '-1.50'::numeric NOT NULL,
    target_shrinkage_weft_pct numeric(5,2) DEFAULT '-1.00'::numeric NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT post_sewing_processes_wash_duration_mins_check CHECK ((wash_duration_mins > 0))
);

ALTER TABLE ONLY public.post_sewing_processes FORCE ROW LEVEL SECURITY;


--
-- Name: qc_garment_defects_v21; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_garment_defects_v21 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    defect_code character varying(64) NOT NULL,
    description character varying(512) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    CONSTRAINT qc_garment_defects_v21_defect_code_check CHECK (((defect_code)::text ~ '^[A-Za-z0-9_-]{2,64}$'::text)),
    CONSTRAINT qc_garment_defects_v21_description_check CHECK ((length(btrim((description)::text)) > 0))
);

ALTER TABLE ONLY public.qc_garment_defects_v21 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_garment_events_v21; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_garment_events_v21 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    garment_id uuid NOT NULL,
    spec_id uuid NOT NULL,
    event_no bigint NOT NULL,
    action character varying(24) NOT NULL,
    defect_code character varying(64),
    measurements_mm jsonb DEFAULT '{}'::jsonb NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    evidence text DEFAULT 'SIGNED_GARMENT_QC_HUMAN_ATTESTATION_NOT_SHIPPING_AUTHORITY'::text NOT NULL,
    CONSTRAINT qc_garment_events_v21_action_check CHECK (((action)::text = ANY ((ARRAY['INSPECT_PASS'::character varying, 'INSPECT_HOLD'::character varying, 'REPAIR_COMPLETE'::character varying, 'REINSPECT_PASS'::character varying, 'REINSPECT_HOLD'::character varying, 'FINISHING_COMPLETE'::character varying, 'QUALITY_RELEASE'::character varying])::text[]))),
    CONSTRAINT qc_garment_events_v21_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT qc_garment_events_v21_check CHECK (((((action)::text = ANY ((ARRAY['INSPECT_HOLD'::character varying, 'REINSPECT_HOLD'::character varying])::text[])) AND (defect_code IS NOT NULL)) OR (((action)::text <> ALL ((ARRAY['INSPECT_HOLD'::character varying, 'REINSPECT_HOLD'::character varying])::text[])) AND (defect_code IS NULL)))),
    CONSTRAINT qc_garment_events_v21_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT qc_garment_events_v21_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT qc_garment_events_v21_event_no_check CHECK ((event_no > 0)),
    CONSTRAINT qc_garment_events_v21_evidence_check CHECK ((evidence = 'SIGNED_GARMENT_QC_HUMAN_ATTESTATION_NOT_SHIPPING_AUTHORITY'::text)),
    CONSTRAINT qc_garment_events_v21_measurements_mm_check CHECK ((jsonb_typeof(measurements_mm) = 'object'::text)),
    CONSTRAINT qc_garment_events_v21_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 1000)))
);

ALTER TABLE ONLY public.qc_garment_events_v21 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_garment_measure_rules_v21; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_garment_measure_rules_v21 (
    tenant_id character varying(64) NOT NULL,
    spec_id uuid NOT NULL,
    size_code character varying(32) NOT NULL,
    measure_code character varying(64) NOT NULL,
    nominal_mm bigint NOT NULL,
    lower_tolerance_mm bigint NOT NULL,
    upper_tolerance_mm bigint NOT NULL,
    CONSTRAINT qc_garment_measure_rules_v21_lower_tolerance_mm_check CHECK ((lower_tolerance_mm >= 0)),
    CONSTRAINT qc_garment_measure_rules_v21_measure_code_check CHECK (((measure_code)::text ~ '^[A-Za-z0-9_-]{2,64}$'::text)),
    CONSTRAINT qc_garment_measure_rules_v21_nominal_mm_check CHECK ((nominal_mm >= 0)),
    CONSTRAINT qc_garment_measure_rules_v21_upper_tolerance_mm_check CHECK ((upper_tolerance_mm >= 0))
);

ALTER TABLE ONLY public.qc_garment_measure_rules_v21 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_garment_specs_v21; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_garment_specs_v21 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    revision character varying(64) NOT NULL,
    source_document_id character varying(36) NOT NULL,
    source_sha256 character(64) NOT NULL,
    finish_station character varying(64) NOT NULL,
    approved_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    inspection_basis text DEFAULT 'EVERY_GARMENT_NOT_AQL_SAMPLE'::text NOT NULL,
    CONSTRAINT qc_garment_specs_v21_check CHECK (((length(btrim((revision)::text)) > 0) AND (length(btrim((approved_by)::text)) > 0))),
    CONSTRAINT qc_garment_specs_v21_inspection_basis_check CHECK ((inspection_basis = 'EVERY_GARMENT_NOT_AQL_SAMPLE'::text))
);

ALTER TABLE ONLY public.qc_garment_specs_v21 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_prod_assignments_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_prod_assignments_v11 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    worker_subject character varying(128) NOT NULL,
    role character varying(32) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    CONSTRAINT qc_prod_assignments_v11_role_check CHECK (((role)::text = ANY ((ARRAY['quality_inspector'::character varying, 'quality_repairer'::character varying])::text[])))
);

ALTER TABLE ONLY public.qc_prod_assignments_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_prod_defect_catalog_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_prod_defect_catalog_v11 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    code character varying(64) NOT NULL,
    description text NOT NULL,
    active boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    CONSTRAINT qc_prod_defect_catalog_v11_code_check CHECK (((code)::text ~ '^[A-Za-z0-9_-]{2,64}$'::text)),
    CONSTRAINT qc_prod_defect_catalog_v11_description_check CHECK (((length(TRIM(BOTH FROM description)) >= 2) AND (length(TRIM(BOTH FROM description)) <= 240)))
);

ALTER TABLE ONLY public.qc_prod_defect_catalog_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_prod_group_events_v11; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_prod_group_events_v11 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    command_id character varying(80) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    completion_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    action character varying(24) NOT NULL,
    inspected_units integer NOT NULL,
    defective_units integer NOT NULL,
    defect_code character varying(64) DEFAULT ''::character varying NOT NULL,
    note character varying(1000) DEFAULT ''::character varying NOT NULL,
    actor_subject character varying(128) NOT NULL,
    proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    prior_state character varying(16) DEFAULT ''::character varying NOT NULL,
    next_state character varying(16) DEFAULT ''::character varying NOT NULL,
    revision integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT qc_prod_group_events_v11_action_check CHECK (((action)::text = ANY ((ARRAY['INSPECT_PASS'::character varying, 'INSPECT_HOLD'::character varying, 'REPAIR_COMPLETE'::character varying, 'REINSPECT_PASS'::character varying, 'REINSPECT_HOLD'::character varying])::text[]))),
    CONSTRAINT qc_prod_group_events_v11_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT qc_prod_group_events_v11_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT qc_prod_group_events_v11_defective_units_check CHECK ((defective_units >= 0)),
    CONSTRAINT qc_prod_group_events_v11_inspected_units_check CHECK ((inspected_units >= 0)),
    CONSTRAINT qc_prod_group_events_v11_payload_sha256_check CHECK ((payload_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT qc_prod_group_events_v11_revision_check CHECK ((revision > 0))
);

ALTER TABLE ONLY public.qc_prod_group_events_v11 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_sim_buyer_bindings_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_sim_buyer_bindings_v10 (
    tenant_id character varying(64) NOT NULL,
    serial character varying(128) NOT NULL,
    buyer_code character varying(96) NOT NULL,
    style_code character varying(96) NOT NULL,
    size_code character varying(32) NOT NULL,
    event_id character varying(80) NOT NULL,
    evidence_state character varying(48) DEFAULT 'UNVERIFIED_SIMULATED_BUYER_BIND'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT qc_sim_buyer_bindings_v10_buyer_code_check CHECK (((length((buyer_code)::text) >= 4) AND (length((buyer_code)::text) <= 96))),
    CONSTRAINT qc_sim_buyer_bindings_v10_evidence_state_check CHECK (((evidence_state)::text = 'UNVERIFIED_SIMULATED_BUYER_BIND'::text))
);

ALTER TABLE ONLY public.qc_sim_buyer_bindings_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_sim_events_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_sim_events_v10 (
    tenant_id character varying(64) NOT NULL,
    event_id character varying(80) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    serial character varying(128) NOT NULL,
    action character varying(32) NOT NULL,
    actor character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    detail character varying(128) DEFAULT ''::character varying NOT NULL,
    reader_payload jsonb NOT NULL,
    prior_state character varying(24) NOT NULL,
    next_state character varying(24) NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    provenance character varying(48) DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION'::character varying NOT NULL,
    CONSTRAINT qc_sim_events_v10_action_check CHECK (((action)::text = ANY ((ARRAY['ENROLL'::character varying, 'INSPECT_PASS'::character varying, 'INSPECT_FAIL'::character varying, 'REPAIR_COMPLETE'::character varying, 'REINSPECT_PASS'::character varying, 'REINSPECT_FAIL'::character varying, 'FINISH_STEP'::character varying, 'BIND_BUYER'::character varying])::text[]))),
    CONSTRAINT qc_sim_events_v10_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT qc_sim_events_v10_payload_sha256_check CHECK ((payload_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT qc_sim_events_v10_provenance_check CHECK (((provenance)::text = 'UI_SIMULATED_NO_RF_OBSERVATION'::text)),
    CONSTRAINT qc_sim_events_v10_reader_payload_check CHECK (((jsonb_typeof(reader_payload) = 'object'::text) AND ((reader_payload ->> 'version'::text) = 'QC_READER_V1'::text) AND ((reader_payload ->> 'source'::text) = 'UI_SIMULATED'::text) AND ((reader_payload ->> 'epc_hex'::text) IS NULL) AND ((reader_payload ->> 'rssi_dbm'::text) IS NULL)))
);

ALTER TABLE ONLY public.qc_sim_events_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_sim_routes_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_sim_routes_v10 (
    tenant_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    profile character varying(32) NOT NULL,
    steps text[] NOT NULL,
    evidence_state character varying(40) DEFAULT 'UNAPPROVED_SIMULATION_ROUTE'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT qc_sim_routes_v10_check CHECK (((((profile)::text = 'WOVEN_STANDARD'::text) AND (steps = ARRAY['TRIM'::text, 'PRESS'::text])) OR (((profile)::text = 'WASH_FINISH'::text) AND (steps = ARRAY['WASH'::text, 'DRY'::text, 'PRESS'::text, 'TRIM'::text])) OR (((profile)::text = 'HANGER_FINISH'::text) AND (steps = ARRAY['TRIM'::text, 'PRESS'::text, 'HANGER'::text])) OR (((profile)::text = 'KNIT_FOLD'::text) AND (steps = ARRAY['TRIM'::text, 'PRESS'::text, 'FOLD'::text])))),
    CONSTRAINT qc_sim_routes_v10_evidence_state_check CHECK (((evidence_state)::text = 'UNAPPROVED_SIMULATION_ROUTE'::text)),
    CONSTRAINT qc_sim_routes_v10_profile_check CHECK (((profile)::text = ANY ((ARRAY['WOVEN_STANDARD'::character varying, 'WASH_FINISH'::character varying, 'HANGER_FINISH'::character varying, 'KNIT_FOLD'::character varying])::text[])))
);

ALTER TABLE ONLY public.qc_sim_routes_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: qc_sim_units_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.qc_sim_units_v10 (
    tenant_id character varying(64) NOT NULL,
    serial character varying(128) NOT NULL,
    lay_draft_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    marker_position integer NOT NULL,
    color_code character varying(64) NOT NULL,
    size_code character varying(32) NOT NULL,
    qc_state character varying(24) DEFAULT 'PENDING'::character varying NOT NULL,
    finishing_count integer DEFAULT 0 NOT NULL,
    last_defect character varying(64) DEFAULT ''::character varying NOT NULL,
    fail_inspector character varying(64) DEFAULT ''::character varying NOT NULL,
    repair_actor character varying(64) DEFAULT ''::character varying NOT NULL,
    last_inspector character varying(64) DEFAULT ''::character varying NOT NULL,
    revision integer DEFAULT 0 NOT NULL,
    source_state character varying(48) DEFAULT 'SEWING_SIMULATION_NO_PRODUCTION_CREDIT'::character varying NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT qc_sim_units_v10_finishing_count_check CHECK (((finishing_count >= 0) AND (finishing_count <= 5))),
    CONSTRAINT qc_sim_units_v10_qc_state_check CHECK (((qc_state)::text = ANY ((ARRAY['PENDING'::character varying, 'HOLD'::character varying, 'REINSPECT'::character varying, 'ACCEPTED'::character varying])::text[]))),
    CONSTRAINT qc_sim_units_v10_revision_check CHECK ((revision >= 0)),
    CONSTRAINT qc_sim_units_v10_serial_check CHECK (((serial)::text ~~ 'SIM-G-%'::text)),
    CONSTRAINT qc_sim_units_v10_source_state_check CHECK (((source_state)::text = 'SEWING_SIMULATION_NO_PRODUCTION_CREDIT'::text))
);

ALTER TABLE ONLY public.qc_sim_units_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_piece_completions_v27; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_piece_completions_v27 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    garment_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    release_id uuid NOT NULL,
    route_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    marker_position integer NOT NULL,
    ply_group integer NOT NULL,
    scan_ids uuid[] NOT NULL,
    worker_subject character varying(128) NOT NULL,
    worker_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    evidence_state text DEFAULT 'WORKER_ATTESTED_UNIT_OPERATION_PENDING_QUALITY_NOT_PAYROLL'::text NOT NULL,
    CONSTRAINT sew_piece_completions_v27_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_piece_completions_v27_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT sew_piece_completions_v27_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_piece_completions_v27_evidence_state_check CHECK ((evidence_state = 'WORKER_ATTESTED_UNIT_OPERATION_PENDING_QUALITY_NOT_PAYROLL'::text)),
    CONSTRAINT sew_piece_completions_v27_marker_position_check CHECK ((marker_position > 0)),
    CONSTRAINT sew_piece_completions_v27_ply_group_check CHECK ((ply_group > 0)),
    CONSTRAINT sew_piece_completions_v27_scan_ids_check CHECK (((cardinality(scan_ids) >= 1) AND (cardinality(scan_ids) <= 32)))
);

ALTER TABLE ONLY public.sew_piece_completions_v27 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_piece_used_scans_v27; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_piece_used_scans_v27 (
    tenant_id character varying(64) NOT NULL,
    scan_id uuid NOT NULL,
    completion_id uuid NOT NULL
);

ALTER TABLE ONLY public.sew_piece_used_scans_v27 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_prod_completions_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_prod_completions_v10 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    release_id uuid NOT NULL,
    route_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    marker_position integer NOT NULL,
    ply_group integer NOT NULL,
    observation_ids uuid[] NOT NULL,
    nominal_ply_count integer NOT NULL,
    worker_subject character varying(128) NOT NULL,
    worker_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    evidence_state character varying(64) DEFAULT 'WORKER_ATTESTED_PENDING_QUALITY_ACCEPTANCE'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT sew_prod_completions_v10_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_prod_completions_v10_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT sew_prod_completions_v10_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_prod_completions_v10_evidence_state_check CHECK (((evidence_state)::text = 'WORKER_ATTESTED_PENDING_QUALITY_ACCEPTANCE'::text)),
    CONSTRAINT sew_prod_completions_v10_nominal_ply_count_check CHECK ((nominal_ply_count > 0)),
    CONSTRAINT sew_prod_completions_v10_observation_ids_check CHECK ((cardinality(observation_ids) > 0))
);

ALTER TABLE ONLY public.sew_prod_completions_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_prod_gateways_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_prod_gateways_v10 (
    tenant_id character varying(64) NOT NULL,
    gateway_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    public_key bytea NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    enrolled_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT sew_prod_gateways_v10_gateway_id_check CHECK (((gateway_id)::text ~ '^[A-Za-z0-9_-]{3,64}$'::text)),
    CONSTRAINT sew_prod_gateways_v10_public_key_check CHECK ((octet_length(public_key) = 32))
);

ALTER TABLE ONLY public.sew_prod_gateways_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_prod_observations_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_prod_observations_v10 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    event_id character varying(80) NOT NULL,
    gateway_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    release_id uuid NOT NULL,
    label_code character varying(29) NOT NULL,
    epc_hex character varying(96) NOT NULL,
    marker_position integer NOT NULL,
    ply_group integer NOT NULL,
    part_code character varying(64) NOT NULL,
    observed_at timestamp with time zone NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    event_sha256 character(64) NOT NULL,
    signature_sha256 character(64) NOT NULL,
    evidence_state character varying(64) DEFAULT 'SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'::character varying NOT NULL,
    CONSTRAINT sew_prod_observations_v10_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT sew_prod_observations_v10_event_sha256_check CHECK ((event_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_prod_observations_v10_evidence_state_check CHECK (((evidence_state)::text = 'SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'::text)),
    CONSTRAINT sew_prod_observations_v10_signature_sha256_check CHECK ((signature_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.sew_prod_observations_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_prod_routes_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_prod_routes_v10 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    lay_draft_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    operation_code character varying(64) NOT NULL,
    revision character varying(64) NOT NULL,
    required_parts text[] NOT NULL,
    predecessors text[] DEFAULT '{}'::text[] NOT NULL,
    station_id character varying(64) NOT NULL,
    approved_by character varying(128) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT sew_prod_routes_v10_operation_code_check CHECK (((operation_code)::text ~ '^[A-Za-z0-9_-]{2,64}$'::text)),
    CONSTRAINT sew_prod_routes_v10_required_parts_check CHECK ((cardinality(required_parts) > 0))
);

ALTER TABLE ONLY public.sew_prod_routes_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_prod_used_observations_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_prod_used_observations_v10 (
    tenant_id character varying(64) NOT NULL,
    observation_id uuid NOT NULL,
    completion_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY public.sew_prod_used_observations_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_prod_worker_stations_v10; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_prod_worker_stations_v10 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    worker_subject character varying(128) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY public.sew_prod_worker_stations_v10 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_sim_credits_v9; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_sim_credits_v9 (
    tenant_id character varying(64) NOT NULL,
    cycle_id uuid NOT NULL,
    operation_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    marker_position integer NOT NULL,
    ply_group integer NOT NULL,
    garment_count integer NOT NULL,
    serials text[] NOT NULL,
    active_ms bigint NOT NULL,
    evidence_state character varying(48) DEFAULT 'SIMULATED_NOT_PRODUCTION_CREDIT'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT sew_sim_credits_v9_active_ms_check CHECK ((active_ms >= 0)),
    CONSTRAINT sew_sim_credits_v9_check CHECK ((cardinality(serials) = garment_count)),
    CONSTRAINT sew_sim_credits_v9_evidence_state_check CHECK (((evidence_state)::text = 'SIMULATED_NOT_PRODUCTION_CREDIT'::text)),
    CONSTRAINT sew_sim_credits_v9_garment_count_check CHECK (((garment_count >= 1) AND (garment_count <= 210)))
);

ALTER TABLE ONLY public.sew_sim_credits_v9 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_sim_cycle_inputs_v9; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_sim_cycle_inputs_v9 (
    tenant_id character varying(64) NOT NULL,
    cycle_id uuid NOT NULL,
    scan_id uuid NOT NULL,
    label_code character varying(29) NOT NULL,
    part_code character varying(64) NOT NULL
);

ALTER TABLE ONLY public.sew_sim_cycle_inputs_v9 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_sim_cycles_v9; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_sim_cycles_v9 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    claim_id character varying(80) NOT NULL,
    claim_sha256 character(64) NOT NULL,
    operation_id uuid NOT NULL,
    lay_draft_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    marker_position integer NOT NULL,
    ply_group integer NOT NULL,
    station_id character varying(64) NOT NULL,
    worker_id character varying(64) NOT NULL,
    machine_class character varying(64) NOT NULL,
    state character varying(24) DEFAULT 'CLAIMED'::character varying NOT NULL,
    active_since timestamp with time zone,
    active_ms bigint DEFAULT 0 NOT NULL,
    paused_ms bigint DEFAULT 0 NOT NULL,
    paused_since timestamp with time zone,
    claimed_at timestamp with time zone DEFAULT now() NOT NULL,
    finished_at timestamp with time zone,
    CONSTRAINT sew_sim_cycles_v9_active_ms_check CHECK ((active_ms >= 0)),
    CONSTRAINT sew_sim_cycles_v9_claim_id_check CHECK (((claim_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT sew_sim_cycles_v9_claim_sha256_check CHECK ((claim_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_sim_cycles_v9_paused_ms_check CHECK ((paused_ms >= 0)),
    CONSTRAINT sew_sim_cycles_v9_state_check CHECK (((state)::text = ANY ((ARRAY['CLAIMED'::character varying, 'PAUSED'::character varying, 'COMPLETED'::character varying, 'REVIEW_REQUIRED'::character varying])::text[])))
);

ALTER TABLE ONLY public.sew_sim_cycles_v9 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_sim_events_v9; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_sim_events_v9 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    command_id character varying(80) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    cycle_id uuid,
    plant_id character varying(64) NOT NULL,
    action character varying(32) NOT NULL,
    station_id character varying(64) NOT NULL,
    worker_id character varying(64) NOT NULL,
    detail text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    provenance character varying(48) DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION'::character varying NOT NULL,
    CONSTRAINT sew_sim_events_v9_action_check CHECK (((action)::text = ANY ((ARRAY['CLAIM'::character varying, 'PAUSE'::character varying, 'RESUME'::character varying, 'COMPLETE'::character varying, 'ISSUE'::character varying])::text[]))),
    CONSTRAINT sew_sim_events_v9_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT sew_sim_events_v9_detail_check CHECK ((length(detail) <= 800)),
    CONSTRAINT sew_sim_events_v9_payload_sha256_check CHECK ((payload_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_sim_events_v9_provenance_check CHECK (((provenance)::text = 'UI_SIMULATED_NO_RF_OBSERVATION'::text))
);

ALTER TABLE ONLY public.sew_sim_events_v9 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_sim_operations_v9; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_sim_operations_v9 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    lay_draft_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    operation_code character varying(64) NOT NULL,
    mode character varying(32) NOT NULL,
    required_parts text[] NOT NULL,
    predecessors text[] DEFAULT '{}'::text[] NOT NULL,
    machine_class character varying(64) NOT NULL,
    sam_seconds numeric(10,2) NOT NULL,
    route_revision character varying(64) DEFAULT 'UI_SIMULATION_V1'::character varying NOT NULL,
    evidence_state character varying(48) DEFAULT 'UNPUBLISHED_SIMULATION_CONTRACT'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT sew_sim_operations_v9_evidence_state_check CHECK (((evidence_state)::text = 'UNPUBLISHED_SIMULATION_CONTRACT'::text)),
    CONSTRAINT sew_sim_operations_v9_machine_class_check CHECK (((length((machine_class)::text) >= 2) AND (length((machine_class)::text) <= 64))),
    CONSTRAINT sew_sim_operations_v9_mode_check CHECK (((mode)::text = ANY ((ARRAY['SINGLE_TO_SINGLE'::character varying, 'BUNDLE_TO_BUNDLE'::character varying, 'BUNDLE_TO_SINGLE'::character varying])::text[]))),
    CONSTRAINT sew_sim_operations_v9_operation_code_check CHECK (((operation_code)::text ~ '^[A-Za-z0-9_-]{2,64}$'::text)),
    CONSTRAINT sew_sim_operations_v9_required_parts_check CHECK (((cardinality(required_parts) >= 1) AND (cardinality(required_parts) <= 16))),
    CONSTRAINT sew_sim_operations_v9_sam_seconds_check CHECK (((sam_seconds > (0)::numeric) AND (sam_seconds <= (36000)::numeric)))
);

ALTER TABLE ONLY public.sew_sim_operations_v9 FORCE ROW LEVEL SECURITY;


--
-- Name: sew_sim_scans_v9; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sew_sim_scans_v9 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    event_id character varying(80) NOT NULL,
    payload_sha256 character(64) NOT NULL,
    lay_draft_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    label_code character varying(29) NOT NULL,
    station_id character varying(64) NOT NULL,
    worker_id character varying(64) NOT NULL,
    reader_id character varying(64) NOT NULL,
    operator_clicked_at timestamp with time zone NOT NULL,
    antenna_port integer NOT NULL,
    epc_hex character varying(96),
    rssi_dbm numeric(6,2),
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    provenance character varying(48) DEFAULT 'UI_SIMULATED_NO_RF_OBSERVATION'::character varying NOT NULL,
    CONSTRAINT sew_sim_scans_v9_antenna_port_check CHECK (((antenna_port >= 1) AND (antenna_port <= 64))),
    CONSTRAINT sew_sim_scans_v9_check CHECK (((epc_hex IS NULL) AND (rssi_dbm IS NULL))),
    CONSTRAINT sew_sim_scans_v9_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT sew_sim_scans_v9_payload_sha256_check CHECK ((payload_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT sew_sim_scans_v9_provenance_check CHECK (((provenance)::text = 'UI_SIMULATED_NO_RF_OBSERVATION'::text))
);

ALTER TABLE ONLY public.sew_sim_scans_v9 FORCE ROW LEVEL SECURITY;


--
-- Name: store_issuances; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_issuances (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    cut_no integer NOT NULL,
    issued_length_m numeric(10,2) NOT NULL,
    remaining_length_m numeric(10,2) NOT NULL,
    issuer_id character varying(64) NOT NULL,
    recipient_id character varying(64) NOT NULL,
    digital_signature_hash character varying(128) NOT NULL,
    issued_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT store_issuances_cut_no_check CHECK ((cut_no > 0)),
    CONSTRAINT store_issuances_issued_length_m_check CHECK ((issued_length_m > (0)::numeric)),
    CONSTRAINT store_issuances_remaining_length_m_check CHECK ((remaining_length_m >= (0)::numeric))
);

ALTER TABLE ONLY public.store_issuances FORCE ROW LEVEL SECURITY;


--
-- Name: store_pick_proposals_v7; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_pick_proposals_v7 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    intake_id character varying(40) NOT NULL,
    request_key character varying(128) NOT NULL,
    review_note text NOT NULL,
    source_order_sha256 character(64) NOT NULL,
    snapshot jsonb NOT NULL,
    snapshot_sha256 character(64) NOT NULL,
    state text DEFAULT 'PROPOSED_HOLD'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT store_pick_proposals_v7_request_key_check CHECK (((request_key)::text ~ '^[A-Za-z0-9_-]{8,128}$'::text)),
    CONSTRAINT store_pick_proposals_v7_review_note_check CHECK (((length(btrim(review_note)) >= 12) AND (length(btrim(review_note)) <= 1000))),
    CONSTRAINT store_pick_proposals_v7_snapshot_check CHECK ((jsonb_typeof(snapshot) = 'object'::text)),
    CONSTRAINT store_pick_proposals_v7_snapshot_sha256_check CHECK ((snapshot_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT store_pick_proposals_v7_source_order_sha256_check CHECK ((source_order_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT store_pick_proposals_v7_state_check CHECK ((state = 'PROPOSED_HOLD'::text))
);

ALTER TABLE ONLY public.store_pick_proposals_v7 FORCE ROW LEVEL SECURITY;


--
-- Name: store_reconciliation_requests_v7; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_reconciliation_requests_v7 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    passport_revision integer NOT NULL,
    request_key character varying(128) NOT NULL,
    note text NOT NULL,
    assessment jsonb NOT NULL,
    assessment_hash character(64) NOT NULL,
    state character varying(48) DEFAULT 'PENDING_INDEPENDENT_RECONCILIATION'::character varying NOT NULL,
    reported_by character varying(64) DEFAULT 'dev'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT store_reconciliation_requests_v7_assessment_check CHECK ((jsonb_typeof(assessment) = 'object'::text)),
    CONSTRAINT store_reconciliation_requests_v7_assessment_hash_check CHECK ((assessment_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT store_reconciliation_requests_v7_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 1000))),
    CONSTRAINT store_reconciliation_requests_v7_passport_revision_check CHECK ((passport_revision > 0)),
    CONSTRAINT store_reconciliation_requests_v7_request_key_check CHECK (((length((request_key)::text) >= 8) AND (length((request_key)::text) <= 128))),
    CONSTRAINT store_reconciliation_requests_v7_state_check CHECK (((state)::text = 'PENDING_INDEPENDENT_RECONCILIATION'::text))
);

ALTER TABLE ONLY public.store_reconciliation_requests_v7 FORCE ROW LEVEL SECURITY;


--
-- Name: store_roll_admission_proofs_v7; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_roll_admission_proofs_v7 (
    tenant_id character varying(64) NOT NULL,
    proof_id character varying(255) NOT NULL,
    admission_id uuid NOT NULL,
    witness_role character varying(20) NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT store_roll_admission_proofs_v7_witness_role_check CHECK (((witness_role)::text = ANY ((ARRAY['STORES_MANAGER'::character varying, 'STORES_INSPECTOR'::character varying])::text[])))
);

ALTER TABLE ONLY public.store_roll_admission_proofs_v7 FORCE ROW LEVEL SECURITY;


--
-- Name: store_roll_admissions_v7; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_roll_admissions_v7 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    request_id uuid NOT NULL,
    roll_id character varying(64) NOT NULL,
    passport_revision integer NOT NULL,
    plant_id character varying(64) NOT NULL,
    assessment_hash character(64) NOT NULL,
    signed_challenge_hash character(64) NOT NULL,
    admitted_mm bigint NOT NULL,
    manager_subject character varying(128) NOT NULL,
    inspector_subject character varying(128) NOT NULL,
    manager_proof_id character varying(255) NOT NULL,
    inspector_proof_id character varying(255) NOT NULL,
    manager_assertion_hash character(64) NOT NULL,
    inspector_assertion_hash character(64) NOT NULL,
    admitted_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT store_roll_admissions_v7_admitted_mm_check CHECK ((admitted_mm > 0)),
    CONSTRAINT store_roll_admissions_v7_assessment_hash_check CHECK ((assessment_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT store_roll_admissions_v7_check CHECK (((manager_subject)::text <> (inspector_subject)::text)),
    CONSTRAINT store_roll_admissions_v7_check1 CHECK (((manager_proof_id)::text <> (inspector_proof_id)::text)),
    CONSTRAINT store_roll_admissions_v7_passport_revision_check CHECK ((passport_revision > 0)),
    CONSTRAINT store_roll_admissions_v7_signed_challenge_hash_check CHECK ((signed_challenge_hash ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.store_roll_admissions_v7 FORCE ROW LEVEL SECURITY;


--
-- Name: store_roll_passport_events_v7; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_roll_passport_events_v7 (
    tenant_id character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    revision integer NOT NULL,
    event_kind character varying(12) NOT NULL,
    epc_hex character varying(64),
    material_sku character varying(128) NOT NULL,
    supplier_lot character varying(128) NOT NULL,
    shade_band character varying(32) NOT NULL,
    bin_location character varying(64) NOT NULL,
    declared_length_mm bigint NOT NULL,
    measured_length_mm bigint NOT NULL,
    usable_width_mm integer NOT NULL,
    inspection_result character varying(12) NOT NULL,
    defects text DEFAULT ''::text NOT NULL,
    source_document_id character varying(36),
    source_note text NOT NULL,
    recorded_by character varying(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    evidence_class character varying(32) DEFAULT 'OPERATOR_REPORTED'::character varying NOT NULL,
    CONSTRAINT store_roll_passport_events_v7_check CHECK (((((event_kind)::text = 'HOLD'::text) AND ((inspection_result)::text = 'HOLD'::text)) OR ((event_kind)::text <> 'HOLD'::text))),
    CONSTRAINT store_roll_passport_events_v7_declared_length_mm_check CHECK ((declared_length_mm > 0)),
    CONSTRAINT store_roll_passport_events_v7_defects_check CHECK ((length(defects) <= 2000)),
    CONSTRAINT store_roll_passport_events_v7_event_kind_check CHECK (((event_kind)::text = ANY ((ARRAY['RECEIPT'::character varying, 'REINSPECT'::character varying, 'HOLD'::character varying])::text[]))),
    CONSTRAINT store_roll_passport_events_v7_evidence_class_check CHECK (((evidence_class)::text = 'OPERATOR_REPORTED'::text)),
    CONSTRAINT store_roll_passport_events_v7_inspection_result_check CHECK (((inspection_result)::text = ANY ((ARRAY['PASS'::character varying, 'HOLD'::character varying, 'REJECTED'::character varying])::text[]))),
    CONSTRAINT store_roll_passport_events_v7_measured_length_mm_check CHECK ((measured_length_mm > 0)),
    CONSTRAINT store_roll_passport_events_v7_revision_check CHECK ((revision > 0)),
    CONSTRAINT store_roll_passport_events_v7_roll_id_check CHECK (((roll_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$'::text)),
    CONSTRAINT store_roll_passport_events_v7_source_note_check CHECK (((length(TRIM(BOTH FROM source_note)) >= 8) AND (length(TRIM(BOTH FROM source_note)) <= 1000))),
    CONSTRAINT store_roll_passport_events_v7_usable_width_mm_check CHECK (((usable_width_mm >= 100) AND (usable_width_mm <= 4000)))
);

ALTER TABLE ONLY public.store_roll_passport_events_v7 FORCE ROW LEVEL SECURITY;


--
-- Name: store_roll_rfid_bindings_v7; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_roll_rfid_bindings_v7 (
    tenant_id character varying(64) NOT NULL,
    epc_hex character varying(64) NOT NULL,
    roll_id character varying(64) NOT NULL,
    bound_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT store_roll_rfid_bindings_v7_epc_hex_check CHECK ((((epc_hex)::text ~ '^[0-9A-F]{24,64}$'::text) AND ((length((epc_hex)::text) % 2) = 0))),
    CONSTRAINT store_roll_rfid_bindings_v7_roll_id_check CHECK (((roll_id)::text ~ '^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$'::text))
);

ALTER TABLE ONLY public.store_roll_rfid_bindings_v7 FORCE ROW LEVEL SECURITY;


--
-- Name: store_rolls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_rolls (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) DEFAULT 'PLANT_DEFAULT'::character varying NOT NULL,
    factory_sku character varying(128) NOT NULL,
    original_length_m numeric(14,3) NOT NULL,
    remaining_length_m numeric(14,3) NOT NULL,
    usable_width_cm numeric(6,2) NOT NULL,
    shade_band character varying(32) NOT NULL,
    defect_map_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    bin_location character varying(64) NOT NULL,
    status character varying(32) NOT NULL,
    reserved_order_id character varying(64),
    reserved_cut_no integer,
    aggregate_version bigint DEFAULT 0 NOT NULL,
    last_rfid_sync timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    inspection_status text DEFAULT 'UNVERIFIED'::text NOT NULL,
    CONSTRAINT store_rolls_inspection_state_v5 CHECK ((inspection_status = ANY (ARRAY['UNVERIFIED'::text, 'PASS'::text, 'HOLD'::text, 'REJECTED'::text]))),
    CONSTRAINT store_rolls_original_length_m_check CHECK ((original_length_m > (0)::numeric)),
    CONSTRAINT store_rolls_remaining_length_m_check CHECK ((remaining_length_m >= (0)::numeric)),
    CONSTRAINT store_rolls_reserved_cut_no_check CHECK (((reserved_cut_no IS NULL) OR (reserved_cut_no >= 0))),
    CONSTRAINT store_rolls_status_check CHECK (((status)::text = ANY ((ARRAY['AVAILABLE'::character varying, 'RESERVED'::character varying, 'ISSUED'::character varying, 'DEPLETED'::character varying, 'QUARANTINE'::character varying])::text[]))),
    CONSTRAINT store_rolls_usable_width_cm_check CHECK ((usable_width_cm > (0)::numeric))
);

ALTER TABLE ONLY public.store_rolls FORCE ROW LEVEL SECURITY;


--
-- Name: store_trims_v13; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_trims_v13 (
    tenant_id character varying(64) NOT NULL,
    id character varying(64) NOT NULL,
    plant_id character varying(64) DEFAULT 'PLANT_DEFAULT'::character varying NOT NULL,
    trim_code character varying(64) NOT NULL,
    description character varying(255) NOT NULL,
    supplier_lot character varying(128) NOT NULL,
    color_code character varying(64) DEFAULT ''::character varying NOT NULL,
    size_code character varying(32) DEFAULT ''::character varying NOT NULL,
    uom character varying(32) NOT NULL,
    received_qty numeric(12,3) NOT NULL,
    remaining_qty numeric(12,3) NOT NULL,
    status character varying(32) DEFAULT 'AVAILABLE'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    bin_location character varying(128),
    CONSTRAINT store_trims_conservation_v24 CHECK ((remaining_qty <= received_qty)),
    CONSTRAINT store_trims_v13_bin_location_check CHECK (((bin_location IS NULL) OR ((length(btrim((bin_location)::text)) >= 2) AND (length(btrim((bin_location)::text)) <= 128)))),
    CONSTRAINT store_trims_v13_received_qty_check CHECK ((received_qty > (0)::numeric)),
    CONSTRAINT store_trims_v13_remaining_qty_check CHECK ((remaining_qty >= (0)::numeric)),
    CONSTRAINT store_trims_v13_status_check CHECK (((status)::text = ANY ((ARRAY['AVAILABLE'::character varying, 'DEPLETED'::character varying, 'HOLD'::character varying, 'REJECTED'::character varying])::text[])))
);

ALTER TABLE ONLY public.store_trims_v13 FORCE ROW LEVEL SECURITY;


--
-- Name: style_assembly_dag; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.style_assembly_dag (
    id character varying(128) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    style_id character varying(64) NOT NULL,
    parent_part_id character varying(128) NOT NULL,
    child_part_id character varying(128) NOT NULL,
    operation_id character varying(64) NOT NULL,
    machine_type_req character varying(64) NOT NULL
);

ALTER TABLE ONLY public.style_assembly_dag FORCE ROW LEVEL SECURITY;


--
-- Name: style_parts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.style_parts (
    id character varying(128) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    style_id character varying(64) NOT NULL,
    part_code character varying(64) NOT NULL,
    part_name character varying(128) NOT NULL,
    sub_bundle_order integer DEFAULT 1 NOT NULL,
    material_sku character varying(128) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT style_parts_sub_bundle_order_check CHECK ((sub_bundle_order > 0))
);

ALTER TABLE ONLY public.style_parts FORCE ROW LEVEL SECURITY;


--
-- Name: styles_master; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.styles_master (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    style_code character varying(64) NOT NULL,
    category character varying(64) DEFAULT 'GARMENT'::character varying NOT NULL,
    fit_profile character varying(64) DEFAULT 'REGULAR_FIT'::character varying NOT NULL,
    target_sam numeric(6,3) DEFAULT 0.60 NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT styles_master_target_sam_check CHECK ((target_sam > (0)::numeric))
);

ALTER TABLE ONLY public.styles_master FORCE ROW LEVEL SECURITY;


--
-- Name: tenants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenants (
    id character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    unit_code character varying(64) DEFAULT 'Unit 001'::character varying NOT NULL,
    shift_start_hour integer DEFAULT 8 NOT NULL,
    default_bundle_size integer DEFAULT 20 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT tenants_default_bundle_size_check CHECK ((default_bundle_size > 0)),
    CONSTRAINT tenants_shift_start_hour_check CHECK (((shift_start_hour >= 0) AND (shift_start_hour <= 23)))
);


--
-- Name: trim_admission_proof_uses_v25; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trim_admission_proof_uses_v25 (
    tenant_id character varying(64) NOT NULL,
    proof_id character varying(255) NOT NULL,
    admission_id uuid NOT NULL,
    role character varying(12) NOT NULL,
    CONSTRAINT trim_admission_proof_uses_v25_role_check CHECK (((role)::text = ANY ((ARRAY['MANAGER'::character varying, 'INSPECTOR'::character varying])::text[])))
);

ALTER TABLE ONLY public.trim_admission_proof_uses_v25 FORCE ROW LEVEL SECURITY;


--
-- Name: trim_issuance_proof_uses_v24; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trim_issuance_proof_uses_v24 (
    tenant_id character varying(64) NOT NULL,
    proof_id character varying(255) NOT NULL,
    issuance_id uuid NOT NULL,
    witness_role character varying(16) NOT NULL,
    CONSTRAINT trim_issuance_proof_uses_v24_witness_role_check CHECK (((witness_role)::text = ANY ((ARRAY['ISSUER'::character varying, 'RECIPIENT'::character varying])::text[])))
);

ALTER TABLE ONLY public.trim_issuance_proof_uses_v24 FORCE ROW LEVEL SECURITY;


--
-- Name: trim_issuances_v13; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trim_issuances_v13 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    trim_stock_id character varying(64) NOT NULL,
    order_id character varying(64) NOT NULL,
    line_id character varying(64) NOT NULL,
    issued_qty numeric(12,3) NOT NULL,
    issuer_id character varying(64) NOT NULL,
    recipient_id character varying(64) NOT NULL,
    issued_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    plant_id character varying(64),
    command_id character varying(80),
    issuer_proof_id character varying(255),
    recipient_proof_id character varying(255),
    challenge_sha256 character(64),
    issuer_assertion_sha256 character(64),
    recipient_assertion_sha256 character(64),
    CONSTRAINT trim_issuances_v13_issued_qty_check CHECK ((issued_qty > (0)::numeric))
);

ALTER TABLE ONLY public.trim_issuances_v13 FORCE ROW LEVEL SECURITY;


--
-- Name: trim_stock_admissions_v25; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trim_stock_admissions_v25 (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    stock_id character varying(64) NOT NULL,
    trim_code character varying(64) NOT NULL,
    description character varying(255) NOT NULL,
    supplier_lot character varying(128) NOT NULL,
    color_code character varying(64) DEFAULT ''::character varying NOT NULL,
    size_code character varying(32) DEFAULT ''::character varying NOT NULL,
    uom character varying(32) NOT NULL,
    received_qty numeric(12,3) NOT NULL,
    bin_location character varying(128) NOT NULL,
    source_document_id character varying(128) NOT NULL,
    source_sha256 character(64) NOT NULL,
    command_id character varying(80) NOT NULL,
    manager_subject character varying(64) NOT NULL,
    inspector_subject character varying(64) NOT NULL,
    manager_issuer character varying(255) NOT NULL,
    inspector_issuer character varying(255) NOT NULL,
    manager_proof_id character varying(255) NOT NULL,
    inspector_proof_id character varying(255) NOT NULL,
    manager_assertion_sha256 character(64) NOT NULL,
    inspector_assertion_sha256 character(64) NOT NULL,
    challenge_sha256 character(64) NOT NULL,
    admitted_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority character varying(64) DEFAULT 'SIGNED_ADMISSION_DOCUMENT_NOT_CALIBRATED_MEASUREMENT'::character varying NOT NULL,
    CONSTRAINT trim_stock_admissions_v25_authority_check CHECK (((authority)::text = 'SIGNED_ADMISSION_DOCUMENT_NOT_CALIBRATED_MEASUREMENT'::text)),
    CONSTRAINT trim_stock_admissions_v25_bin_location_check CHECK (((length(btrim((bin_location)::text)) >= 2) AND (length(btrim((bin_location)::text)) <= 128))),
    CONSTRAINT trim_stock_admissions_v25_challenge_sha256_check CHECK ((challenge_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT trim_stock_admissions_v25_check CHECK ((((manager_subject)::text <> (inspector_subject)::text) AND ((manager_proof_id)::text <> (inspector_proof_id)::text))),
    CONSTRAINT trim_stock_admissions_v25_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT trim_stock_admissions_v25_inspector_assertion_sha256_check CHECK ((inspector_assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT trim_stock_admissions_v25_manager_assertion_sha256_check CHECK ((manager_assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT trim_stock_admissions_v25_received_qty_check CHECK ((received_qty > (0)::numeric)),
    CONSTRAINT trim_stock_admissions_v25_source_sha256_check CHECK ((source_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT trim_stock_admissions_v25_uom_check CHECK (((uom)::text ~ '^[A-Z][A-Z0-9_]{0,31}$'::text))
);

ALTER TABLE ONLY public.trim_stock_admissions_v25 FORCE ROW LEVEL SECURITY;


--
-- Name: wip_exception_cases_v23; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wip_exception_cases_v23 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    part_id uuid NOT NULL,
    station_id character varying(64) NOT NULL,
    issue_type text NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    reporter_subject character varying(128) NOT NULL,
    reporter_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    opened_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_HUMAN_EXCEPTION_REPORT_NOT_PHYSICAL_PROOF'::text NOT NULL,
    CONSTRAINT wip_exception_cases_v23_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_exception_cases_v23_authority_check CHECK ((authority = 'SIGNED_HUMAN_EXCEPTION_REPORT_NOT_PHYSICAL_PROOF'::text)),
    CONSTRAINT wip_exception_cases_v23_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT wip_exception_cases_v23_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_exception_cases_v23_issue_type_check CHECK ((issue_type = ANY (ARRAY['LOST_CARRIER'::text, 'UNREADABLE_CARRIER'::text, 'MISSING_WIP'::text, 'FAILED_SCAN'::text, 'MISROUTED_PART'::text, 'REJECTED_HANDOFF'::text]))),
    CONSTRAINT wip_exception_cases_v23_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 500)))
);

ALTER TABLE ONLY public.wip_exception_cases_v23 FORCE ROW LEVEL SECURITY;


--
-- Name: wip_exception_closures_v23; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wip_exception_closures_v23 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    case_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    part_id uuid NOT NULL,
    station_id character varying(64) NOT NULL,
    scan_id uuid NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    supervisor_subject character varying(128) NOT NULL,
    supervisor_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    closed_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_SUPERVISOR_REVIEW_WITH_NEW_GATEWAY_SCAN_NO_CUSTODY_CHANGE'::text NOT NULL,
    CONSTRAINT wip_exception_closures_v23_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_exception_closures_v23_authority_check CHECK ((authority = 'SIGNED_SUPERVISOR_REVIEW_WITH_NEW_GATEWAY_SCAN_NO_CUSTODY_CHANGE'::text)),
    CONSTRAINT wip_exception_closures_v23_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT wip_exception_closures_v23_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_exception_closures_v23_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 500)))
);

ALTER TABLE ONLY public.wip_exception_closures_v23 FORCE ROW LEVEL SECURITY;


--
-- Name: wip_gateway_scans_v20; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wip_gateway_scans_v20 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    station_id character varying(64) NOT NULL,
    gateway_id character varying(64) NOT NULL,
    event_id character varying(80) NOT NULL,
    part_id uuid NOT NULL,
    carrier_id uuid NOT NULL,
    binding_event_id uuid NOT NULL,
    epc_hex character varying(96) NOT NULL,
    observed_at timestamp with time zone NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    event_sha256 character(64) NOT NULL,
    signature_sha256 character(64) NOT NULL,
    evidence text DEFAULT 'SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'::text NOT NULL,
    CONSTRAINT wip_gateway_scans_v20_event_id_check CHECK (((event_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT wip_gateway_scans_v20_event_sha256_check CHECK ((event_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_gateway_scans_v20_evidence_check CHECK ((evidence = 'SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF'::text)),
    CONSTRAINT wip_gateway_scans_v20_signature_sha256_check CHECK ((signature_sha256 ~ '^[0-9a-f]{64}$'::text))
);

ALTER TABLE ONLY public.wip_gateway_scans_v20 FORCE ROW LEVEL SECURITY;


--
-- Name: wip_movements_v20; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wip_movements_v20 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    event_no bigint NOT NULL,
    plant_id character varying(64) NOT NULL,
    part_id uuid NOT NULL,
    route_id uuid NOT NULL,
    scan_id uuid NOT NULL,
    action character varying(8) NOT NULL,
    station_id character varying(64) NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    worker_subject character varying(128) NOT NULL,
    worker_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    evidence text DEFAULT 'SIGNED_SCAN_AND_WORKER_CUSTODY_ATTESTATION'::text NOT NULL,
    CONSTRAINT wip_movements_v20_action_check CHECK (((action)::text = ANY ((ARRAY['DEPART'::character varying, 'ARRIVE'::character varying])::text[]))),
    CONSTRAINT wip_movements_v20_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_movements_v20_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT wip_movements_v20_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_movements_v20_event_no_check CHECK ((event_no > 0)),
    CONSTRAINT wip_movements_v20_evidence_check CHECK ((evidence = 'SIGNED_SCAN_AND_WORKER_CUSTODY_ATTESTATION'::text))
);

ALTER TABLE ONLY public.wip_movements_v20 FORCE ROW LEVEL SECURITY;


--
-- Name: wip_piece_holds_v26; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wip_piece_holds_v26 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    garment_id uuid NOT NULL,
    station_id character varying(64) NOT NULL,
    reason text NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    reporter_subject character varying(128) NOT NULL,
    reporter_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    opened_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'SIGNED_ONE_GARMENT_HOLD_NOT_PHYSICAL_DEFECT_CERTIFICATION'::text NOT NULL,
    CONSTRAINT wip_piece_holds_v26_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_piece_holds_v26_authority_check CHECK ((authority = 'SIGNED_ONE_GARMENT_HOLD_NOT_PHYSICAL_DEFECT_CERTIFICATION'::text)),
    CONSTRAINT wip_piece_holds_v26_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT wip_piece_holds_v26_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_piece_holds_v26_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 500))),
    CONSTRAINT wip_piece_holds_v26_reason_check CHECK ((reason = ANY (ARRAY['SEWING_DEFECT'::text, 'MISSING_COMPONENT'::text, 'OIL_STAIN'::text, 'NEEDLE_DAMAGE'::text, 'REWORK_REQUIRED'::text])))
);

ALTER TABLE ONLY public.wip_piece_holds_v26 FORCE ROW LEVEL SECURITY;


--
-- Name: wip_piece_reviews_v26; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wip_piece_reviews_v26 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    hold_id uuid NOT NULL,
    plant_id character varying(64) NOT NULL,
    garment_id uuid NOT NULL,
    station_id character varying(64) NOT NULL,
    scan_id uuid NOT NULL,
    review_action text NOT NULL,
    note text NOT NULL,
    command_id character varying(80) NOT NULL,
    command_sha256 character(64) NOT NULL,
    supervisor_subject character varying(128) NOT NULL,
    supervisor_proof_id character varying(255) NOT NULL,
    assertion_sha256 character(64) NOT NULL,
    reviewed_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    authority text DEFAULT 'INDEPENDENT_SIGNED_REVIEW_AND_FRESH_GATEWAY_SCAN_NO_COMPLETION_CREDIT'::text NOT NULL,
    CONSTRAINT wip_piece_reviews_v26_assertion_sha256_check CHECK ((assertion_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_piece_reviews_v26_authority_check CHECK ((authority = 'INDEPENDENT_SIGNED_REVIEW_AND_FRESH_GATEWAY_SCAN_NO_COMPLETION_CREDIT'::text)),
    CONSTRAINT wip_piece_reviews_v26_command_id_check CHECK (((command_id)::text ~ '^[A-Za-z0-9_-]{12,80}$'::text)),
    CONSTRAINT wip_piece_reviews_v26_command_sha256_check CHECK ((command_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT wip_piece_reviews_v26_note_check CHECK (((length(btrim(note)) >= 12) AND (length(btrim(note)) <= 500))),
    CONSTRAINT wip_piece_reviews_v26_review_action_check CHECK ((review_action = 'SUPERVISOR_REVIEWED_FOR_WIP'::text))
);

ALTER TABLE ONLY public.wip_piece_reviews_v26 FORCE ROW LEVEL SECURITY;


--
-- Name: wip_transfer_routes_v20; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wip_transfer_routes_v20 (
    tenant_id character varying(64) NOT NULL,
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    plant_id character varying(64) NOT NULL,
    from_station character varying(64) NOT NULL,
    to_station character varying(64) NOT NULL,
    revision character varying(64) NOT NULL,
    approved_by character varying(128) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT wip_transfer_routes_v20_check CHECK (((from_station)::text <> (to_station)::text)),
    CONSTRAINT wip_transfer_routes_v20_revision_check CHECK ((length(btrim((revision)::text)) > 0))
);

ALTER TABLE ONLY public.wip_transfer_routes_v20 FORCE ROW LEVEL SECURITY;


--
-- Name: work_orders; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.work_orders AS
 SELECT dwo.id,
    dwo.tenant_id,
    dwo.id AS order_id,
    dwo.po_number,
    sm.style_code,
    dwo.line_id,
    dwo.supervisor_id,
    dwo.ordered_qty AS target_qty,
    0 AS completed_qty,
    dwo.status,
    dwo.started_at
   FROM (public.dynamic_work_orders dwo
     JOIN public.styles_master sm ON (((dwo.style_id)::text = (sm.id)::text)));


--
-- Name: workers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workers (
    id character varying(64) NOT NULL,
    tenant_id character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    employee_code character varying(64) NOT NULL,
    primary_skill character varying(64) NOT NULL,
    secondary_skill character varying(64) DEFAULT ''::character varying NOT NULL,
    skill_grade character varying(16) DEFAULT 'A'::character varying NOT NULL,
    hourly_rate_inr numeric(10,2) DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.workers FORCE ROW LEVEL SECURITY;


--
-- Name: workforce_browser_sessions_v16; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workforce_browser_sessions_v16 (
    session_hash bytea NOT NULL,
    csrf_hash bytea NOT NULL,
    idp_issuer text NOT NULL,
    idp_subject character varying(128) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    CONSTRAINT workforce_browser_sessions_v16_check CHECK ((expires_at > created_at)),
    CONSTRAINT workforce_browser_sessions_v16_csrf_hash_check CHECK ((octet_length(csrf_hash) = 32)),
    CONSTRAINT workforce_browser_sessions_v16_idp_issuer_check CHECK ((idp_issuer ~~ 'https://%'::text)),
    CONSTRAINT workforce_browser_sessions_v16_idp_subject_check CHECK ((length((idp_subject)::text) > 0)),
    CONSTRAINT workforce_browser_sessions_v16_session_hash_check CHECK ((octet_length(session_hash) = 32))
);

ALTER TABLE ONLY public.workforce_browser_sessions_v16 FORCE ROW LEVEL SECURITY;


--
-- Name: workforce_chapter_actions_v17; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workforce_chapter_actions_v17 (
    chapter_id smallint NOT NULL,
    permission character varying(80) NOT NULL,
    authority character varying(16) NOT NULL,
    CONSTRAINT workforce_chapter_actions_v17_authority_check CHECK (((authority)::text = ANY ((ARRAY['READ'::character varying, 'SIGNED_READ'::character varying, 'SIGNED_WRITE'::character varying])::text[]))),
    CONSTRAINT workforce_chapter_actions_v17_chapter_id_check CHECK (((chapter_id >= 1) AND (chapter_id <= 10))),
    CONSTRAINT workforce_chapter_actions_v17_permission_check CHECK (((permission)::text ~ '^[a-z][a-z0-9_.-]{2,79}$'::text))
);


--
-- Name: workforce_identity_grants_v15; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workforce_identity_grants_v15 (
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    idp_issuer character varying(512) NOT NULL,
    idp_subject character varying(128) NOT NULL,
    permission character varying(80) NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    approved_by character varying(128) NOT NULL,
    approved_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT workforce_identity_grants_v15_approved_by_check CHECK ((length(TRIM(BOTH FROM approved_by)) > 0)),
    CONSTRAINT workforce_identity_grants_v15_idp_issuer_check CHECK (((idp_issuer)::text ~~ 'https://%'::text)),
    CONSTRAINT workforce_identity_grants_v15_idp_subject_check CHECK ((length(TRIM(BOTH FROM idp_subject)) > 0)),
    CONSTRAINT workforce_identity_grants_v15_permission_check CHECK (((permission)::text ~ '^[a-z][a-z0-9_.-]{2,79}$'::text))
);

ALTER TABLE ONLY public.workforce_identity_grants_v15 FORCE ROW LEVEL SECURITY;


--
-- Name: workforce_oidc_requests_v16; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workforce_oidc_requests_v16 (
    state_hash bytea NOT NULL,
    browser_hash bytea NOT NULL,
    nonce text NOT NULL,
    pkce_verifier text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    CONSTRAINT workforce_oidc_requests_v16_browser_hash_check CHECK ((octet_length(browser_hash) = 32)),
    CONSTRAINT workforce_oidc_requests_v16_check CHECK ((expires_at > created_at)),
    CONSTRAINT workforce_oidc_requests_v16_nonce_check CHECK (((length(nonce) >= 32) AND (length(nonce) <= 128))),
    CONSTRAINT workforce_oidc_requests_v16_pkce_verifier_check CHECK (((length(pkce_verifier) >= 43) AND (length(pkce_verifier) <= 128))),
    CONSTRAINT workforce_oidc_requests_v16_state_hash_check CHECK ((octet_length(state_hash) = 32))
);

ALTER TABLE ONLY public.workforce_oidc_requests_v16 FORCE ROW LEVEL SECURITY;


--
-- Name: plants; Type: TABLE; Schema: s01_identity; Owner: -
--

CREATE TABLE s01_identity.plants (
    id character varying(64) DEFAULT (gen_random_uuid())::text NOT NULL,
    tenant_id character varying(64) NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    timezone character varying(64) DEFAULT 'UTC'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: role_permissions; Type: TABLE; Schema: s01_identity; Owner: -
--

CREATE TABLE s01_identity.role_permissions (
    role_id character varying(64) NOT NULL,
    permission character varying(64) NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: s01_identity; Owner: -
--

CREATE TABLE s01_identity.roles (
    id character varying(64) DEFAULT (gen_random_uuid())::text NOT NULL,
    tenant_id character varying(64) NOT NULL,
    name character varying(64) NOT NULL,
    description text
);


--
-- Name: tenants; Type: TABLE; Schema: s01_identity; Owner: -
--

CREATE TABLE s01_identity.tenants (
    id character varying(64) DEFAULT (gen_random_uuid())::text NOT NULL,
    name character varying(255) NOT NULL,
    slug character varying(64) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: user_role_grants; Type: TABLE; Schema: s01_identity; Owner: -
--

CREATE TABLE s01_identity.user_role_grants (
    id character varying(64) DEFAULT (gen_random_uuid())::text NOT NULL,
    tenant_id character varying(64) NOT NULL,
    user_id character varying(64) NOT NULL,
    role_id character varying(64) NOT NULL,
    plant_id character varying(64),
    granted_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: s01_identity; Owner: -
--

CREATE TABLE s01_identity.users (
    id character varying(64) DEFAULT (gen_random_uuid())::text NOT NULL,
    tenant_id character varying(64) NOT NULL,
    email character varying(255) NOT NULL,
    full_name character varying(255) NOT NULL,
    pin_hash character varying(256),
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: registered_devices; Type: TABLE; Schema: s02_device; Owner: -
--

CREATE TABLE s02_device.registered_devices (
    id character varying(64) DEFAULT (gen_random_uuid())::text NOT NULL,
    tenant_id character varying(64) NOT NULL,
    plant_id character varying(64) NOT NULL,
    device_uid character varying(128) NOT NULL,
    device_type character varying(64) NOT NULL,
    cert_fingerprint character varying(128) NOT NULL,
    public_key_pem text NOT NULL,
    station_id character varying(64),
    status character varying(32) DEFAULT 'ENROLLED'::character varying NOT NULL,
    last_ping_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT registered_devices_device_type_check CHECK (((device_type)::text = ANY ((ARRAY['PHONE_HMI'::character varying, 'TABLET_KIOSK'::character varying, 'RFID_SLED'::character varying, 'BT_RING_SCANNER'::character varying, 'DIGITAL_SCALE'::character varying, 'EDGE_SERVER'::character varying])::text[]))),
    CONSTRAINT registered_devices_status_check CHECK (((status)::text = ANY ((ARRAY['ENROLLED'::character varying, 'ACTIVE'::character varying, 'REVOKED'::character varying, 'OFFLINE'::character varying])::text[])))
);


--
-- Name: attendance_register attendance_register_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attendance_register
    ADD CONSTRAINT attendance_register_pkey PRIMARY KEY (id);


--
-- Name: attendance_register attendance_register_tenant_id_worker_id_shift_date_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attendance_register
    ADD CONSTRAINT attendance_register_tenant_id_worker_id_shift_date_key UNIQUE (tenant_id, worker_id, shift_date);


--
-- Name: bom_trim_requirements bom_trim_requirements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bom_trim_requirements
    ADD CONSTRAINT bom_trim_requirements_pkey PRIMARY KEY (id);


--
-- Name: bundle_registry bundle_registry_active_nfc_uid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bundle_registry
    ADD CONSTRAINT bundle_registry_active_nfc_uid_key UNIQUE (active_nfc_uid);


--
-- Name: bundle_registry bundle_registry_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bundle_registry
    ADD CONSTRAINT bundle_registry_pkey PRIMARY KEY (id);


--
-- Name: buyer_aql_plans_v32 buyer_aql_plans_v32_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_plans_v32
    ADD CONSTRAINT buyer_aql_plans_v32_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: buyer_aql_plans_v32 buyer_aql_plans_v32_tenant_id_plant_id_lot_code_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_plans_v32
    ADD CONSTRAINT buyer_aql_plans_v32_tenant_id_plant_id_lot_code_revision_key UNIQUE (tenant_id, plant_id, lot_code, revision);


--
-- Name: buyer_aql_review_rules_v33 buyer_aql_review_rules_v33_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_review_rules_v33
    ADD CONSTRAINT buyer_aql_review_rules_v33_pkey PRIMARY KEY (tenant_id, plan_id);


--
-- Name: buyer_aql_reviews_v33 buyer_aql_reviews_v33_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_reviews_v33
    ADD CONSTRAINT buyer_aql_reviews_v33_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: buyer_aql_reviews_v33 buyer_aql_reviews_v33_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_reviews_v33
    ADD CONSTRAINT buyer_aql_reviews_v33_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: buyer_aql_reviews_v33 buyer_aql_reviews_v33_tenant_id_plan_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_reviews_v33
    ADD CONSTRAINT buyer_aql_reviews_v33_tenant_id_plan_id_key UNIQUE (tenant_id, plan_id);


--
-- Name: buyer_aql_reviews_v33 buyer_aql_reviews_v33_tenant_id_reviewer_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_reviews_v33
    ADD CONSTRAINT buyer_aql_reviews_v33_tenant_id_reviewer_proof_id_key UNIQUE (tenant_id, reviewer_proof_id);


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_events_v32_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_sample_events_v32
    ADD CONSTRAINT buyer_aql_sample_events_v32_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_events_v32_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_sample_events_v32
    ADD CONSTRAINT buyer_aql_sample_events_v32_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_events_v32_tenant_id_inspector_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_sample_events_v32
    ADD CONSTRAINT buyer_aql_sample_events_v32_tenant_id_inspector_proof_id_key UNIQUE (tenant_id, inspector_proof_id);


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_events_v32_tenant_id_plan_id_garment_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_sample_events_v32
    ADD CONSTRAINT buyer_aql_sample_events_v32_tenant_id_plan_id_garment_id_key UNIQUE (tenant_id, plan_id, garment_id);


--
-- Name: buyer_order_handoffs_v65c1 buyer_order_handoffs_v65c1_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_order_handoffs_v65c1
    ADD CONSTRAINT buyer_order_handoffs_v65c1_pkey PRIMARY KEY (id);


--
-- Name: buyer_order_handoffs_v65c1 buyer_order_handoffs_v65c1_tenant_id_source_digest_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_order_handoffs_v65c1
    ADD CONSTRAINT buyer_order_handoffs_v65c1_tenant_id_source_digest_key UNIQUE (tenant_id, source_digest);


--
-- Name: carton_pack_manifests carton_pack_manifests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.carton_pack_manifests
    ADD CONSTRAINT carton_pack_manifests_pkey PRIMARY KEY (id);


--
-- Name: cut_bundle_drafts_v8 cut_bundle_drafts_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_drafts_v8
    ADD CONSTRAINT cut_bundle_drafts_v8_pkey PRIMARY KEY (tenant_id, lay_draft_id, marker_position, part_code, ply_group);


--
-- Name: cut_bundle_drafts_v8 cut_bundle_drafts_v8_tenant_id_label_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_drafts_v8
    ADD CONSTRAINT cut_bundle_drafts_v8_tenant_id_label_code_key UNIQUE (tenant_id, label_code);


--
-- Name: cut_bundle_scan_witnesses_v8 cut_bundle_scan_witnesses_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8
    ADD CONSTRAINT cut_bundle_scan_witnesses_v8_pkey PRIMARY KEY (id);


--
-- Name: cut_bundle_scan_witnesses_v8 cut_bundle_scan_witnesses_v8_tenant_id_epc_hex_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8
    ADD CONSTRAINT cut_bundle_scan_witnesses_v8_tenant_id_epc_hex_key UNIQUE (tenant_id, epc_hex);


--
-- Name: cut_bundle_scan_witnesses_v8 cut_bundle_scan_witnesses_v8_tenant_id_lay_draft_id_label_c_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8
    ADD CONSTRAINT cut_bundle_scan_witnesses_v8_tenant_id_lay_draft_id_label_c_key UNIQUE (tenant_id, lay_draft_id, label_code);


--
-- Name: cut_bundle_scan_witnesses_v8 cut_bundle_scan_witnesses_v8_tenant_id_scanner_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8
    ADD CONSTRAINT cut_bundle_scan_witnesses_v8_tenant_id_scanner_proof_id_key UNIQUE (tenant_id, scanner_proof_id);


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_reconciliations__tenant_id_spread_report_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19
    ADD CONSTRAINT cut_consumption_reconciliations__tenant_id_spread_report_id_key UNIQUE (tenant_id, spread_report_id);


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_reconciliations_v19_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19
    ADD CONSTRAINT cut_consumption_reconciliations_v19_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_reconciliations_v19_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19
    ADD CONSTRAINT cut_consumption_reconciliations_v19_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_reconciliations_v19_tenant_id_release_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19
    ADD CONSTRAINT cut_consumption_reconciliations_v19_tenant_id_release_id_key UNIQUE (tenant_id, release_id);


--
-- Name: cut_defect_codes_v19 cut_defect_codes_v19_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_defect_codes_v19
    ADD CONSTRAINT cut_defect_codes_v19_pkey PRIMARY KEY (tenant_id, plant_id, code);


--
-- Name: cut_lay_drafts_v8 cut_lay_drafts_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_drafts_v8
    ADD CONSTRAINT cut_lay_drafts_v8_pkey PRIMARY KEY (id);


--
-- Name: cut_lay_drafts_v8 cut_lay_drafts_v8_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_drafts_v8
    ADD CONSTRAINT cut_lay_drafts_v8_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: cut_lay_drafts_v8 cut_lay_drafts_v8_tenant_id_intake_id_cut_no_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_drafts_v8
    ADD CONSTRAINT cut_lay_drafts_v8_tenant_id_intake_id_cut_no_revision_key UNIQUE (tenant_id, intake_id, cut_no, revision);


--
-- Name: cut_lay_drafts_v8 cut_lay_drafts_v8_tenant_id_request_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_drafts_v8
    ADD CONSTRAINT cut_lay_drafts_v8_tenant_id_request_key_key UNIQUE (tenant_id, request_key);


--
-- Name: cut_lay_positions_v8 cut_lay_positions_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_positions_v8
    ADD CONSTRAINT cut_lay_positions_v8_pkey PRIMARY KEY (tenant_id, lay_draft_id, marker_position);


--
-- Name: cut_lay_rolls cut_lay_rolls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_rolls
    ADD CONSTRAINT cut_lay_rolls_pkey PRIMARY KEY (id);


--
-- Name: cut_lay_snapshots_v8 cut_lay_snapshots_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_snapshots_v8
    ADD CONSTRAINT cut_lay_snapshots_v8_pkey PRIMARY KEY (tenant_id, lay_draft_id);


--
-- Name: cut_part_outcomes_v19 cut_part_outcomes_v19_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_part_outcomes_v19
    ADD CONSTRAINT cut_part_outcomes_v19_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: cut_part_outcomes_v19 cut_part_outcomes_v19_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_part_outcomes_v19
    ADD CONSTRAINT cut_part_outcomes_v19_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: cut_part_outcomes_v19 cut_part_outcomes_v19_tenant_id_part_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_part_outcomes_v19
    ADD CONSTRAINT cut_part_outcomes_v19_tenant_id_part_id_key UNIQUE (tenant_id, part_id);


--
-- Name: cut_proof_consumptions_v19 cut_proof_consumptions_v19_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_proof_consumptions_v19
    ADD CONSTRAINT cut_proof_consumptions_v19_pkey PRIMARY KEY (tenant_id, proof_id);


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_pkey PRIMARY KEY (id);


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_tenant_id_gateway_id_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_tenant_id_gateway_id_event_id_key UNIQUE (tenant_id, gateway_id, event_id);


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_tenant_id_spread_report_id_epc_hex_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_tenant_id_spread_report_id_epc_hex_key UNIQUE (tenant_id, spread_report_id, epc_hex);


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_tenant_id_spread_report_id_label_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_tenant_id_spread_report_id_label_code_key UNIQUE (tenant_id, spread_report_id, label_code);


--
-- Name: cut_reader_gateways_v8 cut_reader_gateways_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_gateways_v8
    ADD CONSTRAINT cut_reader_gateways_v8_pkey PRIMARY KEY (tenant_id, gateway_id);


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_tenant_id_original_part_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_tenant_id_original_part_id_key UNIQUE (tenant_id, original_part_id);


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_tenant_id_replacement_part_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_tenant_id_replacement_part_id_key UNIQUE (tenant_id, replacement_part_id);


--
-- Name: cut_sewing_releases_v8 cut_sewing_releases_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_sewing_releases_v8
    ADD CONSTRAINT cut_sewing_releases_v8_pkey PRIMARY KEY (id);


--
-- Name: cut_sewing_releases_v8 cut_sewing_releases_v8_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_sewing_releases_v8
    ADD CONSTRAINT cut_sewing_releases_v8_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: cut_sewing_releases_v8 cut_sewing_releases_v8_tenant_id_quality_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_sewing_releases_v8
    ADD CONSTRAINT cut_sewing_releases_v8_tenant_id_quality_proof_id_key UNIQUE (tenant_id, quality_proof_id);


--
-- Name: cut_sewing_releases_v8 cut_sewing_releases_v8_tenant_id_spread_report_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_sewing_releases_v8
    ADD CONSTRAINT cut_sewing_releases_v8_tenant_id_spread_report_id_key UNIQUE (tenant_id, spread_report_id);


--
-- Name: cut_sewing_releases_v8 cut_sewing_releases_v8_tenant_id_supervisor_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_sewing_releases_v8
    ADD CONSTRAINT cut_sewing_releases_v8_tenant_id_supervisor_proof_id_key UNIQUE (tenant_id, supervisor_proof_id);


--
-- Name: cut_spread_issue_allocations_v8 cut_spread_issue_allocations_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_issue_allocations_v8
    ADD CONSTRAINT cut_spread_issue_allocations_v8_pkey PRIMARY KEY (tenant_id, spread_report_id, issue_id);


--
-- Name: cut_spread_reports_v8 cut_spread_reports_v8_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_reports_v8
    ADD CONSTRAINT cut_spread_reports_v8_pkey PRIMARY KEY (id);


--
-- Name: cut_spread_reports_v8 cut_spread_reports_v8_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_reports_v8
    ADD CONSTRAINT cut_spread_reports_v8_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: cut_spread_reports_v8 cut_spread_reports_v8_tenant_id_lay_draft_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_reports_v8
    ADD CONSTRAINT cut_spread_reports_v8_tenant_id_lay_draft_id_key UNIQUE (tenant_id, lay_draft_id);


--
-- Name: cut_spread_reports_v8 cut_spread_reports_v8_tenant_id_request_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_reports_v8
    ADD CONSTRAINT cut_spread_reports_v8_tenant_id_request_key_key UNIQUE (tenant_id, request_key);


--
-- Name: dock_asn_drafts_v34 dock_asn_drafts_v34_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_asn_drafts_v34
    ADD CONSTRAINT dock_asn_drafts_v34_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: dock_asn_drafts_v34 dock_asn_drafts_v34_tenant_id_asn_reference_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_asn_drafts_v34
    ADD CONSTRAINT dock_asn_drafts_v34_tenant_id_asn_reference_key UNIQUE (tenant_id, asn_reference);


--
-- Name: dock_asn_drafts_v34 dock_asn_drafts_v34_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_asn_drafts_v34
    ADD CONSTRAINT dock_asn_drafts_v34_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: dock_asn_drafts_v34 dock_asn_drafts_v34_tenant_id_manifest_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_asn_drafts_v34
    ADD CONSTRAINT dock_asn_drafts_v34_tenant_id_manifest_id_key UNIQUE (tenant_id, manifest_id);


--
-- Name: dock_asn_drafts_v34 dock_asn_drafts_v34_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_asn_drafts_v34
    ADD CONSTRAINT dock_asn_drafts_v34_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: dock_buyer_spec_bindings_v36 dock_buyer_spec_bindings_v36_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_buyer_spec_bindings_v36
    ADD CONSTRAINT dock_buyer_spec_bindings_v36_pkey PRIMARY KEY (tenant_id, plant_id, spec_id);


--
-- Name: dock_carrier_authorizations_v34 dock_carrier_authorizations_v34_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_authorizations_v34
    ADD CONSTRAINT dock_carrier_authorizations_v34_pkey PRIMARY KEY (tenant_id, plant_id, carrier_code, idp_issuer, idp_subject);


--
-- Name: dock_carrier_receipts_v34 dock_carrier_receipts_v34_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_receipts_v34
    ADD CONSTRAINT dock_carrier_receipts_v34_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: dock_carrier_receipts_v34 dock_carrier_receipts_v34_tenant_id_carrier_code_receipt_re_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_receipts_v34
    ADD CONSTRAINT dock_carrier_receipts_v34_tenant_id_carrier_code_receipt_re_key UNIQUE (tenant_id, carrier_code, receipt_reference);


--
-- Name: dock_carrier_receipts_v34 dock_carrier_receipts_v34_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_receipts_v34
    ADD CONSTRAINT dock_carrier_receipts_v34_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: dock_carrier_receipts_v34 dock_carrier_receipts_v34_tenant_id_manifest_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_receipts_v34
    ADD CONSTRAINT dock_carrier_receipts_v34_tenant_id_manifest_id_key UNIQUE (tenant_id, manifest_id);


--
-- Name: dock_carrier_receipts_v34 dock_carrier_receipts_v34_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_receipts_v34
    ADD CONSTRAINT dock_carrier_receipts_v34_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: dock_carton_scans_v34 dock_carton_scans_v34_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carton_scans_v34
    ADD CONSTRAINT dock_carton_scans_v34_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: dock_carton_scans_v34 dock_carton_scans_v34_tenant_id_gateway_id_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carton_scans_v34
    ADD CONSTRAINT dock_carton_scans_v34_tenant_id_gateway_id_event_id_key UNIQUE (tenant_id, gateway_id, event_id);


--
-- Name: dock_carton_scans_v34 dock_carton_scans_v34_tenant_id_manifest_id_carton_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carton_scans_v34
    ADD CONSTRAINT dock_carton_scans_v34_tenant_id_manifest_id_carton_id_key UNIQUE (tenant_id, manifest_id, carton_id);


--
-- Name: dock_container_seals_v34 dock_container_seals_v34_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_container_seals_v34
    ADD CONSTRAINT dock_container_seals_v34_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: dock_container_seals_v34 dock_container_seals_v34_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_container_seals_v34
    ADD CONSTRAINT dock_container_seals_v34_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: dock_container_seals_v34 dock_container_seals_v34_tenant_id_manifest_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_container_seals_v34
    ADD CONSTRAINT dock_container_seals_v34_tenant_id_manifest_id_key UNIQUE (tenant_id, manifest_id);


--
-- Name: dock_container_seals_v34 dock_container_seals_v34_tenant_id_plant_id_container_ref_s_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_container_seals_v34
    ADD CONSTRAINT dock_container_seals_v34_tenant_id_plant_id_container_ref_s_key UNIQUE (tenant_id, plant_id, container_ref, seal_code);


--
-- Name: dock_container_seals_v34 dock_container_seals_v34_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_container_seals_v34
    ADD CONSTRAINT dock_container_seals_v34_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: dock_external_assertions_v35 dock_external_assertions_v35_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_assertions_v35
    ADD CONSTRAINT dock_external_assertions_v35_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: dock_external_assertions_v35 dock_external_assertions_v35_tenant_id_plant_id_manifest_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_assertions_v35
    ADD CONSTRAINT dock_external_assertions_v35_tenant_id_plant_id_manifest_id_key UNIQUE (tenant_id, plant_id, manifest_id, assertion_kind);


--
-- Name: dock_external_assertions_v35 dock_external_assertions_v35_tenant_id_plant_id_signer_id_e_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_assertions_v35
    ADD CONSTRAINT dock_external_assertions_v35_tenant_id_plant_id_signer_id_e_key UNIQUE (tenant_id, plant_id, signer_id, event_id);


--
-- Name: dock_external_signers_v35 dock_external_signers_v35_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_signers_v35
    ADD CONSTRAINT dock_external_signers_v35_pkey PRIMARY KEY (tenant_id, plant_id, signer_id);


--
-- Name: dock_external_signers_v35 dock_external_signers_v35_tenant_id_plant_id_party_kind_pub_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_signers_v35
    ADD CONSTRAINT dock_external_signers_v35_tenant_id_plant_id_party_kind_pub_key UNIQUE (tenant_id, plant_id, party_kind, public_key);


--
-- Name: dock_gateways_v34 dock_gateways_v34_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_gateways_v34
    ADD CONSTRAINT dock_gateways_v34_pkey PRIMARY KEY (tenant_id, gateway_id);


--
-- Name: dock_station_policies_v34 dock_station_policies_v34_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_station_policies_v34
    ADD CONSTRAINT dock_station_policies_v34_pkey PRIMARY KEY (tenant_id, plant_id);


--
-- Name: document_assertions_v65 document_assertions_v65_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_assertions_v65
    ADD CONSTRAINT document_assertions_v65_pkey PRIMARY KEY (tenant_id, document_id, assertion_id);


--
-- Name: document_jobs_v65 document_jobs_v65_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_jobs_v65
    ADD CONSTRAINT document_jobs_v65_pkey PRIMARY KEY (tenant_id, document_id);


--
-- Name: document_segments_v65 document_segments_v65_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_segments_v65
    ADD CONSTRAINT document_segments_v65_pkey PRIMARY KEY (tenant_id, document_id, ordinal);


--
-- Name: document_sources_v65 document_sources_v65_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_sources_v65
    ADD CONSTRAINT document_sources_v65_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: document_sources_v65 document_sources_v65_tenant_id_sha256_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_sources_v65
    ADD CONSTRAINT document_sources_v65_tenant_id_sha256_key UNIQUE (tenant_id, sha256);


--
-- Name: documents documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_pkey PRIMARY KEY (id);


--
-- Name: dynamic_work_orders dynamic_work_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dynamic_work_orders
    ADD CONSTRAINT dynamic_work_orders_pkey PRIMARY KEY (id);


--
-- Name: fabric_calibration_links_v32 fabric_calibration_links_v32_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_calibration_links_v32
    ADD CONSTRAINT fabric_calibration_links_v32_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: fabric_calibration_links_v32 fabric_calibration_links_v32_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_calibration_links_v32
    ADD CONSTRAINT fabric_calibration_links_v32_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: fabric_calibration_links_v32 fabric_calibration_links_v32_tenant_id_inspector_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_calibration_links_v32
    ADD CONSTRAINT fabric_calibration_links_v32_tenant_id_inspector_proof_id_key UNIQUE (tenant_id, inspector_proof_id);


--
-- Name: fabric_calibration_links_v32 fabric_calibration_links_v32_tenant_id_observation_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_calibration_links_v32
    ADD CONSTRAINT fabric_calibration_links_v32_tenant_id_observation_id_key UNIQUE (tenant_id, observation_id);


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_observations_tenant_id_plant_id_roll_id_sa_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_observations_v30
    ADD CONSTRAINT fabric_shrinkage_observations_tenant_id_plant_id_roll_id_sa_key UNIQUE (tenant_id, plant_id, roll_id, sample_id, treatment_code);


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_observations_v30_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_observations_v30
    ADD CONSTRAINT fabric_shrinkage_observations_v30_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_observations_v30_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_observations_v30
    ADD CONSTRAINT fabric_shrinkage_observations_v30_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_observations_v30_tenant_id_tester_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_observations_v30
    ADD CONSTRAINT fabric_shrinkage_observations_v30_tenant_id_tester_proof_id_key UNIQUE (tenant_id, tester_proof_id);


--
-- Name: fabric_shrinkage_specs_v31 fabric_shrinkage_specs_v31_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_specs_v31
    ADD CONSTRAINT fabric_shrinkage_specs_v31_pkey PRIMARY KEY (tenant_id, plant_id, material_sku, treatment_code, revision);


--
-- Name: factory_approved_reference_versions_v65b5 factory_approved_reference_versions__tenant_id_candidate_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_approved_reference_versions_v65b5
    ADD CONSTRAINT factory_approved_reference_versions__tenant_id_candidate_id_key UNIQUE (tenant_id, candidate_id);


--
-- Name: factory_approved_reference_versions_v65b5 factory_approved_reference_versions_v65b5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_approved_reference_versions_v65b5
    ADD CONSTRAINT factory_approved_reference_versions_v65b5_pkey PRIMARY KEY (tenant_id, entity_type, entity_key, revision);


--
-- Name: factory_candidate_review_versions_v65b5 factory_candidate_review_versions_v65b5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_candidate_review_versions_v65b5
    ADD CONSTRAINT factory_candidate_review_versions_v65b5_pkey PRIMARY KEY (tenant_id, candidate_id, revision);


--
-- Name: factory_candidate_reviews_v65b2 factory_candidate_reviews_v65b2_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_candidate_reviews_v65b2
    ADD CONSTRAINT factory_candidate_reviews_v65b2_pkey PRIMARY KEY (tenant_id, candidate_id);


--
-- Name: factory_document_candidates_v65b2 factory_document_candidates_v65b2_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_document_candidates_v65b2
    ADD CONSTRAINT factory_document_candidates_v65b2_pkey PRIMARY KEY (tenant_id, candidate_id);


--
-- Name: factory_document_candidates_v65b2 factory_document_candidates_v65b2_tenant_id_fingerprint_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_document_candidates_v65b2
    ADD CONSTRAINT factory_document_candidates_v65b2_tenant_id_fingerprint_key UNIQUE (tenant_id, fingerprint);


--
-- Name: factory_document_submissions_v65b5 factory_document_submissions_v65b5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_document_submissions_v65b5
    ADD CONSTRAINT factory_document_submissions_v65b5_pkey PRIMARY KEY (tenant_id, document_id);


--
-- Name: factory_document_submissions_v65b5 factory_document_submissions_v65b5_tenant_id_submission_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_document_submissions_v65b5
    ADD CONSTRAINT factory_document_submissions_v65b5_tenant_id_submission_id_key UNIQUE (tenant_id, submission_id);


--
-- Name: factory_draft_decisions_v65b3 factory_draft_decisions_v65b3_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_draft_decisions_v65b3
    ADD CONSTRAINT factory_draft_decisions_v65b3_pkey PRIMARY KEY (tenant_id, entity_type, entity_key, revision);


--
-- Name: factory_fact_events_v65b factory_fact_events_v65b_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_fact_events_v65b
    ADD CONSTRAINT factory_fact_events_v65b_pkey PRIMARY KEY (tenant_id, field_key, revision);


--
-- Name: factory_instrument_calibrations_v32 factory_instrument_calibrations_v32_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_instrument_calibrations_v32
    ADD CONSTRAINT factory_instrument_calibrations_v32_pkey PRIMARY KEY (tenant_id, plant_id, instrument_id, revision);


--
-- Name: factory_proposed_register_v65b2 factory_proposed_register_v65b2_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_proposed_register_v65b2
    ADD CONSTRAINT factory_proposed_register_v65b2_pkey PRIMARY KEY (tenant_id, candidate_id);


--
-- Name: factory_reference_versions_v65b4 factory_reference_versions_v65b4_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_reference_versions_v65b4
    ADD CONSTRAINT factory_reference_versions_v65b4_pkey PRIMARY KEY (tenant_id, entity_type, entity_key, revision);


--
-- Name: floor_issues floor_issues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.floor_issues
    ADD CONSTRAINT floor_issues_pkey PRIMARY KEY (id);


--
-- Name: gen_bundles_v18 gen_bundles_v18_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_bundles_v18
    ADD CONSTRAINT gen_bundles_v18_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: gen_bundles_v18 gen_bundles_v18_tenant_id_lay_draft_id_label_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_bundles_v18
    ADD CONSTRAINT gen_bundles_v18_tenant_id_lay_draft_id_label_code_key UNIQUE (tenant_id, lay_draft_id, label_code);


--
-- Name: gen_carrier_active_v18 gen_carrier_active_v18_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_active_v18
    ADD CONSTRAINT gen_carrier_active_v18_pkey PRIMARY KEY (tenant_id, carrier_id);


--
-- Name: gen_carrier_active_v18 gen_carrier_active_v18_tenant_id_target_type_target_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_active_v18
    ADD CONSTRAINT gen_carrier_active_v18_tenant_id_target_type_target_id_key UNIQUE (tenant_id, target_type, target_id);


--
-- Name: gen_carrier_events_v18 gen_carrier_events_v18_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_events_v18
    ADD CONSTRAINT gen_carrier_events_v18_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: gen_carrier_events_v18 gen_carrier_events_v18_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_events_v18
    ADD CONSTRAINT gen_carrier_events_v18_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: gen_carrier_events_v18 gen_carrier_events_v18_tenant_id_event_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_events_v18
    ADD CONSTRAINT gen_carrier_events_v18_tenant_id_event_no_key UNIQUE (tenant_id, event_no);


--
-- Name: gen_carrier_events_v18 gen_carrier_events_v18_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_events_v18
    ADD CONSTRAINT gen_carrier_events_v18_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: gen_carriers_v18 gen_carriers_v18_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carriers_v18
    ADD CONSTRAINT gen_carriers_v18_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: gen_carriers_v18 gen_carriers_v18_tenant_id_epc_hex_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carriers_v18
    ADD CONSTRAINT gen_carriers_v18_tenant_id_epc_hex_key UNIQUE (tenant_id, epc_hex);


--
-- Name: gen_cut_parts_v18 gen_cut_parts_v18_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_cut_parts_v18
    ADD CONSTRAINT gen_cut_parts_v18_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: gen_cut_parts_v18 gen_cut_parts_v18_tenant_id_garment_id_bundle_id_part_slot_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_cut_parts_v18
    ADD CONSTRAINT gen_cut_parts_v18_tenant_id_garment_id_bundle_id_part_slot_key UNIQUE (tenant_id, garment_id, bundle_id, part_slot);


--
-- Name: gen_garments_v18 gen_garments_v18_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_garments_v18
    ADD CONSTRAINT gen_garments_v18_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: gen_garments_v18 gen_garments_v18_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_garments_v18
    ADD CONSTRAINT gen_garments_v18_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: gen_garments_v18 gen_garments_v18_tenant_id_lay_draft_id_marker_position_ply_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_garments_v18
    ADD CONSTRAINT gen_garments_v18_tenant_id_lay_draft_id_marker_position_ply_key UNIQUE (tenant_id, lay_draft_id, marker_position, ply_number);


--
-- Name: gen_garments_v18 gen_garments_v18_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_garments_v18
    ADD CONSTRAINT gen_garments_v18_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: hangtag_binds hangtag_binds_epc_sgtin96_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hangtag_binds
    ADD CONSTRAINT hangtag_binds_epc_sgtin96_key UNIQUE (epc_sgtin96);


--
-- Name: hangtag_binds hangtag_binds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hangtag_binds
    ADD CONSTRAINT hangtag_binds_pkey PRIMARY KEY (id);


--
-- Name: hourly_line_metrics hourly_line_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hourly_line_metrics
    ADD CONSTRAINT hourly_line_metrics_pkey PRIMARY KEY (id);


--
-- Name: hourly_line_metrics hourly_line_metrics_tenant_id_line_id_shift_date_hour_windo_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hourly_line_metrics
    ADD CONSTRAINT hourly_line_metrics_tenant_id_line_id_shift_date_hour_windo_key UNIQUE (tenant_id, line_id, shift_date, hour_window);


--
-- Name: intake_document_capture intake_document_capture_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intake_document_capture
    ADD CONSTRAINT intake_document_capture_pkey PRIMARY KEY (id);


--
-- Name: intake_proposed_fact intake_proposed_fact_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intake_proposed_fact
    ADD CONSTRAINT intake_proposed_fact_pkey PRIMARY KEY (capture_id, json_pointer);


--
-- Name: intel_sim_snapshots_v12 intel_sim_snapshots_v12_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intel_sim_snapshots_v12
    ADD CONSTRAINT intel_sim_snapshots_v12_pkey PRIMARY KEY (tenant_id, capture_id);


--
-- Name: intelligence_prod_readers_v13 intelligence_prod_readers_v13_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intelligence_prod_readers_v13
    ADD CONSTRAINT intelligence_prod_readers_v13_pkey PRIMARY KEY (tenant_id, plant_id, worker_subject);


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancies_v29_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancies_v29
    ADD CONSTRAINT jobwork_discrepancies_v29_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancies_v29_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancies_v29
    ADD CONSTRAINT jobwork_discrepancies_v29_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancies_v29_tenant_id_outward_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancies_v29
    ADD CONSTRAINT jobwork_discrepancies_v29_tenant_id_outward_id_key UNIQUE (tenant_id, outward_id);


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancies_v29_tenant_id_reporter_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancies_v29
    ADD CONSTRAINT jobwork_discrepancies_v29_tenant_id_reporter_proof_id_key UNIQUE (tenant_id, reporter_proof_id);


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutio_tenant_id_supervisor_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutio_tenant_id_supervisor_proof_id_key UNIQUE (tenant_id, supervisor_proof_id);


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutions_v31_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutions_v31_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutions_v31_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutions_v31_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutions_v3_tenant_id_discrepancy_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutions_v3_tenant_id_discrepancy_id_key UNIQUE (tenant_id, discrepancy_id);


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_outward_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_outward_id_key UNIQUE (tenant_id, outward_id);


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_receiver_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_receiver_proof_id_key UNIQUE (tenant_id, receiver_proof_id);


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_scan_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_scan_id_key UNIQUE (tenant_id, scan_id);


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_tenant_id_scan_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_tenant_id_scan_id_key UNIQUE (tenant_id, scan_id);


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_tenant_id_sender_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_tenant_id_sender_proof_id_key UNIQUE (tenant_id, sender_proof_id);


--
-- Name: jobwork_used_proofs_v28 jobwork_used_proofs_v28_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_used_proofs_v28
    ADD CONSTRAINT jobwork_used_proofs_v28_pkey PRIMARY KEY (tenant_id, proof_id);


--
-- Name: jobwork_used_scans_v28 jobwork_used_scans_v28_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_used_scans_v28
    ADD CONSTRAINT jobwork_used_scans_v28_pkey PRIMARY KEY (tenant_id, scan_id);


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_outward_id_ack_kind_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_outward_id_ack_kind_key UNIQUE (tenant_id, outward_id, ack_kind);


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_recorder_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_recorder_proof_id_key UNIQUE (tenant_id, recorder_proof_id);


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_vendor_signature_hex_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_vendor_signature_hex_key UNIQUE (tenant_id, vendor_signature_hex);


--
-- Name: jobwork_vendor_keys_v30 jobwork_vendor_keys_v30_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_keys_v30
    ADD CONSTRAINT jobwork_vendor_keys_v30_pkey PRIMARY KEY (tenant_id, plant_id, vendor_id, key_id);


--
-- Name: jobwork_vendors_v28 jobwork_vendors_v28_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendors_v28
    ADD CONSTRAINT jobwork_vendors_v28_pkey PRIMARY KEY (tenant_id, plant_id, vendor_id);


--
-- Name: lines lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.lines
    ADD CONSTRAINT lines_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: machine_roster machine_roster_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.machine_roster
    ADD CONSTRAINT machine_roster_pkey PRIMARY KEY (id);


--
-- Name: material_ceo_principals_v5 material_ceo_principals_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_ceo_principals_v5
    ADD CONSTRAINT material_ceo_principals_v5_pkey PRIMARY KEY (tenant_id);


--
-- Name: material_issues_v5 material_issues_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_pkey PRIMARY KEY (id);


--
-- Name: material_issues_v5 material_issues_v5_tenant_id_command_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_tenant_id_command_key_key UNIQUE (tenant_id, command_key);


--
-- Name: material_issues_v5 material_issues_v5_tenant_id_issuer_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_tenant_id_issuer_proof_id_key UNIQUE (tenant_id, issuer_proof_id);


--
-- Name: material_issues_v5 material_issues_v5_tenant_id_recipient_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_tenant_id_recipient_proof_id_key UNIQUE (tenant_id, recipient_proof_id);


--
-- Name: material_outbox_v5 material_outbox_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_outbox_v5
    ADD CONSTRAINT material_outbox_v5_pkey PRIMARY KEY (id);


--
-- Name: material_outbox_v5 material_outbox_v5_tenant_id_event_kind_aggregate_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_outbox_v5
    ADD CONSTRAINT material_outbox_v5_tenant_id_event_kind_aggregate_id_key UNIQUE (tenant_id, event_kind, aggregate_id);


--
-- Name: material_reservations_v5 material_reservations_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_reservations_v5
    ADD CONSTRAINT material_reservations_v5_pkey PRIMARY KEY (id);


--
-- Name: material_reservations_v5 material_reservations_v5_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_reservations_v5
    ADD CONSTRAINT material_reservations_v5_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: material_reservations_v5 material_reservations_v5_tenant_id_request_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_reservations_v5
    ADD CONSTRAINT material_reservations_v5_tenant_id_request_key_key UNIQUE (tenant_id, request_key);


--
-- Name: material_scenarios_v6 material_scenarios_v6_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_scenarios_v6
    ADD CONSTRAINT material_scenarios_v6_pkey PRIMARY KEY (id);


--
-- Name: material_scenarios_v6 material_scenarios_v6_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_scenarios_v6
    ADD CONSTRAINT material_scenarios_v6_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: material_scenarios_v6 material_scenarios_v6_tenant_id_intake_id_scenario_sha256_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_scenarios_v6
    ADD CONSTRAINT material_scenarios_v6_tenant_id_intake_id_scenario_sha256_key UNIQUE (tenant_id, intake_id, scenario_sha256);


--
-- Name: material_used_proofs_v5 material_used_proofs_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_used_proofs_v5
    ADD CONSTRAINT material_used_proofs_v5_pkey PRIMARY KEY (tenant_id, proof_id);


--
-- Name: material_variances_v5 material_variances_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_variances_v5
    ADD CONSTRAINT material_variances_v5_pkey PRIMARY KEY (id);


--
-- Name: material_variances_v5 material_variances_v5_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_variances_v5
    ADD CONSTRAINT material_variances_v5_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: material_variances_v5 material_variances_v5_tenant_id_request_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_variances_v5
    ADD CONSTRAINT material_variances_v5_tenant_id_request_key_key UNIQUE (tenant_id, request_key);


--
-- Name: material_write_scopes_v5 material_write_scopes_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_write_scopes_v5
    ADD CONSTRAINT material_write_scopes_v5_pkey PRIMARY KEY (tenant_id, order_id);


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_incident_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_incident_id_key UNIQUE (tenant_id, incident_id);


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_reviewer_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_reviewer_proof_id_key UNIQUE (tenant_id, reviewer_proof_id);


--
-- Name: needle_safety_events_v32 needle_safety_events_v32_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_events_v32
    ADD CONSTRAINT needle_safety_events_v32_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: needle_safety_events_v32 needle_safety_events_v32_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_events_v32
    ADD CONSTRAINT needle_safety_events_v32_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: needle_safety_events_v32 needle_safety_events_v32_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_events_v32
    ADD CONSTRAINT needle_safety_events_v32_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: ocr_records ocr_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ocr_records
    ADD CONSTRAINT ocr_records_pkey PRIMARY KEY (id);


--
-- Name: operation_master operation_master_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operation_master
    ADD CONSTRAINT operation_master_pkey PRIMARY KEY (id);


--
-- Name: order_intake_bom_v5 order_intake_bom_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intake_bom_v5
    ADD CONSTRAINT order_intake_bom_v5_pkey PRIMARY KEY (tenant_id, intake_id, line_no);


--
-- Name: order_intake_bom_v5 order_intake_bom_v5_tenant_id_intake_id_material_sku_catego_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intake_bom_v5
    ADD CONSTRAINT order_intake_bom_v5_tenant_id_intake_id_material_sku_catego_key UNIQUE (tenant_id, intake_id, material_sku, category, uom);


--
-- Name: order_intake_skus_v5 order_intake_skus_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intake_skus_v5
    ADD CONSTRAINT order_intake_skus_v5_pkey PRIMARY KEY (tenant_id, intake_id, line_no);


--
-- Name: order_intake_skus_v5 order_intake_skus_v5_tenant_id_intake_id_color_code_size_co_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intake_skus_v5
    ADD CONSTRAINT order_intake_skus_v5_tenant_id_intake_id_color_code_size_co_key UNIQUE (tenant_id, intake_id, color_code, size_code);


--
-- Name: order_intakes_v5 order_intakes_v5_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intakes_v5
    ADD CONSTRAINT order_intakes_v5_pkey PRIMARY KEY (id);


--
-- Name: order_intakes_v5 order_intakes_v5_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intakes_v5
    ADD CONSTRAINT order_intakes_v5_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: order_intakes_v5 order_intakes_v5_tenant_id_source_sha256_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intakes_v5
    ADD CONSTRAINT order_intakes_v5_tenant_id_source_sha256_key UNIQUE (tenant_id, source_sha256);


--
-- Name: order_sku_matrix order_sku_matrix_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_sku_matrix
    ADD CONSTRAINT order_sku_matrix_pkey PRIMARY KEY (id);


--
-- Name: pack_carton_events_v22 pack_carton_events_v22_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_events_v22
    ADD CONSTRAINT pack_carton_events_v22_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: pack_carton_events_v22 pack_carton_events_v22_tenant_id_carton_id_event_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_events_v22
    ADD CONSTRAINT pack_carton_events_v22_tenant_id_carton_id_event_no_key UNIQUE (tenant_id, carton_id, event_no);


--
-- Name: pack_carton_events_v22 pack_carton_events_v22_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_events_v22
    ADD CONSTRAINT pack_carton_events_v22_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: pack_carton_events_v22 pack_carton_events_v22_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_events_v22
    ADD CONSTRAINT pack_carton_events_v22_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: pack_carton_specs_v22 pack_carton_specs_v22_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_specs_v22
    ADD CONSTRAINT pack_carton_specs_v22_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: pack_carton_specs_v22 pack_carton_specs_v22_tenant_id_lay_draft_id_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_specs_v22
    ADD CONSTRAINT pack_carton_specs_v22_tenant_id_lay_draft_id_revision_key UNIQUE (tenant_id, lay_draft_id, revision);


--
-- Name: pack_manifest_events_v22 pack_manifest_events_v22_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_manifest_events_v22
    ADD CONSTRAINT pack_manifest_events_v22_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: pack_manifest_events_v22 pack_manifest_events_v22_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_manifest_events_v22
    ADD CONSTRAINT pack_manifest_events_v22_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: pack_manifest_events_v22 pack_manifest_events_v22_tenant_id_manifest_id_event_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_manifest_events_v22
    ADD CONSTRAINT pack_manifest_events_v22_tenant_id_manifest_id_event_no_key UNIQUE (tenant_id, manifest_id, event_no);


--
-- Name: pack_manifest_events_v22 pack_manifest_events_v22_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_manifest_events_v22
    ADD CONSTRAINT pack_manifest_events_v22_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: pack_prod_assignments_v12 pack_prod_assignments_v12_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_assignments_v12
    ADD CONSTRAINT pack_prod_assignments_v12_pkey PRIMARY KEY (tenant_id, plant_id, station_id, worker_subject, role);


--
-- Name: pack_prod_holds_v12 pack_prod_holds_v12_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_holds_v12
    ADD CONSTRAINT pack_prod_holds_v12_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: pack_prod_holds_v12 pack_prod_holds_v12_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_holds_v12
    ADD CONSTRAINT pack_prod_holds_v12_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: pack_prod_holds_v12 pack_prod_holds_v12_tenant_id_completion_id_qc_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_holds_v12
    ADD CONSTRAINT pack_prod_holds_v12_tenant_id_completion_id_qc_revision_key UNIQUE (tenant_id, completion_id, qc_revision);


--
-- Name: pack_prod_holds_v12 pack_prod_holds_v12_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_holds_v12
    ADD CONSTRAINT pack_prod_holds_v12_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: pack_sim_cartons_v11 pack_sim_cartons_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_cartons_v11
    ADD CONSTRAINT pack_sim_cartons_v11_pkey PRIMARY KEY (tenant_id, carton_code);


--
-- Name: pack_sim_events_v11 pack_sim_events_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_events_v11
    ADD CONSTRAINT pack_sim_events_v11_pkey PRIMARY KEY (tenant_id, event_id);


--
-- Name: pack_sim_items_v11 pack_sim_items_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_items_v11
    ADD CONSTRAINT pack_sim_items_v11_pkey PRIMARY KEY (tenant_id, serial);


--
-- Name: pack_sim_items_v11 pack_sim_items_v11_tenant_id_buyer_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_items_v11
    ADD CONSTRAINT pack_sim_items_v11_tenant_id_buyer_code_key UNIQUE (tenant_id, buyer_code);


--
-- Name: pack_sim_manifest_cartons_v11 pack_sim_manifest_cartons_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_manifest_cartons_v11
    ADD CONSTRAINT pack_sim_manifest_cartons_v11_pkey PRIMARY KEY (tenant_id, carton_code);


--
-- Name: pack_sim_manifests_v11 pack_sim_manifests_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_manifests_v11
    ADD CONSTRAINT pack_sim_manifests_v11_pkey PRIMARY KEY (tenant_id, manifest_code);


--
-- Name: pack_size_weights_v22 pack_size_weights_v22_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_size_weights_v22
    ADD CONSTRAINT pack_size_weights_v22_pkey PRIMARY KEY (tenant_id, spec_id, size_code);


--
-- Name: plant_hardware_assets plant_hardware_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plant_hardware_assets
    ADD CONSTRAINT plant_hardware_assets_pkey PRIMARY KEY (id);


--
-- Name: post_sewing_processes post_sewing_processes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_sewing_processes
    ADD CONSTRAINT post_sewing_processes_pkey PRIMARY KEY (id);


--
-- Name: qc_garment_defects_v21 qc_garment_defects_v21_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_defects_v21
    ADD CONSTRAINT qc_garment_defects_v21_pkey PRIMARY KEY (tenant_id, plant_id, defect_code);


--
-- Name: qc_garment_events_v21 qc_garment_events_v21_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_events_v21
    ADD CONSTRAINT qc_garment_events_v21_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: qc_garment_events_v21 qc_garment_events_v21_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_events_v21
    ADD CONSTRAINT qc_garment_events_v21_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: qc_garment_events_v21 qc_garment_events_v21_tenant_id_garment_id_event_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_events_v21
    ADD CONSTRAINT qc_garment_events_v21_tenant_id_garment_id_event_no_key UNIQUE (tenant_id, garment_id, event_no);


--
-- Name: qc_garment_events_v21 qc_garment_events_v21_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_events_v21
    ADD CONSTRAINT qc_garment_events_v21_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: qc_garment_measure_rules_v21 qc_garment_measure_rules_v21_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_measure_rules_v21
    ADD CONSTRAINT qc_garment_measure_rules_v21_pkey PRIMARY KEY (tenant_id, spec_id, size_code, measure_code);


--
-- Name: qc_garment_specs_v21 qc_garment_specs_v21_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_specs_v21
    ADD CONSTRAINT qc_garment_specs_v21_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: qc_garment_specs_v21 qc_garment_specs_v21_tenant_id_lay_draft_id_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_specs_v21
    ADD CONSTRAINT qc_garment_specs_v21_tenant_id_lay_draft_id_revision_key UNIQUE (tenant_id, lay_draft_id, revision);


--
-- Name: qc_prod_assignments_v11 qc_prod_assignments_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_assignments_v11
    ADD CONSTRAINT qc_prod_assignments_v11_pkey PRIMARY KEY (tenant_id, plant_id, station_id, worker_subject, role);


--
-- Name: qc_prod_defect_catalog_v11 qc_prod_defect_catalog_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_defect_catalog_v11
    ADD CONSTRAINT qc_prod_defect_catalog_v11_pkey PRIMARY KEY (tenant_id, plant_id, code);


--
-- Name: qc_prod_group_events_v11 qc_prod_group_events_v11_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_group_events_v11
    ADD CONSTRAINT qc_prod_group_events_v11_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: qc_prod_group_events_v11 qc_prod_group_events_v11_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_group_events_v11
    ADD CONSTRAINT qc_prod_group_events_v11_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: qc_prod_group_events_v11 qc_prod_group_events_v11_tenant_id_completion_id_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_group_events_v11
    ADD CONSTRAINT qc_prod_group_events_v11_tenant_id_completion_id_revision_key UNIQUE (tenant_id, completion_id, revision);


--
-- Name: qc_prod_group_events_v11 qc_prod_group_events_v11_tenant_id_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_group_events_v11
    ADD CONSTRAINT qc_prod_group_events_v11_tenant_id_proof_id_key UNIQUE (tenant_id, proof_id);


--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_buyer_bindings_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_buyer_bindings_v10
    ADD CONSTRAINT qc_sim_buyer_bindings_v10_pkey PRIMARY KEY (tenant_id, serial);


--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_buyer_bindings_v10_tenant_id_buyer_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_buyer_bindings_v10
    ADD CONSTRAINT qc_sim_buyer_bindings_v10_tenant_id_buyer_code_key UNIQUE (tenant_id, buyer_code);


--
-- Name: qc_sim_events_v10 qc_sim_events_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_events_v10
    ADD CONSTRAINT qc_sim_events_v10_pkey PRIMARY KEY (tenant_id, event_id);


--
-- Name: qc_sim_routes_v10 qc_sim_routes_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_routes_v10
    ADD CONSTRAINT qc_sim_routes_v10_pkey PRIMARY KEY (tenant_id, lay_draft_id, plant_id);


--
-- Name: qc_sim_units_v10 qc_sim_units_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_units_v10
    ADD CONSTRAINT qc_sim_units_v10_pkey PRIMARY KEY (tenant_id, serial);


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_tenant_id_route_id_garment_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_tenant_id_route_id_garment_id_key UNIQUE (tenant_id, route_id, garment_id);


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_tenant_id_worker_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_tenant_id_worker_proof_id_key UNIQUE (tenant_id, worker_proof_id);


--
-- Name: sew_piece_used_scans_v27 sew_piece_used_scans_v27_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_used_scans_v27
    ADD CONSTRAINT sew_piece_used_scans_v27_pkey PRIMARY KEY (tenant_id, scan_id);


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_tenant_id_route_id_marker_position_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_tenant_id_route_id_marker_position_key UNIQUE (tenant_id, route_id, marker_position, ply_group);


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_tenant_id_worker_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_tenant_id_worker_proof_id_key UNIQUE (tenant_id, worker_proof_id);


--
-- Name: sew_prod_gateways_v10 sew_prod_gateways_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_gateways_v10
    ADD CONSTRAINT sew_prod_gateways_v10_pkey PRIMARY KEY (tenant_id, gateway_id);


--
-- Name: sew_prod_observations_v10 sew_prod_observations_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_observations_v10
    ADD CONSTRAINT sew_prod_observations_v10_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_prod_observations_v10 sew_prod_observations_v10_tenant_id_gateway_id_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_observations_v10
    ADD CONSTRAINT sew_prod_observations_v10_tenant_id_gateway_id_event_id_key UNIQUE (tenant_id, gateway_id, event_id);


--
-- Name: sew_prod_routes_v10 sew_prod_routes_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_routes_v10
    ADD CONSTRAINT sew_prod_routes_v10_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_prod_routes_v10 sew_prod_routes_v10_tenant_id_lay_draft_id_plant_id_operati_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_routes_v10
    ADD CONSTRAINT sew_prod_routes_v10_tenant_id_lay_draft_id_plant_id_operati_key UNIQUE (tenant_id, lay_draft_id, plant_id, operation_code, revision);


--
-- Name: sew_prod_used_observations_v10 sew_prod_used_observations_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_used_observations_v10
    ADD CONSTRAINT sew_prod_used_observations_v10_pkey PRIMARY KEY (tenant_id, observation_id);


--
-- Name: sew_prod_worker_stations_v10 sew_prod_worker_stations_v10_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_worker_stations_v10
    ADD CONSTRAINT sew_prod_worker_stations_v10_pkey PRIMARY KEY (tenant_id, plant_id, station_id, worker_subject);


--
-- Name: sew_sim_credits_v9 sew_sim_credits_v9_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_credits_v9
    ADD CONSTRAINT sew_sim_credits_v9_pkey PRIMARY KEY (tenant_id, cycle_id);


--
-- Name: sew_sim_credits_v9 sew_sim_credits_v9_tenant_id_operation_id_lay_draft_id_mark_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_credits_v9
    ADD CONSTRAINT sew_sim_credits_v9_tenant_id_operation_id_lay_draft_id_mark_key UNIQUE (tenant_id, operation_id, lay_draft_id, marker_position, ply_group);


--
-- Name: sew_sim_cycle_inputs_v9 sew_sim_cycle_inputs_v9_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycle_inputs_v9
    ADD CONSTRAINT sew_sim_cycle_inputs_v9_pkey PRIMARY KEY (tenant_id, cycle_id, scan_id);


--
-- Name: sew_sim_cycle_inputs_v9 sew_sim_cycle_inputs_v9_tenant_id_cycle_id_part_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycle_inputs_v9
    ADD CONSTRAINT sew_sim_cycle_inputs_v9_tenant_id_cycle_id_part_code_key UNIQUE (tenant_id, cycle_id, part_code);


--
-- Name: sew_sim_cycle_inputs_v9 sew_sim_cycle_inputs_v9_tenant_id_scan_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycle_inputs_v9
    ADD CONSTRAINT sew_sim_cycle_inputs_v9_tenant_id_scan_id_key UNIQUE (tenant_id, scan_id);


--
-- Name: sew_sim_cycles_v9 sew_sim_cycles_v9_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycles_v9
    ADD CONSTRAINT sew_sim_cycles_v9_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_sim_cycles_v9 sew_sim_cycles_v9_tenant_id_claim_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycles_v9
    ADD CONSTRAINT sew_sim_cycles_v9_tenant_id_claim_id_key UNIQUE (tenant_id, claim_id);


--
-- Name: sew_sim_cycles_v9 sew_sim_cycles_v9_tenant_id_operation_id_lay_draft_id_marke_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycles_v9
    ADD CONSTRAINT sew_sim_cycles_v9_tenant_id_operation_id_lay_draft_id_marke_key UNIQUE (tenant_id, operation_id, lay_draft_id, marker_position, ply_group);


--
-- Name: sew_sim_events_v9 sew_sim_events_v9_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_events_v9
    ADD CONSTRAINT sew_sim_events_v9_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_sim_events_v9 sew_sim_events_v9_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_events_v9
    ADD CONSTRAINT sew_sim_events_v9_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: sew_sim_operations_v9 sew_sim_operations_v9_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_operations_v9
    ADD CONSTRAINT sew_sim_operations_v9_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_sim_operations_v9 sew_sim_operations_v9_tenant_id_lay_draft_id_operation_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_operations_v9
    ADD CONSTRAINT sew_sim_operations_v9_tenant_id_lay_draft_id_operation_code_key UNIQUE (tenant_id, lay_draft_id, operation_code);


--
-- Name: sew_sim_scans_v9 sew_sim_scans_v9_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_scans_v9
    ADD CONSTRAINT sew_sim_scans_v9_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: sew_sim_scans_v9 sew_sim_scans_v9_tenant_id_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_scans_v9
    ADD CONSTRAINT sew_sim_scans_v9_tenant_id_event_id_key UNIQUE (tenant_id, event_id);


--
-- Name: store_issuances store_issuances_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_issuances
    ADD CONSTRAINT store_issuances_pkey PRIMARY KEY (id);


--
-- Name: store_pick_proposals_v7 store_pick_proposals_v7_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_pick_proposals_v7
    ADD CONSTRAINT store_pick_proposals_v7_pkey PRIMARY KEY (id);


--
-- Name: store_pick_proposals_v7 store_pick_proposals_v7_tenant_id_request_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_pick_proposals_v7
    ADD CONSTRAINT store_pick_proposals_v7_tenant_id_request_key_key UNIQUE (tenant_id, request_key);


--
-- Name: store_reconciliation_requests_v7 store_reconciliation_requests_v7_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_reconciliation_requests_v7
    ADD CONSTRAINT store_reconciliation_requests_v7_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: store_reconciliation_requests_v7 store_reconciliation_requests_v7_tenant_id_request_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_reconciliation_requests_v7
    ADD CONSTRAINT store_reconciliation_requests_v7_tenant_id_request_key_key UNIQUE (tenant_id, request_key);


--
-- Name: store_roll_admission_proofs_v7 store_roll_admission_proofs_v7_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admission_proofs_v7
    ADD CONSTRAINT store_roll_admission_proofs_v7_pkey PRIMARY KEY (tenant_id, proof_id);


--
-- Name: store_roll_admissions_v7 store_roll_admissions_v7_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admissions_v7
    ADD CONSTRAINT store_roll_admissions_v7_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: store_roll_admissions_v7 store_roll_admissions_v7_tenant_id_request_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admissions_v7
    ADD CONSTRAINT store_roll_admissions_v7_tenant_id_request_id_key UNIQUE (tenant_id, request_id);


--
-- Name: store_roll_admissions_v7 store_roll_admissions_v7_tenant_id_roll_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admissions_v7
    ADD CONSTRAINT store_roll_admissions_v7_tenant_id_roll_id_key UNIQUE (tenant_id, roll_id);


--
-- Name: store_roll_passport_events_v7 store_roll_passport_events_v7_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_passport_events_v7
    ADD CONSTRAINT store_roll_passport_events_v7_pkey PRIMARY KEY (tenant_id, roll_id, revision);


--
-- Name: store_roll_rfid_bindings_v7 store_roll_rfid_bindings_v7_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_rfid_bindings_v7
    ADD CONSTRAINT store_roll_rfid_bindings_v7_pkey PRIMARY KEY (tenant_id, epc_hex);


--
-- Name: store_roll_rfid_bindings_v7 store_roll_rfid_bindings_v7_tenant_id_roll_id_epc_hex_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_rfid_bindings_v7
    ADD CONSTRAINT store_roll_rfid_bindings_v7_tenant_id_roll_id_epc_hex_key UNIQUE (tenant_id, roll_id, epc_hex);


--
-- Name: store_roll_rfid_bindings_v7 store_roll_rfid_bindings_v7_tenant_id_roll_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_rfid_bindings_v7
    ADD CONSTRAINT store_roll_rfid_bindings_v7_tenant_id_roll_id_key UNIQUE (tenant_id, roll_id);


--
-- Name: store_rolls store_rolls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_rolls
    ADD CONSTRAINT store_rolls_pkey PRIMARY KEY (id);


--
-- Name: store_trims_v13 store_trims_v13_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_trims_v13
    ADD CONSTRAINT store_trims_v13_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: style_assembly_dag style_assembly_dag_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_assembly_dag
    ADD CONSTRAINT style_assembly_dag_pkey PRIMARY KEY (id);


--
-- Name: style_parts style_parts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_parts
    ADD CONSTRAINT style_parts_pkey PRIMARY KEY (id);


--
-- Name: styles_master styles_master_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.styles_master
    ADD CONSTRAINT styles_master_pkey PRIMARY KEY (id);


--
-- Name: styles_master styles_master_tenant_id_style_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.styles_master
    ADD CONSTRAINT styles_master_tenant_id_style_code_key UNIQUE (tenant_id, style_code);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: trim_admission_proof_uses_v25 trim_admission_proof_uses_v25_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_admission_proof_uses_v25
    ADD CONSTRAINT trim_admission_proof_uses_v25_pkey PRIMARY KEY (tenant_id, proof_id);


--
-- Name: trim_issuance_proof_uses_v24 trim_issuance_proof_uses_v24_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_issuance_proof_uses_v24
    ADD CONSTRAINT trim_issuance_proof_uses_v24_pkey PRIMARY KEY (tenant_id, proof_id);


--
-- Name: trim_issuances_v13 trim_issuances_v13_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_issuances_v13
    ADD CONSTRAINT trim_issuances_v13_pkey PRIMARY KEY (id);


--
-- Name: trim_stock_admissions_v25 trim_stock_admissions_v25_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_stock_admissions_v25
    ADD CONSTRAINT trim_stock_admissions_v25_pkey PRIMARY KEY (id);


--
-- Name: trim_stock_admissions_v25 trim_stock_admissions_v25_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_stock_admissions_v25
    ADD CONSTRAINT trim_stock_admissions_v25_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: trim_stock_admissions_v25 trim_stock_admissions_v25_tenant_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_stock_admissions_v25
    ADD CONSTRAINT trim_stock_admissions_v25_tenant_id_id_key UNIQUE (tenant_id, id);


--
-- Name: trim_stock_admissions_v25 trim_stock_admissions_v25_tenant_id_stock_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_stock_admissions_v25
    ADD CONSTRAINT trim_stock_admissions_v25_tenant_id_stock_id_key UNIQUE (tenant_id, stock_id);


--
-- Name: order_sku_matrix uq_order_color_size; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_sku_matrix
    ADD CONSTRAINT uq_order_color_size UNIQUE (tenant_id, order_id, color_code, size_code);


--
-- Name: wip_exception_cases_v23 wip_exception_cases_v23_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_cases_v23
    ADD CONSTRAINT wip_exception_cases_v23_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: wip_exception_cases_v23 wip_exception_cases_v23_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_cases_v23
    ADD CONSTRAINT wip_exception_cases_v23_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: wip_exception_cases_v23 wip_exception_cases_v23_tenant_id_reporter_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_cases_v23
    ADD CONSTRAINT wip_exception_cases_v23_tenant_id_reporter_proof_id_key UNIQUE (tenant_id, reporter_proof_id);


--
-- Name: wip_exception_closures_v23 wip_exception_closures_v23_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_closures_v23
    ADD CONSTRAINT wip_exception_closures_v23_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: wip_exception_closures_v23 wip_exception_closures_v23_tenant_id_case_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_closures_v23
    ADD CONSTRAINT wip_exception_closures_v23_tenant_id_case_id_key UNIQUE (tenant_id, case_id);


--
-- Name: wip_exception_closures_v23 wip_exception_closures_v23_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_closures_v23
    ADD CONSTRAINT wip_exception_closures_v23_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: wip_exception_closures_v23 wip_exception_closures_v23_tenant_id_supervisor_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_closures_v23
    ADD CONSTRAINT wip_exception_closures_v23_tenant_id_supervisor_proof_id_key UNIQUE (tenant_id, supervisor_proof_id);


--
-- Name: wip_gateway_scans_v20 wip_gateway_scans_v20_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_gateway_scans_v20
    ADD CONSTRAINT wip_gateway_scans_v20_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: wip_gateway_scans_v20 wip_gateway_scans_v20_tenant_id_gateway_id_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_gateway_scans_v20
    ADD CONSTRAINT wip_gateway_scans_v20_tenant_id_gateway_id_event_id_key UNIQUE (tenant_id, gateway_id, event_id);


--
-- Name: wip_movements_v20 wip_movements_v20_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_part_id_event_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_part_id_event_no_key UNIQUE (tenant_id, part_id, event_no);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_scan_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_scan_id_key UNIQUE (tenant_id, scan_id);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_worker_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_worker_proof_id_key UNIQUE (tenant_id, worker_proof_id);


--
-- Name: wip_piece_holds_v26 wip_piece_holds_v26_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_holds_v26
    ADD CONSTRAINT wip_piece_holds_v26_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: wip_piece_holds_v26 wip_piece_holds_v26_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_holds_v26
    ADD CONSTRAINT wip_piece_holds_v26_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: wip_piece_holds_v26 wip_piece_holds_v26_tenant_id_reporter_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_holds_v26
    ADD CONSTRAINT wip_piece_holds_v26_tenant_id_reporter_proof_id_key UNIQUE (tenant_id, reporter_proof_id);


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_tenant_id_command_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_tenant_id_command_id_key UNIQUE (tenant_id, command_id);


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_tenant_id_hold_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_tenant_id_hold_id_key UNIQUE (tenant_id, hold_id);


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_tenant_id_supervisor_proof_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_tenant_id_supervisor_proof_id_key UNIQUE (tenant_id, supervisor_proof_id);


--
-- Name: wip_transfer_routes_v20 wip_transfer_routes_v20_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_transfer_routes_v20
    ADD CONSTRAINT wip_transfer_routes_v20_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: wip_transfer_routes_v20 wip_transfer_routes_v20_tenant_id_plant_id_from_station_to__key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_transfer_routes_v20
    ADD CONSTRAINT wip_transfer_routes_v20_tenant_id_plant_id_from_station_to__key UNIQUE (tenant_id, plant_id, from_station, to_station, revision);


--
-- Name: workers workers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workers
    ADD CONSTRAINT workers_pkey PRIMARY KEY (id);


--
-- Name: workforce_browser_sessions_v16 workforce_browser_sessions_v16_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workforce_browser_sessions_v16
    ADD CONSTRAINT workforce_browser_sessions_v16_pkey PRIMARY KEY (session_hash);


--
-- Name: workforce_chapter_actions_v17 workforce_chapter_actions_v17_chapter_id_permission_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workforce_chapter_actions_v17
    ADD CONSTRAINT workforce_chapter_actions_v17_chapter_id_permission_key UNIQUE (chapter_id, permission);


--
-- Name: workforce_chapter_actions_v17 workforce_chapter_actions_v17_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workforce_chapter_actions_v17
    ADD CONSTRAINT workforce_chapter_actions_v17_pkey PRIMARY KEY (permission);


--
-- Name: workforce_identity_grants_v15 workforce_identity_grants_v15_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workforce_identity_grants_v15
    ADD CONSTRAINT workforce_identity_grants_v15_pkey PRIMARY KEY (tenant_id, plant_id, idp_issuer, idp_subject, permission);


--
-- Name: workforce_oidc_requests_v16 workforce_oidc_requests_v16_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workforce_oidc_requests_v16
    ADD CONSTRAINT workforce_oidc_requests_v16_pkey PRIMARY KEY (state_hash);


--
-- Name: plants plants_pkey; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.plants
    ADD CONSTRAINT plants_pkey PRIMARY KEY (id);


--
-- Name: role_permissions role_permissions_pkey; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.role_permissions
    ADD CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_slug_key; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.tenants
    ADD CONSTRAINT tenants_slug_key UNIQUE (slug);


--
-- Name: plants uq_plant_tenant_code; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.plants
    ADD CONSTRAINT uq_plant_tenant_code UNIQUE (tenant_id, code);


--
-- Name: roles uq_role_tenant_name; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.roles
    ADD CONSTRAINT uq_role_tenant_name UNIQUE (tenant_id, name);


--
-- Name: users uq_user_email; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.users
    ADD CONSTRAINT uq_user_email UNIQUE (tenant_id, email);


--
-- Name: user_role_grants uq_user_grant; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.user_role_grants
    ADD CONSTRAINT uq_user_grant UNIQUE (user_id, role_id, plant_id);


--
-- Name: user_role_grants user_role_grants_pkey; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.user_role_grants
    ADD CONSTRAINT user_role_grants_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: registered_devices registered_devices_pkey; Type: CONSTRAINT; Schema: s02_device; Owner: -
--

ALTER TABLE ONLY s02_device.registered_devices
    ADD CONSTRAINT registered_devices_pkey PRIMARY KEY (id);


--
-- Name: registered_devices uq_device_uid; Type: CONSTRAINT; Schema: s02_device; Owner: -
--

ALTER TABLE ONLY s02_device.registered_devices
    ADD CONSTRAINT uq_device_uid UNIQUE (tenant_id, device_uid);


--
-- Name: buyer_handoffs_intake_v65c1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX buyer_handoffs_intake_v65c1 ON public.buyer_order_handoffs_v65c1 USING btree (tenant_id, intake_id);


--
-- Name: cut_bundle_drafts_by_label_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_bundle_drafts_by_label_v8 ON public.cut_bundle_drafts_v8 USING btree (tenant_id, label_code);


--
-- Name: cut_bundle_drafts_sew_groups_v9; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_bundle_drafts_sew_groups_v9 ON public.cut_bundle_drafts_v8 USING btree (tenant_id, lay_draft_id, marker_position, ply_group, part_code);


--
-- Name: cut_bundle_label_lineage_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX cut_bundle_label_lineage_v8 ON public.cut_bundle_drafts_v8 USING btree (tenant_id, lay_draft_id, label_code);


--
-- Name: cut_bundle_scans_by_spread_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_bundle_scans_by_spread_v8 ON public.cut_bundle_scan_witnesses_v8 USING btree (tenant_id, spread_report_id);


--
-- Name: cut_lay_drafts_by_order_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_lay_drafts_by_order_v8 ON public.cut_lay_drafts_v8 USING btree (tenant_id, intake_id, cut_no, revision DESC);


--
-- Name: cut_lay_drafts_sew_catalog_v9; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_lay_drafts_sew_catalog_v9 ON public.cut_lay_drafts_v8 USING btree (tenant_id, created_at DESC, id DESC);


--
-- Name: cut_part_outcomes_by_release_v19; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_part_outcomes_by_release_v19 ON public.cut_part_outcomes_v19 USING btree (tenant_id, plant_id, release_id, disposition);


--
-- Name: cut_reader_events_by_spread_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_reader_events_by_spread_v8 ON public.cut_reader_events_v8 USING btree (tenant_id, spread_report_id);


--
-- Name: cut_reconcile_by_plant_v19; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_reconcile_by_plant_v19 ON public.cut_consumption_reconciliations_v19 USING btree (tenant_id, plant_id, created_at DESC, id);


--
-- Name: cut_spread_allocation_by_issue_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cut_spread_allocation_by_issue_v8 ON public.cut_spread_issue_allocations_v8 USING btree (tenant_id, issue_id);


--
-- Name: cut_spread_report_lineage_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX cut_spread_report_lineage_v8 ON public.cut_spread_reports_v8 USING btree (tenant_id, id, lay_draft_id);


--
-- Name: dock_external_assertions_manifest_v35; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dock_external_assertions_manifest_v35 ON public.dock_external_assertions_v35 USING btree (tenant_id, plant_id, manifest_id);


--
-- Name: document_jobs_v65_queue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX document_jobs_v65_queue ON public.document_jobs_v65 USING btree (status, updated_at, document_id);


--
-- Name: document_sources_v65_tenant_filename_revision_v14; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX document_sources_v65_tenant_filename_revision_v14 ON public.document_sources_v65 USING btree (tenant_id, lower((filename)::text), captured_at, id);


--
-- Name: document_sources_v65_tenant_time_keyset_v14; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX document_sources_v65_tenant_time_keyset_v14 ON public.document_sources_v65 USING btree (tenant_id, captured_at DESC, id DESC);


--
-- Name: documents_tenant_identity_v25; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX documents_tenant_identity_v25 ON public.documents USING btree (tenant_id, id);


--
-- Name: fabric_shrinkage_latest_v31; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX fabric_shrinkage_latest_v31 ON public.fabric_shrinkage_specs_v31 USING btree (tenant_id, plant_id, material_sku, treatment_code, revision DESC);


--
-- Name: fabric_shrinkage_roll_v30; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX fabric_shrinkage_roll_v30 ON public.fabric_shrinkage_observations_v30 USING btree (tenant_id, plant_id, roll_id, recorded_at DESC);


--
-- Name: factory_approved_by_document_v65b5; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX factory_approved_by_document_v65b5 ON public.factory_approved_reference_versions_v65b5 USING btree (tenant_id, document_id);


--
-- Name: factory_candidates_by_document_v65b2; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX factory_candidates_by_document_v65b2 ON public.factory_document_candidates_v65b2 USING btree (tenant_id, document_id, captured_at DESC);


--
-- Name: factory_draft_decisions_latest_v65b3; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX factory_draft_decisions_latest_v65b3 ON public.factory_draft_decisions_v65b3 USING btree (tenant_id, entity_type, entity_key, revision DESC);


--
-- Name: factory_fact_events_latest_v65b; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX factory_fact_events_latest_v65b ON public.factory_fact_events_v65b USING btree (tenant_id, field_key, revision DESC);


--
-- Name: factory_reference_latest_v65b4; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX factory_reference_latest_v65b4 ON public.factory_reference_versions_v65b4 USING btree (tenant_id, entity_type, entity_key, revision DESC);


--
-- Name: gen_carrier_history_v18; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX gen_carrier_history_v18 ON public.gen_carrier_events_v18 USING btree (tenant_id, plant_id, carrier_id, event_no DESC);


--
-- Name: gen_garments_lay_v18; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX gen_garments_lay_v18 ON public.gen_garments_v18 USING btree (tenant_id, plant_id, lay_draft_id, id);


--
-- Name: gen_parts_by_garment_v18; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX gen_parts_by_garment_v18 ON public.gen_cut_parts_v18 USING btree (tenant_id, garment_id, id);


--
-- Name: idx_bundle_active_nfc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bundle_active_nfc ON public.bundle_registry USING btree (active_nfc_uid);


--
-- Name: idx_intake_capture_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_intake_capture_tenant ON public.intake_document_capture USING btree (tenant_id, captured_at DESC);


--
-- Name: idx_intake_fact_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_intake_fact_tenant ON public.intake_proposed_fact USING btree (tenant_id, capture_id);


--
-- Name: idx_store_rolls_tenant_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_store_rolls_tenant_status ON public.store_rolls USING btree (tenant_id, status);


--
-- Name: intel_sim_snapshots_by_lay_v12; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX intel_sim_snapshots_by_lay_v12 ON public.intel_sim_snapshots_v12 USING btree (tenant_id, lay_draft_id, captured_at DESC);


--
-- Name: ix_material_issue_reservation_v5; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_material_issue_reservation_v5 ON public.material_issues_v5 USING btree (tenant_id, reservation_id);


--
-- Name: ix_material_reservation_roll_v5; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_material_reservation_roll_v5 ON public.material_reservations_v5 USING btree (tenant_id, roll_id);


--
-- Name: jobwork_discrepancies_part_v29; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX jobwork_discrepancies_part_v29 ON public.jobwork_discrepancies_v29 USING btree (tenant_id, plant_id, part_id, reported_at DESC);


--
-- Name: jobwork_disposition_part_v31; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX jobwork_disposition_part_v31 ON public.jobwork_discrepancy_resolutions_v31 USING btree (tenant_id, part_id, recorded_at DESC);


--
-- Name: jobwork_in_part_v28; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX jobwork_in_part_v28 ON public.jobwork_inwards_v28 USING btree (tenant_id, plant_id, part_id, recorded_at DESC);


--
-- Name: jobwork_out_part_v28; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX jobwork_out_part_v28 ON public.jobwork_outwards_v28 USING btree (tenant_id, plant_id, part_id, recorded_at DESC);


--
-- Name: material_issues_tenant_identity_v8; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX material_issues_tenant_identity_v8 ON public.material_issues_v5 USING btree (tenant_id, id);


--
-- Name: material_scenarios_recent_v6; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX material_scenarios_recent_v6 ON public.material_scenarios_v6 USING btree (tenant_id, intake_id, created_at DESC);


--
-- Name: needle_followup_once_v33; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX needle_followup_once_v33 ON public.needle_safety_clearances_v33 USING btree (tenant_id, followup_pass_id);


--
-- Name: order_intakes_tenant_recent_v5; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX order_intakes_tenant_recent_v5 ON public.order_intakes_v5 USING btree (tenant_id, captured_at DESC);


--
-- Name: pack_carton_history_v22; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pack_carton_history_v22 ON public.pack_carton_events_v22 USING btree (tenant_id, plant_id, carton_id, event_no);


--
-- Name: pack_manifest_history_v22; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pack_manifest_history_v22 ON public.pack_manifest_events_v22 USING btree (tenant_id, plant_id, manifest_id, event_no);


--
-- Name: pack_one_active_spec_v22; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX pack_one_active_spec_v22 ON public.pack_carton_specs_v22 USING btree (tenant_id, lay_draft_id) WHERE enabled;


--
-- Name: pack_one_carton_per_garment_v22; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX pack_one_carton_per_garment_v22 ON public.pack_carton_events_v22 USING btree (tenant_id, garment_id) WHERE ((action)::text = 'ADD'::text);


--
-- Name: pack_one_manifest_per_carton_v22; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX pack_one_manifest_per_carton_v22 ON public.pack_manifest_events_v22 USING btree (tenant_id, carton_id) WHERE ((action)::text = 'ADD_CARTON'::text);


--
-- Name: pack_prod_holds_recent_v12; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pack_prod_holds_recent_v12 ON public.pack_prod_holds_v12 USING btree (tenant_id, plant_id, created_at DESC, id DESC);


--
-- Name: pack_sim_items_carton_v11; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pack_sim_items_carton_v11 ON public.pack_sim_items_v11 USING btree (tenant_id, carton_code);


--
-- Name: pack_unique_carton_code_v22; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX pack_unique_carton_code_v22 ON public.pack_carton_events_v22 USING btree (tenant_id, plant_id, carton_code) WHERE ((action)::text = 'OPEN'::text);


--
-- Name: pack_unique_manifest_ref_v22; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX pack_unique_manifest_ref_v22 ON public.pack_manifest_events_v22 USING btree (tenant_id, plant_id, manifest_ref) WHERE ((action)::text = 'OPEN'::text);


--
-- Name: qc_garment_history_v21; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX qc_garment_history_v21 ON public.qc_garment_events_v21 USING btree (tenant_id, plant_id, garment_id, event_no DESC);


--
-- Name: qc_garment_status_v21; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX qc_garment_status_v21 ON public.qc_garment_events_v21 USING btree (tenant_id, plant_id, action, recorded_at);


--
-- Name: qc_one_active_spec_per_lay_v21; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX qc_one_active_spec_per_lay_v21 ON public.qc_garment_specs_v21 USING btree (tenant_id, lay_draft_id) WHERE enabled;


--
-- Name: qc_prod_group_history_v11; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX qc_prod_group_history_v11 ON public.qc_prod_group_events_v11 USING btree (tenant_id, plant_id, completion_id, revision DESC);


--
-- Name: qc_prod_group_recent_v11; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX qc_prod_group_recent_v11 ON public.qc_prod_group_events_v11 USING btree (tenant_id, plant_id, created_at DESC, id DESC);


--
-- Name: qc_sim_events_unit_v10; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX qc_sim_events_unit_v10 ON public.qc_sim_events_v10 USING btree (tenant_id, serial, recorded_at);


--
-- Name: qc_sim_units_lay_v10; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX qc_sim_units_lay_v10 ON public.qc_sim_units_v10 USING btree (tenant_id, lay_draft_id, plant_id, qc_state);


--
-- Name: qc_specs_lay_v21; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX qc_specs_lay_v21 ON public.qc_garment_specs_v21 USING btree (tenant_id, plant_id, lay_draft_id, enabled);


--
-- Name: sew_piece_completion_history_v27; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_piece_completion_history_v27 ON public.sew_piece_completions_v27 USING btree (tenant_id, plant_id, garment_id, recorded_at DESC);


--
-- Name: sew_piece_scans_completion_v27; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_piece_scans_completion_v27 ON public.sew_piece_used_scans_v27 USING btree (tenant_id, completion_id);


--
-- Name: sew_prod_completions_history_v10; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_prod_completions_history_v10 ON public.sew_prod_completions_v10 USING btree (tenant_id, plant_id, created_at DESC, id DESC);


--
-- Name: sew_prod_observations_group_v10; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_prod_observations_group_v10 ON public.sew_prod_observations_v10 USING btree (tenant_id, lay_draft_id, plant_id, station_id, marker_position, ply_group, received_at DESC);


--
-- Name: sew_sim_cycles_active_station_v9; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_sim_cycles_active_station_v9 ON public.sew_sim_cycles_v9 USING btree (tenant_id, lay_draft_id, plant_id, station_id, worker_id, claimed_at DESC) WHERE ((state)::text = ANY ((ARRAY['CLAIMED'::character varying, 'PAUSED'::character varying])::text[]));


--
-- Name: sew_sim_cycles_history_lookup_v9; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_sim_cycles_history_lookup_v9 ON public.sew_sim_cycles_v9 USING btree (tenant_id, lay_draft_id, plant_id, claimed_at DESC, id DESC);


--
-- Name: sew_sim_scans_claim_lookup_v9; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_sim_scans_claim_lookup_v9 ON public.sew_sim_scans_v9 USING btree (tenant_id, lay_draft_id, plant_id, label_code, station_id, worker_id, received_at DESC);


--
-- Name: sew_sim_scans_lookup_v9; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sew_sim_scans_lookup_v9 ON public.sew_sim_scans_v9 USING btree (tenant_id, lay_draft_id, station_id, worker_id, received_at DESC);


--
-- Name: store_pick_proposals_recent_v7; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX store_pick_proposals_recent_v7 ON public.store_pick_proposals_v7 USING btree (tenant_id, created_at DESC);


--
-- Name: store_reconciliation_roll_v7; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX store_reconciliation_roll_v7 ON public.store_reconciliation_requests_v7 USING btree (tenant_id, roll_id, created_at DESC);


--
-- Name: store_roll_admission_recorded_v7; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX store_roll_admission_recorded_v7 ON public.store_roll_admissions_v7 USING btree (tenant_id, admitted_at DESC);


--
-- Name: store_roll_passport_latest_v7; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX store_roll_passport_latest_v7 ON public.store_roll_passport_events_v7 USING btree (tenant_id, roll_id, revision DESC);


--
-- Name: store_rolls_tenant_identity_v5; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX store_rolls_tenant_identity_v5 ON public.store_rolls USING btree (tenant_id, id);


--
-- Name: trim_admissions_by_source_v25; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trim_admissions_by_source_v25 ON public.trim_stock_admissions_v25 USING btree (tenant_id, source_document_id);


--
-- Name: trim_issuances_command_v24; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX trim_issuances_command_v24 ON public.trim_issuances_v13 USING btree (tenant_id, command_id) WHERE (command_id IS NOT NULL);


--
-- Name: trim_issuances_stock_recent_v24; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trim_issuances_stock_recent_v24 ON public.trim_issuances_v13 USING btree (tenant_id, trim_stock_id, issued_at DESC);


--
-- Name: uq_material_ceo_proof_v5; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_material_ceo_proof_v5 ON public.material_variances_v5 USING btree (tenant_id, ceo_proof_id) WHERE (ceo_proof_id IS NOT NULL);


--
-- Name: wip_exception_part_v23; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX wip_exception_part_v23 ON public.wip_exception_cases_v23 USING btree (tenant_id, plant_id, part_id, opened_at DESC);


--
-- Name: wip_movements_metrics_v20; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX wip_movements_metrics_v20 ON public.wip_movements_v20 USING btree (tenant_id, plant_id, route_id, recorded_at) WHERE ((action)::text = 'ARRIVE'::text);


--
-- Name: wip_movements_part_v20; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX wip_movements_part_v20 ON public.wip_movements_v20 USING btree (tenant_id, plant_id, part_id, event_no DESC);


--
-- Name: wip_piece_holds_by_unit_v26; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX wip_piece_holds_by_unit_v26 ON public.wip_piece_holds_v26 USING btree (tenant_id, garment_id, opened_at DESC);


--
-- Name: wip_piece_scan_once_v26; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX wip_piece_scan_once_v26 ON public.wip_piece_reviews_v26 USING btree (tenant_id, scan_id);


--
-- Name: wip_scans_part_v20; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX wip_scans_part_v20 ON public.wip_gateway_scans_v20 USING btree (tenant_id, plant_id, part_id, received_at DESC);


--
-- Name: workforce_browser_sessions_expire_v16; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX workforce_browser_sessions_expire_v16 ON public.workforce_browser_sessions_v16 USING btree (expires_at);


--
-- Name: workforce_browser_sessions_subject_v16; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX workforce_browser_sessions_subject_v16 ON public.workforce_browser_sessions_v16 USING btree (idp_issuer, idp_subject) WHERE (revoked_at IS NULL);


--
-- Name: workforce_grant_subject_v15; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX workforce_grant_subject_v15 ON public.workforce_identity_grants_v15 USING btree (idp_issuer, idp_subject, tenant_id, plant_id) WHERE enabled;


--
-- Name: workforce_oidc_requests_expire_v16; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX workforce_oidc_requests_expire_v16 ON public.workforce_oidc_requests_v16 USING btree (expires_at);


--
-- Name: buyer_aql_reviews_v33 aql_review_proof_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER aql_review_proof_v33 BEFORE INSERT ON public.buyer_aql_reviews_v33 FOR EACH ROW EXECUTE FUNCTION public.guard_factory_review_proof_v33();


--
-- Name: buyer_aql_plans_v32 buyer_aql_plan_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER buyer_aql_plan_guard_v32 BEFORE INSERT ON public.buyer_aql_plans_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_buyer_aql_plan_v32();


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_proof_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER buyer_aql_proof_guard_v32 BEFORE INSERT ON public.buyer_aql_sample_events_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_factory_proof_v32();


--
-- Name: buyer_aql_reviews_v33 buyer_aql_review_guard_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER buyer_aql_review_guard_v33 BEFORE INSERT ON public.buyer_aql_reviews_v33 FOR EACH ROW EXECUTE FUNCTION public.guard_buyer_aql_review_v33();


--
-- Name: buyer_aql_review_rules_v33 buyer_aql_rule_guard_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER buyer_aql_rule_guard_v33 BEFORE INSERT ON public.buyer_aql_review_rules_v33 FOR EACH ROW EXECUTE FUNCTION public.guard_buyer_aql_rules_v33();


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER buyer_aql_sample_guard_v32 BEFORE INSERT ON public.buyer_aql_sample_events_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_buyer_aql_sample_v32();


--
-- Name: fabric_calibration_links_v32 calibration_proof_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER calibration_proof_guard_v32 BEFORE INSERT ON public.fabric_calibration_links_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_factory_proof_v32();


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_guard_v19; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_consumption_guard_v19 BEFORE INSERT ON public.cut_consumption_reconciliations_v19 FOR EACH ROW EXECUTE FUNCTION public.guard_cut_consumption_v19();


--
-- Name: cut_consumption_reconciliations_v19 cut_immutable_v19; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_immutable_v19 BEFORE DELETE OR UPDATE ON public.cut_consumption_reconciliations_v19 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_part_outcomes_v19 cut_immutable_v19; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_immutable_v19 BEFORE DELETE OR UPDATE ON public.cut_part_outcomes_v19 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_proof_consumptions_v19 cut_immutable_v19; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_immutable_v19 BEFORE DELETE OR UPDATE ON public.cut_proof_consumptions_v19 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_recut_links_v19 cut_immutable_v19; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_immutable_v19 BEFORE DELETE OR UPDATE ON public.cut_recut_links_v19 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_spread_issue_allocations_v8 cut_issue_attribution_guard_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_issue_attribution_guard_v8 BEFORE INSERT ON public.cut_spread_issue_allocations_v8 FOR EACH ROW EXECUTE FUNCTION public.validate_cut_issue_attribution_v8();


--
-- Name: cut_part_outcomes_v19 cut_part_outcome_guard_v19; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_part_outcome_guard_v19 BEFORE INSERT ON public.cut_part_outcomes_v19 FOR EACH ROW EXECUTE FUNCTION public.guard_cut_part_outcome_v19();


--
-- Name: cut_reader_events_v8 cut_reader_event_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_reader_event_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_reader_events_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_recut_links_v19 cut_recut_guard_v19; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_recut_guard_v19 BEFORE INSERT ON public.cut_recut_links_v19 FOR EACH ROW EXECUTE FUNCTION public.guard_cut_recut_v19();


--
-- Name: cut_sewing_releases_v8 cut_release_evidence_guard_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_release_evidence_guard_v8 BEFORE INSERT ON public.cut_sewing_releases_v8 FOR EACH ROW EXECUTE FUNCTION public.validate_cut_release_v8();


--
-- Name: cut_sewing_releases_v8 cut_sewing_release_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cut_sewing_release_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_sewing_releases_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_spread_reports_v8 cut_spread_full_coverage_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER cut_spread_full_coverage_v8 AFTER INSERT ON public.cut_spread_reports_v8 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_cut_spread_coverage_v8();


--
-- Name: cut_bundle_scan_witnesses_v8 cutting_evidence_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cutting_evidence_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_bundle_scan_witnesses_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_spread_issue_allocations_v8 cutting_evidence_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cutting_evidence_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_spread_issue_allocations_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_spread_reports_v8 cutting_evidence_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cutting_evidence_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_spread_reports_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_bundle_drafts_v8 cutting_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cutting_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_bundle_drafts_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_lay_drafts_v8 cutting_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cutting_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_lay_drafts_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_lay_positions_v8 cutting_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cutting_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_lay_positions_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_lay_snapshots_v8 cutting_immutable_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cutting_immutable_v8 BEFORE DELETE OR UPDATE ON public.cut_lay_snapshots_v8 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: dock_asn_drafts_v34 dock_asn_guard_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_asn_guard_v34 BEFORE INSERT ON public.dock_asn_drafts_v34 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_asn_v34();


--
-- Name: dock_asn_drafts_v34 dock_asn_proof_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_asn_proof_v34 BEFORE INSERT ON public.dock_asn_drafts_v34 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_proof_v34();


--
-- Name: dock_buyer_spec_bindings_v36 dock_buyer_binding_insert_guard_v36; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_buyer_binding_insert_guard_v36 BEFORE INSERT OR UPDATE ON public.dock_buyer_spec_bindings_v36 FOR EACH ROW EXECUTE FUNCTION public.dock_buyer_binding_guard_v36();


--
-- Name: dock_carrier_receipts_v34 dock_carrier_guard_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_carrier_guard_v34 BEFORE INSERT ON public.dock_carrier_receipts_v34 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_carrier_v34();


--
-- Name: dock_carrier_receipts_v34 dock_carrier_proof_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_carrier_proof_v34 BEFORE INSERT ON public.dock_carrier_receipts_v34 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_proof_v34();


--
-- Name: dock_external_assertions_v35 dock_external_guard_v35; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_external_guard_v35 BEFORE INSERT ON public.dock_external_assertions_v35 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_external_v35();


--
-- Name: dock_external_assertions_v35 dock_external_immutable_v35; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_external_immutable_v35 BEFORE DELETE OR UPDATE ON public.dock_external_assertions_v35 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: dock_asn_drafts_v34 dock_immutable_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_immutable_v34 BEFORE DELETE OR UPDATE ON public.dock_asn_drafts_v34 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: dock_carrier_receipts_v34 dock_immutable_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_immutable_v34 BEFORE DELETE OR UPDATE ON public.dock_carrier_receipts_v34 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: dock_carton_scans_v34 dock_immutable_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_immutable_v34 BEFORE DELETE OR UPDATE ON public.dock_carton_scans_v34 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: dock_container_seals_v34 dock_immutable_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_immutable_v34 BEFORE DELETE OR UPDATE ON public.dock_container_seals_v34 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: dock_carton_scans_v34 dock_scan_guard_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_scan_guard_v34 BEFORE INSERT ON public.dock_carton_scans_v34 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_scan_v34();


--
-- Name: dock_container_seals_v34 dock_seal_guard_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_seal_guard_v34 BEFORE INSERT ON public.dock_container_seals_v34 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_seal_v34();


--
-- Name: dock_container_seals_v34 dock_seal_proof_v34; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER dock_seal_proof_v34 BEFORE INSERT ON public.dock_container_seals_v34 FOR EACH ROW EXECUTE FUNCTION public.guard_dock_proof_v34();


--
-- Name: cut_bundle_scan_witnesses_v8 epc_namespace_bundle_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER epc_namespace_bundle_v8 BEFORE INSERT ON public.cut_bundle_scan_witnesses_v8 FOR EACH ROW EXECUTE FUNCTION public.prevent_cross_type_epc_v8();


--
-- Name: store_roll_rfid_bindings_v7 epc_namespace_roll_v8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER epc_namespace_roll_v8 BEFORE INSERT ON public.store_roll_rfid_bindings_v7 FOR EACH ROW EXECUTE FUNCTION public.prevent_cross_type_epc_v8();


--
-- Name: fabric_calibration_links_v32 fabric_calibration_link_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER fabric_calibration_link_guard_v32 BEFORE INSERT ON public.fabric_calibration_links_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_fabric_calibration_link_v32();


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_guard_v30; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER fabric_shrinkage_guard_v30 BEFORE INSERT ON public.fabric_shrinkage_observations_v30 FOR EACH ROW EXECUTE FUNCTION public.guard_fabric_shrinkage_v30();


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_immutable_v30; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER fabric_shrinkage_immutable_v30 BEFORE DELETE OR UPDATE ON public.fabric_shrinkage_observations_v30 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: fabric_shrinkage_specs_v31 fabric_shrinkage_spec_guard_v31; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER fabric_shrinkage_spec_guard_v31 BEFORE INSERT ON public.fabric_shrinkage_specs_v31 FOR EACH ROW EXECUTE FUNCTION public.guard_fabric_shrinkage_spec_v31();


--
-- Name: gen_bundles_v18 gen_bundle_guard_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_bundle_guard_v18 BEFORE INSERT ON public.gen_bundles_v18 FOR EACH ROW EXECUTE FUNCTION public.guard_gen_bundle_v18();


--
-- Name: gen_carrier_events_v18 gen_carrier_apply_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_carrier_apply_v18 AFTER INSERT ON public.gen_carrier_events_v18 FOR EACH ROW EXECUTE FUNCTION public.apply_gen_carrier_event_v18();


--
-- Name: gen_garments_v18 gen_garment_coverage_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER gen_garment_coverage_v18 AFTER INSERT ON public.gen_garments_v18 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.check_gen_part_coverage_v18();


--
-- Name: gen_garments_v18 gen_garment_guard_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_garment_guard_v18 BEFORE INSERT ON public.gen_garments_v18 FOR EACH ROW EXECUTE FUNCTION public.guard_gen_garment_v18();


--
-- Name: gen_bundles_v18 gen_immutable_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_immutable_v18 BEFORE DELETE OR UPDATE ON public.gen_bundles_v18 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: gen_carrier_events_v18 gen_immutable_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_immutable_v18 BEFORE DELETE OR UPDATE ON public.gen_carrier_events_v18 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: gen_cut_parts_v18 gen_immutable_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_immutable_v18 BEFORE DELETE OR UPDATE ON public.gen_cut_parts_v18 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: gen_garments_v18 gen_immutable_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_immutable_v18 BEFORE DELETE OR UPDATE ON public.gen_garments_v18 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: cut_bundle_scan_witnesses_v8 gen_no_legacy_bundle_collision_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_no_legacy_bundle_collision_v18 BEFORE INSERT ON public.cut_bundle_scan_witnesses_v8 FOR EACH ROW EXECUTE FUNCTION public.guard_legacy_carrier_collision_v18();


--
-- Name: store_roll_rfid_bindings_v7 gen_no_legacy_roll_collision_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_no_legacy_roll_collision_v18 BEFORE INSERT ON public.store_roll_rfid_bindings_v7 FOR EACH ROW EXECUTE FUNCTION public.guard_legacy_carrier_collision_v18();


--
-- Name: gen_cut_parts_v18 gen_part_guard_v18; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER gen_part_guard_v18 BEFORE INSERT ON public.gen_cut_parts_v18 FOR EACH ROW EXECUTE FUNCTION public.guard_gen_part_v18();


--
-- Name: fabric_shrinkage_specs_v31 immutable_factory_v31; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v31 BEFORE DELETE OR UPDATE ON public.fabric_shrinkage_specs_v31 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: jobwork_discrepancy_resolutions_v31 immutable_factory_v31; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v31 BEFORE DELETE OR UPDATE ON public.jobwork_discrepancy_resolutions_v31 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: buyer_aql_plans_v32 immutable_factory_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v32 BEFORE DELETE OR UPDATE ON public.buyer_aql_plans_v32 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: buyer_aql_sample_events_v32 immutable_factory_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v32 BEFORE DELETE OR UPDATE ON public.buyer_aql_sample_events_v32 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: fabric_calibration_links_v32 immutable_factory_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v32 BEFORE DELETE OR UPDATE ON public.fabric_calibration_links_v32 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: factory_instrument_calibrations_v32 immutable_factory_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v32 BEFORE DELETE OR UPDATE ON public.factory_instrument_calibrations_v32 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: needle_safety_events_v32 immutable_factory_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v32 BEFORE DELETE OR UPDATE ON public.needle_safety_events_v32 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: buyer_aql_review_rules_v33 immutable_factory_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v33 BEFORE DELETE OR UPDATE ON public.buyer_aql_review_rules_v33 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: buyer_aql_reviews_v33 immutable_factory_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v33 BEFORE DELETE OR UPDATE ON public.buyer_aql_reviews_v33 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: needle_safety_clearances_v33 immutable_factory_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_factory_v33 BEFORE DELETE OR UPDATE ON public.needle_safety_clearances_v33 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: wip_piece_holds_v26 immutable_piece_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_piece_v26 BEFORE DELETE OR UPDATE ON public.wip_piece_holds_v26 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: wip_piece_reviews_v26 immutable_piece_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_piece_v26 BEFORE DELETE OR UPDATE ON public.wip_piece_reviews_v26 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: factory_instrument_calibrations_v32 instrument_calibration_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER instrument_calibration_guard_v32 BEFORE INSERT ON public.factory_instrument_calibrations_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_instrument_calibration_v32();


--
-- Name: intel_sim_snapshots_v12 intel_sim_snapshots_immutable_v12; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER intel_sim_snapshots_immutable_v12 BEFORE DELETE OR UPDATE ON public.intel_sim_snapshots_v12 FOR EACH ROW EXECUTE FUNCTION public.intel_sim_snapshot_no_edit_v12();


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancy_guard_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_discrepancy_guard_v29 BEFORE INSERT ON public.jobwork_discrepancies_v29 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_discrepancy_v29();


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancy_immutable_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_discrepancy_immutable_v29 BEFORE DELETE OR UPDATE ON public.jobwork_discrepancies_v29 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_disposition_guard_v31; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_disposition_guard_v31 BEFORE INSERT ON public.jobwork_discrepancy_resolutions_v31 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_disposition_v31();


--
-- Name: jobwork_inwards_v28 jobwork_in_evidence_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_in_evidence_v28 AFTER INSERT ON public.jobwork_inwards_v28 FOR EACH ROW EXECUTE FUNCTION public.jobwork_reserve_evidence_v28();


--
-- Name: jobwork_inwards_v28 jobwork_in_immutable_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_in_immutable_v28 BEFORE DELETE OR UPDATE ON public.jobwork_inwards_v28 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: jobwork_inwards_v28 jobwork_inward_guard_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_inward_guard_v28 BEFORE INSERT ON public.jobwork_inwards_v28 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_inward_v28();


--
-- Name: jobwork_outwards_v28 jobwork_out_evidence_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_out_evidence_v28 AFTER INSERT ON public.jobwork_outwards_v28 FOR EACH ROW EXECUTE FUNCTION public.jobwork_reserve_evidence_v28();


--
-- Name: jobwork_outwards_v28 jobwork_out_immutable_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_out_immutable_v28 BEFORE DELETE OR UPDATE ON public.jobwork_outwards_v28 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: jobwork_outwards_v28 jobwork_outward_guard_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_outward_guard_v28 BEFORE INSERT ON public.jobwork_outwards_v28 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_outward_v28();


--
-- Name: jobwork_used_proofs_v28 jobwork_proof_immutable_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_proof_immutable_v28 BEFORE DELETE OR UPDATE ON public.jobwork_used_proofs_v28 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: jobwork_used_scans_v28 jobwork_scan_immutable_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_scan_immutable_v28 BEFORE DELETE OR UPDATE ON public.jobwork_used_scans_v28 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_ack_guard_v30; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_vendor_ack_guard_v30 BEFORE INSERT ON public.jobwork_vendor_acks_v30 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_vendor_ack_v30();


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_ack_immutable_v30; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER jobwork_vendor_ack_immutable_v30 BEFORE DELETE OR UPDATE ON public.jobwork_vendor_acks_v30 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: material_reservations_v5 material_reservation_guard_v5; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER material_reservation_guard_v5 AFTER INSERT OR UPDATE OF reserved_mm, issued_mm, released_mm ON public.material_reservations_v5 FOR EACH ROW EXECUTE FUNCTION public.check_material_reservation_v5();


--
-- Name: material_reservations_v5 material_single_order_v7; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER material_single_order_v7 BEFORE INSERT OR UPDATE OF roll_id, order_id ON public.material_reservations_v5 FOR EACH ROW EXECUTE FUNCTION public.enforce_roll_single_order_v7();


--
-- Name: material_variances_v5 material_variance_immutability_v5; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER material_variance_immutability_v5 BEFORE UPDATE ON public.material_variances_v5 FOR EACH ROW EXECUTE FUNCTION public.protect_material_variance_v5();


--
-- Name: needle_safety_clearances_v33 needle_safety_clearance_guard_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER needle_safety_clearance_guard_v33 BEFORE INSERT ON public.needle_safety_clearances_v33 FOR EACH ROW EXECUTE FUNCTION public.guard_needle_safety_clearance_v33();


--
-- Name: needle_safety_events_v32 needle_safety_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER needle_safety_guard_v32 BEFORE INSERT ON public.needle_safety_events_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_needle_safety_event_v32();


--
-- Name: pack_carton_events_v22 needle_safety_packing_gate_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER needle_safety_packing_gate_v32 BEFORE INSERT ON public.pack_carton_events_v22 FOR EACH ROW EXECUTE FUNCTION public.guard_needle_safety_progress_v32();


--
-- Name: needle_safety_events_v32 needle_safety_proof_guard_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER needle_safety_proof_guard_v32 BEFORE INSERT ON public.needle_safety_events_v32 FOR EACH ROW EXECUTE FUNCTION public.guard_factory_proof_v32();


--
-- Name: qc_garment_events_v21 needle_safety_qc_gate_v32; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER needle_safety_qc_gate_v32 BEFORE INSERT ON public.qc_garment_events_v21 FOR EACH ROW EXECUTE FUNCTION public.guard_needle_safety_progress_v32();


--
-- Name: pack_carton_events_v22 pack_carton_event_guard_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_carton_event_guard_v22 BEFORE INSERT ON public.pack_carton_events_v22 FOR EACH ROW EXECUTE FUNCTION public.guard_pack_carton_event_v22();


--
-- Name: pack_carton_events_v22 pack_carton_immutable_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_carton_immutable_v22 BEFORE DELETE OR UPDATE ON public.pack_carton_events_v22 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: pack_manifest_events_v22 pack_manifest_event_guard_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_manifest_event_guard_v22 BEFORE INSERT ON public.pack_manifest_events_v22 FOR EACH ROW EXECUTE FUNCTION public.guard_pack_manifest_event_v22();


--
-- Name: pack_manifest_events_v22 pack_manifest_immutable_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_manifest_immutable_v22 BEFORE DELETE OR UPDATE ON public.pack_manifest_events_v22 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: pack_prod_holds_v12 pack_prod_hold_guard_v12; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_prod_hold_guard_v12 BEFORE INSERT ON public.pack_prod_holds_v12 FOR EACH ROW EXECUTE FUNCTION public.guard_pack_prod_hold_v12();


--
-- Name: pack_prod_holds_v12 pack_prod_hold_immutable_v12; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_prod_hold_immutable_v12 BEFORE DELETE OR UPDATE ON public.pack_prod_holds_v12 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: pack_sim_cartons_v11 pack_sim_carton_state_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_sim_carton_state_v11 BEFORE UPDATE ON public.pack_sim_cartons_v11 FOR EACH ROW EXECUTE FUNCTION public.pack_sim_carton_state_v11();


--
-- Name: pack_sim_events_v11 pack_sim_events_immutable_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_sim_events_immutable_v11 BEFORE DELETE OR UPDATE ON public.pack_sim_events_v11 FOR EACH ROW EXECUTE FUNCTION public.pack_sim_immutable_v11();


--
-- Name: pack_sim_items_v11 pack_sim_item_delete_guard_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_sim_item_delete_guard_v11 BEFORE DELETE ON public.pack_sim_items_v11 FOR EACH ROW EXECUTE FUNCTION public.pack_sim_item_delete_guard_v11();


--
-- Name: pack_sim_items_v11 pack_sim_item_guard_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_sim_item_guard_v11 BEFORE INSERT ON public.pack_sim_items_v11 FOR EACH ROW EXECUTE FUNCTION public.pack_sim_item_guard_v11();


--
-- Name: pack_sim_manifest_cartons_v11 pack_sim_manifest_cartons_immutable_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_sim_manifest_cartons_immutable_v11 BEFORE DELETE OR UPDATE ON public.pack_sim_manifest_cartons_v11 FOR EACH ROW EXECUTE FUNCTION public.pack_sim_immutable_v11();


--
-- Name: pack_sim_manifest_cartons_v11 pack_sim_manifest_guard_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_sim_manifest_guard_v11 BEFORE INSERT ON public.pack_sim_manifest_cartons_v11 FOR EACH ROW EXECUTE FUNCTION public.pack_sim_manifest_guard_v11();


--
-- Name: pack_sim_manifests_v11 pack_sim_manifest_state_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_sim_manifest_state_v11 BEFORE UPDATE ON public.pack_sim_manifests_v11 FOR EACH ROW EXECUTE FUNCTION public.pack_sim_manifest_state_v11();


--
-- Name: pack_size_weights_v22 pack_size_rule_immutable_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_size_rule_immutable_v22 BEFORE DELETE OR UPDATE ON public.pack_size_weights_v22 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: pack_size_weights_v22 pack_size_rule_insert_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_size_rule_insert_v22 BEFORE INSERT ON public.pack_size_weights_v22 FOR EACH ROW EXECUTE FUNCTION public.pack_size_rule_guard_v22();


--
-- Name: pack_carton_specs_v22 pack_spec_insert_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_spec_insert_v22 BEFORE INSERT ON public.pack_carton_specs_v22 FOR EACH ROW EXECUTE FUNCTION public.pack_spec_insert_guard_v22();


--
-- Name: pack_carton_specs_v22 pack_spec_update_v22; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER pack_spec_update_v22 BEFORE DELETE OR UPDATE ON public.pack_carton_specs_v22 FOR EACH ROW EXECUTE FUNCTION public.pack_spec_update_guard_v22();


--
-- Name: qc_garment_events_v21 qc_events_lock_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_events_lock_v21 BEFORE DELETE OR UPDATE ON public.qc_garment_events_v21 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: gen_cut_parts_v18 qc_freeze_parts_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_freeze_parts_v21 BEFORE INSERT ON public.gen_cut_parts_v18 FOR EACH ROW EXECUTE FUNCTION public.qc_freeze_garment_parts_v21();


--
-- Name: cut_recut_links_v19 qc_freeze_recut_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_freeze_recut_v21 BEFORE INSERT ON public.cut_recut_links_v19 FOR EACH ROW EXECUTE FUNCTION public.qc_freeze_recut_v21();


--
-- Name: qc_garment_events_v21 qc_garment_guard_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_garment_guard_v21 BEFORE INSERT ON public.qc_garment_events_v21 FOR EACH ROW EXECUTE FUNCTION public.guard_qc_garment_event_v21();


--
-- Name: qc_prod_group_events_v11 qc_prod_group_guard_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_prod_group_guard_v11 BEFORE INSERT ON public.qc_prod_group_events_v11 FOR EACH ROW EXECUTE FUNCTION public.guard_qc_prod_group_event_v11();


--
-- Name: qc_prod_group_events_v11 qc_prod_group_immutable_v11; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_prod_group_immutable_v11 BEFORE DELETE OR UPDATE ON public.qc_prod_group_events_v11 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: qc_garment_measure_rules_v21 qc_rule_insert_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_rule_insert_v21 BEFORE INSERT ON public.qc_garment_measure_rules_v21 FOR EACH ROW EXECUTE FUNCTION public.qc_rule_insert_guard_v21();


--
-- Name: qc_garment_measure_rules_v21 qc_rules_lock_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_rules_lock_v21 BEFORE DELETE OR UPDATE ON public.qc_garment_measure_rules_v21 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_buyer_gate_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_sim_buyer_gate_v10 BEFORE INSERT ON public.qc_sim_buyer_bindings_v10 FOR EACH ROW EXECUTE FUNCTION public.qc_sim_buyer_gate_v10();


--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_buyer_immutable_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_sim_buyer_immutable_v10 BEFORE DELETE OR UPDATE ON public.qc_sim_buyer_bindings_v10 FOR EACH ROW EXECUTE FUNCTION public.qc_sim_no_edit_v10();


--
-- Name: qc_sim_events_v10 qc_sim_events_immutable_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_sim_events_immutable_v10 BEFORE DELETE OR UPDATE ON public.qc_sim_events_v10 FOR EACH ROW EXECUTE FUNCTION public.qc_sim_no_edit_v10();


--
-- Name: qc_sim_routes_v10 qc_sim_routes_immutable_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_sim_routes_immutable_v10 BEFORE DELETE OR UPDATE ON public.qc_sim_routes_v10 FOR EACH ROW EXECUTE FUNCTION public.qc_sim_no_edit_v10();


--
-- Name: qc_sim_units_v10 qc_sim_source_guard_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_sim_source_guard_v10 BEFORE INSERT ON public.qc_sim_units_v10 FOR EACH ROW EXECUTE FUNCTION public.qc_sim_source_guard_v10();


--
-- Name: qc_sim_units_v10 qc_sim_unit_guard_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_sim_unit_guard_v10 BEFORE DELETE OR UPDATE ON public.qc_sim_units_v10 FOR EACH ROW EXECUTE FUNCTION public.qc_sim_unit_guard_v10();


--
-- Name: qc_garment_specs_v21 qc_spec_guard_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_spec_guard_v21 BEFORE INSERT ON public.qc_garment_specs_v21 FOR EACH ROW EXECUTE FUNCTION public.guard_qc_spec_v21();


--
-- Name: qc_garment_specs_v21 qc_spec_lock_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER qc_spec_lock_v21 BEFORE DELETE OR UPDATE ON public.qc_garment_specs_v21 FOR EACH ROW EXECUTE FUNCTION public.qc_spec_immutable_v21();


--
-- Name: needle_safety_clearances_v33 safety_review_proof_v33; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER safety_review_proof_v33 BEFORE INSERT ON public.needle_safety_clearances_v33 FOR EACH ROW EXECUTE FUNCTION public.guard_factory_review_proof_v33();


--
-- Name: sew_piece_completions_v27 sew_piece_completion_guard_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_piece_completion_guard_v27 BEFORE INSERT ON public.sew_piece_completions_v27 FOR EACH ROW EXECUTE FUNCTION public.guard_sew_piece_completion_v27();


--
-- Name: sew_piece_completions_v27 sew_piece_immutable_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_piece_immutable_v27 BEFORE DELETE OR UPDATE ON public.sew_piece_completions_v27 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: sew_piece_used_scans_v27 sew_piece_immutable_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_piece_immutable_v27 BEFORE DELETE OR UPDATE ON public.sew_piece_used_scans_v27 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: sew_piece_completions_v27 sew_piece_scans_once_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_piece_scans_once_v27 AFTER INSERT ON public.sew_piece_completions_v27 FOR EACH ROW EXECUTE FUNCTION public.reserve_sew_piece_scans_v27();


--
-- Name: sew_prod_completions_v10 sew_prod_completion_guard_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_prod_completion_guard_v10 BEFORE INSERT ON public.sew_prod_completions_v10 FOR EACH ROW EXECUTE FUNCTION public.guard_sew_prod_completion_v10();


--
-- Name: sew_prod_completions_v10 sew_prod_completion_immutable_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_prod_completion_immutable_v10 BEFORE DELETE OR UPDATE ON public.sew_prod_completions_v10 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: sew_prod_completions_v10 sew_prod_consume_observations_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_prod_consume_observations_v10 AFTER INSERT ON public.sew_prod_completions_v10 FOR EACH ROW EXECUTE FUNCTION public.reserve_sew_prod_observations_v10();


--
-- Name: sew_prod_observations_v10 sew_prod_observation_guard_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_prod_observation_guard_v10 BEFORE INSERT ON public.sew_prod_observations_v10 FOR EACH ROW EXECUTE FUNCTION public.guard_sew_prod_observation_v10();


--
-- Name: sew_prod_observations_v10 sew_prod_observation_immutable_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_prod_observation_immutable_v10 BEFORE DELETE OR UPDATE ON public.sew_prod_observations_v10 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: sew_prod_used_observations_v10 sew_prod_used_observation_immutable_v10; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_prod_used_observation_immutable_v10 BEFORE DELETE OR UPDATE ON public.sew_prod_used_observations_v10 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: sew_sim_credits_v9 sew_sim_credit_guard_v9; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_sim_credit_guard_v9 BEFORE INSERT ON public.sew_sim_credits_v9 FOR EACH ROW EXECUTE FUNCTION public.validate_sew_sim_credit_v9();


--
-- Name: sew_sim_cycles_v9 sew_sim_cycle_state_guard_v9; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_sim_cycle_state_guard_v9 BEFORE DELETE OR UPDATE ON public.sew_sim_cycles_v9 FOR EACH ROW EXECUTE FUNCTION public.guard_sew_sim_cycle_transition_v9();


--
-- Name: sew_sim_credits_v9 sew_sim_immutable_guard_v9; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_sim_immutable_guard_v9 BEFORE DELETE OR UPDATE ON public.sew_sim_credits_v9 FOR EACH ROW EXECUTE FUNCTION public.sew_sim_immutable_v9();


--
-- Name: sew_sim_cycle_inputs_v9 sew_sim_immutable_guard_v9; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_sim_immutable_guard_v9 BEFORE DELETE OR UPDATE ON public.sew_sim_cycle_inputs_v9 FOR EACH ROW EXECUTE FUNCTION public.sew_sim_immutable_v9();


--
-- Name: sew_sim_events_v9 sew_sim_immutable_guard_v9; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_sim_immutable_guard_v9 BEFORE DELETE OR UPDATE ON public.sew_sim_events_v9 FOR EACH ROW EXECUTE FUNCTION public.sew_sim_immutable_v9();


--
-- Name: sew_sim_operations_v9 sew_sim_immutable_guard_v9; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_sim_immutable_guard_v9 BEFORE DELETE OR UPDATE ON public.sew_sim_operations_v9 FOR EACH ROW EXECUTE FUNCTION public.sew_sim_immutable_v9();


--
-- Name: sew_sim_scans_v9 sew_sim_immutable_guard_v9; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sew_sim_immutable_guard_v9 BEFORE DELETE OR UPDATE ON public.sew_sim_scans_v9 FOR EACH ROW EXECUTE FUNCTION public.sew_sim_immutable_v9();


--
-- Name: store_pick_proposals_v7 store_pick_proposal_immutable_v7; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER store_pick_proposal_immutable_v7 BEFORE DELETE OR UPDATE ON public.store_pick_proposals_v7 FOR EACH ROW EXECUTE FUNCTION public.protect_stores_pick_proposal_v7();


--
-- Name: store_roll_admissions_v7 store_roll_admission_immutable_v7; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER store_roll_admission_immutable_v7 BEFORE DELETE OR UPDATE ON public.store_roll_admissions_v7 FOR EACH ROW EXECUTE FUNCTION public.protect_stores_admission_v7();


--
-- Name: store_roll_admission_proofs_v7 store_roll_admission_proof_immutable_v7; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER store_roll_admission_proof_immutable_v7 BEFORE DELETE OR UPDATE ON public.store_roll_admission_proofs_v7 FOR EACH ROW EXECUTE FUNCTION public.protect_stores_admission_v7();


--
-- Name: trim_stock_admissions_v25 trim_admission_cross_ledger_guard_v25; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_admission_cross_ledger_guard_v25 BEFORE INSERT ON public.trim_stock_admissions_v25 FOR EACH ROW EXECUTE FUNCTION public.trim_cross_ledger_proof_guard_v25();


--
-- Name: trim_stock_admissions_v25 trim_admission_immutable_v25; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_admission_immutable_v25 BEFORE DELETE OR UPDATE ON public.trim_stock_admissions_v25 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();


--
-- Name: trim_admission_proof_uses_v25 trim_admission_proof_immutable_v25; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_admission_proof_immutable_v25 BEFORE DELETE OR UPDATE ON public.trim_admission_proof_uses_v25 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();


--
-- Name: trim_stock_admissions_v25 trim_admission_stock_v25; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_admission_stock_v25 AFTER INSERT ON public.trim_stock_admissions_v25 FOR EACH ROW EXECUTE FUNCTION public.guard_trim_admission_v25();


--
-- Name: trim_issuances_v13 trim_issuance_conservation_v24; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_issuance_conservation_v24 AFTER INSERT ON public.trim_issuances_v13 FOR EACH ROW EXECUTE FUNCTION public.guard_trim_issuance_v24();


--
-- Name: trim_issuances_v13 trim_issuance_cross_ledger_guard_v25; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_issuance_cross_ledger_guard_v25 BEFORE INSERT ON public.trim_issuances_v13 FOR EACH ROW EXECUTE FUNCTION public.trim_cross_ledger_proof_guard_v25();


--
-- Name: trim_issuances_v13 trim_issuance_immutable_v24; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_issuance_immutable_v24 BEFORE DELETE OR UPDATE ON public.trim_issuances_v13 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();


--
-- Name: trim_issuance_proof_uses_v24 trim_proof_immutable_v24; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trim_proof_immutable_v24 BEFORE DELETE OR UPDATE ON public.trim_issuance_proof_uses_v24 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();


--
-- Name: wip_exception_closures_v23 wip_exception_close_guard_v23; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_exception_close_guard_v23 BEFORE INSERT ON public.wip_exception_closures_v23 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_exception_close_v23();


--
-- Name: wip_exception_cases_v23 wip_exception_immutable_v23; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_exception_immutable_v23 BEFORE DELETE OR UPDATE ON public.wip_exception_cases_v23 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: wip_exception_closures_v23 wip_exception_immutable_v23; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_exception_immutable_v23 BEFORE DELETE OR UPDATE ON public.wip_exception_closures_v23 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: wip_exception_cases_v23 wip_exception_open_guard_v23; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_exception_open_guard_v23 BEFORE INSERT ON public.wip_exception_cases_v23 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_exception_open_v23();


--
-- Name: wip_gateway_scans_v20 wip_immutable_v20; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_immutable_v20 BEFORE DELETE OR UPDATE ON public.wip_gateway_scans_v20 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: wip_movements_v20 wip_immutable_v20; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_immutable_v20 BEFORE DELETE OR UPDATE ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.protect_cut_draft_v8();


--
-- Name: wip_movements_v20 wip_move_guard_v20; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_move_guard_v20 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_move_v20();


--
-- Name: wip_piece_holds_v26 wip_piece_open_guard_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_piece_open_guard_v26 BEFORE INSERT ON public.wip_piece_holds_v26 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_piece_hold_v26();


--
-- Name: wip_piece_reviews_v26 wip_piece_review_guard_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_piece_review_guard_v26 BEFORE INSERT ON public.wip_piece_reviews_v26 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_piece_review_v26();


--
-- Name: wip_movements_v20 wip_quality_release_fence_v21; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_quality_release_fence_v21 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_quality_release_v21();


--
-- Name: wip_gateway_scans_v20 wip_scan_guard_v20; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER wip_scan_guard_v20 BEFORE INSERT ON public.wip_gateway_scans_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_scan_v20();


--
-- Name: qc_garment_events_v21 zz_qc_exception_hold_v23; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zz_qc_exception_hold_v23 BEFORE INSERT ON public.qc_garment_events_v21 FOR EACH ROW EXECUTE FUNCTION public.guard_qc_exception_hold_v23();


--
-- Name: qc_garment_events_v21 zz_qc_piece_hold_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zz_qc_piece_hold_v26 BEFORE INSERT ON public.qc_garment_events_v21 FOR EACH ROW EXECUTE FUNCTION public.guard_qc_piece_hold_v26();


--
-- Name: sew_prod_completions_v10 zz_sew_completion_piece_hold_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zz_sew_completion_piece_hold_v26 BEFORE INSERT ON public.sew_prod_completions_v10 FOR EACH ROW EXECUTE FUNCTION public.guard_sew_completion_piece_hold_v26();


--
-- Name: wip_movements_v20 zz_wip_exception_hold_v23; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zz_wip_exception_hold_v23 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_exception_hold_v23();


--
-- Name: wip_movements_v20 zz_wip_piece_hold_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zz_wip_piece_hold_v26 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_wip_piece_move_hold_v26();


--
-- Name: sew_prod_completions_v10 zzz_jobwork_disputed_group_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_disputed_group_v29 BEFORE INSERT ON public.sew_prod_completions_v10 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_disputed_progress_v29();


--
-- Name: wip_movements_v20 zzz_jobwork_disputed_move_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_disputed_move_v29 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_disputed_progress_v29();


--
-- Name: jobwork_outwards_v28 zzz_jobwork_disputed_out_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_disputed_out_v29 BEFORE INSERT ON public.jobwork_outwards_v28 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_disputed_progress_v29();


--
-- Name: pack_carton_events_v22 zzz_jobwork_disputed_pack_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_disputed_pack_v29 BEFORE INSERT ON public.pack_carton_events_v22 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_disputed_progress_v29();


--
-- Name: sew_piece_completions_v27 zzz_jobwork_disputed_piece_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_disputed_piece_v29 BEFORE INSERT ON public.sew_piece_completions_v27 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_disputed_progress_v29();


--
-- Name: qc_garment_events_v21 zzz_jobwork_disputed_qc_v29; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_disputed_qc_v29 BEFORE INSERT ON public.qc_garment_events_v21 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_disputed_progress_v29();


--
-- Name: wip_movements_v20 zzz_jobwork_offsite_move_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_offsite_move_v28 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_offsite_v28();


--
-- Name: pack_carton_events_v22 zzz_jobwork_offsite_pack_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_offsite_pack_v28 BEFORE INSERT ON public.pack_carton_events_v22 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_offsite_v28();


--
-- Name: sew_piece_completions_v27 zzz_jobwork_offsite_piece_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_offsite_piece_v28 BEFORE INSERT ON public.sew_piece_completions_v27 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_offsite_v28();


--
-- Name: qc_garment_events_v21 zzz_jobwork_offsite_qc_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_offsite_qc_v28 BEFORE INSERT ON public.qc_garment_events_v21 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_offsite_v28();


--
-- Name: sew_prod_completions_v10 zzz_jobwork_offsite_whole_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_offsite_whole_v28 BEFORE INSERT ON public.sew_prod_completions_v10 FOR EACH ROW EXECUTE FUNCTION public.guard_jobwork_offsite_v28();


--
-- Name: wip_exception_closures_v23 zzz_jobwork_scan_exception_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_scan_exception_v28 BEFORE INSERT ON public.wip_exception_closures_v23 FOR EACH ROW EXECUTE FUNCTION public.reject_jobwork_scan_reuse_v28();


--
-- Name: wip_movements_v20 zzz_jobwork_scan_move_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_scan_move_v28 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.reject_jobwork_scan_reuse_v28();


--
-- Name: sew_piece_completions_v27 zzz_jobwork_scan_piece_complete_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_scan_piece_complete_v28 BEFORE INSERT ON public.sew_piece_completions_v27 FOR EACH ROW EXECUTE FUNCTION public.reject_jobwork_scan_reuse_v28();


--
-- Name: wip_piece_reviews_v26 zzz_jobwork_scan_piece_review_v28; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_jobwork_scan_piece_review_v28 BEFORE INSERT ON public.wip_piece_reviews_v26 FOR EACH ROW EXECUTE FUNCTION public.reject_jobwork_scan_reuse_v28();


--
-- Name: wip_exception_closures_v23 zzz_piece_scan_used_in_exception_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_piece_scan_used_in_exception_v26 BEFORE INSERT ON public.wip_exception_closures_v23 FOR EACH ROW EXECUTE FUNCTION public.guard_piece_review_scan_reuse_v26();


--
-- Name: wip_movements_v20 zzz_piece_scan_used_in_wip_v26; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_piece_scan_used_in_wip_v26 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.guard_piece_review_scan_reuse_v26();


--
-- Name: sew_prod_completions_v10 zzz_sew_piece_no_group_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_sew_piece_no_group_v27 BEFORE INSERT ON public.sew_prod_completions_v10 FOR EACH ROW EXECUTE FUNCTION public.reject_whole_sew_when_piece_v27();


--
-- Name: wip_exception_closures_v23 zzz_sew_piece_scan_exception_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_sew_piece_scan_exception_v27 BEFORE INSERT ON public.wip_exception_closures_v23 FOR EACH ROW EXECUTE FUNCTION public.reject_reused_sew_piece_scan_v27();


--
-- Name: wip_movements_v20 zzz_sew_piece_scan_move_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_sew_piece_scan_move_v27 BEFORE INSERT ON public.wip_movements_v20 FOR EACH ROW EXECUTE FUNCTION public.reject_reused_sew_piece_scan_v27();


--
-- Name: wip_piece_reviews_v26 zzz_sew_piece_scan_review_v27; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER zzz_sew_piece_scan_review_v27 BEFORE INSERT ON public.wip_piece_reviews_v26 FOR EACH ROW EXECUTE FUNCTION public.reject_reused_sew_piece_scan_v27();


--
-- Name: attendance_register attendance_register_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attendance_register
    ADD CONSTRAINT attendance_register_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: attendance_register attendance_register_worker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attendance_register
    ADD CONSTRAINT attendance_register_worker_id_fkey FOREIGN KEY (worker_id) REFERENCES public.workers(id) ON DELETE CASCADE;


--
-- Name: bom_trim_requirements bom_trim_requirements_style_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bom_trim_requirements
    ADD CONSTRAINT bom_trim_requirements_style_id_fkey FOREIGN KEY (style_id) REFERENCES public.styles_master(id) ON DELETE CASCADE;


--
-- Name: bom_trim_requirements bom_trim_requirements_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bom_trim_requirements
    ADD CONSTRAINT bom_trim_requirements_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: bundle_registry bundle_registry_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bundle_registry
    ADD CONSTRAINT bundle_registry_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_plans_v32 buyer_aql_plans_v32_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_plans_v32
    ADD CONSTRAINT buyer_aql_plans_v32_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_plans_v32 buyer_aql_plans_v32_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_plans_v32
    ADD CONSTRAINT buyer_aql_plans_v32_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_plans_v32 buyer_aql_plans_v32_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_plans_v32
    ADD CONSTRAINT buyer_aql_plans_v32_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.documents(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_review_rules_v33 buyer_aql_review_rules_v33_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_review_rules_v33
    ADD CONSTRAINT buyer_aql_review_rules_v33_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_review_rules_v33 buyer_aql_review_rules_v33_tenant_id_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_review_rules_v33
    ADD CONSTRAINT buyer_aql_review_rules_v33_tenant_id_plan_id_fkey FOREIGN KEY (tenant_id, plan_id) REFERENCES public.buyer_aql_plans_v32(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_review_rules_v33 buyer_aql_review_rules_v33_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_review_rules_v33
    ADD CONSTRAINT buyer_aql_review_rules_v33_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.documents(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_reviews_v33 buyer_aql_reviews_v33_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_reviews_v33
    ADD CONSTRAINT buyer_aql_reviews_v33_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_reviews_v33 buyer_aql_reviews_v33_tenant_id_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_reviews_v33
    ADD CONSTRAINT buyer_aql_reviews_v33_tenant_id_plan_id_fkey FOREIGN KEY (tenant_id, plan_id) REFERENCES public.buyer_aql_plans_v32(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_events_v32_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_sample_events_v32
    ADD CONSTRAINT buyer_aql_sample_events_v32_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_events_v32_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_sample_events_v32
    ADD CONSTRAINT buyer_aql_sample_events_v32_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_aql_sample_events_v32 buyer_aql_sample_events_v32_tenant_id_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_aql_sample_events_v32
    ADD CONSTRAINT buyer_aql_sample_events_v32_tenant_id_plan_id_fkey FOREIGN KEY (tenant_id, plan_id) REFERENCES public.buyer_aql_plans_v32(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_order_handoffs_v65c1 buyer_order_handoffs_v65c1_tenant_id_bom_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_order_handoffs_v65c1
    ADD CONSTRAINT buyer_order_handoffs_v65c1_tenant_id_bom_document_id_fkey FOREIGN KEY (tenant_id, bom_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_order_handoffs_v65c1 buyer_order_handoffs_v65c1_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_order_handoffs_v65c1
    ADD CONSTRAINT buyer_order_handoffs_v65c1_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: buyer_order_handoffs_v65c1 buyer_order_handoffs_v65c1_tenant_id_intake_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_order_handoffs_v65c1
    ADD CONSTRAINT buyer_order_handoffs_v65c1_tenant_id_intake_id_fkey FOREIGN KEY (tenant_id, intake_id) REFERENCES public.order_intakes_v5(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: buyer_order_handoffs_v65c1 buyer_order_handoffs_v65c1_tenant_id_po_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buyer_order_handoffs_v65c1
    ADD CONSTRAINT buyer_order_handoffs_v65c1_tenant_id_po_document_id_fkey FOREIGN KEY (tenant_id, po_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: carton_pack_manifests carton_pack_manifests_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.carton_pack_manifests
    ADD CONSTRAINT carton_pack_manifests_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_bundle_drafts_v8 cut_bundle_drafts_v8_tenant_id_lay_draft_id_marker_positio_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_drafts_v8
    ADD CONSTRAINT cut_bundle_drafts_v8_tenant_id_lay_draft_id_marker_positio_fkey FOREIGN KEY (tenant_id, lay_draft_id, marker_position) REFERENCES public.cut_lay_positions_v8(tenant_id, lay_draft_id, marker_position) ON DELETE RESTRICT;


--
-- Name: cut_bundle_scan_witnesses_v8 cut_bundle_scan_witnesses_v8_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8
    ADD CONSTRAINT cut_bundle_scan_witnesses_v8_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_bundle_scan_witnesses_v8 cut_bundle_scan_witnesses_v8_tenant_id_lay_draft_id_label__fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8
    ADD CONSTRAINT cut_bundle_scan_witnesses_v8_tenant_id_lay_draft_id_label__fkey FOREIGN KEY (tenant_id, lay_draft_id, label_code) REFERENCES public.cut_bundle_drafts_v8(tenant_id, lay_draft_id, label_code) ON DELETE RESTRICT;


--
-- Name: cut_bundle_scan_witnesses_v8 cut_bundle_scan_witnesses_v8_tenant_id_spread_report_id_la_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_bundle_scan_witnesses_v8
    ADD CONSTRAINT cut_bundle_scan_witnesses_v8_tenant_id_spread_report_id_la_fkey FOREIGN KEY (tenant_id, spread_report_id, lay_draft_id) REFERENCES public.cut_spread_reports_v8(tenant_id, id, lay_draft_id) ON DELETE RESTRICT;


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_reconciliations_tenant_id_spread_report_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19
    ADD CONSTRAINT cut_consumption_reconciliations_tenant_id_spread_report_id_fkey FOREIGN KEY (tenant_id, spread_report_id) REFERENCES public.cut_spread_reports_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_reconciliations_v19_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19
    ADD CONSTRAINT cut_consumption_reconciliations_v19_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_consumption_reconciliations_v19 cut_consumption_reconciliations_v19_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_consumption_reconciliations_v19
    ADD CONSTRAINT cut_consumption_reconciliations_v19_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_defect_codes_v19 cut_defect_codes_v19_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_defect_codes_v19
    ADD CONSTRAINT cut_defect_codes_v19_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_lay_drafts_v8 cut_lay_drafts_v8_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_drafts_v8
    ADD CONSTRAINT cut_lay_drafts_v8_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_lay_drafts_v8 cut_lay_drafts_v8_tenant_id_intake_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_drafts_v8
    ADD CONSTRAINT cut_lay_drafts_v8_tenant_id_intake_id_fkey FOREIGN KEY (tenant_id, intake_id) REFERENCES public.order_intakes_v5(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_lay_positions_v8 cut_lay_positions_v8_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_positions_v8
    ADD CONSTRAINT cut_lay_positions_v8_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_lay_rolls cut_lay_rolls_roll_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_rolls
    ADD CONSTRAINT cut_lay_rolls_roll_id_fkey FOREIGN KEY (roll_id) REFERENCES public.store_rolls(id) ON DELETE RESTRICT;


--
-- Name: cut_lay_rolls cut_lay_rolls_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_rolls
    ADD CONSTRAINT cut_lay_rolls_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_lay_snapshots_v8 cut_lay_snapshots_v8_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_lay_snapshots_v8
    ADD CONSTRAINT cut_lay_snapshots_v8_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_part_outcomes_v19 cut_part_outcomes_v19_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_part_outcomes_v19
    ADD CONSTRAINT cut_part_outcomes_v19_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_part_outcomes_v19 cut_part_outcomes_v19_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_part_outcomes_v19
    ADD CONSTRAINT cut_part_outcomes_v19_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_part_outcomes_v19 cut_part_outcomes_v19_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_part_outcomes_v19
    ADD CONSTRAINT cut_part_outcomes_v19_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_proof_consumptions_v19 cut_proof_consumptions_v19_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_proof_consumptions_v19
    ADD CONSTRAINT cut_proof_consumptions_v19_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_tenant_id_gateway_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_tenant_id_gateway_id_fkey FOREIGN KEY (tenant_id, gateway_id) REFERENCES public.cut_reader_gateways_v8(tenant_id, gateway_id) ON DELETE RESTRICT;


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_tenant_id_lay_draft_id_label_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_tenant_id_lay_draft_id_label_code_fkey FOREIGN KEY (tenant_id, lay_draft_id, label_code) REFERENCES public.cut_bundle_drafts_v8(tenant_id, lay_draft_id, label_code) ON DELETE RESTRICT;


--
-- Name: cut_reader_events_v8 cut_reader_events_v8_tenant_id_spread_report_id_lay_draft__fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_events_v8
    ADD CONSTRAINT cut_reader_events_v8_tenant_id_spread_report_id_lay_draft__fkey FOREIGN KEY (tenant_id, spread_report_id, lay_draft_id) REFERENCES public.cut_spread_reports_v8(tenant_id, id, lay_draft_id) ON DELETE RESTRICT;


--
-- Name: cut_reader_gateways_v8 cut_reader_gateways_v8_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_reader_gateways_v8
    ADD CONSTRAINT cut_reader_gateways_v8_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_tenant_id_original_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_tenant_id_original_part_id_fkey FOREIGN KEY (tenant_id, original_part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_recut_links_v19 cut_recut_links_v19_tenant_id_replacement_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_recut_links_v19
    ADD CONSTRAINT cut_recut_links_v19_tenant_id_replacement_part_id_fkey FOREIGN KEY (tenant_id, replacement_part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_sewing_releases_v8 cut_sewing_releases_v8_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_sewing_releases_v8
    ADD CONSTRAINT cut_sewing_releases_v8_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_sewing_releases_v8 cut_sewing_releases_v8_tenant_id_spread_report_id_lay_draf_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_sewing_releases_v8
    ADD CONSTRAINT cut_sewing_releases_v8_tenant_id_spread_report_id_lay_draf_fkey FOREIGN KEY (tenant_id, spread_report_id, lay_draft_id) REFERENCES public.cut_spread_reports_v8(tenant_id, id, lay_draft_id) ON DELETE RESTRICT;


--
-- Name: cut_spread_issue_allocations_v8 cut_spread_issue_allocations_v8_tenant_id_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_issue_allocations_v8
    ADD CONSTRAINT cut_spread_issue_allocations_v8_tenant_id_issue_id_fkey FOREIGN KEY (tenant_id, issue_id) REFERENCES public.material_issues_v5(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_spread_issue_allocations_v8 cut_spread_issue_allocations_v8_tenant_id_spread_report_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_issue_allocations_v8
    ADD CONSTRAINT cut_spread_issue_allocations_v8_tenant_id_spread_report_id_fkey FOREIGN KEY (tenant_id, spread_report_id) REFERENCES public.cut_spread_reports_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: cut_spread_reports_v8 cut_spread_reports_v8_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_reports_v8
    ADD CONSTRAINT cut_spread_reports_v8_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: cut_spread_reports_v8 cut_spread_reports_v8_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cut_spread_reports_v8
    ADD CONSTRAINT cut_spread_reports_v8_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: dock_asn_drafts_v34 dock_asn_drafts_v34_tenant_id_carrier_receipt_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_asn_drafts_v34
    ADD CONSTRAINT dock_asn_drafts_v34_tenant_id_carrier_receipt_id_fkey FOREIGN KEY (tenant_id, carrier_receipt_id) REFERENCES public.dock_carrier_receipts_v34(tenant_id, id);


--
-- Name: dock_asn_drafts_v34 dock_asn_drafts_v34_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_asn_drafts_v34
    ADD CONSTRAINT dock_asn_drafts_v34_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_buyer_spec_bindings_v36 dock_buyer_spec_bindings_v36_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_buyer_spec_bindings_v36
    ADD CONSTRAINT dock_buyer_spec_bindings_v36_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_buyer_spec_bindings_v36 dock_buyer_spec_bindings_v36_tenant_id_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_buyer_spec_bindings_v36
    ADD CONSTRAINT dock_buyer_spec_bindings_v36_tenant_id_spec_id_fkey FOREIGN KEY (tenant_id, spec_id) REFERENCES public.pack_carton_specs_v22(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: dock_carrier_authorizations_v34 dock_carrier_authorizations_v34_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_authorizations_v34
    ADD CONSTRAINT dock_carrier_authorizations_v34_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_carrier_receipts_v34 dock_carrier_receipts_v34_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_receipts_v34
    ADD CONSTRAINT dock_carrier_receipts_v34_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_carrier_receipts_v34 dock_carrier_receipts_v34_tenant_id_seal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carrier_receipts_v34
    ADD CONSTRAINT dock_carrier_receipts_v34_tenant_id_seal_id_fkey FOREIGN KEY (tenant_id, seal_id) REFERENCES public.dock_container_seals_v34(tenant_id, id);


--
-- Name: dock_carton_scans_v34 dock_carton_scans_v34_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carton_scans_v34
    ADD CONSTRAINT dock_carton_scans_v34_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_carton_scans_v34 dock_carton_scans_v34_tenant_id_gateway_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_carton_scans_v34
    ADD CONSTRAINT dock_carton_scans_v34_tenant_id_gateway_id_fkey FOREIGN KEY (tenant_id, gateway_id) REFERENCES public.dock_gateways_v34(tenant_id, gateway_id);


--
-- Name: dock_container_seals_v34 dock_container_seals_v34_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_container_seals_v34
    ADD CONSTRAINT dock_container_seals_v34_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_external_assertions_v35 dock_external_assertions_v35_tenant_id_asn_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_assertions_v35
    ADD CONSTRAINT dock_external_assertions_v35_tenant_id_asn_draft_id_fkey FOREIGN KEY (tenant_id, asn_draft_id) REFERENCES public.dock_asn_drafts_v34(tenant_id, id);


--
-- Name: dock_external_assertions_v35 dock_external_assertions_v35_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_assertions_v35
    ADD CONSTRAINT dock_external_assertions_v35_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_external_assertions_v35 dock_external_assertions_v35_tenant_id_plant_id_signer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_assertions_v35
    ADD CONSTRAINT dock_external_assertions_v35_tenant_id_plant_id_signer_id_fkey FOREIGN KEY (tenant_id, plant_id, signer_id) REFERENCES public.dock_external_signers_v35(tenant_id, plant_id, signer_id);


--
-- Name: dock_external_signers_v35 dock_external_signers_v35_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_external_signers_v35
    ADD CONSTRAINT dock_external_signers_v35_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_gateways_v34 dock_gateways_v34_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_gateways_v34
    ADD CONSTRAINT dock_gateways_v34_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dock_station_policies_v34 dock_station_policies_v34_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dock_station_policies_v34
    ADD CONSTRAINT dock_station_policies_v34_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: document_assertions_v65 document_assertions_v65_tenant_id_document_id_source_ordin_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_assertions_v65
    ADD CONSTRAINT document_assertions_v65_tenant_id_document_id_source_ordin_fkey FOREIGN KEY (tenant_id, document_id, source_ordinal) REFERENCES public.document_segments_v65(tenant_id, document_id, ordinal) ON DELETE RESTRICT;


--
-- Name: document_jobs_v65 document_jobs_v65_tenant_id_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_jobs_v65
    ADD CONSTRAINT document_jobs_v65_tenant_id_document_id_fkey FOREIGN KEY (tenant_id, document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: document_segments_v65 document_segments_v65_tenant_id_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_segments_v65
    ADD CONSTRAINT document_segments_v65_tenant_id_document_id_fkey FOREIGN KEY (tenant_id, document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: documents documents_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: dynamic_work_orders dynamic_work_orders_style_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dynamic_work_orders
    ADD CONSTRAINT dynamic_work_orders_style_id_fkey FOREIGN KEY (style_id) REFERENCES public.styles_master(id) ON DELETE RESTRICT;


--
-- Name: dynamic_work_orders dynamic_work_orders_supervisor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dynamic_work_orders
    ADD CONSTRAINT dynamic_work_orders_supervisor_id_fkey FOREIGN KEY (supervisor_id) REFERENCES public.workers(id) ON DELETE RESTRICT;


--
-- Name: dynamic_work_orders dynamic_work_orders_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dynamic_work_orders
    ADD CONSTRAINT dynamic_work_orders_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: fabric_calibration_links_v32 fabric_calibration_links_v32_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_calibration_links_v32
    ADD CONSTRAINT fabric_calibration_links_v32_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: fabric_calibration_links_v32 fabric_calibration_links_v32_tenant_id_observation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_calibration_links_v32
    ADD CONSTRAINT fabric_calibration_links_v32_tenant_id_observation_id_fkey FOREIGN KEY (tenant_id, observation_id) REFERENCES public.fabric_shrinkage_observations_v30(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: fabric_calibration_links_v32 fabric_calibration_links_v32_tenant_id_plant_id_instrument_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_calibration_links_v32
    ADD CONSTRAINT fabric_calibration_links_v32_tenant_id_plant_id_instrument_fkey FOREIGN KEY (tenant_id, plant_id, instrument_id, calibration_revision) REFERENCES public.factory_instrument_calibrations_v32(tenant_id, plant_id, instrument_id, revision) ON DELETE RESTRICT;


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_observations_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_observations_v30
    ADD CONSTRAINT fabric_shrinkage_observations_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: fabric_shrinkage_observations_v30 fabric_shrinkage_observations_v30_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_observations_v30
    ADD CONSTRAINT fabric_shrinkage_observations_v30_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: fabric_shrinkage_specs_v31 fabric_shrinkage_specs_v31_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_specs_v31
    ADD CONSTRAINT fabric_shrinkage_specs_v31_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: fabric_shrinkage_specs_v31 fabric_shrinkage_specs_v31_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fabric_shrinkage_specs_v31
    ADD CONSTRAINT fabric_shrinkage_specs_v31_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.documents(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: factory_approved_reference_versions_v65b5 factory_approved_reference_ve_tenant_id_candidate_id_revie_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_approved_reference_versions_v65b5
    ADD CONSTRAINT factory_approved_reference_ve_tenant_id_candidate_id_revie_fkey FOREIGN KEY (tenant_id, candidate_id, review_revision) REFERENCES public.factory_candidate_review_versions_v65b5(tenant_id, candidate_id, revision) ON DELETE RESTRICT;


--
-- Name: factory_approved_reference_versions_v65b5 factory_approved_reference_versions__tenant_id_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_approved_reference_versions_v65b5
    ADD CONSTRAINT factory_approved_reference_versions__tenant_id_document_id_fkey FOREIGN KEY (tenant_id, document_id) REFERENCES public.factory_document_submissions_v65b5(tenant_id, document_id) ON DELETE RESTRICT;


--
-- Name: factory_candidate_review_versions_v65b5 factory_candidate_review_versions_v_tenant_id_candidate_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_candidate_review_versions_v65b5
    ADD CONSTRAINT factory_candidate_review_versions_v_tenant_id_candidate_id_fkey FOREIGN KEY (tenant_id, candidate_id) REFERENCES public.factory_document_candidates_v65b2(tenant_id, candidate_id) ON DELETE RESTRICT;


--
-- Name: factory_candidate_reviews_v65b2 factory_candidate_reviews_v65b2_tenant_id_candidate_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_candidate_reviews_v65b2
    ADD CONSTRAINT factory_candidate_reviews_v65b2_tenant_id_candidate_id_fkey FOREIGN KEY (tenant_id, candidate_id) REFERENCES public.factory_document_candidates_v65b2(tenant_id, candidate_id) ON DELETE RESTRICT;


--
-- Name: factory_document_candidates_v65b2 factory_document_candidates_v_tenant_id_document_id_source_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_document_candidates_v65b2
    ADD CONSTRAINT factory_document_candidates_v_tenant_id_document_id_source_fkey FOREIGN KEY (tenant_id, document_id, source_ordinal) REFERENCES public.document_segments_v65(tenant_id, document_id, ordinal) ON DELETE RESTRICT;


--
-- Name: factory_document_submissions_v65b5 factory_document_submissions_v65b5_tenant_id_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_document_submissions_v65b5
    ADD CONSTRAINT factory_document_submissions_v65b5_tenant_id_document_id_fkey FOREIGN KEY (tenant_id, document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: factory_draft_decisions_v65b3 factory_draft_decisions_v65b3_tenant_id_candidate_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_draft_decisions_v65b3
    ADD CONSTRAINT factory_draft_decisions_v65b3_tenant_id_candidate_id_fkey FOREIGN KEY (tenant_id, candidate_id) REFERENCES public.factory_proposed_register_v65b2(tenant_id, candidate_id) ON DELETE RESTRICT;


--
-- Name: factory_fact_events_v65b factory_fact_events_v65b_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_fact_events_v65b
    ADD CONSTRAINT factory_fact_events_v65b_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: factory_instrument_calibrations_v32 factory_instrument_calibratio_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_instrument_calibrations_v32
    ADD CONSTRAINT factory_instrument_calibratio_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: factory_instrument_calibrations_v32 factory_instrument_calibrations_v32_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_instrument_calibrations_v32
    ADD CONSTRAINT factory_instrument_calibrations_v32_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: factory_proposed_register_v65b2 factory_proposed_register_v65b2_tenant_id_candidate_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_proposed_register_v65b2
    ADD CONSTRAINT factory_proposed_register_v65b2_tenant_id_candidate_id_fkey FOREIGN KEY (tenant_id, candidate_id) REFERENCES public.factory_document_candidates_v65b2(tenant_id, candidate_id) ON DELETE RESTRICT;


--
-- Name: factory_reference_versions_v65b4 factory_reference_versions_v65b4_tenant_id_candidate_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_reference_versions_v65b4
    ADD CONSTRAINT factory_reference_versions_v65b4_tenant_id_candidate_id_fkey FOREIGN KEY (tenant_id, candidate_id) REFERENCES public.factory_proposed_register_v65b2(tenant_id, candidate_id) ON DELETE RESTRICT;


--
-- Name: factory_reference_versions_v65b4 factory_reference_versions_v6_tenant_id_entity_type_entity_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.factory_reference_versions_v65b4
    ADD CONSTRAINT factory_reference_versions_v6_tenant_id_entity_type_entity_fkey FOREIGN KEY (tenant_id, entity_type, entity_key, draft_revision) REFERENCES public.factory_draft_decisions_v65b3(tenant_id, entity_type, entity_key, revision) ON DELETE RESTRICT;


--
-- Name: floor_issues floor_issues_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.floor_issues
    ADD CONSTRAINT floor_issues_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: gen_bundles_v18 gen_bundles_v18_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_bundles_v18
    ADD CONSTRAINT gen_bundles_v18_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: gen_bundles_v18 gen_bundles_v18_tenant_id_lay_draft_id_label_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_bundles_v18
    ADD CONSTRAINT gen_bundles_v18_tenant_id_lay_draft_id_label_code_fkey FOREIGN KEY (tenant_id, lay_draft_id, label_code) REFERENCES public.cut_bundle_drafts_v8(tenant_id, lay_draft_id, label_code) ON DELETE RESTRICT;


--
-- Name: gen_bundles_v18 gen_bundles_v18_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_bundles_v18
    ADD CONSTRAINT gen_bundles_v18_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: gen_carrier_active_v18 gen_carrier_active_v18_tenant_id_bound_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_active_v18
    ADD CONSTRAINT gen_carrier_active_v18_tenant_id_bound_event_id_fkey FOREIGN KEY (tenant_id, bound_event_id) REFERENCES public.gen_carrier_events_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: gen_carrier_active_v18 gen_carrier_active_v18_tenant_id_carrier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_active_v18
    ADD CONSTRAINT gen_carrier_active_v18_tenant_id_carrier_id_fkey FOREIGN KEY (tenant_id, carrier_id) REFERENCES public.gen_carriers_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: gen_carrier_events_v18 gen_carrier_events_v18_tenant_id_carrier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_events_v18
    ADD CONSTRAINT gen_carrier_events_v18_tenant_id_carrier_id_fkey FOREIGN KEY (tenant_id, carrier_id) REFERENCES public.gen_carriers_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: gen_carrier_events_v18 gen_carrier_events_v18_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carrier_events_v18
    ADD CONSTRAINT gen_carrier_events_v18_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: gen_carriers_v18 gen_carriers_v18_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_carriers_v18
    ADD CONSTRAINT gen_carriers_v18_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: gen_cut_parts_v18 gen_cut_parts_v18_tenant_id_bundle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_cut_parts_v18
    ADD CONSTRAINT gen_cut_parts_v18_tenant_id_bundle_id_fkey FOREIGN KEY (tenant_id, bundle_id) REFERENCES public.gen_bundles_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: gen_cut_parts_v18 gen_cut_parts_v18_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_cut_parts_v18
    ADD CONSTRAINT gen_cut_parts_v18_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: gen_cut_parts_v18 gen_cut_parts_v18_tenant_id_lay_draft_id_label_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_cut_parts_v18
    ADD CONSTRAINT gen_cut_parts_v18_tenant_id_lay_draft_id_label_code_fkey FOREIGN KEY (tenant_id, lay_draft_id, label_code) REFERENCES public.cut_bundle_drafts_v8(tenant_id, lay_draft_id, label_code) ON DELETE RESTRICT;


--
-- Name: gen_garments_v18 gen_garments_v18_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_garments_v18
    ADD CONSTRAINT gen_garments_v18_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: gen_garments_v18 gen_garments_v18_tenant_id_lay_draft_id_marker_position_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_garments_v18
    ADD CONSTRAINT gen_garments_v18_tenant_id_lay_draft_id_marker_position_fkey FOREIGN KEY (tenant_id, lay_draft_id, marker_position) REFERENCES public.cut_lay_positions_v8(tenant_id, lay_draft_id, marker_position) ON DELETE RESTRICT;


--
-- Name: gen_garments_v18 gen_garments_v18_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_garments_v18
    ADD CONSTRAINT gen_garments_v18_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: hangtag_binds hangtag_binds_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hangtag_binds
    ADD CONSTRAINT hangtag_binds_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: hourly_line_metrics hourly_line_metrics_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hourly_line_metrics
    ADD CONSTRAINT hourly_line_metrics_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: intake_proposed_fact intake_proposed_fact_capture_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intake_proposed_fact
    ADD CONSTRAINT intake_proposed_fact_capture_id_fkey FOREIGN KEY (capture_id) REFERENCES public.intake_document_capture(id) ON DELETE RESTRICT;


--
-- Name: intel_sim_snapshots_v12 intel_sim_snapshots_v12_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intel_sim_snapshots_v12
    ADD CONSTRAINT intel_sim_snapshots_v12_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: intel_sim_snapshots_v12 intel_sim_snapshots_v12_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intel_sim_snapshots_v12
    ADD CONSTRAINT intel_sim_snapshots_v12_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: intelligence_prod_readers_v13 intelligence_prod_readers_v13_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.intelligence_prod_readers_v13
    ADD CONSTRAINT intelligence_prod_readers_v13_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancies_v29_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancies_v29
    ADD CONSTRAINT jobwork_discrepancies_v29_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancies_v29_tenant_id_outward_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancies_v29
    ADD CONSTRAINT jobwork_discrepancies_v29_tenant_id_outward_id_fkey FOREIGN KEY (tenant_id, outward_id) REFERENCES public.jobwork_outwards_v28(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancies_v29_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancies_v29
    ADD CONSTRAINT jobwork_discrepancies_v29_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutions_v31_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutions_v31_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutions_v31_tenant_id_outward_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutions_v31_tenant_id_outward_id_fkey FOREIGN KEY (tenant_id, outward_id) REFERENCES public.jobwork_outwards_v28(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutions_v31_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutions_v31_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_discrepancy_resolutions_v31 jobwork_discrepancy_resolutions_v_tenant_id_discrepancy_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_discrepancy_resolutions_v31
    ADD CONSTRAINT jobwork_discrepancy_resolutions_v_tenant_id_discrepancy_id_fkey FOREIGN KEY (tenant_id, discrepancy_id) REFERENCES public.jobwork_discrepancies_v29(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_outward_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_outward_id_fkey FOREIGN KEY (tenant_id, outward_id) REFERENCES public.jobwork_outwards_v28(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_inwards_v28 jobwork_inwards_v28_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_inwards_v28
    ADD CONSTRAINT jobwork_inwards_v28_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.wip_gateway_scans_v20(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_tenant_id_plant_id_vendor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_tenant_id_plant_id_vendor_id_fkey FOREIGN KEY (tenant_id, plant_id, vendor_id) REFERENCES public.jobwork_vendors_v28(tenant_id, plant_id, vendor_id) ON DELETE RESTRICT;


--
-- Name: jobwork_outwards_v28 jobwork_outwards_v28_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_outwards_v28
    ADD CONSTRAINT jobwork_outwards_v28_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.wip_gateway_scans_v20(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_used_proofs_v28 jobwork_used_proofs_v28_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_used_proofs_v28
    ADD CONSTRAINT jobwork_used_proofs_v28_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: jobwork_used_scans_v28 jobwork_used_scans_v28_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_used_scans_v28
    ADD CONSTRAINT jobwork_used_scans_v28_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: jobwork_used_scans_v28 jobwork_used_scans_v28_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_used_scans_v28
    ADD CONSTRAINT jobwork_used_scans_v28_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.wip_gateway_scans_v20(tenant_id, id);


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_outward_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_outward_id_fkey FOREIGN KEY (tenant_id, outward_id) REFERENCES public.jobwork_outwards_v28(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: jobwork_vendor_acks_v30 jobwork_vendor_acks_v30_tenant_id_plant_id_vendor_id_key_i_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_acks_v30
    ADD CONSTRAINT jobwork_vendor_acks_v30_tenant_id_plant_id_vendor_id_key_i_fkey FOREIGN KEY (tenant_id, plant_id, vendor_id, key_id) REFERENCES public.jobwork_vendor_keys_v30(tenant_id, plant_id, vendor_id, key_id) ON DELETE RESTRICT;


--
-- Name: jobwork_vendor_keys_v30 jobwork_vendor_keys_v30_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_keys_v30
    ADD CONSTRAINT jobwork_vendor_keys_v30_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: jobwork_vendor_keys_v30 jobwork_vendor_keys_v30_tenant_id_plant_id_vendor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendor_keys_v30
    ADD CONSTRAINT jobwork_vendor_keys_v30_tenant_id_plant_id_vendor_id_fkey FOREIGN KEY (tenant_id, plant_id, vendor_id) REFERENCES public.jobwork_vendors_v28(tenant_id, plant_id, vendor_id) ON DELETE RESTRICT;


--
-- Name: jobwork_vendors_v28 jobwork_vendors_v28_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobwork_vendors_v28
    ADD CONSTRAINT jobwork_vendors_v28_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: lines lines_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.lines
    ADD CONSTRAINT lines_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: machine_roster machine_roster_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.machine_roster
    ADD CONSTRAINT machine_roster_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: material_ceo_principals_v5 material_ceo_principals_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_ceo_principals_v5
    ADD CONSTRAINT material_ceo_principals_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: material_issues_v5 material_issues_v5_reservation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_reservation_id_fkey FOREIGN KEY (reservation_id) REFERENCES public.material_reservations_v5(id);


--
-- Name: material_issues_v5 material_issues_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: material_issues_v5 material_issues_v5_tenant_id_reservation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_tenant_id_reservation_id_fkey FOREIGN KEY (tenant_id, reservation_id) REFERENCES public.material_reservations_v5(tenant_id, id);


--
-- Name: material_issues_v5 material_issues_v5_tenant_id_roll_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_issues_v5
    ADD CONSTRAINT material_issues_v5_tenant_id_roll_id_fkey FOREIGN KEY (tenant_id, roll_id) REFERENCES public.store_rolls(tenant_id, id);


--
-- Name: material_outbox_v5 material_outbox_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_outbox_v5
    ADD CONSTRAINT material_outbox_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: material_reservations_v5 material_reservations_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_reservations_v5
    ADD CONSTRAINT material_reservations_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: material_reservations_v5 material_reservations_v5_tenant_id_roll_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_reservations_v5
    ADD CONSTRAINT material_reservations_v5_tenant_id_roll_id_fkey FOREIGN KEY (tenant_id, roll_id) REFERENCES public.store_rolls(tenant_id, id);


--
-- Name: material_scenarios_v6 material_scenarios_v6_tenant_id_intake_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_scenarios_v6
    ADD CONSTRAINT material_scenarios_v6_tenant_id_intake_id_fkey FOREIGN KEY (tenant_id, intake_id) REFERENCES public.order_intakes_v5(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: material_used_proofs_v5 material_used_proofs_v5_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_used_proofs_v5
    ADD CONSTRAINT material_used_proofs_v5_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.material_issues_v5(id);


--
-- Name: material_used_proofs_v5 material_used_proofs_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_used_proofs_v5
    ADD CONSTRAINT material_used_proofs_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: material_variances_v5 material_variances_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_variances_v5
    ADD CONSTRAINT material_variances_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: material_variances_v5 material_variances_v5_tenant_id_reservation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_variances_v5
    ADD CONSTRAINT material_variances_v5_tenant_id_reservation_id_fkey FOREIGN KEY (tenant_id, reservation_id) REFERENCES public.material_reservations_v5(tenant_id, id);


--
-- Name: material_variances_v5 material_variances_v5_tenant_id_roll_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_variances_v5
    ADD CONSTRAINT material_variances_v5_tenant_id_roll_id_fkey FOREIGN KEY (tenant_id, roll_id) REFERENCES public.store_rolls(tenant_id, id);


--
-- Name: material_write_scopes_v5 material_write_scopes_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_write_scopes_v5
    ADD CONSTRAINT material_write_scopes_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_evidence_document_i_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_evidence_document_i_fkey FOREIGN KEY (tenant_id, evidence_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_followup_pass_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_followup_pass_id_fkey FOREIGN KEY (tenant_id, followup_pass_id) REFERENCES public.needle_safety_events_v32(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: needle_safety_clearances_v33 needle_safety_clearances_v33_tenant_id_incident_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_clearances_v33
    ADD CONSTRAINT needle_safety_clearances_v33_tenant_id_incident_id_fkey FOREIGN KEY (tenant_id, incident_id) REFERENCES public.needle_safety_events_v32(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: needle_safety_events_v32 needle_safety_events_v32_tenant_id_evidence_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_events_v32
    ADD CONSTRAINT needle_safety_events_v32_tenant_id_evidence_document_id_fkey FOREIGN KEY (tenant_id, evidence_document_id) REFERENCES public.document_sources_v65(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: needle_safety_events_v32 needle_safety_events_v32_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_events_v32
    ADD CONSTRAINT needle_safety_events_v32_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: needle_safety_events_v32 needle_safety_events_v32_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.needle_safety_events_v32
    ADD CONSTRAINT needle_safety_events_v32_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: ocr_records ocr_records_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ocr_records
    ADD CONSTRAINT ocr_records_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: order_intake_bom_v5 order_intake_bom_v5_tenant_id_intake_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intake_bom_v5
    ADD CONSTRAINT order_intake_bom_v5_tenant_id_intake_id_fkey FOREIGN KEY (tenant_id, intake_id) REFERENCES public.order_intakes_v5(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: order_intake_skus_v5 order_intake_skus_v5_tenant_id_intake_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intake_skus_v5
    ADD CONSTRAINT order_intake_skus_v5_tenant_id_intake_id_fkey FOREIGN KEY (tenant_id, intake_id) REFERENCES public.order_intakes_v5(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: order_intakes_v5 order_intakes_v5_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_intakes_v5
    ADD CONSTRAINT order_intakes_v5_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: order_sku_matrix order_sku_matrix_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_sku_matrix
    ADD CONSTRAINT order_sku_matrix_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.dynamic_work_orders(id) ON DELETE CASCADE;


--
-- Name: order_sku_matrix order_sku_matrix_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_sku_matrix
    ADD CONSTRAINT order_sku_matrix_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: pack_carton_events_v22 pack_carton_events_v22_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_events_v22
    ADD CONSTRAINT pack_carton_events_v22_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: pack_carton_events_v22 pack_carton_events_v22_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_events_v22
    ADD CONSTRAINT pack_carton_events_v22_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id);


--
-- Name: pack_carton_events_v22 pack_carton_events_v22_tenant_id_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_events_v22
    ADD CONSTRAINT pack_carton_events_v22_tenant_id_spec_id_fkey FOREIGN KEY (tenant_id, spec_id) REFERENCES public.pack_carton_specs_v22(tenant_id, id);


--
-- Name: pack_carton_specs_v22 pack_carton_specs_v22_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_specs_v22
    ADD CONSTRAINT pack_carton_specs_v22_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: pack_carton_specs_v22 pack_carton_specs_v22_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_specs_v22
    ADD CONSTRAINT pack_carton_specs_v22_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id);


--
-- Name: pack_carton_specs_v22 pack_carton_specs_v22_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_carton_specs_v22
    ADD CONSTRAINT pack_carton_specs_v22_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.document_sources_v65(tenant_id, id);


--
-- Name: pack_manifest_events_v22 pack_manifest_events_v22_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_manifest_events_v22
    ADD CONSTRAINT pack_manifest_events_v22_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: pack_manifest_events_v22 pack_manifest_events_v22_tenant_id_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_manifest_events_v22
    ADD CONSTRAINT pack_manifest_events_v22_tenant_id_spec_id_fkey FOREIGN KEY (tenant_id, spec_id) REFERENCES public.pack_carton_specs_v22(tenant_id, id);


--
-- Name: pack_prod_assignments_v12 pack_prod_assignments_v12_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_assignments_v12
    ADD CONSTRAINT pack_prod_assignments_v12_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: pack_prod_holds_v12 pack_prod_holds_v12_tenant_id_completion_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_holds_v12
    ADD CONSTRAINT pack_prod_holds_v12_tenant_id_completion_id_fkey FOREIGN KEY (tenant_id, completion_id) REFERENCES public.sew_prod_completions_v10(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: pack_prod_holds_v12 pack_prod_holds_v12_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_holds_v12
    ADD CONSTRAINT pack_prod_holds_v12_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: pack_prod_holds_v12 pack_prod_holds_v12_tenant_id_qc_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_prod_holds_v12
    ADD CONSTRAINT pack_prod_holds_v12_tenant_id_qc_event_id_fkey FOREIGN KEY (tenant_id, qc_event_id) REFERENCES public.qc_prod_group_events_v11(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: pack_sim_cartons_v11 pack_sim_cartons_v11_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_cartons_v11
    ADD CONSTRAINT pack_sim_cartons_v11_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: pack_sim_cartons_v11 pack_sim_cartons_v11_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_cartons_v11
    ADD CONSTRAINT pack_sim_cartons_v11_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id);


--
-- Name: pack_sim_events_v11 pack_sim_events_v11_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_events_v11
    ADD CONSTRAINT pack_sim_events_v11_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: pack_sim_items_v11 pack_sim_items_v11_tenant_id_carton_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_items_v11
    ADD CONSTRAINT pack_sim_items_v11_tenant_id_carton_code_fkey FOREIGN KEY (tenant_id, carton_code) REFERENCES public.pack_sim_cartons_v11(tenant_id, carton_code);


--
-- Name: pack_sim_items_v11 pack_sim_items_v11_tenant_id_serial_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_items_v11
    ADD CONSTRAINT pack_sim_items_v11_tenant_id_serial_fkey FOREIGN KEY (tenant_id, serial) REFERENCES public.qc_sim_buyer_bindings_v10(tenant_id, serial);


--
-- Name: pack_sim_manifest_cartons_v11 pack_sim_manifest_cartons_v11_tenant_id_carton_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_manifest_cartons_v11
    ADD CONSTRAINT pack_sim_manifest_cartons_v11_tenant_id_carton_code_fkey FOREIGN KEY (tenant_id, carton_code) REFERENCES public.pack_sim_cartons_v11(tenant_id, carton_code);


--
-- Name: pack_sim_manifest_cartons_v11 pack_sim_manifest_cartons_v11_tenant_id_manifest_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_manifest_cartons_v11
    ADD CONSTRAINT pack_sim_manifest_cartons_v11_tenant_id_manifest_code_fkey FOREIGN KEY (tenant_id, manifest_code) REFERENCES public.pack_sim_manifests_v11(tenant_id, manifest_code);


--
-- Name: pack_sim_manifests_v11 pack_sim_manifests_v11_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_sim_manifests_v11
    ADD CONSTRAINT pack_sim_manifests_v11_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: pack_size_weights_v22 pack_size_weights_v22_tenant_id_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pack_size_weights_v22
    ADD CONSTRAINT pack_size_weights_v22_tenant_id_spec_id_fkey FOREIGN KEY (tenant_id, spec_id) REFERENCES public.pack_carton_specs_v22(tenant_id, id);


--
-- Name: plant_hardware_assets plant_hardware_assets_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plant_hardware_assets
    ADD CONSTRAINT plant_hardware_assets_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: post_sewing_processes post_sewing_processes_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_sewing_processes
    ADD CONSTRAINT post_sewing_processes_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.dynamic_work_orders(id) ON DELETE CASCADE;


--
-- Name: post_sewing_processes post_sewing_processes_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_sewing_processes
    ADD CONSTRAINT post_sewing_processes_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_garment_defects_v21 qc_garment_defects_v21_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_defects_v21
    ADD CONSTRAINT qc_garment_defects_v21_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: qc_garment_events_v21 qc_garment_events_v21_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_events_v21
    ADD CONSTRAINT qc_garment_events_v21_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: qc_garment_events_v21 qc_garment_events_v21_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_events_v21
    ADD CONSTRAINT qc_garment_events_v21_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id);


--
-- Name: qc_garment_events_v21 qc_garment_events_v21_tenant_id_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_events_v21
    ADD CONSTRAINT qc_garment_events_v21_tenant_id_spec_id_fkey FOREIGN KEY (tenant_id, spec_id) REFERENCES public.qc_garment_specs_v21(tenant_id, id);


--
-- Name: qc_garment_measure_rules_v21 qc_garment_measure_rules_v21_tenant_id_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_measure_rules_v21
    ADD CONSTRAINT qc_garment_measure_rules_v21_tenant_id_spec_id_fkey FOREIGN KEY (tenant_id, spec_id) REFERENCES public.qc_garment_specs_v21(tenant_id, id);


--
-- Name: qc_garment_specs_v21 qc_garment_specs_v21_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_specs_v21
    ADD CONSTRAINT qc_garment_specs_v21_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: qc_garment_specs_v21 qc_garment_specs_v21_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_specs_v21
    ADD CONSTRAINT qc_garment_specs_v21_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id);


--
-- Name: qc_garment_specs_v21 qc_garment_specs_v21_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_garment_specs_v21
    ADD CONSTRAINT qc_garment_specs_v21_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.document_sources_v65(tenant_id, id);


--
-- Name: qc_prod_assignments_v11 qc_prod_assignments_v11_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_assignments_v11
    ADD CONSTRAINT qc_prod_assignments_v11_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_prod_defect_catalog_v11 qc_prod_defect_catalog_v11_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_defect_catalog_v11
    ADD CONSTRAINT qc_prod_defect_catalog_v11_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_prod_group_events_v11 qc_prod_group_events_v11_tenant_id_completion_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_group_events_v11
    ADD CONSTRAINT qc_prod_group_events_v11_tenant_id_completion_id_fkey FOREIGN KEY (tenant_id, completion_id) REFERENCES public.sew_prod_completions_v10(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: qc_prod_group_events_v11 qc_prod_group_events_v11_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_prod_group_events_v11
    ADD CONSTRAINT qc_prod_group_events_v11_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_buyer_bindings_v10_tenant_id_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_buyer_bindings_v10
    ADD CONSTRAINT qc_sim_buyer_bindings_v10_tenant_id_event_id_fkey FOREIGN KEY (tenant_id, event_id) REFERENCES public.qc_sim_events_v10(tenant_id, event_id) ON DELETE RESTRICT;


--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_buyer_bindings_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_buyer_bindings_v10
    ADD CONSTRAINT qc_sim_buyer_bindings_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_buyer_bindings_v10_tenant_id_serial_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_buyer_bindings_v10
    ADD CONSTRAINT qc_sim_buyer_bindings_v10_tenant_id_serial_fkey FOREIGN KEY (tenant_id, serial) REFERENCES public.qc_sim_units_v10(tenant_id, serial) ON DELETE RESTRICT;


--
-- Name: qc_sim_events_v10 qc_sim_events_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_events_v10
    ADD CONSTRAINT qc_sim_events_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_sim_events_v10 qc_sim_events_v10_tenant_id_serial_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_events_v10
    ADD CONSTRAINT qc_sim_events_v10_tenant_id_serial_fkey FOREIGN KEY (tenant_id, serial) REFERENCES public.qc_sim_units_v10(tenant_id, serial) ON DELETE RESTRICT;


--
-- Name: qc_sim_routes_v10 qc_sim_routes_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_routes_v10
    ADD CONSTRAINT qc_sim_routes_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_sim_routes_v10 qc_sim_routes_v10_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_routes_v10
    ADD CONSTRAINT qc_sim_routes_v10_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: qc_sim_units_v10 qc_sim_units_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_units_v10
    ADD CONSTRAINT qc_sim_units_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: qc_sim_units_v10 qc_sim_units_v10_tenant_id_lay_draft_id_marker_position_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.qc_sim_units_v10
    ADD CONSTRAINT qc_sim_units_v10_tenant_id_lay_draft_id_marker_position_fkey FOREIGN KEY (tenant_id, lay_draft_id, marker_position) REFERENCES public.cut_lay_positions_v8(tenant_id, lay_draft_id, marker_position) ON DELETE RESTRICT;


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_piece_completions_v27 sew_piece_completions_v27_tenant_id_route_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_completions_v27
    ADD CONSTRAINT sew_piece_completions_v27_tenant_id_route_id_fkey FOREIGN KEY (tenant_id, route_id) REFERENCES public.sew_prod_routes_v10(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_piece_used_scans_v27 sew_piece_used_scans_v27_tenant_id_completion_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_used_scans_v27
    ADD CONSTRAINT sew_piece_used_scans_v27_tenant_id_completion_id_fkey FOREIGN KEY (tenant_id, completion_id) REFERENCES public.sew_piece_completions_v27(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_piece_used_scans_v27 sew_piece_used_scans_v27_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_used_scans_v27
    ADD CONSTRAINT sew_piece_used_scans_v27_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_piece_used_scans_v27 sew_piece_used_scans_v27_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_piece_used_scans_v27
    ADD CONSTRAINT sew_piece_used_scans_v27_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.wip_gateway_scans_v20(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_tenant_id_lay_draft_id_marker_pos_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_tenant_id_lay_draft_id_marker_pos_fkey FOREIGN KEY (tenant_id, lay_draft_id, marker_position) REFERENCES public.cut_lay_positions_v8(tenant_id, lay_draft_id, marker_position) ON DELETE RESTRICT;


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_prod_completions_v10 sew_prod_completions_v10_tenant_id_route_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_completions_v10
    ADD CONSTRAINT sew_prod_completions_v10_tenant_id_route_id_fkey FOREIGN KEY (tenant_id, route_id) REFERENCES public.sew_prod_routes_v10(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_prod_gateways_v10 sew_prod_gateways_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_gateways_v10
    ADD CONSTRAINT sew_prod_gateways_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_prod_observations_v10 sew_prod_observations_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_observations_v10
    ADD CONSTRAINT sew_prod_observations_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_prod_observations_v10 sew_prod_observations_v10_tenant_id_gateway_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_observations_v10
    ADD CONSTRAINT sew_prod_observations_v10_tenant_id_gateway_id_fkey FOREIGN KEY (tenant_id, gateway_id) REFERENCES public.sew_prod_gateways_v10(tenant_id, gateway_id) ON DELETE RESTRICT;


--
-- Name: sew_prod_observations_v10 sew_prod_observations_v10_tenant_id_lay_draft_id_label_cod_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_observations_v10
    ADD CONSTRAINT sew_prod_observations_v10_tenant_id_lay_draft_id_label_cod_fkey FOREIGN KEY (tenant_id, lay_draft_id, label_code) REFERENCES public.cut_bundle_drafts_v8(tenant_id, lay_draft_id, label_code) ON DELETE RESTRICT;


--
-- Name: sew_prod_observations_v10 sew_prod_observations_v10_tenant_id_release_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_observations_v10
    ADD CONSTRAINT sew_prod_observations_v10_tenant_id_release_id_fkey FOREIGN KEY (tenant_id, release_id) REFERENCES public.cut_sewing_releases_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_prod_routes_v10 sew_prod_routes_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_routes_v10
    ADD CONSTRAINT sew_prod_routes_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_prod_routes_v10 sew_prod_routes_v10_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_routes_v10
    ADD CONSTRAINT sew_prod_routes_v10_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_prod_used_observations_v10 sew_prod_used_observations_v10_tenant_id_completion_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_used_observations_v10
    ADD CONSTRAINT sew_prod_used_observations_v10_tenant_id_completion_id_fkey FOREIGN KEY (tenant_id, completion_id) REFERENCES public.sew_prod_completions_v10(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_prod_used_observations_v10 sew_prod_used_observations_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_used_observations_v10
    ADD CONSTRAINT sew_prod_used_observations_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_prod_used_observations_v10 sew_prod_used_observations_v10_tenant_id_observation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_used_observations_v10
    ADD CONSTRAINT sew_prod_used_observations_v10_tenant_id_observation_id_fkey FOREIGN KEY (tenant_id, observation_id) REFERENCES public.sew_prod_observations_v10(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_prod_worker_stations_v10 sew_prod_worker_stations_v10_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_prod_worker_stations_v10
    ADD CONSTRAINT sew_prod_worker_stations_v10_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_sim_credits_v9 sew_sim_credits_v9_tenant_id_cycle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_credits_v9
    ADD CONSTRAINT sew_sim_credits_v9_tenant_id_cycle_id_fkey FOREIGN KEY (tenant_id, cycle_id) REFERENCES public.sew_sim_cycles_v9(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_sim_credits_v9 sew_sim_credits_v9_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_credits_v9
    ADD CONSTRAINT sew_sim_credits_v9_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_sim_credits_v9 sew_sim_credits_v9_tenant_id_operation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_credits_v9
    ADD CONSTRAINT sew_sim_credits_v9_tenant_id_operation_id_fkey FOREIGN KEY (tenant_id, operation_id) REFERENCES public.sew_sim_operations_v9(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_sim_cycle_inputs_v9 sew_sim_cycle_inputs_v9_tenant_id_cycle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycle_inputs_v9
    ADD CONSTRAINT sew_sim_cycle_inputs_v9_tenant_id_cycle_id_fkey FOREIGN KEY (tenant_id, cycle_id) REFERENCES public.sew_sim_cycles_v9(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_sim_cycle_inputs_v9 sew_sim_cycle_inputs_v9_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycle_inputs_v9
    ADD CONSTRAINT sew_sim_cycle_inputs_v9_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.sew_sim_scans_v9(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_sim_cycles_v9 sew_sim_cycles_v9_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycles_v9
    ADD CONSTRAINT sew_sim_cycles_v9_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_sim_cycles_v9 sew_sim_cycles_v9_tenant_id_lay_draft_id_marker_position_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycles_v9
    ADD CONSTRAINT sew_sim_cycles_v9_tenant_id_lay_draft_id_marker_position_fkey FOREIGN KEY (tenant_id, lay_draft_id, marker_position) REFERENCES public.cut_lay_positions_v8(tenant_id, lay_draft_id, marker_position) ON DELETE RESTRICT;


--
-- Name: sew_sim_cycles_v9 sew_sim_cycles_v9_tenant_id_operation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_cycles_v9
    ADD CONSTRAINT sew_sim_cycles_v9_tenant_id_operation_id_fkey FOREIGN KEY (tenant_id, operation_id) REFERENCES public.sew_sim_operations_v9(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_sim_events_v9 sew_sim_events_v9_tenant_id_cycle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_events_v9
    ADD CONSTRAINT sew_sim_events_v9_tenant_id_cycle_id_fkey FOREIGN KEY (tenant_id, cycle_id) REFERENCES public.sew_sim_cycles_v9(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_sim_events_v9 sew_sim_events_v9_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_events_v9
    ADD CONSTRAINT sew_sim_events_v9_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_sim_operations_v9 sew_sim_operations_v9_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_operations_v9
    ADD CONSTRAINT sew_sim_operations_v9_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_sim_operations_v9 sew_sim_operations_v9_tenant_id_lay_draft_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_operations_v9
    ADD CONSTRAINT sew_sim_operations_v9_tenant_id_lay_draft_id_fkey FOREIGN KEY (tenant_id, lay_draft_id) REFERENCES public.cut_lay_drafts_v8(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: sew_sim_scans_v9 sew_sim_scans_v9_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_scans_v9
    ADD CONSTRAINT sew_sim_scans_v9_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: sew_sim_scans_v9 sew_sim_scans_v9_tenant_id_lay_draft_id_label_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sew_sim_scans_v9
    ADD CONSTRAINT sew_sim_scans_v9_tenant_id_lay_draft_id_label_code_fkey FOREIGN KEY (tenant_id, lay_draft_id, label_code) REFERENCES public.cut_bundle_drafts_v8(tenant_id, lay_draft_id, label_code) ON DELETE RESTRICT;


--
-- Name: store_issuances store_issuances_roll_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_issuances
    ADD CONSTRAINT store_issuances_roll_id_fkey FOREIGN KEY (roll_id) REFERENCES public.store_rolls(id) ON DELETE RESTRICT;


--
-- Name: store_issuances store_issuances_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_issuances
    ADD CONSTRAINT store_issuances_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: store_pick_proposals_v7 store_pick_proposals_v7_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_pick_proposals_v7
    ADD CONSTRAINT store_pick_proposals_v7_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: store_pick_proposals_v7 store_pick_proposals_v7_tenant_id_intake_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_pick_proposals_v7
    ADD CONSTRAINT store_pick_proposals_v7_tenant_id_intake_id_fkey FOREIGN KEY (tenant_id, intake_id) REFERENCES public.order_intakes_v5(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: store_reconciliation_requests_v7 store_reconciliation_requests_tenant_id_roll_id_passport_r_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_reconciliation_requests_v7
    ADD CONSTRAINT store_reconciliation_requests_tenant_id_roll_id_passport_r_fkey FOREIGN KEY (tenant_id, roll_id, passport_revision) REFERENCES public.store_roll_passport_events_v7(tenant_id, roll_id, revision);


--
-- Name: store_reconciliation_requests_v7 store_reconciliation_requests_v7_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_reconciliation_requests_v7
    ADD CONSTRAINT store_reconciliation_requests_v7_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: store_roll_admission_proofs_v7 store_roll_admission_proofs_v7_tenant_id_admission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admission_proofs_v7
    ADD CONSTRAINT store_roll_admission_proofs_v7_tenant_id_admission_id_fkey FOREIGN KEY (tenant_id, admission_id) REFERENCES public.store_roll_admissions_v7(tenant_id, id);


--
-- Name: store_roll_admission_proofs_v7 store_roll_admission_proofs_v7_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admission_proofs_v7
    ADD CONSTRAINT store_roll_admission_proofs_v7_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: store_roll_admissions_v7 store_roll_admissions_v7_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admissions_v7
    ADD CONSTRAINT store_roll_admissions_v7_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: store_roll_admissions_v7 store_roll_admissions_v7_tenant_id_request_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admissions_v7
    ADD CONSTRAINT store_roll_admissions_v7_tenant_id_request_id_fkey FOREIGN KEY (tenant_id, request_id) REFERENCES public.store_reconciliation_requests_v7(tenant_id, id);


--
-- Name: store_roll_admissions_v7 store_roll_admissions_v7_tenant_id_roll_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admissions_v7
    ADD CONSTRAINT store_roll_admissions_v7_tenant_id_roll_id_fkey FOREIGN KEY (tenant_id, roll_id) REFERENCES public.store_rolls(tenant_id, id);


--
-- Name: store_roll_admissions_v7 store_roll_admissions_v7_tenant_id_roll_id_passport_revisi_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_admissions_v7
    ADD CONSTRAINT store_roll_admissions_v7_tenant_id_roll_id_passport_revisi_fkey FOREIGN KEY (tenant_id, roll_id, passport_revision) REFERENCES public.store_roll_passport_events_v7(tenant_id, roll_id, revision);


--
-- Name: store_roll_passport_events_v7 store_roll_passport_events_v7_tenant_id_roll_id_epc_hex_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_passport_events_v7
    ADD CONSTRAINT store_roll_passport_events_v7_tenant_id_roll_id_epc_hex_fkey FOREIGN KEY (tenant_id, roll_id, epc_hex) REFERENCES public.store_roll_rfid_bindings_v7(tenant_id, roll_id, epc_hex);


--
-- Name: store_roll_passport_events_v7 store_roll_passport_events_v7_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_roll_passport_events_v7
    ADD CONSTRAINT store_roll_passport_events_v7_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.document_sources_v65(tenant_id, id);


--
-- Name: store_rolls store_rolls_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_rolls
    ADD CONSTRAINT store_rolls_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: store_trims_v13 store_trims_v13_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_trims_v13
    ADD CONSTRAINT store_trims_v13_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: style_assembly_dag style_assembly_dag_child_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_assembly_dag
    ADD CONSTRAINT style_assembly_dag_child_part_id_fkey FOREIGN KEY (child_part_id) REFERENCES public.style_parts(id) ON DELETE CASCADE;


--
-- Name: style_assembly_dag style_assembly_dag_operation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_assembly_dag
    ADD CONSTRAINT style_assembly_dag_operation_id_fkey FOREIGN KEY (operation_id) REFERENCES public.operation_master(id) ON DELETE RESTRICT;


--
-- Name: style_assembly_dag style_assembly_dag_parent_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_assembly_dag
    ADD CONSTRAINT style_assembly_dag_parent_part_id_fkey FOREIGN KEY (parent_part_id) REFERENCES public.style_parts(id) ON DELETE CASCADE;


--
-- Name: style_assembly_dag style_assembly_dag_style_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_assembly_dag
    ADD CONSTRAINT style_assembly_dag_style_id_fkey FOREIGN KEY (style_id) REFERENCES public.styles_master(id) ON DELETE CASCADE;


--
-- Name: style_assembly_dag style_assembly_dag_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_assembly_dag
    ADD CONSTRAINT style_assembly_dag_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: style_parts style_parts_style_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_parts
    ADD CONSTRAINT style_parts_style_id_fkey FOREIGN KEY (style_id) REFERENCES public.styles_master(id) ON DELETE CASCADE;


--
-- Name: style_parts style_parts_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.style_parts
    ADD CONSTRAINT style_parts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: styles_master styles_master_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.styles_master
    ADD CONSTRAINT styles_master_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: trim_admission_proof_uses_v25 trim_admission_proof_uses_v25_admission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_admission_proof_uses_v25
    ADD CONSTRAINT trim_admission_proof_uses_v25_admission_id_fkey FOREIGN KEY (admission_id) REFERENCES public.trim_stock_admissions_v25(id) ON DELETE RESTRICT;


--
-- Name: trim_admission_proof_uses_v25 trim_admission_proof_uses_v25_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_admission_proof_uses_v25
    ADD CONSTRAINT trim_admission_proof_uses_v25_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: trim_issuance_proof_uses_v24 trim_issuance_proof_uses_v24_issuance_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_issuance_proof_uses_v24
    ADD CONSTRAINT trim_issuance_proof_uses_v24_issuance_id_fkey FOREIGN KEY (issuance_id) REFERENCES public.trim_issuances_v13(id) ON DELETE RESTRICT;


--
-- Name: trim_issuance_proof_uses_v24 trim_issuance_proof_uses_v24_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_issuance_proof_uses_v24
    ADD CONSTRAINT trim_issuance_proof_uses_v24_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: trim_issuances_v13 trim_issuances_v13_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_issuances_v13
    ADD CONSTRAINT trim_issuances_v13_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: trim_issuances_v13 trim_issuances_v13_tenant_id_trim_stock_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_issuances_v13
    ADD CONSTRAINT trim_issuances_v13_tenant_id_trim_stock_id_fkey FOREIGN KEY (tenant_id, trim_stock_id) REFERENCES public.store_trims_v13(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: trim_stock_admissions_v25 trim_stock_admissions_v25_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_stock_admissions_v25
    ADD CONSTRAINT trim_stock_admissions_v25_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: trim_stock_admissions_v25 trim_stock_admissions_v25_tenant_id_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_stock_admissions_v25
    ADD CONSTRAINT trim_stock_admissions_v25_tenant_id_source_document_id_fkey FOREIGN KEY (tenant_id, source_document_id) REFERENCES public.documents(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: trim_stock_admissions_v25 trim_stock_admissions_v25_tenant_id_stock_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trim_stock_admissions_v25
    ADD CONSTRAINT trim_stock_admissions_v25_tenant_id_stock_id_fkey FOREIGN KEY (tenant_id, stock_id) REFERENCES public.store_trims_v13(tenant_id, id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;


--
-- Name: wip_exception_cases_v23 wip_exception_cases_v23_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_cases_v23
    ADD CONSTRAINT wip_exception_cases_v23_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: wip_exception_cases_v23 wip_exception_cases_v23_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_cases_v23
    ADD CONSTRAINT wip_exception_cases_v23_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: wip_exception_closures_v23 wip_exception_closures_v23_tenant_id_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_closures_v23
    ADD CONSTRAINT wip_exception_closures_v23_tenant_id_case_id_fkey FOREIGN KEY (tenant_id, case_id) REFERENCES public.wip_exception_cases_v23(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: wip_exception_closures_v23 wip_exception_closures_v23_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_closures_v23
    ADD CONSTRAINT wip_exception_closures_v23_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: wip_exception_closures_v23 wip_exception_closures_v23_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_exception_closures_v23
    ADD CONSTRAINT wip_exception_closures_v23_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.wip_gateway_scans_v20(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: wip_gateway_scans_v20 wip_gateway_scans_v20_tenant_id_binding_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_gateway_scans_v20
    ADD CONSTRAINT wip_gateway_scans_v20_tenant_id_binding_event_id_fkey FOREIGN KEY (tenant_id, binding_event_id) REFERENCES public.gen_carrier_events_v18(tenant_id, id);


--
-- Name: wip_gateway_scans_v20 wip_gateway_scans_v20_tenant_id_carrier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_gateway_scans_v20
    ADD CONSTRAINT wip_gateway_scans_v20_tenant_id_carrier_id_fkey FOREIGN KEY (tenant_id, carrier_id) REFERENCES public.gen_carriers_v18(tenant_id, id);


--
-- Name: wip_gateway_scans_v20 wip_gateway_scans_v20_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_gateway_scans_v20
    ADD CONSTRAINT wip_gateway_scans_v20_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: wip_gateway_scans_v20 wip_gateway_scans_v20_tenant_id_gateway_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_gateway_scans_v20
    ADD CONSTRAINT wip_gateway_scans_v20_tenant_id_gateway_id_fkey FOREIGN KEY (tenant_id, gateway_id) REFERENCES public.sew_prod_gateways_v10(tenant_id, gateway_id);


--
-- Name: wip_gateway_scans_v20 wip_gateway_scans_v20_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_gateway_scans_v20
    ADD CONSTRAINT wip_gateway_scans_v20_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_part_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_part_id_fkey FOREIGN KEY (tenant_id, part_id) REFERENCES public.gen_cut_parts_v18(tenant_id, id);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_route_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_route_id_fkey FOREIGN KEY (tenant_id, route_id) REFERENCES public.wip_transfer_routes_v20(tenant_id, id);


--
-- Name: wip_movements_v20 wip_movements_v20_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_movements_v20
    ADD CONSTRAINT wip_movements_v20_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.wip_gateway_scans_v20(tenant_id, id);


--
-- Name: wip_piece_holds_v26 wip_piece_holds_v26_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_holds_v26
    ADD CONSTRAINT wip_piece_holds_v26_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: wip_piece_holds_v26 wip_piece_holds_v26_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_holds_v26
    ADD CONSTRAINT wip_piece_holds_v26_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_tenant_id_garment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_tenant_id_garment_id_fkey FOREIGN KEY (tenant_id, garment_id) REFERENCES public.gen_garments_v18(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_tenant_id_hold_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_tenant_id_hold_id_fkey FOREIGN KEY (tenant_id, hold_id) REFERENCES public.wip_piece_holds_v26(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: wip_piece_reviews_v26 wip_piece_reviews_v26_tenant_id_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_piece_reviews_v26
    ADD CONSTRAINT wip_piece_reviews_v26_tenant_id_scan_id_fkey FOREIGN KEY (tenant_id, scan_id) REFERENCES public.wip_gateway_scans_v20(tenant_id, id) ON DELETE RESTRICT;


--
-- Name: wip_transfer_routes_v20 wip_transfer_routes_v20_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wip_transfer_routes_v20
    ADD CONSTRAINT wip_transfer_routes_v20_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id);


--
-- Name: workers workers_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workers
    ADD CONSTRAINT workers_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: workforce_identity_grants_v15 workforce_identity_grants_v15_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workforce_identity_grants_v15
    ADD CONSTRAINT workforce_identity_grants_v15_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: plants plants_tenant_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.plants
    ADD CONSTRAINT plants_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES s01_identity.tenants(id) ON DELETE CASCADE;


--
-- Name: role_permissions role_permissions_role_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.role_permissions
    ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES s01_identity.roles(id) ON DELETE CASCADE;


--
-- Name: roles roles_tenant_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.roles
    ADD CONSTRAINT roles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES s01_identity.tenants(id) ON DELETE CASCADE;


--
-- Name: user_role_grants user_role_grants_plant_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.user_role_grants
    ADD CONSTRAINT user_role_grants_plant_id_fkey FOREIGN KEY (plant_id) REFERENCES s01_identity.plants(id) ON DELETE CASCADE;


--
-- Name: user_role_grants user_role_grants_role_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.user_role_grants
    ADD CONSTRAINT user_role_grants_role_id_fkey FOREIGN KEY (role_id) REFERENCES s01_identity.roles(id) ON DELETE CASCADE;


--
-- Name: user_role_grants user_role_grants_tenant_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.user_role_grants
    ADD CONSTRAINT user_role_grants_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES s01_identity.tenants(id) ON DELETE CASCADE;


--
-- Name: user_role_grants user_role_grants_user_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.user_role_grants
    ADD CONSTRAINT user_role_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES s01_identity.users(id) ON DELETE CASCADE;


--
-- Name: users users_tenant_id_fkey; Type: FK CONSTRAINT; Schema: s01_identity; Owner: -
--

ALTER TABLE ONLY s01_identity.users
    ADD CONSTRAINT users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES s01_identity.tenants(id) ON DELETE CASCADE;


--
-- Name: registered_devices registered_devices_plant_id_fkey; Type: FK CONSTRAINT; Schema: s02_device; Owner: -
--

ALTER TABLE ONLY s02_device.registered_devices
    ADD CONSTRAINT registered_devices_plant_id_fkey FOREIGN KEY (plant_id) REFERENCES s01_identity.plants(id) ON DELETE CASCADE;


--
-- Name: registered_devices registered_devices_tenant_id_fkey; Type: FK CONSTRAINT; Schema: s02_device; Owner: -
--

ALTER TABLE ONLY s02_device.registered_devices
    ADD CONSTRAINT registered_devices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES s01_identity.tenants(id) ON DELETE CASCADE;


--
-- Name: attendance_register; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.attendance_register ENABLE ROW LEVEL SECURITY;

--
-- Name: bom_trim_requirements; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.bom_trim_requirements ENABLE ROW LEVEL SECURITY;

--
-- Name: bundle_registry; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.bundle_registry ENABLE ROW LEVEL SECURITY;

--
-- Name: buyer_aql_plans_v32; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.buyer_aql_plans_v32 ENABLE ROW LEVEL SECURITY;

--
-- Name: buyer_aql_review_rules_v33; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.buyer_aql_review_rules_v33 ENABLE ROW LEVEL SECURITY;

--
-- Name: buyer_aql_reviews_v33; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.buyer_aql_reviews_v33 ENABLE ROW LEVEL SECURITY;

--
-- Name: buyer_aql_sample_events_v32; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.buyer_aql_sample_events_v32 ENABLE ROW LEVEL SECURITY;

--
-- Name: buyer_order_handoffs_v65c1 buyer_handoff_tenant_v65c1; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY buyer_handoff_tenant_v65c1 ON public.buyer_order_handoffs_v65c1 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: buyer_order_handoffs_v65c1; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.buyer_order_handoffs_v65c1 ENABLE ROW LEVEL SECURITY;

--
-- Name: carton_pack_manifests; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.carton_pack_manifests ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_bundle_drafts_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_bundle_drafts_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_bundle_scan_witnesses_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_bundle_scan_witnesses_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_consumption_reconciliations_v19; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_consumption_reconciliations_v19 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_defect_codes_v19; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_defect_codes_v19 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_lay_drafts_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_lay_drafts_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_lay_positions_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_lay_positions_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_lay_rolls; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_lay_rolls ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_lay_snapshots_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_lay_snapshots_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_part_outcomes_v19; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_part_outcomes_v19 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_proof_consumptions_v19; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_proof_consumptions_v19 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_reader_events_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_reader_events_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_reader_gateways_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_reader_gateways_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_recut_links_v19; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_recut_links_v19 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_sewing_releases_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_sewing_releases_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_spread_issue_allocations_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_spread_issue_allocations_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_spread_reports_v8; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.cut_spread_reports_v8 ENABLE ROW LEVEL SECURITY;

--
-- Name: cut_consumption_reconciliations_v19 cut_tenant_v19; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cut_tenant_v19 ON public.cut_consumption_reconciliations_v19 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_defect_codes_v19 cut_tenant_v19; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cut_tenant_v19 ON public.cut_defect_codes_v19 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_part_outcomes_v19 cut_tenant_v19; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cut_tenant_v19 ON public.cut_part_outcomes_v19 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_proof_consumptions_v19 cut_tenant_v19; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cut_tenant_v19 ON public.cut_proof_consumptions_v19 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_recut_links_v19 cut_tenant_v19; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cut_tenant_v19 ON public.cut_recut_links_v19 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_bundle_scan_witnesses_v8 cutting_evidence_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_evidence_tenant_v8 ON public.cut_bundle_scan_witnesses_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_spread_issue_allocations_v8 cutting_evidence_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_evidence_tenant_v8 ON public.cut_spread_issue_allocations_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_spread_reports_v8 cutting_evidence_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_evidence_tenant_v8 ON public.cut_spread_reports_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_reader_events_v8 cutting_reader_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_reader_tenant_v8 ON public.cut_reader_events_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_reader_gateways_v8 cutting_reader_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_reader_tenant_v8 ON public.cut_reader_gateways_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_sewing_releases_v8 cutting_reader_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_reader_tenant_v8 ON public.cut_sewing_releases_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_bundle_drafts_v8 cutting_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_tenant_v8 ON public.cut_bundle_drafts_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_lay_drafts_v8 cutting_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_tenant_v8 ON public.cut_lay_drafts_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_lay_positions_v8 cutting_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_tenant_v8 ON public.cut_lay_positions_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_lay_snapshots_v8 cutting_tenant_v8; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY cutting_tenant_v8 ON public.cut_lay_snapshots_v8 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_asn_drafts_v34; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_asn_drafts_v34 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_buyer_spec_bindings_v36; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_buyer_spec_bindings_v36 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_buyer_spec_bindings_v36 dock_buyer_tenant_v36; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_buyer_tenant_v36 ON public.dock_buyer_spec_bindings_v36 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_carrier_authorizations_v34; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_carrier_authorizations_v34 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_carrier_receipts_v34; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_carrier_receipts_v34 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_carton_scans_v34; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_carton_scans_v34 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_container_seals_v34; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_container_seals_v34 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_external_assertions_v35; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_external_assertions_v35 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_external_signers_v35; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_external_signers_v35 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_gateways_v34; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_gateways_v34 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_station_policies_v34; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dock_station_policies_v34 ENABLE ROW LEVEL SECURITY;

--
-- Name: dock_asn_drafts_v34 dock_tenant_v34; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v34 ON public.dock_asn_drafts_v34 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_carrier_authorizations_v34 dock_tenant_v34; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v34 ON public.dock_carrier_authorizations_v34 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_carrier_receipts_v34 dock_tenant_v34; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v34 ON public.dock_carrier_receipts_v34 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_carton_scans_v34 dock_tenant_v34; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v34 ON public.dock_carton_scans_v34 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_container_seals_v34 dock_tenant_v34; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v34 ON public.dock_container_seals_v34 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_gateways_v34 dock_tenant_v34; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v34 ON public.dock_gateways_v34 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_station_policies_v34 dock_tenant_v34; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v34 ON public.dock_station_policies_v34 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_external_assertions_v35 dock_tenant_v35; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v35 ON public.dock_external_assertions_v35 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dock_external_signers_v35 dock_tenant_v35; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY dock_tenant_v35 ON public.dock_external_signers_v35 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: document_assertions_v65; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.document_assertions_v65 ENABLE ROW LEVEL SECURITY;

--
-- Name: document_jobs_v65; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.document_jobs_v65 ENABLE ROW LEVEL SECURITY;

--
-- Name: document_segments_v65; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.document_segments_v65 ENABLE ROW LEVEL SECURITY;

--
-- Name: document_sources_v65; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.document_sources_v65 ENABLE ROW LEVEL SECURITY;

--
-- Name: document_assertions_v65 document_tenant_v65; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY document_tenant_v65 ON public.document_assertions_v65 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: document_jobs_v65 document_tenant_v65; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY document_tenant_v65 ON public.document_jobs_v65 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: document_segments_v65 document_tenant_v65; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY document_tenant_v65 ON public.document_segments_v65 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: document_sources_v65 document_tenant_v65; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY document_tenant_v65 ON public.document_sources_v65 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: document_jobs_v65 document_worker_jobs_v65; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY document_worker_jobs_v65 ON public.document_jobs_v65 TO agiparel_doc_worker USING (true) WITH CHECK (true);


--
-- Name: document_segments_v65 document_worker_segments_v65; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY document_worker_segments_v65 ON public.document_segments_v65 TO agiparel_doc_worker USING (true) WITH CHECK (true);


--
-- Name: document_sources_v65 document_worker_sources_v65; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY document_worker_sources_v65 ON public.document_sources_v65 FOR SELECT TO agiparel_doc_worker USING (true);


--
-- Name: documents; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.documents ENABLE ROW LEVEL SECURITY;

--
-- Name: dynamic_work_orders; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.dynamic_work_orders ENABLE ROW LEVEL SECURITY;

--
-- Name: fabric_shrinkage_observations_v30 f317c_f318a_tenant_v30; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY f317c_f318a_tenant_v30 ON public.fabric_shrinkage_observations_v30 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_vendor_acks_v30 f317c_f318a_tenant_v30; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY f317c_f318a_tenant_v30 ON public.jobwork_vendor_acks_v30 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_vendor_keys_v30 f317c_f318a_tenant_v30; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY f317c_f318a_tenant_v30 ON public.jobwork_vendor_keys_v30 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: fabric_calibration_links_v32; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.fabric_calibration_links_v32 ENABLE ROW LEVEL SECURITY;

--
-- Name: fabric_shrinkage_observations_v30; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.fabric_shrinkage_observations_v30 ENABLE ROW LEVEL SECURITY;

--
-- Name: fabric_shrinkage_specs_v31; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.fabric_shrinkage_specs_v31 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_approved_reference_versions_v65b5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_approved_reference_versions_v65b5 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_candidate_review_versions_v65b5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_candidate_review_versions_v65b5 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_candidate_reviews_v65b2; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_candidate_reviews_v65b2 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_candidate_reviews_v65b2 factory_candidates_tenant_v65b2; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_candidates_tenant_v65b2 ON public.factory_candidate_reviews_v65b2 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_document_candidates_v65b2 factory_candidates_tenant_v65b2; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_candidates_tenant_v65b2 ON public.factory_document_candidates_v65b2 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_proposed_register_v65b2 factory_candidates_tenant_v65b2; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_candidates_tenant_v65b2 ON public.factory_proposed_register_v65b2 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_document_candidates_v65b2; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_document_candidates_v65b2 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_document_submissions_v65b5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_document_submissions_v65b5 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_draft_decisions_v65b3; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_draft_decisions_v65b3 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_draft_decisions_v65b3 factory_draft_tenant_v65b3; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_draft_tenant_v65b3 ON public.factory_draft_decisions_v65b3 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_fact_events_v65b; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_fact_events_v65b ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_fact_events_v65b factory_fact_tenant_v65b; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_fact_tenant_v65b ON public.factory_fact_events_v65b TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_instrument_calibrations_v32; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_instrument_calibrations_v32 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_proposed_register_v65b2; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_proposed_register_v65b2 ENABLE ROW LEVEL SECURITY;

--
-- Name: factory_reference_versions_v65b4 factory_reference_tenant_v65b4; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_reference_tenant_v65b4 ON public.factory_reference_versions_v65b4 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_reference_versions_v65b4; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.factory_reference_versions_v65b4 ENABLE ROW LEVEL SECURITY;

--
-- Name: fabric_shrinkage_specs_v31 factory_tenant_v31; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v31 ON public.fabric_shrinkage_specs_v31 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_discrepancy_resolutions_v31 factory_tenant_v31; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v31 ON public.jobwork_discrepancy_resolutions_v31 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: buyer_aql_plans_v32 factory_tenant_v32; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v32 ON public.buyer_aql_plans_v32 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: buyer_aql_sample_events_v32 factory_tenant_v32; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v32 ON public.buyer_aql_sample_events_v32 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: fabric_calibration_links_v32 factory_tenant_v32; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v32 ON public.fabric_calibration_links_v32 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_instrument_calibrations_v32 factory_tenant_v32; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v32 ON public.factory_instrument_calibrations_v32 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: needle_safety_events_v32 factory_tenant_v32; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v32 ON public.needle_safety_events_v32 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: buyer_aql_review_rules_v33 factory_tenant_v33; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v33 ON public.buyer_aql_review_rules_v33 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: buyer_aql_reviews_v33 factory_tenant_v33; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v33 ON public.buyer_aql_reviews_v33 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: needle_safety_clearances_v33 factory_tenant_v33; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v33 ON public.needle_safety_clearances_v33 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_approved_reference_versions_v65b5 factory_tenant_v65b5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v65b5 ON public.factory_approved_reference_versions_v65b5 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_candidate_review_versions_v65b5 factory_tenant_v65b5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v65b5 ON public.factory_candidate_review_versions_v65b5 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: factory_document_submissions_v65b5 factory_tenant_v65b5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY factory_tenant_v65b5 ON public.factory_document_submissions_v65b5 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: floor_issues; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.floor_issues ENABLE ROW LEVEL SECURITY;

--
-- Name: gen_bundles_v18; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.gen_bundles_v18 ENABLE ROW LEVEL SECURITY;

--
-- Name: gen_carrier_active_v18; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.gen_carrier_active_v18 ENABLE ROW LEVEL SECURITY;

--
-- Name: gen_carrier_events_v18; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.gen_carrier_events_v18 ENABLE ROW LEVEL SECURITY;

--
-- Name: gen_carriers_v18; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.gen_carriers_v18 ENABLE ROW LEVEL SECURITY;

--
-- Name: gen_cut_parts_v18; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.gen_cut_parts_v18 ENABLE ROW LEVEL SECURITY;

--
-- Name: gen_garments_v18; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.gen_garments_v18 ENABLE ROW LEVEL SECURITY;

--
-- Name: gen_bundles_v18 gen_tenant_v18; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY gen_tenant_v18 ON public.gen_bundles_v18 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: gen_carrier_active_v18 gen_tenant_v18; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY gen_tenant_v18 ON public.gen_carrier_active_v18 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: gen_carrier_events_v18 gen_tenant_v18; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY gen_tenant_v18 ON public.gen_carrier_events_v18 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: gen_carriers_v18 gen_tenant_v18; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY gen_tenant_v18 ON public.gen_carriers_v18 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: gen_cut_parts_v18 gen_tenant_v18; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY gen_tenant_v18 ON public.gen_cut_parts_v18 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: gen_garments_v18 gen_tenant_v18; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY gen_tenant_v18 ON public.gen_garments_v18 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: hangtag_binds; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.hangtag_binds ENABLE ROW LEVEL SECURITY;

--
-- Name: hourly_line_metrics; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.hourly_line_metrics ENABLE ROW LEVEL SECURITY;

--
-- Name: intake_document_capture intake_capture_tenant_policy; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY intake_capture_tenant_policy ON public.intake_document_capture TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: intake_document_capture; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.intake_document_capture ENABLE ROW LEVEL SECURITY;

--
-- Name: intake_proposed_fact intake_fact_tenant_policy; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY intake_fact_tenant_policy ON public.intake_proposed_fact TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: intake_proposed_fact; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.intake_proposed_fact ENABLE ROW LEVEL SECURITY;

--
-- Name: intel_sim_snapshots_v12 intel_sim_snapshot_tenant_v12; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY intel_sim_snapshot_tenant_v12 ON public.intel_sim_snapshots_v12 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: intel_sim_snapshots_v12; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.intel_sim_snapshots_v12 ENABLE ROW LEVEL SECURITY;

--
-- Name: intelligence_prod_readers_v13 intelligence_prod_reader_tenant_v13; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY intelligence_prod_reader_tenant_v13 ON public.intelligence_prod_readers_v13 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: intelligence_prod_readers_v13; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.intelligence_prod_readers_v13 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_discrepancies_v29; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_discrepancies_v29 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_discrepancy_resolutions_v31; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_discrepancy_resolutions_v31 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_discrepancies_v29 jobwork_discrepancy_tenant_v29; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY jobwork_discrepancy_tenant_v29 ON public.jobwork_discrepancies_v29 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_inwards_v28; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_inwards_v28 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_outwards_v28; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_outwards_v28 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_inwards_v28 jobwork_tenant_v28; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY jobwork_tenant_v28 ON public.jobwork_inwards_v28 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_outwards_v28 jobwork_tenant_v28; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY jobwork_tenant_v28 ON public.jobwork_outwards_v28 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_used_proofs_v28 jobwork_tenant_v28; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY jobwork_tenant_v28 ON public.jobwork_used_proofs_v28 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_used_scans_v28 jobwork_tenant_v28; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY jobwork_tenant_v28 ON public.jobwork_used_scans_v28 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_vendors_v28 jobwork_tenant_v28; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY jobwork_tenant_v28 ON public.jobwork_vendors_v28 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: jobwork_used_proofs_v28; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_used_proofs_v28 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_used_scans_v28; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_used_scans_v28 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_vendor_acks_v30; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_vendor_acks_v30 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_vendor_keys_v30; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_vendor_keys_v30 ENABLE ROW LEVEL SECURITY;

--
-- Name: jobwork_vendors_v28; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.jobwork_vendors_v28 ENABLE ROW LEVEL SECURITY;

--
-- Name: lines; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.lines ENABLE ROW LEVEL SECURITY;

--
-- Name: machine_roster; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.machine_roster ENABLE ROW LEVEL SECURITY;

--
-- Name: material_ceo_principals_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_ceo_principals_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: material_issues_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_issues_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: material_outbox_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_outbox_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: material_reservations_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_reservations_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: material_scenarios_v6; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_scenarios_v6 ENABLE ROW LEVEL SECURITY;

--
-- Name: material_used_proofs_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_used_proofs_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: material_variances_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_variances_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: material_write_scopes_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.material_write_scopes_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: needle_safety_clearances_v33; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.needle_safety_clearances_v33 ENABLE ROW LEVEL SECURITY;

--
-- Name: needle_safety_events_v32; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.needle_safety_events_v32 ENABLE ROW LEVEL SECURITY;

--
-- Name: ocr_records; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.ocr_records ENABLE ROW LEVEL SECURITY;

--
-- Name: order_intake_bom_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.order_intake_bom_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: order_intake_skus_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.order_intake_skus_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: order_intakes_v5; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.order_intakes_v5 ENABLE ROW LEVEL SECURITY;

--
-- Name: order_sku_matrix; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.order_sku_matrix ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_carton_events_v22; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_carton_events_v22 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_carton_specs_v22; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_carton_specs_v22 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_manifest_events_v22; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_manifest_events_v22 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_prod_assignments_v12; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_prod_assignments_v12 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_prod_holds_v12; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_prod_holds_v12 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_prod_assignments_v12 pack_prod_tenant_v12; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY pack_prod_tenant_v12 ON public.pack_prod_assignments_v12 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_prod_holds_v12 pack_prod_tenant_v12; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY pack_prod_tenant_v12 ON public.pack_prod_holds_v12 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_sim_cartons_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_sim_cartons_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_sim_events_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_sim_events_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_sim_items_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_sim_items_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_sim_manifest_cartons_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_sim_manifest_cartons_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_sim_manifests_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_sim_manifests_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_sim_cartons_v11 pack_sim_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY pack_sim_tenant_v11 ON public.pack_sim_cartons_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_sim_events_v11 pack_sim_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY pack_sim_tenant_v11 ON public.pack_sim_events_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_sim_items_v11 pack_sim_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY pack_sim_tenant_v11 ON public.pack_sim_items_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_sim_manifest_cartons_v11 pack_sim_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY pack_sim_tenant_v11 ON public.pack_sim_manifest_cartons_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_sim_manifests_v11 pack_sim_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY pack_sim_tenant_v11 ON public.pack_sim_manifests_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_size_weights_v22; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.pack_size_weights_v22 ENABLE ROW LEVEL SECURITY;

--
-- Name: pack_carton_events_v22 packing_v22_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY packing_v22_tenant ON public.pack_carton_events_v22 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_carton_specs_v22 packing_v22_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY packing_v22_tenant ON public.pack_carton_specs_v22 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_manifest_events_v22 packing_v22_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY packing_v22_tenant ON public.pack_manifest_events_v22 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: pack_size_weights_v22 packing_v22_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY packing_v22_tenant ON public.pack_size_weights_v22 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: plant_hardware_assets; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.plant_hardware_assets ENABLE ROW LEVEL SECURITY;

--
-- Name: post_sewing_processes; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.post_sewing_processes ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_garment_defects_v21; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_garment_defects_v21 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_garment_events_v21; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_garment_events_v21 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_garment_measure_rules_v21; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_garment_measure_rules_v21 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_garment_specs_v21; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_garment_specs_v21 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_prod_assignments_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_prod_assignments_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_prod_defect_catalog_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_prod_defect_catalog_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_prod_group_events_v11; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_prod_group_events_v11 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_prod_assignments_v11 qc_prod_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_prod_tenant_v11 ON public.qc_prod_assignments_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_prod_defect_catalog_v11 qc_prod_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_prod_tenant_v11 ON public.qc_prod_defect_catalog_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_prod_group_events_v11 qc_prod_tenant_v11; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_prod_tenant_v11 ON public.qc_prod_group_events_v11 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_sim_buyer_bindings_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_sim_buyer_bindings_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_sim_events_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_sim_events_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_sim_routes_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_sim_routes_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_sim_buyer_bindings_v10 qc_sim_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_sim_tenant_v10 ON public.qc_sim_buyer_bindings_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_sim_events_v10 qc_sim_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_sim_tenant_v10 ON public.qc_sim_events_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_sim_routes_v10 qc_sim_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_sim_tenant_v10 ON public.qc_sim_routes_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_sim_units_v10 qc_sim_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_sim_tenant_v10 ON public.qc_sim_units_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_sim_units_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.qc_sim_units_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: qc_garment_defects_v21 qc_v21_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_v21_tenant ON public.qc_garment_defects_v21 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_garment_events_v21 qc_v21_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_v21_tenant ON public.qc_garment_events_v21 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_garment_measure_rules_v21 qc_v21_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_v21_tenant ON public.qc_garment_measure_rules_v21 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: qc_garment_specs_v21 qc_v21_tenant; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY qc_v21_tenant ON public.qc_garment_specs_v21 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_piece_completions_v27; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_piece_completions_v27 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_piece_completions_v27 sew_piece_tenant_v27; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_piece_tenant_v27 ON public.sew_piece_completions_v27 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_piece_used_scans_v27 sew_piece_tenant_v27; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_piece_tenant_v27 ON public.sew_piece_used_scans_v27 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_piece_used_scans_v27; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_piece_used_scans_v27 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_prod_completions_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_prod_completions_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_prod_gateways_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_prod_gateways_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_prod_observations_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_prod_observations_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_prod_routes_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_prod_routes_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_prod_completions_v10 sew_prod_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_prod_tenant_v10 ON public.sew_prod_completions_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_prod_gateways_v10 sew_prod_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_prod_tenant_v10 ON public.sew_prod_gateways_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_prod_observations_v10 sew_prod_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_prod_tenant_v10 ON public.sew_prod_observations_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_prod_routes_v10 sew_prod_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_prod_tenant_v10 ON public.sew_prod_routes_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_prod_used_observations_v10 sew_prod_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_prod_tenant_v10 ON public.sew_prod_used_observations_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_prod_worker_stations_v10 sew_prod_tenant_v10; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_prod_tenant_v10 ON public.sew_prod_worker_stations_v10 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_prod_used_observations_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_prod_used_observations_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_prod_worker_stations_v10; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_prod_worker_stations_v10 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_sim_credits_v9; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_sim_credits_v9 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_sim_cycle_inputs_v9; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_sim_cycle_inputs_v9 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_sim_cycles_v9; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_sim_cycles_v9 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_sim_events_v9; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_sim_events_v9 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_sim_operations_v9; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_sim_operations_v9 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_sim_scans_v9; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.sew_sim_scans_v9 ENABLE ROW LEVEL SECURITY;

--
-- Name: sew_sim_credits_v9 sew_sim_tenant_v9; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_sim_tenant_v9 ON public.sew_sim_credits_v9 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_sim_cycle_inputs_v9 sew_sim_tenant_v9; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_sim_tenant_v9 ON public.sew_sim_cycle_inputs_v9 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_sim_cycles_v9 sew_sim_tenant_v9; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_sim_tenant_v9 ON public.sew_sim_cycles_v9 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_sim_events_v9 sew_sim_tenant_v9; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_sim_tenant_v9 ON public.sew_sim_events_v9 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_sim_operations_v9 sew_sim_tenant_v9; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_sim_tenant_v9 ON public.sew_sim_operations_v9 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: sew_sim_scans_v9 sew_sim_tenant_v9; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY sew_sim_tenant_v9 ON public.sew_sim_scans_v9 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_issuances; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_issuances ENABLE ROW LEVEL SECURITY;

--
-- Name: store_pick_proposals_v7 store_pick_proposals_tenant_v7; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY store_pick_proposals_tenant_v7 ON public.store_pick_proposals_v7 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_pick_proposals_v7; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_pick_proposals_v7 ENABLE ROW LEVEL SECURITY;

--
-- Name: store_reconciliation_requests_v7; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_reconciliation_requests_v7 ENABLE ROW LEVEL SECURITY;

--
-- Name: store_reconciliation_requests_v7 store_reconciliation_tenant_v7; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY store_reconciliation_tenant_v7 ON public.store_reconciliation_requests_v7 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_roll_admission_proofs_v7; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_roll_admission_proofs_v7 ENABLE ROW LEVEL SECURITY;

--
-- Name: store_roll_admissions_v7; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_roll_admissions_v7 ENABLE ROW LEVEL SECURITY;

--
-- Name: store_roll_passport_events_v7; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_roll_passport_events_v7 ENABLE ROW LEVEL SECURITY;

--
-- Name: store_roll_rfid_bindings_v7; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_roll_rfid_bindings_v7 ENABLE ROW LEVEL SECURITY;

--
-- Name: store_rolls; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_rolls ENABLE ROW LEVEL SECURITY;

--
-- Name: store_roll_passport_events_v7 store_tenant_v7; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY store_tenant_v7 ON public.store_roll_passport_events_v7 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_roll_rfid_bindings_v7 store_tenant_v7; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY store_tenant_v7 ON public.store_roll_rfid_bindings_v7 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_trims_v13 store_trims_tenant_v13; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY store_trims_tenant_v13 ON public.store_trims_v13 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_trims_v13; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.store_trims_v13 ENABLE ROW LEVEL SECURITY;

--
-- Name: store_roll_admission_proofs_v7 stores_admission_tenant_v7; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY stores_admission_tenant_v7 ON public.store_roll_admission_proofs_v7 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_roll_admissions_v7 stores_admission_tenant_v7; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY stores_admission_tenant_v7 ON public.store_roll_admissions_v7 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: style_assembly_dag; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.style_assembly_dag ENABLE ROW LEVEL SECURITY;

--
-- Name: style_parts; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.style_parts ENABLE ROW LEVEL SECURITY;

--
-- Name: styles_master; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.styles_master ENABLE ROW LEVEL SECURITY;

--
-- Name: attendance_register tenant_isolation_attendance_register; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_attendance_register ON public.attendance_register USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: bom_trim_requirements tenant_isolation_bom_trim_requirements; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_bom_trim_requirements ON public.bom_trim_requirements USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: bundle_registry tenant_isolation_bundle_registry; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_bundle_registry ON public.bundle_registry USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: carton_pack_manifests tenant_isolation_carton_pack_manifests; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_carton_pack_manifests ON public.carton_pack_manifests USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: cut_lay_rolls tenant_isolation_cut_lay_rolls; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_cut_lay_rolls ON public.cut_lay_rolls USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: documents tenant_isolation_documents; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_documents ON public.documents USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: dynamic_work_orders tenant_isolation_dynamic_work_orders; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_dynamic_work_orders ON public.dynamic_work_orders USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: floor_issues tenant_isolation_floor_issues; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_floor_issues ON public.floor_issues USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: hangtag_binds tenant_isolation_hangtag_binds; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_hangtag_binds ON public.hangtag_binds USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: hourly_line_metrics tenant_isolation_hourly_line_metrics; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_hourly_line_metrics ON public.hourly_line_metrics USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: lines tenant_isolation_lines; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_lines ON public.lines USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: machine_roster tenant_isolation_machine_roster; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_machine_roster ON public.machine_roster USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: ocr_records tenant_isolation_ocr_records; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_ocr_records ON public.ocr_records USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: order_sku_matrix tenant_isolation_order_sku_matrix; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_order_sku_matrix ON public.order_sku_matrix USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: plant_hardware_assets tenant_isolation_plant_hardware_assets; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_plant_hardware_assets ON public.plant_hardware_assets USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: post_sewing_processes tenant_isolation_post_sewing_processes; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_post_sewing_processes ON public.post_sewing_processes USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_issuances tenant_isolation_store_issuances; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_store_issuances ON public.store_issuances USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: store_rolls tenant_isolation_store_rolls; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_store_rolls ON public.store_rolls USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: style_assembly_dag tenant_isolation_style_assembly_dag; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_style_assembly_dag ON public.style_assembly_dag USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: style_parts tenant_isolation_style_parts; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_style_parts ON public.style_parts USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: styles_master tenant_isolation_styles_master; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_styles_master ON public.styles_master USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_ceo_principals_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.material_ceo_principals_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_issues_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.material_issues_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_outbox_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.material_outbox_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_reservations_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.material_reservations_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_used_proofs_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.material_used_proofs_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_variances_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.material_variances_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_write_scopes_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.material_write_scopes_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: order_intake_bom_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.order_intake_bom_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: order_intake_skus_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.order_intake_skus_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: order_intakes_v5 tenant_isolation_v5; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v5 ON public.order_intakes_v5 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: material_scenarios_v6 tenant_isolation_v6; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_v6 ON public.material_scenarios_v6 USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: workers tenant_isolation_workers; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY tenant_isolation_workers ON public.workers USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: trim_admission_proof_uses_v25 trim_admission_proof_tenant_v25; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY trim_admission_proof_tenant_v25 ON public.trim_admission_proof_uses_v25 FOR SELECT TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: trim_admission_proof_uses_v25; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.trim_admission_proof_uses_v25 ENABLE ROW LEVEL SECURITY;

--
-- Name: trim_stock_admissions_v25 trim_admission_tenant_v25; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY trim_admission_tenant_v25 ON public.trim_stock_admissions_v25 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: trim_issuance_proof_uses_v24; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.trim_issuance_proof_uses_v24 ENABLE ROW LEVEL SECURITY;

--
-- Name: trim_issuances_v13 trim_issuances_tenant_v13; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY trim_issuances_tenant_v13 ON public.trim_issuances_v13 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: trim_issuances_v13; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.trim_issuances_v13 ENABLE ROW LEVEL SECURITY;

--
-- Name: trim_issuance_proof_uses_v24 trim_proof_tenant_v24; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY trim_proof_tenant_v24 ON public.trim_issuance_proof_uses_v24 FOR SELECT TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: trim_stock_admissions_v25; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.trim_stock_admissions_v25 ENABLE ROW LEVEL SECURITY;

--
-- Name: wip_exception_cases_v23; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.wip_exception_cases_v23 ENABLE ROW LEVEL SECURITY;

--
-- Name: wip_exception_closures_v23; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.wip_exception_closures_v23 ENABLE ROW LEVEL SECURITY;

--
-- Name: wip_exception_cases_v23 wip_exception_tenant_v23; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY wip_exception_tenant_v23 ON public.wip_exception_cases_v23 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: wip_exception_closures_v23 wip_exception_tenant_v23; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY wip_exception_tenant_v23 ON public.wip_exception_closures_v23 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: wip_gateway_scans_v20; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.wip_gateway_scans_v20 ENABLE ROW LEVEL SECURITY;

--
-- Name: wip_movements_v20; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.wip_movements_v20 ENABLE ROW LEVEL SECURITY;

--
-- Name: wip_piece_holds_v26; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.wip_piece_holds_v26 ENABLE ROW LEVEL SECURITY;

--
-- Name: wip_piece_reviews_v26; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.wip_piece_reviews_v26 ENABLE ROW LEVEL SECURITY;

--
-- Name: wip_piece_holds_v26 wip_piece_tenant_v26; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY wip_piece_tenant_v26 ON public.wip_piece_holds_v26 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: wip_piece_reviews_v26 wip_piece_tenant_v26; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY wip_piece_tenant_v26 ON public.wip_piece_reviews_v26 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: wip_gateway_scans_v20 wip_tenant_v20; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY wip_tenant_v20 ON public.wip_gateway_scans_v20 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: wip_movements_v20 wip_tenant_v20; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY wip_tenant_v20 ON public.wip_movements_v20 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: wip_transfer_routes_v20 wip_tenant_v20; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY wip_tenant_v20 ON public.wip_transfer_routes_v20 TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))) WITH CHECK (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: wip_transfer_routes_v20; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.wip_transfer_routes_v20 ENABLE ROW LEVEL SECURITY;

--
-- Name: workers; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.workers ENABLE ROW LEVEL SECURITY;

--
-- Name: workforce_browser_sessions_v16; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.workforce_browser_sessions_v16 ENABLE ROW LEVEL SECURITY;

--
-- Name: workforce_identity_grants_v15 workforce_grant_tenant_v15; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY workforce_grant_tenant_v15 ON public.workforce_identity_grants_v15 FOR SELECT TO agiparel_app USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: workforce_identity_grants_v15; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.workforce_identity_grants_v15 ENABLE ROW LEVEL SECURITY;

--
-- Name: workforce_oidc_requests_v16; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.workforce_oidc_requests_v16 ENABLE ROW LEVEL SECURITY;

--
-- Name: plants; Type: ROW SECURITY; Schema: s01_identity; Owner: -
--

ALTER TABLE s01_identity.plants ENABLE ROW LEVEL SECURITY;

--
-- Name: roles; Type: ROW SECURITY; Schema: s01_identity; Owner: -
--

ALTER TABLE s01_identity.roles ENABLE ROW LEVEL SECURITY;

--
-- Name: user_role_grants tenant_isolation_grants; Type: POLICY; Schema: s01_identity; Owner: -
--

CREATE POLICY tenant_isolation_grants ON s01_identity.user_role_grants USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: plants tenant_isolation_plants; Type: POLICY; Schema: s01_identity; Owner: -
--

CREATE POLICY tenant_isolation_plants ON s01_identity.plants USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: roles tenant_isolation_roles; Type: POLICY; Schema: s01_identity; Owner: -
--

CREATE POLICY tenant_isolation_roles ON s01_identity.roles USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: users tenant_isolation_users; Type: POLICY; Schema: s01_identity; Owner: -
--

CREATE POLICY tenant_isolation_users ON s01_identity.users USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: user_role_grants; Type: ROW SECURITY; Schema: s01_identity; Owner: -
--

ALTER TABLE s01_identity.user_role_grants ENABLE ROW LEVEL SECURITY;

--
-- Name: users; Type: ROW SECURITY; Schema: s01_identity; Owner: -
--

ALTER TABLE s01_identity.users ENABLE ROW LEVEL SECURITY;

--
-- Name: registered_devices; Type: ROW SECURITY; Schema: s02_device; Owner: -
--

ALTER TABLE s02_device.registered_devices ENABLE ROW LEVEL SECURITY;

--
-- Name: registered_devices tenant_isolation_devices; Type: POLICY; Schema: s02_device; Owner: -
--

CREATE POLICY tenant_isolation_devices ON s02_device.registered_devices USING (((tenant_id)::text = NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text)));


--
-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: -
--

GRANT USAGE ON SCHEMA public TO agiparel_app;
GRANT USAGE ON SCHEMA public TO agiparel_doc_worker;


--
-- Name: FUNCTION apply_gen_carrier_event_v18(); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.apply_gen_carrier_event_v18() FROM PUBLIC;


--
-- Name: FUNCTION dock_integrity_report_v38(t text, p text, m uuid); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.dock_integrity_report_v38(t text, p text, m uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION public.dock_integrity_report_v38(t text, p text, m uuid) TO agiparel_app;


--
-- Name: FUNCTION dock_lineage_audit_v37(t text, p text, m uuid); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.dock_lineage_audit_v37(t text, p text, m uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION public.dock_lineage_audit_v37(t text, p text, m uuid) TO agiparel_app;


--
-- Name: FUNCTION dock_material_trace_v38(t text, p text, m uuid); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.dock_material_trace_v38(t text, p text, m uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION public.dock_material_trace_v38(t text, p text, m uuid) TO agiparel_app;


--
-- Name: FUNCTION jobwork_reserve_evidence_v28(); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.jobwork_reserve_evidence_v28() FROM PUBLIC;


--
-- Name: FUNCTION material_scope_read_locked_v5(p_tenant character varying, p_order character varying, p_plant character varying, p_holder text); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.material_scope_read_locked_v5(p_tenant character varying, p_order character varying, p_plant character varying, p_holder text) FROM PUBLIC;
GRANT ALL ON FUNCTION public.material_scope_read_locked_v5(p_tenant character varying, p_order character varying, p_plant character varying, p_holder text) TO agiparel_app;


--
-- Name: FUNCTION reserve_sew_piece_scans_v27(); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.reserve_sew_piece_scans_v27() FROM PUBLIC;


--
-- Name: FUNCTION workforce_oidc_begin_v16(p_state bytea, p_browser bytea, p_nonce text, p_pkce text); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.workforce_oidc_begin_v16(p_state bytea, p_browser bytea, p_nonce text, p_pkce text) FROM PUBLIC;
GRANT ALL ON FUNCTION public.workforce_oidc_begin_v16(p_state bytea, p_browser bytea, p_nonce text, p_pkce text) TO agiparel_app;


--
-- Name: FUNCTION workforce_oidc_consume_v16(p_state bytea, p_browser bytea); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.workforce_oidc_consume_v16(p_state bytea, p_browser bytea) FROM PUBLIC;
GRANT ALL ON FUNCTION public.workforce_oidc_consume_v16(p_state bytea, p_browser bytea) TO agiparel_app;


--
-- Name: FUNCTION workforce_session_create_v16(p_session bytea, p_csrf bytea, p_issuer text, p_subject text); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.workforce_session_create_v16(p_session bytea, p_csrf bytea, p_issuer text, p_subject text) FROM PUBLIC;
GRANT ALL ON FUNCTION public.workforce_session_create_v16(p_session bytea, p_csrf bytea, p_issuer text, p_subject text) TO agiparel_app;


--
-- Name: FUNCTION workforce_session_csrf_rotate_v16(p_session bytea, p_csrf bytea); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.workforce_session_csrf_rotate_v16(p_session bytea, p_csrf bytea) FROM PUBLIC;
GRANT ALL ON FUNCTION public.workforce_session_csrf_rotate_v16(p_session bytea, p_csrf bytea) TO agiparel_app;


--
-- Name: FUNCTION workforce_session_get_v16(p_session bytea); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.workforce_session_get_v16(p_session bytea) FROM PUBLIC;
GRANT ALL ON FUNCTION public.workforce_session_get_v16(p_session bytea) TO agiparel_app;


--
-- Name: FUNCTION workforce_session_revoke_v16(p_session bytea); Type: ACL; Schema: public; Owner: -
--

REVOKE ALL ON FUNCTION public.workforce_session_revoke_v16(p_session bytea) FROM PUBLIC;
GRANT ALL ON FUNCTION public.workforce_session_revoke_v16(p_session bytea) TO agiparel_app;


--
-- Name: TABLE attendance_register; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.attendance_register TO agiparel_app;


--
-- Name: TABLE bom_trim_requirements; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.bom_trim_requirements TO agiparel_app;


--
-- Name: TABLE bundle_registry; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.bundle_registry TO agiparel_app;


--
-- Name: TABLE buyer_aql_plans_v32; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.buyer_aql_plans_v32 TO agiparel_app;


--
-- Name: TABLE buyer_aql_review_rules_v33; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.buyer_aql_review_rules_v33 TO agiparel_app;


--
-- Name: TABLE buyer_aql_reviews_v33; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.buyer_aql_reviews_v33 TO agiparel_app;


--
-- Name: TABLE buyer_aql_sample_events_v32; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.buyer_aql_sample_events_v32 TO agiparel_app;


--
-- Name: TABLE buyer_order_handoffs_v65c1; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.buyer_order_handoffs_v65c1 TO agiparel_app;


--
-- Name: SEQUENCE buyer_order_handoffs_v65c1_id_seq; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,USAGE ON SEQUENCE public.buyer_order_handoffs_v65c1_id_seq TO agiparel_app;


--
-- Name: TABLE carton_pack_manifests; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.carton_pack_manifests TO agiparel_app;


--
-- Name: TABLE cut_bundle_drafts_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_bundle_drafts_v8 TO agiparel_app;


--
-- Name: TABLE cut_bundle_scan_witnesses_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_bundle_scan_witnesses_v8 TO agiparel_app;


--
-- Name: TABLE cut_consumption_reconciliations_v19; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_consumption_reconciliations_v19 TO agiparel_app;


--
-- Name: TABLE cut_defect_codes_v19; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.cut_defect_codes_v19 TO agiparel_app;


--
-- Name: TABLE cut_lay_drafts_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_lay_drafts_v8 TO agiparel_app;


--
-- Name: TABLE cut_lay_positions_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_lay_positions_v8 TO agiparel_app;


--
-- Name: TABLE cut_lay_rolls; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.cut_lay_rolls TO agiparel_app;


--
-- Name: TABLE cut_lay_snapshots_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_lay_snapshots_v8 TO agiparel_app;


--
-- Name: TABLE cut_part_outcomes_v19; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_part_outcomes_v19 TO agiparel_app;


--
-- Name: TABLE cut_proof_consumptions_v19; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_proof_consumptions_v19 TO agiparel_app;


--
-- Name: TABLE cut_reader_events_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_reader_events_v8 TO agiparel_app;


--
-- Name: TABLE cut_reader_gateways_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.cut_reader_gateways_v8 TO agiparel_app;


--
-- Name: TABLE cut_recut_links_v19; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_recut_links_v19 TO agiparel_app;


--
-- Name: TABLE cut_sewing_releases_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_sewing_releases_v8 TO agiparel_app;


--
-- Name: TABLE cut_spread_issue_allocations_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_spread_issue_allocations_v8 TO agiparel_app;


--
-- Name: TABLE cut_spread_reports_v8; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.cut_spread_reports_v8 TO agiparel_app;


--
-- Name: TABLE dock_asn_drafts_v34; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.dock_asn_drafts_v34 TO agiparel_app;


--
-- Name: TABLE dock_buyer_spec_bindings_v36; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.dock_buyer_spec_bindings_v36 TO agiparel_app;


--
-- Name: TABLE dock_carrier_authorizations_v34; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.dock_carrier_authorizations_v34 TO agiparel_app;


--
-- Name: TABLE dock_carrier_receipts_v34; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.dock_carrier_receipts_v34 TO agiparel_app;


--
-- Name: TABLE dock_carton_scans_v34; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.dock_carton_scans_v34 TO agiparel_app;


--
-- Name: TABLE dock_container_seals_v34; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.dock_container_seals_v34 TO agiparel_app;


--
-- Name: TABLE dock_external_assertions_v35; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.dock_external_assertions_v35 TO agiparel_app;


--
-- Name: TABLE dock_external_signers_v35; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.dock_external_signers_v35 TO agiparel_app;


--
-- Name: TABLE dock_gateways_v34; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.dock_gateways_v34 TO agiparel_app;


--
-- Name: TABLE dock_station_policies_v34; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.dock_station_policies_v34 TO agiparel_app;


--
-- Name: TABLE document_assertions_v65; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.document_assertions_v65 TO agiparel_app;


--
-- Name: TABLE document_jobs_v65; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.document_jobs_v65 TO agiparel_app;
GRANT SELECT ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: COLUMN document_jobs_v65.status; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(status) ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: COLUMN document_jobs_v65.attempts; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(attempts) ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: COLUMN document_jobs_v65.outcome; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(outcome) ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: COLUMN document_jobs_v65.segment_count; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(segment_count) ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: COLUMN document_jobs_v65.started_at; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(started_at) ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: COLUMN document_jobs_v65.finished_at; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(finished_at) ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: COLUMN document_jobs_v65.updated_at; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(updated_at) ON TABLE public.document_jobs_v65 TO agiparel_doc_worker;


--
-- Name: TABLE document_segments_v65; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.document_segments_v65 TO agiparel_app;
GRANT SELECT,INSERT ON TABLE public.document_segments_v65 TO agiparel_doc_worker;


--
-- Name: TABLE document_sources_v65; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.document_sources_v65 TO agiparel_app;
GRANT SELECT ON TABLE public.document_sources_v65 TO agiparel_doc_worker;


--
-- Name: TABLE documents; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.documents TO agiparel_app;


--
-- Name: TABLE dynamic_work_orders; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.dynamic_work_orders TO agiparel_app;


--
-- Name: TABLE fabric_calibration_links_v32; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.fabric_calibration_links_v32 TO agiparel_app;


--
-- Name: TABLE fabric_shrinkage_observations_v30; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.fabric_shrinkage_observations_v30 TO agiparel_app;


--
-- Name: TABLE fabric_shrinkage_specs_v31; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.fabric_shrinkage_specs_v31 TO agiparel_app;


--
-- Name: TABLE factory_approved_reference_versions_v65b5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_approved_reference_versions_v65b5 TO agiparel_app;


--
-- Name: TABLE factory_candidate_review_versions_v65b5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_candidate_review_versions_v65b5 TO agiparel_app;


--
-- Name: TABLE factory_candidate_reviews_v65b2; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_candidate_reviews_v65b2 TO agiparel_app;


--
-- Name: TABLE factory_document_candidates_v65b2; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_document_candidates_v65b2 TO agiparel_app;


--
-- Name: TABLE factory_document_submissions_v65b5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_document_submissions_v65b5 TO agiparel_app;


--
-- Name: TABLE factory_draft_decisions_v65b3; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_draft_decisions_v65b3 TO agiparel_app;


--
-- Name: TABLE factory_fact_events_v65b; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_fact_events_v65b TO agiparel_app;


--
-- Name: TABLE factory_instrument_calibrations_v32; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.factory_instrument_calibrations_v32 TO agiparel_app;


--
-- Name: TABLE factory_proposed_register_v65b2; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_proposed_register_v65b2 TO agiparel_app;


--
-- Name: TABLE factory_reference_versions_v65b4; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.factory_reference_versions_v65b4 TO agiparel_app;


--
-- Name: TABLE floor_issues; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.floor_issues TO agiparel_app;


--
-- Name: TABLE gen_bundles_v18; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.gen_bundles_v18 TO agiparel_app;


--
-- Name: TABLE gen_carrier_active_v18; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.gen_carrier_active_v18 TO agiparel_app;


--
-- Name: TABLE gen_carrier_events_v18; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.gen_carrier_events_v18 TO agiparel_app;


--
-- Name: SEQUENCE gen_carrier_events_v18_event_no_seq; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,USAGE ON SEQUENCE public.gen_carrier_events_v18_event_no_seq TO agiparel_app;


--
-- Name: TABLE gen_carriers_v18; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.gen_carriers_v18 TO agiparel_app;


--
-- Name: TABLE gen_cut_parts_v18; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.gen_cut_parts_v18 TO agiparel_app;


--
-- Name: TABLE gen_garments_v18; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.gen_garments_v18 TO agiparel_app;


--
-- Name: TABLE hangtag_binds; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.hangtag_binds TO agiparel_app;


--
-- Name: TABLE hourly_line_metrics; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.hourly_line_metrics TO agiparel_app;


--
-- Name: TABLE intake_document_capture; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.intake_document_capture TO agiparel_app;


--
-- Name: TABLE intake_proposed_fact; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.intake_proposed_fact TO agiparel_app;


--
-- Name: TABLE intel_sim_snapshots_v12; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.intel_sim_snapshots_v12 TO agiparel_app;


--
-- Name: TABLE intelligence_prod_readers_v13; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.intelligence_prod_readers_v13 TO agiparel_app;


--
-- Name: TABLE jobwork_discrepancies_v29; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.jobwork_discrepancies_v29 TO agiparel_app;


--
-- Name: TABLE jobwork_discrepancy_resolutions_v31; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.jobwork_discrepancy_resolutions_v31 TO agiparel_app;


--
-- Name: TABLE jobwork_inwards_v28; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.jobwork_inwards_v28 TO agiparel_app;


--
-- Name: TABLE jobwork_outwards_v28; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.jobwork_outwards_v28 TO agiparel_app;


--
-- Name: TABLE jobwork_used_proofs_v28; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.jobwork_used_proofs_v28 TO agiparel_app;


--
-- Name: TABLE jobwork_used_scans_v28; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.jobwork_used_scans_v28 TO agiparel_app;


--
-- Name: TABLE jobwork_vendor_acks_v30; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.jobwork_vendor_acks_v30 TO agiparel_app;


--
-- Name: TABLE jobwork_vendor_keys_v30; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.jobwork_vendor_keys_v30 TO agiparel_app;


--
-- Name: TABLE jobwork_vendors_v28; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.jobwork_vendors_v28 TO agiparel_app;


--
-- Name: TABLE lines; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.lines TO agiparel_app;


--
-- Name: TABLE machine_roster; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.machine_roster TO agiparel_app;


--
-- Name: TABLE material_ceo_principals_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.material_ceo_principals_v5 TO agiparel_app;


--
-- Name: TABLE material_issues_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.material_issues_v5 TO agiparel_app;


--
-- Name: TABLE material_outbox_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.material_outbox_v5 TO agiparel_app;


--
-- Name: TABLE material_reservations_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.material_reservations_v5 TO agiparel_app;


--
-- Name: TABLE material_scenarios_v6; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.material_scenarios_v6 TO agiparel_app;


--
-- Name: TABLE material_used_proofs_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.material_used_proofs_v5 TO agiparel_app;


--
-- Name: TABLE material_variances_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.material_variances_v5 TO agiparel_app;


--
-- Name: TABLE material_write_scopes_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.material_write_scopes_v5 TO agiparel_app;


--
-- Name: TABLE needle_safety_clearances_v33; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.needle_safety_clearances_v33 TO agiparel_app;


--
-- Name: TABLE needle_safety_events_v32; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.needle_safety_events_v32 TO agiparel_app;


--
-- Name: TABLE ocr_records; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.ocr_records TO agiparel_app;


--
-- Name: TABLE operation_master; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.operation_master TO agiparel_app;


--
-- Name: TABLE order_intake_bom_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.order_intake_bom_v5 TO agiparel_app;


--
-- Name: TABLE order_intake_skus_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.order_intake_skus_v5 TO agiparel_app;


--
-- Name: TABLE order_intakes_v5; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.order_intakes_v5 TO agiparel_app;


--
-- Name: TABLE order_sku_matrix; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.order_sku_matrix TO agiparel_app;


--
-- Name: TABLE pack_carton_events_v22; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.pack_carton_events_v22 TO agiparel_app;


--
-- Name: TABLE pack_carton_specs_v22; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.pack_carton_specs_v22 TO agiparel_app;


--
-- Name: TABLE pack_manifest_events_v22; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.pack_manifest_events_v22 TO agiparel_app;


--
-- Name: TABLE pack_prod_assignments_v12; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.pack_prod_assignments_v12 TO agiparel_app;


--
-- Name: TABLE pack_prod_holds_v12; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.pack_prod_holds_v12 TO agiparel_app;


--
-- Name: TABLE pack_sim_cartons_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.pack_sim_cartons_v11 TO agiparel_app;


--
-- Name: COLUMN pack_sim_cartons_v11.state; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(state) ON TABLE public.pack_sim_cartons_v11 TO agiparel_app;


--
-- Name: COLUMN pack_sim_cartons_v11.sealed_at; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(sealed_at) ON TABLE public.pack_sim_cartons_v11 TO agiparel_app;


--
-- Name: TABLE pack_sim_events_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.pack_sim_events_v11 TO agiparel_app;


--
-- Name: TABLE pack_sim_items_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE ON TABLE public.pack_sim_items_v11 TO agiparel_app;


--
-- Name: TABLE pack_sim_manifest_cartons_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.pack_sim_manifest_cartons_v11 TO agiparel_app;


--
-- Name: TABLE pack_sim_manifests_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.pack_sim_manifests_v11 TO agiparel_app;


--
-- Name: COLUMN pack_sim_manifests_v11.state; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(state) ON TABLE public.pack_sim_manifests_v11 TO agiparel_app;


--
-- Name: COLUMN pack_sim_manifests_v11.dispatched_at; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(dispatched_at) ON TABLE public.pack_sim_manifests_v11 TO agiparel_app;


--
-- Name: TABLE pack_size_weights_v22; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.pack_size_weights_v22 TO agiparel_app;


--
-- Name: TABLE plant_hardware_assets; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.plant_hardware_assets TO agiparel_app;


--
-- Name: TABLE post_sewing_processes; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.post_sewing_processes TO agiparel_app;


--
-- Name: TABLE qc_garment_defects_v21; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.qc_garment_defects_v21 TO agiparel_app;


--
-- Name: TABLE qc_garment_events_v21; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.qc_garment_events_v21 TO agiparel_app;


--
-- Name: TABLE qc_garment_measure_rules_v21; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.qc_garment_measure_rules_v21 TO agiparel_app;


--
-- Name: TABLE qc_garment_specs_v21; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.qc_garment_specs_v21 TO agiparel_app;


--
-- Name: TABLE qc_prod_assignments_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.qc_prod_assignments_v11 TO agiparel_app;


--
-- Name: TABLE qc_prod_defect_catalog_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.qc_prod_defect_catalog_v11 TO agiparel_app;


--
-- Name: TABLE qc_prod_group_events_v11; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.qc_prod_group_events_v11 TO agiparel_app;


--
-- Name: TABLE qc_sim_buyer_bindings_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.qc_sim_buyer_bindings_v10 TO agiparel_app;


--
-- Name: TABLE qc_sim_events_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.qc_sim_events_v10 TO agiparel_app;


--
-- Name: TABLE qc_sim_routes_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.qc_sim_routes_v10 TO agiparel_app;


--
-- Name: TABLE qc_sim_units_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.qc_state; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(qc_state) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.finishing_count; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(finishing_count) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.last_defect; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(last_defect) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.fail_inspector; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(fail_inspector) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.repair_actor; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(repair_actor) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.last_inspector; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(last_inspector) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.revision; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(revision) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: COLUMN qc_sim_units_v10.updated_at; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(updated_at) ON TABLE public.qc_sim_units_v10 TO agiparel_app;


--
-- Name: TABLE sew_piece_completions_v27; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_piece_completions_v27 TO agiparel_app;


--
-- Name: TABLE sew_piece_used_scans_v27; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.sew_piece_used_scans_v27 TO agiparel_app;


--
-- Name: TABLE sew_prod_completions_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_prod_completions_v10 TO agiparel_app;


--
-- Name: TABLE sew_prod_gateways_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.sew_prod_gateways_v10 TO agiparel_app;


--
-- Name: TABLE sew_prod_observations_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_prod_observations_v10 TO agiparel_app;


--
-- Name: TABLE sew_prod_routes_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.sew_prod_routes_v10 TO agiparel_app;


--
-- Name: TABLE sew_prod_used_observations_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_prod_used_observations_v10 TO agiparel_app;


--
-- Name: TABLE sew_prod_worker_stations_v10; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.sew_prod_worker_stations_v10 TO agiparel_app;


--
-- Name: TABLE sew_sim_credits_v9; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_sim_credits_v9 TO agiparel_app;


--
-- Name: TABLE sew_sim_cycle_inputs_v9; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_sim_cycle_inputs_v9 TO agiparel_app;


--
-- Name: TABLE sew_sim_cycles_v9; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_sim_cycles_v9 TO agiparel_app;


--
-- Name: COLUMN sew_sim_cycles_v9.state; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(state) ON TABLE public.sew_sim_cycles_v9 TO agiparel_app;


--
-- Name: COLUMN sew_sim_cycles_v9.active_since; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(active_since) ON TABLE public.sew_sim_cycles_v9 TO agiparel_app;


--
-- Name: COLUMN sew_sim_cycles_v9.active_ms; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(active_ms) ON TABLE public.sew_sim_cycles_v9 TO agiparel_app;


--
-- Name: COLUMN sew_sim_cycles_v9.paused_ms; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(paused_ms) ON TABLE public.sew_sim_cycles_v9 TO agiparel_app;


--
-- Name: COLUMN sew_sim_cycles_v9.paused_since; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(paused_since) ON TABLE public.sew_sim_cycles_v9 TO agiparel_app;


--
-- Name: COLUMN sew_sim_cycles_v9.finished_at; Type: ACL; Schema: public; Owner: -
--

GRANT UPDATE(finished_at) ON TABLE public.sew_sim_cycles_v9 TO agiparel_app;


--
-- Name: TABLE sew_sim_events_v9; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_sim_events_v9 TO agiparel_app;


--
-- Name: TABLE sew_sim_operations_v9; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_sim_operations_v9 TO agiparel_app;


--
-- Name: TABLE sew_sim_scans_v9; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.sew_sim_scans_v9 TO agiparel_app;


--
-- Name: TABLE store_issuances; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.store_issuances TO agiparel_app;


--
-- Name: TABLE store_pick_proposals_v7; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.store_pick_proposals_v7 TO agiparel_app;


--
-- Name: TABLE store_reconciliation_requests_v7; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.store_reconciliation_requests_v7 TO agiparel_app;


--
-- Name: TABLE store_roll_admission_proofs_v7; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.store_roll_admission_proofs_v7 TO agiparel_app;


--
-- Name: TABLE store_roll_admissions_v7; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.store_roll_admissions_v7 TO agiparel_app;


--
-- Name: TABLE store_roll_passport_events_v7; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.store_roll_passport_events_v7 TO agiparel_app;


--
-- Name: TABLE store_roll_rfid_bindings_v7; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.store_roll_rfid_bindings_v7 TO agiparel_app;


--
-- Name: TABLE store_rolls; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.store_rolls TO agiparel_app;


--
-- Name: TABLE store_trims_v13; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.store_trims_v13 TO agiparel_app;


--
-- Name: TABLE style_assembly_dag; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.style_assembly_dag TO agiparel_app;


--
-- Name: TABLE style_parts; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.style_parts TO agiparel_app;


--
-- Name: TABLE styles_master; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.styles_master TO agiparel_app;


--
-- Name: TABLE tenants; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.tenants TO agiparel_app;


--
-- Name: TABLE trim_issuances_v13; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.trim_issuances_v13 TO agiparel_app;


--
-- Name: TABLE trim_stock_admissions_v25; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.trim_stock_admissions_v25 TO agiparel_app;


--
-- Name: TABLE wip_exception_cases_v23; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.wip_exception_cases_v23 TO agiparel_app;


--
-- Name: TABLE wip_exception_closures_v23; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.wip_exception_closures_v23 TO agiparel_app;


--
-- Name: TABLE wip_gateway_scans_v20; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.wip_gateway_scans_v20 TO agiparel_app;


--
-- Name: TABLE wip_movements_v20; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.wip_movements_v20 TO agiparel_app;


--
-- Name: TABLE wip_piece_holds_v26; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.wip_piece_holds_v26 TO agiparel_app;


--
-- Name: TABLE wip_piece_reviews_v26; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT ON TABLE public.wip_piece_reviews_v26 TO agiparel_app;


--
-- Name: TABLE wip_transfer_routes_v20; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.wip_transfer_routes_v20 TO agiparel_app;


--
-- Name: TABLE work_orders; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.work_orders TO agiparel_app;


--
-- Name: TABLE workers; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.workers TO agiparel_app;


--
-- Name: TABLE workforce_chapter_actions_v17; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.workforce_chapter_actions_v17 TO agiparel_app;


--
-- Name: TABLE workforce_identity_grants_v15; Type: ACL; Schema: public; Owner: -
--

GRANT SELECT ON TABLE public.workforce_identity_grants_v15 TO agiparel_app;


--
-- Name: DEFAULT PRIVILEGES FOR SEQUENCES; Type: DEFAULT ACL; Schema: public; Owner: -
--

ALTER DEFAULT PRIVILEGES FOR ROLE agiparel_admin IN SCHEMA public GRANT SELECT,USAGE ON SEQUENCES TO agiparel_app;


--
-- Name: DEFAULT PRIVILEGES FOR TABLES; Type: DEFAULT ACL; Schema: public; Owner: -
--

ALTER DEFAULT PRIVILEGES FOR ROLE agiparel_admin IN SCHEMA public GRANT SELECT,INSERT,DELETE,UPDATE ON TABLES TO agiparel_app;


--
-- PostgreSQL database dump complete
--

\unrestrict ofxAhRyiDpoZ8iRglI0hFFSEaR8dxFWzFry44RcDEffpQl6KH9fk0OTklVvhxDA

--
-- PostgreSQL database dump
--

\restrict adulFLyrvMP8VKuprYZTE9rZHFtg6KeifnDeTYXfaGTnpVeItUAwjEgsBZzrlWC

-- Dumped from database version 16.15
-- Dumped by pg_dump version 16.15

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Data for Name: tenants; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: workers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: attendance_register; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: styles_master; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: bom_trim_requirements; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: bundle_registry; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: order_intakes_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_lay_drafts_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: documents; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: buyer_aql_plans_v32; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: buyer_aql_review_rules_v33; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: buyer_aql_reviews_v33; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_lay_positions_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_spread_reports_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_sewing_releases_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: gen_garments_v18; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: buyer_aql_sample_events_v32; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: document_sources_v65; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: buyer_order_handoffs_v65c1; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: carton_pack_manifests; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_bundle_drafts_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_bundle_scan_witnesses_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_consumption_reconciliations_v19; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_defect_codes_v19; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_rolls; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_lay_rolls; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_lay_snapshots_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: gen_bundles_v18; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: gen_cut_parts_v18; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_part_outcomes_v19; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_proof_consumptions_v19; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_reader_gateways_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_reader_events_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_recut_links_v19; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_reservations_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_issues_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cut_spread_issue_allocations_v8; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_container_seals_v34; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_carrier_receipts_v34; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_asn_drafts_v34; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_carton_specs_v22; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_buyer_spec_bindings_v36; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_carrier_authorizations_v34; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_gateways_v34; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_carton_scans_v34; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_external_signers_v35; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_external_assertions_v35; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dock_station_policies_v34; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: document_segments_v65; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: document_assertions_v65; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: document_jobs_v65; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: dynamic_work_orders; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: fabric_shrinkage_observations_v30; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_instrument_calibrations_v32; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: fabric_calibration_links_v32; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: fabric_shrinkage_specs_v31; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_document_candidates_v65b2; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_candidate_review_versions_v65b5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_document_submissions_v65b5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_approved_reference_versions_v65b5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_candidate_reviews_v65b2; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_proposed_register_v65b2; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_draft_decisions_v65b3; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_fact_events_v65b; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: factory_reference_versions_v65b4; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: floor_issues; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: gen_carriers_v18; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: gen_carrier_events_v18; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: gen_carrier_active_v18; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: hangtag_binds; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: hourly_line_metrics; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: intake_document_capture; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: intake_proposed_fact; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: intel_sim_snapshots_v12; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: intelligence_prod_readers_v13; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_vendors_v28; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_prod_gateways_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wip_gateway_scans_v20; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_outwards_v28; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_discrepancies_v29; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_discrepancy_resolutions_v31; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_inwards_v28; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_used_proofs_v28; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_used_scans_v28; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_vendor_keys_v30; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: jobwork_vendor_acks_v30; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: lines; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: machine_roster; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_ceo_principals_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_outbox_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_scenarios_v6; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_used_proofs_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_variances_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: material_write_scopes_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: needle_safety_events_v32; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: needle_safety_clearances_v33; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ocr_records; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: operation_master; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: order_intake_bom_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: order_intake_skus_v5; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: order_sku_matrix; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_carton_events_v22; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_manifest_events_v22; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_prod_assignments_v12; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_prod_routes_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_prod_completions_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_prod_group_events_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_prod_holds_v12; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_sim_cartons_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_sim_events_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_sim_units_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_sim_events_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_sim_buyer_bindings_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_sim_items_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_sim_manifests_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_sim_manifest_cartons_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pack_size_weights_v22; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: plant_hardware_assets; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: post_sewing_processes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_garment_defects_v21; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_garment_specs_v21; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_garment_events_v21; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_garment_measure_rules_v21; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_prod_assignments_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_prod_defect_catalog_v11; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: qc_sim_routes_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_piece_completions_v27; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_piece_used_scans_v27; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_prod_observations_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_prod_used_observations_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_prod_worker_stations_v10; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_sim_operations_v9; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_sim_cycles_v9; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_sim_credits_v9; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_sim_scans_v9; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_sim_cycle_inputs_v9; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sew_sim_events_v9; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_issuances; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_pick_proposals_v7; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_roll_rfid_bindings_v7; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_roll_passport_events_v7; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_reconciliation_requests_v7; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_roll_admissions_v7; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_roll_admission_proofs_v7; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: store_trims_v13; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: style_parts; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: style_assembly_dag; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: trim_stock_admissions_v25; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: trim_admission_proof_uses_v25; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: trim_issuances_v13; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: trim_issuance_proof_uses_v24; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wip_exception_cases_v23; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wip_exception_closures_v23; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wip_transfer_routes_v20; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wip_movements_v20; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wip_piece_holds_v26; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wip_piece_reviews_v26; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: workforce_browser_sessions_v16; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: workforce_chapter_actions_v17; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.workforce_chapter_actions_v17 VALUES (1, 'factory.read', 'READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (1, 'factory.manage', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (2, 'orders.read', 'READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (2, 'orders.manage', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (3, 'planning.read', 'READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (3, 'planning.approve', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.read', 'READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.move', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (5, 'cutting.read', 'READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (5, 'cutting.release', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.read', 'SIGNED_READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.complete', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.read', 'SIGNED_READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.inspect', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.repair', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.read', 'SIGNED_READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.stage', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (9, 'intelligence.read', 'READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (10, 'documents.read', 'READ');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (10, 'documents.manage', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.transfer', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.finish', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.release', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.pack', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.seal', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.manifest', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.dispatch_stage', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.exception.report', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.exception.resolve', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.trims.issue', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.trims.receive', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.trims.admit', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.trims.inspect', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.piece.hold', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.piece.review', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'sewing.piece.complete', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'jobwork.outward', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'jobwork.inward', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'jobwork.discrepancy.report', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'jobwork.vendor.ack.record', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.fabric.shrinkage.record', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (6, 'jobwork.discrepancy.resolve', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (4, 'stores.fabric.calibration.link', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.aql.inspect', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.needle.report', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.needle.clear', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (7, 'quality.aql.review', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.dock.seal', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.dock.carrier', 'SIGNED_WRITE');
INSERT INTO public.workforce_chapter_actions_v17 VALUES (8, 'packing.dock.asn.prepare', 'SIGNED_WRITE');


--
-- Data for Name: workforce_identity_grants_v15; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: workforce_oidc_requests_v16; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tenants; Type: TABLE DATA; Schema: s01_identity; Owner: -
--



--
-- Data for Name: plants; Type: TABLE DATA; Schema: s01_identity; Owner: -
--



--
-- Data for Name: roles; Type: TABLE DATA; Schema: s01_identity; Owner: -
--



--
-- Data for Name: role_permissions; Type: TABLE DATA; Schema: s01_identity; Owner: -
--



--
-- Data for Name: users; Type: TABLE DATA; Schema: s01_identity; Owner: -
--



--
-- Data for Name: user_role_grants; Type: TABLE DATA; Schema: s01_identity; Owner: -
--



--
-- Data for Name: registered_devices; Type: TABLE DATA; Schema: s02_device; Owner: -
--



--
-- Name: buyer_order_handoffs_v65c1_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.buyer_order_handoffs_v65c1_id_seq', 1, false);


--
-- Name: gen_carrier_events_v18_event_no_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.gen_carrier_events_v18_event_no_seq', 1, false);


--
-- PostgreSQL database dump complete
--

\unrestrict adulFLyrvMP8VKuprYZTE9rZHFtg6KeifnDeTYXfaGTnpVeItUAwjEgsBZzrlWC
