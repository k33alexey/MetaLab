package metadata

import (
	"strings"
	"testing"
)

// What the help gives form elements for the mobile client alone is not
// carried (METADATA-OBJECTS, «Мобильное приложение»; the owner's decision of
// 14.10.2026): the keyboard offered, spelling checked, errors corrected, the
// case of letters changed and the key ending the input of an input field;
// how a table is asked to refresh and what it does on a narrow screen; the
// use of the current row by a table, a usual group, pages and a command, and
// the table whose row a group, pages or a command use (the type of the use is
// what an element does on a mobile device, the table is used with it alone).
// A description naming any of them is refused by the strict reading. The
// names are written here, from the help, not read from the code. The hint of
// autofill, which the help gives the web client too, stays.
//
// Defect caught: a property of a client ML does not have taken and carried;
// one ML keeps refused with them.
func TestMobileOnlyPropertiesOfElementsAreRefused(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for _, test := range []struct{ kind, property string }{
		{"input-field", "special_text_input_mode: phone-number"},
		{"input-field", "spell_checking: dont-use"},
		{"input-field", "auto_correction: use"},
		{"input-field", "auto_capitalization: none"},
		{"input-field", "return_key_text: send"},
		{"table", "refresh_request: pull-from-top"},
		{"table", "behavior_on_horizontal_compression: auto"},
		{"table", "current_row_use: choice"},
		{"usual-group", "current_row_use: use"},
		{"pages", "current_row_use: dont-use"},
		{"usual-group", "associated_table: Список"},
		{"pages", "associated_table: Список"},
	} {
		name := strings.SplitN(test.property, ":", 2)[0]
		source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Элемент, kind: " + test.kind + ", " + test.property + "}\n")
		if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err == nil || !strings.Contains(err.Error(), "field "+name+" not found") {
			t.Errorf("%s of %s: %v", name, test.kind, err)
		}
	}
	for _, property := range []string{"current_row_use: use", "associated_table: Список"} {
		name := strings.SplitN(property, ":", 2)[0]
		source := formAttrHead + "commands:\n" +
			"  - {id: c0de0000-0000-4000-8000-000000990010, name: Удалить, title: {ru: Удалить}, action: custom, " + property + "}\n" +
			"items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Список, kind: table}\n"
		if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err == nil || !strings.Contains(err.Error(), "field "+name+" not found") {
			t.Errorf("%s of a command: %v", name, err)
		}
	}
	kept := formAttrHead + "commands:\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990010, name: Удалить, title: {ru: Удалить}, action: custom}\n" +
		"items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, auto_fill_hint: email}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Список, kind: table}\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(kept), configuration); err != nil {
		t.Errorf("the hint of autofill: %v", err)
	}
}

// The events the help raises for the mobile client alone are no events of a
// form or of a table in ML: the change of the availability of the main
// server and the reopening from another server, before and on it, on a form
// (the mobile client and the autonomous server); the processing of a request
// to refresh, on a table (the client of the mobile application). The names
// are written here, from the help, not read from the code. The reopening of
// a form and the activation of a row, which every client raises, stay.
//
// Defect caught: a handler of an event no client of ML raises taken and
// carried; an event ML raises refused with them.
func TestMobileOnlyEventsAreRefused(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for _, event := range []string{"on-main-server-availability-change", "before-reopen-from-other-server", "on-reopen-from-other-server"} {
		source := formAttrHead + "events: {" + event + ": Обработчик}\n"
		if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err == nil ||
			!strings.Contains(err.Error(), "form.events."+event+" is not an event of a form") {
			t.Errorf("%s of a form: %v", event, err)
		}
	}
	table := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, events: {refresh-request-processing: Обработчик}}\n")
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(table), configuration); err == nil ||
		!strings.Contains(err.Error(), "events.refresh-request-processing is not an event of a table") {
		t.Errorf("refresh-request-processing of a table: %v", err)
	}
	kept := formAttrHead + "events: {on-reopen: ПриПовторномОткрытии}\n" +
		"items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Список, kind: table, events: {on-activate-row: ПриАктивизацииСтроки}}\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(kept), configuration); err != nil {
		t.Errorf("the reopening of a form and the activation of a row: %v", err)
	}
}

