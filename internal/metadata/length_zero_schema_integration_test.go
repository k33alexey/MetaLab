package metadata

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The tables of a chart of accounts, a chart of calculation types and a task
// without a description, and of an exchange plan without a code and a
// description, are built by PostgreSQL itself, and a second preparation of the
// same project plans nothing.
//
// Defect caught: a column PostgreSQL refuses - character varying(0) - which
// stops the migration of the whole configuration; and a schema that reads back
// different from what it asked for, so that every save would migrate again.
func TestALengthOfZeroMigratesIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	root := metadataProject(t)
	for _, item := range []struct {
		kind Kind
		body string
	}{
		{ChartOfAccountsKind, "code: {type: string, length: 5}\ndescription_length: 0\n"},
		{ChartOfCalculationTypesKind, "code: {type: string, length: 5}\ndescription_length: 0\n"},
		{TaskKind, "number: {type: string, length: 11, auto: true, periodicity: none}\ndescription_length: 0\n"},
		{ExchangePlanKind, "code: {type: string, length: 0}\ndescription_length: 0\n"},
	} {
		noteChartBody(t, root, item.kind, item.body)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('3', 64), GitCommit: repeatCatalogHex('4', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatalf("PostgreSQL refused the tables: %v", err)
	}
	again, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Plan.Changes) != 0 {
		t.Fatalf("the built schema plans again: %+v", again.Plan.Changes)
	}
}
