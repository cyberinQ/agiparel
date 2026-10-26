-- Gate 06.5-C1: immutable source linkage for reviewed, PROPOSED buyer PO/BOM mapping.
-- This is not buyer approval or permission to procure, issue, cut, sew or ship.
BEGIN;
CREATE TABLE IF NOT EXISTS buyer_order_handoffs_v65c1 (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 intake_id varchar(40) NOT NULL,
 po_document_id varchar(36) NOT NULL,
 bom_document_id varchar(36) NOT NULL,
 source_digest char(64) NOT NULL,
 reviewed_by varchar(64) NOT NULL,
 review_note varchar(1000) NOT NULL CHECK (char_length(review_note)>=8),
 mapping jsonb NOT NULL,
 captured_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,source_digest),
 FOREIGN KEY (tenant_id,intake_id) REFERENCES order_intakes_v5(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY (tenant_id,po_document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT,
 FOREIGN KEY (tenant_id,bom_document_id) REFERENCES document_sources_v65(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS buyer_handoffs_intake_v65c1 ON buyer_order_handoffs_v65c1(tenant_id,intake_id);
ALTER TABLE buyer_order_handoffs_v65c1 ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer_order_handoffs_v65c1 FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS buyer_handoff_tenant_v65c1 ON buyer_order_handoffs_v65c1;
CREATE POLICY buyer_handoff_tenant_v65c1 ON buyer_order_handoffs_v65c1
 USING(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''))
 WITH CHECK(tenant_id=NULLIF(current_setting('app.current_tenant_id',true),''));
REVOKE ALL ON buyer_order_handoffs_v65c1 FROM PUBLIC;
GRANT SELECT,INSERT ON buyer_order_handoffs_v65c1 TO agiparel_app;
GRANT USAGE,SELECT ON SEQUENCE buyer_order_handoffs_v65c1_id_seq TO agiparel_app;
REVOKE UPDATE,DELETE,TRUNCATE ON buyer_order_handoffs_v65c1 FROM agiparel_app;
COMMIT;
