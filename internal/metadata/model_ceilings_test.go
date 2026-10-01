package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The ceilings the model keeps are the prototype's, each checked in the
// designer by the owner on 01.10.2026, and each is tested on both sides: the
// defect on one side is a configuration the prototype saves refused on import,
// on the other a value the prototype never lets through. The ceilings that had
// no source are gone, and a value past each of them is now taken.

func ceilingCatalog(code, description string) string {
	return `format: 1
id: ` + catalogID + `
name: Товары
title: {ru: Товары}
code: ` + code + `
description_length: ` + description + `
`
}

func TestTheCeilingsOfTheModelHoldOnBothSides(t *testing.T) {
	t.Parallel()
	decode := func(body string) error {
		_, err := DecodeCatalog("object.yaml", strings.NewReader(body), metadataConfiguration())
		return err
	}
	for name, testCase := range map[string]struct {
		body    string
		refused bool
		says    string
	}{
		"строковый код 50":    {ceilingCatalog("{type: string, length: 50}", "100"), false, ""},
		"строковый код 51":    {ceilingCatalog("{type: string, length: 51}", "100"), true, "code.length must be 1..50"},
		"числовой код 50":     {ceilingCatalog("{type: number, length: 50}", "100"), false, ""},
		"числовой код 51":     {ceilingCatalog("{type: number, length: 51}", "100"), true, "code.length must be 1..50"},
		"наименование 150":    {ceilingCatalog("{type: string, length: 9}", "150"), false, ""},
		"наименование 151":    {ceilingCatalog("{type: string, length: 9}", "151"), true, "description_length must be 1..150"},
		"десять уровней":      {ceilingCatalog("{type: string, length: 9}", "100") + "hierarchy: {enabled: true, kind: items, limit_levels: true, level_count: 10}\n", false, ""},
		"одиннадцать уровней": {ceilingCatalog("{type: string, length: 9}", "100") + "hierarchy: {enabled: true, kind: items, limit_levels: true, level_count: 11}\n", true, "level_count must be 1..10"},
		"имя 255":             {strings.Replace(ceilingCatalog("{type: string, length: 9}", "100"), "name: Товары", "name: Т"+strings.Repeat("о", 254), 1), false, ""},
		"имя 256":             {strings.Replace(ceilingCatalog("{type: string, length: 9}", "100"), "name: Товары", "name: Т"+strings.Repeat("о", 255), 1), true, "name must not exceed 255 characters"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := decode(testCase.body)
			if !testCase.refused {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.says) {
				t.Fatalf("error = %v, expected it to say %q", err, testCase.says)
			}
		})
	}

	// The number of a document has the code's ceiling.
	for _, number := range []struct {
		kind    string
		length  int
		refused bool
	}{{"string", 50, false}, {"string", 51, true}, {"number", 50, false}, {"number", 51, true}} {
		issues := validateNumberShape(DocumentNumber{Type: TypeKind(number.kind), Length: number.length, Periodicity: NumberPeriodNone})
		if refused := len(issues) != 0; refused != number.refused {
			t.Fatalf("%s number of %d: refused = %v, %v", number.kind, number.length, refused, issues)
		}
	}

	// A field's own type: a limited string up to 1024, 0 being unlimited, a
	// number of up to 32 digits with up to as many after the point.
	for name, testCase := range map[string]struct {
		item    Type
		refused bool
	}{
		"строка без ограничения": {Type{Kind: StringType}, false},
		"строка 1024":            {Type{Kind: StringType, Length: 1024}, false},
		"строка 1025":            {Type{Kind: StringType, Length: 1025}, true},
		"число 32 и 32":          {Type{Kind: NumberType, Precision: 32, Scale: 32}, false},
		"число 33":               {Type{Kind: NumberType, Precision: 33}, true},
		"дробь длиннее числа":    {Type{Kind: NumberType, Precision: 2, Scale: 3}, true},
	} {
		issues := validateTypes("types", []Type{testCase.item}, uuid.MustNew())
		if refused := len(issues) != 0; refused != testCase.refused {
			t.Fatalf("%s: refused = %v, %v", name, refused, issues)
		}
	}
}

// An exchange plan takes a longer description than a catalog - 250 in the
// configurations being moved - and the ceiling of 150 must not reach it.
func TestAnExchangePlanTakesALongerDescription(t *testing.T) {
	t.Parallel()
	_, err := DecodeExchangePlan("object.yaml", strings.NewReader(`format: 1
id: `+exchangePlanID+`
name: Филиалы
title: {ru: Филиалы}
code: {type: string, length: 36}
description_length: 250
`), metadataConfiguration())
	if err != nil {
		t.Fatalf("a description of 250 on an exchange plan was refused: %v", err)
	}
}

// The ceilings that had no source are gone: the help names none, and a value
// past each of them is taken.
func TestCeilingsWithoutASourceAreGone(t *testing.T) {
	t.Parallel()
	if _, err := DecodeScheduledJob("job.yaml", strings.NewReader(`format: 1
id: `+jobID+`
name: Задание
title: {ru: Задание}
module: `+jobModule+`
procedure: Загрузить
restart_count_on_failure: 5000
restart_interval_on_failure: 172800
`), metadataConfiguration()); err != nil {
		t.Fatalf("a job repeating more than the old ceiling was refused: %v", err)
	}
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Роль", Title: LocalizedText{"ru": "Роль"}}
	for range 10_001 {
		role.Commands = append(role.Commands, CommandPermission{Form: uuid.MustNew(), Command: uuid.MustNew()})
	}
	if err := ValidateRole("role.yaml", role, metadataConfiguration()); err != nil {
		t.Fatalf("a role of more than ten thousand permissions was refused: %v", err)
	}
	if issues := validateExpressionParameters("expression_in_data_source", strings.Repeat("a", 70_000)); len(issues) != 0 {
		t.Fatalf("a long expression was refused: %v", issues)
	}
}
