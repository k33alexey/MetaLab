package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A button names its command as the prototype writes it, every one of the
// six ways and the code of an element, and keeps it with the parameter it
// passes through YAML and the Studio. A command of the form is found by name
// in any case, an element whose standard command a button runs wherever it
// stands in the form.
//
// Defect caught: a button running a standard command of the form (7618 in
// the exports), of an element (9028), a common command (1954), a command of
// an object (955) or the code of an element (571) refused, so that more than
// twenty thousand buttons are not moved; a command of the form in another
// case refused; an element standing after the button not found; the
// parameter of a command lost.
func TestAButtonNamesItsCommandAsThePrototypeDoes(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	button := func(index int, command string) string {
		return "  - {id: c0de0000-0000-4000-8000-0000009901" + string(rune('0'+index/10)) + string(rune('0'+index%10)) +
			", name: Кнопка" + string(rune('А'+index)) + ", kind: button, command: " + command + "}\n"
	}
	commands := []string{
		"Form.Command.заполнить", "Form.StandardCommand.Close", "Form.Item.Список.StandardCommand.Create", "CommonCommand.ОткрытьОстатки",
		"Task.ЗадачаИсполнителя.Command.Выполнено", "Catalog.Номенклатура.StandardCommand.OpenList", "CommonForm.АдреснаяКнига.StandardCommand.Open",
		"\"3:409b9a53-7f7e-4178-86c1-33176c7c7a7a\"", "\"0\"",
	}
	items := ""
	for index, command := range commands {
		items += button(index, command)
	}
	items += "  - {id: c0de0000-0000-4000-8000-000000990150, name: Показать, kind: button, command: Form.StandardCommand.ShowInList," +
		" command_parameter: {object: DocumentJournal.Взаимодействия}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990151, name: Создать, kind: button, command: Form.Item.Список.StandardCommand.CreateByParameter," +
		" command_parameter: {types: [{kind: string, length: 10}]}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990152, name: Список, kind: table}\n"
	source := strings.Replace(formElementsForm(items), "items:\n", "commands:\n  - {id: c0de0000-0000-4000-8000-000000990199, name: Заполнить, title: {ru: Заполнить}, action: refresh}\nitems:\n", 1)
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	for index, command := range commands {
		if got := form.Items[index].Command; got != strings.Trim(command, "\"") {
			t.Errorf("button %d: %q, want %s", index, got, command)
		}
	}
	if name, ok := form.Items[0].FormCommandName(); !ok || name != "заполнить" {
		t.Errorf("the command of the form run by the button: %q, %v", name, ok)
	}
	if _, ok := form.Items[1].FormCommandName(); ok {
		t.Error("a standard command of the form is taken for a command of the form")
	}
	show, create := form.Items[len(commands)], form.Items[len(commands)+1]
	if show.CommandParameter.Object != "DocumentJournal.Взаимодействия" || len(create.CommandParameter.Types) != 1 {
		t.Fatalf("parameters: %+v, %+v", show.CommandParameter, create.CommandParameter)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil || !reflect.DeepEqual(again.Items, form.Items) {
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
	if err := ValidateManagedForm("studio", received, configuration); err != nil || !reflect.DeepEqual(received.Items, form.Items) {
		t.Fatalf("carried through the Studio: %v", err)
	}
}

// What is no command, a command of the form or an element that is not there,
// and a parameter that is not one are refused with the place.
//
// Defect caught: a command written by identifier, as before, or in a shape
// the prototype does not write accepted; a command of an object of a kind
// that has none (a constant), a command of a common form, which has only
// standard ones; a command of the form or an element that is not there
// accepted; a parameter of two kinds at once, an empty one, or one beside no
// command accepted.
func TestAButtonRefusesWhatIsNoCommand(t *testing.T) {
	t.Parallel()
	shapes := "must be Form.Command.<name>, Form.StandardCommand.<name>, Form.Item.<element>.StandardCommand.<name>, CommonCommand.<name>, " +
		"<kind>.<object>.Command.<name>, <kind>.<object>.StandardCommand.<name> or the code of an element of a form"
	for name, test := range map[string]struct{ item, want string }{
		"идентификатор":        {"kind: button, command: c0de0000-0000-4000-8000-000000990199", shapes},
		"без имени":            {"kind: button, command: Form.Command", shapes},
		"команды формы":        {"kind: button, command: Form.Commands.Заполнить", shapes},
		"команда элемента":     {"kind: button, command: Form.Item.Список.Command.Создать", shapes},
		"форма объекта":        {"kind: button, command: Catalog.Номенклатура.Form.ФормаЭлемента", shapes},
		"константа":            {"kind: button, command: Constant.Склад.Command.Открыть", shapes},
		"команда общей формы":  {"kind: button, command: CommonForm.АдреснаяКнига.Command.Открыть", shapes},
		"русская запись":       {"kind: button, command: Форма.Команда.Заполнить", shapes},
		"нет команды формы":    {"kind: button, command: Form.Command.Записать", "items[0].command names no command of the form"},
		"нет элемента":         {"kind: button, command: Form.Item.Список.StandardCommand.Create", "items[0].command names no element of the form"},
		"команда у поля":       {"kind: input-field, command: Form.Command.Заполнить", "items[0].command is allowed only for buttons"},
		"параметр без команды": {"kind: button, command_parameter: {object: Catalog.Номенклатура}", "items[0].command_parameter is allowed only beside a command"},
		"параметр двойной":     {"kind: button, command: Form.StandardCommand.ShowInList, command_parameter: {object: Catalog.Номенклатура, types: [{kind: string}]}", "items[0].command_parameter is either types or an object, not both"},
		"параметр пустой":      {"kind: button, command: Form.StandardCommand.ShowInList, command_parameter: {}", "items[0].command_parameter must be types or an object"},
		"параметр не объект":   {"kind: button, command: Form.StandardCommand.ShowInList, command_parameter: {object: Номенклатура}", "items[0].command_parameter.object must be <kind>.<name> of an object of the configuration or its identifier"},
		"параметр нулевой":     {"kind: button, command: Form.StandardCommand.ShowInList, command_parameter: {object: 00000000-0000-0000-0000-000000000000}", "items[0].command_parameter.object must be"},
	} {
		source := strings.Replace(formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Элемент, "+test.item+"}\n"), "items:\n",
			"commands:\n  - {id: c0de0000-0000-4000-8000-000000990199, name: Заполнить, title: {ru: Заполнить}, action: refresh}\nitems:\n", 1)
		_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), managedFormConfiguration())
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v, want %q", name, err, test.want)
		}
	}
}

