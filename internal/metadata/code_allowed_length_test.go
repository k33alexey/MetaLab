package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A fixed code is padded to its width and a variable one is not, and the
// difference is a column type: character(n) pads on write, character varying(n)
// stores what was typed. Getting this from the model into the schema is the
// whole of what the property does, so the column is what the test looks at.
func TestAllowedLengthDecidesTheCodeColumn(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		code   CatalogCode
		column string
	}{
		"переменная длина по умолчанию": {CatalogCode{Type: StringType, Length: 9}, "character varying(9)"},
		"фиксированная длина":           {CatalogCode{Type: StringType, Length: 9, FixedLength: true}, "character(9)"},
		// A numeric code has digits, not a width to pad, and the property means
		// nothing for it - the help says so outright.
		"числовой код не дополняется": {CatalogCode{Type: NumberType, Length: 9}, "numeric(9,0)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			catalogID := parseTestUUID(t, "40000000-0000-4000-8000-000000000031")
			catalog := &Catalog{
				Catalogs: []CatalogDefinition{{
					ID: catalogID, Name: "Номенклатура", Code: want.code, DescriptionLength: 150,
				}},
				catalogByID: map[uuid.UUID]int{catalogID: 0},
			}
			schema, err := catalog.ApplicationSchema()
			if err != nil {
				t.Fatal(err)
			}
			tableName, _ := PhysicalCatalogTable(catalogID)
			if table := schemaTable(t, schema, tableName); !hasSchemaColumn(table, "code", want.column, true) &&
				!hasSchemaColumn(table, "code", want.column, false) {
				t.Fatalf("code column is not %s: %+v", want.column, table.Columns)
			}
		})
	}
}

// The number of a document follows the same rule, and through the same shape:
// a document, a business process, a task and a numerator all describe their
// number with DocumentNumber, and the prototype gives the property to exactly
// those four.
func TestAllowedLengthDecidesTheNumberColumn(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		number DocumentNumber
		column string
	}{
		"переменная длина":    {DocumentNumber{Type: StringType, Length: 11}, "character varying(11)"},
		"фиксированная длина": {DocumentNumber{Type: StringType, Length: 11, FixedLength: true}, "character(11)"},
		"числовой номер":      {DocumentNumber{Type: NumberType, Length: 11}, "numeric(11,0)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			documentID := parseTestUUID(t, "50000000-0000-4000-8000-000000000031")
			catalog := &Catalog{
				Documents:    []DocumentDefinition{{ID: documentID, Name: "Накладная", Number: want.number}},
				documentByID: map[uuid.UUID]int{documentID: 0},
			}
			schema, err := catalog.ApplicationSchema()
			if err != nil {
				t.Fatal(err)
			}
			tableName, _ := PhysicalDocumentTable(documentID)
			table := schemaTable(t, schema, tableName)
			if !hasSchemaColumn(table, "number", want.column, true) && !hasSchemaColumn(table, "number", want.column, false) {
				t.Fatalf("number column is not %s: %+v", want.column, table.Columns)
			}
		})
	}
}

// Eight metadata objects have the property and the ninth pointedly does not.
// A chart of accounts takes the shape of its code from the code mask, and the
// prototype gives it no choice of length at all - so a chart that declares one
// is describing a setting the platform would not read.
func TestOnlyTheKindsThatHaveAllowedLengthMayDeclareIt(t *testing.T) {
	t.Parallel()
	t.Run("план счетов не выбирает длину кода", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
		writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false, fixed_length: true}
description_length: 120
code_mask: "@@.@@"
order_length: 5
`)
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "code mask") {
			t.Fatalf("err = %v", err)
		}
	})
	for name, body := range map[string]struct {
		kind    Kind
		id      string
		content string
	}{
		"справочник": {CatalogKind, catalogID, `format: 1
id: ` + catalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true, fixed_length: true}
description_length: 150
`},
		"план видов характеристик": {ChartOfCharacteristicTypesKind, characteristicsID, `format: 1
id: ` + characteristicsID + `
name: Свойства
title: {ru: Свойства}
code: {type: string, length: 9, auto: true, fixed_length: true}
description_length: 100
value_type: [{kind: string, length: 100}]
`},
		"план видов расчёта": {ChartOfCalculationTypesKind, calcTypesStandardID, `format: 1
id: ` + calcTypesStandardID + `
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 9, auto: true, fixed_length: true}
description_length: 100
`},
		"план обмена": {ExchangePlanKind, exchangePlanID, `format: 1
id: ` + exchangePlanID + `
name: Филиалы
title: {ru: Филиалы}
code: {type: string, length: 36, auto: false, fixed_length: true}
description_length: 150
`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, body.id, body.content)
			if _, err := Load(root); err != nil {
				t.Fatalf("a kind that has the property was refused: %v", err)
			}
		})
	}
}

// "Свойство имеет смысл, если тип номера - Строка." A numeric code or number is
// a count of digits; padding it with spaces on the right is not a thing that
// can be asked for, and a file that asks says so at once.
func TestAllowedLengthIsForStringsOnly(t *testing.T) {
	t.Parallel()
	t.Run("числовой код", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: number, length: 9, auto: true, fixed_length: true}
description_length: 150
`)
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "string codes only") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("числовой номер", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, DocumentKind, documentID, `format: 1
id: `+documentID+`
name: Накладная
title: {ru: Накладная}
number: {type: number, length: 9, auto: true, periodicity: year, fixed_length: true}
`)
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "string numbers only") {
			t.Fatalf("err = %v", err)
		}
	})
}

// The numerator was missing from the plan for this point, and it is the one
// place the omission would have shown: a document that shares its numbering
// declares no number of its own, so if the numerator cannot say the numbering
// is fixed, nobody can.
func TestNumeratorSaysWhetherItsNumberIsFixed(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, NumeratorKind, numeratorAllowedLengthID, `format: 1
id: `+numeratorAllowedLengthID+`
name: СквознаяНумерация
title: {ru: Сквозная нумерация}
number: {type: string, length: 11, unique: true, periodicity: year, fixed_length: true}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	numerator, ok := catalog.Numerator("сквознаянумерация")
	if !ok || !numerator.Number.FixedLength {
		t.Fatalf("numerator = %+v, found=%v", numerator.Number, ok)
	}
	// And a document that takes this numbering stores it the numerator's way:
	// the column is the numerator's business, because the number is.
	if column := documentNumberSQLType(numerator.Number); column != "character(11)" {
		t.Fatalf("shared number column = %s", column)
	}
}

const numeratorAllowedLengthID = "a0000000-0000-4000-8000-000000000031"
