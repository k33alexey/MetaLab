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

// The command bar of the form and of a table, the context menu of a field,
// a decoration, a table and an addition, and a command bar standing in the
// form keep what they have and what they hold through YAML and the Studio.
//
// Defect caught: the context menus of the exports (155317, 1916 of them
// holding buttons) and the command bars (18740) not moved at all, so that
// every button they hold is lost; the command bar of a form with no name (47
// forms) refused; the off switch of filling, the alignment of a bar or its
// importance on a narrow screen lost; the context menu of an addition
// standing in the command bar of a table lost.
func TestAMenuAndACommandBarKeepWhatTheyHold(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	source := strings.Replace(formElementsForm(
		"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: {id: c0de0000-0000-4000-8000-000000990002,"+
			" name: ПолеКонтекстноеМеню, no_autofill: true, children: [{id: c0de0000-0000-4000-8000-000000990003, name: Подобрать, kind: button,"+
			" command: Form.StandardCommand.Close}, {id: c0de0000-0000-4000-8000-000000990004, name: Ещё, kind: popup},"+
			" {id: c0de0000-0000-4000-8000-000000990005, name: Группа, kind: button-group}]}}\n"+
			"  - {id: c0de0000-0000-4000-8000-000000990006, name: Надпись, kind: label-decoration, context_menu: {id: c0de0000-0000-4000-8000-000000990007, name: ДекорацияКонтекстноеМеню}}\n"+
			"  - {id: c0de0000-0000-4000-8000-000000990008, name: Список, kind: table,"+
			" context_menu: {id: c0de0000-0000-4000-8000-000000990009, name: СписокContextMenu},"+
			" auto_command_bar: {id: c0de0000-0000-4000-8000-000000990010, name: Список_КоманднаяПанель, no_autofill: true, horizontal_align: right,"+
			" children: [{id: c0de0000-0000-4000-8000-000000990011, name: Поиск, kind: search-string-addition,"+
			" context_menu: {id: c0de0000-0000-4000-8000-000000990012, name: ПоискКонтекстноеМеню}}]}}\n"+
			"  - {id: c0de0000-0000-4000-8000-000000990013, name: Панель, kind: command-bar, horizontal_align: auto, width: 19, read_only: true,"+
			" enable_content_change: true, command_source: form}\n"),
		"items:\n", "auto_command_bar: {id: c0de0000-0000-4000-8000-000000990014, display_importance: very-low, horizontal_align: center,"+
			" children: [{id: c0de0000-0000-4000-8000-000000990015, name: Закрыть, kind: button, command: Form.StandardCommand.Close}]}\nitems:\n", 1)
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	field, table, bar := form.Items[0], form.Items[2], form.Items[3]
	switch {
	case form.AutoCommandBar.Name != "" || form.AutoCommandBar.DisplayImportance != FormDisplayImportanceVeryLow ||
		form.AutoCommandBar.HorizontalAlign != ItemHorizontalCenter || form.AutoCommandBar.Children[0].Name != "Закрыть":
		t.Fatalf("command bar of the form: %+v", form.AutoCommandBar)
	case field.ContextMenu.Name != "ПолеКонтекстноеМеню" || !field.ContextMenu.NoAutofill || len(field.ContextMenu.Children) != 3:
		t.Fatalf("context menu of a field: %+v", field.ContextMenu)
	case form.Items[1].ContextMenu.Name != "ДекорацияКонтекстноеМеню":
		t.Fatalf("context menu of a decoration: %+v", form.Items[1].ContextMenu)
	case table.ContextMenu.Name != "СписокContextMenu" || table.AutoCommandBar.Name != "Список_КоманднаяПанель" || !table.AutoCommandBar.NoAutofill ||
		table.AutoCommandBar.HorizontalAlign != ItemHorizontalRight || table.AutoCommandBar.Children[0].ContextMenu.Name != "ПоискКонтекстноеМеню":
		t.Fatalf("table: %+v, %+v", table.ContextMenu, table.AutoCommandBar)
	case bar.HorizontalAlign != ItemHorizontalAuto || bar.Width != 19 || !bar.ReadOnly || !bar.EnableContentChange || bar.CommandSource != FormCommandSourceForm:
		t.Fatalf("command bar: %+v", bar)
	}
	if nested := field.Nested(); len(nested) != 3 || nested[0].Name != "Подобрать" {
		t.Fatalf("what a field holds: %+v", nested)
	}
	if items := form.FormItems(); len(items) != 5 || items[4].Name != "Закрыть" {
		t.Fatalf("what the form holds: %d", len(items))
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil || !reflect.DeepEqual(again.Items, form.Items) || !reflect.DeepEqual(again.AutoCommandBar, form.AutoCommandBar) {
		t.Fatalf("written back: %v", err)
	}
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil || !reflect.DeepEqual(received.Items, form.Items) ||
		!reflect.DeepEqual(received.AutoCommandBar, form.AutoCommandBar) {
		t.Fatalf("carried through the Studio: %v", err)
	}
}

// A context menu and a command bar stand where the prototype puts them, hold
// what it puts in them, and are named and numbered among the elements of the
// form.
//
// Defect caught: a context menu kept on a group or a button, a command bar
// on a field; a menu with no name anywhere but on the command bar of the
// form; a name or a number taken twice by a menu and an element; a field or
// a search addition in a context menu, a field in a command bar; the
// alignment of a command bar on a context menu; the prototype's "VeryLow";
// a button in a menu not checked as an element.
func TestAMenuAndACommandBarStandWhereThePrototypePutsThem(t *testing.T) {
	t.Parallel()
	menu := func(name, rest string) string {
		return "{id: c0de0000-0000-4000-8000-000000990002, name: " + name + rest + "}"
	}
	for name, test := range map[string]struct{ items, want string }{
		"меню у группы":            {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Группа, kind: usual-group, context_menu: " + menu("М", "") + "}\n", "items[0].context_menu is allowed only for fields, decorations, tables and additions of a table"},
		"меню у кнопки":            {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Кнопка, kind: button, context_menu: " + menu("М", "") + "}\n", "items[0].context_menu is allowed only for fields, decorations, tables and additions of a table"},
		"панель у поля":            {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, auto_command_bar: " + menu("П", "") + "}\n", "items[0].auto_command_bar is allowed only for tables"},
		"меню без имени":           {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: {id: c0de0000-0000-4000-8000-000000990002}}\n", "items[0].context_menu.name must be a valid identifier"},
		"панель таблицы без имени": {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, auto_command_bar: {id: c0de0000-0000-4000-8000-000000990002}}\n", "items[0].auto_command_bar.name must be a valid identifier"},
		"имя меню занято":          {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: " + menu("Поле", "") + "}\n", "name must be unique within the form"},
		"номер меню занят":         {"  - {id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, context_menu: " + menu("М", "") + "}\n", "items[0].context_menu.id must be unique"},
		"нулевой номер":            {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: {id: 00000000-0000-0000-0000-000000000000, name: М}}\n", "items[0].context_menu.id must be a non-zero UUID"},
		"поле в меню":              {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: " + menu("М", ", children: [{id: c0de0000-0000-4000-8000-000000990003, name: Другое, kind: input-field}]") + "}\n", "items[0].context_menu.children[0]: context menu cannot hold input-field"},
		"поиск в меню":             {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: " + menu("М", ", children: [{id: c0de0000-0000-4000-8000-000000990003, name: Поиск, kind: search-string-addition}]") + "}\n", "items[0].context_menu.children[0]: context menu cannot hold search-string-addition"},
		"поле в панели":            {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, auto_command_bar: " + menu("П", ", children: [{id: c0de0000-0000-4000-8000-000000990003, name: Другое, kind: input-field}]") + "}\n", "items[0].auto_command_bar.children[0]: auto command bar cannot hold input-field"},
		"выравнивание меню":        {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: " + menu("М", ", horizontal_align: right") + "}\n", "items[0].context_menu is a context menu: horizontal_align and display_importance are of a command bar"},
		"важность прототипа":       {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, auto_command_bar: " + menu("П", ", display_importance: VeryLow") + "}\n", "items[0].auto_command_bar.display_importance must be auto, very-low, low, usual, high or very-high"},
		"кнопка меню проверяется":  {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: " + menu("М", ", children: [{id: c0de0000-0000-4000-8000-000000990003, name: Кнопка, kind: button, command: Form.Command.Нет}]") + "}\n", "items[0].context_menu.children[0].command names no command of the form"},
	} {
		_, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(test.items)), managedFormConfiguration())
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v, want %q", name, err, test.want)
		}
	}
	form := strings.Replace(formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Закрыть, kind: input-field}\n"), "items:\n",
		"auto_command_bar: {id: c0de0000-0000-4000-8000-000000990002, children: [{id: c0de0000-0000-4000-8000-000000990003, name: закрыть, kind: button}]}\nitems:\n", 1)
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(form), managedFormConfiguration()); err == nil || !strings.Contains(err.Error(), "name must be unique within the form") {
		t.Errorf("a button of the command bar of the form taking the name of an element: %v", err)
	}
}

