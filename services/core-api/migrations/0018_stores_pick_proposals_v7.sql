-- Batch 07D. Durable, idempotent, non-authoritative store pick-list previews.
-- Deliberately grants NO new privileges on store_rolls, material_reservations,
-- material_issues, tenant ownership or material-signing scopes.
-- Apply after migration 0017. Agiparel dev only, as database administrator.
BEGIN;
CREATE TABLE IF NOT EXISTS store_pick_proposals_v7 (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    intake_id varchar(40) NOT NULL,
    request_key varchar(128) NOT NULL CHECK(request_key ~ '^[A-Za-z0-9_-]{8,128}$'),
    review_note text NOT NULL CHECK(length(btrim(review_note)) BETWEEN 12 AND 1000),
    source_order_sha256 char(64) NOT NULL CHECK (source_order_sha256 ~ '^[0-9a-f]{64}$'),
    snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
    snapshot_sha256 char(64) NOT NULL CHECK (snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    state text NOT NULL DEFAULT 'PROPOSED_HOLD' CHECK(state='PROPOSED_HOLD'),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id,intake_id) REFERENCES order_intakes_v5(tenant_id,id) ON DELETE RESTRICT,
    UNIQUE(tenant_id,request_key)
);
CREATE INDEX IF NOT EXISTS store_pick_proposals_recent_v7 ON store_pick_proposals_v7(tenant_id,created_at DESC);
ALTER TABLE store_pick_proposals_v7 ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_pick_proposals_v7 FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS store_pick_proposals_tenant_v7 ON store_pick_proposals_v7;
CREATE POLICY store_pick_proposals_tenant_v7 ON store_pick_proposals_v7
    FOR ALL TO agiparel_app
    USING(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
    WITH CHECK(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON store_pick_proposals_v7 FROM PUBLIC,agiparel_app;
GRANT SELECT,INSERT ON store_pick_proposals_v7 TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON store_pick_proposals_v7 FROM agiparel_app;
-- Guarantee immutable planning records even if runtime grants change later.
CREATE OR REPLACE FUNCTION protect_stores_pick_proposal_v7() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ERR_PICK_PROPOSAL_IMMUTABLE'; END $$;
DROP TRIGGER IF EXISTS store_pick_proposal_immutable_v7 ON store_pick_proposals_v7;
CREATE TRIGGER store_pick_proposal_immutable_v7 BEFORE UPDATE OR DELETE ON store_pick_proposals_v7
 FOR EACH ROW EXECUTE FUNCTION protect_stores_pick_proposal_v7();
COMMIT;
