package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
)

// chartWhole is a chart whose content sets every property of the model, each
// to something other than its zero.
const chartWhole = `    chart:
      type: stacked-column
      series:
        - {id: 2, text: {ru: Себестоимость, "#": Сводная}, color: {source: absolute, rgb: "#D02A35"}, color_priority: true, indicator: true,
           line: {style: solid, width: 2, gap: true}, marker: rhomb, show_in_chart: show, text_changed: true, expand: true}
      series_count: 2
      points:
        - {id: 3, text: {ru: "2014"}, color: {source: web, name: DarkGreen}, color_priority: true, indicator: true,
           line: {style: dotted, width: 1, gap: true}, marker: rect, show_in_chart: dont-show, text_changed: true, expand: true}
      point_count: 2
      summary_series: {id: 1, text: {"#": Сводная}, color: {source: auto}, color_priority: true, indicator: true,
           line: {style: dashed, width: 3, gap: true}, marker: auto, show_in_chart: auto, text_changed: true, expand: true}
      values: [{value: "9.04", tool_tip: Себестоимость}, {value: "-1.6", tool_tip: Прибыль}, {value: "18.11", tool_tip: Себестоимость}, {value: "2.68", tool_tip: Прибыль}]
      active_series: 1
      active_point: 1
      next_series_id: 3
      next_point_id: 1543
      auto_series_text: true
      auto_point_text: true
      labels:
        type: series-value-percent
        delimiter: ","
        location: edge
        value_format: {ru: "ЧДЦ=2"}
        percent_format: {ru: "ЧДЦ=0"}
        text_color: {source: style, from: {standard: FormTextColor}}
        font: {source: style, from: {standard: SmallTextFont}, face: Roboto, size: 9, scale: 100, bold: true, italic: false, underline: false, strikeout: false}
        transparent: true
        back_color: {source: auto}
        border: {source: absolute, line: single, width: 1}
        border_color: {source: system, name: ButtonFace}
      title_area:
        text: {ru: Продажи}
        shown: true
        placement: use-coordinates
        transparent: true
        back_color: {source: absolute, rgb: "#FFFFFF"}
        text_color: {source: web, name: DarkGreen}
        font: {source: auto}
        border: {source: absolute, line: none, width: 0}
        border_color: {source: style, from: {standard: BorderColor}}
        bounds: {left: "0", top: "0.0017825311942959", right: "0.36942675159235666", bottom: "0.92"}
      legend_area:
        shown: true
        placement: bottom
        scrolling: true
        transparent: true
        back_color: {source: absolute, rgb: "#FFFFFF"}
        text_color: {source: absolute, rgb: "#333333"}
        font: {source: style, from: {standard: TextFont}}
        border: {source: absolute, line: single, width: 1}
        border_color: {source: absolute, rgb: "#A0A0A0"}
        bounds: {left: "0.83", top: "0.08", right: "0.017094017094017094017094017", bottom: "0.208588957055214723926380368"}
      plot_area:
        placement: use-coordinates
        transparent: true
        back_color: {source: absolute, rgb: "#FFFFFF"}
        text_color: {source: absolute, rgb: "#333333"}
        font: {source: style, from: {standard: NormalTextFont}, size: 6}
        border: {source: absolute, line: none, width: 0}
        border_color: {source: absolute, rgb: "#A0A0A0"}
        bounds: {left: "0.114401076716016150740242261", top: "0.0266429840142096", right: "0.17", bottom: "0.214912280701754"}
        show_scales: true
        scale_lines: {style: solid, width: 1}
        scale_color: {source: absolute, rgb: "#A9A9A9"}
        surface_color: {source: absolute, rgb: "#A90000"}
        radar_scale_type: circle
        show_data_table: true
        vertical_lines_data_table: true
        horizontal_lines_data_table: true
        keys_in_data_table: true
        align_data_table: right
        data_table_format: "ЧДЦ=2"
        show_series_scale: true
        show_points_scale: true
        show_values_scale: true
        points_scale: &scale
          grid_lines_show_mode: dont-show
          label_angle: -66
          label_font: {source: style, from: {standard: TextFont}, size: 5}
          label_format: {ru: "ЧГ=0"}
          label_orientation: custom-angle
          max_label_rows: 1
          scale_label_location: outside
          scale_line: {style: solid, width: 1}
          scale_location: base-value
          scale_mark_location: inside
          show_in_chart: dont-show
          show_title: show
          title_placement: plot-area
          title_text: {ru: Выполнено, en: Completed}
          title_text_mode: use-text
          title_area: {back_color: {source: auto}, text_color: {source: auto}, font: {source: auto}, border: {source: absolute, line: none, width: 0}, border_color: {source: auto}}
        series_scale: *scale
        values_scale: *scale
      transparent: true
      back_color: {source: style, from: {standard: FieldBackColor}}
      border: {source: absolute, line: single, width: 1}
      border_color: {source: style, from: {standard: BorderColor}}
      max_series: limited
      max_series_count: 4
      max_series_percent: 30
      auto_max_value: true
      max_value: "100"
      auto_min_value: true
      min_value: "-5.5"
      base_value: "1"
      hide_base_value: true
      space_mode: half
      auto_series_separation: maximum
      orientation: south-west
      outline: true
      light: true
      gradient: true
      auto_transposition: true
      animation: use
      selection_mode: none
      color_palette: palette-32
      color_palette_description: palette-8
      spline_mode: smooth-curve
      spline_strain: 95
      semitransparency_percent: 100
      donut_chart_inner_radius: 55
      funnel_neck_height: 10
      funnel_neck_width: 10
      funnel_space: 3
      gauge:
        value_representation: needle
        begin_angle: 10
        end_angle: 180
        thickness: 5
        labels_location: inside-scale
        labels_arc_direction: true
        bush_thickness: 4
        bush_color: {source: absolute, rgb: "#A9A9A9"}
        quality_bands_use_text: true
        quality_bands_use_tool_tip: true
      data_source_mode: true
      state:
        rebuild_time: 4086202
        elements_initialized: true
        title_initialized: true
        legend_initialized: true
        chart_initialized: true
        randomized_new_values: true
        series_design: true
        points_design: true
        transposition: true
        transposed: true
        show_scale_vl: true
        values_scale_format: "ЧДЦ=1"
        x_labels_orientation: vertical
        pie_point: 1
        stock_series: 1
        multi_stage_link_line: {style: solid, width: 1}
        multi_stage_link_color: {source: absolute, rgb: "#000000"}
        points_drop_lines: show
`

