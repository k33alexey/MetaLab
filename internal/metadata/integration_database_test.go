package metadata

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/integrationtest"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The integration tests of the package share one database (AGENTS.md: they
// run one package at a time), and each prepares it the same way: an empty
// application schema, the schema of its own catalog built as a save of the
// data builds it, and the platform's tables it needs. What a test leaves
// behind - the schema, its entry in the migration journal, the counters of
// its objects - is removed when it ends, whatever way it ends. These helpers
// are that preparation; a test says only what is its own.

// integrationCleanupTimeout bounds each removal at the end of a test: the
// test's own context may be spent by then.
const integrationCleanupTimeout = 10 * time.Second

// integrationPool opens the database the integration tests are given, or
// skips the test when none is. maxConnections above zero caps the pool, for a
// test that needs writers to wait for a connection. The pool closes when the
// test ends, after every cleanup registered later.
func integrationPool(t *testing.T, ctx context.Context, maxConnections int) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	if maxConnections > 0 {
		separator := "?"
		if strings.Contains(databaseURL, "?") {
			separator = "&"
		}
		databaseURL += fmt.Sprintf("%spool_max_conns=%d", separator, maxConnections)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// integrationDatabase is integrationPool over an empty application schema:
// whatever an earlier test left is dropped now, and what this one builds is
// dropped when it ends.
func integrationDatabase(t *testing.T, ctx context.Context, maxConnections int) *pgxpool.Pool {
	t.Helper()
	pool := integrationPool(t, ctx, maxConnections)
	dropSchema := "DROP SCHEMA IF EXISTS " + pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize() + " CASCADE"
	integrationCleanup(t, pool, dropSchema)
	if _, err := pool.Exec(ctx, dropSchema); err != nil {
		t.Fatal(err)
	}
	return pool
}

// integrationCleanup runs a statement against the database when the test
// ends. Its error is not the test's: the test has already said what it had to.
func integrationCleanup(t *testing.T, pool *pgxpool.Pool, statement string, arguments ...any) {
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), integrationCleanupTimeout)
		defer stop()
		_, _ = pool.Exec(ctx, statement, arguments...)
	})
}

// migrateIntegrationSchema builds the application schema of a catalog the way
// a save of the data does: the plan is prepared against the database and then
// executed, confirmed, under the project, and the platform's own tables a
// save creates are there. The project's entry in the journal and what the
// catalog's objects leave in those tables are removed when the test ends. The
// schema built is returned, for a test that compares the database with it.
func migrateIntegrationSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, catalog *Catalog, projectID uuid.UUID) schemadiff.Schema {
	t.Helper()
	forgetIntegrationProject(t, pool, projectID)
	forgetIntegrationObjects(t, pool, catalog)
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: strings.Repeat("a", 64), GitCommit: strings.Repeat("b", 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	// What a save of the data creates beside the schema
	// (publication/save_data.go): a test that writes through a repository
	// finds these whichever test ran before it, or none did.
	for _, ensure := range []func(context.Context, sqlExecutor) error{
		EnsureObjectIntegrityStorage, EnsureRegisterTotalsStorage, EnsureConstantStorage,
	} {
		if err := ensure(ctx, pool); err != nil {
			t.Fatal(err)
		}
	}
	return desired
}

// forgetIntegrationProject removes the project's entries in the migration
// journal when the test ends: the journal lies outside the application schema
// and outlives it. A test that executes a migration itself - to see it
// refused - says so; migrateIntegrationSchema does it for the rest.
func forgetIntegrationProject(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID) {
	integrationCleanup(t, pool, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
}

// forgetIntegrationObjects removes, when the test ends, what the catalog's
// objects leave in the platform's own tables (integrationtest.ForgetObjects).
func forgetIntegrationObjects(t *testing.T, pool *pgxpool.Pool, catalog *Catalog) {
	integrationtest.ForgetObjects(t, pool, catalog.ObjectIDs())
}
