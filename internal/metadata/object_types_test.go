package metadata

import (
	"strings"
	"testing"
)

// The types that live in memory only, and where each may stand. The defect on
// one side is a configuration the prototype saves refused on import - 3748
// defined types of objects, 61 session parameters of fixed collections, 109
// attributes of data processors and reports in the configurations being moved;
// on the other, an object given a column, or a session parameter of a kind the
// help does not let one hold.

const (
	inMemoryDocument  = "51000000-0000-4000-8000-000000000001"
	inMemoryDefined   = "31000000-0000-4000-8000-000000000001"
	inMemorySession   = "71000000-0000-4000-8000-000000000001"
	inMemoryProcessor = "4e910000-0000-4000-8000-000000000001"
)

func inMemoryProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	writeMetadata(t, root, DocumentKind, inMemoryDocument, `format: 1
id: `+inMemoryDocument+`
name: Продажа
title: {ru: Продажа}
number: {type: string, length: 9, auto: true, periodicity: none}
`)
	return root
}

func TestTypesThatLiveInMemoryStandWhereThePrototypePutsThem(t *testing.T) {
	t.Parallel()
	root := inMemoryProject(t)
	// A defined type mixes an object with a primitive - the designer allows it.
	writeMetadata(t, root, DefinedTypeKind, inMemoryDefined, `format: 1
id: `+inMemoryDefined+`
name: ОбъектПродажи
title: {ru: Объект продажи}
types: [{kind: document-object, reference: `+inMemoryDocument+`}, {kind: string, length: 10}]
`)
	writeMetadata(t, root, SessionParameterKind, inMemorySession, `format: 1
id: `+inMemorySession+`
name: НастройкиСеанса
title: {ru: Настройки сеанса}
types: [{kind: fixed-structure}]
`)
	writeMetadata(t, root, DataProcessorKind, inMemoryProcessor, `format: 1
id: `+inMemoryProcessor+`
name: Загрузка
title: {ru: Загрузка}
attributes:
  - {id: 4e910000-0000-4000-8000-000000000002, name: Строки, title: {ru: Строки}, types: [{kind: value-table}]}
  - {id: 4e910000-0000-4000-8000-000000000003, name: Период, title: {ru: Период}, types: [{kind: standard-period}]}
  - {id: 4e910000-0000-4000-8000-000000000004, name: Документ, title: {ru: Документ}, types: [{kind: document-object, reference: `+inMemoryDocument+`}]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("what the prototype writes was refused: %v", err)
	}
	if _, err := catalog.ApplicationSchema(); err != nil {
		t.Fatalf("the schema refused a project with nothing to store in memory types: %v", err)
	}
	defined, ok := catalog.DefinedType("ОбъектПродажи")
	if !ok || len(defined.Types) != 2 || defined.Types[0].Kind != DocumentObjectType {
		t.Fatalf("the defined type lost its object: %+v", defined.Types)
	}
}

func TestTypesThatLiveInMemoryAreRefusedWhereNothingHoldsThem(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct {
		kind    Kind
		id      string
		body    string
		message string
	}{
		"объект у реквизита справочника": {CatalogKind, catalogID, `format: 1
id: ` + catalogID + `
name: Товары
title: {ru: Товары}
code: {type: string, length: 9}
description_length: 100
attributes:
  - {id: 41000000-0000-4000-8000-000000000002, name: Документ, title: {ru: Документ}, types: [{kind: document-object, reference: ` + inMemoryDocument + `}]}
`, "lives in memory only and cannot stand in a field the database stores"},
		"таблица значений у параметра сеанса": {SessionParameterKind, inMemorySession, `format: 1
id: ` + inMemorySession + `
name: Таблица
title: {ru: Таблица}
types: [{kind: value-table}]
`, "cannot stand in a session parameter"},
		"определяемый тип в определяемом типе": {DefinedTypeKind, inMemoryDefined, `format: 1
id: ` + inMemoryDefined + `
name: Вложенный
title: {ru: Вложенный}
types: [{kind: defined-type, reference: 31000000-0000-4000-8000-000000000009}]
`, "cannot stand in a defined type"},
		"любая ссылка в определяемом типе": {DefinedTypeKind, inMemoryDefined, `format: 1
id: ` + inMemoryDefined + `
name: Любая
title: {ru: Любая}
types: [{kind: any-ref}]
`, "cannot stand in a defined type"},
		"объект без объекта метаданных": {DefinedTypeKind, inMemoryDefined, `format: 1
id: ` + inMemoryDefined + `
name: Без
title: {ru: Без}
types: [{kind: document-object}]
`, "reference is required"},
		"любой отчёт в определяемом типе": {DefinedTypeKind, inMemoryDefined, `format: 1
id: ` + inMemoryDefined + `
name: ЛюбойОтчет
title: {ru: ЛюбойОтчет}
types: [{kind: report-object}]
`, "reference is required"},
		"объект неизвестного документа": {DefinedTypeKind, inMemoryDefined, `format: 1
id: ` + inMemoryDefined + `
name: Чужой
title: {ru: Чужой}
types: [{kind: document-object, reference: 51000000-0000-4000-8000-000000000099}]
`, "of unknown object"},
		"объект справочника на документ": {DefinedTypeKind, inMemoryDefined, `format: 1
id: ` + inMemoryDefined + `
name: Перепутан
title: {ru: Перепутан}
types: [{kind: catalog-object, reference: ` + inMemoryDocument + `}]
`, "of unknown object"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := inMemoryProject(t)
			writeMetadata(t, root, testCase.kind, testCase.id, testCase.body)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("error = %v, expected it to say %q", err, testCase.message)
			}
		})
	}
}

// A defined type of objects given to a stored attribute is let through by the
// designer and has nothing to store: an object is what a reference points at.
// The decision of the owner of 01.10.2026 is to refuse it when the database is
// built, by name - including inside a composite type, which would otherwise
// get one jsonb column without a word.
func TestAStoredFieldTypedByObjectsIsRefusedWhenTheDatabaseIsBuilt(t *testing.T) {
	t.Parallel()
	for name, types := range map[string]string{
		"только объект":     `[{kind: document-object, reference: ` + inMemoryDocument + `}]`,
		"объект со строкой": `[{kind: document-object, reference: ` + inMemoryDocument + `}, {kind: string, length: 10}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := inMemoryProject(t)
			writeMetadata(t, root, DefinedTypeKind, inMemoryDefined, `format: 1
id: `+inMemoryDefined+`
name: ОбъектПродажи
title: {ru: Объект продажи}
types: `+types+`
`)
			writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Товары
title: {ru: Товары}
code: {type: string, length: 9}
description_length: 100
attributes:
  - {id: 41000000-0000-4000-8000-000000000002, name: Документ, title: {ru: Документ}, types: [{kind: defined-type, reference: `+inMemoryDefined+`}]}
`)
			catalog, err := Load(root)
			if err != nil {
				if strings.Contains(err.Error(), "cannot be stored") {
					return
				}
				t.Fatalf("refused for another reason: %v", err)
			}
			if _, err := catalog.ApplicationSchema(); err == nil || !strings.Contains(err.Error(), "cannot be stored") {
				t.Fatalf("an object was given a column: %v", err)
			}
		})
	}
}
