package schemadiff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const migrationLockID int64 = 0x4d4c534348454d41 // "MLSCHEMA"

var (
	ErrConfirmationRequired = errors.New("schema migration confirmation is required")
	ErrDestructiveDenied    = errors.New("destructive schema migration was not explicitly allowed")
	ErrPlanChanged          = errors.New("PostgreSQL schema changed after the migration plan was prepared")
)

type PreparedPlan struct {
	Plan         Plan   `json:"plan"`
	SHA256       string `json:"sha256"`
	TargetSHA256 string `json:"targetSha256"`
}

type MigrationRequest struct {
	ProjectID          uuid.UUID
	PackageSHA256      string
	GitCommit          string
	Desired            Schema
	ExpectedPlanSHA256 string
	Confirmed          bool
	AllowDestructive   bool
}

type MigrationRecord struct {
	ID               uuid.UUID       `json:"id"`
	ProjectID        uuid.UUID       `json:"projectId"`
	PackageSHA256    string          `json:"packageSha256"`
	GitCommit        string          `json:"gitCommit"`
	PlanSHA256       string          `json:"planSha256"`
	SchemaSHA256     string          `json:"schemaSha256"`
	Status           string          `json:"status"`
	DestructiveCount int             `json:"destructiveCount"`
	Changes          json.RawMessage `json:"changes"`
	Error            string          `json:"error,omitempty"`
	StartedAt        time.Time       `json:"startedAt"`
	CompletedAt      time.Time       `json:"completedAt"`
}

func Prepare(ctx context.Context, pool *pgxpool.Pool, desired Schema) (PreparedPlan, error) {
	actual, err := Inspect(ctx, pool, desired.Name)
	if err != nil {
		return PreparedPlan{}, err
	}
	plan, err := Compare(desired, actual)
	if err != nil {
		return PreparedPlan{}, err
	}
	digest, err := PlanSHA256(plan)
	if err != nil {
		return PreparedPlan{}, err
	}
	targetDigest, err := SchemaSHA256(desired)
	if err != nil {
		return PreparedPlan{}, err
	}
	return PreparedPlan{Plan: plan, SHA256: digest, TargetSHA256: targetDigest}, nil
}

func PlanSHA256(plan Plan) (string, error) {
	content, err := json.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("encode migration plan: %w", err)
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func SchemaSHA256(schema Schema) (string, error) {
	schema = cloneSchema(schema)
	if err := schema.NormalizeAndValidate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("encode target schema: %w", err)
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

// Execute rechecks and applies the confirmed plan in one PostgreSQL transaction.
func Execute(ctx context.Context, pool *pgxpool.Pool, request MigrationRequest) (MigrationRecord, error) {
	if !request.Confirmed {
		return MigrationRecord{}, ErrConfirmationRequired
	}
	if pool == nil || request.ProjectID.IsZero() || !validDigest(request.PackageSHA256) || !validDigest(request.ExpectedPlanSHA256) || validateCommit(request.GitCommit) != nil {
		return MigrationRecord{}, fmt.Errorf("invalid schema migration request")
	}
	request.Desired = cloneSchema(request.Desired)
	if err := request.Desired.NormalizeAndValidate(); err != nil {
		return MigrationRecord{}, err
	}
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return MigrationRecord{}, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Release()
	if err := initializeJournal(ctx, connection); err != nil {
		return MigrationRecord{}, err
	}
	transaction, err := connection.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MigrationRecord{}, fmt.Errorf("begin schema migration: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, "SET LOCAL lock_timeout = '10s'; SET LOCAL statement_timeout = '10min'"); err != nil {
		return MigrationRecord{}, fmt.Errorf("configure schema migration transaction: %w", err)
	}
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLockID); err != nil {
		return MigrationRecord{}, fmt.Errorf("lock schema migration: %w", err)
	}
	actual, err := inspectCatalog(ctx, transaction, request.Desired.Name)
	if err != nil {
		return MigrationRecord{}, err
	}
	plan, err := Compare(request.Desired, actual)
	if err != nil {
		return MigrationRecord{}, err
	}
	digest, err := PlanSHA256(plan)
	if err != nil {
		return MigrationRecord{}, err
	}
	if digest != request.ExpectedPlanSHA256 {
		return MigrationRecord{}, ErrPlanChanged
	}
	if plan.DestructiveCount > 0 && !request.AllowDestructive {
		return MigrationRecord{}, ErrDestructiveDenied
	}
	statements, err := migrationStatements(plan)
	if err != nil {
		return MigrationRecord{}, err
	}
	targetDigest, err := SchemaSHA256(request.Desired)
	if err != nil {
		return MigrationRecord{}, err
	}
	changes, err := json.Marshal(plan.Changes)
	if err != nil {
		return MigrationRecord{}, fmt.Errorf("encode migration changes: %w", err)
	}
	id, err := uuid.New()
	if err != nil {
		return MigrationRecord{}, err
	}
	record := MigrationRecord{
		ID: id, ProjectID: request.ProjectID, PackageSHA256: request.PackageSHA256, GitCommit: request.GitCommit,
		PlanSHA256: digest, SchemaSHA256: targetDigest, Status: "running", DestructiveCount: plan.DestructiveCount, Changes: changes,
	}
	if err := insertMigration(ctx, transaction, record); err != nil {
		return MigrationRecord{}, err
	}
	for _, statement := range statements {
		if _, err := transaction.Exec(ctx, statement); err != nil {
			_ = transaction.Rollback(ctx)
			record.Status, record.Error = "failed", boundedError(err)
			record = recordFailure(ctx, connection, record)
			return record, fmt.Errorf("execute schema migration: %w", err)
		}
	}
	if err := transaction.QueryRow(ctx, `
UPDATE ml_core.migration_journal
SET status = 'succeeded', completed_at = clock_timestamp()
WHERE id = $1
RETURNING started_at, completed_at`, id.String()).Scan(&record.StartedAt, &record.CompletedAt); err != nil {
		_ = transaction.Rollback(ctx)
		record.Status, record.Error = "failed", boundedError(err)
		record = recordFailure(ctx, connection, record)
		return record, fmt.Errorf("complete schema migration journal: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		record.Error = boundedError(err)
		if errors.Is(err, pgx.ErrTxCommitRollback) {
			record.Status = "failed"
			record = recordFailure(ctx, connection, record)
		} else {
			record.Status = "unknown"
		}
		return record, fmt.Errorf("commit schema migration: %w", err)
	}
	record.Status = "succeeded"
	return record, nil
}

func initializeJournal(ctx context.Context, connection *pgxpool.Conn) error {
	transaction, err := connection.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migration journal initialization: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, "SET LOCAL lock_timeout = '10s'"); err != nil {
		return fmt.Errorf("configure migration journal initialization: %w", err)
	}
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("lock migration journal initialization: %w", err)
	}
	if err := ensureJournal(ctx, transaction); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration journal initialization: %w", err)
	}
	return nil
}

type sqlExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func ensureJournal(ctx context.Context, connection sqlExecer) error {
	_, err := connection.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS ml_core;
CREATE TABLE IF NOT EXISTS ml_core.migration_journal (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    package_sha256 text NOT NULL CHECK (length(package_sha256) = 64),
    git_commit text NOT NULL,
    plan_sha256 text NOT NULL CHECK (length(plan_sha256) = 64),
    schema_sha256 text NOT NULL CHECK (length(schema_sha256) = 64),
    status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    destructive_count integer NOT NULL CHECK (destructive_count >= 0),
    changes jsonb NOT NULL,
    error text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS migration_journal_started_idx ON ml_core.migration_journal(started_at DESC);`)
	if err != nil {
		return fmt.Errorf("initialize schema migration journal: %w", err)
	}
	return nil
}

func insertMigration(ctx context.Context, transaction pgx.Tx, record MigrationRecord) error {
	_, err := transaction.Exec(ctx, `
INSERT INTO ml_core.migration_journal(
	id, project_id, package_sha256, git_commit, plan_sha256, schema_sha256, status, destructive_count, changes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		record.ID.String(), record.ProjectID.String(), record.PackageSHA256, record.GitCommit,
		record.PlanSHA256, record.SchemaSHA256, record.Status, record.DestructiveCount, record.Changes)
	if err != nil {
		return fmt.Errorf("start schema migration journal: %w", err)
	}
	return nil
}

func recordFailure(ctx context.Context, connection *pgxpool.Conn, record MigrationRecord) MigrationRecord {
	failureContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = connection.QueryRow(failureContext, `
INSERT INTO ml_core.migration_journal(
	id, project_id, package_sha256, git_commit, plan_sha256, schema_sha256, status, destructive_count, changes, error, completed_at
) VALUES ($1, $2, $3, $4, $5, $6, 'failed', $7, $8, $9, clock_timestamp())
ON CONFLICT (id) DO UPDATE SET status = 'failed', error = EXCLUDED.error, completed_at = EXCLUDED.completed_at
RETURNING started_at, completed_at`,
		record.ID.String(), record.ProjectID.String(), record.PackageSHA256, record.GitCommit,
		record.PlanSHA256, record.SchemaSHA256, record.DestructiveCount, record.Changes, record.Error).Scan(&record.StartedAt, &record.CompletedAt)
	return record
}

func ListMigrations(ctx context.Context, pool *pgxpool.Pool, limit int) ([]MigrationRecord, error) {
	if pool == nil || limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("migration journal limit must be between 1 and 1000")
	}
	rows, err := pool.Query(ctx, `
SELECT id::text, project_id::text, package_sha256, git_commit, plan_sha256, schema_sha256, status,
       destructive_count, changes, error, started_at, COALESCE(completed_at, started_at)
FROM ml_core.migration_journal ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list schema migrations: %w", err)
	}
	defer rows.Close()
	items := make([]MigrationRecord, 0)
	for rows.Next() {
		var item MigrationRecord
		var id, projectID string
		if err := rows.Scan(&id, &projectID, &item.PackageSHA256, &item.GitCommit, &item.PlanSHA256, &item.SchemaSHA256, &item.Status,
			&item.DestructiveCount, &item.Changes, &item.Error, &item.StartedAt, &item.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan schema migration: %w", err)
		}
		item.ID, err = uuid.Parse(id)
		if err == nil {
			item.ProjectID, err = uuid.Parse(projectID)
		}
		if err != nil {
			return nil, fmt.Errorf("parse schema migration identity: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema migrations: %w", err)
	}
	return items, nil
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validateCommit(value string) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 20 && len(decoded) != 32 {
		return fmt.Errorf("invalid Git commit")
	}
	return nil
}

func boundedError(err error) string {
	value := err.Error()
	if len(value) > 2048 {
		value = value[:2048]
	}
	return value
}
