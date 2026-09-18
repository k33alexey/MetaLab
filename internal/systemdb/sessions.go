package systemdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	portalIdleLifetime     = 12 * time.Hour
	portalAbsoluteLifetime = 7 * 24 * time.Hour
	// sessionAbandonedAfter is how long a session may go unseen before it is
	// treated as abandoned and released. A live client touches its session at
	// most once a minute (see the throttle in authenticateSession and
	// resumeDatabase), so two minutes of silence means nobody is there - while
	// a browser closed without logging out would otherwise hold the account's
	// only session for the whole idle lifetime.
	sessionAbandonedAfter = 2 * time.Minute
)

var (
	ErrSessionNotFound          = errors.New("session not found or expired")
	ErrDatabaseNotRunning       = errors.New("database is not running")
	ErrNewSessionsForbidden     = errors.New("new sessions are forbidden for this database")
	ErrPasswordChangeRequired   = errors.New("password change is required")
	ErrApplicationSessionActive = errors.New("an application session for this user and database is already active elsewhere")
	ErrPortalSessionActive      = errors.New("a portal session for this account is already active elsewhere")
)

// PortalSession is a revocable authentication session. Its bearer token is never persisted.
type PortalSession struct {
	ID                 uuid.UUID `json:"id"`
	UserID             uuid.UUID `json:"userId"`
	Login              string    `json:"login"`
	PlatformAdmin      bool      `json:"platformAdministrator"`
	MetadataAdmin      bool      `json:"metadataAdministrator"`
	MustChangePassword bool      `json:"mustChangePassword"`
	RemoteAddress      string    `json:"remoteAddress"`
	UserAgent          string    `json:"userAgent"`
	CreatedAt          time.Time `json:"createdAt"`
	LastSeenAt         time.Time `json:"lastSeenAt"`
	IdleExpiresAt      time.Time `json:"idleExpiresAt"`
	AbsoluteExpiresAt  time.Time `json:"absoluteExpiresAt"`
}

// DatabaseSession represents one user's active entry into one application database.
type DatabaseSession struct {
	ID              uuid.UUID  `json:"id"`
	PortalSessionID uuid.UUID  `json:"portalSessionId"`
	DatabaseID      uuid.UUID  `json:"databaseId"`
	DatabaseName    string     `json:"databaseName"`
	UserID          uuid.UUID  `json:"userId"`
	Login           string     `json:"login"`
	RemoteAddress   string     `json:"remoteAddress"`
	UserAgent       string     `json:"userAgent"`
	StartedAt       time.Time  `json:"startedAt"`
	LastSeenAt      time.Time  `json:"lastSeenAt"`
	Message         string     `json:"message,omitempty"`
	MessageAt       *time.Time `json:"messageAt,omitempty"`
}

// SessionRepository persists portal authentication and per-database sessions.
type SessionRepository struct{ pool *pgxpool.Pool }

// CreatePortal creates a bounded session for an authenticated user and token digest.
func (repository *SessionRepository) CreatePortal(ctx context.Context, userID uuid.UUID, tokenHash []byte, remoteAddress, userAgent string) (PortalSession, error) {
	return repository.createSession(ctx, userID, tokenHash, remoteAddress, userAgent, "portal")
}

// CreateManager creates an independent Manager session for an eligible account.
func (repository *SessionRepository) CreateManager(ctx context.Context, userID uuid.UUID, tokenHash []byte, remoteAddress, userAgent string) (PortalSession, error) {
	return repository.createSession(ctx, userID, tokenHash, remoteAddress, userAgent, "manager")
}

