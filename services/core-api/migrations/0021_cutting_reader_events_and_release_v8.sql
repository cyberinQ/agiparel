-- Cutting reader gateway evidence and independently signed cutting-to-sewing release.
-- An enrolled gateway proves the SOURCE of a report, not independently the RF physics.
-- Keys must be provisioned OUT OF BAND by plant security, never via development login.
BEGIN;
CREATE TABLE cut_reader_gateways_v8 (
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 gateway_id varchar(64) NOT NULL CHECK (gateway_id ~ '^[A-Za-z0-9_-]{3,64}$'),
 plant_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL CHECK (length(station_id) BETWEEN 2 AND 64),
 public_key bytea NOT NULL CHECK (octet_length(public_key)=32),
 enabled boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,gateway_id)
);
-- Provisioning is DBA-controlled; the agiparel_app role cannot INSERT/UPDATE keys.
CREATE TABLE cut_reader_events_v8 (
 id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
 tenant_id varchar(64) NOT NULL,
 plant_id varchar(64) NOT NULL,
 gateway_id varchar(64) NOT NULL,
 station_id varchar(64) NOT NULL,
 event_id varchar(64) NOT NULL CHECK(event_id ~ '^[A-Za-z0-9_-]{12,64}$'),
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(),
 lay_draft_id uuid NOT NULL,
 spread_report_id uuid NOT NULL,
 label_code varchar(29) NOT NULL,
 epc_hex varchar(96) NOT NULL CHECK(epc_hex ~ '^[0-9A-F]{24,96}$' AND length(epc_hex)%2=0),
 antenna_port integer NOT NULL CHECK(antenna_port BETWEEN 1 AND 64),
 event_sha256 char(64) NOT NULL CHECK(event_sha256 ~ '^[0-9a-f]{64}$'),
 signature_sha256 char(64) NOT NULL CHECK(signature_sha256 ~ '^[0-9a-f]{64}$'),
 state varchar(40) NOT NULL DEFAULT 'SIGNED_GATEWAY_REPORT_UNVERIFIED_RF'
  CHECK (state='SIGNED_GATEWAY_REPORT_UNVERIFIED_RF'),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,gateway_id,event_id),
 UNIQUE(tenant_id,spread_report_id,label_code),
 UNIQUE(tenant_id,spread_report_id,epc_hex),
 FOREIGN KEY(tenant_id,gateway_id) REFERENCES cut_reader_gateways_v8(tenant_id,gateway_id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,spread_report_id,lay_draft_id) REFERENCES cut_spread_reports_v8(tenant_id,id,lay_draft_id) ON DELETE RESTRICT,
 FOREIGN KEY(tenant_id,lay_draft_id,label_code) REFERENCES cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code) ON DELETE RESTRICT
);
CREATE INDEX cut_reader_events_by_spread_v8 ON cut_reader_events_v8(tenant_id,spread_report_id);
CREATE TABLE cut_sewing_releases_v8 (
 id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
 tenant_id varchar(64) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
 plant_id varchar(64) NOT NULL,
 spread_report_id uuid NOT NULL,
 lay_draft_id uuid NOT NULL,
 evidence_sha256 char(64) NOT NULL CHECK(evidence_sha256 ~ '^[0-9a-f]{64}$'),
 supervisor_subject varchar(128) NOT NULL,
 quality_subject varchar(128) NOT NULL,
 supervisor_proof_id varchar(255) NOT NULL,
 quality_proof_id varchar(255) NOT NULL,
 supervisor_assertion_sha256 char(64) NOT NULL,
 quality_assertion_sha256 char(64) NOT NULL,
 bundle_count integer NOT NULL CHECK(bundle_count > 0),
 state varchar(48) NOT NULL DEFAULT 'RELEASED_TO_SEWING_SIGNED'
  CHECK(state='RELEASED_TO_SEWING_SIGNED'),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(supervisor_subject<>quality_subject),
 CHECK(supervisor_proof_id<>quality_proof_id),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,spread_report_id),
 UNIQUE(tenant_id,supervisor_proof_id), UNIQUE(tenant_id,quality_proof_id),
 FOREIGN KEY(tenant_id,spread_report_id,lay_draft_id) REFERENCES cut_spread_reports_v8(tenant_id,id,lay_draft_id) ON DELETE RESTRICT
);
-- In addition to application checks, direct SQL cannot manufacture a released
-- row absent complete per-bundle human and gateway evidence.
CREATE FUNCTION validate_cut_release_v8() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE expected integer; humans integer; machines integer; plant_issues integer; issued_total bigint; reported bigint;
BEGIN
 SELECT count(*)::int INTO expected FROM cut_bundle_drafts_v8 b
  WHERE b.tenant_id=NEW.tenant_id AND b.lay_draft_id=NEW.lay_draft_id;
 SELECT count(*)::int INTO humans FROM cut_bundle_scan_witnesses_v8 w
  WHERE w.tenant_id=NEW.tenant_id AND w.spread_report_id=NEW.spread_report_id AND w.plant_id=NEW.plant_id;
 SELECT count(*)::int INTO machines FROM cut_reader_events_v8 e
  JOIN cut_bundle_scan_witnesses_v8 w ON w.tenant_id=e.tenant_id AND w.spread_report_id=e.spread_report_id
   AND w.label_code=e.label_code AND w.epc_hex=e.epc_hex AND w.plant_id=NEW.plant_id
  JOIN cut_reader_gateways_v8 device ON device.tenant_id=e.tenant_id AND device.gateway_id=e.gateway_id
   AND device.enabled AND device.plant_id=NEW.plant_id
  WHERE e.tenant_id=NEW.tenant_id AND e.spread_report_id=NEW.spread_report_id AND e.plant_id=NEW.plant_id;
 SELECT s.reported_consumption_mm INTO reported FROM cut_spread_reports_v8 s WHERE s.tenant_id=NEW.tenant_id AND s.id=NEW.spread_report_id;
 SELECT COALESCE(sum(allocated_mm),0)::bigint INTO issued_total FROM cut_spread_issue_allocations_v8
  WHERE tenant_id=NEW.tenant_id AND spread_report_id=NEW.spread_report_id;
 SELECT count(*)::int INTO plant_issues FROM cut_spread_issue_allocations_v8 a
  JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id
  JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
  WHERE a.tenant_id=NEW.tenant_id AND a.spread_report_id=NEW.spread_report_id AND sr.plant_id<>NEW.plant_id;
 IF expected=0 OR expected<>NEW.bundle_count OR humans<>expected OR machines<>expected OR plant_issues<>0 OR issued_total<>reported THEN
  RAISE EXCEPTION 'ERR_CUT_RELEASE_INCOMPLETE_EVIDENCE';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER cut_release_evidence_guard_v8 BEFORE INSERT ON cut_sewing_releases_v8
 FOR EACH ROW EXECUTE FUNCTION validate_cut_release_v8();
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['cut_reader_gateways_v8','cut_reader_events_v8','cut_sewing_releases_v8'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',tbl);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',tbl);
  EXECUTE format('CREATE POLICY cutting_reader_tenant_v8 ON public.%I FOR ALL TO agiparel_app USING(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),'''')) WITH CHECK(tenant_id=NULLIF(current_setting(''app.current_tenant_id'',true),''''))',tbl);
  EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC,agiparel_app',tbl);
  EXECUTE format('GRANT SELECT ON public.%I TO agiparel_app',tbl);
 END LOOP;
END $$;
GRANT INSERT ON cut_reader_events_v8,cut_sewing_releases_v8 TO agiparel_app;
CREATE TRIGGER cut_reader_event_immutable_v8 BEFORE UPDATE OR DELETE ON cut_reader_events_v8
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
CREATE TRIGGER cut_sewing_release_immutable_v8 BEFORE UPDATE OR DELETE ON cut_sewing_releases_v8
 FOR EACH ROW EXECUTE FUNCTION protect_cut_draft_v8();
-- In contrast, gateway enrollment is an intentionally administrative operation.
COMMIT;
