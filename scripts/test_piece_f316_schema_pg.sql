-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only checks after migration 0045. Do not create factory evidence.
DO $$ DECLARE tbl text; forced boolean; has_insert boolean; has_update boolean; BEGIN
 FOREACH tbl IN ARRAY ARRAY['wip_piece_holds_v26','wip_piece_reviews_v26'] LOOP
  SELECT relforcerowsecurity INTO forced FROM pg_class WHERE oid=('public.'||tbl)::regclass;
  IF NOT forced THEN RAISE EXCEPTION 'ERR_F316_RLS_NOT_FORCED %',tbl; END IF;
  SELECT has_table_privilege('agiparel_app','public.'||tbl,'INSERT'),
         has_table_privilege('agiparel_app','public.'||tbl,'UPDATE') INTO has_insert,has_update;
  IF NOT has_insert OR has_update THEN RAISE EXCEPTION 'ERR_F316_PRIVILEGES %',tbl; END IF;
 END LOOP;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.wip_movements_v20'::regclass AND tgname='zz_wip_piece_hold_v26' AND tgenabled='O')
 THEN RAISE EXCEPTION 'ERR_F316_NO_WIP_HOLD'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.qc_garment_events_v21'::regclass AND tgname='zz_qc_piece_hold_v26' AND tgenabled='O')
 THEN RAISE EXCEPTION 'ERR_F316_NO_QC_HOLD'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.sew_prod_completions_v10'::regclass AND tgname='zz_sew_completion_piece_hold_v26' AND tgenabled='O')
 THEN RAISE EXCEPTION 'ERR_F316_NO_WHOLE_GROUP_CREDIT_HOLD'; END IF;
 IF NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='sewing.piece.hold') OR
 NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE permission='sewing.piece.review')
 THEN RAISE EXCEPTION 'ERR_F316_PERMISSION_CATALOG'; END IF;
 RAISE NOTICE 'PASS F3.16: per-garment signed hold, independent scan-backed review, RLS, WIP/QC/whole-bundle credit gates';
END $$;
