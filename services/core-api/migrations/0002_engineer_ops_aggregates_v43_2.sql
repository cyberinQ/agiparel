-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Platform-wide count-only aggregate for authenticated Engineer Ops.
-- No PII, tenant records, or direct cross-tenant SELECT grants to application role.
BEGIN;
CREATE OR REPLACE FUNCTION public.engineer_ops_counts_v43_2()
RETURNS TABLE(customer_count bigint, user_count bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $ops$
 SELECT (SELECT count(*)::bigint FROM s01_identity.tenants),
        (SELECT count(*)::bigint FROM s01_identity.users);
$ops$;
REVOKE ALL ON FUNCTION public.engineer_ops_counts_v43_2() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.engineer_ops_counts_v43_2() TO agiparel_app;
COMMIT;