// What a button of a menu or of a command bar refers to is resolved in the
// project as for any other element: its common picture and its command.
//
// Defect caught: the buttons of menus and command bars left out of what the
// load resolves, so that a picture or a command removed from the
// configuration reads as sound there; a picture file of a button of the
// command bar of the form not looked for.
func TestTheButtonsOfMenusAreResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		place, button string
		files         []string
		gone, refused string
	}{
		"общая картинка в меню":      {place: "menu", button: "picture: {common: " + cmpCommonPicture + "}"},
		"удалённая картинка в меню":  {place: "menu", button: "picture: {common: " + refGone + "}", gone: "element Кнопка picture"},
		"нет общей команды в панели": {place: "bar", button: "command: CommonCommand.Нет", gone: "element Кнопка command"},
		"свой файл в панели формы":   {place: "bar", button: "picture: {file: Picture.png}", files: []string{"Кнопка/Picture.png"}},
		"нет своего файла в панели":  {place: "bar", button: "picture: {file: Picture.png}", refused: "element Кнопка picture is shown with picture file Picture.png, which its folder does not hold"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			button := "{id: c0de0000-0000-4000-8000-000000990003, name: Кнопка, kind: button, " + test.button + "}"
			added := "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, context_menu: {id: c0de0000-0000-4000-8000-000000990002, name: ПолеКонтекстноеМеню, children: [" + button + "]}}\n"
			if test.place == "bar" {
				added = "auto_command_bar: {id: c0de0000-0000-4000-8000-000000990002, name: ФормаКоманднаяПанель, children: [" + button + "]}\n"
			}
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", added+"attributes:\n", 1))
			for _, file := range test.files {
				writeFile(t, filepath.Join(filepath.Dir(path), project.FormItemsDirectory, file), "image")
			}
			switch {
			case test.gone != "":
				if found := unresolvedOf(t, root); !containsWhere(found, "catalog Номенклатура form ФормаЭлемента "+test.gone) {
					t.Fatalf("unresolved = %+v", found)
				}
			case test.refused != "":
				if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("err = %v, want %q", err, test.refused)
				}
			default:
				if _, err := Load(root); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// Additions of a table and context menus may share a name, as the prototype
// names them itself and saves the name repeated; every other element keeps a
// name of its own, and a shared name is noted.
//
// Defect caught: the three forms of the exports whose additions are all
// named "Addition" and their menus "ContextMenu" refused; a field, a button,
// a command bar or a field and an addition allowed to share a name, so that
// code and pictures reach the wrong element; a shared name carried silently.
func TestOnlyAdditionsAndContextMenusShareAName(t *testing.T) {
	t.Parallel()
	addition := func(id, kind, menu string) string {
		return "  - {id: c0de0000-0000-4000-8000-0000009900" + id + ", name: Addition, kind: " + kind +
			", context_menu: {id: c0de0000-0000-4000-8000-0000009901" + id + ", name: " + menu + "}}\n"
	}
	shared := addition("01", "search-string-addition", "ContextMenu") + addition("02", "view-status-addition", "contextmenu") +
		addition("03", "search-control-addition", "ContextMenu")
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(shared)), managedFormConfiguration()); err != nil {
		t.Fatalf("the additions of the exports: %v", err)
	}
	for name, items := range map[string]string{
		"два поля":          "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field}\n  - {id: c0de0000-0000-4000-8000-000000990002, name: поле, kind: label-field}\n",
		"поле и дополнение": "  - {id: c0de0000-0000-4000-8000-000000990001, name: Addition, kind: input-field}\n" + addition("02", "view-status-addition", "М"),
		"дополнение и поле": addition("01", "view-status-addition", "М") + "  - {id: c0de0000-0000-4000-8000-000000990002, name: Addition, kind: input-field}\n",
		"меню и поле":       addition("01", "view-status-addition", "Поле") + "  - {id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field}\n",
		"панели таблиц": "  - {id: c0de0000-0000-4000-8000-000000990001, name: Т1, kind: table, auto_command_bar: {id: c0de0000-0000-4000-8000-000000990011, name: Панель}}\n" +
			"  - {id: c0de0000-0000-4000-8000-000000990002, name: Т2, kind: table, auto_command_bar: {id: c0de0000-0000-4000-8000-000000990012, name: Панель}}\n",
	} {
		if _, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), managedFormConfiguration()); err == nil || !strings.Contains(err.Error(), "name must be unique within the form") {
			t.Errorf("%s: %v", name, err)
		}
	}
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(content), "attributes:\n", "items:\n"+shared+"attributes:\n", 1))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, note := range catalog.Notes() {
		if note.Kind == NoteRepeatedElementName {
			found[note.Where] = note.Written
		}
	}
	want := map[string]string{"catalog Номенклатура form ФормаЭлемента element Addition": "3 times", "catalog Номенклатура form ФормаЭлемента element ContextMenu": "3 times"}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("notes = %v, want %v", found, want)
	}
}
