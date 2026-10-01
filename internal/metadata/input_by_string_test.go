package metadata

import (
	"strings"
	"testing"
)

const (
	inputCatalog  = "41000000-0000-4000-8000-000000000001"
	inputTaxCode  = "41000000-0000-4000-8000-000000000002"
	inputPartner  = "41000000-0000-4000-8000-000000000003"
	inputPartners = "41000000-0000-4000-8000-000000000004"
	inputNote     = "41000000-0000-4000-8000-000000000005"
)

// inputCatalogBody is a catalog with three attributes: one a string with an
// index, one a reference, one a string without an index. The last two are here to
// be refused.
func inputCatalogBody(tail string) string {
	return `format: 1
id: ` + inputCatalog + `
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: ` + inputTaxCode + `
    name: ИНН
    title: {ru: ИНН}
    types: [{kind: string, length: 12}]
    indexing: index
  - id: ` + inputPartner + `
    name: Партнер
    title: {ru: Партнёр}
    types: [{kind: catalog, reference: ` + inputPartners + `}]
    indexing: index
  - id: ` + inputNote + `
    name: Заметка
    title: {ru: Заметка}
    types: [{kind: string, length: 50}]
` + tail
}

// The whole of the group on a catalog, with a field of each role and the order
// the demonstration configuration actually writes: the description first, the
// code after it, an own attribute last.
func TestCatalogKeepsHowItIsFoundByTypedText(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, inputPartners, "Партнеры")
	writeMetadata(t, root, CatalogKind, inputCatalog, inputCatalogBody(`input_by_string:
  - {standard: Наименование}
  - {standard: Код}
  - {attribute: ИНН}
search_string_mode: any-part
full_text_search_on_input: use
choice_data_get_mode: background
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("контрагенты")
	if !ok {
		t.Fatal("the catalog was not read")
	}
	want := []ObjectField{{Standard: "Наименование"}, {Standard: "Код"}, {Attribute: "ИНН"}}
	if len(definition.InputByString) != len(want) {
		t.Fatalf("input_by_string = %+v", definition.InputByString)
	}
	for index, field := range want {
		if definition.InputByString[index] != field {
			t.Fatalf("field %d = %+v, want %+v", index, definition.InputByString[index], field)
		}
	}
	if definition.SearchStringMode != SearchAnyPart ||
		definition.FullTextSearchOnInput != FullTextOnInputUse ||
		definition.ChoiceDataGetMode != ChoiceDataBackground {
		t.Fatalf("input = %+v", definition.ObjectInput)
	}
}

// The order is the search order, so a copy of a definition must not share the
// list with the original: a caller reordering its copy would reorder the
// configuration.
func TestCopyOfAnObjectDoesNotShareItsSearchedFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, inputPartners, "Партнеры")
	writeMetadata(t, root, CatalogKind, inputCatalog, inputCatalogBody(`input_by_string:
  - {standard: Наименование}
  - {standard: Код}
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := catalog.CatalogDefinition("контрагенты")
	first.InputByString[0] = ObjectField{Standard: "Код"}
	second, _ := catalog.CatalogDefinition("контрагенты")
	if second.InputByString[0].Standard != "Наименование" {
		t.Fatalf("the order was changed through a copy: %+v", second.InputByString)
	}
}

// All eight kinds that carry the group take all four settings. The four reach
// them through two shared shapes, so this guards the wiring of both.
func TestEveryKindWithAnInputFieldIsFoundByTypedText(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		content string
		field   string
	}{
		"справочник": {CatalogKind, `code: {type: string, length: 9, auto: true}
description_length: 150
`, "Наименование"},
		"план видов характеристик": {ChartOfCharacteristicTypesKind, `code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
`, "Код"},
		"план счетов": {ChartOfAccountsKind, `code: {type: string, length: 5, auto: false}
description_length: 120
`, "Код"},
		"план видов расчёта": {ChartOfCalculationTypesKind, `code: {type: string, length: 9}
description_length: 100
`, "Наименование"},
		"план обмена": {ExchangePlanKind, `code: {type: string, length: 36, auto: false}
description_length: 150
`, "Код"},
		"документ": {DocumentKind, `number: {type: string, length: 11, auto: true, periodicity: year}
`, "Номер"},
		"бизнес-процесс": {BusinessProcessKind, `number: {type: string, length: 11, auto: true, periodicity: year}
`, "Номер"},
		"задача": {TaskKind, `number: {type: string, length: 11, auto: true, periodicity: none}
description_length: 150
addressing_attributes: []
`, "Наименование"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, inputCatalog, `format: 1
id: `+inputCatalog+`
name: Объект
title: {ru: Объект}
`+body.content+`input_by_string:
  - {standard: `+body.field+`}
search_string_mode: begin
full_text_search_on_input: dont-use
choice_data_get_mode: directly
`)
			if _, err := Load(root); err != nil {
				t.Fatalf("a kind with an input field refused part of the group: %v", err)
			}
		})
	}
}

