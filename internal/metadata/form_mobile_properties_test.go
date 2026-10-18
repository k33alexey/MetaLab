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
