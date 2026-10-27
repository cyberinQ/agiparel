-- Batch 08A. Source-linked, append-only marker lays and draft bundle genealogy.
-- All records are PROPOSED_HOLD: NOT authoritative cutting, machine vision,
-- a physical RFID scan, material consumption or a garment-serial assignment.
-- Apply after 0018. Never modify signed stores/material records.
BEGIN;
CREATE TABLE cut_lay_drafts_v8 (
 id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 intake_id varchar(40) NOT NULL,
 cut_no integer NOT NULL CHECK(cut_no BETWEEN 1 AND 100000),
 revision integer NOT NULL CHECK(revision BETWEEN 1 AND 10001),
 request_key varchar(128) NOT NULL CHECK(request_key ~ '^[A-Za-z0-9_-]{8,128}$'),
 input_sha256 char(64) NOT NULL CHECK(input_sha256 ~ '^[0-9a-f]{64}$'),
 source_order_sha256 char(64) NOT NULL CHECK(source_order_sha256 ~ '^[0-9a-f]{64}$'),
 marker_code varchar(64) NOT NULL,
 fabric_sku varchar(128) NOT NULL,
 shade_band varchar(32) NOT NULL,
 marker_length_mm bigint NOT NULL CHECK(marker_length_mm BETWEEN 100 AND 100000000),
 usable_width_mm integer NOT NULL CHECK(usable_width_mm BETWEEN 100 AND 4000),
 reported_plies integer NOT NULL CHECK(reported_plies BETWEEN 1 AND 210),
 bundle_ply_group_size integer NOT NULL CHECK(bundle_ply_group_size BETWEEN 1 AND 210),
 planned_garments integer NOT NULL CHECK(planned_garments BETWEEN 1 AND 16800),
 review_note text NOT NULL CHECK(length(btrim(review_note)) BETWEEN 12 AND 1000),
 state varchar(32) NOT NULL DEFAULT 'PROPOSED_HOLD' CHECK(state='PROPOSED_HOLD'),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,intake_id,cut_no,revision),
 UNIQUE(tenant_id,request_key),
 FOREIGN KEY(tenant_id,intake_id) REFERENCES order_intakes_v5(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX cut_lay_drafts_by_order_v8 ON cut_lay_drafts_v8(tenant_id,intake_id,cut_no,revision DESC);

CREATE TABLE cut_lay_snapshots_v8 (
 tenant_id varchar(64) NOT NULL,
 lay_draft_id uuid NOT NULL,
 snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 snapshot_sha256 char(64) NOT NULL CHECK(snapshot_sha256 ~ '^[0-9a-f]{64}$'),
 PRIMARY KEY(tenant_id,lay_draft_id),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE cut_lay_positions_v8 (
 tenant_id varchar(64) NOT NULL,
 lay_draft_id uuid NOT NULL,
 marker_position integer NOT NULL CHECK(marker_position BETWEEN 1 AND 80),
 color_code varchar(64) NOT NULL,
 size_code varchar(32) NOT NULL,
 PRIMARY KEY(tenant_id,lay_draft_id,marker_position),
 FOREIGN KEY(tenant_id,lay_draft_id) REFERENCES cut_lay_drafts_v8(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE cut_bundle_drafts_v8 (
 tenant_id varchar(64) NOT NULL,
 lay_draft_id uuid NOT NULL,
 label_code varchar(29) NOT NULL CHECK(label_code ~ '^CB08-[0-9A-F]{24}$'),
 marker_position integer NOT NULL,
 color_code varchar(64) NOT NULL,
 size_code varchar(32) NOT NULL,
 part_code varchar(64) NOT NULL,
 pieces_per_garment integer NOT NULL CHECK(pieces_per_garment BETWEEN 1 AND 8),
 ply_group integer NOT NULL CHECK(ply_group BETWEEN 1 AND 210),
 ply_start integer NOT NULL CHECK(ply_start BETWEEN 1 AND 210),
 ply_end integer NOT NULL CHECK(ply_end BETWEEN ply_start AND 210),
 piece_count integer NOT NULL CHECK(piece_count=(ply_end-ply_start+1)*pieces_per_garment),
 rfid_state varchar(32) NOT NULL DEFAULT 'UNBOUND_DRAFT_LABEL' CHECK(rfid_state='UNBOUND_DRAFT_LABEL'),
 PRIMARY KEY(tenant_id,lay_draft_id,marker_position,part_code,ply_group),
 UNIQUE(tenant_id,label_code),
 FOREIGN KEY(tenant_id,lay_draft_id,marker_position) REFERENCES cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position) ON DELETE RESTRICT
);
CREATE INDEX cut_bundle_drafts_by_label_v8 ON cut_bundle_drafts_v8(tenant_id,label_code);

DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['cut_lay_drafts_v8','cut_lay_snapshots_v8','cut_lay_positions_v8','cut_bundle_drafts_v8'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY cutting_tenant_v8 ON public.%I FOR ALL TO agiparel_app USING(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT,INSERT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
-- Prevent edits even by an administrative SQL session; corrections append revisions.
CREATE FUNCTION protect_cut_draft_v8() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ERR_CUT_DRAFT_IMMUTABLE'; END $$;
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['cut_lay_drafts_v8','cut_lay_snapshots_v8','cut_lay_positions_v8','cut_bundle_drafts_v8'] LOOP
  EXECUTE format('CREATE TRIGGER cutting_immutable_v8 BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8()',tbl);
 END LOOP;
END $$;
COMMIT;
