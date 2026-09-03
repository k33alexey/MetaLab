package systemdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	portalIdleLifetime     = 12 * time.Hour
	portalAbsoluteLifetime = 7 * 24 * time.Hour
)

var (
	ErrSessionNotFound        = errors.New("session not found or expired")
	ErrDatabaseNotRunning     = errors.New("database is not running")
	ErrNewSessionsForbidden   = errors.New("new sessions are forbidden for this database")
	ErrPasswordChangeRequired = errors.New("password change is required")
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
	if userID.IsZero() || len(tokenHash) != 32 {
		return PortalSession{}, fmt.Errorf("invalid portal session identity")
	}
	id, err := uuid.New()
	if err != nil {
		return PortalSession{}, err
	}
	remoteAddress = boundedText(remoteAddress, 512)
	userAgent = boundedText(userAgent, 1000)
	return scanPortalSession(repository.pool.QueryRow(ctx, `
WITH inserted AS (
    INSERT INTO ml_system.portal_sessions(
        id, user_id, token_hash, remote_address, user_agent, idle_expires_at, absolute_expires_at
    ) VALUES ($1, $2, $3, $4, $5, clock_timestamp() + $6::interval, clock_timestamp() + $7::interval)
    RETURNING *
)
SELECT sessions.id::text, sessions.user_id::text, users.login,
       users.platform_administrator, users.metadata_administrator, users.must_change_password,
       sessions.remote_address, sessions.user_agent, sessions.created_at, sessions.last_seen_at,
       sessions.idle_expires_at, sessions.absolute_expires_at
FROM inserted AS sessions JOIN ml_system.users AS users ON users.id = sessions.user_id`,
		id.String(), userID.String(), tokenHash, remoteAddress, userAgent,
		portalIdleLifetime.String(), portalAbsoluteLifetime.String()))
}

// AuthenticatePortal verifies and touches a live bearer session by digest.
func (repository *SessionRepository) AuthenticatePortal(ctx context.Context, tokenHash []byte) (PortalSession, error) {
	if len(tokenHash) != 32 {
		return PortalSession{}, ErrSessionNotFound
	}
	session, err := scanPortalSession(repository.pool.QueryRow(ctx, `
WITH touched AS (
    UPDATE ml_system.portal_sessions
    SET last_seen_at = clock_timestamp(),
        idle_expires_at = LEAST(absolute_expires_at, clock_timestamp() + $2::interval)
    WHERE token_hash = $1 AND revoked_at IS NULL
      AND idle_expires_at > clock_timestamp() AND absolute_expires_at > clock_timestamp()
      AND last_seen_at < clock_timestamp() - interval '1 minute'
    RETURNING *
), active AS (
    SELECT * FROM touched
    UNION ALL
    SELECT sessions.* FROM ml_system.portal_sessions AS sessions
    WHERE sessions.token_hash = $1 AND sessions.revoked_at IS NULL
      AND sessions.idle_expires_at > clock_timestamp() AND sessions.absolute_expires_at > clock_timestamp()
      AND NOT EXISTS (SELECT 1 FROM touched)
    LIMIT 1
)
SELECT sessions.id::text, sessions.user_id::text, users.login,
       users.platform_administrator, users.metadata_administrator, users.must_change_password,
       sessions.remote_address, sessions.user_agent, sessions.created_at, sessions.last_seen_at,
       sessions.idle_expires_at, sessions.absolute_expires_at
FROM active AS sessions JOIN ml_system.users AS users ON users.id = sessions.user_id
WHERE users.enabled`, tokenHash, portalIdleLifetime.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return PortalSession{}, ErrSessionNotFound
	}
	return session, err
}

// RevokePortal terminates the portal session and every database session derived from it.
func (repository *SessionRepository) RevokePortal(ctx context.Context, tokenHash []byte) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin portal logout: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var id string
	err = transaction.QueryRow(ctx, `
UPDATE ml_system.portal_sessions SET revoked_at = clock_timestamp()
WHERE token_hash = $1 AND revoked_at IS NULL RETURNING id::text`, tokenHash).Scan(&id)
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
	var running, allowed bool
	if err := repository.pool.QueryRow(ctx, `
SELECT state = 'running', allow_new_sessions FROM ml_system.databases WHERE id = $1`, databaseID.String()).Scan(&running, &allowed); errors.Is(err, pgx.ErrNoRows) {
		return DatabaseSession{}, ErrDatabaseNotFound
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
	session, err := scanDatabaseSession(repository.pool.QueryRow(ctx, `
WITH opened AS (
    INSERT INTO ml_system.database_sessions(id, portal_session_id, database_id)
    VALUES ($1, $2, $3)
    ON CONFLICT (portal_session_id, database_id) WHERE terminated_at IS NULL
    DO UPDATE SET last_seen_at = clock_timestamp()
    RETURNING *
)
SELECT sessions.id::text, sessions.portal_session_id::text, sessions.database_id::text, databases.name,
       portal.user_id::text, users.login, portal.remote_address, portal.user_agent,
       sessions.started_at, sessions.last_seen_at, COALESCE(sessions.message, ''), sessions.message_created_at
FROM opened AS sessions
JOIN ml_system.portal_sessions AS portal ON portal.id = sessions.portal_session_id
JOIN ml_system.users AS users ON users.id = portal.user_id
JOIN ml_system.databases AS databases ON databases.id = sessions.database_id`,
		id.String(), portalSessionID.String(), databaseID.String()))
	if err != nil {
		return DatabaseSession{}, fmt.Errorf("open database session: %w", err)
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
JOIN ml_system.users AS users ON users.id = portal.user_id
JOIN ml_system.databases AS databases ON databases.id = sessions.database_id`,
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
