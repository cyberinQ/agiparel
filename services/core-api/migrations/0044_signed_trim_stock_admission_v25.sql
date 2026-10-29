-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.14B: approved-document, two-person signed new trim lot admission.
-- 0044 runs after 0043; never backfills or adjusts historical balances.
BEGIN;

INSERT INTO public.workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (4,'stores.trims.admit','SIGNED_WRITE'), (4,'stores.trims.inspect','SIGNED_WRITE')
 ON CONFLICT(permission) DO NOTHING;

CREATE UNIQUE INDEX documents_tenant_identity_v25 ON public.documents(tenant_id,id);
-- Legacy lots have unknown locations. Do not invent defaults for them.
ALTER TABLE public.store_trims_v13 ADD COLUMN bin_location varchar(128)
 CHECK(bin_location IS NULL OR length(btrim(bin_location)) BETWEEN 2 AND 128);
CREATE TABLE public.trim_stock_admissions_v25 (
 id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
 tenant_id varchar(64) NOT NULL REFERENCES public.tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 stock_id varchar(64) NOT NULL,
 trim_code varchar(64) NOT NULL,
 description varchar(255) NOT NULL,
 supplier_lot varchar(128) NOT NULL,
 color_code varchar(64) NOT NULL DEFAULT '',
 size_code varchar(32) NOT NULL DEFAULT '',
 uom varchar(32) NOT NULL CHECK(uom ~ '^[A-Z][A-Z0-9_]{0,31}$'),
 received_qty numeric(12,3) NOT NULL CHECK(received_qty>0),
 bin_location varchar(128) NOT NULL CHECK(length(btrim(bin_location)) BETWEEN 2 AND 128),
 source_document_id varchar(128) NOT NULL,
 source_sha256 char(64) NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 command_id varchar(80) NOT NULL CHECK(command_id ~ '^[A-Za-z0-9_-]{12,80}$'),
 manager_subject varchar(64) NOT NULL,
 inspector_subject varchar(64) NOT NULL,
 manager_issuer varchar(255) NOT NULL,
 inspector_issuer varchar(255) NOT NULL,
 manager_proof_id varchar(255) NOT NULL,
 inspector_proof_id varchar(255) NOT NULL,
 manager_assertion_sha256 char(64) NOT NULL CHECK(manager_assertion_sha256 ~ '^[0-9a-f]{64}$'),
 inspector_assertion_sha256 char(64) NOT NULL CHECK(inspector_assertion_sha256 ~ '^[0-9a-f]{64}$'),
 challenge_sha256 char(64) NOT NULL CHECK(challenge_sha256 ~ '^[0-9a-f]{64}$'),
 admitted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 authority varchar(64) NOT NULL DEFAULT 'SIGNED_ADMISSION_DOCUMENT_NOT_CALIBRATED_MEASUREMENT'
  CHECK(authority='SIGNED_ADMISSION_DOCUMENT_NOT_CALIBRATED_MEASUREMENT'),
 CHECK(manager_subject<>inspector_subject AND manager_proof_id<>inspector_proof_id),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,stock_id), UNIQUE(tenant_id,command_id),
 FOREIGN KEY(tenant_id,source_document_id) REFERENCES public.documents(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,stock_id) REFERENCES public.store_trims_v13(tenant_id,id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX trim_admissions_by_source_v25 ON public.trim_stock_admissions_v25(tenant_id,source_document_id);
ALTER TABLE public.trim_stock_admissions_v25 ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.trim_stock_admissions_v25 FORCE ROW LEVEL SECURITY;
CREATE POLICY trim_admission_tenant_v25 ON public.trim_stock_admissions_v25 FOR ALL TO agiparel_app
 USING(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON public.trim_stock_admissions_v25 FROM PUBLIC,agiparel_app;
GRANT SELECT,INSERT ON public.trim_stock_admissions_v25 TO agiparel_app;

CREATE TABLE public.trim_admission_proof_uses_v25 (
 tenant_id varchar(64) NOT NULL REFERENCES public.tenants(id) ON DELETE RESTRICT,
 proof_id varchar(255) NOT NULL,
 admission_id uuid NOT NULL REFERENCES public.trim_stock_admissions_v25(id) ON DELETE RESTRICT,
 role varchar(12) NOT NULL CHECK(role IN ('MANAGER','INSPECTOR')),
 PRIMARY KEY(tenant_id,proof_id)
);
ALTER TABLE public.trim_admission_proof_uses_v25 ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.trim_admission_proof_uses_v25 FORCE ROW LEVEL SECURITY;
CREATE POLICY trim_admission_proof_tenant_v25 ON public.trim_admission_proof_uses_v25 FOR SELECT TO agiparel_app
 USING(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON public.trim_admission_proof_uses_v25 FROM PUBLIC,agiparel_app;

-- Cross-ledger single-use assertions must remain unique even when issuance and
-- admission race concurrently on independent database tables. Lock both proof
-- IDs in deterministic order before checking the other ledger.
CREATE OR REPLACE FUNCTION public.trim_cross_ledger_proof_guard_v25() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
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
CREATE TRIGGER trim_admission_cross_ledger_guard_v25 BEFORE INSERT ON public.trim_stock_admissions_v25
 FOR EACH ROW EXECUTE FUNCTION public.trim_cross_ledger_proof_guard_v25();
CREATE TRIGGER trim_issuance_cross_ledger_guard_v25 BEFORE INSERT ON public.trim_issuances_v13
 FOR EACH ROW EXECUTE FUNCTION public.trim_cross_ledger_proof_guard_v25();

-- Application may create a new stock lot only by inserting a signed admission
-- that passes this atomic trigger; direct INSERT/UPDATE remains revoked (0043).
CREATE OR REPLACE FUNCTION public.guard_trim_admission_v25() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
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
CREATE TRIGGER trim_admission_stock_v25 AFTER INSERT ON public.trim_stock_admissions_v25
 FOR EACH ROW EXECUTE FUNCTION public.guard_trim_admission_v25();
CREATE TRIGGER trim_admission_immutable_v25 BEFORE UPDATE OR DELETE ON public.trim_stock_admissions_v25
 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();
CREATE TRIGGER trim_admission_proof_immutable_v25 BEFORE UPDATE OR DELETE ON public.trim_admission_proof_uses_v25
 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();
REVOKE UPDATE,DELETE,TRUNCATE ON public.trim_stock_admissions_v25 FROM agiparel_app;
-- Reaffirm that migration 0043's stock write restrictions remain in force.
REVOKE INSERT,UPDATE,DELETE,TRUNCATE ON public.store_trims_v13 FROM agiparel_app;
COMMIT;
