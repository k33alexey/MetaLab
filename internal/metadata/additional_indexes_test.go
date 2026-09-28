package metadata

import (
	"fmt"
	"strings"
	"testing"
)

const (
	indexCatalogID   = "d0e00000-0000-4000-8000-000000000001"
	indexRegisterID  = "d0e00000-0000-4000-8000-000000000002"
	indexJournalID   = "d0e00000-0000-4000-8000-000000000003"
	indexDocumentID  = "d0e00000-0000-4000-8000-000000000004"
	indexAttributeID = "d0e00000-0000-4000-8000-000000000010"
	indexSecondID    = "d0e00000-0000-4000-8000-000000000011"
	indexPartID      = "d0e00000-0000-4000-8000-000000000012"
	indexPartFieldID = "d0e00000-0000-4000-8000-000000000013"
	indexDimensionID = "d0e00000-0000-4000-8000-000000000014"
	indexResourceID  = "d0e00000-0000-4000-8000-000000000015"
	indexColumnID    = "d0e00000-0000-4000-8000-000000000016"
)

func indexCatalogBody(body string) string {
	return `format: 1
id: ` + indexCatalogID + `
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - {id: ` + indexAttributeID + `, name: ИНН, title: {ru: ИНН}, types: [{kind: string, length: 12}]}
  - {id: ` + indexSecondID + `, name: КПП, title: {ru: КПП}, types: [{kind: string, length: 9}]}
table_parts:
  - id: ` + indexPartID + `
    name: Контакты
    title: {ru: Контакты}
    attributes:
      - {id: ` + indexPartFieldID + `, name: Телефон, title: {ru: Телефон}, types: [{kind: string, length: 20}]}
` + body
}

// The mechanism: a named index over the object's own table, over a table part,
// with key fields and non-key fields carried along.
func TestAnObjectAsksForIndexesOfItsOwn(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, indexCatalogID, indexCatalogBody(`additional_indexes:
  - name: ПоИНН
    indexed_fields: [ИНН]
    additional_fields: [Наименование]
  - name: ПоТелефону
    table: Контакты
    indexed_fields: [Телефон, НомерСтроки]
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := catalog.CatalogDefinition("Контрагенты")
	if len(definition.AdditionalIndexes) != 2 {
		t.Fatalf("the indexes were lost: %+v", definition.AdditionalIndexes)
	}
	switch first := definition.AdditionalIndexes[0]; {
	case first.Name != "ПоИНН":
		t.Fatalf("the name was lost: %+v", first)
	case first.Table != "":
		t.Fatalf("the object's own table is the empty one: %+v", first)
	case len(first.IndexedFields) != 1 || first.IndexedFields[0] != "ИНН":
		t.Fatalf("the key fields were lost: %+v", first)
	case len(first.AdditionalFields) != 1 || first.AdditionalFields[0] != "Наименование":
		t.Fatalf("the carried fields were lost: %+v", first)
	}
	if second := definition.AdditionalIndexes[1]; second.Table != "Контакты" {
		t.Fatalf("the table part was lost: %+v", second)
	}
}

// Everything an index is refused for, and each for its own reason.
func TestAnIndexIsRefusedWhenItCannotBeBuilt(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		body    string
		refusal string
	}{
		"без имени": {`additional_indexes:
  - {indexed_fields: [ИНН]}
`, "name must not be empty"},
		"имя не идентификатор": {`additional_indexes:
  - {name: "по ИНН", indexed_fields: [ИНН]}
`, "name must be an identifier"},
		"два индекса под одним именем": {`additional_indexes:
  - {name: ПоИНН, indexed_fields: [ИНН]}
  - {name: поинн, indexed_fields: [КПП]}
`, "repeats the name of additional_indexes[0]"},
		"без ключевых полей": {`additional_indexes:
  - {name: Пустой, additional_fields: [ИНН]}
`, "indexed_fields must name at least one field"},
		"поля у объекта нет": {`additional_indexes:
  - {name: ПоОкпо, indexed_fields: [ОКПО]}
`, "which is not a field of the object's own table"},
		"таблицы у объекта нет": {`additional_indexes:
  - {name: ПоСтрокам, table: Строки, indexed_fields: [ИНН]}
`, "which this object has not got"},
		"поле табличной части в индексе основной таблицы": {`additional_indexes:
  - {name: ПоТелефону, indexed_fields: [Телефон]}
`, "which is not a field of the object's own table"},
		"поле повторено": {`additional_indexes:
  - {name: ПоИНН, indexed_fields: [ИНН, ИНН]}
`, "which is already in this index"},
		"поле и ключевое, и дополнительное": {`additional_indexes:
  - {name: ПоИНН, indexed_fields: [ИНН], additional_fields: [ИНН]}
`, "which is already in this index"},
		"стандартное поле годится": {`additional_indexes:
  - {name: ПоКоду, indexed_fields: [Код], additional_fields: [Наименование, ПометкаУдаления]}
`, ""},
		"порядок полей — другой индекс, не копия": {`additional_indexes:
  - {name: Первый, indexed_fields: [ИНН, КПП]}
  - {name: Второй, indexed_fields: [КПП, ИНН]}
`, ""},
		"два индекса по тем же полям не копия": {`additional_indexes:
  - {name: Первый, indexed_fields: [ИНН]}
  - {name: Второй, indexed_fields: [ИНН], additional_fields: [КПП]}
`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, indexCatalogID, indexCatalogBody(want.body))
			_, err := Load(root)
			switch {
			case want.refusal == "" && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case want.refusal != "":
				if err == nil || !strings.Contains(err.Error(), want.refusal) {
					t.Fatalf("%s: %v", name, err)
				}
			}
		})
	}
}

// A virtual table of the kind is accepted by name and its fields are left
// alone: they are the query language's fields, not the object's, and the model
// of them is block 6. A name that is no table of this kind is still refused.
func TestAnIndexOverAVirtualTableIsCarriedNotResolved(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		table   string
		refusal string
	}{
		"остатки регистра":                {"Остатки", ""},
		"обороты регистра":                {"Обороты", ""},
		"регистрация изменений":           {"Изменения", ""},
		"среза у регистра накопления нет": {"СрезПоследних", "which this object has not got"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, AccumulationRegisterKind, indexRegisterID, `format: 1
id: `+indexRegisterID+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: balance
dimensions:
  - {id: `+indexDimensionID+`, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: string, length: 50}]}
resources:
  - {id: `+indexResourceID+`, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}
additional_indexes:
  - name: Ускоряющий
    table: `+want.table+`
    indexed_fields: [ЧегоУНасНетВовсе]
`)
			_, err := Load(root)
			switch {
			case want.refusal == "" && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case want.refusal != "":
				if err == nil || !strings.Contains(err.Error(), want.refusal) {
					t.Fatalf("%s: %v", name, err)
				}
			}
		})
	}
}

