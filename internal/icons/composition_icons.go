package icons

import (
	"fmt"
	"math"
	"strconv"
)

// The pictures of data composition and of the query wizard, drawn from the
// window «Выбор картинки» (docs/materials/block-00/standartnye-kartinki/);
// the settings of a report lay their sign over the table they set, the
// wizard's pictures are its tables. «Конструктор запроса» itself is the
// configurator's command (editors.go). A table, a list, a sheet is always
// the whole picture and its sign stands over it (the owner's rule of
// 16.10.2026).

// fullTable is the table of three columns over the whole grid: the
// settings of a report lay their sign over it, never shrinking it.
func fullTable() []Part {
	return gridTable(1.5, 1.5, 13, 13)
}

// list is a list over the whole grid, its header and rows: what the query
// wizard and the settings wizard build.
func list(header bool) []Part {
	parts := []Part{rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M3.5 7H8M3.5 9.5H10M3.5 12H8")}
	if header {
		return append(parts, colored(box(Accent, 1.5, 1.5, 13, 2.5), steel))
	}
	return append(parts, path(Detail, "M3.5 4.5H9"))
}

// wizardPluses are the three red pluses in the upper right corner of what a
// wizard builds: the sparks of the prototype's wand, which on their own make
// the most used command plain at 16 px (the owner's choice of 16.10.2026).
func wizardPluses() []Part {
	return over(colored(path(Mark, "M10.75 2V5M9.25 3.5H12.25M13.25 6V9M11.75 7.5H14.75M9.75 9V12M8.25 10.5H11.25"), red))
}

// groupingRow is the sign of a grouping: three cells side by side, the
// row of a report's structure that groups it, standing at x, y.
func groupingRow(x, y, width float64) []Part {
	cell, outline, inside := width/3, "", ""
	for index := range 3 {
		left := x + float64(index)*cell
		outline += fmt.Sprintf("M%s %sH%sV%sH%sZ", number(round(left)), number(y), number(round(left+cell)), number(y+3.5), number(round(left)))
		inside += fmt.Sprintf("M%s %sH%sV%sH%sZ", number(round(left+1)), number(y+1), number(round(left+cell-1)), number(y+2.5), number(round(left+1)))
	}
	return []Part{colored(path(Plate, outline), steel), colored(path(Accent, inside), pale), colored(path(Detail, outline), steel)}
}

// newPlus is the green plus at the upper left of what is made anew, laid
// over it: a grouping, a table, a chart, a nested schema of a report, a
// nested query and a description of a temporary table. Bare, not in a
// circle - a circle crowds the picture (the owner's remark of 16.10.2026).
func newPlus() []Part {
	return over(colored(path(Mark, "M4.5 1.5V7.5M1.5 4.5H7.5"), green))
}

// dotted is a line of dots along the sides of a temporary table, from
// point to point across the grid: a dot every two points.
func dotted(points ...[2]float64) string {
	d := ""
	for index := 1; index < len(points); index++ {
		from, to := points[index-1], points[index]
		length := math.Hypot(to[0]-from[0], to[1]-from[1])
		steps := int(length / 2)
		for step := 0; step <= steps; step++ {
			share := float64(step) / float64(steps)
			x, y := from[0]+(to[0]-from[0])*share, from[1]+(to[1]-from[1])*share
			d += fmt.Sprintf("M%s %sh0", number(round(x)), number(round(y)))
		}
	}
	return d
}

// tempTable is a temporary table: the header of a table over sides and
// columns of dots, as the prototype draws what lives only in the query.
func tempTable(x, y, width, height float64) []Part {
	right, bottom := x+width, y+height
	return []Part{
		colored(rect(Accent, x, y, width, 2.5), steel),
		path(Detail, dotted([2]float64{x, y + 3.5}, [2]float64{x, bottom}, [2]float64{right, bottom}, [2]float64{right, y + 3.5})+
			dotted([2]float64{round(x + width/3), y + 3.5}, [2]float64{round(x + width/3), bottom - 1})+
			dotted([2]float64{round(x + width*2/3), y + 3.5}, [2]float64{round(x + width*2/3), bottom - 1})),
	}
}

// tableDescription is what describes a temporary table, its fields of
// three kinds as a blue diamond, a green and a gold disc.
func tableDescription() []Part {
	return over(
		colored(path(Accent, "M5.5 8L8.25 10.75L5.5 13.5L2.75 10.75Z"), blue),
		colored(circle(Accent, 12, 7.5, 2.5), green),
		colored(circle(Accent, 12, 12.5, 2), coin),
	)
}

// nestedQuery is a query in a query: a table standing in front of another.
func nestedQuery() []Part {
	return join([]Part{box(Body, 1.5, 1.5, 9, 8), path(Detail, "M1.5 4H10.5")},
		[]Part{box(Plate, 5.5, 5.5, 9, 9)}, gridTable(5.5, 5.5, 9, 9))
}

// brush is the brush of conditional appearance laid over the table it
// paints, a thick brown handle and red bristles.
func brush() []Part {
	return over(
		colored(path(Accent, "M7.25 7.25L13.25 1L15 2.75L8.75 8.75Z"), brown),
		colored(path(Accent, "M6.75 6.5L9.5 9.25Q9.5 12.75 6.5 14.25Q4 15.25 1 15Q3 13.25 3.5 11Q4 7.25 6.75 6.5Z"), red),
	)
}

// greyed is a colour as the grey of its own lightness, the way the
// prototype draws a picture «…Недоступный».
func greyed(color string) string {
	value, err := strconv.ParseUint(color[1:], 16, 32)
	if err != nil {
		return color
	}
	r, g, b := float64(value>>16&0xff), float64(value>>8&0xff), float64(value&0xff)
	grey := int(math.Round(0.299*r + 0.587*g + 0.114*b))
	return fmt.Sprintf("#%02x%02x%02x", grey, grey, grey)
}

// withDisabled adds to icons the unavailable picture of each named one: the
// same drawing in grey, named "<id>-disabled". The prototype draws its
// pictures «…Недоступный» so, and here the drawing is written once. A solid
// sign turns grey ink, which lightens on a dark theme as ink does, else the
// grey arrow of an order is lost on a dark page (the owner's remark of
// 16.10.2026).
func withDisabled(icons []Icon, ids ...string) []Icon {
	for _, id := range ids {
		for _, icon := range icons {
			if icon.ID != id {
				continue
			}
			parts := make([]Part, len(icon.Parts))
			for index, part := range icon.Parts {
				if part.Color != "" {
					part.Color = greyed(part.Color)
					if part.Role == Accent {
						part.Role = InkSpot
					}
				}
				parts[index] = part
			}
			accent := icon.Accent
			if accent != "" {
				accent = greyed(accent)
			}
			icons = append(icons, Icon{ID: id + "-disabled", Color: greyed(icon.Color), Accent: accent, Parts: parts})
		}
	}
	return icons
}

var compositionIcons = withDisabled([]Icon{
	// Компоновка данных: настройки отчёта поверх таблицы.
	{ID: "dcs-selection", Color: slate, Accent: steel, Parts: join(fullTable(), over(
		colored(bar(6.5, 5, 8, 2.5), blue), colored(bar(6.5, 8.5, 8, 2.5), blue), colored(bar(6.5, 12, 8, 2.5), blue),
	))},
	{ID: "dcs-filter", Color: slate, Accent: steel, Parts: join(fullTable(), over(filterOver(4.5, 1.5, 10)...))},
	{ID: "dcs-output-parameters", Color: slate, Accent: steel, Parts: join(fullTable(), over(tint(gear(10.5, 5.5, 4.75), gray)...))},
	{ID: "dcs-data-parameters", Color: slate, Accent: steel, Parts: join(fullTable(), over(
		colored(path(InkSpot, "M1 6L4.5 2.5V4.75H11.5V2.5L15 6L11.5 9.5V7.25H4.5V9.5Z"), black),
	))},
	{ID: "dcs-user-fields", Color: slate, Accent: steel, Parts: join(fullTable(), over(colored(path(Detail, fx(4, 0.25, 0.75)), navy)))},
	{ID: "dcs-group-fields", Color: slate, Parts: append(groupingRow(1.5, 1.5, 13),
		box(Body, 1.5, 5, 13, 9.5), path(Detail, "M5.83 5V14.5M10.17 5V14.5"),
	)},
	{ID: "dcs-order", Color: slate, Accent: steel, Parts: join(fullTable(), over(colored(path(Accent, downArrow(8, 1.5, 14.5)), blue)))},
	{ID: "dcs-conditional-appearance", Color: slate, Accent: steel, Parts: join(fullTable(), brush())},
	{ID: "dcs-standard-settings", Color: slate, Parts: join([]Part{
		rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M3.5 4.5H8.5M3.5 7H8.5M3.5 9.5H7M3.5 12H6"),
	}, over(
		mound(8.5),
		colored(path(Detail, "M12 10.5V7Q12 4.5 9.5 4.5H7.5M7.5 2.25L4.75 4.5L7.5 6.75Z"), navy),
	))},
	{ID: "dcs-settings-wizard", Color: slate, Parts: join(list(false), wizardPluses())},
	// Структура настроек: новая группировка, таблица, диаграмма, вложенная
	// схема.
	{ID: "dcs-new-group", Color: slate, Parts: join(groupingRow(1.5, 9.5, 13), newPlus())},
	{ID: "dcs-new-table", Color: slate, Accent: steel, Parts: join(fullTable(), newPlus())},
	{ID: "dcs-new-chart", Color: gray, Parts: join([]Part{
		path(Detail, "M1.5 1.5V14.5H14.5"),
		colored(rect(Accent, 3.5, 9.5, 2, 4.5), blue), colored(rect(Accent, 6.5, 6.5, 2, 7.5), yellow),
		colored(rect(Accent, 9.5, 10, 2, 4), brown), colored(rect(Accent, 12.5, 2.5, 2, 11.5), red),
	}, newPlus())},
	{ID: "dcs-new-nested-schema", Color: slate, Parts: join([]Part{
		path(Body, "M5.5 4.5H11.5L14.5 7.5V14.5H5.5Z"),
		path(Plate, "M1.5 1.5H8.5L11.5 4.5V12.5H1.5Z"), path(Body, "M1.5 1.5H8.5L11.5 4.5V12.5H1.5Z"), path(Detail, "M8.5 1.5V4.5H11.5"),
	}, newPlus())},
	// Конструктор запроса.
	{ID: "query-nested-query", Color: slate, Accent: steel, Parts: nestedQuery()},
	{ID: "query-create-nested-query", Color: slate, Accent: steel, Parts: join(nestedQuery(), newPlus())},
	{ID: "query-temp-table", Color: slate, Parts: tempTable(1.5, 1.5, 13, 13)},
	{ID: "query-temp-tables-group", Color: slate, Parts: join(tempTable(1.5, 1.5, 13, 13), []Part{
		colored(path(Plate, "M6.5 8.5H9.5L10.5 9.5H14.5V14.5H6.5Z"), groupColor),
		colored(path(Body, "M6.5 8.5H9.5L10.5 9.5H14.5V14.5H6.5Z"), groupColor),
		colored(rect(Accent, 7, 11, 7, 1.25), yellow),
	})},
	{ID: "query-create-temp-table-drop-query", Color: slate, Parts: join(tempTable(1.5, 1.5, 13, 13), over(
		colored(path(Mark, "M10.5 10.5L14.5 14.5M14.5 10.5L10.5 14.5"), red),
	))},
	{ID: "query-temp-table-description", Color: slate, Parts: join(tempTable(1.5, 1.5, 13, 13), tableDescription())},
	{ID: "query-create-temp-table-description", Color: slate, Parts: join(tempTable(1.5, 1.5, 13, 13), tableDescription(), newPlus())},
	{ID: "query-replace-table", Color: gray, Accent: gray, Parts: join(gridTable(1.5, 1.5, 7, 8), []Part{
		box(Plate, 7.5, 6.5, 7, 8),
	}, gridTable(7.5, 6.5, 7, 8), []Part{
		colored(path(Mark, "M2 11L5.5 14.5M5.5 11L2 14.5"), red),
		colored(path(Mark, "M9 3L10.75 4.75L14.5 1"), green),
	})},
	// The pencil stands in the middle of the plan, between its nodes.
	{ID: "query-show-changes-tables", Color: blue, Accent: green, Parts: join(exchangePlan(), over(
		colored(path(Body, "M4.5 12.5L5.1 10.1L9.75 5.45L11.55 7.25L6.9 11.9Z"), "#6f9a2a"),
	))},
	{ID: "query-table-parameters", Color: "#6b7178", Parts: join([]Part{
		box(Body, 1.5, 1.5, 13, 13), path(Detail, "M1.5 5H14.5M1.5 8.5H14.5M1.5 12H14.5M6 1.5V14.5"),
	}, tint(gear(10.5, 10.5, 4.75), gray))},
}, "dcs-selection", "dcs-filter", "dcs-output-parameters", "dcs-group-fields", "dcs-order", "dcs-conditional-appearance")
