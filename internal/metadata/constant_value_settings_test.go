package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func constantConfiguration() project.Project {
	return project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"},
		DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
}

// The defect this catches: a constant carried a name, a type and a default and
// nothing of what a value needs to be shown and entered. The prototype gives it
// the format, the tooltip, the mask, the bounds and the whole of choice - all
// 230 constants of the demo configuration carry them - and the reader refused
// the words, because the model had nowhere to put them.
//
// It was invisible by eye for a reason worth keeping in mind: a constant does
// have a field called `format` in our model, and it is the version of the
// metadata file, not the format the value is shown in. The two matched by name
// and the gap looked closed.
func TestConstantCarriesTheSettingsOfAValue(t *testing.T) {
	t.Parallel()
	source := `format: 1
id: ` + uuid.MustNew().String() + `
name: КурсПоУмолчанию
title: {ru: Курс по умолчанию}
comment: Курс, если в регистре ничего нет
types: [{kind: number, precision: 10, scale: 4}]
fill_checking: show-error
presentation:
  format: {ru: "ЧДЦ=4"}
  edit_format: {ru: "ЧДЦ=4; ЧН=0"}
  tooltip: {ru: Курс валюты}
  mark_negatives: true
  min_value: "0"
  max_value: "1000"
choice:
  quick_choice: use
  folders_and_items: items
`
	value, err := DecodeConstant("constant.yaml", strings.NewReader(source), constantConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if value.FillChecking != ShowFillingError {
		t.Fatalf("fill checking = %q", value.FillChecking)
	}
	if value.Presentation.Format["ru"] != "ЧДЦ=4" || !value.Presentation.MarkNegatives {
		t.Fatalf("presentation = %+v", value.Presentation)
	}
	if value.Presentation.MinValue == nil || value.Presentation.MaxValue == nil {
		t.Fatalf("bounds = %+v / %+v", value.Presentation.MinValue, value.Presentation.MaxValue)
	}
	if value.Presentation.ToolTip["ru"] == "" || value.Choice.QuickChoice != UsageUse {
		t.Fatalf("tooltip=%+v choice=%+v", value.Presentation.ToolTip, value.Choice)
	}
}

func TestConstantRefusesSettingsThatSayNothing(t *testing.T) {
	t.Parallel()
	head := `format: 1
id: ` + uuid.MustNew().String() + `
name: КурсПоУмолчанию
title: {ru: Курс по умолчанию}
types: [{kind: number, precision: 10, scale: 4}]
`
	cases := map[string]struct{ tail, message string }{
		// A bound of another type than the value is compared with nothing, so
		// it rejects nothing: the constant accepts what it was set up to refuse.
		"неизвестный режим проверки заполнения": {`fill_checking: сомневаться`, "fill_checking"},
		"неизвестный режим выбора":              {`choice: {quick_choice: иногда}`, "choice.quick_choice"},
		// A constant stands alone. A link takes the value of a choice parameter
		// from a sibling field, and a constant has no siblings - the link would
		// be read by nobody and the parameter would stay empty.
		"связь параметра выбора": {`choice:
  parameter_links:
    - {name: Отбор.Владелец, source: {attribute: ` + uuid.MustNew().String() + `}, change: clear}`, "is not a field of this object"},
		// The prototype gives a constant no indexing: there is one row of one
		// value, and nothing to find it by.
		"индексирование": {`indexing: index`, "indexing"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeConstant("constant.yaml", strings.NewReader(head+testCase.tail+"\n"), constantConfiguration())
			if err == nil {
				t.Fatal("the constant was accepted")
			}
			if !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("error = %q, expected it to name %q", err, testCase.message)
			}
		})
	}
}

// A copy of a constant shares nothing with the catalog: a caller that changes
// what it got must not change what the next caller gets.
func TestConstantCopyKeepsItsOwnSettings(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	source := `format: 1
id: ` + id.String() + `
name: КурсПоУмолчанию
title: {ru: Курс по умолчанию}
types: [{kind: number, precision: 10, scale: 4}]
presentation: {tooltip: {ru: Курс валюты}}
choice: {parameters: [{name: Отбор.Действует, values: [{kind: boolean, data: "true"}]}]}
`
	value, err := DecodeConstant("constant.yaml", strings.NewReader(source), constantConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{Constants: []Constant{value}, constantByID: map[uuid.UUID]int{id: 0}}
	first, ok := catalog.ConstantByID(id)
	if !ok {
		t.Fatal("the constant did not come back")
	}
	first.Presentation.ToolTip["ru"] = "изменено"
	first.Choice.Parameters[0].Name = "изменено"
	second, _ := catalog.ConstantByID(id)
	if second.Presentation.ToolTip["ru"] != "Курс валюты" || second.Choice.Parameters[0].Name != "Отбор.Действует" {
		t.Fatalf("the copy shared its settings: %+v / %+v", second.Presentation.ToolTip, second.Choice.Parameters)
	}
}