// Fourteen kinds carry the mechanism, not four as the map said. The journal
// and the sequence are the two the register list would never have found: the
// journal stores nothing of its own and the sequence is not an object.
func TestTheKindsTheMapDidNotName(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, DocumentKind, indexDocumentID, `format: 1
id: `+indexDocumentID+`
name: Накладная
title: {ru: Накладная}
number: {type: string, length: 11, periodicity: year, auto: true}
attributes:
  - {id: `+indexAttributeID+`, name: Склад, title: {ru: Склад}, types: [{kind: string, length: 50}]}
`)
	writeMetadata(t, root, DocumentJournalKind, indexJournalID, `format: 1
id: `+indexJournalID+`
name: Складские
title: {ru: Складские}
documents: [`+indexDocumentID+`]
columns:
  - {id: `+indexColumnID+`, name: Склад, title: {ru: Склад}, references: [`+indexAttributeID+`]}
additional_indexes:
  - {name: ПоСкладу, indexed_fields: [Склад], additional_fields: [Дата]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	journal, ok := catalog.DocumentJournal("Складские")
	if !ok || len(journal.AdditionalIndexes) != 1 {
		t.Fatalf("a journal lost its index: %+v found=%v", journal.AdditionalIndexes, ok)
	}
	// A column the journal has not got is refused there as anywhere else.
	writeMetadata(t, root, DocumentJournalKind, indexJournalID, `format: 1
id: `+indexJournalID+`
name: Складские
title: {ru: Складские}
documents: [`+indexDocumentID+`]
additional_indexes:
  - {name: ПоСкладу, indexed_fields: [Склад]}
`)
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "which is not a field of") {
		t.Fatalf("a journal took an index over a column it has not got: %v", err)
	}
}

// PostgreSQL takes at most thirty-two columns in one index, key and carried
// together. The ceiling is ours, not the prototype's, and it is better said
// here than discovered by a migration that cannot finish.
func TestAnIndexIsRefusedWhenTheDatabaseWillNotTakeIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	var attributes, fields strings.Builder
	for position := 1; position <= 33; position++ {
		attributes.WriteString(fmt.Sprintf(
			"  - {id: d0e00000-0000-4000-8000-%012d, name: Поле%d, title: {ru: Поле%d}, types: [{kind: string, length: 4}]}\n",
			position, position, position))
		if position > 1 {
			fields.WriteString(", ")
		}
		fields.WriteString(fmt.Sprintf("Поле%d", position))
	}
	writeMetadata(t, root, CatalogKind, indexCatalogID, `format: 1
id: `+indexCatalogID+`
name: Широкий
title: {ru: Широкий}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
`+attributes.String()+`additional_indexes:
  - name: Слишком
    indexed_fields: [`+fields.String()+`]
`)
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "an index takes at most 32") {
		t.Fatalf("a thirty-three column index was accepted: %v", err)
	}
}
