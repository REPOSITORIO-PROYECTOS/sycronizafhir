CREATE TABLE IF NOT EXISTS sync_device_connections (
    session_id UUID PRIMARY KEY,
    device_id TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    source_kind TEXT,
    local_db TEXT
);

CREATE INDEX IF NOT EXISTS idx_sync_device_connections_device_id
ON sync_device_connections (device_id);

CREATE INDEX IF NOT EXISTS idx_sync_device_connections_last_seen_at
ON sync_device_connections (last_seen_at DESC);
