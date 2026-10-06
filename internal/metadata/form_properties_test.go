package metadata

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// formWindowWhole sets every property of the window of a form away from its
// default.
const formWindowWhole = formAttrHead + `window_opening_mode: lock-whole-interface
no_auto_title: true
hide_title: true
hide_close_button: true
command_bar_location: none
enter_key_behavior: default-button
disabled: true
conversations: dont-show
no_auto_url: true
no_auto_fill_check: true
auto_save_data_in_settings: true
save_data_in_settings: true
not_customizable: true
`

// The window of a form keeps every property it was given, and comes back the
// same written out as YAML and carried through the Studio as JSON.
//
// Defect caught: a property of the window read into the wrong field or into
// none - the description refuses unknown properties, so one left out of the
// model refuses the form, and one mapped wrong loses its value without a
// word; and a property lost on the way through the Studio, which writes the
// form back. Every field of the window is checked to be set, so a property
// added to the model and not to this test fails it.
func TestTheWindowOfAFormKeepsEveryProperty(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formWindowWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	want := FormWindow{
		WindowOpeningMode: FormWindowLockWholeInterface, NoAutoTitle: true, HideTitle: true, HideCloseButton: true,
		CommandBarLocation: FormCommandBarNone, EnterKeyBehavior: FormEnterDefaultButton, Disabled: true,
		Conversations: FormConversationsDontShow, NoAutoURL: true, NoAutoFillCheck: true,
		AutoSaveDataInSettings: true, SaveDataInSettings: true, NotCustomizable: true,
	}
	if form.FormWindow != want {
		t.Fatalf("window = %+v, want %+v", form.FormWindow, want)
	}
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
		t.Fatalf("a form written back is refused: %v\n%s", err, written)
	}
	if again.FormWindow != want {
		t.Fatalf("written back: %+v", again.FormWindow)
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
		t.Fatalf("a form carried through the Studio is refused: %v", err)
	}
	if received.FormWindow != want {
		t.Fatalf("carried through the Studio: %+v", received.FormWindow)
	}
}

// A form that says nothing of its window has the defaults of the prototype,
// and a written default is accepted as it is.
//
// Defect caught: a default kept the other way round - a form that does not
// write ShowTitle shown without its title, the 5000 forms that do not write
// WindowOpeningMode opened as if they locked their owner.
func TestTheWindowOfAFormDefaultsAsThePrototype(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if form.FormWindow != (FormWindow{}) {
		t.Fatalf("window = %+v", form.FormWindow)
	}
	source := formAttrHead + "window_opening_mode: independent\ncommand_bar_location: auto\nenter_key_behavior: control-navigation\nconversations: auto\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err != nil {
		t.Fatalf("a written default is refused: %v", err)
	}
}

// A value of the window that is none of the help's is refused by name.
//
// Defect caught: a value the form cannot run accepted, so that the form opens
// with whatever the runtime falls back to; and a refusal that does not say
// which property.
func TestTheWindowOfAFormRefusesWhatIsNoneOfItsValues(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for property, want := range map[string]string{
		"window_opening_mode: modal":    "window_opening_mode must be independent, lock-owner-window or lock-whole-interface",
		"command_bar_location: left":    "command_bar_location must be auto, top, bottom or none",
		"enter_key_behavior: tab":       "enter_key_behavior must be control-navigation or default-button",
		"conversations: always":         "conversations must be auto, show or dont-show",
		"show_title: false":             "show_title",
		"customizable: false":           "customizable",
		"auto_save_data_in_settings: 1": "line 6: cannot unmarshal",
	} {
		t.Run(property, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+property+"\n"), configuration)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}
