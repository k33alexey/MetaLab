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

// A form and its elements keep the events their procedures handle through
// YAML and the Studio: the events of the form, of a field, a table, a label,
// pages and an extended tooltip, and an event written by its identifier.
//
// Defect caught: the 85828 events of the exports lost, so that no handler of
// a moved form is ever called - the creation of the form on the server
// (10076), the change of a field (27372), the choice of a row of a table;
// the handler of one event read into another; an event written by its
// identifier refused, so that the 64 forms carrying one are not moved.
func TestAFormAndItsElementsKeepTheirEvents(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "events: {on-create-at-server: ПриСозданииНаСервере, on-open: ПриОткрытии, 390d5e4b-e732-4c88-8748-9e211a416984: ПриЧтенииНаСервере}\n" +
		"items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, events: {on-change: ПолеПриИзменении, start-choice: ПолеНачалоВыбора}," +
		" extended_tooltip: {id: c0de0000-0000-4000-8000-000000990002, name: ПолеРасширеннаяПодсказка, kind: label-decoration, events: {url-processing: ПодсказкаОбработкаНавигационнойСсылки}}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990003, name: Список, kind: table, events: {selection: СписокВыбор, on-activate-row: СписокПриАктивизацииСтроки," +
		" on-get-data-at-server: СписокПриПолученииДанныхНаСервере}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990004, name: Страницы, kind: pages, events: {on-current-page-change: СтраницыПриСменеСтраницы}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990005, name: Надпись, kind: label-decoration, events: {click: НадписьНажатие}}\n"
	source := formAttrHead + items
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, form ManagedForm) {
		t.Helper()
		switch {
		case form.Events["on-create-at-server"] != "ПриСозданииНаСервере" || form.Events["on-open"] != "ПриОткрытии" ||
			form.Events["390d5e4b-e732-4c88-8748-9e211a416984"] != "ПриЧтенииНаСервере" || len(form.Events) != 3:
			t.Fatalf("%s: the events of the form: %v", source, form.Events)
		case form.Items[0].Events["on-change"] != "ПолеПриИзменении" || form.Items[0].Events["start-choice"] != "ПолеНачалоВыбора":
			t.Fatalf("%s: the events of a field: %v", source, form.Items[0].Events)
		case form.Items[0].ExtendedTooltip.Events["url-processing"] != "ПодсказкаОбработкаНавигационнойСсылки":
			t.Fatalf("%s: the events of a tooltip: %v", source, form.Items[0].ExtendedTooltip.Events)
		case len(form.Items[1].Events) != 3 || form.Items[1].Events["on-get-data-at-server"] != "СписокПриПолученииДанныхНаСервере":
			t.Fatalf("%s: the events of a table: %v", source, form.Items[1].Events)
		case form.Items[2].Events["on-current-page-change"] != "СтраницыПриСменеСтраницы" || form.Items[3].Events["click"] != "НадписьНажатие":
			t.Fatalf("%s: the events of pages and a label: %v %v", source, form.Items[2].Events, form.Items[3].Events)
		}
	}
	check("read", form)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again)
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
	check("carried through the Studio", received)
	if !reflect.DeepEqual(received.Items, form.Items) || !reflect.DeepEqual(received.Events, form.Events) {
		t.Fatal("the events changed on the way")
	}
}

// Each kind of element raises the events the help gives it and no other:
// written out here from the help, apart from the lists the check reads.
//
// Defect caught: an event of a table let through on an input field, the
// change of a field on a group or a button, which raise none; an event a
// kind raises refused on it - the choice of an input field, the change of a
// check box, the drag of a picture, the user settings of a dynamic list.
func TestEachKindOfElementRaisesTheEventsTheHelpGivesIt(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		kind      FormElementKind
		raised    []FormEvent
		notRaised []FormEvent
	}{
		"поле ввода":         {FormElementInputField, []FormEvent{"on-change", "start-choice", "choice-processing", "auto-complete", "tuning", "text-edit-end"}, []FormEvent{"selection", "click", "on-activate-row"}},
		"поле надписи":       {FormElementLabelField, []FormEvent{"on-change", "click", "url-processing"}, []FormEvent{"start-choice"}},
		"флажок":             {FormElementCheckBoxField, []FormEvent{"on-change"}, []FormEvent{"click", "start-choice"}},
		"переключатель":      {FormElementRadioButtonField, []FormEvent{"on-change"}, []FormEvent{"click"}},
		"поле картинки":      {FormElementPictureField, []FormEvent{"on-change", "click", "drag-start"}, []FormEvent{"url-processing"}},
		"табличный документ": {FormElementSpreadsheetDocumentField, []FormEvent{"selection", "detail-processing", "additional-detail-processing", "on-change-area-content", "before-write"}, []FormEvent{"click"}},
		"таблица":            {FormElementTable, []FormEvent{"selection", "on-activate-row", "before-add-row", "on-get-data-at-server", "before-load-user-settings-at-server"}, []FormEvent{"on-open", "click"}},
		"надпись":            {FormElementLabelDecoration, []FormEvent{"click", "url-processing"}, []FormEvent{"on-change", "drag"}},
		"картинка":           {FormElementPictureDecoration, []FormEvent{"click", "drag", "drag-check"}, []FormEvent{"url-processing"}},
		"страницы":           {FormElementPages, []FormEvent{"on-current-page-change"}, []FormEvent{"on-change"}},
		"обычная группа":     {FormElementUsualGroup, nil, []FormEvent{"on-change", "click"}},
		"кнопка":             {FormElementButton, nil, []FormEvent{"click", "on-change"}},
		"строка поиска":      {FormElementSearchStringAddition, nil, []FormEvent{"on-change"}},
	} {
		for _, event := range test.raised {
			if issues := validateFormEvents("items[0]", map[FormEvent]string{event: "Обработчик"}, formElementEvents[test.kind], "a "+string(test.kind)); len(issues) != 0 {
				t.Errorf("%s: %s refused: %v", name, event, issues)
			}
		}
		for _, event := range test.notRaised {
			issues := validateFormEvents("items[0]", map[FormEvent]string{event: "Обработчик"}, formElementEvents[test.kind], "a "+string(test.kind))
			if len(issues) != 1 || issues[0] != "items[0].events."+string(event)+" is not an event of a "+string(test.kind) {
				t.Errorf("%s: %s: %v", name, event, issues)
			}
		}
	}
}

