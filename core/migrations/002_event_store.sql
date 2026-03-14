CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL,
    recorded_at INTEGER NOT NULL,
    sequence INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_events_aggregate_seq ON events(aggregate_type, aggregate_id, sequence);
CREATE INDEX idx_events_aggregate ON events(aggregate_type, aggregate_id);

-- Recreate accounts with TEXT id for UUID-based aggregate identity.
DROP TABLE IF EXISTS accounts;
CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
