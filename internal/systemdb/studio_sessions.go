package systemdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const studioSessionLifetime = 30 * time.Second

var (
	ErrStudioSessionLocked   = errors.New("ML Studio is already open for this database")
	ErrStudioSessionNotFound = errors.New("ML Studio session not found or terminated")
)

// StudioSession is the exclusive, renewable editor lease for one registered database.
type StudioSession struct {
	ID         uuid.UUID `json:"id"`
	DatabaseID uuid.UUID `json:"databaseId"`
	ProjectID  uuid.UUID `json:"projectId"`
	OwnerName  string    `json:"ownerName"`
	HostName   string    `json:"hostName"`
	ProcessID  int64     `json:"processId"`
	StartedAt  time.Time `json:"startedAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// StudioLockError reports the current lease holder without exposing its token.
type StudioLockError struct{ Session StudioSession }

func (locked *StudioLockError) Error() string {
	return fmt.Sprintf("%s: %s on %s", ErrStudioSessionLocked, locked.Session.OwnerName, locked.Session.HostName)
}

func (locked *StudioLockError) Unwrap() error { return ErrStudioSessionLocked }

// StudioSessionRepository persists exclusive leases independently of Manager lifetime.
type StudioSessionRepository struct{ pool *pgxpool.Pool }

// Acquire creates a lease or atomically replaces an expired/terminated lease.
func (repository *StudioSessionRepository) Acquire(
	ctx context.Context, databaseID, projectID, id uuid.UUID, tokenHash []byte, ownerName, hostName string, processID int64,
) (StudioSession, error) {
	ownerName, hostName = strings.TrimSpace(ownerName), strings.TrimSpace(hostName)
	if databaseID.IsZero() || projectID.IsZero() || id.IsZero() || len(tokenHash) != 32 || ownerName == "" || hostName == "" || processID <= 0 {
		return StudioSession{}, fmt.Errorf("invalid ML Studio session")
	}
	ownerName, hostName = boundedText(ownerName, 256), boundedText(hostName, 256)
	session, err := scanStudioSession(repository.pool.QueryRow(ctx, `
INSERT INTO ml_system.studio_sessions(
    database_id, id, project_id, token_hash, owner_name, host_name, process_id, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, clock_timestamp() + $8::interval)
ON CONFLICT (database_id) DO UPDATE SET
    id = EXCLUDED.id, project_id = EXCLUDED.project_id, token_hash = EXCLUDED.token_hash,
    owner_name = EXCLUDED.owner_name, host_name = EXCLUDED.host_name, process_id = EXCLUDED.process_id,
    started_at = clock_timestamp(), last_seen_at = clock_timestamp(),
    expires_at = clock_timestamp() + $8::interval, terminated_at = NULL
WHERE ml_system.studio_sessions.terminated_at IS NOT NULL
   OR ml_system.studio_sessions.expires_at <= clock_timestamp()
RETURNING id::text, database_id::text, project_id::text, owner_name, host_name, process_id,
          started_at, last_seen_at, expires_at`,
		databaseID.String(), id.String(), projectID.String(), tokenHash, ownerName, hostName, processID, studioSessionLifetime.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		current, currentErr := repository.GetActive(ctx, databaseID)
		if currentErr != nil {
			return StudioSession{}, errors.Join(ErrStudioSessionLocked, currentErr)
		}
		return StudioSession{}, &StudioLockError{Session: current}
	}
	if err != nil {
		return StudioSession{}, fmt.Errorf("acquire ML Studio session: %w", err)
	}
	return session, nil
}

// Heartbeat renews only the lease that owns the matching opaque-token digest.
func (repository *StudioSessionRepository) Heartbeat(ctx context.Context, id uuid.UUID, tokenHash []byte, processID int64) (StudioSession, error) {
	if processID <= 0 {
		return StudioSession{}, fmt.Errorf("invalid ML Studio process identifier")
	}
	session, err := scanStudioSession(repository.pool.QueryRow(ctx, `
UPDATE ml_system.studio_sessions
SET last_seen_at = clock_timestamp(), expires_at = clock_timestamp() + $3::interval, process_id = $4
WHERE id = $1 AND token_hash = $2 AND terminated_at IS NULL AND expires_at > clock_timestamp()
RETURNING id::text, database_id::text, project_id::text, owner_name, host_name, process_id,
          started_at, last_seen_at, expires_at`, id.String(), tokenHash, studioSessionLifetime.String(), processID))
	if errors.Is(err, pgx.ErrNoRows) {
		return StudioSession{}, ErrStudioSessionNotFound
	}
	if err != nil {
		return StudioSession{}, fmt.Errorf("renew ML Studio session: %w", err)
	}
	return session, nil
}

// Release terminates the matching owned lease.
func (repository *StudioSessionRepository) Release(ctx context.Context, id uuid.UUID, tokenHash []byte) error {
	result, err := repository.pool.Exec(ctx, `
UPDATE ml_system.studio_sessions SET terminated_at = clock_timestamp()
WHERE id = $1 AND token_hash = $2 AND terminated_at IS NULL`, id.String(), tokenHash)
	if err != nil {
		return fmt.Errorf("release ML Studio session: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrStudioSessionNotFound
	}
	return nil
}

// Terminate forcibly ends the active lease for a database.
func (repository *StudioSessionRepository) Terminate(ctx context.Context, databaseID uuid.UUID) error {
	result, err := repository.pool.Exec(ctx, `
UPDATE ml_system.studio_sessions SET terminated_at = clock_timestamp()
WHERE database_id = $1 AND terminated_at IS NULL AND expires_at > clock_timestamp()`, databaseID.String())
	if err != nil {
		return fmt.Errorf("terminate ML Studio session: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrStudioSessionNotFound
	}
	return nil
}

// GetActive returns the current unexpired lease for a database.
func (repository *StudioSessionRepository) GetActive(ctx context.Context, databaseID uuid.UUID) (StudioSession, error) {
	session, err := scanStudioSession(repository.pool.QueryRow(ctx, `
SELECT id::text, database_id::text, project_id::text, owner_name, host_name, process_id,
       started_at, last_seen_at, expires_at
FROM ml_system.studio_sessions
WHERE database_id = $1 AND terminated_at IS NULL AND expires_at > clock_timestamp()`, databaseID.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return StudioSession{}, ErrStudioSessionNotFound
	}
	if err != nil {
		return StudioSession{}, fmt.Errorf("read ML Studio session: %w", err)
	}
	return session, nil
}

// ListActive returns current leases newest first.
func (repository *StudioSessionRepository) ListActive(ctx context.Context) ([]StudioSession, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT id::text, database_id::text, project_id::text, owner_name, host_name, process_id,
       started_at, last_seen_at, expires_at
FROM ml_system.studio_sessions
WHERE terminated_at IS NULL AND expires_at > clock_timestamp()
ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list ML Studio sessions: %w", err)
	}
	defer rows.Close()
	items := make([]StudioSession, 0)
	for rows.Next() {
		item, scanErr := scanStudioSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ML Studio sessions: %w", err)
	}
	return items, nil
}

func scanStudioSession(row rowScanner) (StudioSession, error) {
	var session StudioSession
	var id, databaseID, projectID string
	if err := row.Scan(&id, &databaseID, &projectID, &session.OwnerName, &session.HostName, &session.ProcessID,
		&session.StartedAt, &session.LastSeenAt, &session.ExpiresAt); err != nil {
		return StudioSession{}, err
	}
	var err error
	session.ID, err = uuid.Parse(id)
	if err == nil {
		session.DatabaseID, err = uuid.Parse(databaseID)
	}
	if err == nil {
		session.ProjectID, err = uuid.Parse(projectID)
	}
	if err != nil {
		return StudioSession{}, fmt.Errorf("parse ML Studio session identity: %w", err)
	}
	return session, nil
}
