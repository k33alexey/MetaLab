package systemdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const initialAdministratorLockID int64 = migrationLockID + 1

var (
	// ErrInitialAdministratorExists prevents a second first-run administrator.
	ErrInitialAdministratorExists = errors.New("initial administrator already exists")
	// ErrInvalidCredentials does not disclose whether an account exists.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidRecoveryCode does not disclose whether an account or code exists.
	ErrInvalidRecoveryCode = errors.New("invalid or already used recovery code")
	// ErrUserNotFound indicates an unknown account in a local administrative action.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidLogin indicates a malformed internal account name.
	ErrInvalidLogin = errors.New("invalid administrator login")
)

// Administrator describes the first platform-wide administrator account.
type Administrator struct {
	ID                 uuid.UUID
	Login              string
	MustChangePassword bool
	CreatedAt          time.Time
}

// EmergencyCredentials are displayed once after a local emergency reset.
type EmergencyCredentials struct {
	TemporaryPassword string
	RecoveryCodes     []string
}

// AdministratorRepository owns first-run and emergency administrator operations.
type AdministratorRepository struct {
	pool *pgxpool.Pool
}

// InitialSetupRequired reports whether ML System contains no accounts yet.
func (repository *AdministratorRepository) InitialSetupRequired(ctx context.Context) (bool, error) {
	var exists bool
	if err := repository.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM ml_system.users)").Scan(&exists); err != nil {
		return false, fmt.Errorf("check initial administrator: %w", err)
	}
	return !exists, nil
}

// CreateInitial creates the only first-run administrator and one-time recovery codes.
func (repository *AdministratorRepository) CreateInitial(ctx context.Context, login, password string) (Administrator, []string, error) {
	if err := validateLogin(login); err != nil {
		return Administrator{}, nil, err
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return Administrator{}, nil, err
	}
	codes, digests, err := newRecoveryCodes()
	if err != nil {
		return Administrator{}, nil, err
	}
	id, err := uuid.New()
	if err != nil {
		return Administrator{}, nil, err
	}
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Administrator{}, nil, fmt.Errorf("begin initial administrator creation: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", initialAdministratorLockID); err != nil {
		return Administrator{}, nil, fmt.Errorf("lock initial administrator creation: %w", err)
	}
	var exists bool
	if err := transaction.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM ml_system.users)").Scan(&exists); err != nil {
		return Administrator{}, nil, fmt.Errorf("check existing users: %w", err)
	}
	if exists {
		return Administrator{}, nil, ErrInitialAdministratorExists
	}
	administrator := Administrator{ID: id, Login: login}
	if err := transaction.QueryRow(ctx, `
INSERT INTO ml_system.users(id, login, password_hash, platform_administrator, metadata_administrator)
VALUES ($1, $2, $3, TRUE, TRUE)
RETURNING created_at`, id.String(), login, passwordHash).Scan(&administrator.CreatedAt); err != nil {
		return Administrator{}, nil, fmt.Errorf("create initial administrator: %w", err)
	}
	if err := insertRecoveryCodes(ctx, transaction, id, digests); err != nil {
		return Administrator{}, nil, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Administrator{}, nil, fmt.Errorf("commit initial administrator creation: %w", err)
	}
	return administrator, codes, nil
}

// Authenticate verifies an enabled internal account without revealing lookup details.
func (repository *AdministratorRepository) Authenticate(ctx context.Context, login, password string) (Administrator, error) {
	var administrator Administrator
	var id, passwordHash string
	var enabled, platformAdministrator bool
	err := repository.pool.QueryRow(ctx, `
SELECT id::text, login, password_hash, platform_administrator, must_change_password, enabled, created_at
FROM ml_system.users WHERE lower(login) = lower($1)`, login).Scan(
		&id, &administrator.Login, &passwordHash, &platformAdministrator,
		&administrator.MustChangePassword, &enabled, &administrator.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		auth.SpendPasswordWork(password)
		return Administrator{}, ErrInvalidCredentials
	}
	if err != nil {
		return Administrator{}, fmt.Errorf("read administrator credentials: %w", err)
	}
	if !enabled || !platformAdministrator {
		auth.SpendPasswordWork(password)
		return Administrator{}, ErrInvalidCredentials
	}
	valid, err := auth.VerifyPassword(passwordHash, password)
	if err != nil {
		return Administrator{}, fmt.Errorf("verify administrator password: %w", err)
	}
	if !valid {
		return Administrator{}, ErrInvalidCredentials
	}
	administrator.ID, err = uuid.Parse(id)
	if err != nil {
		return Administrator{}, fmt.Errorf("parse administrator identifier: %w", err)
	}
	return administrator, nil
}

