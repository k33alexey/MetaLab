package icons

import (
	"fmt"
	"math"
)

// The pictures of a spreadsheet document and of the drawings it holds, drawn
// from the window «Выбор картинки» (docs/materials/block-00/standartnye-kartinki/).
// The configurator's editor of a spreadsheet shows the same pictures on the
// commands of its menu «Таблица» - Вид, Примечания, Настройки печати,
// Рисунки (docs/block-00/redaktor-tablichnogo.md) - so a command of that
// editor takes its icon from here. The sheet of cells is always the whole
// picture and a sign stands over it (the owner's rule of 16.10.2026).

// cells is a spreadsheet over the whole grid: its cells in rows and
// columns, the narrow first column the headers of its rows.
func cells() []Part {
	return []Part{
		box(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M5.5 1.5V14.5M10.5 1.5V14.5M1.5 4.5H14.5M1.5 7.5H14.5M1.5 10.5H14.5"),
	}
}

// commentedCells is the sheet with its current cell in green at the lower
// left and the note of a comment laid over its upper right.
func commentedCells() []Part {
	return join(cells(), []Part{colored(box(Accent, 1.5, 10.5, 4, 4), green)}, note(6.5, 1.5, 8))
}

// note is the note of a comment, a framed card of size points at x, y with
// two lines of its text: alone over a frame it shows the comments, over a
// sheet it is inserted or deleted.
func note(x, y, size float64) []Part {
	first, second := thirds(y, size)
	return []Part{
		box(Plate, x, y, size, size),
		colored(box(Body, x, y, size, size), steel),
		colored(path(Detail, fmt.Sprintf("M%s %sH%sM%s %sH%s",
			number(x+2), number(first), number(x+size-2),
			number(x+2), number(second), number(x+size-3))), steel),
	}
}

// pageBreak is the sheet with its lower right corner turned: the page it
// breaks into.
func pageBreak() []Part {
	return []Part{
		path(Body, "M1.5 1.5H14.5V10.5L10.5 14.5H1.5Z"),
		path(Detail, "M5.5 1.5V14.5M10.5 1.5V14.5M1.5 4.5H14.5M1.5 7.5H14.5M1.5 10.5H14.5M10.5 14.5V10.5"),
	}
}

// struckOut is the red cross at the upper left of what is deleted from a
// sheet: a comment, a break of the page.
func struckOut() []Part {
	return []Part{colored(path(Mark, "M2 2L7 7M7 2L2 7"), red)}
}

// groupBox is the button of a group at the left of a sheet, a box with its
// minus or plus.
func groupBox(x, y float64, sign string) []Part {
	return []Part{box(Plate, x, y, 4, 4), box(Body, x, y, 4, 4), colored(path(Detail, sign), ink)}
}

// chartBars are the bars of a chart, green, blue, yellow, brown and red,
// standing on bottom and rising at most height: a chart alone, a new chart
// of a report and the chart of a pivot table.
func chartBars(bottom, height float64) []Part {
	bars := []struct {
		x, share float64
		color    string
	}{{2, 0.8, green}, {4, 0.3, blue}, {7, 0.65, yellow}, {9, 0.45, brown}, {12, 1, red}}
	parts := make([]Part, 0, len(bars))
	for _, bar := range bars {
		top := math.Round(bottom - height*bar.share)
		parts = append(parts, colored(box(Accent, bar.x, top, 2, bottom-top), bar.color))
	}
	return parts
}

// chart is the bars over the two axes of a chart.
func chart() []Part {
	return append([]Part{path(Detail, "M1.5 1.5V14.5H14.5")}, chartBars(14, 11)...)
}

var spreadsheetIcons = []Icon{
	// Табличный документ: примечания, разрывы страниц, вид.
	{ID: "spreadsheet-insert-comment", Color: gray, Parts: commentedCells()},
	{ID: "spreadsheet-delete-comment", Color: gray, Parts: join(commentedCells(), struckOut())},
	{ID: "spreadsheet-show-comments", Color: gray, Parts: join([]Part{box(Body, 1.5, 6.5, 8, 8)}, note(5.5, 1.5, 9))},
	{ID: "spreadsheet-insert-page-break", Color: gray, Parts: pageBreak()},
	{ID: "spreadsheet-delete-page-break", Color: gray, Parts: join(pageBreak(), struckOut())},
	{ID: "spreadsheet-show-grid", Color: gray, Parts: []Part{
		colored(box(Accent, 1, 1, 14, 3), ink),
		path(Detail, dotted([2]float64{1.5, 5.5}, [2]float64{1.5, 14.5})+dotted([2]float64{5.5, 5.5}, [2]float64{5.5, 14.5})+
			dotted([2]float64{10.5, 5.5}, [2]float64{10.5, 14.5})+dotted([2]float64{14.5, 5.5}, [2]float64{14.5, 14.5})+
			dotted([2]float64{3.5, 7.5}, [2]float64{14.5, 7.5})+dotted([2]float64{3.5, 10.5}, [2]float64{14.5, 10.5})+
			dotted([2]float64{3.5, 14.5}, [2]float64{12.5, 14.5})),
	}},
	{ID: "spreadsheet-show-headers", Color: gray, Parts: append(cells(),
		colored(path(Accent, "M1.5 1.5H14.5V4.5H5.5V14.5H1.5Z"), steel),
	)},
	{ID: "spreadsheet-show-groups", Color: gray, Parts: join([]Part{
		box(Body, 10.5, 1.5, 4, 13),
		path(Detail, "M10.5 4.5H14.5M10.5 7.5H14.5M10.5 10.5H14.5"),
		colored(path(Detail, "M3.5 5.5V13.5H5.5M7.5 10.5V13.5H9"), ink),
	}, groupBox(1.5, 1.5, "M2.5 3.5H4.5"), groupBox(5.5, 6.5, "M6.5 8.5H8.5M7.5 7.5V9.5"))},
	// «Редактирование» в меню «Таблица», «Только просмотр» в библиотеке:
	// карандаш «Изменить» через весь лист (выбор
	// владельца 16.10.2026: малый карандаш в углу тонул в сетке).
	{ID: "spreadsheet-read-only", Color: gray, Parts: join(cells(), editingPencil())},
	// Рисунки табличного документа.
	{ID: "chart", Color: gray, Parts: chart()},
	{ID: "gantt-chart", Color: slate, Parts: []Part{
		box(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M5.5 1.5V14.5M2.5 3.5H3.5M2.5 4.5H3.5M2.5 6.5H3.5M2.5 7.5H3.5M2.5 9.5H3.5M2.5 10.5H3.5M2.5 12.5H3.5M2.5 13.5H3.5"),
		colored(box(Accent, 7, 3, 4, 2), brown), colored(box(Accent, 10, 6, 3, 2), brown),
		colored(box(Accent, 8, 9, 3, 2), brown), colored(box(Accent, 9, 12, 4, 2), brown),
	}},
	{ID: "dendrogram", Color: gray, Parts: []Part{
		path(Detail, "M1.5 1.5V14.5H14.5"),
		colored(path(Detail, "M3.5 14.5V10.5H7.5V14.5M9.5 14.5V7.5H13.5V14.5M5.5 10.5V4.5H11.5V7.5"), ink),
	}},
	{ID: "geographical-schema", Color: blue, Parts: []Part{
		circle(Body, 7.5, 7.5, 6),
		colored(path(Accent, "M3 6Q4 3 7 2.5Q7.5 4.5 6.5 6Q7 8 5.5 9.5Q3.5 8.5 3 6Z"), green),
		colored(path(Accent, "M10 4Q12.5 4 13.5 6.5Q13 9 11.5 9.5Q10.5 8 10.5 6.5Q9.5 5.5 10 4Z"), green),
		colored(path(Accent, "M7.5 10.5Q9.5 10 10 11.5Q9 12.5 8 12.75Q7.25 11.75 7.5 10.5Z"), green),
		colored(path(InkSpot, "M4.5 16Q4.5 14 7.5 14Q10.5 14 10.5 16Z"), black),
	}},
	{ID: "graphical-schema", Color: slate, Parts: []Part{
		path(Body, page),
		path(Detail, pageFold),
		path(Detail, "M5.5 6V11M7 4.5H10.5V8"),
		colored(box(Accent, 4, 3, 3, 3), blue),
		colored(circle(Accent, 10.5, 9.5, 1.5), orange),
		colored(box(Accent, 4, 11, 5, 2), green),
	}},
}