// A form has no purpose (owner, 15.10.2026, METADATA-OBJECTS «Только
// управляемое приложение»): with no mobile platform, an application of the
// platform is the one value left and says nothing - every form is offered in
// ML's one client, and no export has a form for the mobile platform alone. A
// description naming one is refused by the strict reading.
//
// Defect caught: the purpose of a form taken and carried, as it was until
// 2.360.
func TestAFormHasNoPurpose(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for _, purposes := range []string{"[platform-application]", "[platform-application, mobile-platform-application]"} {
		if _, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+"purposes: "+purposes+"\n"), configuration); err == nil ||
			!strings.Contains(err.Error(), "field purposes not found") {
			t.Errorf("purposes %s: %v", purposes, err)
		}
	}
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead), configuration); err != nil {
		t.Errorf("a form without a purpose: %v", err)
	}
}

// The importance of an element on a narrow screen, the collapse of the items
// of a form by it and what an element or a command does while the main server
// is out of reach are not carried (2.371; METADATA-OBJECTS, «Мобильное
// приложение»): the help gives the importance «for the screen of a mobile
// device», on every element and on the command bar, the collapse to the mobile
// client, and the behaviour without the main server to the mobile client and
// the mobile application alone - the thin and the web client are named for
// none. A description naming one is refused by the strict reading, whichever
// element, command bar, form or command it is written on. The names are
// written here, from the help, not read from the code.
//
// Defect caught: a property of the mobile client taken and carried - on 1582
// elements, five command bars and every command that writes it.
func TestImportanceAndTheMainServerAreRefused(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for _, test := range []struct{ kind, property string }{
		{"input-field", "display_importance: very-low"},
		{"usual-group", "display_importance: high"},
		{"page", "display_importance: low"},
		{"table", "display_importance: very-high"},
		{"button", "display_importance: usual"},
		{"label-decoration", "display_importance: auto"},
		{"check-box-field", "on_main_server_unavailable: dont-change-behavior"},
		{"table", "on_main_server_unavailable: make-disable"},
		{"label-decoration", "on_main_server_unavailable: auto"},
		{"button", "on_main_server_unavailable: make-disable"},
	} {
		name := strings.SplitN(test.property, ":", 2)[0]
		source := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Элемент, kind: " + test.kind + ", " + test.property + "}\n")
		if test.kind == "page" {
			source = formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990002, name: Страницы, kind: pages, children: [" +
				"{id: c0de0000-0000-4000-8000-000000990001, name: Элемент, kind: page, " + test.property + "}]}\n")
		}
		if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err == nil || !strings.Contains(err.Error(), "field "+name+" not found") {
			t.Errorf("%s of %s: %v", name, test.kind, err)
		}
	}
	bar := formAttrHead + "auto_command_bar: {id: c0de0000-0000-4000-8000-000000990014, display_importance: very-low}\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(bar), configuration); err == nil || !strings.Contains(err.Error(), "field display_importance not found") {
		t.Errorf("display_importance of the command bar of the form: %v", err)
	}
	collapse := formAttrHead + "collapse_by_importance: use\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(collapse), configuration); err == nil || !strings.Contains(err.Error(), "field collapse_by_importance not found") {
		t.Errorf("collapse_by_importance of a form: %v", err)
	}
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+commandCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
commands:
  - {id: `+commandID+`, name: Открыть, title: {ru: Открыть}, on_server_unavailable: not-available}
`), metadataConfiguration()); err == nil || !strings.Contains(err.Error(), "field on_server_unavailable not found") {
		t.Errorf("on_server_unavailable of a command of an object: %v", err)
	}
	if _, err := DecodeCommonCommand("object.yaml", strings.NewReader(`format: 1
id: `+commonCommandID+`
name: Открыть
title: {ru: Открыть}
group: actions-panel-tools
on_server_unavailable: available
`), metadataConfiguration()); err == nil || !strings.Contains(err.Error(), "field on_server_unavailable not found") {
		t.Errorf("on_server_unavailable of a common command: %v", err)
	}
	kept := formAttrHead + "scale_variant: compact\nauto_command_bar: {id: c0de0000-0000-4000-8000-000000990014, horizontal_align: center}\n" +
		"items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: input-field, user_visible: {common: false}}\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(kept), configuration); err != nil {
		t.Errorf("the variant of the scale, the alignment of the bar and the visibility of a field: %v", err)
	}
}