// RecoverPassword consumes one recovery code and replaces the password.
func (repository *AdministratorRepository) RecoverPassword(ctx context.Context, login, code, newPassword string) error {
	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	digest, err := auth.RecoveryCodeDigest(code)
	if err != nil {
		return ErrInvalidRecoveryCode
	}
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin password recovery: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var id string
	err = transaction.QueryRow(ctx, `
UPDATE ml_system.recovery_codes AS codes
SET used_at = clock_timestamp()
FROM ml_system.users AS users
WHERE codes.user_id = users.id AND lower(users.login) = lower($1)
  AND users.enabled AND codes.code_hash = $2 AND codes.used_at IS NULL
RETURNING users.id::text`, login, digest[:]).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidRecoveryCode
	}
	if err != nil {
		return fmt.Errorf("consume recovery code: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
UPDATE ml_system.users
SET password_hash = $2, must_change_password = FALSE, updated_at = clock_timestamp()
WHERE id = $1`, id, passwordHash); err != nil {
		return fmt.Errorf("recover administrator password: %w", err)
	}
	if err := revokeUserSessions(ctx, transaction, id, nil); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit password recovery: %w", err)
	}
	return nil
}

// EmergencyReset creates a temporary password and rotates recovery codes.
// Authorization for this operation belongs to the local OS-only CLI boundary.
func (repository *AdministratorRepository) EmergencyReset(ctx context.Context, login string) (EmergencyCredentials, error) {
	if err := validateLogin(login); err != nil {
		return EmergencyCredentials{}, err
	}
	temporaryPassword, err := auth.GenerateTemporaryPassword()
	if err != nil {
		return EmergencyCredentials{}, err
	}
	passwordHash, err := auth.HashPassword(temporaryPassword)
	if err != nil {
		return EmergencyCredentials{}, err
	}
	codes, digests, err := newRecoveryCodes()
	if err != nil {
		return EmergencyCredentials{}, err
	}
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return EmergencyCredentials{}, fmt.Errorf("begin emergency password reset: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var idText string
	err = transaction.QueryRow(ctx, `
UPDATE ml_system.users SET password_hash = $2, must_change_password = TRUE, updated_at = clock_timestamp()
WHERE lower(login) = lower($1) AND enabled
RETURNING id::text`, login, passwordHash).Scan(&idText)
	if errors.Is(err, pgx.ErrNoRows) {
		return EmergencyCredentials{}, ErrUserNotFound
	}
	if err != nil {
		return EmergencyCredentials{}, fmt.Errorf("reset administrator password: %w", err)
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return EmergencyCredentials{}, fmt.Errorf("parse administrator identifier: %w", err)
	}
	if _, err := transaction.Exec(ctx, "DELETE FROM ml_system.recovery_codes WHERE user_id = $1", id.String()); err != nil {
		return EmergencyCredentials{}, fmt.Errorf("remove old recovery codes: %w", err)
	}
	if err := insertRecoveryCodes(ctx, transaction, id, digests); err != nil {
		return EmergencyCredentials{}, err
	}
	if err := revokeUserSessions(ctx, transaction, idText, nil); err != nil {
		return EmergencyCredentials{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return EmergencyCredentials{}, fmt.Errorf("commit emergency password reset: %w", err)
	}
	return EmergencyCredentials{TemporaryPassword: temporaryPassword, RecoveryCodes: codes}, nil
}

// ChangePassword verifies the current password and clears the mandatory-change flag.
func (repository *AdministratorRepository) ChangePassword(ctx context.Context, login, currentPassword, newPassword string) error {
	return repository.changePassword(ctx, login, currentPassword, newPassword, nil)
}

// ChangePasswordKeepingSession changes the password and atomically revokes every other session.
func (repository *AdministratorRepository) ChangePasswordKeepingSession(ctx context.Context, login, currentPassword, newPassword string, keepSessionID uuid.UUID) error {
	return repository.changePassword(ctx, login, currentPassword, newPassword, &keepSessionID)
}

func (repository *AdministratorRepository) changePassword(ctx context.Context, login, currentPassword, newPassword string, keepSessionID *uuid.UUID) error {
	administrator, err := repository.Authenticate(ctx, login, currentPassword)
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
WHERE id = $1 AND enabled`, administrator.ID.String(), passwordHash)
	if err != nil {
		return fmt.Errorf("change administrator password: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrInvalidCredentials
	}
	if err := revokeUserSessions(ctx, transaction, administrator.ID.String(), keepSessionID); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}

func revokeUserSessions(ctx context.Context, transaction pgx.Tx, userID string, keepSessionID *uuid.UUID) error {
	var keep any
	if keepSessionID != nil {
		keep = keepSessionID.String()
	}
	if _, err := transaction.Exec(ctx, `
UPDATE ml_system.database_sessions SET terminated_at = clock_timestamp()
WHERE terminated_at IS NULL AND portal_session_id IN (
    SELECT id FROM ml_system.portal_sessions
    WHERE user_id = $1 AND ($2::uuid IS NULL OR id <> $2)
)`, userID, keep); err != nil {
		return fmt.Errorf("terminate sessions after password change: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
UPDATE ml_system.portal_sessions SET revoked_at = clock_timestamp()
WHERE user_id = $1 AND revoked_at IS NULL AND ($2::uuid IS NULL OR id <> $2)`, userID, keep); err != nil {
		return fmt.Errorf("revoke sessions after password change: %w", err)
	}
	return nil
}

func validateLogin(login string) error {
	if login == "" || login != strings.TrimSpace(login) || !utf8.ValidString(login) || utf8.RuneCountInString(login) > 128 {
		return ErrInvalidLogin
	}
	for _, symbol := range login {
		if unicode.IsControl(symbol) {
			return ErrInvalidLogin
		}
	}
	return nil
}

func newRecoveryCodes() ([]string, [][]byte, error) {
	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		return nil, nil, err
	}
	digests := make([][]byte, len(codes))
	for index, code := range codes {
		digest, err := auth.RecoveryCodeDigest(code)
		if err != nil {
			return nil, nil, err
		}
		digests[index] = append([]byte(nil), digest[:]...)
	}
	return codes, digests, nil
}

func insertRecoveryCodes(ctx context.Context, transaction pgx.Tx, id uuid.UUID, digests [][]byte) error {
	batch := &pgx.Batch{}
	for _, digest := range digests {
		batch.Queue("INSERT INTO ml_system.recovery_codes(user_id, code_hash) VALUES ($1, $2)", id.String(), digest)
	}
	results := transaction.SendBatch(ctx, batch)
	for range digests {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("create recovery codes: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("finish recovery codes: %w", err)
	}
	return nil
}
