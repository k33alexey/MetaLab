package systemdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// User is an enabled or disabled internal ML account with system-wide flags.
type User struct {
	ID                    uuid.UUID
	Login                 string
	PlatformAdministrator bool
	MetadataAdministrator bool
	MustChangePassword    bool
	Enabled               bool
	CreatedAt             time.Time
}

// UserCreation contains validated fields for a new internal account.
type UserCreation struct {
	ID                    uuid.UUID
	Login                 string
	Password              string
	PlatformAdministrator bool
	MetadataAdministrator bool
}

// UserRepository owns authentication shared by Portal users and administrators.
type UserRepository struct{ pool *pgxpool.Pool }

// Create adds an enabled internal account. Authorization belongs to the caller.
func (repository *UserRepository) Create(ctx context.Context, creation UserCreation) (User, error) {
	if creation.ID.IsZero() {
		return User{}, fmt.Errorf("user identifier is required")
	}
	if err := validateLogin(creation.Login); err != nil {
		return User{}, err
	}
	passwordHash, err := auth.HashPassword(creation.Password)
	if err != nil {
		return User{}, err
	}
	user := User{
		ID: creation.ID, Login: creation.Login, PlatformAdministrator: creation.PlatformAdministrator,
		MetadataAdministrator: creation.MetadataAdministrator, Enabled: true,
	}
	err = repository.pool.QueryRow(ctx, `
INSERT INTO ml_system.users(id, login, password_hash, platform_administrator, metadata_administrator)
VALUES ($1, $2, $3, $4, $5)
RETURNING created_at`, creation.ID.String(), creation.Login, passwordHash,
		creation.PlatformAdministrator, creation.MetadataAdministrator).Scan(&user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("create internal user: %w", err)
	}
	return user, nil
}

// Authenticate verifies any enabled internal account without disclosing lookup details.
func (repository *UserRepository) Authenticate(ctx context.Context, login, password string) (User, error) {
	return authenticateInternalUser(ctx, repository.pool, login, password, false)
}

// ChangePasswordKeepingSession changes a user's password and revokes every other session.
func (repository *UserRepository) ChangePasswordKeepingSession(
	ctx context.Context, login, currentPassword, newPassword string, keepSessionID uuid.UUID,
) error {
	user, err := repository.Authenticate(ctx, login, currentPassword)
	if err != nil {
		return err
	}
	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	result, err := transaction.Exec(ctx, `
UPDATE ml_system.users
SET password_hash = $2, must_change_password = FALSE, updated_at = clock_timestamp()
WHERE id = $1 AND enabled`, user.ID.String(), passwordHash)
	if err != nil {
		return fmt.Errorf("change user password: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrInvalidCredentials
	}
	if err := revokeUserSessions(ctx, transaction, user.ID.String(), &keepSessionID); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}

func authenticateInternalUser(
	ctx context.Context, pool *pgxpool.Pool, login, password string, requirePlatformAdministrator bool,
) (User, error) {
	var user User
	var id, passwordHash string
	err := pool.QueryRow(ctx, `
SELECT id::text, login, password_hash, platform_administrator, metadata_administrator,
       must_change_password, enabled, created_at
FROM ml_system.users WHERE lower(login) = lower($1)`, login).Scan(
		&id, &user.Login, &passwordHash, &user.PlatformAdministrator, &user.MetadataAdministrator,
		&user.MustChangePassword, &user.Enabled, &user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		auth.SpendPasswordWork(password)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("read user credentials: %w", err)
	}
	if !user.Enabled || requirePlatformAdministrator && !user.PlatformAdministrator {
		auth.SpendPasswordWork(password)
		return User{}, ErrInvalidCredentials
	}
	valid, err := auth.VerifyPassword(passwordHash, password)
	if err != nil {
		return User{}, fmt.Errorf("verify user password: %w", err)
	}
	if !valid {
		return User{}, ErrInvalidCredentials
	}
	user.ID, err = uuid.Parse(id)
	if err != nil {
		return User{}, fmt.Errorf("parse user identifier: %w", err)
	}
	return user, nil
}
