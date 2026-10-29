-- v43.2: count-only global aggregates; runtime role must not see identity rows directly.
DO $test$
DECLARE c bigint; u bigint;
BEGIN
  IF NOT has_function_privilege('agiparel_app','public.engineer_ops_counts_v43_2()','EXECUTE') THEN
    RAISE EXCEPTION 'agiparel_app must be able to invoke count aggregate';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_proc p, LATERAL pg_catalog.aclexplode(p.proacl) a WHERE p.oid='public.engineer_ops_counts_v43_2()'::regprocedure AND a.grantee=0 AND a.privilege_type='EXECUTE') THEN
    RAISE EXCEPTION 'aggregate function must not be public';
  END IF;
  IF has_table_privilege('agiparel_app','s01_identity.users','SELECT') THEN
    RAISE EXCEPTION 'application role must not obtain raw identity-user access';
  END IF;
  SELECT customer_count,user_count INTO c,u FROM public.engineer_ops_counts_v43_2();
  IF c IS NULL OR u IS NULL OR c < 0 OR u < 0 THEN
    RAISE EXCEPTION 'non-null non-negative identity counts required';
  END IF;
  RAISE NOTICE 'PASS v43.2: only aggregate counts permitted, underlying identity user rows denied';
END $test$;
