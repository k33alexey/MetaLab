package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
)

// plannerWhole is a planner whose content sets every property of the model,
// each to something other than its zero, but the header of the time scale,
// which is fixed off: false is a value of its own there, apart from the
// header fixed by itself.
const plannerWhole = `    planner:
      items:
        - &item
          text: Встреча
          text_formatted: true
          tooltip: Совещание отдела
          begin: "2015-08-06T01:00:00"
          end: "2015-08-06T04:00:00"
          back_color: {source: auto}
          text_color: {source: absolute, rgb: "#333333"}
          border_color: {source: style, from: {standard: FormBackColor}}
          font: {source: auto, bold: true}
          border: {source: absolute, line: single, width: 1}
          deleted: true
          replacement_date: "2015-08-07T01:00:00"
          enable_edit_mode: disable-stretch
          state: {id: 821efef7-461d-41c2-abc6-77cb513ba998}
        - *item
      back_color: {source: absolute, rgb: "#FFFFFF"}
      text_color: {source: auto}
      line_color: {source: web, name: Gray}
      border_color: {source: style, from: {standard: FormBackColor}}
      font: {source: style, from: {standard: TextFont}, size: 9}
      border: {source: absolute, line: single, width: 1}
      begin_of_representation_period: "2015-01-01T00:00:00"
      end_of_representation_period: "2015-12-31T23:59:59"
      current_representation_periods:
        - {begin: "2015-08-06T00:00:00", end: "2015-08-06T23:59:59"}
        - {begin: "2015-08-08T00:00:00", end: "2015-08-08T23:59:59"}
      align_item_boundaries_by_time_scale: true
      show_wrapped_headers: true
      show_wrapped_time_scale_headers: true
      wrapped_time_scale_header_format: {"#": "DLF=\"DD\""}
      periodic_variant_unit: day
      periodic_variant_repetition: 1
      time_scale_wrap_begin_indent: 9
      time_scale_wrap_end_indent: 7
      time_scale:
        location: left
        transparent: true
        back_color: {source: auto}
        text_color: {source: style, from: {standard: FormTextColor}}
        items:
          - &level {unit: hour, repetition: 1, visible: true, point_lines: {style: solid, width: 1}, line_color: {source: auto},
                    day_format: month-day, format: {ru: "ДФ=ЧЧ:мм"}, back_color: {source: auto}, text_color: {source: auto},
                    show_periodical_labels: true, labels_ticks: 3}
          - *level
        current_level: 1
      show_current_date: true
      items_time_representation: begin-and-end-time
      items_behavior_when_space_insufficient: show-all-items
      auto_column_min_width: true
      auto_row_min_height: true
      min_column_width: 40
      min_row_height: 20
      fix_dimensions_header: true
      fix_time_scale_header: false
      new_items_text_type: formatted-string
`

func plannerForm(planner string) string {
	return formAttrHead + "attributes:\n  - id: c0de0000-0000-4000-8000-000000990020\n    name: Планировщик\n    types: [{kind: planner}]\n" + planner
}

