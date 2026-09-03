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

// DatabaseMode separates production operation from isolated development and debugging.
type DatabaseMode string

// DatabaseHealthStatus is the latest verified connection state.
type DatabaseHealthStatus string

const (
	DatabaseStopped     DatabaseState = "stopped"
	DatabaseStarting    DatabaseState = "starting"
	DatabaseRunning     DatabaseState = "running"
	DatabaseStopping    DatabaseState = "stopping"
	DatabaseMaintenance DatabaseState = "maintenance"
	DatabaseError       DatabaseState = "error"
)

const (
	DatabasePrimary DatabaseMode = "primary"
	DatabaseDebug   DatabaseMode = "debug"
)

const (
	DatabaseHealthUnknown   DatabaseHealthStatus = "unknown"
	DatabaseHealthHealthy   DatabaseHealthStatus = "healthy"
	DatabaseHealthUnhealthy DatabaseHealthStatus = "unhealthy"
)

var (
	ErrDatabaseNotFound         = errors.New("registered database not found")
	ErrDatabaseNameExists       = errors.New("database name is already registered")
	ErrPhysicalDatabaseExists   = errors.New("physical PostgreSQL database is already registered")
	ErrDatabaseStateConflict    = errors.New("registered database state changed concurrently")
	ErrDatabaseCannotUnregister = errors.New("only a stopped database can be unregistered")
	ErrDatabaseHasDebugCopies   = errors.New("database has registered debug copies")
	ErrDatabaseHasBackups       = errors.New("database has registered backups")
)

// RegisteredDatabase is a non-secret ML System registry entry.
type RegisteredDatabase struct {
	ID               uuid.UUID               `json:"id"`
	Name             string                  `json:"name"`
	PhysicalID       uuid.UUID               `json:"physicalId"`
	Connection       postgresconn.Descriptor `json:"connection"`
	Mode             DatabaseMode            `json:"mode"`
	SourceDatabaseID *uuid.UUID              `json:"sourceDatabaseId,omitempty"`
	AllowNewSessions bool                    `json:"allowNewSessions"`
	HealthStatus     DatabaseHealthStatus    `json:"healthStatus"`
	HealthMessage    string                  `json:"healthMessage,omitempty"`
	HealthCheckedAt  *time.Time              `json:"healthCheckedAt,omitempty"`
	State            DatabaseState           `json:"state"`
	StateRevision    int64                   `json:"stateRevision"`
	LastError        string                  `json:"lastError,omitempty"`
	StateChangedAt   time.Time               `json:"stateChangedAt"`
	CreatedAt        time.Time               `json:"createdAt"`
	UpdatedAt        time.Time               `json:"updatedAt"`
}

// DatabaseRegistration contains a validated new registry entry.
type DatabaseRegistration struct {
	ID               uuid.UUID
	Name             string
	PhysicalID       uuid.UUID
	Connection       postgresconn.Descriptor
	Mode             DatabaseMode
	SourceDatabaseID *uuid.UUID
}

// DatabaseCapabilities are mandatory platform restrictions derived from mode.
type DatabaseCapabilities struct {
	Debugging                     bool `json:"debugging"`
	AutomaticScheduledJobs        bool `json:"automaticScheduledJobs"`
	ExternalIntegrations          bool `json:"externalIntegrations"`
	Equipment                     bool `json:"equipment"`
	ManualJobRequiresConfirmation bool `json:"manualJobRequiresConfirmation"`
}

// Capabilities returns restrictions that cannot be weakened by project code.
func (database RegisteredDatabase) Capabilities() DatabaseCapabilities {
	if database.Mode == DatabaseDebug {
		return DatabaseCapabilities{Debugging: true, ManualJobRequiresConfirmation: true}
	}
	return DatabaseCapabilities{AutomaticScheduledJobs: true, ExternalIntegrations: true, Equipment: true}
}

// DatabaseRepository persists the authoritative registry of application databases.
type DatabaseRepository struct {
	pool *pgxpool.Pool
}

// Register creates a stopped registry entry and enforces physical uniqueness in PostgreSQL.
func (repository *DatabaseRepository) Register(ctx context.Context, registration DatabaseRegistration) (RegisteredDatabase, error) {
	if registration.Mode == "" {
		registration.Mode = DatabasePrimary
	}
	if err := validateDatabaseRegistration(registration); err != nil {
		return RegisteredDatabase{}, err
	}
	item, err := scanRegisteredDatabase(repository.pool.QueryRow(ctx, `
INSERT INTO ml_system.databases(
    id, name, physical_id, host, port, database_name, username, ssl_mode, secret_key, mode, source_database_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
          mode, COALESCE(source_database_id::text, ''),
          allow_new_sessions, health_status, COALESCE(health_message, ''), health_checked_at,
          state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at`,
		registration.ID.String(), registration.Name, registration.PhysicalID.String(),
		registration.Connection.Host, registration.Connection.Port, registration.Connection.Database,
		registration.Connection.User, registration.Connection.SSLMode, registration.Connection.SecretKey,
		registration.Mode, optionalUUIDText(registration.SourceDatabaseID),
	))
	if err != nil {
		return RegisteredDatabase{}, mapDatabaseConstraintError(err)
	}
	return item, nil
}

