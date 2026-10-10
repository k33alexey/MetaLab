package metadata

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

// Each property the help names and no export writes is noted once, on its
// own place and with its value; a property of an export of the configurator
// beside it is not.
//
// Defect caught: a property without a sample accepted silently - left out of
// the list of the rule, so that the import report never asks how the
// configurator writes it; a note on another element or under the name of
// another property, which sends the review to the wrong place; a property
// with a sample noted, burying the real notes.
func TestEachPropertyWithoutASampleIsNotedOnItsPlace(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ element, property, written string }{
		"путь значения":             {"kind: input-field, multiple_value_value_data_path: Метки.Значение", "multiple_value_value_data_path", "Метки.Значение"},
		"путь представления":        {"kind: input-field, multiple_value_presentation_data_path: Метки.Имя", "multiple_value_presentation_data_path", "Метки.Имя"},
		"путь картинки":             {"kind: input-field, multiple_value_picture_data_path: Метки.Вид", "multiple_value_picture_data_path", "Метки.Вид"},
		"дубли":                     {"kind: input-field, allow_multiple_values_duplicates: true", "allow_multiple_values_duplicates", "true"},
		"картинка значений":         {"kind: input-field, multiple_values_picture: {standard: Change}", "multiple_values_picture", ""},
		"автозаполнение":            {"kind: input-field, auto_fill_hint: email", "auto_fill_hint", "email"},
		"ограничение типа":          {"kind: input-field, type_restriction: [{kind: string, length: 10}]", "type_restriction", ""},
		"чёрно-белый":               {"kind: spreadsheet-document-field, black_and_white_view: false", "black_and_white_view", "false"},
		"перенесённая шкала":        {"kind: planner-field, wrapped_time_scale_header_hyperlink: true", "wrapped_time_scale_header_hyperlink", "true"},
		"интервалы Ганта":           {"kind: gantt-chart-field, intervals_selection_mode: single", "intervals_selection_mode", "single"},
		"заголовок скрытой":         {"kind: usual-group, hidden_representation_title_back_color: {source: web, name: Red}", "hidden_representation_title_back_color", ""},
		"панель иерархии":           {"kind: table, hierarchy_panel_location: left", "hierarchy_panel_location", "left"},
		"рядом свойство с образцом": {"kind: input-field, multiple_values_hyperlink: true, auto_fill_hint: email", "auto_fill_hint", "email"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeCommonForm(t, root, "Метки", "format: 1\nid: "+uuid.MustNew().String()+"\nname: Метки\ntitle: {ru: Метки}\nkind: common\n"+
				"items:\n  - {id: "+uuid.MustNew().String()+", name: Соседний, kind: input-field}\n"+
				"  - {id: "+uuid.MustNew().String()+", name: Элемент, "+test.element+"}\n")
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			notes := catalog.Notes()
			want := Note{Kind: NoteWithoutSample, Where: "common form Метки element Элемент " + test.property, Written: test.written}
			if len(notes) != 1 || notes[0] != want {
				t.Fatalf("notes = %+v, want %+v", notes, want)
			}
		})
	}
}

