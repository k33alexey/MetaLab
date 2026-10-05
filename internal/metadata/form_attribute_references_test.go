package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	refForm      = "c0de0000-0000-4000-8000-000000000101"
	refAttribute = "c0de0000-0000-4000-8000-000000000102"
	refColumn    = "c0de0000-0000-4000-8000-000000000103"
	refExtra     = "c0de0000-0000-4000-8000-000000000104"
	refCommonAt  = "c0de0000-0000-4000-8000-000000000105"
	refFilter    = "c0de0000-0000-4000-8000-000000000106"
	refGone      = "c0de0000-0000-4000-8000-000000000999"
)

// formReferencesProject is the composite project with a form of the catalog
// Номенклатура and attributes on the common form, each naming objects of the
// project in its type, its functional options and its rights.
func formReferencesProject(t *testing.T) string {
	t.Helper()
	root := compositeProject(t)
	writeFile(t, filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile),
		"format: 1\nid: "+refForm+"\nname: ФормаЭлемента\ntitle: {ru: Форма элемента}\nkind: object\nattributes:\n"+
			"  - id: "+refAttribute+"\n    name: Объект\n    types: [{kind: catalog-object, reference: "+cmpGoods+"}]\n    main: true\n"+
			"    functional_options: ["+cmpOption+"]\n"+
			"    view: {common: false, roles: [{role: "+cmpRole+", value: true}]}\n"+
			"    additional_columns:\n      - table: Объект.Остатки\n        columns:\n"+
			"          - {id: "+refExtra+", name: Склад, types: [{kind: catalog, reference: "+cmpWarehouses+"}], edit: {common: false, roles: [{role: "+cmpRole+", value: true}]}}\n"+
			"  - id: "+refColumn+"\n    name: Цены\n    types: [{kind: value-table}]\n    columns:\n"+
			"      - {id: c0de0000-0000-4000-8000-000000000107, name: Цена, types: [{kind: defined-type, reference: "+cmpDefinedType+"}], functional_options: ["+cmpOption+"]}\n"+
			"  - {id: "+refFilter+", name: Отбор, types: [{kind: platform, name: Отбор}]}\n")
	writeCommonForm(t, root, "АдреснаяКнига", "format: 1\nid: "+cmpCommonForm+"\nname: АдреснаяКнига\ntitle: {ru: АдреснаяКнига}\nkind: common\nattributes:\n"+
		"  - {id: "+refCommonAt+", name: Пользователь, types: [{kind: catalog, reference: "+cmpUsers+"}], functional_options: ["+cmpOption+"]}\n")
	return root
}

// The attributes of a form of an object and of a common form are read whole
// at load, and every reference resolves; a platform type by name in a form is
// noted like one on a report.
//
// Defect caught: the forms of objects read for their identity alone, so that
// nothing their attributes name is ever checked and a platform type in a form
// passes without the note the import report needs.
func TestFormAttributesAreResolvedAtLoad(t *testing.T) {
	t.Parallel()
	catalog, err := Load(formReferencesProject(t))
	if err != nil {
		t.Fatal(err)
	}
	notes := catalog.Notes()
	if len(notes) != 1 || notes[0].Kind != NotePlatformTypeByName || notes[0].Where != "catalog Номенклатура form ФормаЭлемента attribute Отбор" || notes[0].Written != "Отбор" {
		t.Fatalf("notes = %+v", notes)
	}
}

