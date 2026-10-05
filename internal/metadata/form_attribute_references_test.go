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
			"  - {id: "+refFilter+", name: Отбор, types: [{kind: platform, name: Отбор}]}\n"+
			"  - {id: c0de0000-0000-4000-8000-000000000108, name: Список, types: [{kind: dynamic-list}], dynamic_list: {main_table: {object: "+cmpBalances+", virtual: Balance}}}\n"+
			"  - {id: c0de0000-0000-4000-8000-000000000109, name: Склады, types: [{kind: value-list}], value_type: [{kind: catalog, reference: "+cmpUsers+"}]}\n")
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
		"тип элементов списка значений": {"catalogs/Номенклатура/forms/ФормаЭлемента/form.yaml", cmpUsers, true,
			"catalog Номенклатура form ФормаЭлемента attribute Склады value type"},
		"основная таблица динамического списка": {"catalogs/Номенклатура/forms/ФормаЭлемента/form.yaml", cmpBalances, false,
			"catalog Номенклатура form ФормаЭлемента attribute Список main table"},
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
			if test.refused {
				_, err := Load(root)
				if err == nil || !strings.HasPrefix(err.Error(), test.where+" ") || !strings.Contains(err.Error(), refGone) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			// Every mention of the target was broken; each place is named.
			found := unresolvedOf(t, root)
			for _, item := range found {
				if item.ID.String() != refGone {
					t.Fatalf("unresolved = %+v", found)
				}
			}
			if !containsWhere(found, test.where) {
				t.Fatalf("nothing at %q: %+v", test.where, found)
			}
		})
	}
}

func containsWhere(unresolved []UnresolvedReference, where string) bool {
	for _, item := range unresolved {
		if item.Where == where {
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

// The titles of the attributes of a form, of their columns and of the
// columns added to a table are noted under the same rules as the texts of
// objects: a language the configuration does not declare, no language, a
// text of spaces. The titles of a sound form carry none.
//
// Defect caught: the forms left out of the walk of texts - they are not part
// of the catalog - so that a title in a language nobody declared passes
// without the note the import report needs; and a title of a column, an
// added column or a common form missed while the attribute's own is seen.
func TestTheTitlesOfFormAttributesAreNotedLikeTextsOfObjects(t *testing.T) {
	t.Parallel()
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	form := string(content)
	for _, change := range []struct{ old, new string }{
		{"    name: Объект\n", "    name: Объект\n    title: {ru: Объект, de: Objekt}\n"},
		{"name: Склад, types:", "name: Склад, title: {\"\": Склад}, types:"},
		{"name: Цена, types:", "name: Цена, title: {ru: \"  \"}, types:"},
	} {
		if !strings.Contains(form, change.old) {
			t.Fatalf("the form has no %q", change.old)
		}
		form = strings.Replace(form, change.old, change.new, 1)
	}
	writeFile(t, path, form)
	writeCommonForm(t, root, "АдреснаяКнига", "format: 1\nid: "+cmpCommonForm+"\nname: АдреснаяКнига\ntitle: {ru: АдреснаяКнига}\nkind: common\nattributes:\n"+
		"  - {id: "+refCommonAt+", name: Пользователь, title: {de: Benutzer}, types: [{kind: catalog, reference: "+cmpUsers+"}]}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, note := range catalog.Notes() {
		got[string(note.Kind)+" "+note.Where] = note.Written
	}
	for place, written := range map[string]string{
		"text-in-undeclared-language catalog Номенклатура form ФормаЭлемента attribute Объект title.de":                          "Objekt",
		"text-without-language catalog Номенклатура form ФормаЭлемента attribute Объект table Объект.Остатки column Склад title": "Склад",
		"text-of-spaces catalog Номенклатура form ФормаЭлемента attribute Цены column Цена title.ru":                             `"  "`,
		"text-in-undeclared-language common form АдреснаяКнига attribute Пользователь title.de":                                  "Benutzer",
	} {
		if got[place] != written {
			t.Errorf("no note %q = %q; notes %v", place, written, got)
		}
	}
	// The platform type of the attribute Отбор, and the four above.
	if len(got) != 5 {
		t.Errorf("notes = %v", got)
	}
}
