package metadata

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	managedID, manualID := uuid.MustNew(), uuid.MustNew()
	managedItemID, manualItemID := uuid.MustNew(), uuid.MustNew()
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
	repository, pool := predefinedSyncFixture(t, ctx, catalog)

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

// predefinedSyncFixture builds the schema of a catalog of catalogs in the test
// database and returns the repository a synchronization of predefined data
// runs on, with the pool to look at the rows directly. Everything it creates is
// dropped when the test ends.
func predefinedSyncFixture(t *testing.T, ctx context.Context, catalog *Catalog) (*CatalogRepository, *pgxpool.Pool) {
	t.Helper()
	pool := integrationDatabase(t, ctx, 0)
	projectID := uuid.MustNew()
	ids := make([]string, len(catalog.Catalogs))
	for index, definition := range catalog.Catalogs {
		ids[index] = definition.ID.String()
	}
	migrateIntegrationSchema(t, ctx, pool, catalog, projectID)
	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	return repository, pool
}

// A predefined item whose code is spaces alone is created with the empty code,
// as METADATA-OBJECTS.md, «Код предопределённого элемента», asks: with
// automatic codes each gets the next code of its own, without them the string
// code is empty and the numeric one zero. lombard1 has three such items in one
// catalog with a fixed code, automatic codes and the check for repeats.
//
// Defect caught: the spaces written as they are - the fixed code pads them to
// the empty code and the second item breaks the uniqueness of the code, a
// variable code keeps a value other than the empty one, a numeric code refuses
// the item and with it the whole synchronization.
func TestPredefinedCodeOfSpacesIsTheEmptyCodeIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixedID, variableID, numericID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	parties := []PredefinedCatalogItem{
		{ID: uuid.MustNew(), Name: "Банк", Code: "         "},
		{ID: uuid.MustNew(), Name: "Касса", Code: "  "},
		{ID: uuid.MustNew(), Name: "Ломбард", Code: " "},
	}
	variableItem, numericItem := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{
			{ID: fixedID, Name: "Контрагенты", DescriptionLength: 100, Predefined: parties,
				Code: CatalogCode{Type: StringType, Length: 9, Auto: true, Unique: true, FixedLength: true}},
			{ID: variableID, Name: "Валюты", DescriptionLength: 100,
				Code:       CatalogCode{Type: StringType, Length: 3, Unique: true},
				Predefined: []PredefinedCatalogItem{{ID: variableItem, Name: "Рубль", Code: "   "}}},
			{ID: numericID, Name: "Ставки", DescriptionLength: 100,
				Code:       CatalogCode{Type: NumberType, Length: 3},
				Predefined: []PredefinedCatalogItem{{ID: numericItem, Name: "Основная", Code: "  "}}},
		},
		catalogByName: map[string]int{"контрагенты": 0, "валюты": 1, "ставки": 2},
		catalogByID:   map[uuid.UUID]int{fixedID: 0, variableID: 1, numericID: 2},
	}
	repository, pool := predefinedSyncFixture(t, ctx, catalog)
	result, err := repository.SynchronizePredefined(ctx)
	if err != nil {
		t.Fatalf("synchronization refused the codes of spaces: %v", err)
	}
	if result.Created != 5 {
		t.Fatalf("sync = %+v, want five created", result)
	}
	codes := map[string]bool{}
	for _, item := range parties {
		record, err := repository.Get(ctx, CatalogReference{CatalogID: fixedID, ObjectID: item.ID})
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(record.Code) == "" || codes[record.Code] {
			t.Fatalf("%s got code %q: want a next code of its own", item.Name, record.Code)
		}
		codes[record.Code] = true
	}
	for _, row := range []struct {
		catalog uuid.UUID
		item    uuid.UUID
		want    string
	}{{variableID, variableItem, ""}, {numericID, numericItem, "0"}} {
		table, _ := PhysicalCatalogTable(row.catalog)
		var code string
		if err := pool.QueryRow(ctx, "SELECT code::text FROM "+qualifiedCatalogTable(table)+" WHERE ref = $1", row.item.String()).Scan(&code); err != nil {
			t.Fatal(err)
		}
		if code != row.want {
			t.Fatalf("code stored %q, want %q", code, row.want)
		}
	}
}