// Which of the platform's own fields a kind is searched by is the kind's own
// answer, not one list for all. The help rolls the business process and the task
// into one sentence and gives both Наименование and Номер; a business process has
// no Наименование, and accepting it would promise a search over a field that is
// not there.
func TestABusinessProcessHasNoDescriptionToBeFoundBy(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		kind    Kind
		body    string
		field   string
		accepts bool
	}{
		"у задачи есть наименование": {TaskKind, `number: {type: string, length: 11, auto: true, periodicity: none}
description_length: 150
addressing_attributes: []
`, "Наименование", true},
		"у бизнес-процесса нет наименования": {BusinessProcessKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "Наименование", false},
		"у документа нет кода": {DocumentKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "Код", false},
		"справочник по номеру не ищут": {CatalogKind, `code: {type: string, length: 9, auto: true}
description_length: 150
`, "Номер", false},
		"и не по ссылке, хотя она стандартная": {CatalogKind, `code: {type: string, length: 9, auto: true}
description_length: 150
`, "Ссылка", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, want.kind, inputCatalog, `format: 1
id: `+inputCatalog+`
name: Объект
title: {ru: Объект}
`+want.body+`input_by_string:
  - {standard: `+want.field+`}
`)
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("refused a field this kind is searched by: %v", err)
			case !want.accepts && (err == nil || !strings.Contains(err.Error(), "not a standard field this kind of object is searched by")):
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// An own attribute may be searched only if the text can be compared against it
// and the comparison can use an index. Both halves are the prototype's rule, and
// both matter: the search runs on every keystroke.
func TestOnlyAnIndexedStringOrNumberIsSearchedByText(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		field   string
		message string
	}{
		"строка с индексом": {"ИНН", ""},
		"ссылка — нельзя":   {"Партнер", "must be of one type, string or number"},
		"без индекса":       {"Заметка", "must be indexed to be searched by"},
		"которого нет":      {"Артикул", "not an attribute of this object"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			catalogNamed(t, root, inputPartners, "Партнеры")
			writeMetadata(t, root, CatalogKind, inputCatalog, inputCatalogBody(`input_by_string:
  - {attribute: `+want.field+`}
`))
			_, err := Load(root)
			switch {
			case want.message == "" && err != nil:
				t.Fatalf("refused a field the prototype allows: %v", err)
			case want.message != "" && (err == nil || !strings.Contains(err.Error(), want.message)):
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// A field names one thing. Naming both roles, or neither, or the same field
// twice, describes a search nobody can perform.
func TestASearchedFieldNamesOneThingOnce(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		list    string
		message string
	}{
		"и то и другое": {"  - {standard: Код, attribute: ИНН}\n", "names both a standard field and an attribute"},
		"ни то ни другое": {"  - {}\n",
			"names neither a standard field nor an attribute"},
		"один и тот же дважды": {"  - {standard: Код}\n  - {standard: Код}\n",
			"is already among the searched fields"},
		"по-английски он тот же": {"  - {standard: Код}\n  - {standard: Code}\n",
			"is already among the searched fields"},
		"реквизит дважды": {"  - {attribute: ИНН}\n  - {attribute: инн}\n",
			"is already among the searched fields"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			catalogNamed(t, root, inputPartners, "Партнеры")
			writeMetadata(t, root, CatalogKind, inputCatalog, inputCatalogBody("input_by_string:\n"+want.list))
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), want.message) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// An empty list is not a missing one. Two catalogs of the demonstration
// configuration carry no searched fields at all, and that is what they mean: the
// object is not entered by string.
func TestAnObjectMayBeFoundByNoTextAtAll(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, inputPartners, "Партнеры")
	writeMetadata(t, root, CatalogKind, inputCatalog, inputCatalogBody("input_by_string: []\n"))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("контрагенты")
	if !ok {
		t.Fatal("the catalog was not read")
	}
	if len(definition.InputByString) != 0 {
		t.Fatalf("input_by_string = %+v", definition.InputByString)
	}
}

// Each of the three enumerations takes the prototype's values and no others. The
// third is the one to watch: "directly" and "background" are the whole of it, and
// a mode named after a query would describe the manager module's event instead.
func TestTypedTextSettingsTakeOnlyThePrototypesValues(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		setting string
		message string
	}{
		"способ поиска строки":  {"search_string_mode: middle", "search_string_mode must be begin or any-part"},
		"полнотекстовый поиск":  {"full_text_search_on_input: auto", "full_text_search_on_input must be use or dont-use"},
		"получение данных":      {"choice_data_get_mode: from-query", "choice_data_get_mode must be directly or background"},
		"и не молча по-другому": {"search_string_mode: Begin", "search_string_mode must be begin or any-part"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			catalogNamed(t, root, inputPartners, "Партнеры")
			writeMetadata(t, root, CatalogKind, inputCatalog, inputCatalogBody(want.setting+"\n"))
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), want.message) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// An enumeration is picked from a list, not typed into: it has three of the six
// settings of input and none of the four here. A file that gives it one of them
// is refused when it is read.
func TestAnEnumerationIsNotFoundByTypedText(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, EnumerationKind, inputCatalog, `format: 1
id: `+inputCatalog+`
name: Состояния
title: {ru: Состояния}
values:
  - id: `+enumValueID+`
    name: Новое
    title: {ru: Новое}
search_string_mode: begin
`)
	if _, err := Load(root); err == nil {
		t.Fatal("an enumeration accepted a setting of input by string")
	}
}
