-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- V35: externally signed carrier/buyer assertions with current trust-root checks.
-- Not transport delivery, legal buyer acceptance, shipment posting, or stock credit.
BEGIN;

-- Provisioner-only trust anchors. Never seeded or writable by agiparel_app.
CREATE TABLE dock_external_signers_v35 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 signer_id varchar(64) NOT NULL CHECK(signer_id ~ '^[A-Za-z0-9_-]{3,64}$'),
 party_kind text NOT NULL CHECK(party_kind IN ('CARRIER','BUYER')),
 party_code varchar(64) NOT NULL CHECK(party_code ~ '^[A-Za-z0-9_-]{3,64}$'),
 public_key bytea NOT NULL CHECK(octet_length(public_key)=32),
 enabled boolean NOT NULL DEFAULT false,
 approved_by varchar(128) NOT NULL CHECK(length(btrim(approved_by))>0),
 PRIMARY KEY (tenant_id,plant_id,signer_id),
 UNIQUE (tenant_id,plant_id,party_kind,public_key)
);

CREATE TABLE dock_external_assertions_v35 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 id uuid NOT NULL DEFAULT uuid_generate_v4(),
 plant_id varchar(64) NOT NULL,
 manifest_id uuid NOT NULL,
 asn_draft_id uuid NOT NULL,
 signer_id varchar(64) NOT NULL,
 party_kind text NOT NULL CHECK(party_kind IN ('CARRIER','BUYER')),
 party_code varchar(64) NOT NULL,
 assertion_kind text NOT NULL CHECK(assertion_kind IN ('CARRIER_ASN_ACK','BUYER_LOT_REVIEW')),
 decision text NOT NULL CHECK(decision IN ('ACKNOWLEDGED','REVIEWED_ACCEPT','REVIEWED_REJECT')),
 event_id varchar(80) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 external_reference varchar(128) NOT NULL CHECK(length(btrim(external_reference)) BETWEEN 6 AND 128),
 asn_reference varchar(128) NOT NULL,
 payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 issued_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 event_sha256 char(64) NOT NULL CHECK(event_sha256 ~ '^[0-9a-f]{64}$'),
 signer_key_sha256 char(64) NOT NULL CHECK(signer_key_sha256 ~ '^[0-9a-f]{64}$'),
 signature_sha256 char(64) NOT NULL CHECK(signature_sha256 ~ '^[0-9a-f]{64}$'),
 authority text NOT NULL DEFAULT 'EXTERNAL_SIGNED_ASSERTION_NOT_INDEPENDENT_DELIVERY_OR_LEGAL_ACCEPTANCE'
  CHECK(authority='EXTERNAL_SIGNED_ASSERTION_NOT_INDEPENDENT_DELIVERY_OR_LEGAL_ACCEPTANCE'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,plant_id,signer_id,event_id),
 UNIQUE(tenant_id,plant_id,manifest_id,assertion_kind),
 FOREIGN KEY(tenant_id,asn_draft_id) REFERENCES dock_asn_drafts_v34(tenant_id,id),
 FOREIGN KEY(tenant_id,plant_id,signer_id) REFERENCES dock_external_signers_v35(tenant_id,plant_id,signer_id),
 CHECK ((assertion_kind='CARRIER_ASN_ACK' AND party_kind='CARRIER' AND decision='ACKNOWLEDGED') OR
        (assertion_kind='BUYER_LOT_REVIEW' AND party_kind='BUYER' AND decision IN ('REVIEWED_ACCEPT','REVIEWED_REJECT')))
);
CREATE INDEX dock_external_assertions_manifest_v35 ON dock_external_assertions_v35(tenant_id,plant_id,manifest_id);

-- SQL independently rejects cross-tenant/plant manifests, stale signatures,
-- changing ASN payloads and unapproved counterparties. It does NOT validate
-- Ed25519: that happens at ingress before INSERT and is bound by event hash.
CREATE FUNCTION guard_dock_external_v35() RETURNS trigger LANGUAGE plpgsql AS $$
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
 END IF;
 NEW.received_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE TRIGGER dock_external_guard_v35 BEFORE INSERT ON dock_external_assertions_v35
 FOR EACH ROW EXECUTE FUNCTION guard_dock_external_v35();
CREATE TRIGGER dock_external_immutable_v35 BEFORE UPDATE OR DELETE ON dock_external_assertions_v35
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();

