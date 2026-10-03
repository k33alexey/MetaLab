package metadata

import (
	"strings"
	"testing"
)

const choiceID = "40000000-0000-4000-8000-000000000081"

// The whole set on a catalog: how a row is edited, which of its two names stands
// for it, and the three settings of how one is picked.
func TestCatalogKeepsHowItIsEnteredAndPicked(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, choiceID, `format: 1
id: `+choiceID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
edit_type: both-ways
default_presentation: as-code
quick_choice: true
choice_mode: quick-choice
create_on_input: use
choice_history_on_input: dont-use
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("номенклатура")
	if !ok {
		t.Fatal("the catalog was not read")
	}
	if definition.EditType != EditBothWays || definition.DefaultPresentation != PresentAsCode ||
		!definition.QuickChoice || definition.ChoiceMode != ChoiceQuickOnly ||
		definition.CreateOnInput != UsageUse || definition.ChoiceHistoryOnInput != ChoiceHistoryDontUse {
		t.Fatalf("choice = %+v", definition.ObjectChoice)
	}
}

// All five reference kinds carry the whole set. They reach it through one shared
// shape, so this guards the wiring rather than each kind's own code.
func TestEveryReferenceKindCarriesTheWholeChoice(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		content string
	}{
		"справочник": {CatalogKind, `code: {type: string, length: 9, auto: true}
description_length: 150
`},
		"план видов характеристик": {ChartOfCharacteristicTypesKind, `code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
`},
		"план счетов": {ChartOfAccountsKind, `code: {type: string, length: 5, auto: false}
description_length: 120
`},
		"план видов расчёта": {ChartOfCalculationTypesKind, `code: {type: string, length: 9}
description_length: 100
`},
		"план обмена": {ExchangePlanKind, `code: {type: string, length: 36, auto: false}
description_length: 150
`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, choiceID, `format: 1
id: `+choiceID+`
name: Объект
title: {ru: Объект}
`+body.content+`edit_type: in-list
default_presentation: as-description
quick_choice: true
choice_mode: both-ways
create_on_input: dont-use
choice_history_on_input: auto
`)
			if _, err := Load(root); err != nil {
				t.Fatalf("a reference kind refused part of its choice: %v", err)
			}
		})
	}
}

// A task has no code. Its presentation is therefore not the reference kinds'
// code-or-description but number-or-description, and the prototype gives it an
// enumeration of its own for exactly that reason. Accepting "as-code" would let
// a configuration ask for a name the object has not got.
func TestATaskStandsForItselfByNumberNotCode(t *testing.T) {
	t.Parallel()
	taskBody := func(presentation string) string {
		return `format: 1
id: ` + choiceID + `
name: Поручение
title: {ru: Поручение}
number: {type: string, length: 11, auto: true, periodicity: none}
description_length: 150
addressing_attributes: []
default_presentation: ` + presentation + `
`
	}
	for name, want := range map[string]struct {
		value   string
		accepts bool
	}{
		"как номер":        {"as-number", true},
		"как наименование": {"as-description", true},
		"как код — нельзя": {"as-code", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, TaskKind, choiceID, taskBody(want.value))
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("refused: %v", err)
			case !want.accepts && (err == nil || !strings.Contains(err.Error(), "as-number or as-description")):
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// What each numbered kind has and has not. A business process is edited and
// picked but stands for nothing in one line; a document is not edited in a list
// either. Fields the prototype does not give are refused when the file is read,
// because they are not there to be read.
func TestNumberedKindsCarryOnlyWhatTheyHave(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		kind    Kind
		body    string
		setting string
		accepts bool
	}{
		"бизнес-процесс редактируется": {BusinessProcessKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "edit_type: in-dialog", true},
		"у бизнес-процесса нет представления": {BusinessProcessKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "default_presentation: as-code", false},
		"у бизнес-процесса нет быстрого выбора": {BusinessProcessKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "quick_choice: true", false},
		"документ создаётся при вводе": {DocumentKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "create_on_input: use", true},
		"документ в списке не правят": {DocumentKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "edit_type: in-list", false},
		"у документа нет режима выбора": {DocumentKind,
			"number: {type: string, length: 11, auto: true, periodicity: year}\n", "choice_mode: both-ways", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, want.kind, choiceID, `format: 1
id: `+choiceID+`
name: Объект
title: {ru: Объект}
`+want.body+want.setting+"\n")
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("refused a setting this kind has: %v", err)
			case !want.accepts && err == nil:
				t.Fatalf("accepted %q on a kind that has no such setting", want.setting)
			}
		})
	}
}

// Quick choice under «choose only from a form» does nothing, and the
// configurator writes it there as it writes the posting settings of a
// document that does not post. It is carried and noted on the five reference
// kinds and on an enumeration alike, by the same code, so the two cannot
// drift.
//
// Defect caught: a configuration refused over a flag that does nothing, on one
// kind and not the other.
func TestQuickChoiceUnderChoosingOnlyFromAFormIsCarried(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		content string
	}{
		"справочник": {CatalogKind, `code: {type: string, length: 9, auto: true}
description_length: 150
`},
		"перечисление": {EnumerationKind, `values:
  - id: ` + enumValueID + `
    name: Новый
    title: {ru: Новый}
`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, choiceID, `format: 1
id: `+choiceID+`
name: Объект
title: {ru: Объект}
`+body.content+`choice_mode: from-form
quick_choice: true
`)
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			var noted bool
			for _, note := range catalog.Notes() {
				noted = noted || note.Kind == NoteQuickChoiceUnderFormChoice
			}
			if !noted {
				t.Fatal("quick choice under choosing from a form was accepted without a note")
			}
		})
	}
}

// Each of the four enumerations takes the prototype's values and no others. A
// misspelling read as silence would look like the platform's own default.
func TestChoiceSettingsTakeOnlyThePrototypesValues(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		setting string
		message string
	}{
		"способ редактирования":  {"edit_type: in-form", "edit_type must be"},
		"основное представление": {"default_presentation: as-name", "default_presentation must be as-code or as-description"},
		"режим выбора":           {"choice_mode: sometimes", "choice_mode must be"},
		"создание при вводе":     {"create_on_input: maybe", "create_on_input must be auto, use or dont-use"},
		"история выбора":         {"choice_history_on_input: never", "choice_history_on_input must be auto, use or dont-use"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, choiceID, `format: 1
id: `+choiceID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
`+want.setting+"\n")
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), want.message) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
