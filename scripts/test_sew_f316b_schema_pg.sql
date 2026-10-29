-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Read-only schema verification. No production sample records inserted.
DO $$ DECLARE tbl text; BEGIN
 FOR tbl IN SELECT unnest(ARRAY['sew_piece_completions_v27','sew_piece_used_scans_v27']) LOOP
  IF NOT EXISTS (SELECT 1 FROM pg_class t JOIN pg_namespace n ON n.oid=t.relnamespace
   WHERE n.nspname='public' AND t.relname=tbl AND t.relrowsecurity AND t.relforcerowsecurity)
  THEN RAISE EXCEPTION 'F3.16B RLS missing: %',tbl; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname='public' AND tablename=tbl
   AND policyname='sew_piece_tenant_v27') THEN RAISE EXCEPTION 'F3.16B tenant policy missing: %',tbl; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger tg JOIN pg_class t ON t.oid=tg.tgrelid
   WHERE t.relname=tbl AND tg.tgname='sew_piece_immutable_v27' AND NOT tg.tgisinternal)
  THEN RAISE EXCEPTION 'F3.16B immutable trigger missing: %',tbl; END IF;
 END LOOP;
 IF has_table_privilege('agiparel_app','public.sew_piece_used_scans_v27','INSERT')
 OR has_table_privilege('agiparel_app','public.sew_piece_used_scans_v27','UPDATE')
 OR has_table_privilege('agiparel_app','public.sew_piece_completions_v27','UPDATE')
 THEN RAISE EXCEPTION 'F3.16B unexpected app privilege'; END IF;
 IF NOT has_table_privilege('agiparel_app','public.sew_piece_completions_v27','INSERT')
 THEN RAISE EXCEPTION 'F3.16B application cannot insert signed completions'; END IF;
 IF (SELECT count(*) FROM pg_trigger tg WHERE tg.tgname IN
 ('sew_piece_completion_guard_v27','sew_piece_scans_once_v27','zzz_sew_piece_no_group_v27',
  'zzz_sew_piece_scan_move_v27','zzz_sew_piece_scan_exception_v27','zzz_sew_piece_scan_review_v27')
  AND NOT tg.tgisinternal)<>6
 THEN RAISE EXCEPTION 'F3.16B required guards missing'; END IF;
 IF NOT EXISTS(SELECT 1 FROM workforce_chapter_actions_v17 WHERE chapter_id=6
  AND permission='sewing.piece.complete' AND authority='SIGNED_WRITE')
 THEN RAISE EXCEPTION 'F3.16B signed action missing'; END IF;
 RAISE NOTICE 'PASS F3.16B: signed piece operation, RLS, scan exclusivity, hold/whole-group guards, immutable evidence';
END $$;