// List returns all entries in stable display-name order.
func (repository *DatabaseRepository) List(ctx context.Context) ([]RegisteredDatabase, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
       mode, COALESCE(source_database_id::text, ''),
       allow_new_sessions, health_status, COALESCE(health_message, ''), health_checked_at,
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
       mode, COALESCE(source_database_id::text, ''),
       allow_new_sessions, health_status, COALESCE(health_message, ''), health_checked_at,
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
          mode, COALESCE(source_database_id::text, ''),
          allow_new_sessions, health_status, COALESCE(health_message, ''), health_checked_at,
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
          mode, COALESCE(source_database_id::text, ''),
          allow_new_sessions, health_status, COALESCE(health_message, ''), health_checked_at,
          state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at`, id.String())
	item, err := scanRegisteredDatabase(row)
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23503" {
		switch postgresError.ConstraintName {
		case "databases_source_database_id_fkey":
			return RegisteredDatabase{}, ErrDatabaseHasDebugCopies
		case "backups_database_id_fkey":
			return RegisteredDatabase{}, ErrDatabaseHasBackups
		}
	}
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
	var id, physicalID, sourceDatabaseID string
	err := row.Scan(
		&id, &item.Name, &physicalID, &item.Connection.Host, &item.Connection.Port,
		&item.Connection.Database, &item.Connection.User, &item.Connection.SSLMode, &item.Connection.SecretKey,
		&item.Mode, &sourceDatabaseID,
		&item.AllowNewSessions, &item.HealthStatus, &item.HealthMessage, &item.HealthCheckedAt,
		&item.State, &item.StateRevision, &item.LastError, &item.StateChangedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return RegisteredDatabase{}, err
	}
	if err := parseDatabaseIDs(&item, id, physicalID, sourceDatabaseID); err != nil {
		return RegisteredDatabase{}, err
	}
	return item, nil
}

// SetSessionAccess enables or blocks creation of new sessions without disconnecting existing ones.
func (repository *DatabaseRepository) SetSessionAccess(ctx context.Context, id uuid.UUID, allowed bool) (RegisteredDatabase, error) {
	if id.IsZero() {
		return RegisteredDatabase{}, fmt.Errorf("registered database identifier is required")
	}
	item, err := scanRegisteredDatabase(repository.pool.QueryRow(ctx, `
UPDATE ml_system.databases
SET allow_new_sessions = $2, updated_at = clock_timestamp()
WHERE id = $1
RETURNING id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
          mode, COALESCE(source_database_id::text, ''),
          allow_new_sessions, health_status, COALESCE(health_message, ''), health_checked_at,
          state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at`, id.String(), allowed))
	if errors.Is(err, pgx.ErrNoRows) {
		return RegisteredDatabase{}, ErrDatabaseNotFound
	}
	return item, err
}

// RecordHealth persists a bounded, non-secret result of a connection check.
func (repository *DatabaseRepository) RecordHealth(ctx context.Context, id uuid.UUID, healthy bool, message string) (RegisteredDatabase, error) {
	if id.IsZero() {
		return RegisteredDatabase{}, fmt.Errorf("registered database identifier is required")
	}
	status := DatabaseHealthUnhealthy
	if healthy {
		status, message = DatabaseHealthHealthy, ""
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > 1000 {
		message = string([]rune(message)[:1000])
	}
	item, err := scanRegisteredDatabase(repository.pool.QueryRow(ctx, `
UPDATE ml_system.databases
SET health_status = $2, health_message = NULLIF($3, ''), health_checked_at = clock_timestamp(), updated_at = clock_timestamp()
WHERE id = $1
RETURNING id::text, name, physical_id::text, host, port, database_name, username, ssl_mode, secret_key,
          mode, COALESCE(source_database_id::text, ''),
          allow_new_sessions, health_status, COALESCE(health_message, ''), health_checked_at,
          state, state_revision, COALESCE(last_error, ''), state_changed_at, created_at, updated_at`, id.String(), status, message))
	if errors.Is(err, pgx.ErrNoRows) {
		return RegisteredDatabase{}, ErrDatabaseNotFound
	}
	return item, err
}

func parseDatabaseIDs(item *RegisteredDatabase, id, physicalID, sourceDatabaseID string) error {
	var err error
	item.ID, err = uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("parse registered database identifier: %w", err)
	}
	item.PhysicalID, err = uuid.Parse(physicalID)
	if err != nil {
		return fmt.Errorf("parse physical database identifier: %w", err)
	}
	if sourceDatabaseID != "" {
		source, parseErr := uuid.Parse(sourceDatabaseID)
		if parseErr != nil {
			return fmt.Errorf("parse source database identifier: %w", parseErr)
		}
		item.SourceDatabaseID = &source
	}
	return nil
}

func validateDatabaseRegistration(registration DatabaseRegistration) error {
	if registration.ID.IsZero() || registration.PhysicalID.IsZero() {
		return fmt.Errorf("database and physical identifiers are required")
	}
	if err := ValidateDatabaseName(registration.Name); err != nil {
		return err
	}
	if err := registration.Connection.Validate(); err != nil {
		return err
	}
	if registration.Mode != DatabasePrimary && registration.Mode != DatabaseDebug {
		return fmt.Errorf("invalid database mode %q", registration.Mode)
	}
	if registration.Mode == DatabasePrimary && registration.SourceDatabaseID != nil {
		return fmt.Errorf("primary database cannot have a source database")
	}
	if registration.SourceDatabaseID != nil && *registration.SourceDatabaseID == registration.ID {
		return fmt.Errorf("database cannot use itself as source")
	}
	return nil
}

// ValidateDatabaseName checks a user-facing registry name before expensive provisioning starts.
func ValidateDatabaseName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || utf8.RuneCountInString(name) > 128 {
		return fmt.Errorf("invalid registered database name %q", name)
	}
	for _, symbol := range name {
		if unicode.IsControl(symbol) {
			return fmt.Errorf("invalid registered database name %q", name)
		}
	}
	return nil
}

func optionalUUIDText(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
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
