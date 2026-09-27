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

// The setting governs a mechanism that already exists, so the test is about the
// mechanism: a catalog that says its predefined data is not updated
// automatically must come out of a synchronization untouched, while its
// neighbour in the same run is brought into line.
//
// Untouched means all three things the synchronizer does are skipped - creating
// a missing row, assigning the predefined name, detaching one the description no
// longer mentions. Doing two of the three would be worse than doing none: a
// classifier filled from outside would keep its rows and lose the identity the
// code refers to them by.
func TestPredefinedDataIsLeftAloneWhenTheObjectSaysSoIntegration(t *testing.T) {
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
	projectID := uuid.MustNew()
	managedID, manualID := uuid.MustNew(), uuid.MustNew()
	managedItemID, manualItemID := uuid.MustNew(), uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.object_sequences WHERE metadata_id = ANY($1::uuid[])",
			[]string{managedID.String(), manualID.String()})
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}

	catalog := &Catalog{
		Catalogs: []CatalogDefinition{
			{
				ID: managedID, Name: "Статусы", DescriptionLength: 100,
				Code:       CatalogCode{Type: StringType, Length: 4, Auto: true, Unique: true},
				Predefined: []PredefinedCatalogItem{{ID: managedItemID, Name: "Новый", Description: "Новый"}},
			},
			{
				// A classifier filled from outside: the description names an
				// item, and the platform is told to keep its hands off.
				ID: manualID, Name: "Классификатор", DescriptionLength: 100,
				Code:                 CatalogCode{Type: StringType, Length: 4, Auto: true, Unique: true},
				Predefined:           []PredefinedCatalogItem{{ID: manualItemID, Name: "Основной", Description: "Основной"}},
				PredefinedDataUpdate: PredefinedDataUpdateManual,
			},
		},
		catalogByName: map[string]int{"статусы": 0, "классификатор": 1},
		catalogByID:   map[uuid.UUID]int{managedID: 0, manualID: 1},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('1', 64), GitCommit: repeatCatalogHex('2', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureObjectIntegrityStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}
	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}

	result, err := repository.SynchronizePredefined(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// One created for the managed catalog, and the manual one reported as
	// skipped rather than silently counted as "nothing to do".
	if result.Created != 1 || result.Skipped != 1 {
		t.Fatalf("sync = %+v, want one created and one skipped", result)
	}
	if _, err := repository.Get(ctx, CatalogReference{CatalogID: managedID, ObjectID: managedItemID}); err != nil {
		t.Fatalf("the managed catalog's predefined item was not created: %v", err)
	}
	if _, err := repository.Get(ctx, CatalogReference{CatalogID: manualID, ObjectID: manualItemID}); err == nil {
		t.Fatal("a row was created in a catalog that said its predefined data is not updated automatically")
	}

	// And the identity is left alone too. A row put there by the developer keeps
	// whatever predefined name it was given - the synchronizer neither assigns
	// nor clears it.
	table, _ := PhysicalCatalogTable(manualID)
	if _, err := pool.Exec(ctx, "INSERT INTO "+qualifiedCatalogTable(table)+
		" (ref, code, description, deletion_mark, predefined_name, version) VALUES ($1, '0001', 'Заведён вручную', false, 'Основной', 1)",
		manualItemID.String()); err != nil {
		t.Fatal(err)
	}
	again, err := repository.SynchronizePredefined(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again.Assigned != 0 || again.Detached != 0 || again.Skipped != 1 {
		t.Fatalf("second sync = %+v, want nothing assigned or detached", again)
	}
	var name string
	if err := pool.QueryRow(ctx, "SELECT predefined_name FROM "+qualifiedCatalogTable(table)+" WHERE ref = $1",
		manualItemID.String()).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Основной" {
		t.Fatalf("predefined name = %q: the synchronizer touched a catalog it was told to leave alone", name)
	}
}