// The content of a planner comes back whole through YAML and the Studio:
// every property of the model, set in the source, is set in what is read
// back.
//
// Defect caught: a property of the planner, of an item, of a period it shows,
// of its time scale or an item of the scale, or the state of an item, lost on
// the way, so that the planner arrives drawn otherwise than the developer
// drew it; a header fixed off read back as fixed by itself; a property added
// to the model and left out of this test - the source sets every field,
// which the test checks before anything else.
func TestAPlannerKeepsItsContent(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(plannerForm(plannerWhole)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	planner := form.Attributes[0].Planner
	zero := zeroFields("planner", reflect.ValueOf(planner))
	if want := []string{"planner.FixTimeScaleHeader"}; !slices.Equal(zero, want) {
		t.Fatalf("the source leaves out: %v, want only %v", zero, want)
	}
	if planner.FixTimeScaleHeader == nil || *planner.FixTimeScaleHeader {
		t.Fatalf("the header of the time scale fixed off reads %v", planner.FixTimeScaleHeader)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
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
	if !reflect.DeepEqual(again.Attributes[0].Planner, planner) || !reflect.DeepEqual(received.Attributes[0].Planner, planner) {
		t.Fatalf("the planner changed on the way:\n%+v\n%+v", again.Attributes[0].Planner, received.Attributes[0].Planner)
	}
	copied := cloneFormAttributes(form.Attributes)[0].Planner
	copied.Items[0].Text = "Другое"
	copied.TimeScale.Items[0].Format["ru"] = "Другое"
	*copied.FixDimensionsHeader = false
	if planner.Items[0].Text != "Встреча" || planner.TimeScale.Items[0].Format["ru"] != "ДФ=ЧЧ:мм" || !*planner.FixDimensionsHeader {
		t.Fatal("a copy of the form shares the content of its planner")
	}
}

// What is wrong in the content of a planner is refused, naming the place.
//
// Defect caught: the prototype's spelling of a value (BeginTime, auto for a
// fixed header) taken as written; a value of no list taken; a date that is
// none, in a bound, a period shown, an item; a negative width, height,
// indent or repetition; an item whose identifier is no UUID; a time scale
// standing on an item it does not have; content on an attribute that is no
// planner.
func TestAPlannerRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	planner := func(rest string) string { return plannerForm("    planner:\n" + rest) }
	for name, test := range map[string]struct{ source, want string }{
		"время прототипа":     {planner("      items_time_representation: BeginTime\n"), "attributes[0].planner.items_time_representation must be one of begin-and-end-time,"},
		"нехватка места":      {planner("      items_behavior_when_space_insufficient: collapse\n"), "attributes[0].planner.items_behavior_when_space_insufficient must be one of collapse-items, show-all-items"},
		"тип текста":          {planner("      new_items_text_type: html\n"), "attributes[0].planner.new_items_text_type must be one of string, formatted-string"},
		"единица":             {planner("      periodic_variant_unit: decade\n"), "attributes[0].planner.periodic_variant_unit must be one of second,"},
		"фиксация авто":       {planner("      fix_dimensions_header: auto\n"), "cannot unmarshal !!str `auto` into bool"},
		"кратность":           {planner("      periodic_variant_repetition: -1\n"), "attributes[0].planner.periodic_variant_repetition must not be negative"},
		"отступ начала":       {planner("      time_scale_wrap_begin_indent: -1\n"), "attributes[0].planner.time_scale_wrap_begin_indent must not be negative"},
		"отступ конца":        {planner("      time_scale_wrap_end_indent: -1\n"), "attributes[0].planner.time_scale_wrap_end_indent must not be negative"},
		"ширина":              {planner("      min_column_width: -1\n"), "attributes[0].planner.min_column_width must not be negative"},
		"высота":              {planner("      min_row_height: -1\n"), "attributes[0].planner.min_row_height must not be negative"},
		"начало отображения":  {planner("      begin_of_representation_period: \"01.01.2015\"\n"), "attributes[0].planner.begin_of_representation_period must be a date written as"},
		"конец отображения":   {planner("      end_of_representation_period: \"2015-12-31\"\n"), "attributes[0].planner.end_of_representation_period must be a date written as"},
		"начало периода":      {planner("      current_representation_periods: [{begin: \"2015-08-06T25:00:00\"}]\n"), "attributes[0].planner.current_representation_periods[0].begin must be a date written as"},
		"конец периода":       {planner("      current_representation_periods: [{end: \"2015-08-06\"}]\n"), "attributes[0].planner.current_representation_periods[0].end must be a date written as"},
		"начало элемента":     {planner("      items: [{begin: \"2015-08-06 01:00:00\"}]\n"), "attributes[0].planner.items[0].begin must be a date written as"},
		"конец элемента":      {planner("      items: [{end: \"2015-08-06T04:00:00Z\"}]\n"), "attributes[0].planner.items[0].end must be a date written as"},
		"дата замещения":      {planner("      items: [{replacement_date: \"2015-08-06\"}]\n"), "attributes[0].planner.items[0].replacement_date must be a date written as"},
		"режим":               {planner("      items: [{enable_edit_mode: EnableEdit}]\n"), "attributes[0].planner.items[0].enable_edit_mode must be one of disable-drag-and-stretch,"},
		"идентификатор":       {planner("      items: [{state: {id: 821efef7}}]\n"), "attributes[0].planner.items[0].state.id must be a UUID"},
		"цвет элемента":       {planner("      items: [{back_color: {source: absolute, rgb: white}}]\n"), "attributes[0].planner.items[0].back_color.rgb must be a colour"},
		"рамка элемента":      {planner("      items: [{border: {source: absolute, line: wavy, width: 1}}]\n"), "attributes[0].planner.items[0].border"},
		"шрифт элемента":      {planner("      items: [{font: {source: auto, face: Roboto}}]\n"), "attributes[0].planner.items[0].font.face belongs to a font"},
		"цвет линий":          {planner("      line_color: {source: absolute, rgb: gray}\n"), "attributes[0].planner.line_color.rgb must be a colour"},
		"цвет рамки":          {planner("      border_color: {source: absolute, rgb: gray}\n"), "attributes[0].planner.border_color.rgb must be a colour"},
		"фон":                 {planner("      back_color: {source: absolute, rgb: gray}\n"), "attributes[0].planner.back_color.rgb must be a colour"},
		"формат переносов":    {planner("      wrapped_time_scale_header_format: {\"d=e\": DD}\n"), "attributes[0].planner.wrapped_time_scale_header_format.d=e is not a language code"},
		"положение шкалы":     {planner("      time_scale: {location: middle}\n"), "attributes[0].planner.time_scale.location must be one of top,"},
		"текущий уровень":     {planner("      time_scale: {items: [{unit: day}], current_level: 1}\n"), "attributes[0].planner.time_scale.current_level must be the number of an item"},
		"единица шкалы":       {planner("      time_scale: {items: [{unit: Hour}]}\n"), "attributes[0].planner.time_scale.items[0].unit must be one of second,"},
		"не планировщик":      {formAttrHead + "attributes:\n  - id: c0de0000-0000-4000-8000-000000990020\n    name: Диаграмма\n    types: [{kind: gantt-chart}]\n    planner: {show_current_date: true}\n", "attributes[0].planner belongs to an attribute that is a planner"},
		"пустое содержимое":   {planner("      time_scale: {}\n"), ""},
		"пустая дата":         {planner("      begin_of_representation_period: \"0001-01-01T00:00:00\"\n      items: [{replacement_date: \"0001-01-01T00:00:00\"}]\n"), ""},
		"пустые даты периода": {planner("      current_representation_periods: [{}]\n"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(test.source), configuration)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// What the content of a planner takes from the configuration is resolved at
// load, and what the model carries without knowing what it is, is noted.
//
// Defect caught: a colour, a font or a border taken from a style item the
// configuration no longer has loading clean - in the planner, in an item of
// it, in its time scale or an item of the scale; a colour taken from a style
// item that is a font accepted; the identifier of an item, the current level
// or the label ticks of the scale, or a text in "#" carried with no note; a
// note on a planner that has none of them.
func TestWhatAPlannerTakesIsResolved(t *testing.T) {
	t.Parallel()
	const font, color = "c0de0000-0000-4000-8000-000000990031", "c0de0000-0000-4000-8000-000000990032"
	for name, test := range map[string]struct {
		planner string
		gone    string
		refused string
		notes   []string
	}{
		"чистая": {planner: "{font: {source: style, from: {item: " + font + "}}, items: [{back_color: {source: style, from: {item: " + color + "}}}]}"},
		"элемента стиля нет у планировщика": {planner: "{line_color: {source: style, from: {item: " + refGone + "}}}",
			gone: "planner line_color"},
		"элемента стиля нет у элемента": {planner: "{items: [{}, {font: {source: style, from: {item: " + refGone + "}}}]}",
			gone: "planner items[1].font"},
		"элемента стиля нет в шкале": {planner: "{time_scale: {back_color: {source: style, from: {item: " + refGone + "}}}}",
			gone: "planner time_scale.back_color"},
		"элемента стиля нет в уровне": {planner: "{time_scale: {items: [{unit: day, text_color: {source: style, from: {item: " + refGone + "}}}]}}",
			gone: "planner time_scale.items[0].text_color"},
		"цвет из шрифта": {planner: "{border_color: {source: style, from: {item: " + font + "}}}",
			refused: "planner border_color takes its value from style item Мелкий, which is a font and not a color"},
		"состояние": {planner: "{items: [{text: Встреча}, {state: {id: 821efef7-461d-41c2-abc6-77cb513ba998}}], " +
			"time_scale: {items: [{unit: day}, {unit: hour, labels_ticks: 7}], current_level: 1}}",
			notes: []string{"planner-state-unexplained items[1].state, time_scale.current_level, time_scale.items[1].labels_ticks"}},
		"язык #": {planner: "{wrapped_time_scale_header_format: {\"#\": DD}, time_scale: {items: [{unit: hour, format: {\"#\": \"DF=HH\"}}]}}",
			notes: []string{"chart-text-any-language #", "chart-text-any-language #"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			writeMetadata(t, root, StyleItemKind, font, "format: 1\nid: "+font+"\nname: Мелкий\ntitle: {ru: Мелкий}\ntype: font\nvalue: {font: {source: auto}}\n")
			writeMetadata(t, root, StyleItemKind, color, "format: 1\nid: "+color+"\nname: Фон\ntitle: {ru: Фон}\ntype: color\nvalue: {color: {source: auto}}\n")
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, strings.Replace(string(content), "attributes:\n",
				"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990020, name: Планировщик, types: [{kind: planner}], planner: "+test.planner+"}\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента attribute Планировщик "
			if test.gone != "" {
				if found := unresolvedOf(t, root); !containsWhere(found, where+test.gone) {
					t.Fatalf("unresolved = %+v", found)
				}
				return
			}
			catalog, err := Load(root)
			if test.refused != "" {
				if err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("err = %v, want %q", err, test.refused)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if unresolved := catalog.UnresolvedReferences(); len(unresolved) != 0 {
				t.Fatalf("unresolved = %+v", unresolved)
			}
			var found []string
			for _, note := range catalog.Notes() {
				if strings.HasPrefix(note.Where, where) {
					found = append(found, string(note.Kind)+" "+note.Written)
				}
			}
			if !reflect.DeepEqual(found, test.notes) {
				t.Fatalf("notes = %q, want %q", found, test.notes)
			}
		})
	}
}
