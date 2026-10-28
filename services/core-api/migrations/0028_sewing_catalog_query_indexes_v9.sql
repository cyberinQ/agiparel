-- Author: Sunil Jain, 2026-10-28
-- Chapter 06: indexes for tenant-scoped, keyset-paginated sewing rehearsal.
-- Additive only: no records, privileges, triggers or provenance are changed.
BEGIN;
CREATE INDEX IF NOT EXISTS cut_lay_drafts_sew_catalog_v9
  ON cut_lay_drafts_v8(tenant_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS cut_bundle_drafts_sew_groups_v9
  ON cut_bundle_drafts_v8(tenant_id, lay_draft_id, marker_position, ply_group, part_code);
CREATE INDEX IF NOT EXISTS sew_sim_scans_claim_lookup_v9
  ON sew_sim_scans_v9(tenant_id, lay_draft_id, plant_id, label_code, station_id, worker_id, received_at DESC);
CREATE INDEX IF NOT EXISTS sew_sim_cycles_history_lookup_v9
  ON sew_sim_cycles_v9(tenant_id, lay_draft_id, plant_id, claimed_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS sew_sim_cycles_active_station_v9
  ON sew_sim_cycles_v9(tenant_id, lay_draft_id, plant_id, station_id, worker_id, claimed_at DESC)
  WHERE state IN ('CLAIMED','PAUSED');
COMMIT;