// A common command, an object with its command and the object a parameter
// names are found in the project by name, in any case; what is not there is
// a reference to nothing, and the code of an element is noted.
//
// Defect caught: a command of an object or a common command never looked
// for, so that a button running a command removed from the configuration
// reads as sound; a command looked for on another object or among the
// standard ones; a common form taken for an object without commands; the
// code of an element carried silently, against the iron rule.
func TestTheCommandOfAButtonIsResolvedInTheProject(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		command, parameter string
		gone               string
		noted              bool
	}{
		"общая команда":           {command: "CommonCommand.открытьостатки"},
		"команда обработки":       {command: "DataProcessor.Заполнение.Command.Заполнить"},
		"стандартная справочника": {command: "Catalog.Номенклатура.StandardCommand.OpenList"},
		"стандартная общей формы": {command: "CommonForm.АдреснаяКнига.StandardCommand.Open"},
		"параметр объект":         {command: "Form.StandardCommand.ShowInList", parameter: "{object: AccumulationRegister.Остатки}"},
		"параметр типы":           {command: "Form.StandardCommand.ShowInList", parameter: "{types: [{kind: catalog, reference: " + cmpGoods + "}]}"},
		"нет общей команды":       {command: "CommonCommand.Нет", gone: "command"},
		"нет команды у обработки": {command: "DataProcessor.Заполнение.Command.Нет", gone: "command"},
		"команда не того объекта": {command: "Catalog.Номенклатура.Command.Заполнить", gone: "command"},
		"нет объекта":             {command: "Report.Нет.StandardCommand.Open", gone: "command"},
		"нет общей формы":         {command: "CommonForm.Нет.StandardCommand.Open", gone: "command"},
		"параметр нет объекта":    {command: "Form.StandardCommand.ShowInList", parameter: "{object: DocumentJournal.Нет}", gone: "command parameter"},
		"параметр идентификатор":  {command: "Form.StandardCommand.ShowInList", parameter: "{object: 2a07a9b0-3116-4524-93b2-9a1336ae3707}", gone: "command parameter"},
		"код элемента":            {command: "\"3:409b9a53-7f7e-4178-86c1-33176c7c7a7a\"", noted: true},
		"ноль":                    {command: "\"0\"", noted: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			processor := uuid.MustNew().String()
			writeMetadata(t, root, DataProcessorKind, processor, "format: 1\nid: "+processor+"\nname: Заполнение\ntitle: {ru: Заполнение}\n"+
				"commands:\n  - {id: "+uuid.MustNew().String()+", name: Заполнить, title: {ru: Заполнить}}\n")
			writeCommandModule(t, root, DataProcessorKind, "Заполнение", "Заполнить")
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Кнопка, kind: button, command: " + test.command
			if test.parameter != "" {
				element += ", command_parameter: " + test.parameter
			}
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"}\nattributes:\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента element Кнопка "
			if test.gone != "" {
				if found := unresolvedOf(t, root); !containsWhere(found, where+test.gone) {
					t.Fatalf("unresolved = %+v", found)
				}
				return
			}
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			noted := false
			for _, note := range catalog.Notes() {
				if note.Kind == NoteFormReferenceAsWritten {
					noted = note.Where == where+"command" && note.Written == strings.Trim(test.command, "\"")
				}
			}
			if noted != test.noted {
				t.Fatalf("noted = %v: %+v", noted, catalog.Notes())
			}
		})
	}
}