func (repository *SessionRepository) createSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, remoteAddress, userAgent, purpose string) (PortalSession, error) {
	if userID.IsZero() || len(tokenHash) != 32 {
		return PortalSession{}, fmt.Errorf("invalid portal session identity")
	}
	id, err := uuid.New()
	if err != nil {
		return PortalSession{}, err
	}
	remoteAddress = boundedText(remoteAddress, 512)
	userAgent = boundedText(userAgent, 1000)
	session, err := scanPortalSession(repository.pool.QueryRow(ctx, `
WITH released AS (
    -- An account has one portal session. A session nobody has touched for
    -- longer than the abandonment window is released here so the account can
    -- sign in again; one that is still being used is not, and the unique
    -- index below turns this login into a refusal rather than a takeover.
    UPDATE ml_system.portal_sessions SET revoked_at = clock_timestamp()
    WHERE user_id = $2 AND purpose = $8 AND $8 = 'portal' AND revoked_at IS NULL
      AND (idle_expires_at <= clock_timestamp() OR absolute_expires_at <= clock_timestamp()
           OR last_seen_at <= clock_timestamp() - $9::interval)
    RETURNING id
), closed AS (
    UPDATE ml_system.database_sessions SET terminated_at = clock_timestamp()
    WHERE portal_session_id IN (SELECT id FROM released) AND terminated_at IS NULL
    RETURNING 1
), inserted AS (
    INSERT INTO ml_system.portal_sessions(
        id, user_id, token_hash, remote_address, user_agent, idle_expires_at, absolute_expires_at, purpose
    )
    -- The count of released sessions is read here so that releasing runs
    -- BEFORE the insert: the order of data-modifying CTEs is otherwise
    -- unspecified, and an insert that ran first would collide with the very
    -- row this statement is about to free.
    SELECT $1, users.id, $3, $4, $5, clock_timestamp() + $6::interval, clock_timestamp() + $7::interval, $8
    FROM ml_system.users AS users
    WHERE (SELECT count(*) FROM released) >= 0
      AND users.id = $2 AND users.enabled AND ($8 = 'portal' OR
        users.platform_administrator OR users.metadata_administrator OR EXISTS (
            SELECT 1 FROM ml_system.database_access AS access
            WHERE access.user_id = users.id AND access.revoked_at IS NULL AND access.database_administrator
        ))
    RETURNING *
)
SELECT sessions.id::text, sessions.user_id::text, users.login,
       users.platform_administrator, users.metadata_administrator, users.must_change_password,
       sessions.remote_address, sessions.user_agent, sessions.created_at, sessions.last_seen_at,
       sessions.idle_expires_at, sessions.absolute_expires_at
FROM inserted AS sessions JOIN ml_system.users AS users ON users.id = sessions.user_id`,
		id.String(), userID.String(), tokenHash, remoteAddress, userAgent,
		portalIdleLifetime.String(), portalAbsoluteLifetime.String(), purpose, sessionAbandonedAfter.String()))
	// The refusal comes from the database, not from a check before the insert:
	// two logins racing each other would both pass such a check.
	var violation *pgconn.PgError
	if errors.As(err, &violation) && violation.Code == "23505" && violation.ConstraintName == "portal_sessions_one_active_idx" {
		return PortalSession{}, ErrPortalSessionActive
	}
	return session, err
}

// AuthenticatePortal verifies and touches a live bearer session by digest.
func (repository *SessionRepository) AuthenticatePortal(ctx context.Context, tokenHash []byte) (PortalSession, error) {
	return repository.authenticateSession(ctx, tokenHash, "portal")
}

// AuthenticateManager rechecks current system rights on every request.
func (repository *SessionRepository) AuthenticateManager(ctx context.Context, tokenHash []byte) (PortalSession, error) {
	return repository.authenticateSession(ctx, tokenHash, "manager")
}

