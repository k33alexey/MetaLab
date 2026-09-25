package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	enumObject     = "cf000000-0000-4000-8000-000000000001"
	enumValueOne   = "cf000000-0000-4000-8000-000000000010"
	enumValueTwo   = "cf000000-0000-4000-8000-000000000011"
	enumListForm   = "cf000000-0000-4000-8000-000000000021"
	enumChoiceForm = "cf000000-0000-4000-8000-000000000022"
	enumAuxList    = "cf000000-0000-4000-8000-000000000023"
	enumAuxChoice  = "cf000000-0000-4000-8000-000000000024"
	enumCommand    = "cf000000-0000-4000-8000-000000000030"
	enumTemplate   = "cf000000-0000-4000-8000-000000000040"
)

// An enumeration is not only a list of values. It is shown, chosen from and
// acted upon like any other object, and everything that serves that - four
// forms, a manager module, commands and templates, the way it is picked - used
// to be missing from the model, which meant losing it at import in silence.
func TestEnumerationKeepsEverythingItIsShownAndChosenBy(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, EnumerationKind, enumObject, `format: 1
id: `+enumObject+`
name: СтатусыЗаказа
title: {ru: Статусы заказа}
comment: Жизненный цикл заказа покупателя
explanation: {ru: В каком состоянии находится заказ}
list_presentation: {ru: Статусы заказа}
extended_list_presentation: {ru: Статусы заказов покупателей}
choice_mode: from-form
choice_history_on_input: dont-use
use_standard_commands: true
forms:
  list: `+enumListForm+`
  choice: `+enumChoiceForm+`
  auxiliary_list: `+enumAuxList+`
  auxiliary_choice: `+enumAuxChoice+`
values:
  - {id: `+enumValueOne+`, name: Новый, title: {ru: Новый}, comment: Только что создан}
  - {id: `+enumValueTwo+`, name: Отгружен, title: {ru: Отгружен}}
commands:
  - id: `+enumCommand+`
    name: ОткрытьСписок
    title: {ru: Открыть список}
    group: navigation-panel-ordinary
templates:
  - {id: `+enumTemplate+`, name: Справка, title: {ru: Справка}, kind: text}
`)
	writeObjectModule(t, root, EnumerationKind, "СтатусыЗаказа", project.ManagerModuleFile)
	writeCommandModule(t, root, EnumerationKind, "СтатусыЗаказа", "ОткрытьСписок")
	for _, form := range []string{enumListForm, enumChoiceForm, enumAuxList, enumAuxChoice} {
		writeObjectForm(t, root, EnumerationKind, "СтатусыЗаказа", form)
	}
	writeTemplateContent(t, root, EnumerationKind, "СтатусыЗаказа", enumTemplate, "content.txt", "текст")

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	enumeration, ok := catalog.Enumeration("СтатусыЗаказа")
	if !ok {
		t.Fatal("the enumeration did not load")
	}
	switch {
	case enumeration.Comment == "" || len(enumeration.Explanation) == 0:
		t.Fatalf("the comment or the explanation was lost: %+v", enumeration)
	case len(enumeration.ListPresentation) == 0 || len(enumeration.ExtendedListPresentation) == 0:
		t.Fatalf("the presentations of the list were lost: %+v", enumeration)
	case enumeration.ChoiceMode != ChoiceFromForm:
		t.Fatalf("how a value is picked was lost: %+v", enumeration)
	case enumeration.ChoiceHistoryOnInput != ChoiceHistoryDontUse:
		t.Fatalf("the choice history setting was lost: %+v", enumeration)
	case !enumeration.UseStandardCommands:
		t.Fatalf("whether the platform offers its own commands was lost: %+v", enumeration)
	case enumeration.Forms.List == nil || enumeration.Forms.Choice == nil:
		t.Fatalf("the forms were lost: %+v", enumeration.Forms)
	case enumeration.Forms.AuxiliaryList == nil || enumeration.Forms.AuxiliaryChoice == nil:
		t.Fatalf("the auxiliary forms were lost: %+v", enumeration.Forms)
	case len(enumeration.Commands) != 1 || len(enumeration.Templates) != 1:
		t.Fatalf("the commands or the templates were lost: %+v", enumeration)
	case enumeration.Values[0].Comment != "Только что создан":
		t.Fatalf("the comment of a value was lost: %+v", enumeration.Values[0])
	}

	// The catalog hands out copies of an enumeration too.
	enumeration.Values[0].Name = "Подменено"
	enumeration.Commands[0].Name = "Подменено"
	again, _ := catalog.Enumeration("СтатусыЗаказа")
	if again.Values[0].Name != "Новый" || again.Commands[0].Name != "ОткрытьСписок" {
		t.Fatal("an enumeration was handed out by reference")
	}
}

// An enumeration keeps a folder of its own now, like every other object that
// has modules and forms of its own. Before this it was one flat file, and
// there was physically nowhere to put a form or the body of a command.
func TestEnumerationKeepsItsOwnFolder(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, EnumerationKind, enumObject, `format: 1
id: `+enumObject+`
name: СтатусыЗаказа
title: {ru: Статусы заказа}
values: [{id: `+enumValueOne+`, name: Новый, title: {ru: Новый}}]
`)
	description := filepath.Join(root, "metadata", string(EnumerationKind), "СтатусыЗаказа", "object.yaml")
	if _, err := os.Stat(description); err != nil {
		t.Fatalf("the enumeration was not written into a folder of its own: %v", err)
	}
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}

	// A form the enumeration declares has to be there, the same as anywhere
	// else - and it is looked for inside that folder.
	writeMetadata(t, root, EnumerationKind, enumObject, `format: 1
id: `+enumObject+`
name: СтатусыЗаказа
title: {ru: Статусы заказа}
forms: {auxiliary_choice: `+enumAuxChoice+`}
values: [{id: `+enumValueOne+`, name: Новый, title: {ru: Новый}}]
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a form that does not exist was accepted")
	}
	writeObjectForm(t, root, EnumerationKind, "СтатусыЗаказа", enumAuxChoice)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}

// What is checked in an enumeration is what its properties mean together.
func TestBrokenEnumerationsAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"способ выбора не существует": {"choice_mode: по-желанию",
			"choice_mode must be both-ways, from-form or quick-choice"},
		"история выбора не существует": {"choice_history_on_input: иногда",
			"choice_history_on_input must be auto, use or dont-use"},
		"быстрый выбор при выборе из формы": {"choice_mode: from-form\nquick_choice: true",
			"quick_choice contradicts choice_mode from-form"},
		"форма нулевая": {"forms: {auxiliary_list: 00000000-0000-0000-0000-000000000000}",
			"forms.auxiliary_list must be a non-zero UUID"},
		"вида макета не существует": {"templates:\n  - {id: " + enumTemplate +
			", name: Макет, title: {ru: Макет}, kind: слайды}",
			"templates[0].kind is not a kind of template"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeEnumeration("object.yaml", strings.NewReader(`format: 1
id: `+enumObject+`
name: СтатусыЗаказа
title: {ru: Статусы заказа}
values: [{id: `+enumValueOne+`, name: Новый, title: {ru: Новый}}]
`+broken.body+`
`), metadataManifest())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// writeObjectForm writes an empty managed form file where an object keeps its
// forms.
func writeObjectForm(t *testing.T, root string, kind Kind, objectName, formID string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", string(kind), objectName, "forms")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, formID+".yaml"), []byte("format: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
