package metadata

import (
	"strings"
	"testing"
)

// extensionNamesForm is a common form whose main attribute is main, with
// properties of its extension, a table with its context menu and search
// string, a usual group, an input field and a string attribute.
func extensionNamesForm(main, properties string) string {
	return "format: 1\nid: c0de0000-0000-4000-8000-000000990201\nname: Имена\ntitle: {ru: Имена}\nkind: common\n" + properties +
		"items:\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990202, name: ГруппаНастроек, kind: usual-group}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990203, name: Поле, kind: input-field, data_path: Вариант}\n" +
		"  - id: c0de0000-0000-4000-8000-000000990204\n    name: Дерево\n    kind: table\n" +
		"    context_menu: {id: c0de0000-0000-4000-8000-000000990205, name: ДеревоКонтекстноеМеню}\n" +
		"    search_string_addition: {id: c0de0000-0000-4000-8000-000000990206, name: ДеревоСтрокаПоиска, kind: search-string-addition}\n" +
		"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990207, name: Объект, main: true, types: [" + main + "]}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990208, name: Вариант, types: [{kind: string}]}\n"
}

// The properties of a form of a report and of a hierarchical list that name
// a part of the form are held to it: the attribute the name of the variant
// is shown in (23 of 23 names of the exports), the group of the user's
// settings (98 of 98 usual groups) and the table of the tree of groups (15,
// and 9 times its context menu or search string). A name of nothing is
// refused; an element of another kind and a number are carried and noted.
//
// Defect caught: a name of nothing loading clean, so the report shows its
// variant nowhere and the list builds no tree; the context menu the
// prototype saves for the list of groups refused, so that 8 forms of erp,
// acc and lombard1 are not moved; an element of the wrong kind or a number
// carried silently; the folder of the settings of a settings composer
// refused, though the help gives that form its user's settings too.
func TestTheNamesOfAFormOfAReportOrAListAreResolved(t *testing.T) {
	t.Parallel()
	report := "{kind: report-object, reference: " + cmpReport + "}"
	list := "{kind: dynamic-list}"
	composer := "{kind: settings-composer}"
	load := func(t *testing.T, form string) (*Catalog, error) {
		root := formReferencesProject(t)
		writeCommonForm(t, root, "Имена", form)
		return Load(root)
	}
	for name, test := range map[string]struct{ main, properties string }{
		"отчёт по именам":             {report, "variant_appearance: вариант\ncustom_settings_folder: группаНастроек\n"},
		"список групп — таблица":      {list, "group_list: Дерево\n"},
		"папка настроек компоновщика": {composer, "custom_settings_folder: ГруппаНастроек\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			catalog, err := load(t, extensionNamesForm(test.main, test.properties))
			if err != nil {
				t.Fatal(err)
			}
			for _, note := range catalog.Notes() {
				if strings.HasPrefix(note.Where, "common form Имена ") {
					t.Fatalf("a name of the right kind is noted: %+v", note)
				}
			}
		})
	}
	for name, test := range map[string]struct{ main, properties, issue string }{
		"вариант — элемент, не реквизит": {report, "variant_appearance: Поле\n", "variant_appearance names no attribute of the form"},
		"папка — ничто":                  {report, "custom_settings_folder: Подвал\n", "custom_settings_folder names no element of the form"},
		"папка — реквизит":               {report, "custom_settings_folder: Вариант\n", "custom_settings_folder names no element of the form"},
		"список групп — ничто":           {list, "group_list: Список\n", "group_list names no element of the form"},
		"список групп — реквизит":        {list, "group_list: Вариант\n", "group_list names no element of the form"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := load(t, extensionNamesForm(test.main, test.properties)); err == nil || !strings.Contains(err.Error(), test.issue) {
				t.Fatalf("Load() error = %v, want %q", err, test.issue)
			}
		})
	}
	for name, test := range map[string]struct {
		main, properties string
		kind             NoteKind
		where, written   string
	}{
		"список групп — контекстное меню": {list, "group_list: ДеревоКонтекстноеМеню\n", NoteGroupListNotTable, "group_list", "ДеревоКонтекстноеМеню (context-menu)"},
		"список групп — строка поиска":    {list, "group_list: деревоСтрокаПоиска\n", NoteGroupListNotTable, "group_list", "деревоСтрокаПоиска (search-string-addition)"},
		"список групп — группа":           {list, "group_list: ГруппаНастроек\n", NoteGroupListNotTable, "group_list", "ГруппаНастроек (usual-group)"},
		"папка — поле":                    {report, "custom_settings_folder: Поле\n", NoteSettingsFolderNotGroup, "custom_settings_folder", "Поле (input-field)"},
		"папка — таблица":                 {composer, "custom_settings_folder: Дерево\n", NoteSettingsFolderNotGroup, "custom_settings_folder", "Дерево (table)"},
		"вариант числом":                  {report, "variant_appearance: \"2\"\n", NoteFormReferenceAsWritten, "variant_appearance", "2"},
		"результат числом":                {report, "report_result: \"0\"\n", NoteFormReferenceAsWritten, "report_result", "0"},
		"расшифровка числом":              {report, "details_data: \"4\"\n", NoteFormReferenceAsWritten, "details_data", "4"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			catalog, err := load(t, extensionNamesForm(test.main, test.properties))
			if err != nil {
				t.Fatal(err)
			}
			var found []Note
			for _, note := range catalog.Notes() {
				if strings.HasPrefix(note.Where, "common form Имена ") {
					found = append(found, note)
				}
			}
			if len(found) != 1 || found[0].Kind != test.kind || found[0].Where != "common form Имена "+test.where || found[0].Written != test.written {
				t.Fatalf("notes = %+v, want %s %q at %s", found, test.kind, test.written, test.where)
			}
		})
	}
}