// Every reference of a form the prototype writes as a code or a number is
// noted where it stands, and a sound reference is not.
//
// Defect caught: the code of an element in the source of commands, a number
// in a data path or in the path to the data of a title, or a code in a link
// of a field carried silently, against the iron rule; a plain data path, or
// one with the leading "~" the help explains, noted as if unknown.
func TestEveryReferenceWrittenAsACodeIsNoted(t *testing.T) {
	t.Parallel()
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	items := "items:\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990001, name: Панель, kind: command-bar, command_source: \"8:409b9a53-7f7e-4178-86c1-33176c7c7a7a\"}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Группа, kind: usual-group, title_data_path: \"43\"}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Поле, kind: input-field, data_path: \"~Объект.Код~Объект.Наименование\"," +
		" type_link: {data_path: \"48:409b9a53-7f7e-4178-86c1-33176c7c7a7a/0:3c1e525b-0000-4000-8000-000000000001\"}," +
		" choice_parameter_links: [{name: Отбор.Владелец, data_path: \"22\"}]}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990004, name: Здоровое, kind: input-field, data_path: ~Объект.Код," +
		" choice_parameter_links: [{name: Отбор.Владелец, data_path: Объект.Владелец}]}\n"
	writeFile(t, path, strings.Replace(string(content), "attributes:\n", items+"attributes:\n", 1))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, note := range catalog.Notes() {
		if note.Kind == NoteFormReferenceAsWritten {
			found[strings.TrimPrefix(note.Where, "catalog Номенклатура form ФормаЭлемента element ")] = note.Written
		}
	}
	want := map[string]string{
		"Панель command_source": "8:409b9a53-7f7e-4178-86c1-33176c7c7a7a", "Группа title_data_path": "43",
		"Поле data_path": "~Объект.Код~Объект.Наименование", "Поле type link": "48:409b9a53-7f7e-4178-86c1-33176c7c7a7a/0:3c1e525b-0000-4000-8000-000000000001",
		"Поле choice parameter link Отбор.Владелец": "22",
	}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("notes = %v\nwant %v", found, want)
	}
}

