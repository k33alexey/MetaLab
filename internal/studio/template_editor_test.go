package studio

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
)

// TestTheConstructorMakesATemplate catches the constructor of a template
// making something else than it was told, or what the prototype's does not
// make: a spreadsheet named Макет with its synonym and comment and no
// content; binary data and an external component with the content loaded
// from a file, where the load reads it; HTML with no file. An Active
// document, a file loaded into a kind that takes none and a name taken are
// refused with nothing left behind; the kind is not changed after, while
// the name and the synonym are; the project loads with all of them.
func TestTheConstructorMakesATemplate(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	made, err := workspace.CreateTemplate(TemplateDraft{Name: "Макет", Title: metadata.LocalizedText{"ru": "Макет"}, Comment: "печать", Kind: metadata.SpreadsheetTemplate})
	if err != nil {
		t.Fatal(err)
	}
	if made.Path != "metadata/common-templates/Макет/object.yaml" || made.Template.Kind != "spreadsheet" || made.Template.Title["ru"] != "Макет" ||
		made.Template.Comment != "печать" || made.KindTitle != "Общий макет" {
		t.Fatalf("the spreadsheet made is %+v at %s", made.Template, made.Path)
	}
	folder := func(name string) string { return filepath.Join(workspace.root, "metadata", "common-templates", name) }
	if entries, _ := os.ReadDir(folder("Макет")); len(entries) != 1 {
		t.Errorf("a spreadsheet of no content keeps %d files, want its description alone", len(entries))
	}
	binary := []byte{0, 1, 2, 250}
	for _, item := range []struct {
		name string
		kind metadata.TemplateKind
	}{{"Данные", metadata.BinaryTemplate}, {"Компонента", metadata.AddInTemplate}} {
		if _, err := workspace.CreateTemplate(TemplateDraft{Name: item.name, Kind: item.kind, Content: binary}); err != nil {
			t.Fatalf("%s: %v", item.kind, err)
		}
		if content, err := os.ReadFile(filepath.Join(folder(item.name), "content.bin")); err != nil || !bytes.Equal(content, binary) {
			t.Errorf("%s keeps %v (%v), want the file loaded", item.kind, content, err)
		}
	}
	if _, err := workspace.CreateTemplate(TemplateDraft{Name: "Страница", Kind: metadata.HTMLTemplate}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "common-templates"))
	for _, refused := range []TemplateDraft{
		{Name: "Документ", Kind: metadata.ActiveDocument},
		{Name: "Текст", Kind: metadata.TextTemplate, Content: binary},
		{Name: "макет", Kind: metadata.SpreadsheetTemplate},
	} {
		if _, err := workspace.CreateTemplate(refused); err == nil {
			t.Errorf("%+v was made", refused)
		}
	}
	if after, _ := os.ReadDir(filepath.Join(workspace.root, "metadata", "common-templates")); len(after) != len(before) {
		t.Errorf("the refused templates left %d folders, want %d", len(after), len(before))
	}
	if answer, err := workspace.CheckName(NameCheck{Kind: "common-templates", Name: "МАКЕТ"}); err != nil || answer.Refusal != "Имя не уникально!" {
		t.Errorf("a name taken among the templates answered %q (%v)", answer.Refusal, err)
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		t.Fatalf("the project with the templates does not load: %v", err)
	}
	if template, ok := loaded.CommonTemplate("Компонента"); !ok || template.Kind != metadata.AddInTemplate {
		t.Errorf("the project reads Компонента as %+v", template)
	}

	changed := made.Template
	changed.Kind = metadata.TextTemplate
	if _, err := workspace.SaveTemplateEditor(made.Path, changed, made.Revision); err == nil {
		t.Error("the kind of a template was changed")
	}
	renamed := made.Template
	renamed.Name, renamed.Title = "ПечатьСчета", metadata.LocalizedText{"ru": "Печать счета"}
	saved, err := workspace.SaveTemplateEditor(made.Path, renamed, made.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Path != "metadata/common-templates/ПечатьСчета/object.yaml" || saved.Template.Title["ru"] != "Печать счета" {
		t.Errorf("renamed: %s %+v", saved.Path, saved.Template)
	}
}

// TestTheKindsOfTheConstructorAreTheModels catches a kind the constructor
// offers that the model does not take, or one of the ten left out: the
// list is the prototype's (obshchij-maket.md), written out here.
func TestTheKindsOfTheConstructorAreTheModels(t *testing.T) {
	t.Parallel()
	want := []string{"Табличный документ", "Текстовый документ", "Двоичные данные", "Active document", "HTML документ", "Географическая схема",
		"Графическая схема", "Схема компоновки данных", "Макет оформления компоновки данных", "Внешняя компонента"}
	offered := kinds.TemplateKinds()
	if len(offered) != len(want) {
		t.Fatalf("the constructor offers %d kinds, want %d", len(offered), len(want))
	}
	for index, kind := range offered {
		if kind[1] != want[index] {
			t.Errorf("kind %d is %q, want %q", index, kind[1], want[index])
		}
		if kind[0] != string(metadata.HTMLTemplate) && metadata.TemplateContentFile(metadata.TemplateKind(kind[0])) == "" {
			t.Errorf("%s is no kind of template the model keeps", kind[0])
		}
	}
}
