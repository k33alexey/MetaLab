package prototype

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS ml_prototype_calculations (
    id BIGSERIAL PRIMARY KEY,
    left_value DOUBLE PRECISION NOT NULL,
    right_value DOUBLE PRECISION NOT NULL,
    result DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
)`

// PostgresStore persists the measured prototype scenario in PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// OpenPostgresStore creates a validated PostgreSQL connection pool.
func OpenPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	configuration, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	store := &PostgresStore{pool: pool}
	if err := store.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

// Close releases all database connections.
func (store *PostgresStore) Close() { store.pool.Close() }

// Ping verifies that PostgreSQL is reachable.
func (store *PostgresStore) Ping(ctx context.Context) error {
	if err := store.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return nil
}

// Migrate creates the isolated prototype table.
func (store *PostgresStore) Migrate(ctx context.Context) error {
	if _, err := store.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate prototype schema: %w", err)
	}
	return nil
}

// Save persists a calculation and returns its generated identity.
func (store *PostgresStore) Save(ctx context.Context, calculation Calculation) (Calculation, error) {
	err := store.pool.QueryRow(ctx, `
        INSERT INTO ml_prototype_calculations(left_value, right_value, result, created_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id`,
		calculation.Left, calculation.Right, calculation.Result, calculation.CreatedAt,
	).Scan(&calculation.ID)
	if err != nil {
		return Calculation{}, fmt.Errorf("save prototype calculation: %w", err)
	}
	return calculation, nil
}

// Stats reads the count and most recently stored result.
func (store *PostgresStore) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	err := store.pool.QueryRow(ctx, `
        SELECT COUNT(*), COALESCE((SELECT result FROM ml_prototype_calculations ORDER BY id DESC LIMIT 1), 0)
        FROM ml_prototype_calculations`,
	).Scan(&stats.Count, &stats.LastResult)
	if err != nil {
		return Stats{}, fmt.Errorf("read prototype stats: %w", err)
	}
	return stats, nil
}