// The code the configurator leaves where a property named an element of the
// form that was deleted since - one segment, the number the element had and
// the identifier of an element of a form - is a remnant of what was deleted,
// in every place the prototype writes a reference to an element: a button's
// command, the source of commands, a link of a field, the table of a
// command, the group of the user settings of a list, and what a form of a
// report and of a hierarchical list names. A data path takes no such code. A
// code with another identifier stays a note, and so does a code of several
// segments outside a link (TestACodedPathOfALinkIsARemnant).
//
// Defect caught: the code of a deleted element carried as a note whose sense
// is not known, though the exports of the configurator show it is what a
// deleted element leaves (a live one is written by name); one place of a
// reference left out; a code of a path of a link, or one with another
// identifier, taken for a deleted element.
func TestTheCodeOfADeletedElementIsARemnant(t *testing.T) {
	t.Parallel()
	const gone = "5:02023637-7868-4a5f-8576-835a76e0c9ba"
	const other = "5:409b9a53-7f7e-4178-86c1-33176c7c7a7a"
	const path = "48:02023637-7868-4a5f-8576-835a76e0c9ba/0:3c1e525b-0000-4000-8000-000000000001"
	commonForm := func(main, properties string) string {
		return "format: 1\nid: c0de0000-0000-4000-8000-000000990060\nname: Отчёт\ntitle: {ru: Отчёт}\nkind: common\n" + properties +
			"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990061, name: Объект, main: true, types: [" + main + "]}\n"
	}
	report := "{kind: report-object, reference: " + cmpReport + "}"
	for name, test := range map[string]struct {
		items, commands, rest, common string
		where                         string
	}{
		"команда кнопки": {items: "  - {id: c0de0000-0000-4000-8000-000000990001, name: Кнопка, kind: button, command: \"" + gone + "\"}\n",
			where: "element Кнопка command"},
		"источник команд": {items: "  - {id: c0de0000-0000-4000-8000-000000990001, name: Панель, kind: command-bar, command_source: \"" + gone + "\"}\n",
			where: "element Панель command_source"},
		"связь параметра выбора": {items: "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, data_path: Объект.Код," +
			" choice_parameter_links: [{name: Отбор.Владелец, data_path: \"" + gone + "\"}]}\n", where: "element Поле choice parameter link Отбор.Владелец"},
		"связь по типу": {items: "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, data_path: Объект.Код," +
			" type_link: {data_path: \"" + gone + "\"}}\n", where: "element Поле type link"},
		"таблица команды": {commands: "commands:\n  - {id: c0de0000-0000-4000-8000-000000990010, name: Удалить, title: {ru: Удалить}, action: custom," +
			" current_row_use: use, associated_table: \"" + gone + "\"}\n", where: "command Удалить associated_table"},
		"группа настроек списка": {items: "  - {id: c0de0000-0000-4000-8000-000000990001, name: Таблица, kind: table, data_path: Список," +
			" dynamic_list: {period: {variant: custom}, user_settings_group: \"" + gone + "\"}}\n", where: "element Таблица user_settings_group"},
		"список групп":               {common: commonForm("{kind: dynamic-list}", "group_list: \""+gone+"\"\n"), where: "group_list"},
		"папка настроек отчёта":      {common: commonForm(report, "custom_settings_folder: \""+gone+"\"\n"), where: "custom_settings_folder"},
		"отображение варианта":       {common: commonForm(report, "variant_appearance: \""+gone+"\"\n"), where: "variant_appearance"},
		"результат отчёта":           {common: commonForm(report, "report_result: \""+gone+"\"\n"), where: "report_result"},
		"данные расшифровки":         {common: commonForm(report, "details_data: \""+gone+"\"\n"), where: "details_data"},
		"мобильная командная панель": {rest: "mobile_command_bar: [\"" + gone + "\"]\n", where: "mobile_command_bar[0]"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			at := "catalog Номенклатура form ФормаЭлемента "
			file := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			if test.common != "" {
				writeCommonForm(t, root, "Отчёт", test.common)
				at = "common form Отчёт "
				file = filepath.Join(root, "metadata", "common-forms", "Отчёт", project.FormMetadataFile)
			} else {
				content, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				form := string(content)
				if test.items != "" {
					form = strings.Replace(form, "attributes:\n", "items:\n"+test.items+"attributes:\n", 1)
				}
				writeFile(t, file, form+test.commands+test.rest)
			}
			unresolved := unresolvedOf(t, root)
			if len(unresolved) != 1 || unresolved[0].Where != at+test.where || unresolved[0].Written != gone {
				t.Fatalf("unresolved = %+v, want %q", unresolved, at+test.where)
			}
			// The same place with a code that is no deleted element is a note.
			others := []string{other, path}
			if strings.Contains(test.where, " link") {
				others = others[:1]
			}
			for _, written := range others {
				replaceInFile(t, file, gone, written)
				catalog, err := Load(root)
				if err != nil {
					t.Fatalf("%s: %v", written, err)
				}
				noted := false
				for _, note := range catalog.Notes() {
					if note.Kind == NoteFormReferenceAsWritten && note.Where == at+test.where && note.Written == written {
						noted = true
					}
				}
				if !noted {
					t.Fatalf("%s at %s is not noted: %+v", written, test.where, catalog.Notes())
				}
				replaceInFile(t, file, written, gone)
			}
		})
	}
}

