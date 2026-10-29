-- Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
-- Disposable transaction: prove actual app-role RLS, deny wrong-tenant rows.
-- Runs against PostgreSQL 16 after 0053; rolls all fixture rows back.
BEGIN;
INSERT INTO tenants(id,name) VALUES
 ('F34_ISOLATION_A','Dock isolation fixture A'),
 ('F34_ISOLATION_B','Dock isolation fixture B');
INSERT INTO dock_station_policies_v34(tenant_id,plant_id,max_scan_age_minutes,enabled,approved_by)
 VALUES ('F34_ISOLATION_A','PLANT-01',45,true,'CI_PROVISIONER'),
        ('F34_ISOLATION_B','PLANT-01',45,true,'CI_PROVISIONER');
SET LOCAL ROLE agiparel_app;
SELECT set_config('app.current_tenant_id','F34_ISOLATION_A',true);
DO $$ BEGIN
 IF (SELECT count(*) FROM dock_station_policies_v34 WHERE plant_id='PLANT-01')<>1
 OR NOT EXISTS(SELECT 1 FROM dock_station_policies_v34 WHERE tenant_id='F34_ISOLATION_A')
 OR EXISTS(SELECT 1 FROM dock_station_policies_v34 WHERE tenant_id='F34_ISOLATION_B')
 THEN RAISE EXCEPTION 'ERR_F34_TENANT_A_EXPOSED'; END IF;
END $$;
SELECT set_config('app.current_tenant_id','F34_ISOLATION_B',true);
DO $$ BEGIN
 IF (SELECT count(*) FROM dock_station_policies_v34 WHERE plant_id='PLANT-01')<>1
 OR NOT EXISTS(SELECT 1 FROM dock_station_policies_v34 WHERE tenant_id='F34_ISOLATION_B')
 OR EXISTS(SELECT 1 FROM dock_station_policies_v34 WHERE tenant_id='F34_ISOLATION_A')
 THEN RAISE EXCEPTION 'ERR_F34_TENANT_B_EXPOSED'; END IF;
END $$;
SELECT set_config('app.current_tenant_id','',true);
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM dock_station_policies_v34)
 THEN RAISE EXCEPTION 'ERR_F34_NULL_TENANT_LEAK'; END IF;
END $$;
ROLLBACK;
