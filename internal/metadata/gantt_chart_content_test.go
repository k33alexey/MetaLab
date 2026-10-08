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

// ganttWhole is a Gantt chart whose content sets every property of the
// model, each to something other than its zero; the chart inside is
// chartWhole.
var ganttWhole = "    gantt_chart:\n  " + strings.ReplaceAll(strings.TrimSuffix(chartWhole, "\n"), "\n", "\n  ") + "\n" + `      points:
        text: {ru: Операции}
        font: {source: style, from: {standard: TextFont}, size: 9}
        color: {source: absolute, rgb: "#000000"}
        second_color: {source: web, name: DarkGreen}
        back_color: {source: auto}
        text_color: {source: style, from: {standard: FormTextColor}}
        auto_text: true
        state: {item_key: 1, key: 2, parent_key: 3, left_key: 4, right_key: 5, ext_key: 6, cache_key: 7, base_data: 4294959665,
                test_mode: true, use_values_reverse_behavior: true}
      series:
        text: {ru: Ресурсы}
        color: {source: absolute, rgb: "#000000"}
        second_color: {source: absolute, rgb: "#000000"}
        between_intervals_hatch_color: {source: auto}
        auto_text: true
        state: {item_key: 1, key: 2, parent_key: 3, left_key: 4, right_key: 5, ext_key: 6, cache_key: 7, base_data: 4294907841,
                test_mode: true, use_values_reverse_behavior: true}
      plot_area:
        title: {ru: Операция}
        time_scale:
          location: top
          transparent: true
          back_color: {source: style, from: {standard: FieldBackColor}}
          text_color: {source: absolute, rgb: "#333333"}
          items:
            - &item {unit: month, repetition: 1, visible: true, point_lines: {style: dotted, width: 1}, line_color: {source: absolute, rgb: "#C0C0C0"},
                     day_format: month-day-week-day, format: {ru: "ДФ=MMMM"}, back_color: {source: auto}, text_color: {source: auto},
                     show_periodical_labels: true, labels_ticks: 5}
            - *item
          current_level: 1
        outbound_whole_interval_color: {source: absolute, rgb: "#FFFFFF"}
        link_lines: {style: solid, width: 1}
        link_lines_color: {source: absolute, rgb: "#000080"}
        show_points_text: show
        show_data: dont-show
        text_placement: wrap
      show_empty_values: true
      scale_keeping: period
      periodic_variant_unit: month
      periodic_variant_repetition: 2
      auto_detect_whole_interval: true
      begin_of_whole_interval: "2016-05-01T00:00:00"
      end_of_whole_interval: "2016-06-01T00:00:00"
      interval_representation: three-dimensional
      interval_text_representation: dont-show
      value_text_representation: right
      vertical_stretch: stretch-rows-and-data
      vertical_scroll: true
      state:
        visual_begin: "2016-05-01T00:00:00"
        none_variant_chars: 3
        none_variant_measure: day
        background_intervals_ticks: 1
        background_intervals_ticks_inner: 2
`

func ganttForm(gantt string) string {
	return formAttrHead + "attributes:\n  - id: c0de0000-0000-4000-8000-000000990020\n    name: График\n    types: [{kind: gantt-chart}]\n" + gantt
}