func (repository *SessionRepository) authenticateSession(ctx context.Context, tokenHash []byte, purpose string) (PortalSession, error) {
	if len(tokenHash) != 32 {
		return PortalSession{}, ErrSessionNotFound
	}
	session, err := scanPortalSession(repository.pool.QueryRow(ctx, `
WITH touched AS (
    UPDATE ml_system.portal_sessions
    SET last_seen_at = clock_timestamp(),
        idle_expires_at = LEAST(absolute_expires_at, clock_timestamp() + $2::interval)
    WHERE token_hash = $1 AND purpose = $3 AND revoked_at IS NULL
      AND idle_expires_at > clock_timestamp() AND absolute_expires_at > clock_timestamp()
      AND last_seen_at < clock_timestamp() - interval '1 minute'
    RETURNING *
), active AS (
    SELECT * FROM touched
    UNION ALL
    SELECT sessions.* FROM ml_system.portal_sessions AS sessions
    WHERE sessions.token_hash = $1 AND sessions.purpose = $3 AND sessions.revoked_at IS NULL
      AND sessions.idle_expires_at > clock_timestamp() AND sessions.absolute_expires_at > clock_timestamp()
      AND NOT EXISTS (SELECT 1 FROM touched)
    LIMIT 1
)
SELECT sessions.id::text, sessions.user_id::text, users.login,
       users.platform_administrator, users.metadata_administrator, users.must_change_password,
       sessions.remote_address, sessions.user_agent, sessions.created_at, sessions.last_seen_at,
       sessions.idle_expires_at, sessions.absolute_expires_at
FROM active AS sessions JOIN ml_system.users AS users ON users.id = sessions.user_id
WHERE users.enabled AND ($3 = 'portal' OR users.platform_administrator OR users.metadata_administrator OR EXISTS (
    SELECT 1 FROM ml_system.database_access AS access
    WHERE access.user_id = users.id AND access.revoked_at IS NULL AND access.database_administrator
))`, tokenHash, portalIdleLifetime.String(), purpose))
	if errors.Is(err, pgx.ErrNoRows) {
		return PortalSession{}, ErrSessionNotFound
	}
	return session, err
}

// RevokePortal terminates the portal session and every database session derived from it.
func (repository *SessionRepository) RevokePortal(ctx context.Context, tokenHash []byte) error {
	return repository.revokeSession(ctx, tokenHash, "portal")
}

// RevokeManager does not revoke the same user's independent Portal session.
func (repository *SessionRepository) RevokeManager(ctx context.Context, tokenHash []byte) error {
	return repository.revokeSession(ctx, tokenHash, "manager")
}

func (repository *SessionRepository) revokeSession(ctx context.Context, tokenHash []byte, purpose string) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin portal logout: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var id string
	err = transaction.QueryRow(ctx, `
UPDATE ml_system.portal_sessions SET revoked_at = clock_timestamp()
WHERE token_hash = $1 AND purpose = $2 AND revoked_at IS NULL RETURNING id::text`, tokenHash, purpose).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke portal session: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
UPDATE ml_system.database_sessions SET terminated_at = clock_timestamp()
WHERE portal_session_id = $1 AND terminated_at IS NULL`, id); err != nil {
		return fmt.Errorf("terminate database sessions on logout: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit portal logout: %w", err)
	}
	return nil
}

// OpenDatabase creates or refreshes a session only while the database accepts logins.
func (repository *SessionRepository) OpenDatabase(ctx context.Context, portalSessionID, databaseID uuid.UUID) (DatabaseSession, error) {
	if portalSessionID.IsZero() || databaseID.IsZero() {
		return DatabaseSession{}, fmt.Errorf("portal and database identifiers are required")
	}
	if existing, err := repository.resumeDatabase(ctx, portalSessionID, databaseID); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrSessionNotFound) {
		return DatabaseSession{}, err
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return DatabaseSession{}, fmt.Errorf("begin database session: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var running, allowed bool
	if err := transaction.QueryRow(ctx, `
SELECT databases.state = 'running', databases.allow_new_sessions
FROM ml_system.databases AS databases
JOIN ml_system.portal_sessions AS portal ON portal.id = $1
JOIN ml_system.users AS users ON users.id = portal.user_id AND users.enabled
JOIN ml_system.database_access AS access
  ON access.user_id = portal.user_id AND access.database_id = databases.id
  AND access.app_access AND access.revoked_at IS NULL
