package metadata

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The three fields against a real database: written, read back, and refused
// where the row they describe would mean something else.
//
// It has to be the real database, because half of what is being checked is
// what the database knows and the process does not - that the parent is there,
// that it is a folder, that the owning row exists - and because the columns
// these fields live in were built by the schema and never written into until
// now.
func TestCatalogRowKeepsWhereItSitsIntegration(t *testing.T) {
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
	partnersID, contractsID, flatID, filesID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	predefinedFolderID, predefinedItemID := uuid.MustNew(), uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.object_sequences WHERE metadata_id = ANY($1::uuid[])",
			[]string{partnersID.String(), contractsID.String(), flatID.String(), filesID.String()})
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}

	catalog := &Catalog{
		Catalogs: []CatalogDefinition{
			{ID: partnersID, Name: "Партнёры", DescriptionLength: 150,
				Code:      CatalogCode{Type: StringType, Length: 9, Auto: true},
				Hierarchy: Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy},
				Predefined: []PredefinedCatalogItem{
					{ID: predefinedFolderID, Name: "Основные", Description: "Основные", IsFolder: true},
					{ID: predefinedItemID, Name: "Наш", Description: "Наша организация", Parent: "Основные"},
				}},
			{ID: contractsID, Name: "Договоры", DescriptionLength: 150,
				Code:   CatalogCode{Type: StringType, Length: 9, Auto: true},
				Owners: []uuid.UUID{partnersID}, Subordination: SubordinateToItems},
			{ID: flatID, Name: "Плоский", DescriptionLength: 150,
				Code: CatalogCode{Type: StringType, Length: 9, Auto: true}},
			// Two owners of different kinds: the reference cannot be a key
			// into one table, so it is stored as the object beside the row.
			{ID: filesID, Name: "Файлы", DescriptionLength: 150,
				Code:   CatalogCode{Type: StringType, Length: 9, Auto: true},
				Owners: []uuid.UUID{partnersID, flatID}, Subordination: SubordinateToItems},
		},
		catalogByName: map[string]int{"партнёры": 0, "договоры": 1, "плоский": 2, "файлы": 3},
		catalogByID:   map[uuid.UUID]int{partnersID: 0, contractsID: 1, flatID: 2, filesID: 3},
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
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('e', 64), GitCommit: repeatCatalogHex('f', 40), Desired: desired,
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

	// Predefined data arrives as the tree the configuration described. Until
	// the three fields reached the data it arrived flat: the parent was read,
	// validated and then dropped on the way to the row.
	if _, err := repository.SynchronizePredefined(ctx); err != nil {
		t.Fatal(err)
	}
	predefinedItem, err := repository.Get(ctx, CatalogReference{CatalogID: partnersID, ObjectID: predefinedItemID})
	if err != nil || predefinedItem.Parent != predefinedFolderID || predefinedItem.IsFolder {
		t.Fatalf("the predefined tree arrived flat: %+v error=%v", predefinedItem, err)
	}
	predefinedFolder, err := repository.Get(ctx, CatalogReference{CatalogID: partnersID, ObjectID: predefinedFolderID})
	if err != nil || !predefinedFolder.IsFolder {
		t.Fatalf("the predefined folder arrived as an item: %+v error=%v", predefinedFolder, err)
	}

	folder, err := repository.NewFolder(ctx, "Партнёры", nil)
	if err != nil {
		t.Fatal(err)
	}
	folder.Description = "Поставщики"
	if err := repository.Save(ctx, folder, nil); err != nil {
		t.Fatal(err)
	}
	partner, err := repository.New(ctx, "Партнёры", nil)
	if err != nil {
		t.Fatal(err)
	}
	partner.Description, partner.Parent = "Первый", folder.Reference.ObjectID
	if err := repository.Save(ctx, partner, nil); err != nil {
		t.Fatal(err)
	}

	// Read back: the tree and the folder flag survive the round trip, which is
	// the whole point - they used to be dropped between the record and the row.
	loaded, err := repository.Get(ctx, partner.Reference)
	if err != nil || loaded.Parent != folder.Reference.ObjectID || loaded.IsFolder {
		t.Fatalf("the parent was lost on the way to the row: %+v error=%v", loaded, err)
	}
	loadedFolder, err := repository.Get(ctx, folder.Reference)
	if err != nil || !loadedFolder.IsFolder || !loadedFolder.Parent.IsZero() {
		t.Fatalf("folder=%+v error=%v", loadedFolder, err)
	}

	contract, err := repository.New(ctx, "Договоры", nil)
	if err != nil {
		t.Fatal(err)
	}
	contract.Description = "Договор №1"
	contract.Owner = Value{Kind: CatalogType, Object: partnersID, Data: partner.Reference.ObjectID.String()}
	if err := repository.Save(ctx, contract, nil); err != nil {
		t.Fatal(err)
	}
	loadedContract, err := repository.Get(ctx, contract.Reference)
	if err != nil || loadedContract.Owner.Data != partner.Reference.ObjectID.String() || loadedContract.Owner.Object != partnersID {
		t.Fatalf("the owner was lost on the way to the row: %+v error=%v", loadedContract, err)
	}

	// A composite owner: stored as two columns, read back as the object it
	// points at, and found again by the query and by the deletion check.
	subject, err := repository.New(ctx, "Плоский", nil)
	if err != nil {
		t.Fatal(err)
	}
	subject.Description = "Тема"
	if err := repository.Save(ctx, subject, nil); err != nil {
		t.Fatal(err)
	}
	file, err := repository.New(ctx, "Файлы", nil)
	if err != nil {
		t.Fatal(err)
	}
	file.Description = "Файл"
	file.Owner = Value{Kind: CatalogType, Object: flatID, Data: subject.Reference.ObjectID.String()}
	if err := repository.Save(ctx, file, nil); err != nil {
		t.Fatal(err)
	}
	loadedFile, err := repository.Get(ctx, file.Reference)
	if err != nil || loadedFile.Owner.Object != flatID || loadedFile.Owner.Data != subject.Reference.ObjectID.String() {
		t.Fatalf("a composite owner was lost on the way to the row: %+v error=%v", loadedFile, err)
	}

	// The query shows the three fields. It used not to: the columns were in
	// the table and in no query.
	runtime, err := NewRuntimeWithCatalogs(nil, repository, catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := runtime.executeQuery(ctx, "ВЫБРАТЬ Ссылка, Родитель, ЭтоГруппа ИЗ Справочник.Партнёры ГДЕ Наименование = \"Первый\"", nil)
	if err != nil || len(rows.rows) != 1 {
		t.Fatalf("hierarchy query rows=%d error=%v", len(rows.rows), err)
	}
	parentObject, _ := rows.rows[0][1].AsRuntimeObject()
	parent, valid := parentObject.(*catalogReferenceObject)
	folderFlag, _ := rows.rows[0][2].AsBoolean()
	if !valid || parent.reference.ObjectID != folder.Reference.ObjectID || folderFlag {
		t.Fatalf("the parent came back as %v (folder=%v)", parentObject, folderFlag)
	}
	owned, err := runtime.executeQuery(ctx, "ВЫБРАТЬ Владелец ИЗ Справочник.Договоры", nil)
	if err != nil || len(owned.rows) != 1 {
		t.Fatalf("owner query rows=%d error=%v", len(owned.rows), err)
	}
	ownerObject, _ := owned.rows[0][0].AsRuntimeObject()
	if owner, ok := ownerObject.(*catalogReferenceObject); !ok || owner.reference.ObjectID != partner.Reference.ObjectID {
		t.Fatalf("the owner came back as %v", ownerObject)
	}
	composite, err := runtime.executeQuery(ctx, "ВЫБРАТЬ Владелец ИЗ Справочник.Файлы", nil)
	if err != nil || len(composite.rows) != 1 {
		t.Fatalf("composite owner query rows=%d error=%v", len(composite.rows), err)
	}
	compositeObject, _ := composite.rows[0][0].AsRuntimeObject()
	if owner, ok := compositeObject.(*catalogReferenceObject); !ok || owner.reference != (CatalogReference{CatalogID: flatID, ObjectID: subject.Reference.ObjectID}) {
		t.Fatalf("the composite owner came back as %v", compositeObject)
	}
	if _, err := runtime.executeQuery(ctx, "ВЫБРАТЬ Родитель ИЗ Справочник.Плоский", nil); err == nil {
		t.Fatal("a flat catalog answered a query about its parent")
	}

	// Each of these is a row that would be written and mean something else.
	for name, want := range map[string]struct {
		build   func() *CatalogRecord
		refusal string
	}{
		"родитель — элемент, а не группа": {func() *CatalogRecord {
			row, _ := repository.New(ctx, "Партнёры", nil)
			row.Description, row.Parent = "Второй", partner.Reference.ObjectID
			return row
		}, "is an item, and only a folder holds rows"},
		"родителя нет в справочнике": {func() *CatalogRecord {
			row, _ := repository.New(ctx, "Партнёры", nil)
			row.Description, row.Parent = "Третий", uuid.MustNew()
			return row
		}, "is not a row of this catalog"},
		"владелец — группа, а подчинение элементам": {func() *CatalogRecord {
			row, _ := repository.New(ctx, "Договоры", nil)
			row.Description = "Договор с папкой"
			row.Owner = Value{Kind: CatalogType, Object: partnersID, Data: folder.Reference.ObjectID.String()}
			return row
		}, "subordinate to items, and"},
		"владельца нет в справочнике": {func() *CatalogRecord {
			row, _ := repository.New(ctx, "Договоры", nil)
			row.Description = "Договор без владельца"
			row.Owner = Value{Kind: CatalogType, Object: partnersID, Data: uuid.MustNew().String()}
			return row
		}, "which has no row"},
	} {
		t.Run(name, func(t *testing.T) {
			err := repository.Save(ctx, want.build(), nil)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), want.refusal) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}

	// A folder does not become an item by being saved as one. Whether a row is
	// a folder is decided when it is made, and the prototype says so outright.
	turned, err := repository.Get(ctx, folder.Reference)
	if err != nil {
		t.Fatal(err)
	}
	turned.IsFolder = false
	if err := repository.Save(ctx, turned, nil); err == nil || !strings.Contains(err.Error(), "decided when it is made") {
		t.Fatalf("a folder turned into an item: %v", err)
	}

	// The deletion check sees both of the new links. Neither used to exist for
	// it, and the composite one has no foreign key behind it at all.
	folderToDelete, err := repository.Get(ctx, folder.Reference)
	if err != nil {
		t.Fatal(err)
	}
	folderToDelete.DeletionMark = true
	if err := repository.Save(ctx, folderToDelete, nil); err != nil {
		t.Fatal(err)
	}
	_, err = repository.Delete(ctx, folderToDelete, nil, nil)
	var referenced *ReferenceIntegrityError
	if !errors.As(err, &referenced) || referenced.Uses[0].Field != "Родитель" {
		t.Fatalf("a folder holding rows was deleted: %v", err)
	}
	ownerToDelete, err := repository.Get(ctx, partner.Reference)
	if err != nil {
		t.Fatal(err)
	}
	ownerToDelete.DeletionMark = true
	if err := repository.Save(ctx, ownerToDelete, nil); err != nil {
		t.Fatal(err)
	}
	_, err = repository.Delete(ctx, ownerToDelete, nil, nil)
	if !errors.As(err, &referenced) || referenced.Uses[0].Field != "Владелец" {
		t.Fatalf("an owner with subordinate rows was deleted: %v", err)
	}
	// The composite owner carries no foreign key at all, so this search is the
	// only thing between a deleted owner and a row left pointing at it.
	compositeOwner, err := repository.Get(ctx, subject.Reference)
	if err != nil {
		t.Fatal(err)
	}
	compositeOwner.DeletionMark = true
	if err := repository.Save(ctx, compositeOwner, nil); err != nil {
		t.Fatal(err)
	}
	_, err = repository.Delete(ctx, compositeOwner, nil, nil)
	if !errors.As(err, &referenced) || referenced.Uses[0].Field != "Владелец" {
		t.Fatalf("an owner of a composite subordinate was deleted: %v", err)
	}
}
