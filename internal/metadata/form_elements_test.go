package metadata

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
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
		"путь к данным у декорации": {"  - {id: c0de0000-0000-4000-8000-000000990001, name: Надпись, kind: label-decoration, data_path: Объект}\n", "items[0].data_path is allowed only for fields and tables"},
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
