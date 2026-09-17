ALTER TABLE ml_system.database_sessions
    ADD COLUMN user_id UUID REFERENCES ml_system.users(id) ON DELETE CASCADE;

UPDATE ml_system.database_sessions AS sessions
SET user_id = portal.user_id
FROM ml_system.portal_sessions AS portal
WHERE portal.id = sessions.portal_session_id;

ALTER TABLE ml_system.database_sessions
    ALTER COLUMN user_id SET NOT NULL;

DROP INDEX ml_system.database_sessions_one_active_idx;

CREATE UNIQUE INDEX database_sessions_one_active_idx
    ON ml_system.database_sessions (user_id, database_id)
    WHERE terminated_at IS NULL;