func chartForm(chart string) string {
	return formAttrHead + "attributes:\n  - id: c0de0000-0000-4000-8000-000000990020\n    name: Диаграмма\n    types: [{kind: chart}]\n" + chart
}

// zeroFields names every field of a value, however deep, left at its zero.
func zeroFields(path string, value reflect.Value) []string {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return []string{path}
		}
		// A colour, a font, a border and a line are values of their own with
		// ways that exclude one another; that one is there is enough.
		switch value.Interface().(type) {
		case *ColorValue, *FontValue, *BorderValue, *RouteStroke:
			return nil
		}
		return zeroFields(path, value.Elem())
	case reflect.Struct:
		var zero []string
		for index := range value.NumField() {
			if field := value.Type().Field(index); field.IsExported() {
				zero = append(zero, zeroFields(path+"."+field.Name, value.Field(index))...)
			}
		}
		return zero
	case reflect.Slice, reflect.Map:
		if value.Len() == 0 {
			return []string{path}
		}
		if value.Kind() == reflect.Slice {
			var zero []string
			for index := range value.Len() {
				zero = append(zero, zeroFields(fmt.Sprintf("%s[%d]", path, index), value.Index(index))...)
			}
			return zero
		}
		return nil
	}
	if value.IsZero() {
		return []string{path}
	}
	return nil
}

