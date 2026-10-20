-- ============================================================================
-- DOMAIN 8: WORKFORCE, ATTENDANCE & SKILL MATRIX
-- ============================================================================
CREATE TABLE IF NOT EXISTS workers (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    employee_code VARCHAR(64) NOT NULL,
    primary_skill VARCHAR(64) NOT NULL,
    skill_grade VARCHAR(8) NOT NULL CHECK (skill_grade IN ('A', 'B', 'C', 'TRAINEE')),
    hourly_rate_inr NUMERIC(10,2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE workers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_workers ON workers FOR ALL 
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

CREATE TABLE IF NOT EXISTS attendance_register (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    worker_id VARCHAR(64) NOT NULL REFERENCES workers(id),
    shift_date DATE NOT NULL,
    clock_in_time TIMESTAMPTZ,
    clock_out_time TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PRESENT', 'ABSENT', 'LATE', 'ON_LEAVE')),
    assigned_line_id VARCHAR(64),
    assigned_station_id VARCHAR(64)
);
ALTER TABLE attendance_register ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_attendance ON attendance_register FOR ALL 
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

-- ============================================================================
-- DOMAIN 9: MACHINE ROSTER & HEALTH
-- ============================================================================
CREATE TABLE IF NOT EXISTS machine_roster (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    line_id VARCHAR(64) NOT NULL,
    station_id VARCHAR(64),
    name VARCHAR(128) NOT NULL,
    canonical_class VARCHAR(64) NOT NULL,
    rated_capacity_uph INT NOT NULL CHECK (rated_capacity_uph > 0),
    status VARCHAR(32) NOT NULL CHECK (status IN ('ACTIVE', 'IDLE', 'BREAKDOWN', 'MAINTENANCE')),
    active_operator_id VARCHAR(64) REFERENCES workers(id)
);
ALTER TABLE machine_roster ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_machine_roster ON machine_roster FOR ALL 
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

-- ============================================================================
-- DOMAIN 10: HOURLY PRODUCTION VELOCITY (TIME-SERIES AGGREGATE)
-- ============================================================================
CREATE TABLE IF NOT EXISTS hourly_line_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL REFERENCES tenants(id),
    line_id VARCHAR(64) NOT NULL,
    shift_date DATE NOT NULL,
    hour_window INT NOT NULL CHECK (hour_window BETWEEN 1 AND 12),
    target_units INT NOT NULL,
    actual_units INT NOT NULL DEFAULT 0,
    defect_units INT NOT NULL DEFAULT 0,
    andon_downtime_minutes INT NOT NULL DEFAULT 0,
    dhu_pct NUMERIC(5,2) GENERATED ALWAYS AS (
        CASE WHEN actual_units > 0 THEN (defect_units::numeric / actual_units::numeric) * 100 ELSE 0 END
    ) STORED,
    recorded_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_line_hour UNIQUE (tenant_id, line_id, shift_date, hour_window)
);
ALTER TABLE hourly_line_metrics ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_hourly_metrics ON hourly_line_metrics FOR ALL 
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), ''));

-- Seed Pilot Workers & Ground Truth Attendance for Noble Garments
INSERT INTO tenants (id, name) VALUES ('TENANT_DEFAULT', 'Noble Garments Okhla') ON CONFLICT DO NOTHING;

INSERT INTO workers (id, tenant_id, name, employee_code, primary_skill, skill_grade) VALUES
('W-01', 'TENANT_DEFAULT', 'Ram Kumar', 'EMP_A811', 'SNLS_TOPSTITCH', 'A'),
('W-02', 'TENANT_DEFAULT', 'Sita Devi', 'EMP_A812', '4-OL_SERGING', 'A'),
('W-03', 'TENANT_DEFAULT', 'Sunita Sharma', 'EMP_A813', 'KANSAI_HEMMING', 'B'),
('W-04', 'TENANT_DEFAULT', 'Mohan Lal', 'EMP_A814', 'SNLS_BACK_RISE', 'A'),
('W-05', 'TENANT_DEFAULT', 'Ramesh Chand', 'EMP_MECH_01', 'LINE_MECHANIC', 'A')
ON CONFLICT DO NOTHING;

INSERT INTO attendance_register (tenant_id, worker_id, shift_date, clock_in_time, status, assigned_line_id, assigned_station_id) VALUES
('TENANT_DEFAULT', 'W-01', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '4 hours', 'PRESENT', 'jin 02', 'ST-01'),
('TENANT_DEFAULT', 'W-02', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '4 hours', 'PRESENT', 'jin 02', 'ST-02'),
('TENANT_DEFAULT', 'W-03', CURRENT_DATE, NULL, 'ABSENT', 'jin 02', 'ST-03'), -- Worker absent!
('TENANT_DEFAULT', 'W-04', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '4 hours', 'PRESENT', 'jin 02', 'ST-04'),
('TENANT_DEFAULT', 'W-05', CURRENT_DATE, CURRENT_TIMESTAMP - INTERVAL '5 hours', 'PRESENT', 'jin 02', 'MECHANIC_FLOOR')
ON CONFLICT DO NOTHING;