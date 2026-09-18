package publication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/gitclient"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ActivationMode selects which database-mode rules apply to a save.
type ActivationMode string

const (
	ActivationPrimary ActivationMode = "primary"
	ActivationDebug   ActivationMode = "debug"
)

// ErrDirtyPrimary rejects saving uncommitted ML Project state to a Primary
// database - re-implemented on the base_package/ path (096) against Git
// status directly, replacing the retired package-manifest Dirty flag (021).
var ErrDirtyPrimary = errors.New("an uncommitted ML Project cannot be saved to a primary database")

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

// SaveDataRequest describes one "Сохранить данные" invocation - the
// package-free replacement for the retired BuildFile+Activate flow
// (iterations 021/024). It migrates PostgreSQL directly from the live
// project directory instead of an intermediate .mlpkg artifact.
type SaveDataRequest struct {
	Root      string
	Mode      ActivationMode
	Confirmed bool
	// Consent carries what the operator agreed to, per kind of damage; see
	// schemadiff.MigrationConsent.
	Consent schemadiff.MigrationConsent
}

// SavedState is what "Сохранить данные" leaves behind once it succeeds -
// the single current state of the database, not a version history.
type SavedState struct {
	ProjectID     uuid.UUID
	GitCommit     string
	ContentSHA256 string
	SchemaSHA256  string
	MigrationID   uuid.UUID
	SavedAt       time.Time
}

// SaveData migrates PostgreSQL schema and refreshes the stored metadata/BSL
// snapshot directly from the live project directory, in one transaction.
// ML Service never reads project files itself; everything it needs to run
// ends up in ml_core.database_state as part of this call.
func SaveData(ctx context.Context, pool *pgxpool.Pool, request SaveDataRequest) (SavedState, schemadiff.MigrationRecord, error) {
	if !request.Confirmed {
		return SavedState{}, schemadiff.MigrationRecord{}, schemadiff.ErrConfirmationRequired
	}
	if pool == nil || (request.Mode != ActivationPrimary && request.Mode != ActivationDebug) {
		return SavedState{}, schemadiff.MigrationRecord{}, fmt.Errorf("invalid save-data request")
	}
	dirty, revision, err := gitWorkingTreeStatus(ctx, request.Root)
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, err
	}
	if request.Mode == ActivationPrimary && dirty {
		return SavedState{}, schemadiff.MigrationRecord{}, ErrDirtyPrimary
	}
	manifest, _, err := inspect(ctx, request.Root, SourceState{GitCommit: revision, Dirty: dirty})
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, fmt.Errorf("read ML Project: %w", err)
	}
	catalog, err := manifest.Runtime.Catalog()
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, err
	}
	modules, err := metadata.LoadProjectModules(request.Root, catalog)
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, fmt.Errorf("read BSL modules: %w", err)
	}
	snapshot, err := manifest.Runtime.WithModules(modules)
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, err
	}
	if err := snapshot.Validate(); err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, fmt.Errorf("validate runtime metadata: %w", err)
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, fmt.Errorf("encode runtime metadata: %w", err)
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, err
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, err
	}
	migrationRequest := schemadiff.MigrationRequest{
		ProjectID: manifest.ProjectID, PackageSHA256: manifest.ContentSHA256, GitCommit: manifest.GitCommit,
		Desired: desired, ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256,
		Confirmed: true, Consent: request.Consent,
	}
	saved := SavedState{
		ProjectID: manifest.ProjectID, GitCommit: manifest.GitCommit,
		ContentSHA256: manifest.ContentSHA256, SchemaSHA256: manifest.SchemaSHA256,
	}
	hooks := schemadiff.TransactionHooks{
		BeforeCommit: func(ctx context.Context, transaction pgx.Tx, migration schemadiff.MigrationRecord) error {
			if err := ensureDatabaseStateStore(ctx, transaction); err != nil {
				return err
			}
			if err := metadata.EnsureObjectIntegrityStorage(ctx, transaction); err != nil {
				return err
			}
			if err := metadata.EnsureConstantStorage(ctx, transaction); err != nil {
				return err
			}
			if err := metadata.SyncConstantStorage(ctx, transaction, manifest.ConstantIDs); err != nil {
				return err
			}
			saved.MigrationID = migration.ID
			return saveDatabaseState(ctx, transaction, snapshotJSON, &saved)
		},
	}
	migration, err := schemadiff.ExecuteWithHooks(ctx, pool, migrationRequest, hooks)
	if err != nil {
		return SavedState{}, migration, err
	}
	return saved, migration, nil
}

