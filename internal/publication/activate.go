package publication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type ActivationMode string

const (
	ActivationPrimary ActivationMode = "primary"
	ActivationDebug   ActivationMode = "debug"
)

var (
	ErrStalePublication = errors.New("active publication changed; rebuild and review the publication plan")
	ErrSchemaDiverged   = errors.New("PostgreSQL schema differs from the active ML publication")
	ErrProjectMismatch  = errors.New("ML Project does not match the project already published to this database")
	ErrDirtyPrimary     = errors.New("an uncommitted ML Project cannot be published to a primary database")
	ErrPackageChanged   = errors.New("publication package changed while it was being activated")
	ErrAlreadyActive    = errors.New("publication package is already active")
)

// ActivationRequest binds an already reviewed package and migration plan to the
// exact active database version observed by ML Studio.
type ActivationRequest struct {
	PackagePath                 string
	Desired                     schemadiff.Schema
	Prepared                    schemadiff.PreparedPlan
	ExpectedGitCommit           string
	ExpectedActivePackageSHA256 string
	Mode                        ActivationMode
	Confirmed                   bool
	AllowDestructive            bool
}

type Version struct {
	ID            uuid.UUID       `json:"id"`
	ProjectID     uuid.UUID       `json:"projectId"`
	PackageSHA256 string          `json:"packageSha256"`
	ContentSHA256 string          `json:"contentSha256"`
	GitCommit     string          `json:"gitCommit"`
	Dirty         bool            `json:"dirty"`
	SchemaSHA256  string          `json:"schemaSha256"`
	MigrationID   uuid.UUID       `json:"migrationId"`
	Manifest      json.RawMessage `json:"manifest"`
	ActivatedAt   time.Time       `json:"activatedAt"`
}

type ActiveVersion struct {
	Version
	Generation int64 `json:"generation"`
}

