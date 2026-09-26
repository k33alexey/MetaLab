package metadata

import (
	"strings"
	"testing"
)

const (
	storageCatalog   = "c9000000-0000-4000-8000-000000000001"
	storagePartners  = "c9000000-0000-4000-8000-000000000002"
	storageComment   = "c9000000-0000-4000-8000-000000000010"
	storagePartner   = "c9000000-0000-4000-8000-000000000011"
	storageGroupNote = "c9000000-0000-4000-8000-000000000012"
	storagePart      = "c9000000-0000-4000-8000-000000000020"
	storageLine      = "c9000000-0000-4000-8000-000000000021"
)

// storageGoods is a hierarchical catalog: only such an object has folders for
// a field to belong to.
func storageGoods(body string) string {
	return `format: 1
id: ` + storageCatalog + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items, folders_on_top: true}
` + body
}

func TestFieldKeepsWhatFillsItAndWhatFindsIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, storagePartners, "Партнеры")
	writeMetadata(t, root, CatalogKind, storageCatalog, storageGoods(`attributes:
  - id: `+storageComment+`
    name: Комментарий
    title: {ru: Комментарий}
    types: [{kind: string, length: 100}]
    indexing: index
    full_text_search: use
    data_history: use
    use: for-item
    filling: {value: {kind: string, data: "без комментариев"}, from_filling_value: true}
  - id: `+storagePartner+`
    name: Партнер
    title: {ru: Партнёр}
    types: [{kind: catalog, reference: `+storagePartners+`}]
    indexing: index-with-additional-order
    full_text_search: dont-use
    data_history: dont-use
    use: for-folder-and-item
  - id: `+storageGroupNote+`
    name: ОписаниеГруппы
    title: {ru: Описание группы}
    types: [{kind: string, length: 100}]
    use: for-folder
table_parts:
  - id: `+storagePart+`
    name: Строки
    title: {ru: Строки}
    attributes:
      - {id: `+storageLine+`, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 10, scale: 3}], indexing: dont-index}
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	goods, ok := catalog.CatalogDefinition("Номенклатура")
	if !ok || len(goods.Attributes) != 3 {
		t.Fatalf("the catalog did not load: found=%v %+v", ok, goods.Attributes)
	}
	comment, partner, note := goods.Attributes[0], goods.Attributes[1], goods.Attributes[2]
	switch {
	case comment.Filling.Value == nil || !comment.Filling.FromFillingValue:
		t.Fatalf("what a new value starts as was lost: %+v", comment.Filling)
	case comment.Indexing != IndexField || partner.Indexing != IndexWithAdditionalOrder:
		t.Fatalf("indexing lost the third of its three answers: %+v %+v", comment.Indexing, partner.Indexing)
	case comment.FullTextSearch != UsageUse || partner.FullTextSearch != UsageDontUse:
		t.Fatalf("participation in full-text search was lost: %+v", partner)
	case comment.DataHistory != UsageUse || partner.DataHistory != UsageDontUse:
		t.Fatalf("participation in data history was lost: %+v", partner)
	case comment.Use != UseForItem || partner.Use != UseForFolderAndItem || note.Use != UseForFolder:
		t.Fatalf("whom a field belongs to was lost: %+v", goods.Attributes)
	}

	// The index follows the setting, and both indexing answers build one: what
	// the second of them adds the prototype's help does not say, so nothing
	// here invents it.
	table, _, err := catalog.catalogTables(goods)
	if err != nil {
		t.Fatal(err)
	}
	indexed := map[string]bool{}
	for _, index := range table.Indexes {
		for _, key := range index.Keys {
			indexed[key] = true
		}
	}
	for _, attribute := range []Attribute{comment, partner} {
		column, err := PhysicalAttributeColumn(attribute.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !indexed[column] {
			t.Fatalf("%s is indexed in the description and not in the table: %+v", attribute.Name, table.Indexes)
		}
	}

	goods.Attributes[0].Filling.Value.Data = "изменено"
	again, _ := catalog.CatalogDefinition("Номенклатура")
	if again.Attributes[0].Filling.Value.Data != "без комментариев" {
		t.Fatal("a filling value was handed out by reference")
	}
}

// Whom a field belongs to is a question only a catalog and a chart of
// characteristic types are asked, and only an object with folders can answer
// with a folder.
func TestAttributeUseBelongsWhereThereAreFolders(t *testing.T) {
	t.Parallel()
	flat := `format: 1
id: ` + storageCatalog + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - {id: ` + storageGroupNote + `, name: ОписаниеГруппы, title: {ru: Описание группы}, types: [{kind: string, length: 10}], use: for-folder}
`
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(flat), metadataConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "use reaches folders, and this object has none") {
		t.Fatalf("a field for folders of an object without folders: %v", err)
	}

	inPart := storageGoods(`table_parts:
  - id: ` + storagePart + `
    name: Строки
    title: {ru: Строки}
    attributes:
      - {id: ` + storageLine + `, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 10, scale: 3}], use: for-item}
`)
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(inPart), metadataConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "not of a table part") {
		t.Fatalf("a line of a table part claimed to belong to items: %v", err)
	}

	document := `format: 1
id: ` + storageCatalog + `
name: Заказ
title: {ru: Заказ}
number: {type: string, length: 9, auto: true, periodicity: none}
attributes:
  - {id: ` + storageComment + `, name: Комментарий, title: {ru: Комментарий}, types: [{kind: string, length: 10}], use: for-item}
`
	if _, err := DecodeDocument("object.yaml", strings.NewReader(document), metadataConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "catalog or a chart of characteristic types") {
		t.Fatalf("a document's attribute claimed to belong to items: %v", err)
	}
}

func TestBrokenFieldStorageSettingsAreRefused(t *testing.T) {
	t.Parallel()
	field := func(settings string) string {
		return `attributes:
  - id: ` + storageComment + `
    name: Комментарий
    title: {ru: Комментарий}
    types: [{kind: string, length: 100}]
` + settings
	}
	for name, broken := range map[string]struct{ body, want string }{
		"индексирование не из трёх": {field(`    indexing: maybe`),
			"must be dont-index, index or index-with-additional-order"},
		"полнотекстовый поиск на усмотрение платформы": {field(`    full_text_search: auto`),
			"full_text_search must be use or dont-use"},
		"история данных на усмотрение платформы": {field(`    data_history: auto`),
			"data_history must be use or dont-use"},
		"использование неизвестно какое": {field(`    use: for-nobody`),
			"use must be for-item, for-folder or for-folder-and-item"},
		"значение заполнения другого типа": {field(`    filling: {value: {kind: number, data: "1"}}`),
			"is of a type the field cannot hold"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(storageGoods(broken.body+"\n")), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}
