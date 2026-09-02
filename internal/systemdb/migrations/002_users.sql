CREATE TABLE ml_system.users (
    id UUID PRIMARY KEY,
    login TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    platform_administrator BOOLEAN NOT NULL DEFAULT FALSE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT users_login_not_blank CHECK (btrim(login) <> ''),
    CONSTRAINT users_login_length CHECK (char_length(login) <= 128)
);

CREATE UNIQUE INDEX users_login_case_insensitive_uq
    ON ml_system.users ((lower(login)));

CREATE TABLE ml_system.recovery_codes (
    user_id UUID NOT NULL REFERENCES ml_system.users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    used_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, code_hash),
    CONSTRAINT recovery_code_hash_length CHECK (octet_length(code_hash) = 32)
);

