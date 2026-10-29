-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.23 v36: provisioner-bound buyer/manifest authority, exact carton set,
-- live diagnostics. Neither authenticated buyer review nor dock evidence posts shipments.
BEGIN;

-- Buyer authorization is tied to the immutable packing spec chosen by the
-- manifest OPEN event. Enrolment is privileged and defaults to disabled.
CREATE TABLE dock_buyer_spec_bindings_v36 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 spec_id uuid NOT NULL,
 buyer_code varchar(64) NOT NULL CHECK (buyer_code ~ '^[A-Za-z0-9_-]{3,64}$'),
 enabled boolean NOT NULL DEFAULT false,
 contract_reference varchar(128) NOT NULL CHECK(length(btrim(contract_reference)) BETWEEN 6 AND 128),
 approved_by varchar(128) NOT NULL CHECK(length(btrim(approved_by))>0),
 approved_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,plant_id,spec_id),
 FOREIGN KEY(tenant_id,spec_id) REFERENCES pack_carton_specs_v22(tenant_id,id) ON DELETE RESTRICT
);
CREATE FUNCTION dock_buyer_binding_guard_v36() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER dock_buyer_binding_insert_guard_v36 BEFORE INSERT OR UPDATE
 ON dock_buyer_spec_bindings_v36 FOR EACH ROW EXECUTE FUNCTION dock_buyer_binding_guard_v36();

-- Scope must be checked live: disabling a binding/revoking a buyer key must
-- invalidate a formerly positive assertion, without rewriting its history.
CREATE FUNCTION dock_buyer_scope_current_v36(t text,p text,m uuid) RETURNS boolean
LANGUAGE sql VOLATILE AS $$
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

-- Exactly one staged manifest, no extra/missing carton, matching device scan,
-- and precisely one carton OPEN/SEAL with at least one garment ADD. Conserves
-- THIS manifest's carton identities, not global raw inventory or accounting.
CREATE OR REPLACE FUNCTION dock_carton_reconciled_v35(t text,p text,m uuid) RETURNS boolean
LANGUAGE sql VOLATILE AS $$
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

-- Replace rather than layering a permissive OR onto earlier v35 predicate.
CREATE OR REPLACE FUNCTION dock_external_current_v35(t text,p text,m uuid) RETURNS boolean
LANGUAGE sql VOLATILE AS $$
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

-- The database guards INSERT as well as the live verification response:
-- a valid buyer key for a different packing spec is insufficient.
CREATE OR REPLACE FUNCTION guard_dock_external_v35() RETURNS trigger LANGUAGE plpgsql AS $$
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

-- A named, read-only exception list; no authority to close/waive blockers.
CREATE FUNCTION dock_diagnostics_v36(t text,p text,m uuid) RETURNS jsonb
LANGUAGE sql VOLATILE AS $$
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

ALTER TABLE dock_buyer_spec_bindings_v36 ENABLE ROW LEVEL SECURITY;
ALTER TABLE dock_buyer_spec_bindings_v36 FORCE ROW LEVEL SECURITY;
CREATE POLICY dock_buyer_tenant_v36 ON dock_buyer_spec_bindings_v36 FOR ALL TO agiparel_app
 USING (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK (tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON dock_buyer_spec_bindings_v36 FROM PUBLIC,agiparel_app;
GRANT SELECT ON dock_buyer_spec_bindings_v36 TO agiparel_app;
COMMIT;