// gitWorkingTreeStatus reports whether the ML Project directory has
// uncommitted changes and its current commit. The directory must already be
// its own Git repository (see gitclient.Open) - ML Project is never nested
// inside another repository, and every mode requires at least one commit.
func gitWorkingTreeStatus(ctx context.Context, root string) (dirty bool, revision string, err error) {
	client, err := gitclient.Open(ctx, root)
	if err != nil {
		return false, "", fmt.Errorf("open ML Project Git repository: %w", err)
	}
	status, err := client.Status(ctx)
	if err != nil {
		return false, "", err
	}
	if status.Revision == "" {
		return false, "", fmt.Errorf("ML Project must have at least one Git commit before saving data")
	}
	return len(status.Entries) != 0, status.Revision, nil
}

// CurrentDatabaseState returns the single state used for new application
// operations, replacing the old versioned publication_state/publication_versions
// tables (021/024, retired) with one current row per database.
func CurrentDatabaseState(ctx context.Context, pool *pgxpool.Pool) (metadata.RuntimeSnapshot, bool, error) {
	if pool == nil {
		return metadata.RuntimeSnapshot{}, false, fmt.Errorf("publication database is required")
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('ml_core.database_state') IS NOT NULL").Scan(&exists); err != nil {
		return metadata.RuntimeSnapshot{}, false, fmt.Errorf("inspect saved database state: %w", err)
	}
	if !exists {
		return metadata.RuntimeSnapshot{}, false, nil
	}
	var encoded []byte
	err := pool.QueryRow(ctx, "SELECT runtime FROM ml_core.database_state WHERE singleton").Scan(&encoded)
	if err != nil {
		if err == pgx.ErrNoRows {
			return metadata.RuntimeSnapshot{}, false, nil
		}
		return metadata.RuntimeSnapshot{}, false, fmt.Errorf("read saved database state: %w", err)
	}
	var snapshot metadata.RuntimeSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return metadata.RuntimeSnapshot{}, false, fmt.Errorf("decode saved database state: %w", err)
	}
	return snapshot, true, nil
}

func ensureDatabaseStateStore(ctx context.Context, transaction pgx.Tx) error {
	_, err := transaction.Exec(ctx, `
CREATE TABLE IF NOT EXISTS ml_core.database_state (
    singleton boolean PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    project_id uuid NOT NULL,
    git_commit text NOT NULL,
    content_sha256 text NOT NULL CHECK (length(content_sha256) = 64),
    schema_sha256 text NOT NULL CHECK (length(schema_sha256) = 64),
    migration_id uuid NOT NULL REFERENCES ml_core.migration_journal(id),
    runtime jsonb NOT NULL,
    saved_at timestamptz NOT NULL DEFAULT clock_timestamp()
);`)
	if err != nil {
		return fmt.Errorf("initialize saved database state store: %w", err)
	}
	return nil
}

func saveDatabaseState(ctx context.Context, transaction pgx.Tx, snapshotJSON []byte, saved *SavedState) error {
	err := transaction.QueryRow(ctx, `
INSERT INTO ml_core.database_state(singleton, project_id, git_commit, content_sha256, schema_sha256, migration_id, runtime)
VALUES (TRUE, $1, $2, $3, $4, $5, $6)
ON CONFLICT (singleton) DO UPDATE
SET project_id = EXCLUDED.project_id, git_commit = EXCLUDED.git_commit,
    content_sha256 = EXCLUDED.content_sha256, schema_sha256 = EXCLUDED.schema_sha256,
    migration_id = EXCLUDED.migration_id, runtime = EXCLUDED.runtime, saved_at = clock_timestamp()
RETURNING saved_at`, saved.ProjectID.String(), saved.GitCommit, saved.ContentSHA256, saved.SchemaSHA256,
		saved.MigrationID.String(), snapshotJSON).Scan(&saved.SavedAt)
	if err != nil {
		return fmt.Errorf("save database state: %w", err)
	}
	return nil
}
