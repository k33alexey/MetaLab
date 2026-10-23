// Package integrationtest is what the integration tests of several packages
// share against the database ML_TEST_DATABASE_URL names: what a save of the
// data and the objects of a project leave outside the application schema,
// and its removal when a test ends. Only tests import it.
package integrationtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// cleanupTimeout bounds the removal at the end of a test: the test's own
// context may be spent by then.
const cleanupTimeout = 10 * time.Second

// objectTables are the platform's own tables in ml_core that key what an
// object leaves in them by the column named: the counters of codes and
// numbers, the marks of deletion, how the totals of a register are kept, the
// values of constants. They lie outside the application schema, so dropping
// it leaves them.
var objectTables = map[string]string{
	"object_sequences": "metadata_id", "object_deletions": "metadata_id",
	"register_totals_modes": "metadata_id", "constant_values": "constant_id",
}

// ForgetObjects removes, when the test ends, the rows the objects leave in
// the platform's own tables. A removal that fails fails the test: a test that
// cannot clean up leaves rows for the next run to trip on. A table that is not
// there holds nothing to remove (nothingThere).
func ForgetObjects(t testing.TB, pool *pgxpool.Pool, objects []uuid.UUID) {
	t.Helper()
	ids := make([]string, 0, len(objects))
	for _, id := range objects {
		ids = append(ids, id.String())
	}
	for table, column := range objectTables {
		cleanup(t, pool, "DELETE FROM ml_core."+table+" WHERE "+column+" = ANY($1::uuid[])", ids)
	}
}

// ForgetSavedProject removes, when the test ends, what a save of the data of
// the project leaves: the application schema, the state of the database, the
// project's entries in the migration journal - all three outlive the
// schema's objects otherwise - and the rows of its objects (ForgetObjects).
// The pool must still be open then: register its closing before this.
func ForgetSavedProject(t testing.TB, pool *pgxpool.Pool, project uuid.UUID, objects []uuid.UUID) {
	t.Helper()
	cleanup(t, pool, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", project.String())
	cleanup(t, pool, "DROP TABLE IF EXISTS ml_core.database_state")
	cleanup(t, pool, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
	ForgetObjects(t, pool, objects)
}

func cleanup(t testing.TB, pool *pgxpool.Pool, statement string, arguments ...any) {
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), cleanupTimeout)
		defer stop()
		if _, err := pool.Exec(ctx, statement, arguments...); err != nil && !nothingThere(err) {
			t.Errorf("clean up after the test (%s): %v", statement, err)
		}
	})
}

// nothingThere says the removal found no table to remove from: the platform
// makes its tables of objects in ml_core only inside a save, so a test that
// failed on a fresh database before its first save leaves none, and a
// complaint about them would only bury the reason it failed (review 2.464,
// П4). undefined_table by its SQLSTATE, which PostgreSQL gives for a table in
// a schema that is not there too.
func nothingThere(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "42P01"
}