WHERE databases.id = $2 AND portal.purpose = 'portal' AND portal.revoked_at IS NULL
  AND portal.idle_expires_at > clock_timestamp() AND portal.absolute_expires_at > clock_timestamp()
FOR SHARE OF access`, portalSessionID.String(), databaseID.String()).Scan(&running, &allowed); errors.Is(err, pgx.ErrNoRows) {
		return DatabaseSession{}, ErrDatabaseAccessDenied
	} else if err != nil {
		return DatabaseSession{}, fmt.Errorf("read database session access: %w", err)
	}
	if !running {
		return DatabaseSession{}, ErrDatabaseNotRunning
	}
	if !allowed {
		return DatabaseSession{}, ErrNewSessionsForbidden
	}
	id, err := uuid.New()
	if err != nil {
		return DatabaseSession{}, err
	}
	// The unique index is scoped to (user_id, database_id), not
	// (portal_session_id, database_id): the same user opening the same
	// database from a second portal session (a different device/tab) must
	// conflict with their own already-active session there, not create a
	// second one. ON CONFLICT ... DO UPDATE ... WHERE only applies the
	// update (and returns the row) when the conflicting row already belongs
	// to THIS portal session - resuming the same tab. When it belongs to a
	// different portal session, the WHERE makes the whole statement a
	// no-op and RETURNING yields zero rows, which surfaces below as
	// pgx.ErrNoRows and is mapped to ErrApplicationSessionActive.
	session, err := scanDatabaseSession(transaction.QueryRow(ctx, `
WITH released AS (
    -- The same rule as for the portal session: a session nobody has touched
    -- for longer than the abandonment window is not "in use elsewhere", it is
    -- a browser that was closed. Without this the user is locked out of their
    -- own database until the portal session itself expires.
    UPDATE ml_system.database_sessions SET terminated_at = clock_timestamp()
    WHERE database_id = $3 AND terminated_at IS NULL
      AND last_seen_at <= clock_timestamp() - $4::interval
      AND user_id = (SELECT user_id FROM ml_system.portal_sessions WHERE id = $2)
    RETURNING 1
), opened AS (
    INSERT INTO ml_system.database_sessions(id, portal_session_id, database_id, user_id)
    SELECT $1, $2, $3, portal.user_id
    FROM ml_system.portal_sessions AS portal WHERE portal.id = $2
    ON CONFLICT (user_id, database_id) WHERE terminated_at IS NULL
    DO UPDATE SET last_seen_at = clock_timestamp()
    WHERE ml_system.database_sessions.portal_session_id = EXCLUDED.portal_session_id
    RETURNING *
)
SELECT sessions.id::text, sessions.portal_session_id::text, sessions.database_id::text, databases.name,
       portal.user_id::text, users.login, portal.remote_address, portal.user_agent,
       sessions.started_at, sessions.last_seen_at, COALESCE(sessions.message, ''), sessions.message_created_at
FROM opened AS sessions
JOIN ml_system.portal_sessions AS portal ON portal.id = sessions.portal_session_id
JOIN ml_system.users AS users ON users.id = portal.user_id
JOIN ml_system.databases AS databases ON databases.id = sessions.database_id`,
		id.String(), portalSessionID.String(), databaseID.String(), sessionAbandonedAfter.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return DatabaseSession{}, ErrApplicationSessionActive
	}
	if err != nil {
		return DatabaseSession{}, fmt.Errorf("open database session: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return DatabaseSession{}, fmt.Errorf("commit database session: %w", err)
	}
	return session, nil
}

// ResumeDatabase touches an existing active session without creating a replacement.
func (repository *SessionRepository) ResumeDatabase(ctx context.Context, portalSessionID, databaseID uuid.UUID) (DatabaseSession, error) {
	if portalSessionID.IsZero() || databaseID.IsZero() {
		return DatabaseSession{}, ErrSessionNotFound
	}
	return repository.resumeDatabase(ctx, portalSessionID, databaseID)
}

// AcknowledgeMessage removes a delivered message from the same active session.
func (repository *SessionRepository) AcknowledgeMessage(ctx context.Context, portalSessionID, id uuid.UUID) error {
	result, err := repository.pool.Exec(ctx, `
