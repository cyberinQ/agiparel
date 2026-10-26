-- Agiparel Batch 05: raw, source-backed, PROPOSED-only structured PO/BOM intake.
-- Apply once to the disposable development database after 0001..0006.
-- Never infer buyer approval or a production release from an uploaded CSV.
BEGIN;
CREATE TABLE IF NOT EXISTS order_intakes_v5 (
  id varchar(40) PRIMARY KEY,
  tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  po_number varchar(64) NOT NULL,
  style_code varchar(64) NOT NULL,
  ordered_qty bigint NOT NULL CHECK(ordered_qty > 0),
  source_sha256 char(64) NOT NULL,
  po_filename varchar(255) NOT NULL,
  bom_filename varchar(255) NOT NULL,
  po_original bytea NOT NULL,
  bom_original bytea NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'PROPOSED' CHECK(status='PROPOSED'),
  captured_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tenant_id,id),
  UNIQUE(tenant_id,source_sha256)
);
CREATE INDEX IF NOT EXISTS order_intakes_tenant_recent_v5 ON order_intakes_v5(tenant_id,captured_at DESC);
CREATE TABLE IF NOT EXISTS order_intake_skus_v5 (
  tenant_id varchar(64) NOT NULL,
  intake_id varchar(40) NOT NULL,
  line_no integer NOT NULL CHECK(line_no>0),
  color_code varchar(64) NOT NULL,
  size_code varchar(32) NOT NULL,
  upc varchar(64) NOT NULL DEFAULT '',
  ordered_qty bigint NOT NULL CHECK(ordered_qty>0),
  PRIMARY KEY(tenant_id,intake_id,line_no),
  FOREIGN KEY(tenant_id,intake_id) REFERENCES order_intakes_v5(tenant_id,id) ON DELETE RESTRICT,
  UNIQUE(tenant_id,intake_id,color_code,size_code)
);
CREATE TABLE IF NOT EXISTS order_intake_bom_v5 (
  tenant_id varchar(64) NOT NULL,
  intake_id varchar(40) NOT NULL,
  line_no integer NOT NULL CHECK(line_no>0),
  material_sku varchar(128) NOT NULL,
  category varchar(16) NOT NULL CHECK(category IN ('FABRIC','TRIM')),
  qty_per_garment numeric(14,3) NOT NULL CHECK(qty_per_garment>0),
  uom varchar(16) NOT NULL,
  PRIMARY KEY(tenant_id,intake_id,line_no),
  FOREIGN KEY(tenant_id,intake_id) REFERENCES order_intakes_v5(tenant_id,id) ON DELETE RESTRICT,
  UNIQUE(tenant_id,intake_id,material_sku,category,uom)
);
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['order_intakes_v5','order_intake_skus_v5','order_intake_bom_v5'] LOOP
    EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',t);
    EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',t);
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_v5 ON public.%I',t);
    EXECUTE format('CREATE POLICY tenant_isolation_v5 ON public.%I USING (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK (tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',t);
    EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',t);
  END LOOP;
END $$;
-- Historical proposals must remain immutable to the runtime role.
REVOKE UPDATE, DELETE, TRUNCATE ON order_intakes_v5,order_intake_skus_v5,order_intake_bom_v5 FROM agiparel_app;
COMMIT;
