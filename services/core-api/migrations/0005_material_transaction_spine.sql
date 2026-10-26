-- Author: Sunil Jain, 2026-10-26 — Persist atomic material reservations and independently witnessed issuances.
-- Apply AFTER baseline 0001..0004 using the established migration runner.
-- Existing rolls default to UNVERIFIED; physical inspection must promote them to PASS.
BEGIN;

ALTER TABLE store_rolls
    ALTER COLUMN original_length_m TYPE NUMERIC(14,3),
    ALTER COLUMN remaining_length_m TYPE NUMERIC(14,3);
ALTER TABLE store_rolls ADD COLUMN IF NOT EXISTS inspection_status TEXT NOT NULL DEFAULT 'UNVERIFIED';
ALTER TABLE store_rolls ADD CONSTRAINT store_rolls_inspection_state_v5
    CHECK (inspection_status IN ('UNVERIFIED','PASS','HOLD','REJECTED'));
CREATE UNIQUE INDEX IF NOT EXISTS store_rolls_tenant_identity_v5 ON store_rolls(tenant_id,id);

CREATE TABLE IF NOT EXISTS material_reservations_v5 (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    roll_id VARCHAR(64) NOT NULL,
    order_id VARCHAR(64) NOT NULL,
    cut_no INTEGER NOT NULL CHECK (cut_no > 0),
    reserved_mm BIGINT NOT NULL CHECK (reserved_mm > 0),
    issued_mm BIGINT NOT NULL DEFAULT 0 CHECK (issued_mm >= 0),
    released_mm BIGINT NOT NULL DEFAULT 0 CHECK (released_mm >= 0),
    request_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (issued_mm + released_mm <= reserved_mm),
    UNIQUE (tenant_id,id),
    UNIQUE (tenant_id,request_key),
    FOREIGN KEY (tenant_id,roll_id) REFERENCES store_rolls(tenant_id,id)
);
CREATE INDEX IF NOT EXISTS ix_material_reservation_roll_v5
    ON material_reservations_v5(tenant_id,roll_id);

CREATE TABLE IF NOT EXISTS material_issues_v5 (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    reservation_id UUID NOT NULL REFERENCES material_reservations_v5(id),
    roll_id VARCHAR(64) NOT NULL,
    order_id VARCHAR(64) NOT NULL,
    cut_no INTEGER NOT NULL CHECK (cut_no > 0),
    length_mm BIGINT NOT NULL CHECK (length_mm > 0),
    command_key VARCHAR(128) NOT NULL,
    issuer_subject VARCHAR(128) NOT NULL,
    recipient_subject VARCHAR(128) NOT NULL,
    issuer_proof_id VARCHAR(255) NOT NULL,
    recipient_proof_id VARCHAR(255) NOT NULL,
    issuer_assertion_hash CHAR(64) NOT NULL,
    recipient_assertion_hash CHAR(64) NOT NULL,
    signed_challenge_hash CHAR(64) NOT NULL,
    committed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (issuer_subject <> recipient_subject),
    CHECK (issuer_proof_id <> recipient_proof_id),
    UNIQUE(tenant_id,command_key),
    UNIQUE(tenant_id,issuer_proof_id),
    UNIQUE(tenant_id,recipient_proof_id),
    FOREIGN KEY (tenant_id,reservation_id) REFERENCES material_reservations_v5(tenant_id,id),
    FOREIGN KEY (tenant_id,roll_id) REFERENCES store_rolls(tenant_id,id)
);
CREATE INDEX IF NOT EXISTS ix_material_issue_reservation_v5
    ON material_issues_v5(tenant_id,reservation_id);

-- Prevent a verified proof being reused across the issuer and recipient roles
-- (two independent UNIQUE columns cannot enforce cross-column uniqueness).
CREATE TABLE IF NOT EXISTS material_used_proofs_v5 (
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    proof_id VARCHAR(255) NOT NULL,
    issue_id UUID NOT NULL REFERENCES material_issues_v5(id),
    witness_role TEXT NOT NULL CHECK (witness_role IN ('ISSUER','RECIPIENT')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id,proof_id)
);

CREATE TABLE IF NOT EXISTS material_outbox_v5 (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    aggregate_kind VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(128) NOT NULL,
    event_kind VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMPTZ,
    UNIQUE(tenant_id,event_kind,aggregate_id)
);

-- Safety net across all writers: reserve cannot exceed *unissued* stock.
-- Row-level lock serializes per-roll changes across concurrent transactions.
CREATE OR REPLACE FUNCTION check_material_reservation_v5() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
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
DROP TRIGGER IF EXISTS material_reservation_guard_v5 ON material_reservations_v5;
CREATE TRIGGER material_reservation_guard_v5
    AFTER INSERT OR UPDATE OF reserved_mm,issued_mm,released_mm ON material_reservations_v5
    FOR EACH ROW EXECUTE FUNCTION check_material_reservation_v5();

DO $$ DECLARE name TEXT; BEGIN
    FOREACH name IN ARRAY ARRAY['material_reservations_v5','material_issues_v5','material_used_proofs_v5','material_outbox_v5'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',name);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_v5 ON %I',name);
        EXECUTE format('CREATE POLICY tenant_isolation_v5 ON %I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id = NULLIF(current_setting(''app.current_tenant_id'',true),''''))',name);
        EXECUTE format('GRANT SELECT,INSERT,UPDATE,DELETE ON %I TO agiparel_app',name);
    END LOOP;
END $$;
COMMIT;
