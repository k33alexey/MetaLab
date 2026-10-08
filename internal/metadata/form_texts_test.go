package metadata

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// Every text of a form - of the form itself, of an element at any depth and
// its extended tooltip, of a command, of a value of a choice list, of a
// chart, of the conditional appearance - is noted under the same rules as
// the texts of objects, each named as the other notes of a form name their
// place. "#" is a language of its own only in a chart: there it has a note
// of its own and is not noted again; anywhere else it is a language nobody
// declared. A sound text carries no note.
//
// Defect caught: the walk of the texts of a form reaching the attributes and
// nothing else, so that a tooltip or a title of an element in a language
// nobody declared passes without the note the import report needs; an
// element inside a group, an extended tooltip or a text of a part of the
// properties of an element left out; the columns added to the second table
// named after the first; a text in "#"
// outside a chart taken for the any-language of a chart and passed in
// silence, or one inside a chart noted twice.
func TestTheTextsOfAFormAreNotedLikeTextsOfObjects(t *testing.T) {
	t.Parallel()
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	form := strings.Replace(string(content), "title: {ru: Форма элемента}\n",
		"title: {ru: Форма элемента, de: Element}\nexplanation: {\"\": Пояснение}\n", 1)
	form = strings.Replace(form, "    additional_columns:\n", "    additional_columns:\n"+
		"      - table: Объект.Цены\n        columns:\n          - {id: c0de0000-0000-4000-8000-000000990030, name: Цена, title: {de: Preis}}\n", 1)
	form = strings.Replace(form, "name: Склад, types:", "name: Склад, title: {de: Lager}, types:", 1)
	form = strings.Replace(form, "attributes:\n", "attributes:\n"+
		"  - {id: c0de0000-0000-4000-8000-000000990020, name: Диаграмма, types: [{kind: chart}], chart: {type: pie, active_series: -1,"+
		" summary_series: {id: 1, text: {\"#\": Сводная, de: Summe}}, title_area: {text: {ru: Продажи, \"#\": Продажи}}}}\n", 1)
	form += "commands:\n  - {id: c0de0000-0000-4000-8000-000000990010, name: Записать, title: {ru: Записать, en: Save}, tool_tip: {ru: \" \"}, action: custom}\n" +
		"items:\n  - id: c0de0000-0000-4000-8000-000000990001\n    name: Группа\n    kind: usual-group\n    title: {ru: Группа}\n    children:\n" +
		"      - {id: c0de0000-0000-4000-8000-000000990002, name: Наименование, kind: input-field, data_path: Объект.Наименование," +
		" title: {\"#\": Наименование}, tool_tip: {ru: Наименование, uk: Найменування}, input_hint: {en: Name}, choice_list: [{value: {kind: string, data: А}, presentation: {en: A}}]," +
		" extended_tooltip: {id: c0de0000-0000-4000-8000-000000990003, name: НаименованиеРасширеннаяПодсказка, kind: label-decoration, title: {ru: \"\\t\"}}}\n" +
		"conditional_appearance:\n  - {fields: [{field: Наименование}], appearance: [{parameter: text, value: {kind: localized-string, text: {de: Text}}}]}\n"
	writeFile(t, path, form)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	const at = "catalog Номенклатура form ФормаЭлемента"
	got := map[string]string{}
	for _, note := range catalog.Notes() {
		if strings.HasPrefix(note.Where, at) && note.Kind != NotePlatformTypeByName {
			got[string(note.Kind)+" "+strings.TrimPrefix(note.Where, at)] = note.Written
		}
	}
	want := map[string]string{
		"text-in-undeclared-language  title.de":                                                    "Element",
		"text-without-language  explanation":                                                       "Пояснение",
		"text-in-undeclared-language  attribute Диаграмма chart summary_series text.de":            "Summe",
		"chart-text-any-language  attribute Диаграмма chart summary_series.text":                   "#",
		"chart-text-any-language  attribute Диаграмма chart title_area.text":                       "#",
		"text-in-undeclared-language  command Записать title.en":                                   "Save",
		"text-of-spaces  command Записать tool_tip.ru":                                             `" "`,
		"text-in-undeclared-language  element Наименование title.#":                                "Наименование",
		"text-in-undeclared-language  element Наименование input_hint.en":                          "Name",
		"text-in-undeclared-language  attribute Объект table Объект.Цены column Цена title.de":     "Preis",
		"text-in-undeclared-language  attribute Объект table Объект.Остатки column Склад title.de": "Lager",
		"text-in-undeclared-language  element Наименование choice_list 0 presentation.en":          "A",
		"text-of-spaces  element НаименованиеРасширеннаяПодсказка title.ru":                        `"\t"`,
		"text-in-undeclared-language  conditional_appearance 0 appearance 0 value text.de":         "Text",
	}
	if !reflect.DeepEqual(got, want) {
		for place, written := range want {
			if got[place] != written {
				t.Errorf("no note %q = %q", place, written)
			}
		}
		for place, written := range got {
			if _, ok := want[place]; !ok {
				t.Errorf("note %q = %q is not expected", place, written)
			}
		}
	}
}

// The walk of the texts of a form reaches every text the model of a form
// holds: a form with a text in every place a text can stand - every list
// given one item, every pointer set - hands every one of them to the notes.
//
// Defect caught: a place of a text the walk does not enter - a text behind a
// pointer, in a list, inside the content of a chart or a setting, a type
// wrongly remembered as holding no text - so that a text added to the model
// of a form is never noted.
func TestTheWalkOfTheTextsOfAFormReachesEveryText(t *testing.T) {
	t.Parallel()
	var form ManagedForm
	placed := fillTexts(reflect.ValueOf(&form).Elem(), map[reflect.Type]bool{}, new(int))
	if placed < 40 {
		t.Fatalf("only %d texts placed: the filling missed the model", placed)
	}
	seen := map[string]bool{}
	walk := formTextWalk{form: "form", path: []string{"form"}, visit: func(_ string, text LocalizedText, _ bool) {
		seen[text["de"]] = true
	}}
	walk.value(reflect.ValueOf(&form).Elem(), false)
	if len(seen) != placed {
		t.Fatalf("the walk saw %d of %d texts", len(seen), placed)
	}
}

// fillTexts sets every text the value can hold to a text of its own, gives
// every list one item and every pointer a value, and counts the texts. A type
// met again below itself is left empty: it holds what the outer one does.
func fillTexts(value reflect.Value, inside map[reflect.Type]bool, count *int) int {
	if inside[value.Type()] {
		return *count
	}
	switch value.Kind() {
	case reflect.Map:
		if value.Type() == reflect.TypeFor[LocalizedText]() {
			*count++
			value.Set(reflect.ValueOf(LocalizedText{"de": strings.Repeat("x", *count)}))
			return *count
		}
		item := reflect.New(value.Type().Elem()).Elem()
		fillTexts(item, inside, count)
		value.Set(reflect.MakeMap(value.Type()))
		value.SetMapIndex(reflect.New(value.Type().Key()).Elem(), item)
	case reflect.Pointer:
		if inside[value.Type().Elem()] {
			return *count
		}
		value.Set(reflect.New(value.Type().Elem()))
		fillTexts(value.Elem(), inside, count)
	case reflect.Slice:
		if inside[value.Type().Elem()] {
			return *count
		}
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
		fillTexts(value.Index(0), inside, count)
	case reflect.Struct:
		inside[value.Type()] = true
		for index := range value.NumField() {
			if value.Type().Field(index).IsExported() {
				fillTexts(value.Field(index), inside, count)
			}
		}
		delete(inside, value.Type())
	}
	return *count
}
