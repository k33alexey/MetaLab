package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

var ErrConstantValueNotFound = errors.New("constant value not found")

type StoredConstant struct {
	ConstantID uuid.UUID
	Value      Value
	Revision   int64
	ChangedBy  *uuid.UUID
	ChangedAt  time.Time
}

type ConstantRepository struct {
	pool    *pgxpool.Pool
	catalog *Catalog
}

func NewConstantRepository(pool *pgxpool.Pool, catalog *Catalog) (*ConstantRepository, error) {
	if pool == nil || catalog == nil {
		return nil, fmt.Errorf("constant repository requires PostgreSQL and metadata catalog")
	}
	return &ConstantRepository{pool: pool, catalog: catalog}, nil
}

type sqlExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// EnsureConstantStorage creates the platform-owned constant value table.
func EnsureConstantStorage(ctx context.Context, executor sqlExecutor) error {
	if executor == nil {
		return fmt.Errorf("constant storage executor is required")
	}
	_, err := executor.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS ml_core;
CREATE TABLE IF NOT EXISTS ml_core.constant_values (
    constant_id uuid PRIMARY KEY,
    value jsonb NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    changed_by uuid,
    changed_at timestamptz NOT NULL DEFAULT clock_timestamp()
)`)
	if err != nil {
		return fmt.Errorf("ensure constant value storage: %w", err)
	}
	return nil
}

// SyncConstantStorage removes values of constants absent from the activated project.
func SyncConstantStorage(ctx context.Context, executor sqlExecutor, constants []uuid.UUID) error {
	if executor == nil {
		return fmt.Errorf("constant storage executor is required")
	}
	identifiers := make([]string, len(constants))
	for index, id := range constants {
		if id.IsZero() {
			return fmt.Errorf("constant metadata UUID must not be zero")
		}
		identifiers[index] = id.String()
	}
	_, err := executor.Exec(ctx, "DELETE FROM ml_core.constant_values WHERE NOT (constant_id = ANY($1::uuid[]))", identifiers)
	if err != nil {
		return fmt.Errorf("synchronize constant value storage: %w", err)
	}
	return nil
}

func (repository *ConstantRepository) Set(ctx context.Context, name string, value Value, actor *uuid.UUID) (StoredConstant, error) {
	constant, ok := repository.catalog.Constant(name)
	if !ok {
		return StoredConstant{}, fmt.Errorf("unknown constant %q", name)
	}
	normalized, err := repository.catalog.NormalizeValue(constant, value)
	if err != nil {
		return StoredConstant{}, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return StoredConstant{}, fmt.Errorf("encode constant %s: %w", name, err)
	}
	var actorValue any
	if actor != nil {
		if actor.IsZero() {
			return StoredConstant{}, fmt.Errorf("constant actor UUID must not be zero")
		}
		actorValue = actor.String()
	}
	var id string
	var changedBy *string
	var result StoredConstant
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return StoredConstant{}, err
	}
	err = query.QueryRow(ctx, `
INSERT INTO ml_core.constant_values(constant_id, value, changed_by)
VALUES ($1, $2, $3)
ON CONFLICT (constant_id) DO UPDATE SET
    value = EXCLUDED.value,
    revision = ml_core.constant_values.revision + 1,
    changed_by = EXCLUDED.changed_by,
    changed_at = clock_timestamp()
RETURNING constant_id::text, value, revision, changed_by::text, changed_at`, constant.ID.String(), string(encoded), actorValue).Scan(
		&id, &encoded, &result.Revision, &changedBy, &result.ChangedAt,
	)
	if err != nil {
		return StoredConstant{}, recordDataError(ctx, repository.pool, fmt.Errorf("save constant %s: %w", name, err))
	}
	return decodeStoredConstant(id, encoded, result.Revision, changedBy, result.ChangedAt, repository.catalog)
}

func (repository *ConstantRepository) Get(ctx context.Context, name string) (StoredConstant, error) {
	constant, ok := repository.catalog.Constant(name)
	if !ok {
		return StoredConstant{}, fmt.Errorf("unknown constant %q", name)
	}
	var id string
	var encoded []byte
	var revision int64
	var changedBy *string
	var changedAt time.Time
	query, err := queryData(ctx, repository.pool)
	if err != nil {
		return StoredConstant{}, err
	}
	err = query.QueryRow(ctx, `
SELECT constant_id::text, value, revision, changed_by::text, changed_at
FROM ml_core.constant_values WHERE constant_id = $1`, constant.ID.String()).Scan(&id, &encoded, &revision, &changedBy, &changedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredConstant{}, fmt.Errorf("%w: %s", ErrConstantValueNotFound, name)
	}
	if err != nil {
		return StoredConstant{}, recordDataError(ctx, repository.pool, fmt.Errorf("read constant %s: %w", name, err))
	}
	return decodeStoredConstant(id, encoded, revision, changedBy, changedAt, repository.catalog)
}

func decodeStoredConstant(idText string, encoded []byte, revision int64, changedBy *string, changedAt time.Time, catalog *Catalog) (StoredConstant, error) {
	if revision < 1 {
		return StoredConstant{}, fmt.Errorf("invalid stored constant revision")
	}
	id, err := uuid.Parse(idText)
	if err != nil {
		return StoredConstant{}, fmt.Errorf("parse stored constant UUID: %w", err)
	}
	constant, ok := catalog.ConstantByID(id)
	if !ok {
		return StoredConstant{}, fmt.Errorf("stored value references unknown constant %s", id)
	}
	var value Value
	if err := json.Unmarshal(encoded, &value); err != nil {
		return StoredConstant{}, fmt.Errorf("decode constant %s: %w", constant.Name, err)
	}
	value, err = catalog.NormalizeValue(constant, value)
	if err != nil {
		return StoredConstant{}, err
	}
	result := StoredConstant{ConstantID: id, Value: value, Revision: revision, ChangedAt: changedAt}
	if changedBy != nil {
		actor, parseErr := uuid.Parse(*changedBy)
		if parseErr != nil {
			return StoredConstant{}, fmt.Errorf("parse constant actor UUID: %w", parseErr)
		}
		result.ChangedBy = &actor
	}
	return result, nil
}
