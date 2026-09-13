-- Manager and Portal use the same accounts but never accept each other's tokens.
ALTER TABLE ml_system.portal_sessions
    ADD COLUMN purpose TEXT NOT NULL DEFAULT 'portal',
    ADD CONSTRAINT sessions_purpose CHECK (purpose IN ('portal', 'manager'));

ALTER TABLE ml_system.studio_sessions
    ADD COLUMN owner_user_id UUID REFERENCES ml_system.users(id) ON DELETE CASCADE;

-- Old leases have no authenticated ML identity and must be reopened from Manager.
UPDATE ml_system.studio_sessions SET terminated_at = clock_timestamp()
WHERE terminated_at IS NULL;
