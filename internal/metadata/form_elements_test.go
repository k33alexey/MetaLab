package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
)

// formElementsForm is a form holding the given YAML items.
func formElementsForm(items string) string {
	return formAttrHead + "items:\n" + items
}

// formElementSerial names the elements apart; the tests that use it run side
// by side.
var formElementSerial atomic.Int64

// formElement writes one element of the given kind, with children.
func formElement(kind FormElementKind, children ...string) string {
	serial := formElementSerial.Add(1)
	element := fmt.Sprintf("{id: c0de0000-0000-4000-8000-%012d, name: Э%d, kind: %s", 900000+serial, serial, kind)
	if len(children) != 0 {
		element += ", children: [" + strings.Join(children, ", ") + "]"
	}
	return element + "}"
}

// Every kind of element the forms being moved write stands where they put
// it: each of the twenty fields, the seven groups, the two decorations, the
// button, the table and the three additions of a table, nested as the 8726
// forms of the four exports nest them.
//
// Defect caught: a kind the prototype writes missing from the model, so that
// every form holding one is refused; a nesting the prototype makes refused -
// a page in pages, a column of a table in a group of columns, a button in a
// popup in a command bar, a search string beside a table in a usual group.
func TestEveryKindOfElementStandsWhereThePrototypePutsIt(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	var fields []string
	for kind, class := range formElementClasses {
		if class == formFieldClass {
			fields = append(fields, formElement(kind))
		}
	}
	if len(fields) != 20 {
		t.Fatalf("fields = %d, the help gives twenty", len(fields))
	}
	items := "  - " + formElement(FormElementUsualGroup, fields...) + "\n" +
		"  - " + formElement(FormElementPages, formElement(FormElementPage, formElement(FormElementInputField), formElement(FormElementLabelDecoration))) + "\n" +
		"  - " + formElement(FormElementTable, formElement(FormElementLabelField), formElement(FormElementColumnGroup, formElement(FormElementCheckBoxField), formElement(FormElementColumnGroup, formElement(FormElementPictureField)))) + "\n" +
		"  - " + formElement(FormElementCommandBar, formElement(FormElementButton), formElement(FormElementPopup, formElement(FormElementButton), formElement(FormElementButtonGroup, formElement(FormElementButton))), formElement(FormElementSearchStringAddition), formElement(FormElementSearchControlAddition)) + "\n" +
		"  - " + formElement(FormElementUsualGroup, formElement(FormElementPictureDecoration), formElement(FormElementButton), formElement(FormElementPopup), formElement(FormElementViewStatusAddition), formElement(FormElementSearchStringAddition)) + "\n" +
		"  - " + formElement(FormElementSpreadsheetDocumentField) + "\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration); err != nil {
		t.Fatal(err)
	}
}

