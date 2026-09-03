// Package systemdb owns the small PostgreSQL database used by ML Service itself.
package systemdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Database is a validated ML System connection pool and its repositories.
type Database struct {
	pool           *pgxpool.Pool
	Settings       *SettingsRepository
	Administrators *AdministratorRepository
	Databases      *DatabaseRepository
	Sessions       *SessionRepository
	Audit          *AuditRepository
	Backups        *BackupRepository
}

// Open connects to PostgreSQL, validates it and applies embedded migrations.
func Open(ctx context.Context, databaseURL string) (*Database, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("ML System PostgreSQL configuration is required")
	}
	configuration, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse ML System PostgreSQL configuration: %w", err)
	}
	return OpenConfig(ctx, configuration)
}

// OpenConfig connects using an in-memory configuration that may contain secrets.
func OpenConfig(ctx context.Context, configuration *pgxpool.Config) (*Database, error) {
	if configuration == nil {
		return nil, fmt.Errorf("ML System PostgreSQL configuration is required")
	}
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		return nil, fmt.Errorf("create ML System PostgreSQL pool: %w", err)
	}
	database := &Database{pool: pool}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping ML System PostgreSQL: %w", err)
	}
	connection, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("acquire ML System migration connection: %w", err)
	}
	err = migrate(ctx, connection.Conn())
	connection.Release()
	if err != nil {
		pool.Close()
		return nil, err
	}
	database.Settings = &SettingsRepository{pool: pool}
	database.Administrators = &AdministratorRepository{pool: pool}
	database.Databases = &DatabaseRepository{pool: pool}
	database.Sessions = &SessionRepository{pool: pool}
	database.Audit = &AuditRepository{pool: pool}
	database.Backups = &BackupRepository{pool: pool}
	return database, nil
}

// Close releases all ML System database connections.
func (database *Database) Close() { database.pool.Close() }

// Ping verifies that ML System PostgreSQL is reachable.
func (database *Database) Ping(ctx context.Context) error {
	if err := database.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping ML System PostgreSQL: %w", err)
	}
	return nil
}