// The content of a chart comes back whole through YAML and the Studio: every
// property of the model, set in the source, is set in what is read back.
//
// Defect caught: a property of the chart, of a series, a point, an area, a
// scale or the gauge lost on the way, so that the chart arrives drawn
// otherwise than the developer drew it; the place of an area cut to a float
// (the prototype writes 27 digits); a property added to the model and left
// out of this test - the source sets every field, which the test checks
// before anything else.
func TestAChartKeepsItsContent(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(chartForm(chartWhole)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	chart := form.Attributes[0].Chart
	if zero := zeroFields("chart", reflect.ValueOf(chart)); len(zero) != 0 {
		t.Fatalf("the source leaves out: %v", zero)
	}
	if chart.PlotArea.Bounds.Left != "0.114401076716016150740242261" || chart.Values[1].Value != "-1.6" {
		t.Fatalf("numbers: %+v %+v", chart.PlotArea.Bounds, chart.Values)
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
	if !reflect.DeepEqual(again.Attributes[0].Chart, chart) || !reflect.DeepEqual(received.Attributes[0].Chart, chart) {
		t.Fatalf("the chart changed on the way:\n%+v\n%+v", again.Attributes[0].Chart, received.Attributes[0].Chart)
	}
}

// What is wrong in the content of a chart is refused, naming the place.
//
// Defect caught: the prototype's spelling of a value (Column3D) taken as
// written; a value of no list taken; values that do not fill the series by
// the points; a series described beyond the count; a number that is none; a
// series or a point the chart stands on that it does not have; a font of no
// face but a face; a text keyed by no language; content on an attribute that
// is no chart.
func TestAChartRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	chart := func(rest string) string {
		return chartForm("    chart:\n      type: pie\n      active_series: -1\n      active_point: 0\n" + rest)
	}
	for name, test := range map[string]struct{ source, want string }{
		"тип прототипа":   {chart("      labels: {type: SeriesValue}\n"), "attributes[0].chart.labels.type must be one of none,"},
		"тип диаграммы":   {chartForm("    chart: {type: Column3D, active_series: -1}\n"), "attributes[0].chart.type must be one of area,"},
		"без типа":        {chartForm("    chart: {active_series: -1}\n"), "attributes[0].chart.type must be one of"},
		"значения":        {chart("      series_count: 2\n      point_count: 2\n      values: [{value: \"1\"}, {value: \"2\"}]\n"), "attributes[0].chart.values must hold one value for each series and point, 4"},
		"серия сверх":     {chart("      series: [{id: 1}]\n"), "attributes[0].chart.series_count must count every series described"},
		"точка сверх":     {chart("      points: [{id: 1}]\n"), "attributes[0].chart.point_count must count every point described"},
		"не число":        {chart("      series_count: 1\n      point_count: 1\n      values: [{value: \"1,5\"}]\n"), "attributes[0].chart.values[0].value must be a decimal number"},
		"граница":         {chart("      title_area: {bounds: {left: \"0.1e3\", top: \"0\", right: \"0\", bottom: \"0\"}}\n"), "attributes[0].chart.title_area.bounds.left must be a decimal number"},
		"активная серия":  {chartForm("    chart: {type: pie, active_series: 2, active_point: 0, series_count: 2}\n"), "attributes[0].chart.active_series must be the number of a series"},
		"активная точка":  {chartForm("    chart: {type: pie, active_series: -1, active_point: -2}\n"), "attributes[0].chart.active_point must be the number of a point"},
		"шрифт авто":      {chart("      legend_area: {font: {source: auto, face: Roboto}}\n"), "attributes[0].chart.legend_area.font.face belongs to a font named by its face or based on another"},
		"язык":            {chart("      title_area: {text: {\"d=e\": Продажи}}\n"), "attributes[0].chart.title_area.text.d=e is not a language code"},
		"маркер":          {chart("      series_count: 1\n      series: [{id: 1, marker: star}]\n"), "attributes[0].chart.series[0].marker must be one of"},
		"линия":           {chart("      plot_area: {scale_lines: {style: wavy}}\n"), "attributes[0].chart.plot_area.scale_lines.style is not a way a line is drawn"},
		"шкала":           {chart("      plot_area: {values_scale: {label_angle: 400}}\n"), "attributes[0].chart.plot_area.values_scale.label_angle must be an angle in degrees"},
		"процент":         {chart("      max_series_percent: 101\n"), "attributes[0].chart.max_series_percent must be a percentage"},
		"отрицательное":   {chart("      funnel_space: -1\n"), "attributes[0].chart.funnel_space must not be negative"},
		"угол":            {chart("      gauge: {end_angle: 361}\n"), "attributes[0].chart.gauge.end_angle must be an angle in degrees"},
		"состояние":       {chart("      state: {x_labels_orientation: Vertical}\n"), "attributes[0].chart.state.x_labels_orientation must be one of"},
		"цвет":            {chart("      back_color: {source: absolute, rgb: white}\n"), "attributes[0].chart.back_color.rgb must be a colour written as #RRGGBB"},
		"рамка":           {chart("      labels: {border: {source: absolute, line: single, width: 6}}\n"), "attributes[0].chart.labels.border.width must be between 0 and 5"},
		"не диаграмма":    {formAttrHead + "attributes:\n  - id: c0de0000-0000-4000-8000-000000990020\n    name: Таблица\n    types: [{kind: value-table}]\n    chart: {type: pie, active_series: -1}\n", "attributes[0].chart belongs to an attribute that is a chart"},
		"шрифт со стилем": {chart("      legend_area: {font: {source: style, from: {standard: TextFont}, face: Roboto}}\n"), ""},
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

// What the content of a chart takes from the configuration is resolved at
// load, and what the model carries without knowing what it is, is noted.
//
// Defect caught: a colour, a font or a border taken from a style item the
// configuration no longer has loading clean - deep in a scale or a series as
// much as on top; a colour taken from a style item that is a font accepted;
// the state of the prototype's own object, the values of unknown order or a
// text in "#" carried with no note; a note on a chart that has none of them.
func TestWhatAChartTakesIsResolved(t *testing.T) {
	t.Parallel()
	const font, color = "c0de0000-0000-4000-8000-000000990031", "c0de0000-0000-4000-8000-000000990032"
	for name, test := range map[string]struct {
		chart   string
		gone    string
		refused string
		notes   []string
	}{
		"чистая": {chart: "{type: pie, active_series: -1, labels: {font: {source: style, from: {item: " + font + "}}}, plot_area: {values_scale: {title_area: {back_color: {source: style, from: {item: " + color + "}}}}}}"},
		"элемента стиля нет на верху": {chart: "{type: pie, active_series: -1, back_color: {source: style, from: {item: " + refGone + "}}}",
			gone: "chart back_color"},
		"элемента стиля нет в шкале": {chart: "{type: pie, active_series: -1, plot_area: {points_scale: {label_font: {source: style, from: {item: " + refGone + "}}}}}",
			gone: "chart plot_area.points_scale.label_font"},
		"элемента стиля нет в серии": {chart: "{type: pie, active_series: -1, series_count: 1, series: [{id: 1, color: {source: style, from: {item: " + refGone + "}}}]}",
			gone: "chart series[0].color"},
		"цвет из шрифта": {chart: "{type: pie, active_series: -1, legend_area: {text_color: {source: style, from: {item: " + font + "}}}}",
			refused: "chart legend_area.text_color takes its value from style item Мелкий, which is a font and not a color"},
		"состояние": {chart: "{type: pie, active_series: -1, state: {rebuild_time: 4086202, chart_initialized: true}}",
			notes: []string{"chart-state-unexplained rebuild_time, chart_initialized"}},
		"значения": {chart: "{type: line, active_series: -1, series_count: 2, point_count: 1, values: [{value: \"1\"}, {value: \"2\"}]}",
			notes: nil},
		"язык #": {chart: "{type: pie, active_series: -1, summary_series: {id: 1, text: {\"#\": Сводная}}, labels: {value_format: {ru: \"ЧДЦ=\", \"#\": \"ЧДЦ=\"}}}",
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
				"attributes:\n  - {id: c0de0000-0000-4000-8000-000000990020, name: Диаграмма, types: [{kind: chart}], chart: "+test.chart+"}\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента attribute Диаграмма "
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

// The values of a chart are read series by series: the list the prototype
// writes holds all the points of the first series, then of the second. The
// chart is the one of the run on the platform (08.10.2026): 2 series × 3
// points, the value of series S at point T is ST and its tooltip names it.
//
// Defect caught: the list read point by point - the value of the second
// series at the first point taken from the second place of the list, 12 in
// place of 21; a series or a point out of the chart read from the list.
func TestTheValuesOfAChartRunSeriesBySeries(t *testing.T) {
	t.Parallel()
	source := formAttrHead + "attributes:\n  - id: c0de0000-0000-4000-8000-000000990020\n    name: Диаграмма\n    types: [{kind: chart}]\n" +
		"    chart: {type: column, active_series: -1, series_count: 2, point_count: 3, values: [" +
		"{value: \"11\", tool_tip: С1Т1}, {value: \"12\", tool_tip: С1Т2}, {value: \"13\", tool_tip: С1Т3}, " +
		"{value: \"21\", tool_tip: С2Т1}, {value: \"22\", tool_tip: С2Т2}, {value: \"23\", tool_tip: С2Т3}]}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), managedFormConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	chart := form.Attributes[0].Chart
	for series := range 2 {
		for point := range 3 {
			value, ok := chart.ValueAt(series, point)
			want := fmt.Sprintf("%d%d", series+1, point+1)
			if !ok || string(value.Value) != want || value.ToolTip != fmt.Sprintf("С%dТ%d", series+1, point+1) {
				t.Errorf("series %d point %d = %+v %v, want %s", series, point, value, ok, want)
			}
		}
	}
	for _, outside := range [][2]int{{-1, 0}, {2, 0}, {0, -1}, {0, 3}} {
		if value, ok := chart.ValueAt(outside[0], outside[1]); ok {
			t.Errorf("series %d point %d is outside the chart and reads %+v", outside[0], outside[1], value)
		}
	}
	empty := ChartContent{SeriesCount: 2, PointCount: 3}
	if _, ok := empty.ValueAt(0, 0); ok {
		t.Error("a chart that holds no values reads one")
	}
}
