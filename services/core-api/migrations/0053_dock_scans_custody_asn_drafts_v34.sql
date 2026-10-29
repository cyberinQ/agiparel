-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.20B-D: independently signed dock-gateway carton scan, supervisor container
-- seal, onboarded-carrier human receipt, and internal ASN draft. Neither carrier
-- API delivery nor legal buyer acceptance nor shipment credit is inferred.
BEGIN;
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (8,'packing.dock.seal','SIGNED_WRITE'),
 (8,'packing.dock.carrier','SIGNED_WRITE'),
 (8,'packing.dock.asn.prepare','SIGNED_WRITE')
 ON CONFLICT(permission) DO NOTHING;

-- These enrollment/authority records are provisioner-only; the app can read,
-- never create or modify them. Every physical reader has a distinct public key.
CREATE TABLE dock_gateways_v34 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 gateway_id varchar(64) NOT NULL CHECK(gateway_id ~ '^[A-Za-z0-9_-]{3,64}$'),
 public_key bytea NOT NULL CHECK(octet_length(public_key)=32),
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 PRIMARY KEY(tenant_id,gateway_id)
);
CREATE TABLE dock_carrier_authorizations_v34 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 carrier_code varchar(64) NOT NULL CHECK(carrier_code ~ '^[A-Za-z0-9_-]{3,64}$'),
 idp_issuer varchar(512) NOT NULL CHECK(idp_issuer LIKE 'https://%'),
 idp_subject varchar(128) NOT NULL CHECK(length(btrim(idp_subject))>0),
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 PRIMARY KEY(tenant_id,plant_id,carrier_code,idp_issuer,idp_subject)
);
CREATE TABLE dock_station_policies_v34 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 max_scan_age_minutes integer NOT NULL CHECK(max_scan_age_minutes BETWEEN 5 AND 1440),
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL,
 PRIMARY KEY(tenant_id,plant_id)
);

-- Authoritative *current* preflight. Always recomputed, including AFTER a
-- historical positive seal or carrier receipt (new safety holds reblock).
CREATE FUNCTION dock_manifest_safe_v34(t text,p text,m uuid) RETURNS boolean
LANGUAGE plpgsql VOLATILE AS $$
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

-- Authenticated by Ed25519 at HTTP ingress; SQL independently checks that the
-- gateway is enrolled, carton belongs to staged manifest, and code matches.
CREATE TABLE dock_carton_scans_v34 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 manifest_id uuid NOT NULL, carton_id uuid NOT NULL,
 gateway_id varchar(64) NOT NULL,
 event_id varchar(80) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 carton_code varchar(80) NOT NULL,
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 event_sha256 char(64) NOT NULL CHECK(event_sha256 ~ '^[0-9a-f]{64}$'),
 gateway_key_sha256 char(64) NOT NULL CHECK(gateway_key_sha256 ~ '^[0-9a-f]{64}$'),
 signature_sha256 char(64) NOT NULL CHECK(signature_sha256 ~ '^[0-9a-f]{64}$'),
 authority text NOT NULL DEFAULT 'ENROLLED_GATEWAY_SCAN_NOT_INDEPENDENT_RF_CERTIFICATION'
  CHECK(authority='ENROLLED_GATEWAY_SCAN_NOT_INDEPENDENT_RF_CERTIFICATION'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,gateway_id,event_id), UNIQUE(tenant_id,manifest_id,carton_id),
 FOREIGN KEY(tenant_id,gateway_id) REFERENCES dock_gateways_v34(tenant_id,gateway_id)
);
-- carton_id is the logical carton identity, validated against OPEN/ADD_CARTON.
CREATE FUNCTION guard_dock_scan_v34() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER dock_scan_guard_v34 BEFORE INSERT ON dock_carton_scans_v34
 FOR EACH ROW EXECUTE FUNCTION guard_dock_scan_v34();

CREATE TABLE dock_container_seals_v34 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL, manifest_id uuid NOT NULL,
 container_ref varchar(80) NOT NULL CHECK(length(btrim(container_ref))>=3),
 seal_code varchar(80) NOT NULL CHECK(length(btrim(seal_code))>=3),
 scanned_cartons integer NOT NULL CHECK(scanned_cartons>0),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 actor_subject varchar(128) NOT NULL, proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'SUPERVISOR_SEALED_CONTAINER_NOT_CARRIER_HANDOVER'
  CHECK(authority='SUPERVISOR_SEALED_CONTAINER_NOT_CARRIER_HANDOVER'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,manifest_id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,proof_id), UNIQUE(tenant_id,plant_id,container_ref,seal_code)
);
CREATE FUNCTION guard_dock_seal_v34() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER dock_seal_guard_v34 BEFORE INSERT ON dock_container_seals_v34
 FOR EACH ROW EXECUTE FUNCTION guard_dock_seal_v34();

