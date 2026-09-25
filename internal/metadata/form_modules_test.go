package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	formModuleCatalog = "d6000000-0000-4000-8000-000000000001"
	formModuleForm    = "d6000000-0000-4000-8000-000000000010"
)

// writeFormModule writes the module of one form beside the form itself, under
// the name of the role it plays. Nothing declares it: the file is all there is.
func writeFormModule(t *testing.T, root string, kind Kind, object, form, source string) {
	t.Helper()
	path := filepath.Join(root, "metadata", string(kind), object, "forms", form, project.FormModuleFile)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

// formModuleProject writes one catalog with one form.
func formModuleProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, formModuleCatalog, `format: 1
id: `+formModuleCatalog+`
name: Товары
title: {ru: Товары}
code: {type: string, length: 9, auto: true}
description_length: 150
forms: {object: ФормаЭлемента}
`)
	writeObjectForm(t, root, CatalogKind, "Товары", "ФормаЭлемента", formModuleForm)
	return root
}

// A form's module lies beside the form, under the name of its role, and is
// declared by lying there. It is compiled under a name that says both whose
// form it is and which form: two objects may well each keep a ФормаСписка, and
// one name for both would be one module overwriting the other.
func TestFormModuleIsTheFileBesideTheForm(t *testing.T) {
	t.Parallel()
	root := formModuleProject(t)
	writeFormModule(t, root, CatalogKind, "Товары", "ФормаЭлемента",
		"&НаКлиенте\nПроцедура Заполнить(Команда)\nКонецПроцедуры\n")

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	want := "metadata/catalogs/Товары/forms/ФормаЭлемента/" + project.FormModuleFile
	for _, module := range modules {
		if module.Filename != want {
			continue
		}
		if module.Name != "МодульФормыСправочника.Товары.ФормаЭлемента" {
			t.Fatalf("the form module was compiled as %q", module.Name)
		}
		return
	}
	t.Fatalf("the form module was not read at all: %+v", modules)
}

// A form of an object may not name a module. Naming one would be a second
// place to keep in step with the file, and the two disagree the first time
// either moves - which is the whole reason the identifier went away.
func TestFormThatNamesAModuleIsRefused(t *testing.T) {
	t.Parallel()
	root := formModuleProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Товары", "forms", "ФормаЭлемента", project.FormMetadataFile)
	body := "format: 1\nid: " + formModuleForm + "\nname: ФормаЭлемента\ntitle: {ru: Форма}\nkind: object\n" +
		"module: d6000000-0000-4000-8000-000000000020\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("a form naming its own module was accepted")
	}
	if !strings.Contains(err.Error(), "names a module") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// The folder of a form holds the form and its module, and nothing else. A
// third file there is either a module misspelled, which would silently never
// run, or something that does not belong in the configuration at all.
func TestFormFolderHoldsOnlyTheFormAndItsModule(t *testing.T) {
	t.Parallel()
	for name, file := range map[string]string{
		"модуль с опечаткой": "МодульФорма.bsl",
		"чужой модуль":       project.ObjectModuleFile,
		"посторонний файл":   "заметки.txt",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formModuleProject(t)
			writeFormModule(t, root, CatalogKind, "Товары", "ФормаЭлемента", "// модуль\n")
			stray := filepath.Join(root, "metadata", string(CatalogKind), "Товары", "forms", "ФормаЭлемента", file)
			if err := os.WriteFile(stray, []byte("// стороннее\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), "a form keeps only its description and its module") {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// A form with no module is an ordinary form: it simply has no code of its own.
func TestFormWithoutAModuleIsFine(t *testing.T) {
	t.Parallel()
	if _, err := Load(formModuleProject(t)); err != nil {
		t.Fatal(err)
	}
}
