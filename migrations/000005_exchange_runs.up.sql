CREATE TABLE IF NOT EXISTS exchange_runs (
    id BIGSERIAL PRIMARY KEY,
    market TEXT NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_minutes DOUBLE PRECISION,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_exchange_runs_started_at
    ON exchange_runs(started_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_exchange_runs_incomplete_market
    ON exchange_runs(market)
    WHERE completed_at IS NULL;
