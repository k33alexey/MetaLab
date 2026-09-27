package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	placementFolders = "b1a50000-0000-4000-8000-000000000001"
	placementFlat    = "b1a50000-0000-4000-8000-000000000002"
	placementOwned   = "b1a50000-0000-4000-8000-000000000003"
	placementItemOne = "b1a50000-0000-4000-8000-000000000011"
	placementItemTwo = "b1a50000-0000-4000-8000-000000000012"
)

// placementCatalog is a hand-made configuration: a catalog with folders, a flat
// one, and one subordinate to the first. Nothing here touches the database, so
// it answers only what the definition alone decides.
func placementCatalog(t *testing.T, subordination SubordinationKind) *CatalogRepository {
	t.Helper()
	folders, flat, owned := mustUUID(t, placementFolders), mustUUID(t, placementFlat), mustUUID(t, placementOwned)
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{
			{ID: folders, Name: "Папки", DescriptionLength: 100, Code: CatalogCode{Type: StringType, Length: 9},
				Hierarchy: Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy}},
			{ID: flat, Name: "Плоский", DescriptionLength: 100, Code: CatalogCode{Type: StringType, Length: 9}},
			{ID: owned, Name: "Подчинённый", DescriptionLength: 100, Code: CatalogCode{Type: StringType, Length: 9},
				Owners: []uuid.UUID{folders}, Subordination: subordination},
		},
		catalogByName: map[string]int{"папки": 0, "плоский": 1, "подчинённый": 2},
		catalogByID:   map[uuid.UUID]int{folders: 0, flat: 1, owned: 2},
	}
	return &CatalogRepository{catalog: catalog}
}

