package systemdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// DatabaseAccessLevel distinguishes the database owner from explicitly admitted members.
type DatabaseAccessLevel string

const (
	DatabaseOwner  DatabaseAccessLevel = "owner"
	DatabaseMember DatabaseAccessLevel = "member"
)

var (
	ErrDatabaseAccessDenied = errors.New("database is not available to this user")
	ErrDatabaseOwnerOnly    = errors.New("database access can only be managed by its owner or an authorized administrator")
)

// DatabaseAccess is the server-side personal Portal membership of one user.
type DatabaseAccess struct {
	UserID          uuid.UUID
	DatabaseID      uuid.UUID
	Level           DatabaseAccessLevel
	AppAccess       bool
	StudioAccess    bool
	DatabaseAdmin   bool
	GrantedByUserID *uuid.UUID
	GrantedAt       time.Time
}

// DatabaseAccessRepository owns personal Portal lists and database membership.
type DatabaseAccessRepository struct{ pool *pgxpool.Pool }

// List returns all active database relationships for one user.
func (repository *DatabaseAccessRepository) List(ctx context.Context, userID uuid.UUID) ([]DatabaseAccess, error) {
	return repository.list(ctx, userID, false)
}

// ListApp returns only databases explicitly assigned to the user's Portal.
func (repository *DatabaseAccessRepository) ListApp(ctx context.Context, userID uuid.UUID) ([]DatabaseAccess, error) {
	return repository.list(ctx, userID, true)
}

