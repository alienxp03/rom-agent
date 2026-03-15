CREATE TABLE IF NOT EXISTS exchange_thing_snapshots (
    thing_id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    refreshed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_exchange_thing_snapshots_refreshed_at
    ON exchange_thing_snapshots(refreshed_at DESC);
