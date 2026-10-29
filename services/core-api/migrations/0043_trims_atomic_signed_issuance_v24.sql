-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- F3.14: forward-only trim stock conservation and independently signed issuance.
-- Existing legacy issue rows are preserved; do not replay/deduct them blindly.
BEGIN;

DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.store_trims_v13 WHERE remaining_qty > received_qty) THEN
  RAISE EXCEPTION 'ERR_F314_HISTORICAL_TRIM_BALANCE_EXCEEDS_RECEIPT';
 END IF;
END $$;

ALTER TABLE public.store_trims_v13 FORCE ROW LEVEL SECURITY;
ALTER TABLE public.trim_issuances_v13 FORCE ROW LEVEL SECURITY;
ALTER TABLE public.store_trims_v13
 ADD CONSTRAINT store_trims_conservation_v24 CHECK (remaining_qty <= received_qty);

ALTER TABLE public.trim_issuances_v13
 ADD COLUMN plant_id varchar(64),
 ADD COLUMN command_id varchar(80),
 ADD COLUMN issuer_proof_id varchar(255),
 ADD COLUMN recipient_proof_id varchar(255),
 ADD COLUMN challenge_sha256 char(64),
 ADD COLUMN issuer_assertion_sha256 char(64),
 ADD COLUMN recipient_assertion_sha256 char(64);
-- NOT NULL cannot be imposed on historical rows. Every NEW row is checked by trigger.
CREATE UNIQUE INDEX trim_issuances_command_v24 ON public.trim_issuances_v13(tenant_id,command_id) WHERE command_id IS NOT NULL;
CREATE INDEX trim_issuances_stock_recent_v24 ON public.trim_issuances_v13(tenant_id,trim_stock_id,issued_at DESC);

CREATE TABLE public.trim_issuance_proof_uses_v24 (
 tenant_id varchar(64) NOT NULL REFERENCES public.tenants(id) ON DELETE RESTRICT,
 proof_id varchar(255) NOT NULL,
 issuance_id uuid NOT NULL REFERENCES public.trim_issuances_v13(id) ON DELETE RESTRICT,
 witness_role varchar(16) NOT NULL CHECK(witness_role IN ('ISSUER','RECIPIENT')),
 PRIMARY KEY(tenant_id,proof_id)
);
ALTER TABLE public.trim_issuance_proof_uses_v24 ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.trim_issuance_proof_uses_v24 FORCE ROW LEVEL SECURITY;
CREATE POLICY trim_proof_tenant_v24 ON public.trim_issuance_proof_uses_v24 FOR SELECT TO agiparel_app
 USING(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON public.trim_issuance_proof_uses_v24 FROM PUBLIC,agiparel_app;
-- The privileged migration owner alone writes consumed-proof evidence by trigger.

CREATE OR REPLACE FUNCTION public.guard_trim_issuance_v24() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
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
END $$;

-- AFTER INSERT ensures the immutable issuance exists before proof-use FKs are inserted.
-- Returning NULL from an AFTER ROW trigger leaves the INSERT row intact.
DROP TRIGGER IF EXISTS trim_issuance_conservation_v24 ON public.trim_issuances_v13;
CREATE TRIGGER trim_issuance_conservation_v24 AFTER INSERT ON public.trim_issuances_v13
 FOR EACH ROW EXECUTE FUNCTION public.guard_trim_issuance_v24();

CREATE OR REPLACE FUNCTION public.deny_trim_ledger_mutation_v24() RETURNS trigger
 LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'ERR_F314_APPEND_ONLY'; END $$;
CREATE TRIGGER trim_issuance_immutable_v24 BEFORE UPDATE OR DELETE ON public.trim_issuances_v13
 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();
CREATE TRIGGER trim_proof_immutable_v24 BEFORE UPDATE OR DELETE ON public.trim_issuance_proof_uses_v24
 FOR EACH ROW EXECUTE FUNCTION public.deny_trim_ledger_mutation_v24();

REVOKE UPDATE, DELETE, TRUNCATE ON public.trim_issuances_v13 FROM agiparel_app;
-- No unsigned stock receipts or direct balance edits via application DB role.
-- New stock admission requires a separately authorized, audited receipt workflow.
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.store_trims_v13 FROM agiparel_app;
INSERT INTO public.workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (4,'stores.trims.issue','SIGNED_WRITE'),(4,'stores.trims.receive','SIGNED_WRITE')
 ON CONFLICT(permission) DO NOTHING;
COMMIT;