// The content of a Gantt chart comes back whole through YAML and the Studio:
// every property of the model, set in the source, is set in what is read
// back, the chart inside included.
//
// Defect caught: a property of the Gantt chart, of the root of its points or
// series, of its plot area, time scale or one of its items, or of its state
// lost on the way, so that the chart arrives drawn otherwise than the
// developer drew it; a property added to the model and left out of this test
// - the source sets every field but those the help gives a point and a
// series not, which the test checks before anything else.
func TestAGanttChartKeepsItsContent(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(ganttForm(ganttWhole)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	gantt := form.Attributes[0].GanttChart
	zero := zeroFields("gantt", reflect.ValueOf(gantt))
	if want := []string{"gantt.Points.BetweenIntervalsHatchColor", "gantt.Series.Font", "gantt.Series.BackColor", "gantt.Series.TextColor"}; !slices.Equal(zero, want) {
		t.Fatalf("the source leaves out: %v, want only %v", zero, want)
	}
	if gantt.Chart.PlotArea.Bounds.Left != "0.114401076716016150740242261" || gantt.Points.State.BaseData != 4294959665 {
		t.Fatalf("numbers: %+v %+v", gantt.Chart.PlotArea.Bounds, gantt.Points.State)
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
	if !reflect.DeepEqual(again.Attributes[0].GanttChart, gantt) || !reflect.DeepEqual(received.Attributes[0].GanttChart, gantt) {
		t.Fatalf("the Gantt chart changed on the way:\n%+v\n%+v", again.Attributes[0].GanttChart, received.Attributes[0].GanttChart)
	}
	copied := cloneFormAttributes(form.Attributes)[0].GanttChart
	copied.PlotArea.TimeScale.Items[0].Unit = "year"
	copied.Chart.Series[0].Text["ru"] = "Другое"
	if gantt.PlotArea.TimeScale.Items[0].Unit != "month" || gantt.Chart.Series[0].Text["ru"] != "Себестоимость" {
		t.Fatal("a copy of the form shares the content of its Gantt chart")
	}
}

// What is wrong in the content of a Gantt chart is refused, naming the
// place.
//
// Defect caught: the prototype's spelling of a value (AllData) taken as
// written; a value of no list taken; a whole interval that ends before it
// begins or reaches past the bounds of the help; a date that is none; a
// time scale standing on an item it does not have; a look the help does not
// give a point or a series; what is wrong in the chart inside let through;
// content on an attribute that is no Gantt chart.
func TestAGanttChartRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	gantt := func(rest string) string {
		return ganttForm("    gantt_chart:\n      chart: {type: column, active_series: -1}\n" + rest)
	}
	for name, test := range map[string]struct{ source, want string }{
		"масштаб прототипа":  {gantt("      scale_keeping: AllData\n"), "attributes[0].gantt_chart.scale_keeping must be one of all-data,"},
		"единица":            {gantt("      periodic_variant_unit: decade\n"), "attributes[0].gantt_chart.periodic_variant_unit must be one of second,"},
		"интервал":           {gantt("      interval_representation: flat-3d\n"), "attributes[0].gantt_chart.interval_representation must be one of flat,"},
		"текст интервала":    {gantt("      interval_text_representation: none\n"), "attributes[0].gantt_chart.interval_text_representation must be one of auto,"},
		"текст значения":     {gantt("      value_text_representation: left\n"), "attributes[0].gantt_chart.value_text_representation must be one of none, right"},
		"растягивание":       {gantt("      vertical_stretch: stretch\n"), "attributes[0].gantt_chart.vertical_stretch must be one of none,"},
		"данные":             {gantt("      plot_area: {show_data: none}\n"), "attributes[0].gantt_chart.plot_area.show_data must be one of auto,"},
		"текст точек":        {gantt("      plot_area: {show_points_text: none}\n"), "attributes[0].gantt_chart.plot_area.show_points_text must be one of auto,"},
		"размещение":         {gantt("      plot_area: {text_placement: clip}\n"), "attributes[0].gantt_chart.plot_area.text_placement must be one of auto, cut, wrap"},
		"линии связей":       {gantt("      plot_area: {link_lines: {style: wavy}}\n"), "attributes[0].gantt_chart.plot_area.link_lines.style is not a way a line is drawn"},
		"цвет связей":        {gantt("      plot_area: {link_lines_color: {source: absolute, rgb: navy}}\n"), "attributes[0].gantt_chart.plot_area.link_lines_color.rgb must be a colour"},
		"заголовок":          {gantt("      plot_area: {title: {\"d=e\": Операция}}\n"), "attributes[0].gantt_chart.plot_area.title.d=e is not a language code"},
		"положение шкалы":    {gantt("      plot_area: {time_scale: {location: middle}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.location must be one of top,"},
		"без единицы":        {gantt("      plot_area: {time_scale: {items: [{repetition: 1}]}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.items[0].unit must be one of second,"},
		"кратность":          {gantt("      plot_area: {time_scale: {items: [{unit: day, repetition: -1}]}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.items[0].repetition must not be negative"},
		"формат дня":         {gantt("      plot_area: {time_scale: {items: [{unit: day, day_format: day}]}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.items[0].day_format must be one of"},
		"линии точек":        {gantt("      plot_area: {time_scale: {items: [{unit: day, point_lines: {style: wavy}}]}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.items[0].point_lines.style is not a way"},
		"текущий уровень":    {gantt("      plot_area: {time_scale: {items: [{unit: day}], current_level: 1}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.current_level must be the number of an item"},
		"уровень меньше":     {gantt("      plot_area: {time_scale: {current_level: -1}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.current_level must be the number of an item"},
		"конец до начала":    {gantt("      begin_of_whole_interval: \"2016-06-01T00:00:00\"\n      end_of_whole_interval: \"2016-05-01T00:00:00\"\n"), "attributes[0].gantt_chart.end_of_whole_interval must not be earlier than its begin"},
		"начало раньше":      {gantt("      begin_of_whole_interval: \"0999-12-31T23:59:59\"\n"), "attributes[0].gantt_chart.begin_of_whole_interval must not be earlier than 1000-01-01T00:00:00"},
		"конец позже":        {gantt("      end_of_whole_interval: \"3000-01-01T00:00:01\"\n"), "attributes[0].gantt_chart.end_of_whole_interval must not be later than 3000-01-01T00:00:00"},
		"не дата":            {gantt("      begin_of_whole_interval: \"01.05.2016\"\n"), "attributes[0].gantt_chart.begin_of_whole_interval must be a date written as 2006-01-02T15:04:05"},
		"доли секунды":       {gantt("      end_of_whole_interval: \"2016-05-01T00:00:00.5\"\n"), "attributes[0].gantt_chart.end_of_whole_interval must be a date written as"},
		"фон шкалы":          {gantt("      plot_area: {time_scale: {back_color: {source: absolute, rgb: white}}}\n"), "attributes[0].gantt_chart.plot_area.time_scale.back_color.rgb must be a colour"},
		"видимое начало":     {gantt("      state: {visual_begin: \"2016-05-01\"}\n"), "attributes[0].gantt_chart.state.visual_begin must be a date written as"},
		"мера без варианта":  {gantt("      state: {none_variant_measure: Day}\n"), "attributes[0].gantt_chart.state.none_variant_measure must be one of"},
		"символы":            {gantt("      state: {none_variant_chars: -1}\n"), "attributes[0].gantt_chart.state.none_variant_chars must not be negative"},
		"кратность варианта": {gantt("      periodic_variant_repetition: -1\n"), "attributes[0].gantt_chart.periodic_variant_repetition must not be negative"},
		"шрифт серии":        {gantt("      series: {font: {source: auto}}\n"), "attributes[0].gantt_chart.series is a series, which has no font, back colour or text colour of its own"},
		"фон серии":          {gantt("      series: {back_color: {source: auto}}\n"), "attributes[0].gantt_chart.series is a series, which has no font"},
		"текст серии":        {gantt("      series: {text_color: {source: auto}}\n"), "attributes[0].gantt_chart.series is a series, which has no font"},
		"штриховка точки":    {gantt("      points: {between_intervals_hatch_color: {source: auto}}\n"), "attributes[0].gantt_chart.points.between_intervals_hatch_color belongs to a series"},
		"цвет точки":         {gantt("      points: {second_color: {source: absolute, rgb: black}}\n"), "attributes[0].gantt_chart.points.second_color.rgb must be a colour"},
		"шрифт точки":        {gantt("      points: {font: {source: auto, face: Roboto}}\n"), "attributes[0].gantt_chart.points.font.face belongs to a font"},
		"диаграмма внутри":   {ganttForm("    gantt_chart:\n      chart: {type: Column3D, active_series: -1}\n"), "attributes[0].gantt_chart.chart.type must be one of area,"},
		"без диаграммы":      {ganttForm("    gantt_chart: {show_empty_values: true}\n"), "attributes[0].gantt_chart.chart.type must be one of area,"},
		"не Ганта":           {formAttrHead + "attributes:\n  - id: c0de0000-0000-4000-8000-000000990020\n    name: Диаграмма\n    types: [{kind: chart}]\n    gantt_chart: {chart: {type: pie, active_series: -1}}\n", "attributes[0].gantt_chart belongs to an attribute that is a Gantt chart"},
		"граница интервала":  {gantt("      begin_of_whole_interval: \"1000-01-01T00:00:00\"\n      end_of_whole_interval: \"3000-01-01T00:00:00\"\n"), ""},
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

// What the content of a Gantt chart takes from the configuration is resolved
// at load, and what the model carries without knowing what it is, is noted.
//
// Defect caught: a colour or a font taken from a style item the
// configuration no longer has loading clean - in the time scale, in an item
// of it, in the root of the points, in the chart inside; a colour taken from
// a style item that is a font accepted; the state of the prototype's own
// object - of the Gantt chart, of its roots, of its time scale, of the chart
// inside - or a text in "#" carried with no note; a note on a Gantt chart
// that has none of them.
func TestWhatAGanttChartTakesIsResolved(t *testing.T) {
	t.Parallel()
	const font, color = "c0de0000-0000-4000-8000-000000990031", "c0de0000-0000-4000-8000-000000990032"
	const chart = "chart: {type: column, active_series: -1}"
	for name, test := range map[string]struct {
		gantt   string
		gone    string
		refused string
		notes   []string
	}{
		"чистая": {gantt: "{" + chart + ", points: {font: {source: style, from: {item: " + font + "}}}, plot_area: {time_scale: {back_color: {source: style, from: {item: " + color + "}}}}}"},
		"элемента стиля нет в шкале": {gantt: "{" + chart + ", plot_area: {time_scale: {text_color: {source: style, from: {item: " + refGone + "}}}}}",
			gone: "gantt_chart plot_area.time_scale.text_color"},
		"элемента стиля нет в уровне": {gantt: "{" + chart + ", plot_area: {time_scale: {items: [{unit: day, line_color: {source: style, from: {item: " + refGone + "}}}]}}}",
			gone: "gantt_chart plot_area.time_scale.items[0].line_color"},
		"элемента стиля нет в точках": {gantt: "{" + chart + ", points: {font: {source: style, from: {item: " + refGone + "}}}}",
			gone: "gantt_chart points.font"},
		"элемента стиля нет в диаграмме": {gantt: "{chart: {type: column, active_series: -1, back_color: {source: style, from: {item: " + refGone + "}}}}",
			gone: "gantt_chart chart.back_color"},
		"цвет из шрифта": {gantt: "{" + chart + ", plot_area: {link_lines_color: {source: style, from: {item: " + font + "}}}}",
			refused: "gantt_chart plot_area.link_lines_color takes its value from style item Мелкий, which is a font and not a color"},
		"состояние": {gantt: "{" + chart + ", state: {visual_begin: \"2016-05-01T00:00:00\", none_variant_chars: 3, none_variant_measure: day, " +
			"background_intervals_ticks: 1, background_intervals_ticks_inner: 2}}",
			notes: []string{"gantt-chart-state-unexplained visual_begin, none_variant_chars, none_variant_measure, background_intervals_ticks, background_intervals_ticks_inner"}},
		"состояние корней и шкалы": {gantt: "{" + chart + ", points: {state: {base_data: 4294959665}}, series: {state: {test_mode: true}}, " +
			"plot_area: {time_scale: {items: [{unit: day}, {unit: hour, labels_ticks: 7}], current_level: 1}}}",
			notes: []string{"gantt-chart-state-unexplained points.state, series.state, plot_area.time_scale.current_level, plot_area.time_scale.items[1].labels_ticks"}},
		"состояние диаграммы внутри": {gantt: "{chart: {type: column, active_series: -1, state: {chart_initialized: true}}}",
			notes: []string{"chart-state-unexplained chart_initialized"}},
		"язык #": {gantt: "{chart: {type: column, active_series: -1, summary_series: {id: 1, text: {\"#\": Сводная}}}, plot_area: {title: {\"#\": Операция}}}",
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
				"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990020, name: График, types: [{kind: gantt-chart}], gantt_chart: "+test.gantt+"}\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента attribute График "
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
