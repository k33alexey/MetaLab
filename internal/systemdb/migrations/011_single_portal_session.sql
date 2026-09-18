-- Единственный активный Portal-сеанс на учётную запись обеспечивается базой,
-- а не проверкой в коде: проверку можно обойти гонкой двух входов, индекс -
-- нет. Условие частичное по той же причине, что и у прикладных сеансов:
-- отозванные строки остаются историей и мешать новому входу не должны.
--
-- Сеансы ML Manager (purpose = 'manager') под это правило не попадают: они
-- заводятся отдельно и требованием единственности не охвачены.
UPDATE ml_system.portal_sessions AS sessions
SET revoked_at = clock_timestamp()
WHERE purpose = 'portal' AND revoked_at IS NULL AND EXISTS (
    SELECT 1 FROM ml_system.portal_sessions AS newer
    WHERE newer.user_id = sessions.user_id AND newer.purpose = 'portal'
      AND newer.revoked_at IS NULL
      AND (newer.last_seen_at, newer.id) > (sessions.last_seen_at, sessions.id)
);

CREATE UNIQUE INDEX portal_sessions_one_active_idx
    ON ml_system.portal_sessions (user_id)
    WHERE purpose = 'portal' AND revoked_at IS NULL;