// The multiple values of an input field are kept through YAML and the
// Studio, each said or not said as written.
//
// Defect caught: a property read into the wrong field or into none, or lost
// through the Studio - a value list shown as links (42 fields of other
// exports) or with check boxes turned off (6) falling back to the default of
// the platform; a "no" of check boxes read as not said.
func TestAnInputFieldKeepsItsMultipleValues(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	items := "  - {id: c0de0000-0000-4000-8000-000000990001, name: Метки, kind: input-field, multiple_value_data_path: Метки.Значение," +
		" multiple_value_value_data_path: Метки.Значение, multiple_value_presentation_data_path: Метки.Представление," +
		" multiple_value_picture_data_path: Метки.Картинка, multiple_values_hyperlink: true, show_check_boxes_in_drop_list: false," +
		" allow_input_empty_multiple_values: true, allow_multiple_values_duplicates: true, multiple_values_picture: {standard: Change}," +
		" multiple_value_picture_size: small, multiple_value_picture_shape: circle, multiple_values_text_color: {source: web, name: White}," +
		" multiple_values_back_color: {source: absolute, rgb: \"#190E70\"}, multiple_values_font: {source: auto}}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formElementsForm(items)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	values := form.Items[0].FieldMultipleValues
	value := reflect.ValueOf(values)
	for index := range value.NumField() {
		if value.Field(index).IsZero() {
			t.Errorf("%s is not set by the test", value.Type().Field(index).Name)
		}
	}
	switch {
	case values.MultipleValueDataPath != "Метки.Значение" || values.MultipleValueValueDataPath != "Метки.Значение" ||
		values.MultipleValuePresentationDataPath != "Метки.Представление" || values.MultipleValuePictureDataPath != "Метки.Картинка":
		t.Fatalf("data paths: %+v", values)
	case !*values.MultipleValuesHyperlink || *values.ShowCheckBoxesInDropList || !values.AllowInputEmptyMultipleValues || !values.AllowMultipleValuesDuplicates:
		t.Fatalf("switches: %+v", values)
	case values.MultipleValuesPicture.Standard != "Change" || values.MultipleValuePictureSize != "small" || values.MultipleValuePictureShape != "circle":
		t.Fatalf("picture: %+v", values)
	case values.MultipleValuesTextColor.Name != "White" || values.MultipleValuesBackColor.RGB != "#190E70" || values.MultipleValuesFont.Source != AutoFont:
		t.Fatalf("look: %+v", values)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "show_check_boxes_in_drop_list: false\n") {
		t.Fatalf("check boxes turned off are not written:\n%s", written)
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

// Each property of multiple values, set alone on every kind of element,
// stands only on an input field, and a value it cannot take is refused by
// name.
//
// Defect caught: multiple values kept on a label field or a table, where
// nothing edits a list; a property left out of the check for being empty, so
// that it passes on any element; the prototype's spelling of a size or a
// shape taken as written; a data path with spaces, a picture naming two, a
// colour without its value accepted.
func TestMultipleValuesStandOnlyOnAnInputField(t *testing.T) {
	t.Parallel()
	yes := true
	color, font := &ColorValue{Source: AutoColor}, &FontValue{Source: AutoFont}
	full := FieldMultipleValues{MultipleValueDataPath: "М.З", MultipleValueValueDataPath: "М.З", MultipleValuePresentationDataPath: "М.П",
		MultipleValuePictureDataPath: "М.К", MultipleValuesHyperlink: &yes, ShowCheckBoxesInDropList: &yes, AllowInputEmptyMultipleValues: true,
		AllowMultipleValuesDuplicates: true, MultipleValuesPicture: &PictureReference{Standard: "Change"}, MultipleValuePictureSize: "auto",
		MultipleValuePictureShape: "auto", MultipleValuesTextColor: color, MultipleValuesBackColor: color, MultipleValuesFont: font}
	value := reflect.ValueOf(full)
	for index := range value.NumField() {
		name := value.Type().Field(index).Name
		if value.Field(index).IsZero() {
			t.Fatalf("%s is not set by the test", name)
		}
		for kind := range formElementClasses {
			var alone FieldMultipleValues
			reflect.ValueOf(&alone).Elem().Field(index).Set(value.Field(index))
			issues := validateFieldMultipleValues("items[0]", alone, kind)
			refused := len(issues) == 1 && issues[0] == "items[0] has the multiple values of an input field"
			if want := kind != FormElementInputField; want != refused || !want && len(issues) != 0 {
				t.Errorf("%s alone on %s: %v", name, kind, issues)
			}
		}
	}
	for name, test := range map[string]struct {
		values FieldMultipleValues
		want   string
	}{
		"размер прототипа":   {FieldMultipleValues{MultipleValuePictureSize: "Small"}, "items[0].multiple_value_picture_size must be auto, small, medium or large"},
		"фигура прототипа":   {FieldMultipleValues{MultipleValuePictureShape: "Rect"}, "items[0].multiple_value_picture_shape must be auto, rect or circle"},
		"путь с пробелом":    {FieldMultipleValues{MultipleValueDataPath: " Метки"}, "items[0].multiple_value_data_path must be a data path without surrounding spaces"},
		"путь картинки":      {FieldMultipleValues{MultipleValuePictureDataPath: "Метки..Вид"}, "items[0].multiple_value_picture_data_path must be names separated by dots"},
		"картинка двумя":     {FieldMultipleValues{MultipleValuesPicture: &PictureReference{Standard: "Change", File: "Picture.png"}}, "items[0].multiple_values_picture"},
		"цвет без значения":  {FieldMultipleValues{MultipleValuesBackColor: &ColorValue{Source: AbsoluteColor}}, "items[0].multiple_values_back_color"},
		"шрифт без значения": {FieldMultipleValues{MultipleValuesFont: &FontValue{Source: "style"}}, "items[0].multiple_values_font"},
	} {
		issues := validateFieldMultipleValues("items[0]", test.values, FormElementInputField)
		if !slices.ContainsFunc(issues, func(issue string) bool { return strings.HasPrefix(issue, test.want) }) {
			t.Errorf("%s: %v, want %q", name, issues, test.want)
		}
	}
	form := formElementsForm("  - {id: c0de0000-0000-4000-8000-000000990001, name: Поле, kind: label-field, multiple_values_hyperlink: true}\n")
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(form), managedFormConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "items[0] has the multiple values of an input field") {
		t.Errorf("multiple values on a label field read from a file: %v", err)
	}
}

// What the new properties name is resolved with the project: a style item of
// another type is refused, one that is gone is a reference to nothing, as is
// a common picture, and a type a field is narrowed to must be one the
// project has.
//
// Defect caught: the font of multiple values or the colour of the title of a
// hidden group taken from a style item of another type, or from one that is
// gone, without a word, because their places were left out of the walk over
// style items; a picture of multiple values naming a gone common picture
// loading clean; a type restriction naming a gone catalog accepted.
func TestWhatTheNewPropertiesNameIsResolved(t *testing.T) {
	t.Parallel()
	const font, color, gone = "e1000000-0000-4000-8000-000000000001", "e1000000-0000-4000-8000-000000000002", "e1000000-0000-4000-8000-000000000003"
	for name, test := range map[string]struct {
		element, refused, unresolved string
	}{
		"шрифт значений цветом": {element: "kind: input-field, multiple_values_font: {source: style, from: {item: " + color + "}}",
			refused: "element Поле multiple_values_font takes its value from style item Фон, which is a color and not a font"},
		"цвет значений шрифтом": {element: "kind: input-field, multiple_values_text_color: {source: style, from: {item: " + font + "}}",
			refused: "element Поле multiple_values_text_color takes its value from style item Мелкий, which is a font and not a color"},
		"фон значений удалён": {element: "kind: input-field, multiple_values_back_color: {source: style, from: {item: " + gone + "}}",
			unresolved: "element Поле multiple_values_back_color"},
		"заголовок скрытой шрифтом": {element: "kind: usual-group, hidden_representation_title_back_color: {source: style, from: {item: " + font + "}}",
			refused: "element Поле hidden_representation_title_back_color takes its value from style item Мелкий, which is a font and not a color"},
		"картинка значений удалена": {element: "kind: input-field, multiple_values_picture: {common: " + gone + "}",
			unresolved: "element Поле multiple_values_picture"},
		"тип удалён": {element: "kind: input-field, type_restriction: [{kind: catalog, reference: " + gone + "}]",
			refused: "element Поле type restriction"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, StyleItemKind, font, "format: 1\nid: "+font+"\nname: Мелкий\ntitle: {ru: Мелкий}\ntype: font\nvalue: {font: {source: auto}}\n")
			writeMetadata(t, root, StyleItemKind, color, "format: 1\nid: "+color+"\nname: Фон\ntitle: {ru: Фон}\ntype: color\nvalue: {color: {source: auto}}\n")
			writeCommonForm(t, root, "Метки", "format: 1\nid: "+uuid.MustNew().String()+"\nname: Метки\ntitle: {ru: Метки}\nkind: common\n"+
				"items:\n  - {id: "+uuid.MustNew().String()+", name: Поле, "+test.element+"}\n")
			if test.refused != "" {
				if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("err = %v, want %q", err, test.refused)
				}
				return
			}
			if found := unresolvedOf(t, root); !containsWhere(found, "common form Метки "+test.unresolved) {
				t.Fatalf("unresolved = %+v", found)
			}
		})
	}
}
