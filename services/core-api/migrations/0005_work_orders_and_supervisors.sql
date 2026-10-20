-- ============================================================================
-- DOMAIN 11: WORK ORDERS & PRODUCTION RUNS
-- ============================================================================
CREATE TABLE IF NOT EXISTS work_orders (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    order_id VARCHAR(64) NOT NULL,
    po_number VARCHAR(64) NOT NULL,
    style_code VARCHAR(64) NOT NULL,
    line_id VARCHAR(64) NOT NULL,
    supervisor_id VARCHAR(64) NOT NULL,
    target_qty INT NOT NULL,
    completed_qty INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PLANNED', 'IN_PROGRESS', 'COMPLETED', 'HOLD')),
    started_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE work_orders ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_work_orders ON work_orders FOR ALL 
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

-- Seed Active Pilot Work Order
INSERT INTO work_orders (id, tenant_id, order_id, po_number, style_code, line_id, supervisor_id, target_qty, completed_qty, status)
VALUES (
    'WO-MK6574J9-01', 
    'TENANT_DEFAULT', 
    'ORD-2026-001', 
    'MK6574J9', 
    'KK524867', 
    'jin 02', 
    'SUPV_RAM_KUMAR', 
    600, 
    3, 
    'IN_PROGRESS'
) ON CONFLICT (id) DO NOTHING;

-- Seed Supervisor profile
INSERT INTO workers (id, tenant_id, name, employee_code, primary_skill, skill_grade)
VALUES ('SUPV_RAM_KUMAR', 'TENANT_DEFAULT', 'Ram Kumar', 'EMP_SUPV_01', 'LINE_SUPERVISION', 'A')
ON CONFLICT (id) DO NOTHING;