// What the prototype never nests is refused, naming the place, the holder and
// what it cannot hold; a kind that is none of the form's is refused by name -
// the five of the earlier model among them.
//
// Defect caught: a page standing outside pages, a field holding elements, a
// button among the columns of a table, a field in a command bar - a form the
// platform cannot lay out accepted; an old kind silently taken for a new one.
func TestAnElementHoldsOnlyWhatThePrototypeNestsInIt(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ items, want string }{
		"страница вне страниц":      {"  - " + formElement(FormElementPage) + "\n", "items[0]: a form cannot hold page"},
		"страница в обычной группе": {"  - " + formElement(FormElementUsualGroup, formElement(FormElementPage)) + "\n", "items[0].children[0]: usual-group cannot hold page"},
		"группа в страницах":        {"  - " + formElement(FormElementPages, formElement(FormElementUsualGroup)) + "\n", "items[0].children[0]: pages cannot hold usual-group"},
		"кнопка в таблице":          {"  - " + formElement(FormElementTable, formElement(FormElementButton)) + "\n", "table cannot hold button"},
		"группа в группе колонок":   {"  - " + formElement(FormElementTable, formElement(FormElementColumnGroup, formElement(FormElementUsualGroup))) + "\n", "column-group cannot hold usual-group"},
		"поле в командной панели":   {"  - " + formElement(FormElementCommandBar, formElement(FormElementInputField)) + "\n", "command-bar cannot hold input-field"},
		"панель в группе кнопок":    {"  - " + formElement(FormElementCommandBar, formElement(FormElementButtonGroup, formElement(FormElementCommandBar))) + "\n", "button-group cannot hold command-bar"},
		"поле с вложенным":          {"  - " + formElement(FormElementInputField, formElement(FormElementLabelDecoration)) + "\n", "input-field cannot hold label-decoration"},
		"декорация с вложенным":     {"  - " + formElement(FormElementLabelDecoration, formElement(FormElementButton)) + "\n", "label-decoration cannot hold button"},
		"состояние в панели":        {"  - " + formElement(FormElementCommandBar, formElement(FormElementViewStatusAddition)) + "\n", "command-bar cannot hold view-status-addition"},
		"прежнее поле":              {"  - " + formElement("field") + "\n", "items[0].kind field is not a kind of element of a form"},
		"прежняя надпись":           {"  - " + formElement("label") + "\n", "items[0].kind label is not a kind of element of a form"},
		"прежняя группа":            {"  - " + formElement("group") + "\n", "items[0].kind group is not a kind of element of a form"},
		"тип кнопки не у кнопки":    {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, button_type: hyperlink}\n", "items[0].button_type is allowed only for buttons"},
		"тип кнопки":                {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Кнопка, kind: button, button_type: link}\n", "items[0].button_type must be usual-button, hyperlink, command-bar-button or command-bar-hyperlink"},
		"ориентация у таблицы":      {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Таблица, kind: table, orientation: vertical}\n", "items[0].orientation is allowed only for usual groups, pages and groups of columns"},
		"путь к данным у декорации": {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Надпись, kind: label-decoration, data_path: Объект}\n", "items[0].data_path is allowed only for fields, tables and buttons"},
		"только просмотр у группы":  {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Группа, kind: usual-group, read_only: true}\n", "items[0].read_only is allowed only for fields and tables"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(test.items)), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// A button keeps its type, a usual group, a page and a group of columns their
// orientation, and an orientation left out is accepted as vertical.
//
// Defect caught: the type of a button lost, so that a hyperlink is drawn as a
// button; a group that does not write its layout refused, as the prototype
// writes none for a vertical one.
func TestAButtonKeepsItsTypeAndAGroupItsLayout(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Ссылка, kind: button, button_type: hyperlink}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Группа, kind: usual-group}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Страницы, kind: pages, children: [{id: c0de0000-0000-4000-8000-000000990004, name: Страница, kind: page, orientation: horizontal}]}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if form.Items[0].ButtonType != FormButtonHyperlink || form.Items[1].Orientation != "" || form.Items[2].Children[0].Orientation != FormHorizontal {
		t.Fatalf("items = %+v", form.Items)
	}
}

// What every element has - its tooltip, how the tooltip shows, whom it is
// shown to - is kept on a field, a group, a decoration, a table and a button,
// and comes back the same through YAML and the Studio.
//
// Defect caught: a common property kept on one kind and lost on another; the
// roles an element is shown to lost, so that a field the prototype shows to
// no one until the user turns it on - 6590 of them - shows to everyone.
func TestEveryElementKeepsWhatEveryElementHas(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	common := ", tool_tip: {ru: Подсказка}, tool_tip_representation: show-bottom, user_visible: {common: false, roles: [{role: " + formAttrRole + ", value: true}]}"
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field" + common + "}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Группа, kind: usual-group" + common + "}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Надпись, kind: label-decoration" + common + "}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990004, name: Таблица, kind: table" + common + "}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990005, name: Кнопка, kind: button, tool_tip_representation: balloon, user_visible: {common: false}}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		for _, item := range items[:4] {
			if item.ToolTip["ru"] != "Подсказка" || item.ToolTipRepresentation != FormToolTipShowBottom || item.UserVisible == nil ||
				item.UserVisible.Common || len(item.UserVisible.Roles) != 1 || item.UserVisible.Roles[0].Role.String() != formAttrRole {
				t.Fatalf("%s: %s lost what every element has: %+v", source, item.Kind, item)
			}
		}
		if button := items[4]; button.ToolTipRepresentation != FormToolTipBalloon || button.UserVisible == nil || button.UserVisible.Common {
			t.Fatalf("%s: the button: %+v", source, button)
		}
	}
	check("read", form.Items)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.Items)
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
	check("carried through the Studio", received.Items)
}