// Activate verifies the artifact, rechecks Git/schema expectations and commits
// its schema migration and active-version pointer in one PostgreSQL transaction.
func Activate(ctx context.Context, pool *pgxpool.Pool, request ActivationRequest) (ActiveVersion, schemadiff.MigrationRecord, error) {
	if !request.Confirmed {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, schemadiff.ErrConfirmationRequired
	}
	if pool == nil || (request.Mode != ActivationPrimary && request.Mode != ActivationDebug) {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, fmt.Errorf("invalid publication activation request")
	}
	manifest, err := VerifyFile(ctx, request.PackagePath)
	if err != nil {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, err
	}
	if manifest.GitCommit == "" || manifest.GitCommit != strings.TrimSpace(request.ExpectedGitCommit) {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, ErrStalePublication
	}
	if request.Mode == ActivationPrimary && manifest.Dirty {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, ErrDirtyPrimary
	}
	if request.ExpectedActivePackageSHA256 != "" && !validSHA256(request.ExpectedActivePackageSHA256) {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, fmt.Errorf("invalid expected active publication digest")
	}
	targetSHA256, err := schemadiff.SchemaSHA256(request.Desired)
	if err != nil {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, err
	}
	planSHA256, err := schemadiff.PlanSHA256(request.Prepared.Plan)
	if err != nil {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, err
	}
	if request.Prepared.SHA256 != planSHA256 || !validSHA256(request.Prepared.ActualSHA256) || request.Prepared.TargetSHA256 != targetSHA256 {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, fmt.Errorf("publication plan does not match its target schema")
	}
	packageSHA256, err := digestFile(ctx, request.PackagePath)
	if err != nil {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, fmt.Errorf("encode publication manifest: %w", err)
	}
	versionID, err := uuid.New()
	if err != nil {
		return ActiveVersion{}, schemadiff.MigrationRecord{}, err
	}
	version := ActiveVersion{Version: Version{
		ID: versionID, ProjectID: manifest.ProjectID, PackageSHA256: packageSHA256,
		ContentSHA256: manifest.ContentSHA256, GitCommit: manifest.GitCommit, Dirty: manifest.Dirty,
		SchemaSHA256: targetSHA256, Manifest: manifestJSON,
	}}
	migrationRequest := schemadiff.MigrationRequest{
		ProjectID: manifest.ProjectID, PackageSHA256: packageSHA256, GitCommit: manifest.GitCommit,
		Desired: request.Desired, ExpectedPlanSHA256: request.Prepared.SHA256,
		ExpectedSchemaSHA256: request.Prepared.ActualSHA256,
		Confirmed:            true, AllowDestructive: request.AllowDestructive,
	}
	hooks := schemadiff.TransactionHooks{
		BeforeApply: func(ctx context.Context, transaction pgx.Tx, _ schemadiff.MigrationRecord) error {
			if err := ensureActivationStore(ctx, transaction); err != nil {
				return err
			}
			active, found, err := currentInTransaction(ctx, transaction, true)
			if err != nil {
				return err
			}
			if !found {
				if request.ExpectedActivePackageSHA256 != "" {
					return ErrStalePublication
				}
				return nil
			}
			if active.ProjectID != manifest.ProjectID {
				return ErrProjectMismatch
			}
			if active.PackageSHA256 == packageSHA256 {
				return ErrAlreadyActive
			}
			if active.PackageSHA256 != request.ExpectedActivePackageSHA256 {
				return ErrStalePublication
			}
			if active.SchemaSHA256 != request.Prepared.ActualSHA256 {
				return ErrSchemaDiverged
			}
			var previouslyPublished bool
			if err := transaction.QueryRow(ctx,
				"SELECT EXISTS(SELECT 1 FROM ml_core.publication_versions WHERE package_sha256 = $1)",
				packageSHA256).Scan(&previouslyPublished); err != nil {
				return fmt.Errorf("check publication history: %w", err)
			}
			if previouslyPublished {
				return ErrStalePublication
			}
			return nil
		},
		BeforeCommit: func(ctx context.Context, transaction pgx.Tx, migration schemadiff.MigrationRecord) error {
			currentPackageSHA256, err := digestFile(ctx, request.PackagePath)
			if err != nil {
				return err
			}
			if currentPackageSHA256 != packageSHA256 {
				return ErrPackageChanged
			}
			version.MigrationID = migration.ID
			return activateInTransaction(ctx, transaction, &version)
		},
	}
	migration, err := schemadiff.ExecuteWithHooks(ctx, pool, migrationRequest, hooks)
	if err != nil {
		return ActiveVersion{}, migration, err
	}
	return version, migration, nil
}

// Current returns the single version used for new application operations.
func Current(ctx context.Context, pool *pgxpool.Pool) (ActiveVersion, bool, error) {
	if pool == nil {
		return ActiveVersion{}, false, fmt.Errorf("publication database is required")
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('ml_core.publication_state') IS NOT NULL").Scan(&exists); err != nil {
		return ActiveVersion{}, false, fmt.Errorf("inspect active publication store: %w", err)
	}
	if !exists {
		return ActiveVersion{}, false, nil
	}
	return currentInTransaction(ctx, pool, false)
}

