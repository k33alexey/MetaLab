package systemdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

var ErrManagerAccessDenied = errors.New("ML Manager access denied")

// DatabaseOwnerInfo describes registry ownership, not a PostgreSQL or Studio user.
type DatabaseOwnerInfo struct {
	UserID uuid.UUID `json:"userId"`
	Login  string    `json:"login"`
}

// OwnersForDatabases batches ownership lookup for an already authorized registry view.
// Callers must not supply database identifiers from an untrusted HTTP request.
func (repository *DatabaseAccessRepository) OwnersForDatabases(ctx context.Context, databaseIDs []uuid.UUID) (map[uuid.UUID]DatabaseOwnerInfo, error) {
	result := make(map[uuid.UUID]DatabaseOwnerInfo)
	if len(databaseIDs) == 0 {
		return result, nil
	}
	ids := make([]string, len(databaseIDs))
	for i, id := range databaseIDs {
		if id.IsZero() {
			return nil, ErrDatabaseAccessDenied
		}
		ids[i] = id.String()
	}
	rows, err := repository.pool.Query(ctx, `SELECT access.database_id::text,users.id::text,users.login
FROM ml_system.database_access AS access JOIN ml_system.users AS users ON users.id=access.user_id
WHERE access.database_id=ANY($1::uuid[]) AND access.access_level='owner' AND access.revoked_at IS NULL`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var databaseText, userText string
		var owner DatabaseOwnerInfo
		if err := rows.Scan(&databaseText, &userText, &owner.Login); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(databaseText)
		if err != nil {
			return nil, err
		}
		owner.UserID, err = uuid.Parse(userText)
		if err != nil {
			return nil, err
		}
		result[id] = owner
	}
	return result, rows.Err()
}

func assignRegisteredDatabaseOwner(ctx context.Context, tx pgx.Tx, databaseID uuid.UUID, ownerID *uuid.UUID) error {
	if ownerID == nil {
		return assignInitialDatabaseOwner(ctx, tx, databaseID)
	}
	result, err := tx.Exec(ctx, `INSERT INTO ml_system.database_access(user_id,database_id,access_level,studio_access,database_administrator,granted_by_user_id)
SELECT id,$1,'owner',metadata_administrator,TRUE,id FROM ml_system.users
WHERE id=$2 AND enabled AND (metadata_administrator OR platform_administrator)`, databaseID.String(), ownerID.String())
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrManagerAccessDenied
	}
	return nil
}

// DatabasePermissions are independent grants; ownership never implies App access.
type DatabasePermissions struct {
	App    bool `json:"app"`
	Studio bool `json:"studio"`
	Admin  bool `json:"admin"`
}

// SetPermissions replaces the three explicit grants atomically with session revocation.
func (repository *DatabaseAccessRepository) SetPermissions(ctx context.Context, actorID, userID, databaseID uuid.UUID, permissions DatabasePermissions) error {
	if actorID.IsZero() || userID.IsZero() || databaseID.IsZero() {
		return ErrDatabaseAccessDenied
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockDatabasePermissions(ctx, tx, databaseID); err != nil {
		return err
	}
	allowed, err := canManageDatabaseAccess(ctx, tx, actorID, databaseID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDatabaseOwnerOnly
	}
	var metadataAdmin bool
	err = tx.QueryRow(ctx, `SELECT metadata_administrator FROM ml_system.users WHERE id=$1 AND enabled FOR SHARE`, userID.String()).Scan(&metadataAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatabaseAccessDenied
	}
	if err != nil {
		return err
	}
	if permissions.Studio && !metadataAdmin {
		return fmt.Errorf("Studio requires the Metadata Administrator system role")
	}
	_, err = tx.Exec(ctx, `
INSERT INTO ml_system.database_access(user_id,database_id,access_level,app_access,studio_access,database_administrator,granted_by_user_id,revoked_at)
VALUES ($1,$2,'member',$3,$4,$5,$6,CASE WHEN $3 OR $4 OR $5 THEN NULL ELSE clock_timestamp() END)
ON CONFLICT(user_id,database_id) DO UPDATE SET
 app_access=$3, studio_access=$4, database_administrator=$5, granted_by_user_id=$6,
 revoked_at=CASE WHEN $3 OR $4 OR $5 OR ml_system.database_access.access_level='owner' THEN NULL ELSE clock_timestamp() END`,
		userID.String(), databaseID.String(), permissions.App, permissions.Studio, permissions.Admin, actorID.String())
	if err != nil {
		return err
	}
	if !permissions.App {
		_, err = tx.Exec(ctx, `UPDATE ml_system.database_sessions SET terminated_at=clock_timestamp()
WHERE database_id=$2 AND terminated_at IS NULL AND portal_session_id IN (SELECT id FROM ml_system.portal_sessions WHERE user_id=$1)`, userID.String(), databaseID.String())
		if err != nil {
			return err
		}
	}
	if !permissions.Studio {
		_, err = tx.Exec(ctx, `UPDATE ml_system.studio_sessions SET terminated_at=clock_timestamp()
WHERE database_id=$2 AND owner_user_id=$1 AND terminated_at IS NULL`, userID.String(), databaseID.String())
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO ml_system.audit_events(level,event_code,database_id,user_id,message,details)
VALUES('info','database.permissions_changed',$1,$2,'Database access updated',jsonb_build_object('userId',$3::text,'app',$4::boolean,'studio',$5::boolean,'admin',$6::boolean))`,
		databaseID.String(), actorID.String(), userID.String(), permissions.App, permissions.Studio, permissions.Admin)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Serialize all grant editors, including legacy App-only methods. A non-key
// update lock remains compatible with FK locks taken by session creation.
func lockDatabasePermissions(ctx context.Context, tx pgx.Tx, databaseID uuid.UUID) error {
	var exists string
	err := tx.QueryRow(ctx, `SELECT id::text FROM ml_system.databases WHERE id=$1 FOR NO KEY UPDATE`, databaseID.String()).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatabaseAccessDenied
	}
	return err
}

// ListForDatabase returns assignments only to an administrator of that database.
func (repository *DatabaseAccessRepository) ListForDatabase(ctx context.Context, actorID, databaseID uuid.UUID) ([]DatabaseAccess, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	allowed, err := canManageDatabaseAccess(ctx, tx, actorID, databaseID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrDatabaseAccessDenied
	}
	rows, err := tx.Query(ctx, `SELECT user_id::text, database_id::text, access_level, app_access, studio_access, database_administrator,
COALESCE(granted_by_user_id::text,''),granted_at,users.login FROM ml_system.database_access
JOIN ml_system.users AS users ON users.id=user_id WHERE database_id=$1 AND revoked_at IS NULL ORDER BY lower(users.login),user_id`, databaseID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DatabaseAccess{}
	for rows.Next() {
		item, err := scanDatabaseAccess(rows, true)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
