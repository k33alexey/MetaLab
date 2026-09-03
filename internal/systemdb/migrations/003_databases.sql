CREATE TABLE ml_system.databases (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    physical_id UUID NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    database_name TEXT NOT NULL,
    username TEXT NOT NULL,
    ssl_mode TEXT NOT NULL,
    secret_key TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'stopped',
    state_revision BIGINT NOT NULL DEFAULT 1,
    last_error TEXT,
    state_changed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT databases_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT databases_name_length CHECK (char_length(name) <= 128),
    CONSTRAINT databases_host_not_blank CHECK (btrim(host) <> ''),
    CONSTRAINT databases_port_range CHECK (port BETWEEN 1 AND 65535),
    CONSTRAINT databases_database_name_not_blank CHECK (btrim(database_name) <> ''),
    CONSTRAINT databases_username_not_blank CHECK (btrim(username) <> ''),
    CONSTRAINT databases_ssl_mode CHECK (ssl_mode IN ('disable', 'require', 'verify-ca', 'verify-full')),
    CONSTRAINT databases_state CHECK (state IN ('stopped', 'starting', 'running', 'stopping', 'maintenance', 'error')),
    CONSTRAINT databases_state_revision_positive CHECK (state_revision > 0)
);

CREATE UNIQUE INDEX databases_name_case_insensitive_uq
    ON ml_system.databases ((lower(name)));

CREATE UNIQUE INDEX databases_physical_id_uq
    ON ml_system.databases (physical_id);

CREATE UNIQUE INDEX databases_secret_key_uq
    ON ml_system.databases (secret_key);
