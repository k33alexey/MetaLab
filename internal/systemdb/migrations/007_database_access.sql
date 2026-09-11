CREATE TABLE ml_system.database_access (
    user_id UUID NOT NULL REFERENCES ml_system.users(id) ON DELETE CASCADE,
    database_id UUID NOT NULL REFERENCES ml_system.databases(id) ON DELETE CASCADE,
    access_level TEXT NOT NULL,
    app_access BOOLEAN NOT NULL DEFAULT FALSE,
    studio_access BOOLEAN NOT NULL DEFAULT FALSE,
    database_administrator BOOLEAN NOT NULL DEFAULT FALSE,
    granted_by_user_id UUID REFERENCES ml_system.users(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, database_id),
    CONSTRAINT database_access_level CHECK (access_level IN ('owner', 'member')),
    CONSTRAINT database_access_not_empty CHECK (
        revoked_at IS NOT NULL OR access_level = 'owner' OR app_access OR studio_access OR database_administrator
    )
);

CREATE UNIQUE INDEX database_access_one_owner_uq
    ON ml_system.database_access (database_id)
    WHERE access_level = 'owner' AND revoked_at IS NULL;

CREATE INDEX database_access_user_active_idx
    ON ml_system.database_access (user_id, granted_at DESC)
    WHERE revoked_at IS NULL;

WITH first_administrator AS (
    SELECT id
    FROM ml_system.users
    WHERE enabled AND platform_administrator
    ORDER BY created_at, id
    LIMIT 1
)
INSERT INTO ml_system.database_access(
    user_id, database_id, access_level, app_access, studio_access, database_administrator, granted_by_user_id
)
SELECT administrator.id, databases.id, 'owner', FALSE, TRUE, TRUE, administrator.id
FROM first_administrator AS administrator
CROSS JOIN ml_system.databases AS databases
ON CONFLICT (user_id, database_id) DO NOTHING;

INSERT INTO ml_system.database_access(user_id, database_id, access_level, app_access, granted_by_user_id)
SELECT DISTINCT portal.user_id, sessions.database_id, 'member', TRUE, portal.user_id
FROM ml_system.database_sessions AS sessions
JOIN ml_system.portal_sessions AS portal ON portal.id = sessions.portal_session_id
ON CONFLICT (user_id, database_id) DO UPDATE
SET app_access = TRUE, revoked_at = NULL;