// The path of a link of a field written as a code of several segments - the
// first a table, an attribute of the form or a deleted element, the next a
// field of it - is what the configurator leaves when the path leads nowhere,
// and is a remnant of what was deleted, in a link of a choice parameter and
// in a link by type alike, in every writing of the exports: through a table
// with an attribute ("48:…/0:<id>"), through an attribute of the form
// ("1/0:<id>"), with a number ("342:…/15") and another number before the
// identifier ("35:…/18:<id>"). A link written by a number, a code of one
// segment with another identifier and a path from another identifier, none
// of them in the exports as a path, stay notes.
//
// Defect caught: 36 such paths of the exports, each leading to a deleted
// table or to a field the table or the object does not have, carried as
// notes whose sense is not known instead of remnants; one of the two links
// left out; a path of one segment that is no deleted element, or a path from
// an identifier that is not of an element, taken for a remnant.
func TestACodedPathOfALinkIsARemnant(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		written string
		remnant bool
	}{
		"таблица и колонка":         {"48:02023637-7868-4a5f-8576-835a76e0c9ba/0:3c1e525b-0000-4000-8000-000000000001", true},
		"реквизит формы и поле":     {"1/0:ba7dcb3b-b8b9-4d44-ab5f-56f7a228f10d", true},
		"удалённый элемент и номер": {"342:02023637-7868-4a5f-8576-835a76e0c9ba/15", true},
		"другой номер перед полем":  {"35:02023637-7868-4a5f-8576-835a76e0c9ba/18:5bdad865-0000-4000-8000-000000000002", true},
		"число": {"20", false},
		"код с другим идентификатором":   {"5:409b9a53-7f7e-4178-86c1-33176c7c7a7a", false},
		"путь от другого идентификатора": {"48:409b9a53-7f7e-4178-86c1-33176c7c7a7a/0:3c1e525b-0000-4000-8000-000000000001", false},
	} {
		for _, link := range []struct{ yaml, where string }{
			{"choice_parameter_links: [{name: Отбор.Владелец, data_path: \"%s\"}]", "choice parameter link Отбор.Владелец"},
			{"type_link: {data_path: \"%s\"}", "type link"},
		} {
			t.Run(name+"/"+link.where, func(t *testing.T) {
				t.Parallel()
				root := formReferencesProject(t)
				file := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
				content, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				item := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, data_path: Объект.Код, " + fmt.Sprintf(link.yaml, test.written) + "}\n"
				writeFile(t, file, strings.Replace(string(content), "attributes:\n", "items:\n"+item+"attributes:\n", 1))
				if _, err := Load(root); errors.Is(err, ErrUnresolvedReference) != test.remnant {
					t.Fatalf("the strict load: %v", err)
				}
				catalog, err := read(root, true, false)
				if err != nil {
					t.Fatal(err)
				}
				where := "catalog Номенклатура form ФормаЭлемента element Поле " + link.where
				remnant := false
				for _, reference := range catalog.UnresolvedReferences() {
					if reference.Where == where && reference.Written == test.written {
						remnant = true
					}
				}
				noted := false
				for _, note := range catalog.Notes() {
					if note.Kind == NoteFormReferenceAsWritten && note.Where == where && note.Written == test.written {
						noted = true
					}
				}
				if remnant != test.remnant || noted == test.remnant {
					t.Fatalf("remnant %v, noted %v; unresolved %+v, notes %+v", remnant, noted, catalog.UnresolvedReferences(), catalog.Notes())
				}
			})
		}
	}
}

