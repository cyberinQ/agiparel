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
