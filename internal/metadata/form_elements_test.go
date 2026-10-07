package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
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

// A field keeps its size and where it stands, through YAML and the Studio:
// both stretches in all three of their states, sizes past anything the
// exports hold, as the prototype sets no limit.
//
// Defect caught: a property of the size read into the wrong field or into
// none, or lost through the Studio; a stretch kept as a plain yes or no, so
// that a field not told to stretch - which stretches or not by its kind -
// is made not to; a limit on a size the prototype does not have.
func TestAFieldKeepsItsSizeAndWhereItStands(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, width: 100000, height: 40, no_auto_max_width: true," +
		" max_width: 1000, no_auto_max_height: true, max_height: 51, horizontal_stretch: false, vertical_stretch: true," +
		" group_horizontal_align: right, group_vertical_align: center, horizontal_align: left, vertical_align: bottom}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Документ, kind: spreadsheet-document-field, horizontal_stretch: true, vertical_stretch: false}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Надпись, kind: label-field}\n"
	no, yes := false, true
	want := FieldLayout{Width: 100000, Height: 40, NoAutoMaxWidth: true, MaxWidth: 1000, NoAutoMaxHeight: true, MaxHeight: 51,
		HorizontalStretch: &no, VerticalStretch: &yes, GroupHorizontalAlign: ItemHorizontalRight, GroupVerticalAlign: ItemVerticalCenter,
		HorizontalAlign: ItemHorizontalLeft, VerticalAlign: ItemVerticalBottom}
	check := func(source string, items []ManagedFormElement) {
		t.Helper()
		if !reflect.DeepEqual(items[0].FieldLayout, want) {
			t.Fatalf("%s: %+v, want %+v", source, items[0].FieldLayout, want)
		}
		if !reflect.DeepEqual(items[1].FieldLayout, FieldLayout{HorizontalStretch: &yes, VerticalStretch: &no}) || items[2].FieldLayout != (FieldLayout{}) {
			t.Fatalf("%s: stretches: %+v, %+v", source, items[1].FieldLayout, items[2].FieldLayout)
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

// The size and alignment of a field stand only on a field, a size is not
// negative, and an alignment is one the help gives.
//
// Defect caught: the size of a field kept on a group, where the size of a
// group - another property - belongs; a negative size; justify, which the
// help gives the text of a cell and not an item, accepted.
func TestAFieldRefusesWhatIsWrongInItsSize(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"ширина у группы":          {"kind: usual-group, width: 10", "items[0] has the size and alignment of a field"},
		"растягивание у кнопки":    {"kind: button, horizontal_stretch: true", "items[0] has the size and alignment of a field"},
		"выравнивание у декорации": {"kind: label-decoration, horizontal_align: left", "items[0] has the size and alignment of a field"},
		"ширина":                  {"kind: input-field, width: -1", "items[0].width must not be negative"},
		"высота":                  {"kind: input-field, height: -1", "items[0].height must not be negative"},
		"максимальная ширина":     {"kind: input-field, max_width: -1", "items[0].max_width must not be negative"},
		"максимальная высота":     {"kind: input-field, max_height: -1", "items[0].max_height must not be negative"},
		"в группе по горизонтали": {"kind: input-field, group_horizontal_align: justify", "items[0].group_horizontal_align must be auto, left, center or right"},
		"в группе по вертикали":   {"kind: input-field, group_vertical_align: middle", "items[0].group_vertical_align must be auto, top, center or bottom"},
		"по горизонтали":          {"kind: input-field, horizontal_align: justify", "items[0].horizontal_align must be auto, left, center or right"},
		"по вертикали":            {"kind: input-field, vertical_align: middle", "items[0].vertical_align must be auto, top, center or bottom"},
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

// A field keeps how it and its title are drawn, each colour, font and border
// written every way the prototype writes one on a field, through YAML and the
// Studio.
//
// Defect caught: a colour of the system palette (win:Highlight), a font of
// the system (sys:DefaultGUIFont) or an absolute one refused, so that a form
// drawn with one is not moved; a value read into the wrong property or lost
// through the Studio.
func TestAFieldKeepsHowItIsDrawn(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field," +
		" font: {source: style, from: {standard: NormalTextFont}, bold: true, size: 10}," +
		" text_color: {source: web, name: MediumGray}, back_color: {source: absolute, rgb: '#777777'}," +
		" border_color: {source: style, from: {standard: BorderColor}}, border: {source: absolute, line: single, width: 2}," +
		" title_font: {source: system, face: DefaultGUIFont, italic: false}, title_text_color: {source: system, name: Highlight}," +
		" title_back_color: {source: auto}, title_height: 2}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Надпись, kind: label-field, font: {source: absolute, face: MS Shell Dlg, size: 12, scale: 100}," +
		" border: {source: absolute, line: none, width: 1}}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	look := form.Items[0].FieldLook
	value := reflect.ValueOf(look)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	switch {
	case look.Font.Source != StyleFont || look.Font.From.Standard != "NormalTextFont" || look.Font.Bold == nil || !*look.Font.Bold || look.Font.Size != 10:
		t.Fatalf("font: %+v", look.Font)
	case look.TextColor.Name != "MediumGray" || look.BackColor.RGB != "#777777" || look.BorderColor.From.Standard != "BorderColor":
		t.Fatalf("colours: %+v %+v %+v", look.TextColor, look.BackColor, look.BorderColor)
	case look.Border.Line != SingleBorderLine || look.Border.Width != 2 || look.TitleHeight != 2:
		t.Fatalf("border or title height: %+v %d", look.Border, look.TitleHeight)
	case look.TitleFont.Source != SystemFont || look.TitleTextColor.Source != SystemColor || look.TitleBackColor.Source != AutoColor:
		t.Fatalf("title: %+v %+v %+v", look.TitleFont, look.TitleTextColor, look.TitleBackColor)
	case form.Items[1].Font.Face != "MS Shell Dlg" || form.Items[1].Border.Line != NoBorderLine:
		t.Fatalf("label field: %+v", form.Items[1].FieldLook)
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

// The look of a field stands only on a field and is checked as a value of a
// style item is.
//
// Defect caught: the look of a field kept on a group; a colour that is no
// colour, a font with no face, a border thicker than the configurator takes,
// a negative title height accepted.
func TestAFieldRefusesWhatIsWrongInHowItIsDrawn(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"шрифт у группы":   {"kind: usual-group, font: {source: auto}", "items[0] has the look of a field"},
		"цвет у кнопки":    {"kind: button, text_color: {source: auto}", "items[0] has the look of a field"},
		"цвет не цвет":     {"kind: input-field, back_color: {source: absolute, rgb: red}", "items[0].back_color.rgb must be a colour written as #RRGGBB"},
		"цвет заголовка":   {"kind: input-field, title_back_color: {source: style}", "items[0].title_back_color.from must name the style item"},
		"шрифт без имени":  {"kind: input-field, title_font: {source: absolute, size: 10}", "items[0].title_font.face must name the font"},
		"рамка толще":      {"kind: input-field, border: {source: absolute, line: single, width: 6}", "items[0].border.width must be between 0 and 5"},
		"цвет рамки":       {"kind: input-field, border_color: {source: paint}", "items[0].border_color.source must be absolute, web, system, auto or style"},
		"текст":            {"kind: input-field, text_color: {source: web}", "items[0].text_color.name must name a colour of the palette"},
		"шрифт":            {"kind: input-field, font: {source: style, from: {standard: NormalTextFont}, scale: 1000}", "items[0].font.scale must be a percentage"},
		"высота заголовка": {"kind: input-field, title_height: -1", "items[0].title_height must not be negative"},
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

// A style item of the configuration a field takes its look from is resolved
// with the project: one that is there and of the type taken stands, one that
// is gone is a reference to nothing, one of another type refuses the project.
//
// Defect caught: a field drawn with a style item the project does not have,
// loading clean; a font taken from a style item that is a colour accepted,
// which has nothing to draw the font with.
func TestTheStyleItemsAFieldIsDrawnWithAreResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		look    string
		refused string
		gone    bool
	}{
		"цвет из элемента стиля":    {"text_color: {source: style, from: {item: " + cmpStyleItem + "}}", "", false},
		"удалённый элемент стиля":   {"back_color: {source: style, from: {item: " + refGone + "}}", "", true},
		"шрифт из цвета":            {"title_font: {source: style, from: {item: " + cmpStyleItem + "}}", "catalog Номенклатура form ФормаЭлемента element Поле title_font takes its value from style item ЦветВажного, which is a color and not a font", false},
		"вложенный, рамка из цвета": {"border: {source: style, from: {item: " + cmpStyleItem + "}}", "element Поле border takes its value from style item ЦветВажного, which is a color and not a border", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Группа, kind: usual-group, children: [" +
				"{id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, " + test.look + "}]}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			switch {
			case test.gone:
				if found := unresolvedOf(t, root); !containsWhere(found, "catalog Номенклатура form ФормаЭлемента element Поле back_color") {
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

// A field keeps what it has as a column of a table - its header and footer,
// their pictures, the font, colours and alignment of the footer, whether it
// stays in place, and how its cells are shown - through YAML and the Studio,
// and on a field that stands outside a table, as the prototype writes it.
//
// Defect caught: a column taken out of the header or the footer shown there
// after the move; a total in the footer, its picture or its colour lost or
// read into the wrong property; the alignment in the footer of a field outside
// a table refused, so that 479 such fields are not moved.
func TestAFieldKeepsWhatItHasAsAColumn(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Таблица, kind: table, children: [" +
		"{id: c0de0000-0000-4000-8000-000000990002, name: Сумма, kind: input-field, hidden_in_header: true, hidden_in_footer: true," +
		" header_picture: {standard: Change, load_transparent: true, transparent_pixel: {x: 4, y: 2}}," +
		" footer_picture: {common: c0de0000-0000-4000-8000-000000000043}, header_horizontal_align: center," +
		" footer_text: {ru: Итого}, footer_data_path: Объект.Товары.TotalСумма, footer_horizontal_align: right," +
		" footer_font: {source: system, face: DefaultGUIFont, bold: true}, footer_text_color: {source: system, name: Highlight}," +
		" footer_back_color: {source: web, name: MediumGray}, fixing_in_table: left, cell_hyperlink: true, auto_cell_height: true}]}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Вне, kind: label-field, footer_horizontal_align: left, auto_cell_height: true}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	column := form.Items[0].Children[0].FieldColumn
	value := reflect.ValueOf(column)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	switch {
	case !column.HiddenInHeader || !column.HiddenInFooter || column.FixingInTable != FormFixingLeft || !column.CellHyperlink || !column.AutoCellHeight:
		t.Fatalf("column: %+v", column)
	case column.HeaderPicture.Standard != "Change" || column.HeaderPicture.TransparentPixel == nil || column.FooterPicture.Common == nil:
		t.Fatalf("pictures: %+v %+v", column.HeaderPicture, column.FooterPicture)
	case column.HeaderHorizontalAlign != ItemHorizontalCenter || column.FooterHorizontalAlign != ItemHorizontalRight:
		t.Fatalf("alignment: %q %q", column.HeaderHorizontalAlign, column.FooterHorizontalAlign)
	case column.FooterText["ru"] != "Итого" || column.FooterDataPath != "Объект.Товары.TotalСумма":
		t.Fatalf("footer: %+v %q", column.FooterText, column.FooterDataPath)
	case column.FooterFont.Face != "DefaultGUIFont" || column.FooterTextColor.Name != "Highlight" || column.FooterBackColor.Name != "MediumGray":
		t.Fatalf("footer look: %+v %+v %+v", column.FooterFont, column.FooterTextColor, column.FooterBackColor)
	case form.Items[1].FooterHorizontalAlign != ItemHorizontalLeft || !form.Items[1].AutoCellHeight:
		t.Fatalf("outside a table: %+v", form.Items[1].FieldColumn)
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

// What a field has as a column stands only on a field and is checked as each
// value is checked elsewhere.
//
// Defect caught: a footer kept on a group or a button; a picture naming two
// sources, a footer path that is no path, a colour that is no colour, an
// unknown fixing accepted; two pictures of one element drawing one file, so
// that editing either changes both, or a file that is no image.
func TestAFieldRefusesWhatIsWrongAsAColumn(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"подвал у группы":     {"kind: usual-group, footer_text: {ru: Итого}", "items[0] has what a field has as a column of a table"},
		"гиперссылка кнопки":  {"kind: button, cell_hyperlink: true", "items[0] has what a field has as a column of a table"},
		"две картинки":        {"kind: input-field, header_picture: {standard: Change, common: c0de0000-0000-4000-8000-000000000043}", "items[0].header_picture names more than one"},
		"один файл дважды":    {"kind: input-field, header_picture: {file: HeaderPicture.png}, footer_picture: {file: headerpicture.PNG}", "items[0].footer_picture.file is the file of header_picture too"},
		"своя не картинка":    {"kind: input-field, header_picture: {file: HeaderPicture.txt}", "items[0].header_picture.file must be the name of an image file"},
		"путь подвала":        {"kind: input-field, footer_data_path: Объект..Сумма", "items[0].footer_data_path must be names separated by dots"},
		"цвет подвала":        {"kind: input-field, footer_back_color: {source: absolute, rgb: red}", "items[0].footer_back_color.rgb must be a colour written as #RRGGBB"},
		"шрифт подвала":       {"kind: input-field, footer_font: {source: absolute, size: 10}", "items[0].footer_font.face must name the font"},
		"закрепление":         {"kind: input-field, fixing_in_table: top", "items[0].fixing_in_table must be none, left or right"},
		"положение в шапке":   {"kind: input-field, header_horizontal_align: justify", "items[0].header_horizontal_align must be auto, left, center or right"},
		"положение в подвале": {"kind: input-field, footer_horizontal_align: justify", "items[0].footer_horizontal_align must be auto, left, center or right"},
		"текст подвала":       {"kind: input-field, footer_text: {ru: \"\\x01\"}", "items[0].footer_text.ru must say something in printable characters"},
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

// The common pictures and the style items the header and footer of a column
// are drawn with are resolved with the project, as the look of a field is.
//
// Defect caught: a column drawn with a common picture or a style item the
// project does not have, loading clean; a footer font taken from a colour
// accepted.
func TestWhatAColumnIsDrawnWithIsResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		column  string
		refused string
		gone    string
	}{
		"общая картинка":          {"header_picture: {common: " + cmpCommonPicture + "}", "", ""},
		"удалённая картинка":      {"footer_picture: {common: " + refGone + "}", "", "footer_picture"},
		"удалённый элемент стиля": {"footer_back_color: {source: style, from: {item: " + refGone + "}}", "", "footer_back_color"},
		"цвет подвала из стиля":   {"footer_text_color: {source: style, from: {item: " + cmpStyleItem + "}}", "", ""},
		"шрифт подвала из цвета":  {"footer_font: {source: style, from: {item: " + cmpStyleItem + "}}", "element Поле footer_font takes its value from style item ЦветВажного, which is a color and not a font", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Таблица, kind: table, children: [" +
				"{id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, " + test.column + "}]}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			switch {
			case test.gone != "":
				if found := unresolvedOf(t, root); !containsWhere(found, "catalog Номенклатура form ФормаЭлемента element Поле "+test.gone) {
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

// Each property of a column, set alone on an element that is no field, is
// refused: none of them is forgotten when the column is checked for being
// empty.
//
// Defect caught: a property left out of the check for an empty column, so
// that a button or a group keeps it alone without a word.
func TestEveryPropertyOfAColumnStandsOnlyOnAField(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, hidden_in_header: true, hidden_in_footer: true," +
		" header_picture: {standard: Change}, footer_picture: {standard: Change}, header_horizontal_align: center," +
		" footer_text: {ru: Итого}, footer_data_path: Объект.Сумма, footer_horizontal_align: right," +
		" footer_font: {source: auto}, footer_text_color: {source: auto}, footer_back_color: {source: auto}," +
		" fixing_in_table: left, cell_hyperlink: true, auto_cell_height: true}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Кнопка, kind: button}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	full := reflect.ValueOf(form.Items[0].FieldColumn)
	for index := range full.NumField() {
		name := full.Type().Field(index).Name
		if full.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		alone := form
		alone.Items = slices.Clone(form.Items)
		reflect.ValueOf(&alone.Items[1].FieldColumn).Elem().Field(index).Set(full.Field(index))
		err := ValidateManagedForm("form.yaml", alone, configuration)
		if err == nil || !strings.Contains(err.Error(), "items[1] has what a field has as a column of a table") {
			t.Errorf("%s alone on a button: %v", name, err)
		}
	}
}

// An input field keeps its buttons - each yes, no or not said - where its
// choice button stands and what is drawn on it, and when its clear and open
// buttons show, through YAML and the Studio.
//
// Defect caught: a button turned off in the prototype (the choice button 1317
// times, the clear button 517) read as not said and shown again by the type
// edited; a button read into its neighbour; the picture of the choice button,
// where it stands or when the clear button shows lost on the way.
func TestAnInputFieldKeepsItsButtons(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, choice_button: false, open_button: true," +
		" clear_button: false, create_button: true, drop_list_button: false, spin_button: true, choice_list_button: false," +
		" choice_button_representation: show-in-drop-list-and-in-input-field," +
		" choice_button_picture: {standard: InputFieldCalendar, load_transparent: true}," +
		" auto_show_clear_button: filled-only, auto_show_open_button: always}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Простое, kind: input-field}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	buttons := form.Items[0].FieldButtons
	value := reflect.ValueOf(buttons)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	said := func(button *bool) string {
		if button == nil {
			return "-"
		}
		return strconv.FormatBool(*button)
	}
	got := strings.Join([]string{said(buttons.ChoiceButton), said(buttons.OpenButton), said(buttons.ClearButton), said(buttons.CreateButton),
		said(buttons.DropListButton), said(buttons.SpinButton), said(buttons.ChoiceListButton)}, " ")
	switch {
	case got != "false true false true false true false":
		t.Fatalf("buttons: %s", got)
	case buttons.ChoiceButtonRepresentation != FormChoiceButtonShowInDropListAndInInputField:
		t.Fatalf("representation: %q", buttons.ChoiceButtonRepresentation)
	case buttons.ChoiceButtonPicture.Standard != "InputFieldCalendar" || !buttons.ChoiceButtonPicture.LoadTransparent:
		t.Fatalf("picture: %+v", buttons.ChoiceButtonPicture)
	case buttons.AutoShowClearButton != FormAutoShowButtonFilledOnly || buttons.AutoShowOpenButton != FormAutoShowButtonAlways:
		t.Fatalf("auto show: %q %q", buttons.AutoShowClearButton, buttons.AutoShowOpenButton)
	case !form.Items[1].FieldButtons.empty():
		t.Fatalf("not said: %+v", form.Items[1].FieldButtons)
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

// The buttons of an input field stand only on an input field and are checked
// as each value is checked elsewhere.
//
// Defect caught: a choice button kept on a label field, a check box or a
// button, which have none; an unknown place of the choice button or mode of
// showing accepted; a picture of the choice button naming two sources, a file
// that is no image, or the file of another picture of the element.
func TestAnInputFieldRefusesWrongButtons(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"у поля надписи":      {"kind: label-field, choice_button: true", "items[0] has the buttons of an input field"},
		"у флажка":            {"kind: check-box-field, clear_button: false", "items[0] has the buttons of an input field"},
		"у кнопки":            {"kind: button, auto_show_open_button: always", "items[0] has the buttons of an input field"},
		"отображение":         {"kind: input-field, choice_button_representation: ShowInInputField", "items[0].choice_button_representation must be auto, show-in-input-field, show-in-drop-list or show-in-drop-list-and-in-input-field"},
		"автопоказ очистки":   {"kind: input-field, auto_show_clear_button: never", "items[0].auto_show_clear_button must be auto, always or filled-only"},
		"автопоказ открытия":  {"kind: input-field, auto_show_open_button: never", "items[0].auto_show_open_button must be auto, always or filled-only"},
		"две картинки":        {"kind: input-field, choice_button_picture: {standard: Change, common: c0de0000-0000-4000-8000-000000000043}", "items[0].choice_button_picture names more than one"},
		"своя не картинка":    {"kind: input-field, choice_button_picture: {file: ChoiceButtonPicture.txt}", "items[0].choice_button_picture.file must be the name of an image file"},
		"файл картинки шапки": {"kind: input-field, header_picture: {file: Picture.png}, choice_button_picture: {file: picture.png}", "items[0].choice_button_picture.file is the file of header_picture too"},
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

// Each button property, set alone on a field that is no input field, is
// refused: none of them is forgotten when the buttons are checked for being
// empty.
//
// Defect caught: a property left out of the check for no buttons, so that a
// label field keeps it alone without a word.
func TestEveryButtonStandsOnlyOnAnInputField(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, choice_button: false, open_button: false," +
		" clear_button: false, create_button: false, drop_list_button: false, spin_button: false, choice_list_button: false," +
		" choice_button_representation: auto, choice_button_picture: {standard: Change}," +
		" auto_show_clear_button: auto, auto_show_open_button: auto}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Надпись, kind: label-field}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	full := reflect.ValueOf(form.Items[0].FieldButtons)
	for index := range full.NumField() {
		name := full.Type().Field(index).Name
		if full.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		alone := form
		alone.Items = slices.Clone(form.Items)
		reflect.ValueOf(&alone.Items[1].FieldButtons).Elem().Field(index).Set(full.Field(index))
		err := ValidateManagedForm("form.yaml", alone, configuration)
		if err == nil || !strings.Contains(err.Error(), "items[1] has the buttons of an input field") {
			t.Errorf("%s alone on a label field: %v", name, err)
		}
	}
}

// The picture of the choice button is resolved with the project: a common
// picture must be there, and a file of the element's own must lie in the
// element's folder, as the other pictures of an element are.
//
// Defect caught: a choice button drawn with a common picture the project
// does not have loading clean; a picture of the element's own left out of the
// pictures of the element, so that its file is refused as kept for nothing,
// or a reference to a file the folder does not hold loading clean.
func TestThePictureOfAChoiceButtonIsResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		picture string
		files   []string
		refused string
		gone    string
	}{
		"общая картинка":     {picture: "{common: " + cmpCommonPicture + "}"},
		"удалённая картинка": {picture: "{common: " + refGone + "}", gone: "choice_button_picture"},
		"свой файл":          {picture: "{file: ChoiceButtonPicture.png}", files: []string{"Поле/ChoiceButtonPicture.png"}},
		"своего файла нет":   {picture: "{file: ChoiceButtonPicture.png}", refused: "element Поле choice_button_picture is shown with picture file ChoiceButtonPicture.png, which its folder does not hold"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, choice_button_picture: " + test.picture + "}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			for _, file := range test.files {
				writeFile(t, filepath.Join(filepath.Dir(path), project.FormItemsDirectory, file), "image")
			}
			switch {
			case test.gone != "":
				if found := unresolvedOf(t, root); !containsWhere(found, "catalog Номенклатура form ФормаЭлемента element Поле "+test.gone) {
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

// An input field keeps how text is typed into it through YAML and the
// Studio; the password mode stands on a label field too.
//
// Defect caught: wrapping or editing the text turned off in the prototype
// (6007 and 1600 times) turned on again after the move; a multiline field
// written false read as not said; a mask losing its spaces; the hint, the
// keyboard of a mobile client or the mode of updating the text lost or read
// into a neighbour; the password mode of a label field refused, so that the
// form is not moved.
func TestAnInputFieldKeepsHowTextIsTyped(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, no_wrap: true, no_text_edit: true," +
		" multi_line: false, extended_edit: true, password_mode: false, mask: \"99 99\", input_hint: {ru: Введите код}," +
		" edit_text_update: on-value-change, special_text_input_mode: phone-number, spell_checking: dont-use, auto_correction: use," +
		" height_control_variant: use-content-height}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Пароль, kind: label-field, password_mode: true}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	input := form.Items[0].FieldTextInput
	value := reflect.ValueOf(input)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	switch {
	case !input.NoWrap || !input.NoTextEdit || *input.MultiLine || !*input.ExtendedEdit || *input.PasswordMode:
		t.Fatalf("switches: %+v", input)
	case input.Mask != "99 99" || input.InputHint["ru"] != "Введите код":
		t.Fatalf("mask and hint: %q %+v", input.Mask, input.InputHint)
	case input.EditTextUpdate != FormEditTextUpdateOnValueChange || input.SpecialTextInputMode != FormSpecialTextInputPhoneNumber:
		t.Fatalf("modes: %q %q", input.EditTextUpdate, input.SpecialTextInputMode)
	case input.SpellChecking != FormTextInputUseDontUse || input.AutoCorrection != FormTextInputUseUse:
		t.Fatalf("checking: %q %q", input.SpellChecking, input.AutoCorrection)
	case input.HeightControlVariant != FormHeightControlUseContentHeight:
		t.Fatalf("height: %q", input.HeightControlVariant)
	case form.Items[1].PasswordMode == nil || !*form.Items[1].PasswordMode:
		t.Fatalf("label field: %+v", form.Items[1].FieldTextInput)
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

// How text is typed stands only on an input field - the password mode on a
// label field too - and each value is checked.
//
// Defect caught: a mask kept on a label field or a hint on a check box; an
// unknown mode of updating the text, keyboard, checking or height accepted;
// a hint in no printable characters.
func TestAnInputFieldRefusesWrongTextInput(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"маска поля надписи": {"kind: label-field, password_mode: true, mask: \"999\"", "items[0] has the text input of an input field"},
		"пароль флажка":      {"kind: check-box-field, password_mode: true", "items[0] has the text input of an input field"},
		"подсказка группы":   {"kind: usual-group, input_hint: {ru: Код}", "items[0] has the text input of an input field"},
		"обновление текста":  {"kind: input-field, edit_text_update: OnValueChange", "items[0].edit_text_update must be auto, always, on-value-change or dont-use"},
		"клавиатура":         {"kind: input-field, special_text_input_mode: phone", "items[0].special_text_input_mode must be auto, none, digits, digits-and-punctuation, email, phone-number or url"},
		"орфография":         {"kind: input-field, spell_checking: never", "items[0].spell_checking must be auto, use or dont-use"},
		"автоисправление":    {"kind: input-field, auto_correction: never", "items[0].auto_correction must be auto, use or dont-use"},
		"высота":             {"kind: input-field, height_control_variant: rows", "items[0].height_control_variant must be auto, use-content-height or use-height-in-form-rows"},
		"подсказка без слов": {"kind: input-field, input_hint: {ru: \"\\x01\"}", "items[0].input_hint.ru must say something in printable characters"},
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

// Each property of typing text, set alone on a check box, is refused, and
// alone on a label field each but the password mode: none is forgotten when
// the text input is checked for being empty.
//
// Defect caught: a property left out of the check for no text input, so that
// another field keeps it alone without a word; the password mode refused on a
// label field, or another property let through there with it.
func TestEveryPropertyOfTextInputStandsOnlyOnAnInputField(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, no_wrap: true, no_text_edit: true," +
		" multi_line: false, extended_edit: false, password_mode: false, mask: \"9\", input_hint: {ru: Код}," +
		" edit_text_update: auto, special_text_input_mode: auto, spell_checking: auto, auto_correction: auto," +
		" height_control_variant: auto}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Флажок, kind: check-box-field}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Надпись, kind: label-field}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	full := reflect.ValueOf(form.Items[0].FieldTextInput)
	for index := range full.NumField() {
		name := full.Type().Field(index).Name
		if full.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		for _, on := range []int{1, 2} {
			alone := form
			alone.Items = slices.Clone(form.Items)
			reflect.ValueOf(&alone.Items[on].FieldTextInput).Elem().Field(index).Set(full.Field(index))
			err := ValidateManagedForm("form.yaml", alone, configuration)
			allowed := on == 2 && name == "PasswordMode"
			refused := err != nil && strings.Contains(err.Error(), fmt.Sprintf("items[%d] has the text input of an input field", on))
			if allowed && err != nil || !allowed && !refused {
				t.Errorf("%s alone on %s: %v", name, alone.Items[on].Kind, err)
			}
		}
	}
}

// A field keeps how it shows a value and bounds a number through YAML and the
// Studio, each property on the fields the help gives it.
//
// Defect caught: a format, a bound or a mark lost or read into a neighbour; a
// mark of negatives written false read as not said; a bound rounded on the
// way - a decimal of more digits than a float holds; the format of a label
// field, the format of a check box shown as a tumbler (93 times) or the scale
// of a progress bar refused, so that the form is not moved.
func TestAFieldKeepsItsFormatAndBounds(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Сумма, kind: input-field, format: {ru: ЧДЦ=2, uk: ЧДЦ=2}," +
		" edit_format: {ru: ЧГ=0}, min_value: -12345678901234567890.123456789, max_value: 65535," +
		" mark_negatives: false, auto_mark_incomplete: true}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Дата, kind: label-field, format: {ru: ДЛФ=D}, mark_negatives: true}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Флажок, kind: check-box-field, edit_format: {ru: БЛ=Нет; БИ=Да}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990004, name: Ход, kind: progress-bar-field, min_value: 0, max_value: 100.50}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	format := form.Items[0].FieldFormat
	value := reflect.ValueOf(format)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	switch {
	case format.Format["uk"] != "ЧДЦ=2" || format.EditFormat["ru"] != "ЧГ=0":
		t.Fatalf("formats: %+v %+v", format.Format, format.EditFormat)
	case format.MinValue != "-12345678901234567890.123456789" || format.MaxValue != "65535":
		t.Fatalf("bounds: %q %q", format.MinValue, format.MaxValue)
	case *format.MarkNegatives || !*format.AutoMarkIncomplete:
		t.Fatalf("marks: %v %v", *format.MarkNegatives, *format.AutoMarkIncomplete)
	case form.Items[1].Format["ru"] != "ДЛФ=D" || !*form.Items[1].MarkNegatives:
		t.Fatalf("label field: %+v", form.Items[1].FieldFormat)
	case form.Items[2].EditFormat["ru"] != "БЛ=Нет; БИ=Да":
		t.Fatalf("check box: %+v", form.Items[2].FieldFormat)
	case form.Items[3].MinValue != "0" || form.Items[3].MaxValue != "100.50":
		t.Fatalf("progress bar: %+v", form.Items[3].FieldFormat)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "min_value: -12345678901234567890.123456789\n") || !strings.Contains(string(written), "max_value: 100.50\n") {
		t.Fatalf("a bound is not written as a number:\n%s", written)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil || !reflect.DeepEqual(again.Items, form.Items) {
		t.Fatalf("written back: %v", err)
	}
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(carried), `"minValue":-12345678901234567890.123456789`) {
		t.Fatalf("a bound is not carried as a number: %s", carried)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil || !reflect.DeepEqual(received.Items, form.Items) {
		t.Fatalf("carried through the Studio: %v", err)
	}
}

// A bound is a number: one in quotes, in another notation or not a number at
// all is refused in YAML and in the Studio alike, and a format is checked as
// a text in languages.
//
// Defect caught: a bound kept as a string, as the attribute keeps its own, so
// that "100" and 100 are two different values; an exponent or a hexadecimal
// accepted and read otherwise than written; a format in no printable
// characters.
func TestAFieldRefusesWrongFormatAndBounds(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"строка":        {"min_value: \"100\"", `"100" is not a number written as decimal digits`},
		"степень":       {"max_value: 1e3", `"1e3" is not a number written as decimal digits`},
		"шестнадцатая":  {"max_value: 0x10", `"0x10" is not a number written as decimal digits`},
		"бесконечность": {"min_value: .inf", `".inf" is not a number written as decimal digits`},
		"список":        {"min_value: [1]", "is not a number written as decimal digits"},
		"формат":        {"format: {ru: \"\\x01\"}", "items[0].format.ru must say something in printable characters"},
		"формат ред.":   {"edit_format: {ru: \"\\x01\"}", "items[0].edit_format.ru must say something in printable characters"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Элемент, kind: input-field, " + test.element + "}\n")
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
	for _, body := range []string{`"100"`, `1e3`, `"x"`, `true`} {
		var number FormNumber
		if err := json.Unmarshal([]byte(body), &number); err == nil {
			t.Errorf("the Studio sends %s as a bound and it is accepted as %q", body, number)
		}
	}
	for _, bound := range []FormNumber{"1e3", "true", "1,5"} {
		if _, err := json.Marshal(FieldFormat{MinValue: bound}); err == nil {
			t.Errorf("a bound %q that is no number is sent to the Studio", bound)
		}
	}
	if issues := validateFieldFormat("items[0]", FieldFormat{MinValue: "1e3"}, FormElementInputField, configuration); len(issues) != 1 ||
		issues[0] != "items[0].min_value must be a number written as decimal digits" {
		t.Errorf("a bound set in code to no number: %v", issues)
	}
}

// Each property of the format, set alone on every kind of element, is
// accepted on the fields the help gives it and refused on any other.
//
// Defect caught: a format kept on a check box, a bound on a label field, a
// mark of negatives on a progress bar - or the reverse, a property refused on
// a field the prototype writes it on.
func TestEachPropertyOfAFormatStandsOnItsFields(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	yes := true
	full := FieldFormat{Format: LocalizedText{"ru": "ЧДЦ=2"}, EditFormat: LocalizedText{"ru": "ЧДЦ=2"}, MinValue: "1", MaxValue: "2",
		MarkNegatives: &yes, AutoMarkIncomplete: &yes}
	allowed := map[string][]FormElementKind{
		"Format":             {FormElementInputField, FormElementLabelField},
		"EditFormat":         {FormElementInputField, FormElementCheckBoxField},
		"MinValue":           {FormElementInputField, FormElementTrackBarField, FormElementProgressBarField},
		"MaxValue":           {FormElementInputField, FormElementTrackBarField, FormElementProgressBarField},
		"MarkNegatives":      {FormElementInputField, FormElementLabelField},
		"AutoMarkIncomplete": {FormElementInputField},
	}
	value := reflect.ValueOf(full)
	for index := range value.NumField() {
		name := value.Type().Field(index).Name
		kinds, known := allowed[name]
		if !known || value.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		for kind := range formElementClasses {
			var alone FieldFormat
			reflect.ValueOf(&alone).Elem().Field(index).Set(value.Field(index))
			issues := validateFieldFormat("items[0]", alone, kind, configuration)
			if want := !slices.Contains(kinds, kind); want != (len(issues) == 1 && strings.Contains(issues[0], "is allowed only for")) || !want && len(issues) != 0 {
				t.Errorf("%s alone on %s: %v", name, kind, issues)
			}
		}
	}
}

// An input field keeps how a value of it is picked through YAML and the
// Studio, the identifier of a choice form that is gone among them.
//
// Defect caught: a quick choice turned off in the prototype (210 times) read
// as not said and decided by the type again; the choice form, the size of the
// drop list, the history or the choice of an empty value lost or read into a
// neighbour; the identifier of a deleted choice form refused, so that the 9
// forms of erp carrying one are not moved.
func TestAnInputFieldKeepsHowAValueIsPicked(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, list_choice_mode: true, quick_choice: false," +
		" choice_form: {kind: catalogs, object: c0de0000-0000-4000-8000-000000000006, name: ФормаВыбора}," +
		" choice_list_height: 5, drop_list_width: 40, choice_history_on_input: dont-use," +
		" auto_choice_incomplete: false, incomplete_choice_mode: on-activate}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Остаток, kind: input-field," +
		" choice_form_gone: 34a62767-ac7d-4451-983f-bd0288c5961a}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	choice := form.Items[0].InputFieldChoice
	choice.ChoiceFormGone = form.Items[1].ChoiceFormGone
	value := reflect.ValueOf(choice)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	switch {
	case !choice.ListChoiceMode || *choice.QuickChoice || *choice.AutoChoiceIncomplete:
		t.Fatalf("switches: %+v", choice)
	case choice.ChoiceForm.Name != "ФормаВыбора" || choice.ChoiceForm.Kind != CatalogKind || choice.ChoiceForm.Object == nil:
		t.Fatalf("choice form: %+v", choice.ChoiceForm)
	case choice.ChoiceListHeight != 5 || choice.DropListWidth != 40:
		t.Fatalf("drop list: %d %d", choice.ChoiceListHeight, choice.DropListWidth)
	case choice.ChoiceHistoryOnInput != ChoiceHistoryDontUse || choice.IncompleteChoiceMode != FormIncompleteChoiceOnActivate:
		t.Fatalf("modes: %q %q", choice.ChoiceHistoryOnInput, choice.IncompleteChoiceMode)
	case choice.ChoiceFormGone.String() != "34a62767-ac7d-4451-983f-bd0288c5961a" || form.Items[1].ChoiceForm != nil:
		t.Fatalf("gone: %+v", form.Items[1].InputFieldChoice)
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

// How a value is picked stands only on an input field, and each value is
// checked.
//
// Defect caught: a choice form kept on a label field or a radio button; a
// choice form of no object kind, or one named and gone at once; a negative
// size of the drop list; an unknown history or mode of choosing an empty
// value accepted.
func TestAnInputFieldRefusesWrongChoice(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"у поля надписи":       {"kind: label-field, quick_choice: true", "items[0] has the choice of an input field"},
		"у переключателя":      {"kind: radio-button-field, list_choice_mode: true", "items[0] has the choice of an input field"},
		"форма без имени":      {"kind: input-field, choice_form: {name: \"\"}", "items[0].choice_form.name must be a valid identifier"},
		"форма без вида":       {"kind: input-field, choice_form: {object: c0de0000-0000-4000-8000-000000000006, name: Форма}", "items[0].choice_form.kind must be set beside the object"},
		"форма и удалённая":    {"kind: input-field, choice_form: {name: Форма}, choice_form_gone: c0de0000-0000-4000-8000-000000000006", "items[0].choice_form_gone stands in place of a choice form, not beside one"},
		"удалённая нулевая":    {"kind: input-field, choice_form_gone: 00000000-0000-0000-0000-000000000000", "items[0].choice_form_gone must be a non-zero UUID"},
		"высота списка":        {"kind: input-field, choice_list_height: -1", "items[0].choice_list_height must not be negative"},
		"ширина списка":        {"kind: input-field, drop_list_width: -1", "items[0].drop_list_width must not be negative"},
		"история":              {"kind: input-field, choice_history_on_input: use", "items[0].choice_history_on_input must be auto or dont-use"},
		"режим незаполненного": {"kind: input-field, incomplete_choice_mode: OnActivate", "items[0].incomplete_choice_mode must be on-activate or on-enter-pressed"},
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

// Each property of picking a value, set alone on a label field, is refused:
// none is forgotten when the choice is checked for being empty.
//
// Defect caught: a property left out of the check for no choice, so that
// another field keeps it alone without a word.
func TestEveryPropertyOfChoiceStandsOnlyOnAnInputField(t *testing.T) {
	t.Parallel()
	no := false
	gone := uuid.MustNew()
	full := InputFieldChoice{ListChoiceMode: true, QuickChoice: &no, ChoiceForm: &ChoiceFormReference{Name: "Форма"}, ChoiceFormGone: &gone,
		ChoiceListHeight: 1, DropListWidth: 1, ChoiceHistoryOnInput: ChoiceHistoryAuto, AutoChoiceIncomplete: &no,
		IncompleteChoiceMode: FormIncompleteChoiceOnEnterPressed}
	value := reflect.ValueOf(full)
	for index := range value.NumField() {
		name := value.Type().Field(index).Name
		if value.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		var alone InputFieldChoice
		reflect.ValueOf(&alone).Elem().Field(index).Set(value.Field(index))
		if issues := validateInputFieldChoice("items[0]", alone, FormElementLabelField); len(issues) != 1 || issues[0] != "items[0] has the choice of an input field" {
			t.Errorf("%s alone on a label field: %v", name, issues)
		}
		if issues := validateInputFieldChoice("items[0]", alone, FormElementInputField); len(issues) != 0 {
			t.Errorf("%s alone on an input field: %v", name, issues)
		}
	}
}

// The object whose form picks a value is resolved with the project, and the
// identifier of a choice form that is gone resolves to nothing.
//
// Defect caught: a choice form of an object the project does not have loading
// clean; a deleted choice form carried silently, so that the field opens
// nothing and no one is told.
func TestTheChoiceFormOfAnInputFieldIsResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ choice, gone string }{
		"форма справочника": {"choice_form: {kind: catalogs, object: " + cmpGoods + ", name: ФормаВыбора}", ""},
		"общая форма":       {"choice_form: {name: АдреснаяКнига}", ""},
		"удалённый объект":  {"choice_form: {kind: catalogs, object: " + refGone + ", name: ФормаВыбора}", "choice_form"},
		"удалённая форма":   {"choice_form_gone: " + refGone, "choice_form_gone"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, " + test.choice + "}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			if test.gone == "" {
				if _, err := Load(root); err != nil {
					t.Fatal(err)
				}
				return
			}
			if found := unresolvedOf(t, root); !containsWhere(found, "catalog Номенклатура form ФормаЭлемента element Поле "+test.gone) {
				t.Fatalf("unresolved = %+v", found)
			}
		})
	}
}

// An input field and a radio button field keep their choice lists through
// YAML and the Studio: every kind of value the prototype writes there, its
// presentation in languages, and whether it is checked.
//
// Defect caught: the choice list of a radio button refused, so that 1990 of
// them are not moved; a value of an enumeration losing its object, Неопределено
// read as an empty string, an empty string read as no value; a value of a
// type the platform defines refused; a presentation or a check lost.
func TestAFieldKeepsItsChoiceList(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Вид, kind: input-field, choice_list: [" +
		"{value: {kind: enumeration, data: c0de0000-0000-4000-8000-000000000071, object: c0de0000-0000-4000-8000-000000000070}, presentation: {ru: Ввод, uk: Введення}}," +
		" {value: {kind: string, data: \"\"}, presentation: {ru: Все}}, {value: {kind: undefined, data: \"\"}, check: true}," +
		" {value: {kind: platform, data: \"ent:AccountType.Active\"}}," +
		" {value: {kind: unresolved-reference, data: \"5647c4ea-0d6d-4d75-8261-5aa213fca033.bca5af74-66f6-44f5-8fa6-cee0b448788a\"}}]}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Режим, kind: radio-button-field, choice_list: [" +
		"{value: {kind: number, data: \"0\"}, presentation: {ru: Исполнителю}}, {value: {kind: boolean, data: \"true\"}}]}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	list, radio := form.Items[0].ChoiceList, form.Items[1].ChoiceList
	switch {
	case len(list) != 5 || len(radio) != 2:
		t.Fatalf("lists: %+v %+v", list, radio)
	case list[0].Value.Kind != EnumerationType || list[0].Value.Object.String() != "c0de0000-0000-4000-8000-000000000070" || list[0].Presentation["uk"] != "Введення":
		t.Fatalf("an enumeration value: %+v", list[0])
	case list[1].Value != (Value{Kind: StringType}) || list[2].Value != (Value{Kind: UndefinedValue}) || list[1].Check || !list[2].Check:
		t.Fatalf("an empty string and Неопределено: %+v %+v", list[1], list[2])
	case list[3].Value != (Value{Kind: PlatformType, Data: "ent:AccountType.Active"}) || list[4].Value.Kind != UnresolvedReferenceValue:
		t.Fatalf("a platform value and a remnant: %+v %+v", list[3], list[4])
	case radio[0].Value != (Value{Kind: NumberType, Data: "0"}) || radio[0].Presentation["ru"] != "Исполнителю":
		t.Fatalf("radio: %+v", radio)
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

// A choice list stands on an input and a radio button field only, and each
// of its values is checked as a value written at design time is.
//
// Defect caught: a choice list kept on a label field or a check box; a value
// that does not say its kind, read as an empty one; a value of a type the
// platform defines without its type, with spaces or with an object; a
// presentation in no printable characters.
func TestAFieldRefusesAWrongChoiceList(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"у поля надписи": {"kind: label-field, choice_list: [{value: {kind: string, data: a}}]", "items[0].choice_list is allowed only for input and radio button fields"},
		"у флажка":       {"kind: check-box-field, choice_list: [{value: {kind: string, data: a}}]", "items[0].choice_list is allowed only for input and radio button fields"},
		"вид не назван":  {"kind: input-field, choice_list: [{value: {kind: \"\", data: \"\"}}]", "items[0].choice_list[0].value.kind must be named"},
		"без типа":       {"kind: input-field, choice_list: [{value: {kind: platform, data: Active}}]", "items[0].choice_list[0].value.data must be the type and the value"},
		"без значения":   {"kind: input-field, choice_list: [{value: {kind: platform, data: ent:AccountType.}}]", "items[0].choice_list[0].value.data must be the type and the value"},
		"с пробелом":     {"kind: input-field, choice_list: [{value: {kind: platform, data: ent:Account Type.Active}}]", "items[0].choice_list[0].value.data must be the type and the value"},
		"с объектом":     {"kind: input-field, choice_list: [{value: {kind: platform, data: ent:AccountType.Active, object: c0de0000-0000-4000-8000-000000000070}}]", "items[0].choice_list[0].value.data must be the type and the value"},
		"представление":  {"kind: radio-button-field, choice_list: [{value: {kind: number, data: \"1\"}, presentation: {ru: \"\\x01\"}}]", "items[0].choice_list[0].presentation.ru must say something in printable characters"},
		"второй элемент": {"kind: input-field, choice_list: [{value: {kind: string, data: a}}, {value: {kind: undefined, data: x}}]", "items[0].choice_list[1].value is undefined and carries nothing"},
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

// A value of a choice list naming a type the project no longer has is a
// remnant, resolved to nothing; one of a type the platform defines is a note;
// an ordinary one is neither.
//
// Defect caught: a remnant in a choice list loading clean, so that the field
// offers a value of nothing and no one is told; a value of a type the
// platform defines carried silently, though nothing executes it; a note on an
// ordinary value, burying the real ones.
func TestTheValuesOfAChoiceListAreResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		value       string
		unresolved  bool
		noteWritten string
	}{
		"строка":        {value: "{kind: string, data: Все}"},
		"остаток":       {value: "{kind: unresolved-reference, data: \"5647c4ea-0d6d-4d75-8261-5aa213fca033.bca5af74-66f6-44f5-8fa6-cee0b448788a\"}", unresolved: true},
		"вид сравнения": {value: "{kind: platform, data: \"dcsset:DataCompositionComparisonType.Equal\"}", noteWritten: "dcsset:DataCompositionComparisonType.Equal"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, choice_list: [{value: " + test.value + "}]}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента element Поле choice list"
			if test.unresolved {
				if found := unresolvedOf(t, root); !containsWhere(found, where) {
					t.Fatalf("unresolved = %+v", found)
				}
				return
			}
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			var noted []Note
			for _, note := range catalog.Notes() {
				if note.Kind == NotePlatformValueByName {
					noted = append(noted, note)
				}
			}
			switch {
			case test.noteWritten == "" && len(noted) != 0:
				t.Fatalf("an ordinary value is noted: %+v", noted)
			case test.noteWritten != "" && (len(noted) != 1 || noted[0].Where != where || noted[0].Written != test.noteWritten):
				t.Fatalf("notes = %+v", noted)
			}
		})
	}
}

// An input field keeps what narrows its choice through YAML and the Studio:
// fixed choice parameters - one value or a list - links to the data of the
// form in every writing the prototype uses, a link by type, the types on offer
// and what a choice may land on.
//
// Defect caught: a parameter set to a list read as one value, so that the
// filter is by equality instead of membership; a link written by a number or
// by the code of an element refused - one that starts with a plain number
// among them, "1/0:…" - so that 67 links are not moved; the
// change of a linked value, the element of the type taken, a type on offer or
// the choice of folders lost; asking for the type turned off in the prototype
// (1342 times) turned on again.
func TestAnInputFieldKeepsItsChoiceParameters(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Номенклатура, kind: input-field, choice_parameters: [" +
		"{name: Отбор.Тип, list: true, values: [{kind: enumeration, data: c0de0000-0000-4000-8000-000000000071, object: c0de0000-0000-4000-8000-000000000070}," +
		" {kind: enumeration, data: c0de0000-0000-4000-8000-000000000072, object: c0de0000-0000-4000-8000-000000000070}]}," +
		" {name: Отбор.Клиент, values: [{kind: boolean, data: \"true\"}]}]," +
		" choice_parameter_links: [{name: Отбор.Владелец, data_path: Объект.Партнер, value_change: dont-change}," +
		" {name: Отбор.Вид, data_path: Items.Список.CurrentData.Вид}, {name: Отбор.Номер, data_path: \"2\"}," +
		" {name: Отбор.Код, data_path: \"48:02023637-7868-4a5f-8576-835a76e0c9ba/0:3c1e525b-09ed-4189-b279-da594cf572f5\"}," +
		" {name: Отбор.Колонка, data_path: \"1/0:ba7dcb3b-b8b9-4d44-ab5f-56f7a228f10d\"}]," +
		" type_link: {data_path: \"342:02023637-7868-4a5f-8576-835a76e0c9ba/15\", link_item: 2}, no_choose_type: true, no_type_domain: true," +
		" available_types: [{kind: string}], choice_folders_and_items: folders-and-items}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	parameters := form.Items[0].InputFieldChoiceParameters
	value := reflect.ValueOf(parameters)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	links := parameters.ChoiceParameterLinks
	switch {
	case len(parameters.ChoiceParameters) != 2 || !parameters.ChoiceParameters[0].List || len(parameters.ChoiceParameters[0].Values) != 2 || parameters.ChoiceParameters[1].List:
		t.Fatalf("parameters: %+v", parameters.ChoiceParameters)
	case len(links) != 5 || links[0].DataPath != "Объект.Партнер" || links[0].ValueChange != ValueChangeDontChange || links[1].ValueChange != "":
		t.Fatalf("links: %+v", links)
	case links[2].DataPath != "2" || !strings.HasPrefix(links[3].DataPath, "48:02023637") || !strings.HasPrefix(links[4].DataPath, "1/0:ba7dcb3b"):
		t.Fatalf("links by number and by code: %+v", links)
	case parameters.TypeLink.LinkItem != 2 || !strings.HasSuffix(parameters.TypeLink.DataPath, "/15"):
		t.Fatalf("type link: %+v", parameters.TypeLink)
	case !parameters.NoChooseType || !parameters.NoTypeDomain || parameters.AvailableTypes[0].Kind != StringType || parameters.ChoiceFoldersAndItems != ChoiceTargetFoldersAndItems:
		t.Fatalf("types: %+v", parameters)
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

// What narrows the choice stands only on an input field, and each part is
// checked as on an attribute.
//
// Defect caught: choice parameters kept on a label field; a parameter with
// no value, several values not said to be a list, a name used twice in one
// list; a link with no path or a path that is none of the writings of the
// prototype, an unknown change; a negative element of a type link; a type
// that is no type on offer; an unknown target of a choice.
func TestAnInputFieldRefusesWrongChoiceParameters(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ element, want string }{
		"у поля надписи":    {"kind: label-field, no_choose_type: true", "items[0] has the choice parameters of an input field"},
		"без значения":      {"kind: input-field, choice_parameters: [{name: Отбор.Вид, values: []}]", "items[0].choice_parameters[0].values must contain at least one value"},
		"не список":         {"kind: input-field, choice_parameters: [{name: Отбор.Вид, values: [{kind: number, data: \"1\"}, {kind: number, data: \"2\"}]}]", "items[0].choice_parameters[0] sets several values, so it must say it is a list"},
		"имя дважды":        {"kind: input-field, choice_parameters: [{name: Отбор.Вид, values: [{kind: number, data: \"1\"}]}, {name: отбор.вид, values: [{kind: number, data: \"2\"}]}]", "items[0].choice_parameters[1].name is used twice in one list"},
		"значение без вида": {"kind: input-field, choice_parameters: [{name: Отбор.Вид, values: [{kind: \"\", data: \"\"}]}]", "items[0].choice_parameters[0].values[0].kind must be named"},
		"связь без пути":    {"kind: input-field, choice_parameter_links: [{name: Отбор.Вид, data_path: \"\"}]", "items[0].choice_parameter_links[0].data_path must be a data path without surrounding spaces"},
		"кривой путь":       {"kind: input-field, choice_parameter_links: [{name: Отбор.Вид, data_path: \"48:Объект\"}]", "items[0].choice_parameter_links[0].data_path must be a data path, a number or the code of an element of a form"},
		"кривой код":        {"kind: input-field, type_link: {data_path: \"48:02023637-7868-4a5f-8576-835a76e0c9ba/x\"}", "items[0].type_link.data_path must be a data path, a number or the code"},
		"код без номера":    {"kind: input-field, type_link: {data_path: \"1/:ba7dcb3b-b8b9-4d44-ab5f-56f7a228f10d\"}", "items[0].type_link.data_path must be a data path, a number or the code"},
		"изменение":         {"kind: input-field, choice_parameter_links: [{name: Отбор.Вид, data_path: Вид, value_change: Clear}]", "items[0].choice_parameter_links[0].value_change must be clear or dont-change"},
		"связь дважды":      {"kind: input-field, choice_parameter_links: [{name: Отбор.Вид, data_path: Вид}, {name: Отбор.Вид, data_path: Тип}]", "items[0].choice_parameter_links[1].name is used twice in one list"},
		"элемент типа":      {"kind: input-field, type_link: {data_path: Вид, link_item: -1}", "items[0].type_link.link_item must not be negative"},
		"доступный тип":     {"kind: input-field, available_types: [{kind: nothing}]", "items[0].available_types"},
		"группы и элементы": {"kind: input-field, choice_folders_and_items: all", "items[0].choice_folders_and_items must be items, folders or folders-and-items"},
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

// Each part of narrowing the choice, set alone on a label field, is refused:
// none is forgotten when they are checked for being empty.
//
// Defect caught: a part left out of the check for no choice parameters, so
// that another field keeps it alone without a word.
func TestEveryChoiceParameterStandsOnlyOnAnInputField(t *testing.T) {
	t.Parallel()
	full := InputFieldChoiceParameters{
		ChoiceParameters:      []ChoiceParameter{{Name: "Отбор.Вид", Values: []Value{{Kind: NumberType, Data: "1"}}}},
		ChoiceParameterLinks:  []FormChoiceParameterLink{{Name: "Отбор.Вид", DataPath: "Вид"}},
		TypeLink:              &FormTypeLink{DataPath: "Вид"},
		NoChooseType:          true,
		NoTypeDomain:          true,
		AvailableTypes:        []Type{{Kind: StringType}},
		ChoiceFoldersAndItems: ChoiceTargetItems,
	}
	value := reflect.ValueOf(full)
	for index := range value.NumField() {
		name := value.Type().Field(index).Name
		if value.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		var alone InputFieldChoiceParameters
		reflect.ValueOf(&alone).Elem().Field(index).Set(value.Field(index))
		if issues := validateInputFieldChoiceParameters("items[0]", alone, FormElementLabelField); len(issues) != 1 || issues[0] != "items[0] has the choice parameters of an input field" {
			t.Errorf("%s alone on a label field: %v", name, issues)
		}
		if issues := validateInputFieldChoiceParameters("items[0]", alone, FormElementInputField); len(issues) != 0 {
			t.Errorf("%s alone on an input field: %v", name, issues)
		}
	}
}

// What narrows the choice is resolved with the project as on an attribute: a
// value naming a type that is gone is a remnant, a value of a type the
// platform defines and a name both fixed and linked are notes, and a type on
// offer must be a type the project has.
//
// Defect caught: a remnant in a choice parameter of a form loading clean; a
// value of the platform or a parameter both fixed and linked carried silently
// in a form (erp has one), though each is a note on an attribute; a type on
// offer naming an object that is gone accepted.
func TestTheChoiceParametersOfAnInputFieldAreResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		parameters string
		unresolved string
		note       NoteKind
		refused    string
	}{
		"обычные": {parameters: "choice_parameters: [{name: Отбор.Вид, values: [{kind: number, data: \"1\"}]}], available_types: [{kind: catalog, reference: " + cmpGoods + "}]"},
		"остаток": {parameters: "choice_parameters: [{name: Отбор.Вид, values: [{kind: unresolved-reference, data: \"5647c4ea-0d6d-4d75-8261-5aa213fca033.bca5af74-66f6-44f5-8fa6-cee0b448788a\"}]}]",
			unresolved: "choice parameter Отбор.Вид"},
		"значение платформы": {parameters: "choice_parameters: [{name: Отбор.ВидСчета, values: [{kind: platform, data: \"ent:AccountType.Active\"}]}]", note: NotePlatformValueByName},
		"задан и связан": {parameters: "choice_parameters: [{name: отбор.владелец, values: [{kind: number, data: \"1\"}]}]," +
			" choice_parameter_links: [{name: Отбор.Владелец, data_path: Объект.Партнер}]", note: NoteChoiceSetAndLinked},
		"удалённый тип": {parameters: "available_types: [{kind: catalog, reference: " + refGone + "}]", refused: "available types"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			element := "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Поле, kind: input-field, " + test.parameters + "}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", element+"attributes:\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента element Поле"
			switch {
			case test.unresolved != "":
				if found := unresolvedOf(t, root); !containsWhere(found, where+" "+test.unresolved) {
					t.Fatalf("unresolved = %+v", found)
				}
				return
			case test.refused != "":
				if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("err = %v, want %q", err, test.refused)
				}
				return
			}
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			var noted []Note
			for _, note := range catalog.Notes() {
				if strings.HasPrefix(note.Where, where) {
					noted = append(noted, note)
				}
			}
			switch {
			case test.note == "" && len(noted) != 0:
				t.Fatalf("an ordinary field is noted: %+v", noted)
			case test.note != "" && (len(noted) != 1 || noted[0].Kind != test.note):
				t.Fatalf("notes = %+v", noted)
			}
		})
	}
}
