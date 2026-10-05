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

// The schema of the composite project is built on PostgreSQL, holds a table
// for every object that keeps data, and a second preparation over the built
// schema plans nothing.
//
// Defect caught: two kinds whose tables, indexes or constraints collide or
// that PostgreSQL refuses only together; a table that is planned and never
// built, or built differently from what is planned, so that every opening of
// the project plans the same migration again.
func TestCompositeProjectSchemaIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]bool{}
	for _, table := range desired.Tables {
		tables[table.Name] = true
	}
	for name, id := range map[string]string{
		"справочник Пользователи": cmpUsers, "справочник Склады": cmpWarehouses, "справочник Номенклатура": cmpGoods,
		"документ": cmpDocument, "план видов характеристик": cmpCharacteristics, "план счетов": cmpAccounts,
		"план видов расчёта": cmpCalculationTypes, "план обмена": cmpExchangePlan, "задача": cmpTask,
		"бизнес-процесс": cmpProcess, "регистр сведений": cmpPrices, "регистр накопления": cmpBalances,
		"регистр бухгалтерии": cmpEntries, "регистр расчёта": cmpPayroll,
	} {
		parsed, err := uuid.Parse(id)
		if err != nil {
			t.Fatal(err)
		}
		table, err := schemadiff.TableName(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if !tables[table] {
			t.Errorf("%s has no table %s in the schema", name, table)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	dropSchema := "DROP SCHEMA IF EXISTS " + pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize() + " CASCADE"
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, dropSchema)
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, dropSchema); err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('c', 64), GitCommit: repeatCatalogHex('d', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	again, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Plan.Changes) != 0 {
		t.Fatalf("the built schema plans %d changes again: %+v", len(again.Plan.Changes), again.Plan.Changes)
	}
}