UPDATE ml_system.database_sessions SET message = NULL, message_created_at = NULL
WHERE id = $1 AND portal_session_id = $2 AND terminated_at IS NULL`, id.String(), portalSessionID.String())
	if err != nil {
		return fmt.Errorf("acknowledge session message: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrSessionNotFound
	}
	return nil
}

// ListDatabaseSessions returns active sessions, optionally limited to one database.
func (repository *SessionRepository) ListDatabaseSessions(ctx context.Context, databaseID *uuid.UUID) ([]DatabaseSession, error) {
	var database any
	if databaseID != nil {
		database = databaseID.String()
	}
	rows, err := repository.pool.Query(ctx, `
SELECT sessions.id::text, sessions.portal_session_id::text, sessions.database_id::text, databases.name,
       portal.user_id::text, users.login, portal.remote_address, portal.user_agent,
       sessions.started_at, sessions.last_seen_at, COALESCE(sessions.message, ''), sessions.message_created_at
FROM ml_system.database_sessions AS sessions
JOIN ml_system.portal_sessions AS portal ON portal.id = sessions.portal_session_id
JOIN ml_system.users AS users ON users.id = portal.user_id
JOIN ml_system.databases AS databases ON databases.id = sessions.database_id
WHERE sessions.terminated_at IS NULL AND portal.revoked_at IS NULL
  AND portal.idle_expires_at > clock_timestamp() AND portal.absolute_expires_at > clock_timestamp()
  AND ($1::uuid IS NULL OR sessions.database_id = $1)
ORDER BY sessions.last_seen_at DESC`, database)
	if err != nil {
		return nil, fmt.Errorf("list database sessions: %w", err)
	}
	defer rows.Close()
	items := make([]DatabaseSession, 0)
	for rows.Next() {
		item, scanErr := scanDatabaseSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate database sessions: %w", err)
	}
	return items, nil
}

// SendMessage replaces the pending administrative message for an active session.
func (repository *SessionRepository) SendMessage(ctx context.Context, id uuid.UUID, message string) error {
	message = strings.TrimSpace(message)
	if id.IsZero() || message == "" || utf8.RuneCountInString(message) > 2000 {
		return fmt.Errorf("invalid session message")
	}
	result, err := repository.pool.Exec(ctx, `
UPDATE ml_system.database_sessions
SET message = $2, message_created_at = clock_timestamp()
WHERE id = $1 AND terminated_at IS NULL`, id.String(), message)
	if err != nil {
		return fmt.Errorf("send session message: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrSessionNotFound
	}
	return nil
}

// TerminateDatabaseSession revokes one active database session.
func (repository *SessionRepository) TerminateDatabaseSession(ctx context.Context, id uuid.UUID) error {
	result, err := repository.pool.Exec(ctx, `
UPDATE ml_system.database_sessions SET terminated_at = clock_timestamp()
WHERE id = $1 AND terminated_at IS NULL`, id.String())
	if err != nil {
		return fmt.Errorf("terminate database session: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrSessionNotFound
	}
	return nil
}

// TerminateDatabaseSessions revokes all current sessions of a database.
func (repository *SessionRepository) TerminateDatabaseSessions(ctx context.Context, databaseID uuid.UUID) (int64, error) {
	result, err := repository.pool.Exec(ctx, `
