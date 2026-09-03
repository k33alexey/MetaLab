ALTER TABLE ml_system.databases
    ADD COLUMN allow_new_sessions BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN health_status TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN health_message TEXT,
    ADD COLUMN health_checked_at TIMESTAMPTZ,
    ADD CONSTRAINT databases_health_status CHECK (health_status IN ('unknown', 'healthy', 'unhealthy'));

ALTER TABLE ml_system.users
    ADD COLUMN metadata_administrator BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE ml_system.users SET metadata_administrator = TRUE WHERE platform_administrator;

CREATE TABLE ml_system.portal_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES ml_system.users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    remote_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    idle_expires_at TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT portal_sessions_token_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT portal_sessions_expiry_order CHECK (idle_expires_at <= absolute_expires_at)
);

CREATE INDEX portal_sessions_user_active_idx
    ON ml_system.portal_sessions (user_id, last_seen_at DESC)
    WHERE revoked_at IS NULL;

CREATE TABLE ml_system.database_sessions (
    id UUID PRIMARY KEY,
    portal_session_id UUID NOT NULL REFERENCES ml_system.portal_sessions(id) ON DELETE CASCADE,
    database_id UUID NOT NULL REFERENCES ml_system.databases(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    message TEXT,
    message_created_at TIMESTAMPTZ,
    terminated_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX database_sessions_one_active_idx
    ON ml_system.database_sessions (portal_session_id, database_id)
    WHERE terminated_at IS NULL;

CREATE INDEX database_sessions_database_active_idx
    ON ml_system.database_sessions (database_id, last_seen_at DESC)
    WHERE terminated_at IS NULL;

CREATE TABLE ml_system.audit_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    level TEXT NOT NULL,
    event_code TEXT NOT NULL,
    database_id UUID REFERENCES ml_system.databases(id) ON DELETE SET NULL,
    user_id UUID REFERENCES ml_system.users(id) ON DELETE SET NULL,
    session_id UUID,
    message TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT audit_events_level CHECK (level IN ('info', 'warning', 'error')),
    CONSTRAINT audit_events_code_not_blank CHECK (btrim(event_code) <> ''),
    CONSTRAINT audit_events_message_not_blank CHECK (btrim(message) <> '')
);

CREATE INDEX audit_events_occurred_at_idx ON ml_system.audit_events (occurred_at DESC);
CREATE INDEX audit_events_database_idx ON ml_system.audit_events (database_id, occurred_at DESC);

CREATE TABLE ml_system.backups (
    id UUID PRIMARY KEY,
    database_id UUID NOT NULL REFERENCES ml_system.databases(id) ON DELETE RESTRICT,
    file_name TEXT NOT NULL UNIQUE,
    size_bytes BIGINT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT backups_file_name_not_blank CHECK (btrim(file_name) <> ''),
    CONSTRAINT backups_size_nonnegative CHECK (size_bytes >= 0),
    CONSTRAINT backups_sha256_length CHECK (char_length(sha256) = 64)
);

CREATE INDEX backups_database_created_idx
    ON ml_system.backups (database_id, created_at DESC);
