-- Application-role definitions live in the published ML Project. Only explicit
-- per-user selections are stored here; there is no implicit administrator role.
CREATE TABLE ml_system.application_role_assignments (
    user_id UUID NOT NULL,
    database_id UUID NOT NULL,
    project_id UUID NOT NULL CHECK (project_id <> '00000000-0000-0000-0000-000000000000'),
    role_ids UUID[] NOT NULL DEFAULT '{}',
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    updated_by_user_id UUID REFERENCES ml_system.users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (user_id, database_id),
    FOREIGN KEY (user_id, database_id) REFERENCES ml_system.database_access(user_id, database_id) ON DELETE CASCADE,
    CHECK (cardinality(role_ids) <= 1024),
    CHECK (array_position(role_ids, NULL) IS NULL),
    CHECK (NOT ('00000000-0000-0000-0000-000000000000'::uuid = ANY(role_ids)))
);