UPDATE ml_system.database_sessions SET terminated_at = clock_timestamp()
WHERE database_id = $1 AND terminated_at IS NULL`, databaseID.String())
	if err != nil {
		return 0, fmt.Errorf("terminate database sessions: %w", err)
	}
	return result.RowsAffected(), nil
}

func scanPortalSession(row rowScanner) (PortalSession, error) {
	var session PortalSession
	var id, userID string
	if err := row.Scan(
		&id, &userID, &session.Login, &session.PlatformAdmin, &session.MetadataAdmin,
		&session.MustChangePassword, &session.RemoteAddress, &session.UserAgent,
		&session.CreatedAt, &session.LastSeenAt, &session.IdleExpiresAt, &session.AbsoluteExpiresAt,
	); err != nil {
		return PortalSession{}, err
	}
	var err error
	session.ID, err = uuid.Parse(id)
	if err == nil {
		session.UserID, err = uuid.Parse(userID)
	}
	if err != nil {
		return PortalSession{}, fmt.Errorf("parse portal session identifier: %w", err)
	}
	return session, nil
}

func scanDatabaseSession(row rowScanner) (DatabaseSession, error) {
	var session DatabaseSession
	var id, portalID, databaseID, userID string
	if err := row.Scan(
		&id, &portalID, &databaseID, &session.DatabaseName, &userID, &session.Login,
		&session.RemoteAddress, &session.UserAgent, &session.StartedAt, &session.LastSeenAt,
		&session.Message, &session.MessageAt,
	); err != nil {
		return DatabaseSession{}, err
	}
	var err error
	session.ID, err = uuid.Parse(id)
	if err == nil {
		session.PortalSessionID, err = uuid.Parse(portalID)
	}
	if err == nil {
		session.DatabaseID, err = uuid.Parse(databaseID)
	}
	if err == nil {
		session.UserID, err = uuid.Parse(userID)
	}
	if err != nil {
		return DatabaseSession{}, fmt.Errorf("parse database session identifier: %w", err)
	}
	return session, nil
}

func (repository *SessionRepository) resumeDatabase(ctx context.Context, portalSessionID, databaseID uuid.UUID) (DatabaseSession, error) {
	session, err := scanDatabaseSession(repository.pool.QueryRow(ctx, `
WITH touched AS (
    UPDATE ml_system.database_sessions
    SET last_seen_at = clock_timestamp()
    WHERE portal_session_id = $1 AND database_id = $2 AND terminated_at IS NULL
      AND last_seen_at < clock_timestamp() - interval '1 minute'
    RETURNING *
), active AS (
    SELECT * FROM touched
    UNION ALL
    SELECT sessions.* FROM ml_system.database_sessions AS sessions
    WHERE sessions.portal_session_id = $1 AND sessions.database_id = $2 AND sessions.terminated_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM touched)
    LIMIT 1
)
SELECT sessions.id::text, sessions.portal_session_id::text, sessions.database_id::text, databases.name,
       portal.user_id::text, users.login, portal.remote_address, portal.user_agent,
       sessions.started_at, sessions.last_seen_at, COALESCE(sessions.message, ''), sessions.message_created_at
FROM active AS sessions
JOIN ml_system.portal_sessions AS portal ON portal.id = sessions.portal_session_id
JOIN ml_system.users AS users ON users.id = portal.user_id AND users.enabled
JOIN ml_system.databases AS databases ON databases.id = sessions.database_id
JOIN ml_system.database_access AS access
  ON access.user_id = portal.user_id AND access.database_id = sessions.database_id
  AND access.app_access AND access.revoked_at IS NULL
WHERE portal.purpose = 'portal' AND portal.revoked_at IS NULL
  AND portal.idle_expires_at > clock_timestamp() AND portal.absolute_expires_at > clock_timestamp()`,
		portalSessionID.String(), databaseID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return DatabaseSession{}, ErrSessionNotFound
	}
	if err != nil {
		return DatabaseSession{}, fmt.Errorf("resume database session: %w", err)
	}
	return session, nil
}

func boundedText(value string, maximum int) string {
	value = strings.TrimSpace(value)
	characters := []rune(value)
	if len(characters) > maximum {
		value = string(characters[:maximum])
	}
	return value
}
