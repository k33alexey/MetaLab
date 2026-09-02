CREATE TABLE ml_system.settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT settings_key_not_blank CHECK (btrim(key) <> '')
);

