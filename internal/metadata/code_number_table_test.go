package metadata

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// codeNumberRow is one row of the table «Код, номер, наименование» of the
// requirements (METADATA-OBJECTS.md), written out here by hand and not read
// from the descriptors the model keeps: a test that took its expectation from
// referenceObjectShape would agree with whatever the shape says.
type codeNumberRow struct {
	kind Kind
	// numbered says the field is a number, not a code: a document, a business
	// process and a task. Their automatic numbering and the check for repeats
	// are described in the section of each kind, not in this table.
	numbered bool
	// numericCode is the column «Тип кода»: number or string, or always a
	// string. A numbered kind has «—» there.
	numericCode bool
	// allowedLength is «Допустимая длина»: the choice between a fixed and a
	// variable code or number.
	allowedLength bool
	// codeZero and descriptionZero are «Длина 0»: a length of 0 is taken and
	// switches the field off.
	codeZero        bool
	descriptionZero bool
	// auto and unique are «Автонумерация» and «Контроль уникальности».
	auto   bool
	unique bool
	// descriptionCeiling is «Длина наименования»; 0 is «—», no description.
	descriptionCeiling int
	// extra is what the kind cannot be read without.
	extra string
}

var codeNumberTable = map[string]codeNumberRow{
	"справочник": {kind: CatalogKind, numericCode: true, allowedLength: true,
		codeZero: true, descriptionZero: true, auto: true, unique: true, descriptionCeiling: 150},
	"план видов характеристик": {kind: ChartOfCharacteristicTypesKind, allowedLength: true,
		codeZero: true, descriptionZero: true, auto: true, unique: true, descriptionCeiling: 150,
		extra: "value_type: [{kind: string, length: 100}]\n"},
	"план счетов": {kind: ChartOfAccountsKind,
		codeZero: true, descriptionZero: true, unique: true, descriptionCeiling: 150},
	"план видов расчёта": {kind: ChartOfCalculationTypesKind, numericCode: true, allowedLength: true,
		codeZero: true, descriptionZero: true, descriptionCeiling: 150},
	// The designer refuses a code and a description of length 0 on an
	// exchange plan (opis of 15.10.2026, plan-obmena.md).
	"план обмена":    {kind: ExchangePlanKind, allowedLength: true, descriptionCeiling: 250},
	"документ":       {kind: DocumentKind, numbered: true, allowedLength: true, codeZero: true},
	"бизнес-процесс": {kind: BusinessProcessKind, numbered: true, allowedLength: true, codeZero: true, extra: processTaskLine},
	// 0 taken (opis of the task, 15.10.2026, docs/block-00/zadacha.md).
	"задача": {kind: TaskKind, numbered: true, allowedLength: true, codeZero: true,
		descriptionZero: true, descriptionCeiling: 150, extra: "addressing_attributes: []\n"},
}

// codeNumberField is what one case writes into the code or the number.
type codeNumberField struct {
	numeric, fixed, auto, unique bool
	length                       int
}

// body writes an object of the row's kind with the code or number given and
// the description length given; a description of -1 is left out.
func (row codeNumberRow) body(id string, field codeNumberField, description int) string {
	kind := "string"
	if field.numeric {
		kind = "number"
	}
	var text strings.Builder
	fmt.Fprintf(&text, "format: 1\nid: %s\nname: Объект\ntitle: {ru: Объект}\n", id)
	if row.numbered {
		fmt.Fprintf(&text, "number: {type: %s, length: %d, auto: true, periodicity: none, fixed_length: %v}\n", kind, field.length, field.fixed)
	} else {
		fmt.Fprintf(&text, "code: {type: %s, length: %d, auto: %v, unique: %v, fixed_length: %v}\n",
			kind, field.length, field.auto, field.unique, field.fixed)
	}
	if description >= 0 {
		fmt.Fprintf(&text, "description_length: %d\n", description)
	}
	return text.String() + row.extra
}