// Each reference of an attribute, a column and an added column of a form,
// broken alone, is refused or carried unresolved - naming the form, the
// attribute and the column.
//
// Defect caught: a reference of a form left unchecked, so that a form pointing
// at nothing loads clean; a check that looks at the attribute and not at its
// columns; a refusal or a note that does not say which form.
func TestEveryReferenceOfAFormAttributeIsChecked(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		file, target string
		refused      bool
		where        string
	}{
		"тип реквизита": {"catalogs/Номенклатура/forms/ФормаЭлемента/form.yaml", cmpGoods, true,
			"catalog Номенклатура form ФормаЭлемента attribute Объект"},
		"тип колонки": {"catalogs/Номенклатура/forms/ФормаЭлемента/form.yaml", cmpDefinedType, true,
			"catalog Номенклатура form ФормаЭлемента attribute Цены column Цена"},
		"тип добавленной колонки": {"catalogs/Номенклатура/forms/ФормаЭлемента/form.yaml", cmpWarehouses, true,
			"catalog Номенклатура form ФормаЭлемента attribute Объект table Объект.Остатки column Склад"},
		"тип реквизита общей формы": {"common-forms/АдреснаяКнига/form.yaml", cmpUsers, true,
			"common form АдреснаяКнига attribute Пользователь"},
		"функциональная опция": {"catalogs/Номенклатура/forms/ФормаЭлемента/form.yaml", cmpOption, false,
			"catalog Номенклатура form ФормаЭлемента attribute Объект functional option"},
		"опция общей формы": {"common-forms/АдреснаяКнига/form.yaml", cmpOption, false,
			"common form АдреснаяКнига attribute Пользователь functional option"},
		"роль": {"catalogs/Номенклатура/forms/ФормаЭлемента/form.yaml", cmpRole, false,
			"catalog Номенклатура form ФормаЭлемента attribute Объект right of role"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", filepath.FromSlash(test.file))
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, strings.ReplaceAll(string(content), test.target, refGone))
			catalog, err := Load(root)
			if test.refused {
				if err == nil || !strings.HasPrefix(err.Error(), test.where+" ") || !strings.Contains(err.Error(), refGone) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var found []Note
			for _, note := range catalog.Notes() {
				if note.Kind == NoteUnresolvedReference {
					found = append(found, note)
				}
			}
			// Every mention of the target was broken; each place is noted.
			if len(found) == 0 {
				t.Fatalf("nothing noted: %+v", catalog.Notes())
			}
			for _, note := range found {
				if note.Written != refGone {
					t.Fatalf("notes = %+v", found)
				}
			}
			if !containsWhere(found, test.where) {
				t.Fatalf("no note at %q: %+v", test.where, found)
			}
		})
	}
}

func containsWhere(notes []Note, where string) bool {
	for _, note := range notes {
		if note.Where == where {
			return true
		}
	}
	return false
}

// A form of an object that does not read is a refusal naming the form, and
// of two such forms the one named first in order is the one reported, however
// the forms happened to be read.
//
// Defect caught: a broken form of an object loading silently, as it did when
// only its identity was read; and a report that names a different form from
// one load to the next because the forms are read side by side.
func TestABrokenFormOfAnObjectRefusesTheProject(t *testing.T) {
	t.Parallel()
	for range 10 {
		root := formReferencesProject(t)
		for _, form := range []string{"ФормаА", "ФормаЯ"} {
			writeFile(t, filepath.Join(root, "metadata", string(CatalogKind), "Склады", "forms", form, project.FormMetadataFile),
				"format: 1\nid: c0de0000-0000-4000-8000-00000000011"+map[string]string{"ФормаА": "1", "ФормаЯ": "2"}[form]+
					"\nname: "+form+"\ntitle: {ru: Ф}\nkind: object\nattributes:\n  - {id: c0de0000-0000-4000-8000-00000000012"+
					map[string]string{"ФормаА": "1", "ФормаЯ": "2"}[form]+", name: А, main: true}\n  - {id: c0de0000-0000-4000-8000-00000000013"+
					map[string]string{"ФормаА": "1", "ФормаЯ": "2"}[form]+", name: Б, main: true}\n")
		}
		_, err := Load(root)
		if err == nil || !strings.HasPrefix(err.Error(), "catalog Склады form ФормаА: ") || !strings.Contains(err.Error(), "is the main attribute already") {
			t.Fatalf("err = %v", err)
		}
	}
}
