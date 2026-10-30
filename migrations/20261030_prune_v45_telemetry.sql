-- Author: Sunil Jain, CyberinQ LLC
-- Migration: Prune telemetry batch function for AeroLake CDC bridge retention

CREATE OR REPLACE FUNCTION prune_v45_telemetry_batch(p_watermark_ts TIMESTAMPTZ, p_limit INT)
RETURNS INT AS $$
DECLARE
    v_deleted INT;
    v_watermark_ms BIGINT;
BEGIN
    v_watermark_ms := (EXTRACT(EPOCH FROM p_watermark_ts) * 1000)::BIGINT;

    DELETE FROM edge_ambient_rf_telemetry_v45
    WHERE ctid IN (
        SELECT ctid FROM edge_ambient_rf_telemetry_v45
        WHERE _cdc_ts <= v_watermark_ms
        LIMIT p_limit
    );
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION prune_v45_telemetry_batch(p_watermark_ts TEXT, p_limit INT)
RETURNS INT AS $$
BEGIN
    RETURN prune_v45_telemetry_batch(p_watermark_ts::TIMESTAMPTZ, p_limit);
END;
$$ LANGUAGE plpgsql;

-- Telemetry table definition
CREATE TABLE IF NOT EXISTS edge_ambient_rf_telemetry_v45 (
    id BIGSERIAL PRIMARY KEY,
    antenna_id VARCHAR(64) NOT NULL,
    rssi DOUBLE PRECISION NOT NULL,
    read_count INT DEFAULT 1,
    payload JSONB,
    _cdc_op VARCHAR(8) DEFAULT 'INSERT',
    _cdc_ts BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT,
    created_at TIMESTAMPTZ DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_telemetry_v45_cdc_ts ON edge_ambient_rf_telemetry_v45 (_cdc_ts);
CREATE INDEX IF NOT EXISTS idx_telemetry_v45_antenna ON edge_ambient_rf_telemetry_v45 (antenna_id);
