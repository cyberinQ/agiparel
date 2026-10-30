-- Migration 0025: Enforce Safe Watermark Boundary on Dark-Data Pruning
CREATE OR REPLACE FUNCTION prune_ambient_rf_telemetry_v45(
    p_retention_interval INTERVAL DEFAULT INTERVAL '14 days',
    p_max_safe_watermark TIMESTAMPTZ DEFAULT NULL
)
RETURNS BIGINT
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_cutoff_time TIMESTAMPTZ;
    v_effective_watermark TIMESTAMPTZ;
    v_deleted_count BIGINT := 0;
BEGIN
    v_cutoff_time := NOW() - p_retention_interval;

    IF p_max_safe_watermark IS NULL THEN
        RAISE EXCEPTION 'prune_ambient_rf_telemetry_v45 aborted: p_max_safe_watermark must be explicitly provided';
    END IF;

    v_effective_watermark := LEAST(v_cutoff_time, p_max_safe_watermark);

    WITH deleted_rows AS (
        DELETE FROM edge_ambient_rf_telemetry_v45
        WHERE read_at <= v_effective_watermark
        RETURNING 1
    )
    SELECT COUNT(*) INTO v_deleted_count FROM deleted_rows;

    RETURN v_deleted_count;
END;
$$;
