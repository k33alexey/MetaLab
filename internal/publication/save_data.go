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
	ProjectName   string
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
	manifest, err := inspect(ctx, request.Root, SourceState{GitCommit: revision, Dirty: dirty})
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
	// Before anything is read from the schema and long before a plan is built:
	// this database has to belong to this project.
	if err := checkDatabaseBelongsToProject(ctx, pool, manifest.ProjectID, manifest.ProjectName); err != nil {
		return SavedState{}, schemadiff.MigrationRecord{}, err
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
		ProjectID: manifest.ProjectID, ProjectName: manifest.ProjectName, GitCommit: manifest.GitCommit,
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
    project_name text NOT NULL DEFAULT '',
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
	if _, err = transaction.Exec(ctx,
		"ALTER TABLE ml_core.database_state ADD COLUMN IF NOT EXISTS project_name text NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("initialize saved database state store: %w", err)
	}
	return nil
}

func saveDatabaseState(ctx context.Context, transaction pgx.Tx, snapshotJSON []byte, saved *SavedState) error {
	err := transaction.QueryRow(ctx, `
INSERT INTO ml_core.database_state(singleton, project_id, project_name, git_commit, content_sha256, schema_sha256, migration_id, runtime)
VALUES (TRUE, $1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (singleton) DO UPDATE
SET project_name = EXCLUDED.project_name, git_commit = EXCLUDED.git_commit,
    content_sha256 = EXCLUDED.content_sha256, schema_sha256 = EXCLUDED.schema_sha256,
    migration_id = EXCLUDED.migration_id, runtime = EXCLUDED.runtime, saved_at = clock_timestamp()
RETURNING saved_at`, saved.ProjectID.String(), saved.ProjectName, saved.GitCommit, saved.ContentSHA256, saved.SchemaSHA256,
		saved.MigrationID.String(), snapshotJSON).Scan(&saved.SavedAt)
	if err != nil {
		return fmt.Errorf("save database state: %w", err)
	}
	return nil
}

// PublicationMarker identifies what is currently saved in one application
// database, without reading the snapshot itself. ML App polls it to notice that
// the configuration changed under an open session: the whole runtime metadata
// is megabytes, this is two short strings.
type PublicationMarker struct {
	ContentSHA256 string    `json:"contentSha256"`
	SavedAt       time.Time `json:"savedAt"`
}

// CurrentPublicationMarker reports what the database was last saved as. A
// database that has never been saved has no marker and no error - that is an
// ordinary state, not a failure.
func CurrentPublicationMarker(ctx context.Context, pool *pgxpool.Pool) (PublicationMarker, bool, error) {
	if pool == nil {
		return PublicationMarker{}, false, fmt.Errorf("publication database is required")
	}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('ml_core.database_state') IS NOT NULL").Scan(&exists); err != nil {
		return PublicationMarker{}, false, fmt.Errorf("inspect saved database state: %w", err)
	}
	if !exists {
		return PublicationMarker{}, false, nil
	}
	var marker PublicationMarker
	err := pool.QueryRow(ctx, "SELECT content_sha256, saved_at FROM ml_core.database_state WHERE singleton").Scan(&marker.ContentSHA256, &marker.SavedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicationMarker{}, false, nil
	}
	if err != nil {
		return PublicationMarker{}, false, fmt.Errorf("read saved database state: %w", err)
	}
	return marker, true, nil
}

// ErrForeignDatabase says the database belongs to another project, or is not
// free to be bound to this one.
var ErrForeignDatabase = errors.New("database does not belong to this ML Project")

// checkDatabaseBelongsToProject refuses to apply a project to a database that
// is not its own, before the schema is read and long before a plan is built.
//
// Consent is no protection here. A plan that says "drop every table and create
// different ones" looks like an ordinary request to lose objects, and the
// developer has just pressed save and is expecting to confirm something - they
// will read it and agree. And the situation is an everyday one: two databases
// of one customer, two clones of the repository, the wrong shortcut.
//
// Four outcomes, and each has to be told apart from the others:
//
//   - the record is there and matches - go on;
//   - the record is there and does not - refuse, naming both sides;
//   - no record and no application tables - a free database, which this save
//     binds to this project;
//   - no record but application tables are there - refuse: the database is
//     occupied by something, and what that something is nobody knows.
func checkDatabaseBelongsToProject(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, projectName string) error {
	var stateExists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('ml_core.database_state') IS NOT NULL").Scan(&stateExists); err != nil {
		return fmt.Errorf("inspect saved database state: %w", err)
	}
	var database string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
		return fmt.Errorf("read database name: %w", err)
	}
	if stateExists {
		var ownerText, ownerName string
		err := pool.QueryRow(ctx,
			"SELECT project_id::text, project_name FROM ml_core.database_state WHERE singleton").Scan(&ownerText, &ownerName)
		switch {
		case err == nil:
			owner, parseErr := uuid.Parse(ownerText)
			if parseErr != nil {
				return fmt.Errorf("%w: database %q carries an unreadable project identifier %q",
					ErrForeignDatabase, database, ownerText)
			}
			if owner == projectID {
				return nil
			}
			// Three names, because there are exactly two causes and the
			// developer tells them apart by reading which one is unexpected:
			// the wrong project is open, or the wrong database is selected.
			return fmt.Errorf("%w: project %s (%s) is open, database %q belongs to project %s (%s)",
				ErrForeignDatabase, projectName, projectID, database, displayName(ownerName), owner)
		case errors.Is(err, pgx.ErrNoRows):
			// The store exists but holds nothing: fall through to the checks
			// for a free database.
		default:
			return fmt.Errorf("read saved database state: %w", err)
		}
	}
	occupied, err := applicationTablesExist(ctx, pool)
	if err != nil {
		return err
	}
	if occupied {
		return fmt.Errorf("%w: database %q holds application tables but no record of what was applied to it",
			ErrForeignDatabase, database)
	}
	return nil
}

// applicationTablesExist reports whether the database already holds tables this
// platform made. Only our own are counted: a table of ours is named t_ and the
// identifier of the object it stores. What an administrator keeps elsewhere in
// the database is not our business and does not make the database occupied.
//
// Inside our own schema it is a different matter, and not this check's: what
// lies in ml_data is ours to manage, so the migration plan proposes to remove
// what the configuration does not describe - and asks for consent, as it does
// for any other loss.
func applicationTablesExist(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var found bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM pg_class AS class
    JOIN pg_namespace AS namespace ON namespace.oid = class.relnamespace
    WHERE class.relkind = 'r' AND namespace.nspname = $1 AND class.relname LIKE 't\_%'
)`, schemadiff.ApplicationSchema).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("inspect application tables: %w", err)
	}
	return found, nil
}

// displayName keeps a message readable when the binding was written before the
// project name was stored beside the identifier.
func displayName(name string) string {
	if name == "" {
		return "без имени"
	}
	return name
}
