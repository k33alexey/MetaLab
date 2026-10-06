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

// formLayoutWhole sets every property of the layout of a form away from its
// default.
const formLayoutWhole = formAttrHead + `vertical_scroll: use-if-necessary
width: 400
height: 150
children_group: horizontal-if-possible
children_width: left-narrowest
horizontal_align: center
vertical_align: bottom
items_and_titles_align: titles-left-data-auto
vertical_spacing: one-and-half
horizontal_spacing: half
scale_variant: compact
collapse_by_importance: dont-use
`

// The layout of a form keeps every property it was given, and comes back the
// same written out as YAML and carried through the Studio as JSON - the value
// of the alignment the help does not name included.
//
// Defect caught: a property of the layout read into the wrong field or into
// none, or lost on the way through the Studio; TitlesLeftDataAuto refused, so
// that the form of sb that writes it is not moved. Every field is checked to
// be set, so a property added to the model and not to this test fails it.
func TestTheLayoutOfAFormKeepsEveryProperty(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formLayoutWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	want := FormLayout{
		VerticalScroll: FormScrollUseIfNecessary, Width: 400, Height: 150, ChildrenGroup: ChildrenHorizontalIfPossible,
		ChildrenWidth: ChildrenWidthLeftNarrowest, HorizontalAlign: ItemHorizontalCenter, VerticalAlign: ItemVerticalBottom,
		ItemsAndTitlesAlign: ItemsAndTitlesTitlesLeftDataAuto, VerticalSpacing: ItemSpacingOneAndHalf,
		HorizontalSpacing: ItemSpacingHalf, ScaleVariant: FormScaleCompact, CollapseByImportance: CollapseByImportanceDontUse,
	}
	if form.FormLayout != want {
		t.Fatalf("layout = %+v, want %+v", form.FormLayout, want)
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
	if again.FormLayout != want {
		t.Fatalf("written back: %+v", again.FormLayout)
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
	if received.FormLayout != want {
		t.Fatalf("carried through the Studio: %+v", received.FormLayout)
	}
}

// A form that says nothing of its layout has the defaults of the prototype,
// a written default is accepted, and a size of no limit stands.
//
// Defect caught: a default kept the other way round, a written Auto refused,
// and a limit on the size of a form that the prototype does not have.
func TestTheLayoutOfAFormDefaultsAsThePrototype(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if form.FormLayout != (FormLayout{}) {
		t.Fatalf("layout = %+v", form.FormLayout)
	}
	source := formAttrHead + "vertical_scroll: auto\nwidth: 100000\nheight: 0\nchildren_group: vertical\nchildren_width: auto\n" +
		"horizontal_align: auto\nvertical_align: auto\nitems_and_titles_align: auto\nvertical_spacing: auto\nhorizontal_spacing: auto\n" +
		"scale_variant: auto\ncollapse_by_importance: auto\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err != nil {
		t.Fatalf("a written default is refused: %v", err)
	}
}

// A value of the layout that is none of the help's, and a negative size, are
// refused by name.
//
// Defect caught: a value the form cannot lay out accepted; a refusal that does
// not say which property; the two spacings checked as one.
func TestTheLayoutOfAFormRefusesWhatIsNoneOfItsValues(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for property, want := range map[string]string{
		"vertical_scroll: always":           "vertical_scroll must be auto, use, use-without-stretch or use-if-necessary",
		"width: -1":                         "width must not be negative",
		"height: -1":                        "height must not be negative",
		"children_group: grid":              "children_group must be vertical, horizontal, always-horizontal or horizontal-if-possible",
		"children_width: right-wide":        "children_width must be auto, equal, left-narrowest, left-narrow, left-wide or left-widest",
		"horizontal_align: justify":         "horizontal_align must be auto, left, center or right",
		"vertical_align: middle":            "vertical_align must be auto, top, center or bottom",
		"items_and_titles_align: titles-up": "items_and_titles_align must be auto, none, items-left-titles-left",
		"vertical_spacing: triple":          "vertical_spacing must be auto, none, half, single, one-and-half or double",
		"horizontal_spacing: triple":        "horizontal_spacing must be auto, none, half, single, one-and-half or double",
		"scale_variant: large":              "scale_variant must be auto, normal or compact",
		"collapse_by_importance: always":    "collapse_by_importance must be auto, use or dont-use",
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
