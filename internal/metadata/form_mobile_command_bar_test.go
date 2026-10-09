package metadata

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// mobileCommandBarForm is a common form whose command bar on a mobile device
// holds bar: the bar of the form, a button of it, a table with its own bar
// and context menu, a usual group, a decoration with its extended tooltip
// and a command of the form named like no element.
func mobileCommandBarForm(bar string) string {
	return "format: 1\nid: " + commonFormID + "\nname: Мобильная\ntitle: {ru: Мобильная}\nkind: common\n" +
		"commands:\n  - {id: c0de0000-0000-4000-8000-000000990101, name: Обновить, title: {ru: Обновить}, action: custom}\n" +
		"auto_command_bar:\n  id: c0de0000-0000-4000-8000-000000990102\n  name: ФормаКоманднаяПанель\n" +
		"  children:\n    - {id: c0de0000-0000-4000-8000-000000990103, name: Записать, kind: button, command: Form.StandardCommand.Write}\n" +
		"items:\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990104, name: Шапка, kind: usual-group}\n" +
		"  - id: c0de0000-0000-4000-8000-000000990105\n    name: Список\n    kind: table\n" +
		"    auto_command_bar: {id: c0de0000-0000-4000-8000-000000990106, name: СписокКоманднаяПанель}\n" +
		"    context_menu: {id: c0de0000-0000-4000-8000-000000990107, name: СписокКонтекстноеМеню}\n" +
		"  - id: c0de0000-0000-4000-8000-000000990108\n    name: Надпись\n    kind: label-decoration\n" +
		"    extended_tooltip: {id: c0de0000-0000-4000-8000-000000990109, name: НадписьРасширеннаяПодсказка, kind: label-decoration}\n" +
		"mobile_command_bar: " + bar + "\n"
}

// The command bar of a form on a mobile device names the groups and buttons
// it holds, in order. Dropped from the model, it vanished at import (283
// forms of the exports); a name of no element would show nothing on the
// device; a value the prototype wrote empty, an element that is no group or
// button and the code of an element are carried as written and reported.
func TestTheMobileCommandBarOfAFormIsKeptAndNamesItsElements(t *testing.T) {
	t.Parallel()
	const kept = "[ФормаКоманднаяПанель, записать, СписокКоманднаяПанель, Шапка]"
	t.Run("состав сохраняется по порядку", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeCommonForm(t, root, "Мобильная", mobileCommandBarForm(kept))
		catalog, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		forms, err := ReadCommonForms(root, metadataConfiguration())
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"ФормаКоманднаяПанель", "записать", "СписокКоманднаяПанель", "Шапка"}
		if len(forms) != 1 || !slices.Equal(forms[0].MobileCommandBar, want) {
			t.Fatalf("mobile command bar = %+v, want %v", forms, want)
		}
		var written bytes.Buffer
		if err := yaml.NewEncoder(&written).Encode(forms[0]); err != nil {
			t.Fatal(err)
		}
		var again ManagedForm
		if err := yaml.Unmarshal(written.Bytes(), &again); err != nil || !slices.Equal(again.MobileCommandBar, want) {
			t.Fatalf("written and read again = %v (%v), want %v", again.MobileCommandBar, err, want)
		}
		clone := cloneRuntimeForm(forms[0])
		clone.MobileCommandBar[0] = "Шапка"
		if forms[0].MobileCommandBar[0] != "ФормаКоманднаяПанель" {
			t.Fatal("a runtime form shares the mobile command bar with the snapshot")
		}
		for _, note := range catalog.Notes() {
			if strings.Contains(note.Where, "mobile_command_bar") {
				t.Fatalf("a group or a button by name is noted: %+v", note)
			}
		}
	})
	for name, test := range map[string]struct{ bar, issue string }{
		"имя никакого элемента":      {"[Шапка, Подвал]", "mobile_command_bar[1] names no element of the form"},
		"имя команды, не элемента":   {"[Обновить]", "mobile_command_bar[0] names no element of the form"},
		"пробелы вокруг имени":       {"[\" Шапка\"]", "mobile_command_bar[0] must be written without surrounding spaces"},
		"код не по форме кода":       {"[\"5:0202\"]", "mobile_command_bar[0] names no element of the form"},
		"имя реквизита, не элемента": {"[Объект]", "mobile_command_bar[0] names no element of the form"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeCommonForm(t, root, "Мобильная", mobileCommandBarForm(test.bar)+
				"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990110, name: Объект, types: [{kind: string}]}\n")
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.issue) {
				t.Fatalf("Load() error = %v, want %q", err, test.issue)
			}
		})
	}
	for name, test := range map[string]struct {
		bar     string
		kind    NoteKind
		written string
	}{
		"пустой пункт":             {"[Шапка, \"\"]", NoteMobileCommandBarEmpty, `""`},
		"расширенная подсказка":    {"[Шапка, НадписьРасширеннаяПодсказка]", NoteMobileCommandBarNotGroup, "НадписьРасширеннаяПодсказка (label-decoration)"},
		"декорация":                {"[Шапка, надпись]", NoteMobileCommandBarNotGroup, "надпись (label-decoration)"},
		"таблица":                  {"[Шапка, Список]", NoteMobileCommandBarNotGroup, "Список (table)"},
		"контекстное меню таблицы": {"[Шапка, СписокКонтекстноеМеню]", NoteMobileCommandBarNotGroup, "СписокКонтекстноеМеню (context-menu)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeCommonForm(t, root, "Мобильная", mobileCommandBarForm(test.bar))
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			var found []Note
			for _, note := range catalog.Notes() {
				if strings.Contains(note.Where, "mobile_command_bar") {
					found = append(found, note)
				}
			}
			if len(found) != 1 || found[0].Kind != test.kind || found[0].Where != "common form Мобильная mobile_command_bar[1]" || found[0].Written != test.written {
				t.Fatalf("notes = %+v, want %s %q at mobile_command_bar[1]", found, test.kind, test.written)
			}
		})
	}
}
