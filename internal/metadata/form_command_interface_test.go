package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
)

// The command interface of a form keeps both its panels line by line through
// YAML and the Studio: the command, whether the form added it, its group and
// place, whom it is shown to, where its parameter comes from; and a line
// repeated whole is kept twice.
//
// Defect caught: a panel or a line lost, so that a command hidden on the form
// shows again (the visibility of 11 827 lines), or one the form added (651) is
// gone; the group, the place or the source of the parameter of a line lost;
// lines folded into one by a key, so that the prototype's repeated lines
// (59 named, about 600 with codes) come out fewer; the order of the lines
// changed, which is the order the platform shows them in.
func TestAFormCommandInterfaceKeepsWhatItHas(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	repeated := "    - {command: CommonCommand.Печать, added: true, group: CommandGroup.Печать, data_path: Объект.Ref}\n"
	source := formAttrHead + "commands:\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990001, name: Обновить, title: {ru: Обновить}, action: custom, handler: Обновить}\n" +
		"items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Список, kind: table}\n" +
		"command_interface:\n  navigation_panel:\n" +
		"    - {command: InformationRegister.Штрихкоды.StandardCommand.OpenByValue.Номенклатура, group: form-navigation-panel-go-to, index: 2," +
		" visibility: {common: false, roles: [{role: c0de0000-0000-4000-8000-000000990003, value: true}]}}\n" +
		"    - {command: \"0:83b807b2-85be-45aa-89fe-364ee411b03a\", visibility: {common: true}}\n" +
		"  command_bar:\n" + repeated + repeated +
		"    - {command: Form.Command.Обновить, group: form-command-bar-important, index: 1}\n" +
		"    - {command: Form.Item.Список.StandardCommand.Add, group: c0de0000-0000-4000-8000-000000990004, data_path: Items.Список.CurrentData.Ref}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, value *FormCommandInterface) {
		t.Helper()
		if value == nil || len(value.NavigationPanel) != 2 || len(value.CommandBar) != 4 {
			t.Fatalf("%s: panels: %+v", source, value)
		}
		first, code := value.NavigationPanel[0], value.NavigationPanel[1]
		switch {
		case first.Command != "InformationRegister.Штрихкоды.StandardCommand.OpenByValue.Номенклатура" || first.Group != FormNavigationPanelGoTo ||
			first.Index != 2 || first.Added:
			t.Fatalf("%s: a line of the navigation panel: %+v", source, first)
		case first.Visibility == nil || first.Visibility.Common || len(first.Visibility.Roles) != 1 || !first.Visibility.Roles[0].Value:
			t.Fatalf("%s: whom it is shown to: %+v", source, first.Visibility)
		case code.Command != "0:83b807b2-85be-45aa-89fe-364ee411b03a" || code.Visibility == nil || !code.Visibility.Common:
			t.Fatalf("%s: a line written as a code: %+v", source, code)
		}
		bar := value.CommandBar
		switch {
		case !bar[0].Added || bar[0].Group != "CommandGroup.Печать" || bar[0].DataPath != "Объект.Ref" || !reflect.DeepEqual(bar[0], bar[1]):
			t.Fatalf("%s: a line repeated: %+v", source, bar[:2])
		case bar[2].Command != "Form.Command.Обновить" || bar[2].Group != FormCommandBarImportant || bar[2].Index != 1:
			t.Fatalf("%s: the order of the lines: %+v", source, bar[2])
		case bar[3].Group != "c0de0000-0000-4000-8000-000000990004" || bar[3].DataPath != "Items.Список.CurrentData.Ref":
			t.Fatalf("%s: a group by identifier: %+v", source, bar[3])
		}
	}
	check("read", form.CommandInterface)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.CommandInterface)
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatal(err)
	}
	check("carried through the Studio", received.CommandInterface)
	if !reflect.DeepEqual(received.CommandInterface, form.CommandInterface) || !reflect.DeepEqual(again.CommandInterface, form.CommandInterface) {
		t.Fatal("the command interface changed on the way")
	}
}