// ListVersions returns activation history newest first.
func ListVersions(ctx context.Context, pool *pgxpool.Pool, limit int) ([]Version, error) {
	if pool == nil || limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("publication history limit must be between 1 and 1000")
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('ml_core.publication_versions') IS NOT NULL").Scan(&exists); err != nil {
		return nil, fmt.Errorf("inspect publication history store: %w", err)
	}
	if !exists {
		return []Version{}, nil
	}
	rows, err := pool.Query(ctx, `
SELECT id::text, project_id::text, package_sha256, content_sha256, git_commit, dirty,
       schema_sha256, migration_id::text, manifest, activated_at
FROM ml_core.publication_versions ORDER BY activated_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list publication history: %w", err)
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		var item Version
		var id, projectID, migrationID string
		if err := rows.Scan(&id, &projectID, &item.PackageSHA256, &item.ContentSHA256, &item.GitCommit,
			&item.Dirty, &item.SchemaSHA256, &migrationID, &item.Manifest, &item.ActivatedAt); err != nil {
			return nil, fmt.Errorf("scan publication history: %w", err)
		}
		item.ID, err = uuid.Parse(id)
		if err == nil {
			item.ProjectID, err = uuid.Parse(projectID)
		}
		if err == nil {
			item.MigrationID, err = uuid.Parse(migrationID)
		}
		if err != nil {
			return nil, fmt.Errorf("parse publication history identity: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate publication history: %w", err)
	}
	return items, nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func currentInTransaction(ctx context.Context, query rowQuerier, lock bool) (ActiveVersion, bool, error) {
	statement := `
SELECT version.id::text, version.project_id::text, version.package_sha256, version.content_sha256,
       version.git_commit, version.dirty, version.schema_sha256, version.migration_id::text,
       version.manifest, version.activated_at, state.generation
FROM ml_core.publication_state AS state
JOIN ml_core.publication_versions AS version ON version.id = state.version_id
WHERE state.singleton`
	if lock {
		statement += " FOR UPDATE OF state"
	}
	var active ActiveVersion
	var id, projectID, migrationID string
	err := query.QueryRow(ctx, statement).Scan(
		&id, &projectID, &active.PackageSHA256, &active.ContentSHA256, &active.GitCommit,
		&active.Dirty, &active.SchemaSHA256, &migrationID, &active.Manifest, &active.ActivatedAt, &active.Generation,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ActiveVersion{}, false, nil
	}
	if err != nil {
		return ActiveVersion{}, false, fmt.Errorf("read active publication: %w", err)
	}
	active.ID, err = uuid.Parse(id)
	if err == nil {
		active.ProjectID, err = uuid.Parse(projectID)
	}
	if err == nil {
		active.MigrationID, err = uuid.Parse(migrationID)
	}
	if err != nil {
		return ActiveVersion{}, false, fmt.Errorf("parse active publication identity: %w", err)
	}
	return active, true, nil
}

func ensureActivationStore(ctx context.Context, transaction pgx.Tx) error {
	_, err := transaction.Exec(ctx, `
CREATE TABLE IF NOT EXISTS ml_core.publication_versions (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    package_sha256 text NOT NULL UNIQUE CHECK (length(package_sha256) = 64),
    content_sha256 text NOT NULL CHECK (length(content_sha256) = 64),
    git_commit text NOT NULL,
    dirty boolean NOT NULL,
    schema_sha256 text NOT NULL CHECK (length(schema_sha256) = 64),
    migration_id uuid NOT NULL UNIQUE REFERENCES ml_core.migration_journal(id),
    manifest jsonb NOT NULL,
    activated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE IF NOT EXISTS ml_core.publication_state (
    singleton boolean PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    project_id uuid NOT NULL,
    version_id uuid NOT NULL UNIQUE REFERENCES ml_core.publication_versions(id),
    generation bigint NOT NULL CHECK (generation > 0)
);`)
	if err != nil {
		return fmt.Errorf("initialize publication store: %w", err)
	}
	return nil
}

func activateInTransaction(ctx context.Context, transaction pgx.Tx, version *ActiveVersion) error {
	err := transaction.QueryRow(ctx, `
INSERT INTO ml_core.publication_versions(
    id, project_id, package_sha256, content_sha256, git_commit, dirty, schema_sha256, migration_id, manifest
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING activated_at`, version.ID.String(), version.ProjectID.String(), version.PackageSHA256,
		version.ContentSHA256, version.GitCommit, version.Dirty, version.SchemaSHA256,
		version.MigrationID.String(), version.Manifest).Scan(&version.ActivatedAt)
	if err != nil {
		return fmt.Errorf("record publication version: %w", err)
	}
	err = transaction.QueryRow(ctx, `
INSERT INTO ml_core.publication_state(singleton, project_id, version_id, generation)
VALUES (TRUE, $1, $2, 1)
ON CONFLICT (singleton) DO UPDATE
SET project_id = EXCLUDED.project_id, version_id = EXCLUDED.version_id,
    generation = ml_core.publication_state.generation + 1
RETURNING generation`, version.ProjectID.String(), version.ID.String()).Scan(&version.Generation)
	if err != nil {
		return fmt.Errorf("activate publication version: %w", err)
	}
	return nil
}

func digestFile(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open publication package for hashing: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := copyContext(ctx, hash, file); err != nil {
		return "", fmt.Errorf("hash publication package: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