-- Conservative manifest/carton-only conservation. This does not prove fabric,
-- subcontract WIP, cost accounting or inventory posting conservation.
CREATE FUNCTION dock_carton_reconciled_v35(t text,p text,m uuid) RETURNS boolean
LANGUAGE sql VOLATILE AS $$
 SELECT EXISTS(SELECT 1 FROM pack_manifest_events_v22 x
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='STAGE')
 AND EXISTS(SELECT 1 FROM pack_manifest_events_v22 x
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON')
 AND NOT EXISTS(
  SELECT 1 FROM pack_manifest_events_v22 x
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m AND x.action='ADD_CARTON'
   AND (
    (SELECT count(*) FROM pack_carton_events_v22 c WHERE c.tenant_id=t AND c.plant_id=p
      AND c.carton_id=x.carton_id AND c.action='OPEN')<>1
    OR (SELECT count(*) FROM pack_carton_events_v22 c WHERE c.tenant_id=t AND c.plant_id=p
      AND c.carton_id=x.carton_id AND c.action='SEAL')<>1
    OR (SELECT count(*) FROM pack_carton_events_v22 c WHERE c.tenant_id=t AND c.plant_id=p
      AND c.carton_id=x.carton_id AND c.action='ADD')<1
    OR EXISTS(
      SELECT 1 FROM pack_carton_events_v22 c
      WHERE c.tenant_id=t AND c.plant_id=p AND c.carton_id=x.carton_id AND c.action='ADD'
       AND (SELECT q.action FROM qc_garment_events_v21 q WHERE q.tenant_id=c.tenant_id
          AND q.plant_id=c.plant_id AND q.garment_id=c.garment_id
          ORDER BY q.event_no DESC LIMIT 1) IS DISTINCT FROM 'QUALITY_RELEASE'
    )
  ))
 AND (SELECT count(*) FROM dock_carton_scans_v34 x JOIN dock_gateways_v34 g
   ON g.tenant_id=x.tenant_id AND g.plant_id=x.plant_id AND g.gateway_id=x.gateway_id
   AND g.enabled AND encode(digest(g.public_key,'sha256'),'hex')=x.gateway_key_sha256
   WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m)
   = (SELECT count(*) FROM pack_manifest_events_v22 x WHERE x.tenant_id=t AND x.plant_id=p
      AND x.manifest_id=m AND x.action='ADD_CARTON')
$$;

-- Current evidence is recomputed at every query. Revoked keys or new safety
-- holds negate historical assertions. A buyer REJECT never becomes acceptance.
CREATE FUNCTION dock_external_current_v35(t text,p text,m uuid) RETURNS boolean
LANGUAGE sql VOLATILE AS $$
 SELECT dock_evidence_current_v34(t,p,m) AND dock_carton_reconciled_v35(t,p,m)
 AND EXISTS(
  SELECT 1 FROM dock_external_assertions_v35 x JOIN dock_external_signers_v35 s
   ON s.tenant_id=x.tenant_id AND s.plant_id=x.plant_id AND s.signer_id=x.signer_id
   AND s.enabled AND s.party_kind='CARRIER' AND s.party_code=x.party_code
   AND encode(digest(s.public_key,'sha256'),'hex')=x.signer_key_sha256
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m
   AND x.assertion_kind='CARRIER_ASN_ACK' AND x.decision='ACKNOWLEDGED')
 AND EXISTS(
  SELECT 1 FROM dock_external_assertions_v35 x JOIN dock_external_signers_v35 s
   ON s.tenant_id=x.tenant_id AND s.plant_id=x.plant_id AND s.signer_id=x.signer_id
   AND s.enabled AND s.party_kind='BUYER' AND s.party_code=x.party_code
   AND encode(digest(s.public_key,'sha256'),'hex')=x.signer_key_sha256
  WHERE x.tenant_id=t AND x.plant_id=p AND x.manifest_id=m
   AND x.assertion_kind='BUYER_LOT_REVIEW' AND x.decision='REVIEWED_ACCEPT')
$$;

DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['dock_external_signers_v35','dock_external_assertions_v35'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY dock_tenant_v35 ON public.%I FOR ALL TO agiparel_app USING (tenant_id=NULLIF(current_setting(%L,true),%L)) WITH CHECK (tenant_id=NULLIF(current_setting(%L,true),%L))',t,'app.current_tenant_id','','app.current_tenant_id','');
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',t);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',t);
 END LOOP;
END $$;
GRANT INSERT ON dock_external_assertions_v35 TO agiparel_app;
COMMIT;