// TestCodeNumberAndDescriptionFollowTheTable walks the kinds with a code or a
// number and checks each column of the table against what the model takes:
// the type of the code, the choice of a fixed length, a length of 0, automatic
// codes, the check for repeats, the ceiling of the code or number (50 at every
// kind) and of the description.
//
// Defect caught: one kind's flag in its descriptor drifting from the table -
// a numeric code taken where the code is always a string, a property refused
// where the prototype gives it, a ceiling of the catalog put on an exchange
// plan, a length of 0 taken silently where the table refuses it.
func TestCodeNumberAndDescriptionFollowTheTable(t *testing.T) {
	t.Parallel()
	for name, row := range codeNumberTable {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			description := row.descriptionCeiling
			if description == 0 {
				description = -1
			}
			load := func(t *testing.T, field codeNumberField, description int) (*Catalog, error) {
				t.Helper()
				root := metadataProject(t)
				id := uuid.MustNew().String()
				writeProcessTask(t, root, row.extra)
				writeMetadata(t, root, row.kind, id, row.body(id, field, description))
				return Load(root)
			}
			// says is what the refusal names, so that a case refused for some
			// other reason does not pass for this one.
			expect := func(t *testing.T, what string, taken bool, says string, field codeNumberField, description int) {
				t.Helper()
				_, err := load(t, field, description)
				if taken && err != nil {
					t.Fatalf("%s is in the table and was refused: %v", what, err)
				}
				if !taken && (err == nil || !strings.Contains(err.Error(), says)) {
					t.Fatalf("%s is not in the table: error = %v, expected it to say %q", what, err, says)
				}
			}
			field := "code"
			if row.numbered {
				field = "number"
			}
			plain := codeNumberField{length: 9}
			expect(t, "a length of 50", true, "", codeNumberField{length: 50}, description)
			expect(t, "a length of 51", false, field+".length must be", codeNumberField{length: 51}, description)
			if !row.numbered {
				expect(t, "a numeric code", row.numericCode, "code.type must be string", codeNumberField{length: 9, numeric: true}, description)
				expect(t, "automatic codes", row.auto, "code.auto", codeNumberField{length: 9, auto: true}, description)
				expect(t, "the check for repeats", row.unique, "code.unique", codeNumberField{length: 9, unique: true}, description)
			}
			expect(t, "a fixed length", row.allowedLength, "fixed_length", codeNumberField{length: 9, fixed: true}, description)
			if row.descriptionCeiling == 0 {
				expect(t, "a description", false, "description_length", plain, 100)
			} else {
				expect(t, "the ceiling of the description", true, "", plain, row.descriptionCeiling)
				lowest := 1
				if row.descriptionZero {
					lowest = 0
				}
				expect(t, "a description past the ceiling", false, fmt.Sprintf("description_length must be %d..%d", lowest, row.descriptionCeiling),
					plain, row.descriptionCeiling+1)
			}
			zero := func(t *testing.T, what string, taken bool, field codeNumberField, description int) {
				t.Helper()
				catalog, err := load(t, field, description)
				switch {
				case taken && err != nil:
					t.Fatalf("%s of 0 is in the table and was refused: %v", what, err)
				case !taken && (err == nil || !strings.Contains(err.Error(), "length must be 1..")):
					t.Fatalf("%s of 0 is not in the table: error = %v", what, err)
				case taken && len(catalog.Notes()) != 0:
					t.Fatalf("%s of 0 is in the table and was taken with a note: %+v", what, catalog.Notes())
				}
			}
			zero(t, "the code or number", row.codeZero, codeNumberField{}, description)
			if row.descriptionCeiling != 0 {
				zero(t, "the description", row.descriptionZero, plain, 0)
			}
		})
	}
}

// A chart of calculation types with no description is not presented by it:
// the window of the chart turns the main presentation to «В виде кода» itself
// when the description gets length 0 (opis of 24.10.2026,
// plan-vidov-rascheta.md, «Досъёмка»). Presented by the code, or with no
// presentation said, it is taken; a description of length 0 on a chart of
// accounts is taken with no note. The expectation is written here. Defects
// caught: a chart with no description presented by it taken; one presented by
// its code refused; the note of a length 0 left in place.
func TestAChartOfCalculationTypesWithNoDescriptionIsPresentedByItsCode(t *testing.T) {
	t.Parallel()
	decode := func(presentation string) error {
		_, err := DecodeChartOfCalculationTypes("object.yaml", strings.NewReader("format: 1\nid: e7000000-0000-4000-8000-000000000001"+
			"\nname: Начисления\ntitle: {ru: Начисления}\ncode: {type: string, length: 5}\ndescription_length: 0\n"+presentation), metadataConfiguration())
		return err
	}
	if err := decode("default_presentation: as-description\n"); err == nil ||
		!strings.Contains(err.Error(), "default_presentation as-description needs a description, and description_length is 0") {
		t.Fatalf("presented by a description it has not: %v", err)
	}
	for _, presentation := range []string{"default_presentation: as-code\n", ""} {
		if err := decode(presentation); err != nil {
			t.Fatalf("%q: refused: %v", presentation, err)
		}
	}
	root := metadataProject(t)
	id := "e7000000-0000-4000-8000-000000000002"
	writeMetadata(t, root, ChartOfAccountsKind, id, "format: 1\nid: "+id+"\nname: Хозрасчетный\ntitle: {ru: Хозрасчетный}\n"+
		"code: {type: string, length: 9}\ndescription_length: 0\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a chart of accounts with no description was refused: %v", err)
	}
	if notes := catalog.Notes(); len(notes) != 0 {
		t.Fatalf("a chart of accounts with no description carries notes: %+v", notes)
	}
}
