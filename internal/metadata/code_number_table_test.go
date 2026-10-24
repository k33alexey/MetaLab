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
	// codeZeroUntil and descriptionZeroUntil name the item of docs/BLOCKS.md
	// that brings the model to the table where it does not agree yet: until
	// it lands, a length of 0 there is taken with NoteLengthZeroUnchecked.
	codeZeroUntil        string
	descriptionZeroUntil string
	// auto and unique are «Автонумерация» and «Контроль уникальности».
	auto   bool
	unique bool
	// descriptionCeiling is «Длина наименования»; 0 is «—», no description.
	descriptionCeiling int
	// extra is what the kind cannot be read without.
	extra string
}

const untilDescriptionZero = "2б «длина наименования 0 у плана счетов и плана видов расчёта — сверить в конфигураторе»"

var codeNumberTable = map[string]codeNumberRow{
	"справочник": {kind: CatalogKind, numericCode: true, allowedLength: true,
		codeZero: true, descriptionZero: true, auto: true, unique: true, descriptionCeiling: 150},
	"план видов характеристик": {kind: ChartOfCharacteristicTypesKind, allowedLength: true,
		codeZero: true, descriptionZero: true, auto: true, unique: true, descriptionCeiling: 150,
		extra: "value_type: [{kind: string, length: 100}]\n"},
	"план счетов": {kind: ChartOfAccountsKind,
		codeZero: true, descriptionZeroUntil: untilDescriptionZero, unique: true, descriptionCeiling: 150},
	"план видов расчёта": {kind: ChartOfCalculationTypesKind, numericCode: true, allowedLength: true,
		codeZero: true, descriptionZeroUntil: untilDescriptionZero, descriptionCeiling: 150},
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
				if row.descriptionZero || row.descriptionZeroUntil != "" {
					lowest = 0
				}
				expect(t, "a description past the ceiling", false, fmt.Sprintf("description_length must be %d..%d", lowest, row.descriptionCeiling),
					plain, row.descriptionCeiling+1)
			}
			zero := func(t *testing.T, what string, taken bool, until string, field codeNumberField, description int) {
				t.Helper()
				if until == "" {
					catalog, err := load(t, field, description)
					switch {
					case taken && err != nil:
						t.Fatalf("%s of 0 is in the table and was refused: %v", what, err)
					case !taken && (err == nil || !strings.Contains(err.Error(), "length must be 1..")):
						t.Fatalf("%s of 0 is not in the table: error = %v", what, err)
					case taken && hasNote(catalog, NoteLengthZeroUnchecked):
						t.Fatalf("%s of 0 is in the table and was taken with a note", what)
					}
					return
				}
				if taken {
					t.Fatalf("%s of 0 is in the table, and nothing waits for %s", what, until)
				}
				catalog, err := load(t, field, description)
				if err != nil || !hasNote(catalog, NoteLengthZeroUnchecked) {
					t.Fatalf("%s of 0 waits for %s and is to be taken with a note: err = %v", what, until, err)
				}
			}
			zero(t, "the code or number", row.codeZero, row.codeZeroUntil, codeNumberField{}, description)
			if row.descriptionCeiling != 0 {
				zero(t, "the description", row.descriptionZero, row.descriptionZeroUntil, plain, 0)
			}
		})
	}
}

func hasNote(catalog *Catalog, kind NoteKind) bool {
	for _, note := range catalog.Notes() {
		if note.Kind == kind {
			return true
		}
	}
	return false
}
