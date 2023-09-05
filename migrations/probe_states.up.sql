CREATE TABLE IF NOT EXISTS magellan_probe_states (
    host            TEXT PRIMARY KEY NOT NULL,
    port            INTEGER,
    protocol        TEXT,
    state           INTEGER,
    updated         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS magellan_probe_states_index_host ON magellan_probe_states (host);
CREATE INDEX IF NOT EXISTS magellan_probe_states_index_state ON magellan_proble_states (state);