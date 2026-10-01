package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The prototype treats an underscore as a letter: a name may start with one and
// hold any number of them (checked by the owner on the platform, 01.10.2026).
// A digit may follow the first character and nothing else may appear at all.
//
// Defect caught: a name with an underscore - first or inside - refused, which
// loses some 3600 names of the configurations being moved; and the opposite,
// a rule loosened into accepting a digit first or a hyphen.
func TestNameTreatsUnderscoreAsALetter(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"_100": true, "_50_50": true, "Имя_Объекта": true, "_": true, "Имя1": true,
		"1Имя": false, "Имя-1": false, "Имя 1": false, "": false,
	} {
		if got := validIdentifier(name); got != want {
			t.Errorf("validIdentifier(%q) = %v, want %v", name, got, want)
		}
	}
}

// An enumeration value named the way the prototype names one in erp and acc,
// and a catalog whose own name and attribute start with an underscore, load.
func TestUnderscoreFirstLoadsWhereThePrototypeKeepsIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, EnumerationKind, enumObject, `format: 1
id: `+enumObject+`
name: СпособыНачисления
title: {ru: Способы начисления}
values: [{id: `+enumValueOne+`, name: _100, title: {ru: 100%}}]
`)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: _Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9}
description_length: 150
attributes:
  - id: `+attributeID+`
    name: _1111
    title: {ru: Реквизит}
    types: [{kind: string, length: 10}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.CatalogDefinition("_Контрагенты"); !ok {
		t.Fatal("the catalog was not read")
	}
}

// A synonym left empty is saved by the prototype for every kind, and the name
// stands in wherever it is shown.
//
// Defect caught: an object, an attribute or a table part without a synonym
// refused at load (38/37/24 in the configurations being moved); and a form
// shown with an empty caption instead of the name.
func TestEmptySynonymIsAcceptedAndTheNameStandsIn(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Контрагенты
code: {type: string, length: 9}
description_length: 150
attributes:
  - id: `+attributeID+`
    name: ИНН
    types: [{kind: string, length: 12}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	form, err := catalog.CatalogForm("Контрагенты", ObjectForm, "ru")
	if err != nil {
		t.Fatal(err)
	}
	if form.Title != "Контрагенты" {
		t.Fatalf("form title = %q, want the name", form.Title)
	}
	var field string
	for _, item := range form.Fields {
		if item.Name == "ИНН" {
			field = item.Title
		}
	}
	if field != "ИНН" {
		t.Fatalf("attribute caption = %q, want the name", field)
	}
}

// A translation that is there must still say something: an empty synonym is a
// synonym with no translations, not one with a blank one.
func TestSynonymTranslationMustNotBeBlank(t *testing.T) {
	t.Parallel()
	issues := validateTitle("title", LocalizedText{"ru": "  "}, metadataConfiguration())
	if len(issues) == 0 {
		t.Fatal("a blank translation was accepted")
	}
	if issues := validateTitle("title", nil, metadataConfiguration()); len(issues) != 0 {
		t.Fatalf("no translations at all refused: %v", issues)
	}
}

// The folder an object lies in is named by the object, so the file system's
// limit on a path component applies - 255 bytes, not characters.
//
// Defect caught: the old limit of 128 bytes, which hid from Studio and
// publication every object with a Cyrillic name longer than 64 characters
// (141/89/48 in the configurations being moved); and a limit counted in
// characters, which lets through a name no file system can store.
func TestFolderNameIsLimitedByTheFileSystemInBytes(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("Я", 96) // the longest name of the three configurations
	if err := project.ObjectName(long); err != nil {
		t.Fatalf("a 96-character Cyrillic name refused: %v", err)
	}
	if err := project.ObjectName(strings.Repeat("Я", 127) + "_"); err != nil {
		t.Fatalf("255 bytes refused: %v", err)
	}
	if err := project.ObjectName(strings.Repeat("Я", 128)); err == nil {
		t.Fatal("256 bytes accepted")
	}
	if err := project.SubordinateName(strings.Repeat("Я", 128)); err == nil {
		t.Fatal("256 bytes accepted for a subordinate name")
	}
}

// A role naming a form by an identifier nothing carries - the form deleted and
// the role left behind, as in one report of erp - is carried, opens the
// generated form, and is listed for the import report.
//
// Defect caught: the stale role refusing the whole configuration at load; the
// stale role silently dropped; and a stale role taken for a form the object
// keeps.
func TestFormRoleNamingADeletedFormIsCarriedAndListed(t *testing.T) {
	t.Parallel()
	stale := uuid.MustNew()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9}
description_length: 150
forms: {object: `+stale.String()+`}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	unresolved := catalog.UnresolvedReferences()
	if len(unresolved) != 1 || unresolved[0].ID != stale || !strings.Contains(unresolved[0].Where, "Контрагенты") {
		t.Fatalf("unresolved = %+v, want the object form of Контрагенты", unresolved)
	}
	definition, _ := catalog.CatalogDefinition("Контрагенты")
	if definition.Forms.Object != stale.String() {
		t.Fatalf("the role was not carried as written: %q", definition.Forms.Object)
	}
	form, err := catalog.CatalogForm("Контрагенты", ObjectForm, "ru")
	if err != nil || !form.Generated {
		t.Fatalf("form = %+v, error = %v; want the generated form", form, err)
	}

}
