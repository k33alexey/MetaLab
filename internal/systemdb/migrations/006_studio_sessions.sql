CREATE TABLE ml_system.studio_sessions (
    database_id UUID PRIMARY KEY REFERENCES ml_system.databases(id) ON DELETE CASCADE,
    id UUID NOT NULL UNIQUE,
    project_id UUID NOT NULL,
    token_hash BYTEA NOT NULL,
    owner_name TEXT NOT NULL,
    host_name TEXT NOT NULL,
    process_id BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    expires_at TIMESTAMPTZ NOT NULL,
    terminated_at TIMESTAMPTZ,
    CONSTRAINT studio_sessions_token_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT studio_sessions_owner_not_blank CHECK (btrim(owner_name) <> ''),
    CONSTRAINT studio_sessions_host_not_blank CHECK (btrim(host_name) <> ''),
    CONSTRAINT studio_sessions_process_positive CHECK (process_id > 0)
);

CREATE INDEX studio_sessions_active_idx
    ON ml_system.studio_sessions (expires_at)
    WHERE terminated_at IS NULL;