// Which of the three fields a catalog has is the object's answer: a flat
// catalog has no parent, a hierarchy of items has no folder flag, a catalog
// nobody owns has no owner. A field written where the catalog has no column
// for it is refused rather than dropped - dropping it would report a write
// that succeeded and hand back a row whose parent is gone.
func TestARowCarriesOnlyThePlacementItsCatalogHas(t *testing.T) {
	t.Parallel()
	repository := placementCatalog(t, SubordinateToItems)
	flat, _ := repository.catalog.CatalogDefinition("Плоский")
	folders, _ := repository.catalog.CatalogDefinition("Папки")
	owned, _ := repository.catalog.CatalogDefinition("Подчинённый")
	for name, want := range map[string]struct {
		definition CatalogDefinition
		record     CatalogRecord
		refusal    string
	}{
		"плоский справочник без родителя": {flat,
			CatalogRecord{Parent: mustUUID(t, placementItemOne)}, "is not hierarchical"},
		"плоский справочник без групп": {flat,
			CatalogRecord{IsFolder: true}, "has no folders"},
		"неподчинённый справочник без владельца": {flat,
			CatalogRecord{Owner: Value{Kind: CatalogType, Object: mustUUID(t, placementFolders), Data: placementItemOne}},
			"is not subordinate"},
		"сам себе родитель": {folders,
			CatalogRecord{Reference: CatalogReference{ObjectID: mustUUID(t, placementItemOne)}, Parent: mustUUID(t, placementItemOne)},
			"cannot be its own parent"},
		"владелец не из объявленных": {owned,
			CatalogRecord{Owner: Value{Kind: CatalogType, Object: mustUUID(t, placementFlat), Data: placementItemOne}},
			"is not allowed here"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			record := want.record
			err := repository.normalizePlacement(want.definition, &record)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), want.refusal) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The columns a write contributes follow the same answer, and the owner is one
// column or two exactly as the table was built: a reference that may point at
// one of several objects cannot be a key into one table, so it carries the
// identity of the object beside the identity of the row.
func TestPlacementWritesTheColumnsTheTableHas(t *testing.T) {
	t.Parallel()
	repository := placementCatalog(t, SubordinateToItems)
	folders, _ := repository.catalog.CatalogDefinition("Папки")
	owned, _ := repository.catalog.CatalogDefinition("Подчинённый")
	columns, arguments, err := repository.placementColumns(folders, &CatalogRecord{Parent: mustUUID(t, placementItemOne), IsFolder: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0] != "parent" || columns[1] != "is_folder" ||
		arguments[0] != placementItemOne || arguments[1] != true {
		t.Fatalf("columns=%v arguments=%v", columns, arguments)
	}

	columns, arguments, err = repository.placementColumns(owned, &CatalogRecord{
		Owner: Value{Kind: CatalogType, Object: mustUUID(t, placementFolders), Data: placementItemTwo}})
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 1 || columns[0] != "owner" || arguments[0] != placementItemTwo {
		t.Fatalf("one owner is a key into one table: columns=%v arguments=%v", columns, arguments)
	}

	several := owned
	several.Owners = append(several.Owners, mustUUID(t, placementFlat))
	columns, arguments, err = repository.placementColumns(several, &CatalogRecord{
		Owner: Value{Kind: CatalogType, Object: mustUUID(t, placementFlat), Data: placementItemTwo}})
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0] != "owner_type" || columns[1] != "owner_ref" ||
		arguments[0] != placementFlat || arguments[1] != placementItemTwo {
		t.Fatalf("several owners are stored composite: columns=%v arguments=%v", columns, arguments)
	}
}

// A new row is an item; a folder is made by its own entry point, because
// whether a row is a folder is decided when it is made and is read-only
// afterwards. A catalog without folders has none to make.
func TestAFolderIsMadeAsAFolder(t *testing.T) {
	t.Parallel()
	repository := placementCatalog(t, SubordinateToItems)
	item, err := repository.New(t.Context(), "Папки", nil)
	if err != nil || item.IsFolder {
		t.Fatalf("a new row is an item: %+v %v", item, err)
	}
	folder, err := repository.NewFolder(t.Context(), "Папки", nil)
	if err != nil || !folder.IsFolder {
		t.Fatalf("folder=%+v error=%v", folder, err)
	}
	if _, err := repository.NewFolder(t.Context(), "Плоский", nil); err == nil || !strings.Contains(err.Error(), "has no folders") {
		t.Fatalf("a catalog without folders made one: %v", err)
	}
}

// A configuration brings whole trees of predefined data. The parent is named
// by the name of another predefined item, and until it was written nothing
// resolved that name: a name that is not there, a parent that is an item where
// only a folder holds rows, and a ring of parents all loaded quietly and
// arrived as a flat list.
func TestPredefinedTreeIsResolvedBeforeItIsWritten(t *testing.T) {
	t.Parallel()
	body := func(predefined string) string {
		return `format: 1
id: ` + placementFolders + `
name: Папки
title: {ru: Папки}
code: {type: string, length: 9, auto: true}
description_length: 100
hierarchy: {enabled: true, kind: folders-and-items}
predefined:
` + predefined
	}
	for name, want := range map[string]struct {
		predefined string
		accepts    bool
		refusal    string
	}{
		"дерево из группы и элемента": {`  - {id: ` + placementItemOne + `, name: Группа, is_folder: true}
  - {id: ` + placementItemTwo + `, name: Элемент, parent: Группа}
`, true, ""},
		"родителя нет в описании": {`  - {id: ` + placementItemOne + `, name: Элемент, parent: Нет}
`, false, "is not a predefined item of this object"},
		"родитель — элемент, а не группа": {`  - {id: ` + placementItemOne + `, name: Первый}
  - {id: ` + placementItemTwo + `, name: Второй, parent: Первый}
`, false, "is an item, and only a folder holds rows"},
		"кольцо родителей": {`  - {id: ` + placementItemOne + `, name: Первая, is_folder: true, parent: Вторая}
  - {id: ` + placementItemTwo + `, name: Вторая, is_folder: true, parent: Первая}
`, false, "makes a ring"},
		"сам себе родитель": {`  - {id: ` + placementItemOne + `, name: Группа, is_folder: true, parent: Группа}
`, false, "is the item itself"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, placementFolders, body(want.predefined))
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case !want.accepts && err == nil:
				t.Fatalf("%s: accepted", name)
			case !want.accepts && !strings.Contains(err.Error(), want.refusal):
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// An owner is not always a catalog. The type of СправочникОбъект.Владелец
// names five kinds - a catalog, a chart of characteristic types, a chart of
// accounts, a chart of calculation types and a node of an exchange plan - and
// the exchange plan was missing from the kinds we allowed: a catalog
// subordinate to one would have been refused on the way in. The demonstration
// configuration uses two of the five, which is why the export alone never
// showed it.
//
// A node of an exchange plan has no folders, so subordination to folders of
// one is still refused, and for the same reason as before.
func TestACatalogMayBelongToANodeOfAnExchangePlan(t *testing.T) {
	t.Parallel()
	exchangePlan := func(root string) {
		writeMetadata(t, root, ExchangePlanKind, placementFolders, `format: 1
id: `+placementFolders+`
name: ОбменСФилиалами
title: {ru: Обмен с филиалами}
code: {type: string, length: 36, auto: false}
description_length: 150
`)
	}
	for name, want := range map[string]struct {
		body    string
		accepts bool
		refusal string
	}{
		"подчинение узлам плана обмена": {`owners: [` + placementFolders + `]
subordination: to-items
`, true, ""},
		"подчинение группам узлов, которых нет": {`owners: [` + placementFolders + `]
subordination: to-folders
`, false, "which has none"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			exchangePlan(root)
			writeMetadata(t, root, CatalogKind, placementOwned, `format: 1
id: `+placementOwned+`
name: Подчинённый
title: {ru: Подчинённый}
code: {type: string, length: 9, auto: true}
description_length: 150
`+want.body)
			catalog, err := Load(root)
			switch {
			case !want.accepts:
				if err == nil || !strings.Contains(err.Error(), want.refusal) {
					t.Fatalf("%s: %v", name, err)
				}
				return
			case err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			}
			definition, _ := catalog.CatalogDefinition("Подчинённый")
			types := catalog.OwnerTypes(definition)
			if len(types) != 1 || types[0].Kind != ExchangePlanType {
				t.Fatalf("what the owner of a row may be was read wrong: %+v", types)
			}
		})
	}
}
