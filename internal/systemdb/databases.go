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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// DatabaseState is the persisted lifecycle state observed by ML Service.
type DatabaseState string

const (
	DatabaseStopped     DatabaseState = "stopped"
	DatabaseStarting    DatabaseState = "starting"
	DatabaseRunning     DatabaseState = "running"
	DatabaseStopping    DatabaseState = "stopping"
	DatabaseMaintenance DatabaseState = "maintenance"
	DatabaseError       DatabaseState = "error"
)

var (
	ErrDatabaseNotFound         = errors.New("registered database not found")
	ErrDatabaseNameExists       = errors.New("database name is already registered")
	ErrPhysicalDatabaseExists   = errors.New("physical PostgreSQL database is already registered")
	ErrDatabaseStateConflict    = errors.New("registered database state changed concurrently")
	ErrDatabaseCannotUnregister = errors.New("only a stopped database can be unregistered")
)

// RegisteredDatabase is a non-secret ML System registry entry.
type RegisteredDatabase struct {
	ID             uuid.UUID               `json:"id"`
	Name           string                  `json:"name"`
	PhysicalID     uuid.UUID               `json:"physicalId"`
	Connection     postgresconn.Descriptor `json:"connection"`
	State          DatabaseState           `json:"state"`
	StateRevision  int64                   `json:"stateRevision"`
	LastError      string                  `json:"lastError,omitempty"`
	StateChangedAt time.Time               `json:"stateChangedAt"`
	CreatedAt      time.Time               `json:"createdAt"`
	UpdatedAt      time.Time               `json:"updatedAt"`
}

// DatabaseRegistration contains a validated new registry entry.
type DatabaseRegistration struct {
	ID         uuid.UUID
	Name       string
	PhysicalID uuid.UUID
	Connection postgresconn.Descriptor
}

// DatabaseRepository persists the authoritative registry of application databases.
type DatabaseRepository struct {
	pool *pgxpool.Pool
}

// Register creates a stopped registry entry and enforces physical uniqueness in PostgreSQL.
func (repository *DatabaseRepository) Register(ctx context.Context, registration DatabaseRegistration) (RegisteredDatabase, error) {
	if err := validateDatabaseRegistration(registration); err != nil {
		return RegisteredDatabase{}, err
	}
	var item RegisteredDatabase
	var id, physicalID string
	err := repository.pool.QueryRow(ctx, `
INSERT INTO ml_system.databases(
    id, name, physical_id, host, port, database_name, username, ssl_mode, secret_key
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
          state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at`,
		registration.ID.String(), registration.Name, registration.PhysicalID.String(),
		registration.Connection.Host, registration.Connection.Port, registration.Connection.Database,
		registration.Connection.User, registration.Connection.SSLMode, registration.Connection.SecretKey,
	).Scan(
		&id, &item.Name, &physicalID, &item.Connection.Host, &item.Connection.Port,
		&item.Connection.Database, &item.Connection.User, &item.Connection.SSLMode, &item.Connection.SecretKey,
		&item.State, &item.StateRevision, &item.LastError, &item.StateChangedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return RegisteredDatabase{}, mapDatabaseConstraintError(err)
	}
	if err := parseDatabaseIDs(&item, id, physicalID); err != nil {
		return RegisteredDatabase{}, err
	}
	return item, nil
}

