ALTER TABLE ml_system.databases
    ADD COLUMN mode TEXT NOT NULL DEFAULT 'primary',
    ADD COLUMN source_database_id UUID REFERENCES ml_system.databases(id) ON DELETE RESTRICT,
    ADD CONSTRAINT databases_mode CHECK (mode IN ('primary', 'debug')),
    ADD CONSTRAINT databases_source_not_self CHECK (source_database_id IS NULL OR source_database_id <> id),
    ADD CONSTRAINT databases_primary_has_no_source CHECK (mode = 'debug' OR source_database_id IS NULL);

CREATE INDEX databases_source_database_id_idx
    ON ml_system.databases (source_database_id)
    WHERE source_database_id IS NOT NULL;