-- This is signed onboarded-carrier personnel evidence; external carrier
-- system acknowledgement and shipping liability remain separate integrations.
CREATE TABLE dock_carrier_receipts_v34 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL, manifest_id uuid NOT NULL,
 seal_id uuid NOT NULL, carrier_code varchar(64) NOT NULL,
 receipt_reference varchar(128) NOT NULL CHECK(length(btrim(receipt_reference))>=6),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 actor_issuer varchar(512) NOT NULL, actor_subject varchar(128) NOT NULL,
 proof_id varchar(255) NOT NULL, assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'ONBOARDED_CARRIER_PERSONNEL_ATTESTATION_NOT_EXTERNAL_API_RECEIPT'
  CHECK(authority='ONBOARDED_CARRIER_PERSONNEL_ATTESTATION_NOT_EXTERNAL_API_RECEIPT'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,manifest_id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,proof_id), UNIQUE(tenant_id,carrier_code,receipt_reference),
 FOREIGN KEY(tenant_id,seal_id) REFERENCES dock_container_seals_v34(tenant_id,id)
);
CREATE FUNCTION guard_dock_carrier_v34() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER dock_carrier_guard_v34 BEFORE INSERT ON dock_carrier_receipts_v34
 FOR EACH ROW EXECUTE FUNCTION guard_dock_carrier_v34();

CREATE TABLE dock_asn_drafts_v34 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(), plant_id varchar(64) NOT NULL, manifest_id uuid NOT NULL,
 carrier_receipt_id uuid NOT NULL,
 asn_reference varchar(128) NOT NULL CHECK(length(btrim(asn_reference))>=6),
 payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 note text NOT NULL CHECK(length(btrim(note)) BETWEEN 20 AND 800),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 command_sha256 char(64) NOT NULL CHECK(command_sha256 ~ '^[0-9a-f]{64}$'),
 actor_subject varchar(128) NOT NULL, proof_id varchar(255) NOT NULL,
 assertion_sha256 char(64) NOT NULL CHECK(assertion_sha256 ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority text NOT NULL DEFAULT 'INTERNAL_ASN_DRAFT_NOT_TRANSMITTED_OR_ACKNOWLEDGED'
  CHECK(authority='INTERNAL_ASN_DRAFT_NOT_TRANSMITTED_OR_ACKNOWLEDGED'),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,manifest_id), UNIQUE(tenant_id,command_id),
 UNIQUE(tenant_id,proof_id), UNIQUE(tenant_id,asn_reference),
 FOREIGN KEY(tenant_id,carrier_receipt_id) REFERENCES dock_carrier_receipts_v34(tenant_id,id)
);
CREATE FUNCTION guard_dock_asn_v34() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER dock_asn_guard_v34 BEFORE INSERT ON dock_asn_drafts_v34
 FOR EACH ROW EXECUTE FUNCTION guard_dock_asn_v34();

-- Current authorization can be revoked after a historical signature. The
-- dashboard must not present revoked devices or carrier signers as verified.
CREATE FUNCTION dock_evidence_current_v34(t text,p text,m uuid) RETURNS boolean
LANGUAGE sql VOLATILE AS $$
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

-- One signed assertion must never authorize two different events (v32/v33/v34).
CREATE FUNCTION guard_dock_proof_v34() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER dock_seal_proof_v34 BEFORE INSERT ON dock_container_seals_v34 FOR EACH ROW EXECUTE FUNCTION guard_dock_proof_v34();
CREATE TRIGGER dock_carrier_proof_v34 BEFORE INSERT ON dock_carrier_receipts_v34 FOR EACH ROW EXECUTE FUNCTION guard_dock_proof_v34();
CREATE TRIGGER dock_asn_proof_v34 BEFORE INSERT ON dock_asn_drafts_v34 FOR EACH ROW EXECUTE FUNCTION guard_dock_proof_v34();

-- Retrofit BOTH earlier proof guards. Otherwise an assertion could first sign a
-- v34 record, then be replayed into v32/v33 (one-way checking is insufficient).
CREATE OR REPLACE FUNCTION guard_factory_proof_v32() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE OR REPLACE FUNCTION guard_factory_review_proof_v33() RETURNS trigger LANGUAGE plpgsql AS $$
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

DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['dock_gateways_v34','dock_carrier_authorizations_v34','dock_station_policies_v34',
 'dock_carton_scans_v34','dock_container_seals_v34','dock_carrier_receipts_v34','dock_asn_drafts_v34'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY dock_tenant_v34 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))',t,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',t);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',t);
  IF t IN ('dock_carton_scans_v34','dock_container_seals_v34','dock_carrier_receipts_v34','dock_asn_drafts_v34') THEN
   EXECUTE format('CREATE TRIGGER dock_immutable_v34 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',t);
  END IF;
 END LOOP;
END $$;
GRANT INSERT ON dock_carton_scans_v34,dock_container_seals_v34,dock_carrier_receipts_v34,dock_asn_drafts_v34 TO agiparel_app;
COMMIT;
