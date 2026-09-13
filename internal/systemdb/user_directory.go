package systemdb

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

var ErrLastPlatformAdministrator = errors.New("at least one enabled platform administrator is required")

// UserAccessUpdate never accepts a password, identifier or caller-supplied identity.
type UserAccessUpdate struct {
	PlatformAdministrator bool `json:"platformAdministrator"`
	MetadataAdministrator bool `json:"metadataAdministrator"`
	Enabled               bool `json:"enabled"`
}

// UpdateAccess authorizes and changes system roles atomically, revoking old sessions.
func (repository *UserRepository) UpdateAccess(ctx context.Context, actorID, userID uuid.UUID, update UserAccessUpdate) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize system role edits so concurrent demotions cannot remove all administrators.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", initialAdministratorLockID); err != nil {
		return err
	}
	var allowed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ml_system.users WHERE id=$1 AND enabled AND platform_administrator AND NOT must_change_password)`, actorID.String()).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ErrManagerAccessDenied
	}
	var wasPlatform, wasMetadata, wasEnabled bool
	if err := tx.QueryRow(ctx, `SELECT platform_administrator,metadata_administrator,enabled FROM ml_system.users WHERE id=$1 FOR UPDATE`, userID.String()).Scan(&wasPlatform, &wasMetadata, &wasEnabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if wasPlatform && wasEnabled && (!update.PlatformAdministrator || !update.Enabled) {
		var another bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ml_system.users WHERE id<>$1 AND enabled AND platform_administrator)`, userID.String()).Scan(&another); err != nil {
			return err
		}
		if !another {
			return ErrLastPlatformAdministrator
		}
	}
	if wasPlatform == update.PlatformAdministrator && wasMetadata == update.MetadataAdministrator && wasEnabled == update.Enabled {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE ml_system.users SET platform_administrator=$2,metadata_administrator=$3,enabled=$4,updated_at=clock_timestamp() WHERE id=$1`, userID.String(), update.PlatformAdministrator, update.MetadataAdministrator, update.Enabled); err != nil {
		return err
	}
	if err := revokeUserSessions(ctx, tx, userID.String(), nil); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ml_system.audit_events(level,event_code,user_id,message,details) VALUES('info','user.access_changed',$1,'Internal account access updated',jsonb_build_object('userId',$2::text,'enabled',$3::boolean,'platformAdministrator',$4::boolean,'metadataAdministrator',$5::boolean))`, actorID.String(), userID.String(), update.Enabled, update.PlatformAdministrator, update.MetadataAdministrator); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// List exposes no password hashes, recovery codes or session tokens.
func (repository *UserRepository) List(ctx context.Context) ([]User, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id::text,login,platform_administrator,metadata_administrator,must_change_password,enabled,created_at FROM ml_system.users ORDER BY lower(login),id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		var user User
		var id string
		if err := rows.Scan(&id, &user.Login, &user.PlatformAdministrator, &user.MetadataAdministrator, &user.MustChangePassword, &user.Enabled, &user.CreatedAt); err != nil {
			return nil, err
		}
		user.ID, err = uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		items = append(items, user)
	}
	return items, rows.Err()
}

func (repository *UserRepository) FindID(ctx context.Context, login string) (uuid.UUID, error) {
	var id string
	err := repository.pool.QueryRow(ctx, `SELECT id::text FROM ml_system.users WHERE lower(login)=lower($1) AND enabled`, login).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrInvalidCredentials
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return uuid.Parse(id)
}

func (repository *SessionRepository) SessionDatabaseID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	var databaseID string
	err := repository.pool.QueryRow(ctx, `SELECT database_id::text FROM ml_system.database_sessions WHERE id=$1 AND terminated_at IS NULL`, id.String()).Scan(&databaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrDatabaseAccessDenied
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return uuid.Parse(databaseID)
}