// TestACodedDataPathIsARemnant catches a data path of an element written as a
// code of several segments being refused - 139 of lombard1 were, and the
// configuration did not load - or being carried silently instead of as a
// remnant of what was deleted; and a code that is no path being let through.
func TestACodedDataPathIsARemnant(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		written           string
		remnant, rejected bool
	}{
		"реквизит формы и поле":        {written: "1/0:ba7dcb3b-b8b9-4d44-ab5f-56f7a228f10d", remnant: true},
		"табличная часть и колонка":    {written: "1/0:09365303-009d-4b39-93ec-6a7cf420f7b3/0:ca657fe8-6a10-42d1-b65f-26a7f3dc6cfd", remnant: true},
		"подвал колонки":               {written: "1/0:844ef438-7e09-4022-8c7b-ad17e9c90794/101000000:232dcdf1-af2a-4000-8000-000000000001", remnant: true},
		"стандартный реквизит":         {written: "1/-2", remnant: true},
		"колонка таблицы значений":     {written: "5/10000000", remnant: true},
		"таблица и колонка":            {written: "1485:02023637-7868-4a5f-8576-835a76e0c9ba/0:8ff6c337-2714-4ff8-89be-58cbbbb70bf0", remnant: true},
		"число":                        {written: "20"},
		"путь от другого кода":         {written: "48:409b9a53-7f7e-4178-86c1-33176c7c7a7a/0:3c1e525b-0000-4000-8000-000000000001"},
		"отрицательное начало":         {written: "-1/0", rejected: true},
		"пустой сегмент":               {written: "1//0", rejected: true},
		"минус без числа":              {written: "1/-", rejected: true},
		"идентификатор не того вида":   {written: "1/0:BA7DCB3B-B8B9-4D44-AB5F-56F7A228F10D", rejected: true},
		"путь, кончающийся косой":      {written: "1/", rejected: true},
		"идентификатор без номера":     {written: "1/:ba7dcb3b-b8b9-4d44-ab5f-56f7a228f10d", rejected: true},
		"код удалённого элемента один": {written: "12:02023637-7868-4a5f-8576-835a76e0c9ba", rejected: true},
	} {
		for _, place := range []struct{ yaml, name string }{
			{"kind: input-field, data_path: \"%s\"", "data_path"},
			{"kind: input-field, data_path: Объект.Код, footer_data_path: \"%s\"", "footer_data_path"},
			{"kind: usual-group, title_data_path: \"%s\"", "title_data_path"},
			{"kind: table, row_picture_data_path: \"%s\"", "row_picture_data_path"},
		} {
			t.Run(name+"/"+place.name, func(t *testing.T) {
				t.Parallel()
				root := formReferencesProject(t)
				file := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
				content, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				item := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, " + fmt.Sprintf(place.yaml, test.written) + "}\n"
				writeFile(t, file, strings.Replace(string(content), "attributes:\n", "items:\n"+item+"attributes:\n", 1))
				_, err = Load(root)
				if test.rejected {
					if err == nil || errors.Is(err, ErrUnresolvedReference) || !strings.Contains(err.Error(), place.name) {
						t.Fatalf("the load: %v", err)
					}
					return
				}
				if errors.Is(err, ErrUnresolvedReference) != test.remnant || err != nil && !test.remnant {
					t.Fatalf("the strict load: %v", err)
				}
				catalog, err := read(root, true, false)
				if err != nil {
					t.Fatal(err)
				}
				where := "catalog Номенклатура form ФормаЭлемента element Поле " + place.name
				remnant := false
				for _, reference := range catalog.UnresolvedReferences() {
					if reference.Where == where && reference.Written == test.written {
						remnant = true
					}
				}
				noted := false
				for _, note := range catalog.Notes() {
					if note.Kind == NoteFormReferenceAsWritten && note.Where == where && note.Written == test.written {
						noted = true
					}
				}
				if remnant != test.remnant || noted == test.remnant {
					t.Fatalf("remnant %v, noted %v; unresolved %+v, notes %+v", remnant, noted, catalog.UnresolvedReferences(), catalog.Notes())
				}
			})
		}
	}
}
