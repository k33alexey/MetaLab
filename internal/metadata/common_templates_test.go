package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const commonTemplateID = "cf000000-0000-4000-8000-000000000001"

// writeCommonTemplate writes one common template - a template belonging to no
// object, whose own folder is the template.
func writeCommonTemplate(t *testing.T, root, id, name string, kind TemplateKind) {
	t.Helper()
	writeMetadata(t, root, CommonTemplateKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
kind: `+string(kind)+`
`)
}

// writeCommonTemplateContent writes one file of a common template's content,
// beside the description rather than one level down: nothing owns the
// template, so its folder is the template itself.
func writeCommonTemplateContent(t *testing.T, root, name, file, content string) {
	t.Helper()
	path, err := project.CommonTemplateContentPath(name, file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A common template is the same thing as an object's template, so all ten
// kinds are carried here too, each with the content its kind says. A kind the
// model does not know would be a template lost without a word.
func TestCommonTemplateOfEveryKindIsCarriedWithItsContent(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	kinds := []TemplateKind{SpreadsheetTemplate, TextTemplate, BinaryTemplate, HTMLTemplate,
		CompositionSchema, CompositionAppearance, GeographicalSchema, GraphicalSchema, ActiveDocument, AddInTemplate}
	names := map[TemplateKind]string{}
	for index, kind := range kinds {
		name := "ОбщийМакет" + string('A'+rune(index))
		names[kind] = name
		writeCommonTemplate(t, root, templateIdentifier(index), name, kind)
		switch kind {
		case HTMLTemplate:
			writeCommonTemplateContent(t, root, name, "ru.html", "<p>Привет</p>")
			writeCommonTemplateContent(t, root, name, "uk.html", "<p>Привіт</p>")
		default:
			writeCommonTemplateContent(t, root, name, kind.contentFile(), "содержимое")
		}
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.CommonTemplates) != len(kinds) {
		t.Fatalf("%d common templates of %d survived: %+v", len(catalog.CommonTemplates), len(kinds), catalog.CommonTemplates)
	}
	for kind, name := range names {
		template, ok := catalog.CommonTemplate(name)
		if !ok {
			t.Fatalf("common template %s was lost", name)
		}
		if template.Kind != kind {
			t.Fatalf("common template %s came back as a %s", name, template.Kind)
		}
		if _, byID := catalog.CommonTemplateByID(template.ID); !byID {
			t.Fatalf("common template %s is not found by its identifier", name)
		}
	}

	// The catalog hands out copies: a template read out of it and changed does
	// not change the one the catalog holds.
	template, _ := catalog.CommonTemplate(names[SpreadsheetTemplate])
	template.Name = "Подменено"
	again, _ := catalog.CommonTemplate(names[SpreadsheetTemplate])
	if again.Name == "Подменено" {
		t.Fatal("common template lookup exposed mutable metadata")
	}
}

// A template with no content at all is ordinary: no editor writes content yet,
// and the reference configuration carries templates whose folder is empty.
func TestCommonTemplateWithoutContentLoads(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonTemplate(t, root, commonTemplateID, "ПустойМакет", SpreadsheetTemplate)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.CommonTemplate("ПустойМакет"); !ok {
		t.Fatal("a common template without content was lost")
	}
}

func TestCommonTemplateFolderIsChecked(t *testing.T) {
	t.Parallel()
	for name, write := range map[string]func(t *testing.T, root string){
		"содержимое чужого вида": func(t *testing.T, root string) {
			writeCommonTemplate(t, root, commonTemplateID, "Макет", TextTemplate)
			writeCommonTemplateContent(t, root, "Макет", "content.bin", "двоичное")
		},
		"лишний файл рядом с описанием": func(t *testing.T, root string) {
			writeCommonTemplate(t, root, commonTemplateID, "Макет", SpreadsheetTemplate)
			writeCommonTemplateContent(t, root, "Макет", "Замётка.txt", "заметка")
		},
		"макет лежит в чужой папке": func(t *testing.T, root string) {
			writeCommonTemplate(t, root, commonTemplateID, "Макет", SpreadsheetTemplate)
			from := filepath.Join(root, "metadata", string(CommonTemplateKind), "Макет")
			if err := os.Rename(from, filepath.Join(filepath.Dir(from), "ДругойМакет")); err != nil {
				t.Fatal(err)
			}
		},
		"неизвестный вид макета": func(t *testing.T, root string) {
			writeMetadata(t, root, CommonTemplateKind, commonTemplateID, `format: 1
id: `+commonTemplateID+`
name: Макет
title: {ru: Макет}
kind: свиток
`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			write(t, root)
			if _, err := Load(root); err == nil {
				t.Fatal("the project loaded")
			}
		})
	}
}

// Two common templates under one name are two things a developer cannot tell
// apart, and one of them would be unreachable.
func TestCommonTemplateNamesAreUnique(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonTemplate(t, root, templateIdentifier(0), "Макет", SpreadsheetTemplate)
	writeMetadata(t, root, CommonTemplateKind, templateIdentifier(1), `format: 1
id: `+templateIdentifier(1)+`
name: макет
title: {ru: Макет}
kind: text
`)
	// The second one lies in a folder of its own, because a file system that
	// ignores case would otherwise refuse to hold both.
	from := filepath.Join(root, "metadata", string(CommonTemplateKind), "макет")
	if err := os.Rename(from, filepath.Join(filepath.Dir(from), "Второй")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "макет") {
		t.Fatalf("two common templates under one name loaded: %v", err)
	}
}
