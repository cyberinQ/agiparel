-- Author: Sunil Jain, CyberinQ LLC, 2026-10-26.
-- Requires 0005. Applied by privileged migrations role; runtime role must NOT
-- own/provision lease or CEO identity tables.
BEGIN;

CREATE TABLE IF NOT EXISTS material_write_scopes_v5 (
 tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
 order_id VARCHAR(64) NOT NULL,
 plant_id VARCHAR(64) NOT NULL,
 mode TEXT NOT NULL CHECK (mode IN ('M0','M1','M2','M3','M4')),
 lease_holder TEXT NOT NULL,
 lease_revision BIGINT NOT NULL CHECK (lease_revision>0),
 legacy_writer_fenced BOOLEAN NOT NULL DEFAULT FALSE,
 approved_by TEXT NOT NULL,
 approved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 lease_expires_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(tenant_id,order_id)
);

CREATE TABLE IF NOT EXISTS material_ceo_principals_v5 (
 tenant_id VARCHAR(64) PRIMARY KEY REFERENCES tenants(id),
 subject TEXT NOT NULL,
 enrolled_by TEXT NOT NULL,
 enrolled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 active BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS material_variances_v5 (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
 tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
 reservation_id UUID NOT NULL,
 plant_id VARCHAR(64) NOT NULL,
 roll_id VARCHAR(64) NOT NULL,
 order_id VARCHAR(64) NOT NULL,
 cut_no INTEGER NOT NULL CHECK(cut_no>0),
 requested_additional_mm BIGINT NOT NULL CHECK(requested_additional_mm>0),
 actual_story TEXT NOT NULL CHECK(length(trim(actual_story))>=20),
 reason_code TEXT NOT NULL,
 requested_by TEXT NOT NULL,
 request_key VARCHAR(128) NOT NULL,
 challenge_hash CHAR(64) NOT NULL DEFAULT repeat('0',64),
 status TEXT NOT NULL DEFAULT 'PENDING_CEO' CHECK(status IN ('PENDING_CEO','APPROVED','REJECTED')),
 ceo_subject TEXT,
 ceo_assertion_hash CHAR(64),
 ceo_proof_id TEXT,
 approved_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,request_key),
 FOREIGN KEY(tenant_id,reservation_id) REFERENCES material_reservations_v5(tenant_id,id),
 FOREIGN KEY(tenant_id,roll_id) REFERENCES store_rolls(tenant_id,id),
 CHECK ((status='APPROVED' AND ceo_subject IS NOT NULL AND ceo_assertion_hash IS NOT NULL AND ceo_proof_id IS NOT NULL AND approved_at IS NOT NULL) OR status<>'APPROVED')
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_material_ceo_proof_v5
 ON material_variances_v5(tenant_id,ceo_proof_id) WHERE ceo_proof_id IS NOT NULL;

DO $$ DECLARE t TEXT; BEGIN
 FOREACH t IN ARRAY ARRAY['material_write_scopes_v5','material_ceo_principals_v5','material_variances_v5'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_v5 ON %I',t);
  EXECUTE format('CREATE POLICY tenant_isolation_v5 ON %I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id = NULLIF(current_setting(''app.current_tenant_id'',true),''''))',t);
 END LOOP;
END $$;
-- 0003 installs broad ALTER DEFAULT PRIVILEGES; REVOKE the implicit
-- write grants before handing the application its *read-only* provisioning registry.
REVOKE ALL ON material_write_scopes_v5,material_ceo_principals_v5 FROM PUBLIC,agiparel_app;
GRANT SELECT ON material_write_scopes_v5,material_ceo_principals_v5 TO agiparel_app;
GRANT SELECT,INSERT,UPDATE ON material_variances_v5 TO agiparel_app;
-- Prevent changes to the request body after insertion or to approved decisions.
CREATE OR REPLACE FUNCTION protect_material_variance_v5() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
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
DROP TRIGGER IF EXISTS material_variance_immutability_v5 ON material_variances_v5;
CREATE TRIGGER material_variance_immutability_v5 BEFORE UPDATE ON material_variances_v5
 FOR EACH ROW EXECUTE FUNCTION protect_material_variance_v5();
-- No INSERT/UPDATE/DELETE on scope/CEO registry granted to app role.

CREATE OR REPLACE FUNCTION public.material_scope_read_locked_v5(
    p_tenant VARCHAR,
    p_order VARCHAR,
    p_plant VARCHAR,
    p_holder TEXT
)
RETURNS TABLE(mode TEXT, legacy_writer_fenced BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
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

REVOKE ALL ON FUNCTION public.material_scope_read_locked_v5(
    VARCHAR,VARCHAR,VARCHAR,TEXT
) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION public.material_scope_read_locked_v5(
    VARCHAR,VARCHAR,VARCHAR,TEXT
) TO agiparel_app;

COMMIT;
