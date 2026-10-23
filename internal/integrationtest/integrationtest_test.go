package integrationtest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// recorder is a test the removals are registered on: it keeps what they
// would run at the end and what they would complain of.
type recorder struct {
	testing.TB
	cleanups   []func()
	complaints []string
}

func (test *recorder) Helper()            { test.TB.Helper() }
func (test *recorder) Cleanup(run func()) { test.cleanups = append(test.cleanups, run) }
func (test *recorder) Errorf(format string, arguments ...any) {
	test.complaints = append(test.complaints, fmt.Sprintf(format, arguments...))
}

// A removal from a table or a schema that is not there is silent, and any
// other failure still fails the test. Defects caught: a test that failed on a
// fresh database before its first save - where the platform has not made its
// tables of objects in ml_core yet - getting a complaint about each of them on
// top of its reason (review 2.464, П4); a removal that fails for another
// reason passed over as silently.
func TestARemovalOfWhatIsNotThereIsSilent(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, removal := range []struct {
		statement string
		fails     bool
	}{
		{"DELETE FROM ml_core.ml_it_table_not_there WHERE metadata_id = ANY($1::uuid[])", false},
		{"DELETE FROM ml_it_schema_not_there.object_sequences WHERE metadata_id = ANY($1::uuid[])", false},
		{"DELETE FROM pg_catalog.pg_class WHERE ml_it_column_not_there = ANY($1::uuid[])", true},
	} {
		test := &recorder{TB: t}
		cleanup(test, pool, removal.statement, []string{})
		for _, run := range test.cleanups {
			run()
		}
		if failed := len(test.complaints) > 0; failed != removal.fails {
			t.Errorf("%s: complaints %v, want failing %v", removal.statement, test.complaints, removal.fails)
		}
	}
}