// List returns all entries in stable display-name order.
func (repository *DatabaseRepository) List(ctx context.Context) ([]RegisteredDatabase, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
       state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at
FROM ml_system.databases
ORDER BY lower(name), id`)
	if err != nil {
		return nil, fmt.Errorf("list registered databases: %w", err)
	}
	defer rows.Close()
	items := make([]RegisteredDatabase, 0)
	for rows.Next() {
		item, scanErr := scanRegisteredDatabase(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate registered databases: %w", err)
	}
	return items, nil
}

// Get returns one entry by stable identifier.
func (repository *DatabaseRepository) Get(ctx context.Context, id uuid.UUID) (RegisteredDatabase, error) {
	if id.IsZero() {
		return RegisteredDatabase{}, fmt.Errorf("registered database identifier is required")
	}
	row := repository.pool.QueryRow(ctx, `
SELECT id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
       state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at
FROM ml_system.databases WHERE id = $1`, id.String())
	item, err := scanRegisteredDatabase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return RegisteredDatabase{}, ErrDatabaseNotFound
	}
	return item, err
}

// Transition changes state with optimistic concurrency and a strict transition graph.
func (repository *DatabaseRepository) Transition(ctx context.Context, id uuid.UUID, expected DatabaseState, expectedRevision int64, next DatabaseState, message string) (RegisteredDatabase, error) {
	if id.IsZero() || expectedRevision <= 0 || !validStateTransition(expected, next) {
		return RegisteredDatabase{}, fmt.Errorf("invalid database state transition %q -> %q", expected, next)
	}
	if next != DatabaseError {
		message = ""
	} else if strings.TrimSpace(message) == "" {
		return RegisteredDatabase{}, fmt.Errorf("database error state requires a message")
	}
	row := repository.pool.QueryRow(ctx, `
UPDATE ml_system.databases
SET state = $4, state_revision = state_revision + 1, last_error = NULLIF($5, ''),
    state_changed_at = clock_timestamp(), updated_at = clock_timestamp()
WHERE id = $1 AND state = $2 AND state_revision = $3
RETURNING id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
          state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at`,
		id.String(), expected, expectedRevision, next, message)
	item, err := scanRegisteredDatabase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		_, getErr := repository.Get(ctx, id)
		if errors.Is(getErr, ErrDatabaseNotFound) {
			return RegisteredDatabase{}, ErrDatabaseNotFound
		}
		if getErr != nil {
			return RegisteredDatabase{}, getErr
		}
		return RegisteredDatabase{}, ErrDatabaseStateConflict
	}
	return item, err
}

// Unregister removes a stopped entry and returns it for protected-secret cleanup.
func (repository *DatabaseRepository) Unregister(ctx context.Context, id uuid.UUID) (RegisteredDatabase, error) {
	if id.IsZero() {
		return RegisteredDatabase{}, fmt.Errorf("registered database identifier is required")
	}
	row := repository.pool.QueryRow(ctx, `
DELETE FROM ml_system.databases WHERE id = $1 AND state = 'stopped'
RETURNING id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
          state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at`, id.String())
	item, err := scanRegisteredDatabase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		current, getErr := repository.Get(ctx, id)
		if errors.Is(getErr, ErrDatabaseNotFound) {
			return RegisteredDatabase{}, ErrDatabaseNotFound
		}
		if getErr != nil {
			return RegisteredDatabase{}, getErr
		}
		if current.State != DatabaseStopped {
			return RegisteredDatabase{}, ErrDatabaseCannotUnregister
		}
		return RegisteredDatabase{}, ErrDatabaseStateConflict
	}
	return item, err
}

type rowScanner interface {
	Scan(...any) error
}

func scanRegisteredDatabase(row rowScanner) (RegisteredDatabase, error) {
	var item RegisteredDatabase
	var id, physicalID string
	err := row.Scan(
		&id, &item.Name, &physicalID, &item.Connection.Host, &item.Connection.Port,
		&item.Connection.Database, &item.Connection.User, &item.Connection.SSLMode, &item.Connection.SecretKey,
		&item.State, &item.StateRevision, &item.LastError, &item.StateChangedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return RegisteredDatabase{}, err
	}
	if err := parseDatabaseIDs(&item, id, physicalID); err != nil {
		return RegisteredDatabase{}, err
	}
	return item, nil
}

func parseDatabaseIDs(item *RegisteredDatabase, id, physicalID string) error {
	var err error
	item.ID, err = uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("parse registered database identifier: %w", err)
	}
	item.PhysicalID, err = uuid.Parse(physicalID)
	if err != nil {
		return fmt.Errorf("parse physical database identifier: %w", err)
	}
	return nil
}

func validateDatabaseRegistration(registration DatabaseRegistration) error {
	if registration.ID.IsZero() || registration.PhysicalID.IsZero() {
		return fmt.Errorf("database and physical identifiers are required")
	}
	if registration.Name == "" || registration.Name != strings.TrimSpace(registration.Name) || utf8.RuneCountInString(registration.Name) > 128 {
		return fmt.Errorf("invalid registered database name %q", registration.Name)
	}
	for _, symbol := range registration.Name {
		if unicode.IsControl(symbol) {
			return fmt.Errorf("invalid registered database name %q", registration.Name)
		}
	}
	if err := registration.Connection.Validate(); err != nil {
		return err
	}
	return nil
}

func mapDatabaseConstraintError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23505" {
		return fmt.Errorf("register database: %w", err)
	}
	switch postgresError.ConstraintName {
	case "databases_name_case_insensitive_uq":
		return ErrDatabaseNameExists
	case "databases_physical_id_uq":
		return ErrPhysicalDatabaseExists
	default:
		return fmt.Errorf("register database: %w", err)
	}
}

func validStateTransition(from, to DatabaseState) bool {
	switch from {
	case DatabaseStopped:
		return to == DatabaseStarting || to == DatabaseMaintenance
	case DatabaseStarting:
		return to == DatabaseRunning || to == DatabaseStopped || to == DatabaseError
	case DatabaseRunning:
		return to == DatabaseStopping || to == DatabaseError
	case DatabaseStopping:
		return to == DatabaseStopped || to == DatabaseError
	case DatabaseMaintenance:
		return to == DatabaseStopped || to == DatabaseStarting || to == DatabaseError
	case DatabaseError:
		return to == DatabaseStopped || to == DatabaseMaintenance
	default:
		return false
	}
}