// What every element has is refused when it is wrong, naming the element.
//
// Defect caught: a tooltip written on a button, which shows the tooltip of
// its command, kept where nothing shows it; a representation the help does
// not give; a role answering twice; a tooltip in a language the
// configuration does not have.
func TestEveryElementRefusesWhatIsWrongInWhatEveryElementHas(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"подсказка у кнопки":      {"kind: button, tool_tip: {ru: П}", "items[0].tool_tip is the tooltip of the command for a button"},
		"отображение подсказки":   {"kind: input-field, tool_tip_representation: hover", "items[0].tool_tip_representation must be auto, none, button, balloon, show-auto, show-top, show-left, show-bottom or show-right"},
		"роль дважды":             {"kind: input-field, user_visible: {common: false, roles: [{role: " + formAttrRole + ", value: true}, {role: " + formAttrRole + ", value: false}]}", "items[0].user_visible.roles[1].role already has its answer"},
		"роль без идентификатора": {"kind: usual-group, user_visible: {common: true, roles: [{role: 00000000-0000-0000-0000-000000000000, value: false}]}", "items[0].user_visible.roles[0].role must be a non-zero UUID"},
		"подсказка не на языке":   {"kind: label-decoration, tool_tip: {\"d=e\": П}", "items[0].tool_tip"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Элемент, " + test.element + "}\n")
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// The roles an element of a form of an object or of a common form is shown to
// are resolved with the project: a role that is gone is a reference to
// nothing named with the form and the element - the deleted role of erp
// written on three fields among them - and the role editor, which reads
// without the roles, does not take a role for one that is gone.
//
// Defect caught: an element shown to a role the project does not have,
// loading clean; a nested element left unresolved; every role of a form
// listed as gone in the role editor.
func TestTheRolesAnElementIsShownToAreResolved(t *testing.T) {
	t.Parallel()
	element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Группа, kind: usual-group, children: [" +
		"{id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, user_visible: {common: false, roles: [{role: ROLE, value: true}]}}]}\n"
	for _, place := range []string{"object", "common"} {
		root := formReferencesProject(t)
		path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
		where := "catalog Номенклатура form ФормаЭлемента element Поле user visibility of role"
		if place == "common" {
			path = filepath.Join(root, "metadata", "common-forms", "АдреснаяКнига", project.FormMetadataFile)
			where = "common form АдреснаяКнига element Поле user visibility of role"
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		withElement := strings.Replace(string(content), "attributes:\n", strings.ReplaceAll(element, "ROLE", cmpRole)+"attributes:\n", 1)
		writeFile(t, path, withElement)
		if _, err := Load(root); err != nil {
			t.Fatalf("%s: a role of the project: %v", place, err)
		}
		writeFile(t, path, strings.Replace(withElement, cmpRole, refGone, 1))
		if found := unresolvedOf(t, root); !containsWhere(found, where) {
			t.Fatalf("%s: unresolved = %+v", place, found)
		}
		catalog, err := read(root, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if found := catalog.UnresolvedReferences(); containsWhere(found, where) {
			t.Fatalf("%s: read without the roles: %+v", place, found)
		}
	}
}

// A field keeps how it is titled, entered and edited, through YAML and the
// Studio; skipping on input keeps all three of its states.
//
// Defect caught: a property of a field read into the wrong field or into
// none, or lost through the Studio; skipping on input kept as a plain yes or
// no, so that the 32 fields the prototype writes "no" on and the ones it
// leaves to the warning on edit become the same.
func TestAFieldKeepsHowItIsTitledEnteredAndEdited(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, title_location: top, skip_on_input: false, default_item: true," +
		" edit_mode: enter-on-input, warning_on_edit: {ru: Осторожно}, warning_on_edit_representation: show, shortcut: Cmd+Shift+F}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Флажок, kind: check-box-field, skip_on_input: true}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Надпись, kind: label-field}\n"
	no, yes := false, true
	want := FieldBehavior{TitleLocation: FormTitleTop, SkipOnInput: &no, DefaultItem: true, EditMode: FormEditEnterOnInput,
		WarningOnEdit: LocalizedText{"ru": "Осторожно"}, WarningOnEditRepresentation: FormWarningOnEditShow, Shortcut: "Cmd+Shift+F"}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		if !reflect.DeepEqual(items[0].FieldBehavior, want) {
			t.Fatalf("%s: %+v, want %+v", source, items[0].FieldBehavior, want)
		}
		if !reflect.DeepEqual(items[1].FieldBehavior, FieldBehavior{SkipOnInput: &yes}) || items[2].SkipOnInput != nil {
			t.Fatalf("%s: skipping on input: %v, %v", source, items[1].SkipOnInput, items[2].SkipOnInput)
		}
	}
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("read", form.Items)
	value := reflect.ValueOf(want)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.Items)
	// A field that says nothing of skipping on input writes nothing of it.
	if strings.Contains(string(written), "null") {
		t.Fatalf("a property not said is written:\n%s", written)
	}
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
	check("carried through the Studio", received.Items)
}

// What only a field has is refused elsewhere, and a value the help does not
// give is refused by name.
//
// Defect caught: the title location or the edit mode kept on a group, where
// nothing runs it; a value the field cannot run accepted.
func TestAFieldRefusesWhatIsWrongInHowItIsEdited(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"заголовок у группы":         {"kind: usual-group, title_location: top", "items[0] has what only a field has"},
		"сочетание у декорации":      {"kind: label-decoration, shortcut: F5", "items[0] has what only a field has"},
		"пропуск у кнопки":           {"kind: button, skip_on_input: true", "items[0] has what only a field has"},
		"по умолчанию у группы":      {"kind: usual-group, default_item: true", "items[0] has what only a field has"},
		"положение заголовка":        {"kind: input-field, title_location: center", "items[0].title_location must be auto, none, left, right, top or bottom"},
		"режим редактирования":       {"kind: input-field, edit_mode: inline", "items[0].edit_mode must be enter, enter-on-input or directly"},
		"отображение предупреждения": {"kind: input-field, warning_on_edit_representation: always", "items[0].warning_on_edit_representation must be auto, show or dont-show"},
		"предупреждение не на языке": {"kind: input-field, warning_on_edit: {\"d=e\": О}", "items[0].warning_on_edit"},
		"пробелы в сочетании":        {"kind: input-field, shortcut: \" F5\"", "items[0].shortcut must be written without surrounding spaces"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Элемент, " + test.element + "}\n")
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// The data path of an element is taken as the prototype writes one - an
// item of a collection by its index, a leading "~", a number, two paths
// joined by "~", and the path a button has - and what is no path is refused.
//
// Defect caught: the 766 data paths of the exports that the earlier rule
// refused - 372 with "~", 290 with an index, the number and the joined
// paths - refusing their forms; a button's path refused; a broken path, an
// empty index or a name starting with a digit accepted.
func TestTheDataPathOfAnElementIsTakenAsThePrototypeWritesIt(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"Объект.Наименование", "~Список.ШагБюджетногоПроцесса", "ОбъектПрототип[0].Владелец", "Объект.ОбработчикиОбновления[0].Идентификатор",
		"КомпоновщикНастроек.Settings.ConditionalAppearance[0].Appearance.Parameter", "Таблица[0][1].Поле", "25", "~Список.Code~Список.Код",
	} {
		if issues := validateElementDataPath("data_path", path); len(issues) != 0 {
			t.Errorf("%q refused: %v", path, issues)
		}
	}
	for _, path := range []string{"", " Объект", "Объект..Поле", "Объект.", "[0].Поле", "Объект[x].Поле", "Объект[0", "Объект[]", "Объект[0]Поле", "~", "Объект.1Поле", "Список.Code~Список.Код", "~Список~", "Объект." + nameAt(maxNameLength+1)} {
		if issues := validateElementDataPath("data_path", path); len(issues) == 0 {
			t.Errorf("%q accepted", path)
		}
	}
	configuration := managedFormConfiguration()
	source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Кнопка, kind: button, data_path: \"~Items.Список.CurrentData.Ref\"}\n")
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err != nil {
		t.Fatalf("the data path of a button: %v", err)
	}
}
