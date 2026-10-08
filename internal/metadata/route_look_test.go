package metadata

import (
	"strings"
	"testing"
)

// The drawing of a map travels whole: the background, the grid and the print
// settings of the map, the look of every point, the name, caption, ports,
// arrows and moved segments of every line, the shapes and the decorative lines
// attached to them, the captions and colours of the variants. A map that
// arrives with all of that reset is one the developer has to draw again.
func TestRouteMapCarriesItsDrawing(t *testing.T) {
	t.Parallel()
	root := routeOnlyProcess(t, `
  look:
    back_color: {source: style, from: {standard: FieldBackColor}}
    grid: true
    grid_mode: lines
    grid_horizontal_step: 15
    grid_vertical_step: 20
    print: {top_margin: 10, left_margin: 10, bottom_margin: 10, right_margin: 10, fit_page: auto}
  points:
    - id: b0000000-0000-4000-8000-000000000030
      name: Старт
      kind: start
      look:
        tooltip: {ru: Начало}
        tab_order: 1
        z_order: 2
        back_color: {source: auto}
        text_color: {source: style, from: {standard: FormTextColor}}
        line_color: {source: absolute, rgb: "#000000"}
        font: {source: system, system: DefaultGUIFont}
        horizontal_align: left
        vertical_align: center
        picture_location: left
        transparent: true
        border: {style: solid, width: 1}
        picture: {standard: BusinessProcessStart}
        picture_size: auto-size
    - {id: b0000000-0000-4000-8000-000000000032, name: Проверка, kind: condition, true_port: 1, false_port: 3}
    - id: b0000000-0000-4000-8000-000000000037
      name: Выбор
      kind: variant-choice
      variants:
        - {name: Согласовать, title: {ru: Согласовать}, back_color: {source: web, name: LightGreen}}
        - {name: Отклонить}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - name: Линия1
      title: {ru: Да}
      from: Старт
      to: Проверка
      from_port: 4
      to_port: 2
      vertices: [{x: 400, y: 60}, {x: 400, y: 100}]
      look: {font: {source: auto}, tab_order: 2}
      line_look:
        stroke: {style: dashed, width: 1}
        begin_arrow: none
        end_arrow: blank
        text_location: middle
        segments: [{index: 1, start: {x: 240, y: 720}, end: {x: 240, y: 860}}]
    - {name: Линия2, from: Проверка, to: Выбор, branch: "true", from_port: 1}
    - {name: Линия3, from: Проверка, to: Завершение, branch: "false", from_port: 3}
    - {from: Выбор, to: Завершение, branch: Согласовать}
    - {from: Выбор, to: Завершение, branch: Отклонить}
  decorations:
    - name: Пояснение
      title: {ru: постановка задачи}
      shape: Document
      flip_mode: 1
      angle: 90
      location: {top: 100, left: 20, bottom: 160, right: 140}
      look: {transparent: true, picture: {standard: Information}, picture_size: stretch}
    - name: Указатель
      line: [{x: 140, y: 130}, {x: 340, y: 130}]
      from: {item: Пояснение, port: 3}
      to: {item: Старт, port: 5}
      line_look: {stroke: {style: dotted}, end_arrow: filled}
    - name: Черта
      line: [{x: 20, y: 200}, {x: 300, y: 200}]
      line_look: {stroke: {style: solid}}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	process, _ := catalog.BusinessProcess("Задание")
	route := process.Route
	if route.Look == nil || route.Look.GridMode != LinesGrid || route.Look.GridHorizontalStep != 15 || route.Look.Print == nil ||
		route.Look.Print.TopMargin != 10 || route.Look.BackColor == nil || route.Look.BackColor.From.Standard != "FieldBackColor" {
		t.Fatalf("the map lost its grid, background or print settings: %+v", route.Look)
	}
	start := route.Points[0].Look
	if start == nil || start.TabOrder != 1 || start.ZOrder != 2 || start.Font == nil || start.Font.System != "DefaultGUIFont" ||
		start.LineColor.RGB != "#000000" || start.Border == nil || start.Border.Style != SolidStroke || start.Picture == nil ||
		start.HorizontalAlign != "left" || !start.Transparent || start.ToolTip["ru"] != "Начало" {
		t.Fatalf("a point lost its look: %+v", start)
	}
	if condition := route.Points[1]; condition.TruePort != 1 || condition.FalsePort != 3 {
		t.Fatalf("a condition lost its ports: %+v", condition)
	}
	if variants := route.Points[2].Variants; len(variants) != 2 || variants[0].Title["ru"] != "Согласовать" || variants[0].BackColor.Name != "LightGreen" {
		t.Fatalf("a variant lost its caption or colour: %+v", variants)
	}
	line := route.Transitions[0]
	if line.Name != "Линия1" || line.Title["ru"] != "Да" || line.FromPort != 4 || line.ToPort != 2 || line.Look == nil ||
		line.LineLook == nil || line.LineLook.Stroke.Style != DashedStroke || line.LineLook.EndArrow != BlankArrow ||
		line.LineLook.TextLocation != MiddleText || len(line.LineLook.Segments) != 1 || line.LineLook.Segments[0].End.Y != 860 {
		t.Fatalf("a line lost its drawing: %+v", line)
	}
	shape, pointer := route.Decorations[0], route.Decorations[1]
	if shape.FlipMode != 1 || shape.Angle != 90 || shape.Look == nil || shape.Look.PictureSize != "stretch" {
		t.Fatalf("a shape lost its turn or picture: %+v", shape)
	}
	if pointer.From == nil || pointer.From.Item != "Пояснение" || pointer.To.Port != 5 || pointer.LineLook.Stroke.Style != DottedStroke {
		t.Fatalf("a decorative line lost what it is attached to: %+v", pointer)
	}
	// What the catalog hands out is a copy.
	start.Font.Face = "Другой"
	route.Transitions[0].LineLook.Segments[0].End.Y = 0
	again, _ := catalog.BusinessProcess("Задание")
	if again.Route.Points[0].Look.Font.System != "DefaultGUIFont" || again.Route.Transitions[0].LineLook.Segments[0].End.Y != 860 {
		t.Fatal("the drawing handed out is shared with the catalog")
	}
}

// Each of these is a drawing the designer does not make: a branch leaving by
// the port of the other, ports on a point without branches, a border on a
// line, a decoration that is both a shape and a line, a line attached to
// nothing, two items of one name, values outside the prototype's lists.
func TestRouteMapRefusesADrawingTheDesignerDoesNotMake(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ condition, line, decoration, look, want string }{
		"ветка через чужой порт": {condition: `true_port: 1, false_port: 3`, line: `from_port: 3`,
			want: "leaves the condition by the port of the other branch"},
		"одинаковые порты": {condition: `true_port: 1, false_port: 1`, want: "two different ports"},
		"рамка у линии":    {line: `look: {border: {style: solid}}`, want: "only a box on the map has"},
		"рамка у фигуры": {decoration: `{name: Фигура, shape: Ellipse, location: {top: 0, left: 0, bottom: 10, right: 10}, look: {border: {style: solid}}}`,
			want: "the outline of a shape is the shape"},
		"фигура и линия": {decoration: `{name: Фигура, shape: Ellipse, location: {top: 0, left: 0, bottom: 10, right: 10}, line: [{x: 0, y: 0}, {x: 1, y: 1}]}`,
			want: "both a shape and a decorative line"},
		"привязка в никуда": {decoration: `{name: Указатель, line: [{x: 0, y: 0}, {x: 1, y: 1}], from: {item: Нигде, port: 1}}`,
			want: "which is not a point or a shape of this map"},
		"привязка к линии": {line: `name: Линия1`, decoration: `{name: Указатель, line: [{x: 0, y: 0}, {x: 1, y: 1}], to: {item: Линия1}}`,
			want: "which is not a point or a shape of this map"},
		"линия с именем точки": {line: `name: Проверка`, want: "are both named Проверка"},
		"чужой стиль линии":    {line: `line_look: {stroke: {style: wavy}}`, want: "is not a way a line is drawn"},
		"чужая стрелка":        {line: `line_look: {end_arrow: open}`, want: "end_arrow must be none, filled or blank"},
		"чужое место подписи":  {line: `line_look: {text_location: last}`, want: "text_location must be first-segment or middle"},
		"чужое выравнивание":   {line: `look: {horizontal_align: middle}`, want: "horizontal_align must be one of"},
		"чужая фигура": {decoration: `{name: Фигура, shape: Star, location: {top: 0, left: 0, bottom: 10, right: 10}}`,
			want: "shape must be one of"},
		"повтор сегмента": {line: `line_look: {segments: [{index: 1, start: {x: 0, y: 0}, end: {x: 1, y: 1}}, {index: 1, start: {x: 0, y: 0}, end: {x: 1, y: 1}}]}`,
			want: "must be a segment of the line, once"},
		"цвет без значения": {line: `look: {text_color: {source: absolute}}`, want: ".rgb must be a colour written as #RRGGBB"},
		"шрифт номером":     {line: `look: {font: {source: style, from: {written: "0"}}}`, want: ".font.from.written belongs to a form only"},
		"чужая сетка":       {look: `grid_mode: squares`, want: "grid_mode must be none, dots, chess or lines"},
		"чужой фон карты":   {look: `back_color: {source: paint}`, want: "route.look.back_color.source must be"},
		"фон карты номером": {look: `back_color: {source: style, from: {written: "0"}}`, want: "route.look.back_color.from.written belongs to a form only"},
		"цвет номером":      {line: `look: {text_color: {source: style, from: {written: "0"}}}`, want: ".text_color.from.written belongs to a form only"},
		"оформление точки":  {condition: `look: {vertical_align: middle}`, want: "route.points[1].look.vertical_align must be one of"},
		"оформление декоративной линии": {decoration: `{name: Черта, line: [{x: 0, y: 0}, {x: 1, y: 1}], line_look: {stroke: {style: wavy}}}`,
			want: "route.decorations[0].line_look.stroke.style"},
		"оформление фигуры": {decoration: `{name: Фигура, shape: Ellipse, location: {top: 0, left: 0, bottom: 10, right: 10}, look: {picture_size: huge}}`,
			want: "route.decorations[0].look.picture_size must be one of"},
		"чужое вписывание": {look: `print: {fit_page: shrink}`, want: "fit_page must be auto, page-width or proportionally"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			decorations := ""
			if broken.decoration != "" {
				decorations = "\n  decorations:\n    - " + broken.decoration
			}
			look := ""
			if broken.look != "" {
				look = "\n  look: {" + broken.look + "}"
			}
			root := routeOnlyProcess(t, look+`
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000032, name: Проверка, kind: condition, `+broken.condition+`}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Проверка}
    - {from: Проверка, to: Завершение, branch: "true", `+broken.line+`}
    - {from: Проверка, to: Завершение, branch: "false"}`+decorations)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("error = %v, want one saying %q", err, broken.want)
			}
		})
	}
}

// Ports belong to a condition, and a variant is named by what its handler
// returns. Two lines may not share a name either.
func TestRoutePointLookRefusesWhatItsKindHasNot(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ point, lines, want string }{
		"порты у действия": {point: `{id: b0000000-0000-4000-8000-000000000031, name: Шаг, kind: processing, true_port: 1, false_port: 3}`,
			lines: `
    - {from: Старт, to: Шаг}
    - {from: Шаг, to: Завершение}`, want: "only a condition has"},
		"вариант без имени": {point: `{id: b0000000-0000-4000-8000-000000000031, name: Шаг, kind: variant-choice, variants: [{name: "1а"}, {name: Б}]}`,
			lines: `
    - {from: Старт, to: Шаг}
    - {from: Шаг, to: Завершение, branch: Б}`, want: "the handler returns the variant by it"},
		"цвет варианта": {point: `{id: b0000000-0000-4000-8000-000000000031, name: Шаг, kind: variant-choice, variants: [{name: А, back_color: {source: web}}, {name: Б}]}`,
			lines: `
    - {from: Старт, to: Шаг}
    - {from: Шаг, to: Завершение, branch: Б}`, want: "variants[0].back_color.name must name a colour"},
		"цвет варианта номером": {point: `{id: b0000000-0000-4000-8000-000000000031, name: Шаг, kind: variant-choice, variants: [{name: А, back_color: {source: style, from: {written: "0"}}}, {name: Б}]}`,
			lines: `
    - {from: Старт, to: Шаг}
    - {from: Шаг, to: Завершение, branch: Б}`, want: "variants[0].back_color.from.written belongs to a form only"},
		"две линии одного имени": {point: `{id: b0000000-0000-4000-8000-000000000031, name: Шаг, kind: processing}`,
			lines: `
    - {name: Линия1, from: Старт, to: Шаг}
    - {name: Линия1, from: Шаг, to: Завершение}`, want: "route.transitions[1].name must be unique"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := routeOnlyProcess(t, `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - `+broken.point+`
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:`+broken.lines)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("error = %v, want one saying %q", err, broken.want)
			}
		})
	}
}
