CREATE TABLE IF NOT EXISTS scan_results (
    id BIGSERIAL PRIMARY KEY,
    record_id TEXT NOT NULL,
    thing_id BIGINT NOT NULL,
    price BIGINT NOT NULL,
    stock_count INTEGER NOT NULL DEFAULT 0,
    refine_level INTEGER NOT NULL DEFAULT 0,
    enchant INTEGER,
    enchant_level INTEGER NOT NULL DEFAULT 0,
    broken BOOLEAN NOT NULL DEFAULT FALSE,
    snap_at TIMESTAMP WITH TIME ZONE NOT NULL,
    snap_end_at TIMESTAMP WITH TIME ZONE,
    projection_signature TEXT NOT NULL,
    snap_ids BIGINT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_results_record_id ON scan_results(record_id);
CREATE INDEX IF NOT EXISTS idx_scan_results_thing_id ON scan_results(thing_id);
CREATE INDEX IF NOT EXISTS idx_scan_results_projection_signature ON scan_results(projection_signature);
CREATE INDEX IF NOT EXISTS idx_scan_results_snap_at ON scan_results(snap_at DESC);
