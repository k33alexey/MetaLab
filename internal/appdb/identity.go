// Package appdb owns the small platform marker stored in every application database.
package appdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const identityLockID int64 = 0x4d4c415050494445 // "MLAPPIDE"

// ErrSystemDatabase prevents registering ML System as an application database.
var ErrSystemDatabase = errors.New("ML System cannot be registered as an application database")

// EnsureIdentity atomically creates or reads the stable identity carried by a physical database.
func EnsureIdentity(ctx context.Context, descriptor postgresconn.Descriptor, password string) (uuid.UUID, error) {
	pool, err := open(ctx, descriptor, password)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer pool.Close()
	transaction, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("begin application database identity: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", identityLockID); err != nil {
		return uuid.UUID{}, fmt.Errorf("lock application database identity: %w", err)
	}
	var systemDatabase bool
	if err := transaction.QueryRow(ctx, "SELECT to_regclass('ml_system.schema_migrations') IS NOT NULL").Scan(&systemDatabase); err != nil {
		return uuid.UUID{}, fmt.Errorf("inspect application database: %w", err)
	}
	if systemDatabase {
		return uuid.UUID{}, ErrSystemDatabase
	}
	if _, err := transaction.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS ml_core;
CREATE TABLE IF NOT EXISTS ml_core.database_identity (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    id UUID NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
)`); err != nil {
		return uuid.UUID{}, fmt.Errorf("initialize application database identity: %w", err)
	}
	candidate, err := uuid.New()
	if err != nil {
		return uuid.UUID{}, err
	}
	if _, err := transaction.Exec(ctx, `
INSERT INTO ml_core.database_identity(singleton, id)
VALUES (TRUE, $1)
ON CONFLICT (singleton) DO NOTHING`, candidate.String()); err != nil {
		return uuid.UUID{}, fmt.Errorf("create application database identity: %w", err)
	}
	var identityText, actualDatabase, encoding string
	err = transaction.QueryRow(ctx, `
SELECT identity.id::text, current_database(), current_setting('server_encoding')
FROM ml_core.database_identity AS identity
WHERE identity.singleton`).Scan(&identityText, &actualDatabase, &encoding)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, fmt.Errorf("application database identity is missing")
	}
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("read application database identity: %w", err)
	}
	if actualDatabase != descriptor.Database {
		return uuid.UUID{}, fmt.Errorf("connected to PostgreSQL database %q instead of %q", actualDatabase, descriptor.Database)
	}
	if encoding != "UTF8" {
		return uuid.UUID{}, fmt.Errorf("application PostgreSQL encoding must be UTF8, got %s", encoding)
	}
	identity, err := uuid.Parse(identityText)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse application database identity: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return uuid.UUID{}, fmt.Errorf("commit application database identity: %w", err)
	}
	return identity, nil
}

// ResetIdentity gives a newly restored debug copy a new physical identity.
func ResetIdentity(ctx context.Context, descriptor postgresconn.Descriptor, password string) (uuid.UUID, error) {
	pool, err := open(ctx, descriptor, password)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer pool.Close()
	transaction, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("begin reset application database identity: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", identityLockID); err != nil {
		return uuid.UUID{}, fmt.Errorf("lock application database identity: %w", err)
	}
	identity, err := uuid.New()
	if err != nil {
		return uuid.UUID{}, err
	}
	var updated string
	err = transaction.QueryRow(ctx, `
UPDATE ml_core.database_identity SET id = $1, created_at = clock_timestamp()
WHERE singleton
RETURNING id::text`, identity.String()).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, fmt.Errorf("restored application database identity is missing")
	}
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("reset application database identity: %w", err)
	}
	if updated != identity.String() {
		return uuid.UUID{}, fmt.Errorf("reset application database identity returned an unexpected value")
	}
	if err := transaction.Commit(ctx); err != nil {
		return uuid.UUID{}, fmt.Errorf("commit reset application database identity: %w", err)
	}
	return identity, nil
}

func open(ctx context.Context, descriptor postgresconn.Descriptor, password string) (*pgxpool.Pool, error) {
	configuration, err := descriptor.PoolConfig(password)
	if err != nil {
		return nil, err
	}
	configuration.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		return nil, fmt.Errorf("create application database connection: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to application PostgreSQL at %s: %w", descriptor.Address(), err)
	}
	return pool, nil
}