// What is wrong in the command interface of a form is refused, naming the
// panel and the line.
//
// Defect caught: a group of the other panel taken (the important group of the
// command bar in the navigation panel), or the prototype's spelling of a
// group (FormCommandBarImportant) taken as written; a command written no way
// a command is, or naming a command or an element the form does not have; a
// register opened by value with no dimension; a place before the first; a
// role with no identifier; a data path that is none.
func TestAFormCommandInterfaceRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form := func(panel, line string) string {
		return formAttrHead + "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Список, kind: table}\n" +
			"command_interface:\n  " + panel + ":\n    - {" + line + "}\n"
	}
	for name, test := range map[string]struct{ source, want string }{
		"группа другой панели": {form("navigation_panel", "command: CommonCommand.Печать, group: form-command-bar-important"),
			"command_interface.navigation_panel[0].group must be one of form-navigation-panel-important, form-navigation-panel-go-to, form-navigation-panel-see-also"},
		"группа прототипа": {form("command_bar", "command: CommonCommand.Печать, group: FormCommandBarImportant"),
			"command_interface.command_bar[0].group must be a group of the panel, CommandGroup.<name> or the identifier of a command group"},
		"группа без имени": {form("command_bar", "command: CommonCommand.Печать, group: CommandGroup.Группа печати"),
			"command_interface.command_bar[0].group must be a group of the panel"},
		"не команда": {form("command_bar", "command: Печать"), "command_interface.command_bar[0].command must be Form.Command.<name>"},
		"по значению через два": {form("navigation_panel", "command: InformationRegister.Штрихкоды.StandardCommand.OpenByValue.Номенклатура.Ссылка"),
			"command_interface.navigation_panel[0].command must be Form.Command.<name>"},
		"по значению не регистра": {form("navigation_panel", "command: Catalog.Товары.StandardCommand.OpenByValue.Владелец"),
			"command_interface.navigation_panel[0].command must be Form.Command.<name>"},
		"команды формы нет":    {form("command_bar", "command: Form.Command.Обновить"), "command_interface.command_bar[0].command names no command of the form"},
		"элемента формы нет":   {form("command_bar", "command: Form.Item.Товары.StandardCommand.Add"), "command_interface.command_bar[0].command names no element of the form"},
		"место до первого":     {form("command_bar", "command: CommonCommand.Печать, index: -1"), "command_interface.command_bar[0].index must be a place in the group, from 1"},
		"роль без ссылки":      {form("command_bar", "command: CommonCommand.Печать, visibility: {common: true, roles: [{role: 00000000-0000-0000-0000-000000000000, value: false}]}"), "command_interface.command_bar[0].visibility.roles[0].role must be a non-zero UUID"},
		"путь к данным":        {form("command_bar", "command: CommonCommand.Печать, data_path: Объект..Ref"), "command_interface.command_bar[0].data_path must be names separated by dots"},
		"путь с пробелом":      {form("command_bar", "command: CommonCommand.Печать, data_path: ' Объект.Ref'"), "command_interface.command_bar[0].data_path must be a data path without surrounding spaces"},
		"элемент другой формы": {form("navigation_panel", "command: Form.Item.Список.StandardCommand.Add, group: form-navigation-panel-important, index: 0"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(test.source), configuration)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// What the command interface of a form names in the project is resolved at
// load: a command, a register and its master dimension, a command group and
// its panel, a role. What the prototype wrote as a code, and a line repeated
// whole, is noted.
//
// Defect caught: a common command, a register, a dimension or a command group
// the project no longer has loading clean; a register opened by a dimension
// that is not its master one taken as resolved; a group of the actions panel
// standing in the command bar of a form accepted; a hidden command shown to
// a role that is gone loading clean; the 8600 commands written as codes and
// the lines repeated whole carried with no note, or a note on a line that is
// not repeated, only like another.
func TestWhatAFormCommandInterfaceNamesIsResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		panel, lines string
		gone         string
		refused      string
		notes        []NoteKind
	}{
		"всё на месте": {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, group: CommandGroup.Печать, visibility: {common: false, roles: [{role: " + cmpRole + ", value: true}]}}\n" +
			"    - {command: CommonCommand.ОткрытьОстатки, group: CommandGroup.Печать, index: 1}\n" +
			"    - {command: InformationRegister.Штрихкоды.StandardCommand.OpenByValue.Номенклатура}\n" +
			"    - {command: Catalog.Номенклатура.StandardCommand.Create, data_path: Объект.Ref}\n"},
		"общей команды нет":        {panel: "command_bar", lines: "    - {command: CommonCommand.Нет}\n", gone: "command_bar[0] command"},
		"регистра нет":             {panel: "navigation_panel", lines: "    - {command: InformationRegister.Нет.StandardCommand.OpenByValue.Номенклатура}\n", gone: "navigation_panel[0] command"},
		"измерения нет":            {panel: "navigation_panel", lines: "    - {command: InformationRegister.Штрихкоды.StandardCommand.OpenByValue.Склад}\n", gone: "navigation_panel[0] command"},
		"измерение не ведущее":     {panel: "navigation_panel", lines: "    - {command: InformationRegister.Штрихкоды.StandardCommand.OpenByValue.Упаковка}\n", gone: "navigation_panel[0] command"},
		"группы нет":               {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, group: CommandGroup.Нет}\n", gone: "command_bar[0] group"},
		"группа по идентификатору": {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, group: " + refGone + "}\n", gone: "command_bar[0] group"},
		"роли нет": {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, visibility: {common: true, roles: [{role: " + refGone + ", value: false}]}}\n",
			gone: "command_bar[0] right of role"},
		"группа другой панели": {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, group: CommandGroup.Сервис}\n",
			refused: "command_bar[0] group CommandGroup.Сервис stands in the actions-panel, not in the form-command-bar"},
		"группа панели навигации": {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, group: CommandGroup.Переходы}\n",
			refused: "stands in the form-navigation-panel, not in the form-command-bar"},
		"код": {panel: "navigation_panel", lines: "    - {command: \"0\"}\n    - {command: \"0\"}\n    - {command: \"3:83b807b2-85be-45aa-89fe-364ee411b03a\", data_path: \"~Объект.Ref~Объект.Склад\"}\n",
			notes: []NoteKind{NoteFormReferenceAsWritten, NoteFormReferenceAsWritten, NoteFormReferenceAsWritten, NoteFormReferenceAsWritten}},
		"повтор целиком": {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, added: true}\n    - {command: CommonCommand.ОткрытьОстатки, added: true}\n",
			notes: []NoteKind{NoteRepeatedInterfaceCommand}},
		"повтор с другой видимостью": {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки, visibility: {common: false}}\n    - {command: CommonCommand.ОткрытьОстатки, visibility: {common: true}}\n"},
		"повтор в другой панели":     {panel: "command_bar", lines: "    - {command: CommonCommand.ОткрытьОстатки}\n  navigation_panel:\n    - {command: CommonCommand.ОткрытьОстатки}\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			writeMetadata(t, root, InformationRegisterKind, "c0de0000-0000-4000-8000-000000990010", "format: 1\nid: c0de0000-0000-4000-8000-000000990010\n"+
				"name: Штрихкоды\ntitle: {ru: Штрихкоды}\nwrite_mode: independent\nperiodicity: none\ndimensions:\n"+
				"  - {id: c0de0000-0000-4000-8000-000000990011, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: "+cmpGoods+"}], master: true}\n"+
				"  - {id: c0de0000-0000-4000-8000-000000990012, name: Упаковка, title: {ru: Упаковка}, types: [{kind: catalog, reference: "+cmpGoods+"}]}\n")
			writeMetadata(t, root, CommandGroupKind, "c0de0000-0000-4000-8000-000000990013", "format: 1\nid: c0de0000-0000-4000-8000-000000990013\n"+
				"name: Печать\ntitle: {ru: Печать}\ncategory: form-command-bar\n")
			writeMetadata(t, root, CommandGroupKind, "c0de0000-0000-4000-8000-000000990014", "format: 1\nid: c0de0000-0000-4000-8000-000000990014\n"+
				"name: Переходы\ntitle: {ru: Переходы}\ncategory: form-navigation-panel\n")
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(content)+"command_interface:\n  "+test.panel+":\n"+test.lines)
			where := "catalog Номенклатура form ФормаЭлемента command interface "
			if test.gone != "" {
				if found := unresolvedOf(t, root); !containsWhere(found, where+test.gone) {
					t.Fatalf("unresolved = %+v", found)
				}
				return
			}
			catalog, err := Load(root)
			if test.refused != "" {
				if err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("err = %v, want %q", err, test.refused)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if unresolved := catalog.UnresolvedReferences(); len(unresolved) != 0 {
				t.Fatalf("unresolved = %+v", unresolved)
			}
			var found []NoteKind
			for _, note := range catalog.Notes() {
				if strings.HasPrefix(note.Where, where) {
					found = append(found, note.Kind)
				}
			}
			if !reflect.DeepEqual(found, test.notes) {
				t.Fatalf("notes = %v, want %v", found, test.notes)
			}
		})
	}
}