func (repository *DatabaseAccessRepository) list(ctx context.Context, userID uuid.UUID, appOnly bool) ([]DatabaseAccess, error) {
	if userID.IsZero() {
		return nil, ErrDatabaseAccessDenied
	}
	rows, err := repository.pool.Query(ctx, `
SELECT user_id::text, database_id::text, access_level, app_access, studio_access, database_administrator,
       COALESCE(granted_by_user_id::text, ''), granted_at
FROM ml_system.database_access
WHERE user_id = $1 AND revoked_at IS NULL AND (NOT $2 OR app_access)
ORDER BY granted_at, database_id`, userID.String(), appOnly)
	if err != nil {
		return nil, fmt.Errorf("list database access: %w", err)
	}
	defer rows.Close()
	items := make([]DatabaseAccess, 0)
	for rows.Next() {
		item, scanErr := scanDatabaseAccess(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate database access: %w", err)
	}
	return items, nil
}

// Get returns an active database relationship.
func (repository *DatabaseAccessRepository) Get(ctx context.Context, userID, databaseID uuid.UUID) (DatabaseAccess, error) {
	return repository.get(ctx, userID, databaseID, false)
}

// GetApp requires explicit ML App access without disclosing whether the database exists.
func (repository *DatabaseAccessRepository) GetApp(ctx context.Context, userID, databaseID uuid.UUID) (DatabaseAccess, error) {
	return repository.get(ctx, userID, databaseID, true)
}

func (repository *DatabaseAccessRepository) get(ctx context.Context, userID, databaseID uuid.UUID, appOnly bool) (DatabaseAccess, error) {
	if userID.IsZero() || databaseID.IsZero() {
		return DatabaseAccess{}, ErrDatabaseAccessDenied
	}
	item, err := scanDatabaseAccess(repository.pool.QueryRow(ctx, `
SELECT user_id::text, database_id::text, access_level, app_access, studio_access, database_administrator,
       COALESCE(granted_by_user_id::text, ''), granted_at
FROM ml_system.database_access
WHERE user_id = $1 AND database_id = $2 AND revoked_at IS NULL
  AND (NOT $3 OR app_access)`, userID.String(), databaseID.String(), appOnly))
	if errors.Is(err, pgx.ErrNoRows) {
		return DatabaseAccess{}, ErrDatabaseAccessDenied
	}
	if err != nil {
		return DatabaseAccess{}, fmt.Errorf("read database access: %w", err)
	}
	return item, nil
}

// GrantApp assigns a database to the user's Portal.
func (repository *DatabaseAccessRepository) GrantApp(ctx context.Context, actorID, userID, databaseID uuid.UUID) (DatabaseAccess, error) {
	if actorID.IsZero() || userID.IsZero() || databaseID.IsZero() {
		return DatabaseAccess{}, ErrDatabaseOwnerOnly
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return DatabaseAccess{}, fmt.Errorf("begin database access grant: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	allowed, err := canManageDatabaseAccess(ctx, transaction, actorID, databaseID)
	if err != nil {
		return DatabaseAccess{}, err
	}
	if !allowed {
		return DatabaseAccess{}, ErrDatabaseOwnerOnly
	}
	item, err := scanDatabaseAccess(transaction.QueryRow(ctx, `
INSERT INTO ml_system.database_access(user_id, database_id, access_level, app_access, granted_by_user_id)
SELECT users.id, databases.id, 'member', TRUE, $1
FROM ml_system.users AS users
CROSS JOIN ml_system.databases AS databases
WHERE users.id = $2 AND users.enabled AND databases.id = $3
ON CONFLICT (user_id, database_id) DO UPDATE
SET access_level = CASE
        WHEN ml_system.database_access.access_level = 'owner' THEN 'owner'
        ELSE 'member'
    END,
    app_access = TRUE,
    granted_by_user_id = CASE
        WHEN ml_system.database_access.access_level = 'owner' THEN ml_system.database_access.granted_by_user_id
        ELSE EXCLUDED.granted_by_user_id
    END,
    granted_at = CASE
        WHEN ml_system.database_access.access_level = 'owner' THEN ml_system.database_access.granted_at
        ELSE clock_timestamp()
    END,
    revoked_at = NULL
RETURNING user_id::text, database_id::text, access_level, app_access, studio_access, database_administrator,
          COALESCE(granted_by_user_id::text, ''), granted_at`,
		actorID.String(), userID.String(), databaseID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return DatabaseAccess{}, ErrDatabaseAccessDenied
	}
	if err != nil {
		return DatabaseAccess{}, fmt.Errorf("grant database access: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return DatabaseAccess{}, fmt.Errorf("commit database access grant: %w", err)
	}
	return item, nil
}

// RevokeApp removes a database from the user's Portal and terminates its application sessions.
func (repository *DatabaseAccessRepository) RevokeApp(ctx context.Context, actorID, userID, databaseID uuid.UUID) error {
	if actorID.IsZero() || userID.IsZero() || databaseID.IsZero() {
		return ErrDatabaseOwnerOnly
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin database access revocation: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	allowed, err := canManageDatabaseAccess(ctx, transaction, actorID, databaseID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDatabaseOwnerOnly
	}
	var level DatabaseAccessLevel
	var studioAccess, databaseAdministrator bool
	err = transaction.QueryRow(ctx, `
SELECT access_level, studio_access, database_administrator
FROM ml_system.database_access
WHERE user_id = $1 AND database_id = $2 AND revoked_at IS NULL AND app_access
FOR UPDATE`, userID.String(), databaseID.String()).Scan(&level, &studioAccess, &databaseAdministrator)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatabaseAccessDenied
	}
	if err != nil {
		return fmt.Errorf("lock database access: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
UPDATE ml_system.database_sessions
SET terminated_at = clock_timestamp()
WHERE database_id = $2 AND terminated_at IS NULL
  AND portal_session_id IN (SELECT id FROM ml_system.portal_sessions WHERE user_id = $1)`,
		userID.String(), databaseID.String()); err != nil {
		return fmt.Errorf("terminate revoked database sessions: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
UPDATE ml_system.database_access
SET app_access = FALSE,
    revoked_at = CASE
        WHEN $3 = 'member' AND NOT $4 AND NOT $5 THEN clock_timestamp()
        ELSE NULL
    END
WHERE user_id = $1 AND database_id = $2`,
		userID.String(), databaseID.String(), level, studioAccess, databaseAdministrator); err != nil {
		return fmt.Errorf("revoke database access: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit database access revocation: %w", err)
	}
	return nil
}

func canManageDatabaseAccess(ctx context.Context, transaction pgx.Tx, actorID, databaseID uuid.UUID) (bool, error) {
	var allowed bool
	err := transaction.QueryRow(ctx, `
SELECT EXISTS(
    SELECT 1 FROM ml_system.users AS users
    WHERE users.id = $1 AND users.enabled AND (
        users.platform_administrator OR EXISTS(
            SELECT 1 FROM ml_system.database_access AS access
            WHERE access.user_id = users.id AND access.database_id = $2
              AND (access.access_level = 'owner' OR access.database_administrator)
              AND access.revoked_at IS NULL
        )
    )
)`, actorID.String(), databaseID.String()).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("authorize database access management: %w", err)
	}
	return allowed, nil
}

func assignInitialDatabaseOwner(ctx context.Context, transaction pgx.Tx, databaseID uuid.UUID) error {
	_, err := transaction.Exec(ctx, `
INSERT INTO ml_system.database_access(
    user_id, database_id, access_level, studio_access, database_administrator, granted_by_user_id
)
SELECT users.id, $1, 'owner', TRUE, TRUE, users.id
FROM ml_system.users AS users
WHERE users.enabled AND users.platform_administrator
  AND NOT EXISTS (
      SELECT 1 FROM ml_system.database_access
      WHERE database_id = $1 AND access_level = 'owner' AND revoked_at IS NULL
  )
ORDER BY users.created_at, users.id
LIMIT 1
ON CONFLICT (user_id, database_id) DO NOTHING`, databaseID.String())
	if err != nil {
		return fmt.Errorf("assign initial database owner: %w", err)
	}
	return nil
}

func assignAllUnownedDatabases(ctx context.Context, transaction pgx.Tx, userID uuid.UUID) error {
	_, err := transaction.Exec(ctx, `
INSERT INTO ml_system.database_access(
    user_id, database_id, access_level, studio_access, database_administrator, granted_by_user_id
)
SELECT $1, databases.id, 'owner', TRUE, TRUE, $1
FROM ml_system.databases AS databases
WHERE NOT EXISTS (
    SELECT 1 FROM ml_system.database_access
    WHERE database_id = databases.id AND access_level = 'owner' AND revoked_at IS NULL
)
ON CONFLICT (user_id, database_id) DO NOTHING`, userID.String())
	if err != nil {
		return fmt.Errorf("assign existing databases to initial owner: %w", err)
	}
	return nil
}

func scanDatabaseAccess(row rowScanner) (DatabaseAccess, error) {
	var item DatabaseAccess
	var userID, databaseID, grantedBy string
	if err := row.Scan(
		&userID, &databaseID, &item.Level, &item.AppAccess, &item.StudioAccess, &item.DatabaseAdmin,
		&grantedBy, &item.GrantedAt,
	); err != nil {
		return DatabaseAccess{}, err
	}
	var err error
	item.UserID, err = uuid.Parse(userID)
	if err == nil {
		item.DatabaseID, err = uuid.Parse(databaseID)
	}
	if err == nil && grantedBy != "" {
		var parsed uuid.UUID
		parsed, err = uuid.Parse(grantedBy)
		item.GrantedByUserID = &parsed
	}
	if err != nil {
		return DatabaseAccess{}, fmt.Errorf("parse database access identifier: %w", err)
	}
	return item, nil
}