// What is wrong in an event is refused, naming the place.
//
// Defect caught: the prototype's spelling of an event, or an identifier
// written in capitals or empty, taken as an event; a handler that is no name
// of a procedure; an event of an element on the form.
func TestAnEventRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ events, want string }{
		"написание прототипа":      {"events: {OnOpen: ПриОткрытии}", "form.events.OnOpen is not an event of a form"},
		"событие элемента":         {"events: {on-change: ПриИзменении}", "form.events.on-change is not an event of a form"},
		"идентификатор заглавными": {"events: {390D5E4B-E732-4C88-8748-9E211A416984: ПриЧтении}", "form.events.390D5E4B-E732-4C88-8748-9E211A416984 is not an event of a form"},
		"нулевой идентификатор":    {"events: {00000000-0000-0000-0000-000000000000: ПриЧтении}", "is not an event of a form"},
		"обработчик с пробелом":    {"events: {on-open: При Открытии}", "form.events.on-open must name a procedure of the module of the form"},
		"обработчика нет":          {"events: {on-open: \"\"}", "form.events.on-open must name a procedure of the module of the form"},
		"у элемента":               {"items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, events: {OnChange: ПриИзменении}}", "items[0].events.OnChange is not an event of a input-field"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+test.events+"\n"), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// An event of a form its main attribute does not raise, and an event written
// by its identifier, are noted at load: their handlers are never called. An
// event the form raises by its main attribute is not.
//
// Defect caught: a handler of writing on the server on a form with no main
// attribute (14 in the exports) or an event by identifier (64) carried
// silently, as if it were called - one on an element nested in a group too;
// an attribute of a catalog that is not the main one taken as making the
// form raise the writing; a note on every form of a catalog that
// handles its own writing, burying the real ones.
func TestEventsAFormDoesNotRaiseAreNoted(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		events  string
		element string
		want    []Note
	}{
		"событие справочника": {events: "events: {before-write-at-server: ПередЗаписьюНаСервере, on-open: ПриОткрытии}\n"},
		"не вызывается": {events: "events: {before-write-at-server: ПередЗаписьюНаСервере}\n", want: []Note{{Kind: NoteFormEventNotRaised,
			Where: "common form Обработка event before-write-at-server", Written: "ПередЗаписьюНаСервере"}}},
		"реквизит не основной": {events: "events: {before-write-at-server: ПередЗаписьюНаСервере}\n" +
			"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990098, name: Товар, types: [{kind: catalog-object, reference: " + cmpGoods + "}]}\n",
			want: []Note{{Kind: NoteFormEventNotRaised, Where: "common form Обработка event before-write-at-server", Written: "ПередЗаписьюНаСервере"}}},
		"по идентификатору": {events: "events: {390d5e4b-e732-4c88-8748-9e211a416984: ПриЧтенииНаСервере}\n", want: []Note{{Kind: NoteEventByIdentifier,
			Where: "common form Обработка event 390d5e4b-e732-4c88-8748-9e211a416984", Written: "ПриЧтенииНаСервере"}}},
		"элемент по идентификатору": {element: "items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Группа, kind: usual-group, children: [" +
			"{id: c0de0000-0000-4000-8000-000000990001, name: Отчет, kind: spreadsheet-document-field," +
			" events: {b7646583-04d3-4905-8f04-8985914bd1b7: ОтчетПередЗаписью}}]}\n", want: []Note{{Kind: NoteEventByIdentifier,
			Where: "common form Обработка element Отчет event b7646583-04d3-4905-8f04-8985914bd1b7", Written: "ОтчетПередЗаписью"}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			// A catalog object form raises the writing on the server; the same
			// event on a common form with no main attribute is never raised.
			if name == "событие справочника" {
				path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, strings.Replace(string(content), "attributes:\n", test.events+"attributes:\n", 1))
			} else {
				writeCommonForm(t, root, "Обработка", "format: 1\nid: c0de0000-0000-4000-8000-000000990099\nname: Обработка\ntitle: {ru: Обработка}\nkind: common\n"+
					test.events+test.element)
			}
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			var found []Note
			for _, note := range catalog.Notes() {
				if note.Kind == NoteEventByIdentifier || note.Kind == NoteFormEventNotRaised {
					found = append(found, note)
				}
			}
			if !reflect.DeepEqual(found, test.want) {
				t.Fatalf("notes = %+v, want %+v", found, test.want)
			}
		})
	}
}
