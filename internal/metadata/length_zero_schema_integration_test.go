package metadata

import (
	"context"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The tables of a chart of accounts, a chart of calculation types and a task
// without a description are built by PostgreSQL itself, and a second
// preparation of the same project plans nothing. An exchange plan takes no
// length of 0 (2.494).
//
// Defect caught: a column PostgreSQL refuses - character varying(0) - which
// stops the migration of the whole configuration; and a schema that reads back
// different from what it asked for, so that every save would migrate again.
func TestALengthOfZeroMigratesIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := integrationDatabase(t, ctx, 0)
	projectID := uuid.MustNew()
	root := metadataProject(t)
	for _, item := range []struct {
		kind Kind
		body string
	}{
		{ChartOfAccountsKind, "code: {type: string, length: 5}\ndescription_length: 0\n"},
		{ChartOfCalculationTypesKind, "code: {type: string, length: 5}\ndescription_length: 0\n"},
		{TaskKind, "number: {type: string, length: 11, auto: true, periodicity: none}\ndescription_length: 0\n"},
	} {
		noteChartBody(t, root, item.kind, item.body)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	desired := migrateIntegrationSchema(t, ctx, pool, catalog, projectID)
	again, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Plan.Changes) != 0 {
		t.Fatalf("the built schema plans again: %+v", again.Plan.Changes)
	}
}
