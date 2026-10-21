package icons

import (
	"fmt"
	"math"
)

// The pictures of the library the configurator has no command of its own
// for, drawn from the window «Выбор картинки» as the inventory photographed
// it (docs/materials/block-00/standartnye-kartinki/). The prototype paints a
// disc or an arrow solid; in the two-tone style it is a body with its mark,
// as «add» is.

// black is the ink of the prototype's black marks and signs: the cross of
// «close», the letters of sorting, the question of help.
const black = "#3a3f45"

// badge is a disc at the lower corner of what it marks, hiding what lies
// under it, with its sign inside in a thin line: the plus of a copy, the
// cross of a deletion mark. A thick mark filled the small disc into a spot;
// a thin one in a larger disc reads at once (the owner's choice of
// 16.10.2026).
func badge(cx, cy float64, color, sign string) []Part {
	const r = 4
	return []Part{colored(circle(Plate, cx, cy, r), color), colored(circle(Body, cx, cy, r), color), colored(path(Line, sign), color)}
}

// gearInk is the dark grey of the gear: with no halo a grey as light as the
// lines of a table merged with them, so it is dark on a light page and,
// lightening as ink does, light on a dark one (the owner's remark of
// 16.10.2026).
const gearInk = "#5f6770"

// gear is a wheel of six teeth with a hole, solid grey, outer radius r on
// whole points round cx, cy: the settings of a report, a storage of settings,
// a setting alone. Its teeth are square, so that the top and the bottom one
// are sharp columns of pixels, its depth, teeth and hole grow with it. At 16
// px an outlined gear of nine pixels read as a grey spot with any number of
// teeth (the owner's choice of 16.10.2026 on the pass of 16 physical
// pixels); it lies on its picture with no halo, as every sign does.
func gear(cx, cy, r float64) []Part {
	const teeth = 6
	half := func(value float64) float64 { return math.Round(value*2) / 2 }
	depth, w, hole := half(r*0.3), math.Round(r/4), half(r/3)
	body := r - depth
	along := math.Sqrt(body*body - w*w)
	at := func(angle, u, v float64) string {
		x, y := math.Cos(angle), math.Sin(angle)
		return number(round(cx+x*u-y*v)) + " " + number(round(cy+y*u+x*v))
	}
	arc := fmt.Sprintf("A%s %s 0 0 1 ", number(body), number(body))
	d := ""
	for index := range teeth {
		angle := (float64(index)*360/teeth - 90) * math.Pi / 180
		if index == 0 {
			d += "M" + at(angle, along, -w)
		} else {
			d += arc + at(angle, along, -w)
		}
		d += "L" + at(angle, r, -w) + "L" + at(angle, r, w) + "L" + at(angle, along, w)
	}
	outline := d + arc + at(-math.Pi/2, along, -w) + "Z"
	cut := fmt.Sprintf("M%s %sA%s %s 0 1 0 %s %sA%s %s 0 1 0 %s %sZ", number(cx), number(cy-hole), number(hole), number(hole),
		number(cx), number(cy+hole), number(hole), number(hole), number(cx), number(cy-hole))
	return []Part{colored(path(InkSpot, outline+cut), gearInk)}
}

// postedDocument is a document with its lines and coins beneath it, which
// posting moves: the arrow over it says which way.
func postedDocument(arrow []Part) []Part {
	return join([]Part{
		rect(Body, 1.5, 1.5, 9, 11),
		path(Detail, "M3.5 4.5H6.5M3.5 6.5H8.5M3.5 8.5H8.5"),
	}, coinStack(11, 11.5, 4.5, 1.75, 2, 1, Rim), arrow)
}

// listOrder is a list with the arrow of what is done to all its levels:
// expanded down, collapsed up - its shaft and its head.
func listOrder(shaft, tip string) []Part {
	return []Part{
		path(Detail, "M8.5 3.5H14.5M8.5 6.5H14.5M8.5 9.5H14.5M8.5 12.5H14.5"),
		path(Accent, bullets(5, 3, 4)),
		colored(path(Line, shaft), orange), colored(path(Accent, tip), orange),
	}
}

// bullets are the square dots of the rows of a list, two pixels each, the
// first at x, top and the rest every three pixels down: the line of a row
// stands level with the top of its dot.
func bullets(x, top float64, count int) string {
	d := ""
	for index := range count {
		y := top + 3*float64(index)
		d += fmt.Sprintf("M%s %sH%sV%sH%sZ", number(x), number(y), number(x+2), number(y+2), number(x))
	}
	return d
}

// letterA and letterYa are the first and last letters of the alphabet the
// sorting of a list goes by, standing at the top or the bottom.
func letterA(top float64) Part {
	return colored(path(Line, fmt.Sprintf("M1.5 %sL3.75 %sL6 %sM2.1 %sH5.4",
		number(top+5.5), number(top), number(top+5.5), number(top+4))), blue)
}

func letterYa(top float64) Part {
	return colored(path(Line, fmt.Sprintf("M5.5 %sV%sH3.5Q2 %s 2 %sQ2 %s 3.5 %sH5.5M3.75 %sL2 %s",
		number(top+5.5), number(top), number(top), number(top+1.5), number(top+3), number(top+3), number(top+3), number(top+5.5))), red)
}

func sortList(first, last func(float64) Part) []Part {
	return []Part{first(1.5), last(8.5), sortArrow(12, 2, 14)}
}

// sortArrow is the solid arrow of sorting, from top down to its point at
// tip: dark on a light page, light on a dark one, in the colour of ink.
func sortArrow(x, top, tip float64) Part {
	return colored(path(InkSpot, downArrow(x, top, tip)), black)
}

// downArrow is the shape of a solid arrow from top down to its point at tip:
// the arrow of sorting a list and, blue, of the order of data composition.
func downArrow(x, top, tip float64) string {
	return fmt.Sprintf("M%s %sH%sV%sH%sL%s %sL%s %sH%sZ",
		number(x-1), number(top), number(x+1), number(tip-4), number(x+2.5),
		number(x), number(tip), number(x-2.5), number(tip-4), number(x-1))
}

// crossedDisc is a disc with a cross in it: «delete» and «stop» of the
// library in red, «delete directly» grey with the cross red.
func crossedDisc() []Part {
	return []Part{circle(Body, 8, 8, 6.5), path(Mark, "M5.5 5.5L10.5 10.5M10.5 5.5L5.5 10.5")}
}

// taskBoard is the board of a task with the lines of what it asks, the
// sign of what is done to it in the lower right corner.
func taskBoard(sign ...Part) []Part {
	return join(board(), []Part{path(Detail, "M3.5 6.5H9.5M3.5 8.5H9.5M3.5 10.5H7.5")}, sign)
}

// listFrame is a list in its frame, as the modes of viewing a list show it:
// what stands in it says which mode.
func listFrame(inside ...Part) []Part {
	return append([]Part{rect(Body, 1.5, 1.5, 13, 13)}, inside...)
}

// zoomLens is the glass of the scale with the sign of the way it goes in it.
func zoomLens(sign string) []Part {
	return append(lens(6, 6, 4.5), colored(path(Accent, sign), ink))
}

// mound is the heap of the values a form keeps, a yellow triangle whose top
// the arrow never covers, and valuesSheet the form they are taken from:
// restoring brings them back to it, saving takes them away.
func mound(left float64) Part {
	return colored(path(Accent, fmt.Sprintf("M%s 15L%s 11L%s 15Z", number(left), number(left+3.5), number(left+7))), coin)
}

func valuesSheet(left float64) []Part {
	return []Part{
		rect(Body, left, 1.5, 8, 10),
		path(Detail, fmt.Sprintf("M%s 4.5H%sM%s 6.5H%sM%s 8.5H%s", number(left+2), number(left+6), number(left+2), number(left+6), number(left+2), number(left+6))),
	}
}

// filterOver is the funnel of a selection laid over what it selects from,
// hiding it by its own shape: the history of selections, selecting and
// sorting, selecting by the current value, switching selection off.
func filterOver(left, top, width float64) []Part {
	return []Part{colored(path(Plate, funnel(left, top, width)), gray), colored(path(Body, funnel(left, top, width)), gray)}
}

// colouredRows are the rows of a tree told apart by colour, each hanging on
// its stalk: the tree a list is searched or sorted in.
func colouredRows(left float64) []Part {
	return []Part{
		path(Detail, fmt.Sprintf("M%s 5V12.5M%s 6.5H%sM%s 9.5H%sM%s 12.5H%s",
			number(left), number(left), number(left+1.5), number(left), number(left+1.5), number(left), number(left+1.5))),
		colored(rect(Accent, left-0.5, 3, 3, 2), red), colored(rect(Accent, left+1.5, 6, 3, 2), yellow),
		colored(rect(Accent, left+1.5, 9, 3, 2), blue), colored(rect(Accent, left+1.5, 12, 3, 2), green),
	}
}

// pointer is the arrow of the mouse that picks a row: the top level of a
// tree, a row found in it.
const pointer = "M8.5 4.5V14.5L10.75 12.5L12.25 15.5L13.75 14.75L12.5 12.5H15.5Z"

var standardIcons = []Icon{
	// Действия формы и списка.
	{ID: "refresh", Color: green, Accent: green, Parts: []Part{
		path(Line, "M13.5 8.5A5.5 5.5 0 1 1 8 3"),
		path(Accent, "M8 0.5L11.5 3L8 5.5Z"),
	}},
	{ID: "generate-report", Color: green, Accent: green, Parts: []Part{
		path(Accent, "M4 2L13.5 8L4 14Z"),
	}},
	{ID: "mark-to-delete", Color: slate, Parts: join([]Part{
		path(Body, "M5.5 1.5H10.5L14.5 5.5V14.5H5.5Z"), path(Detail, "M10.5 1.5V5.5H14.5"),
	}, badge(4.5, 11.5, red, "M3 10L6 13M6 10L3 13"))},
	{ID: "delete-directly", Color: gray, Accent: red, Parts: crossedDisc()},
	{ID: "delete-circle", Color: red, Accent: red, Parts: crossedDisc()},
	{ID: "write-and-close", Color: slate, Accent: green, Parts: []Part{
		path(Body, page), path(Detail, pageFold),
		path(Mark, "M4.5 10L7 12.5L11.5 8"),
	}},
	{ID: "post", Color: slate, Accent: coin, Parts: postedDocument([]Part{
		colored(path(Line, "M7.5 3Q12.5 2.5 12.5 7.5"), green),
		colored(path(Accent, head(12.5, 9.5, 0, 1, 2)), green),
	})},
	{ID: "undo-posting", Color: slate, Accent: coin, Parts: postedDocument([]Part{
		colored(path(Line, "M12.5 8.5Q12.5 3 9 3"), red),
		colored(path(Accent, "M10 0.5L7.5 3L10 5.5Z"), red),
	})},
	{ID: "move-left", Color: blue, Accent: blue, Parts: solidArrow(270)},
	{ID: "move-right", Color: blue, Accent: blue, Parts: solidArrow(90)},
	{ID: "report-settings", Color: gray, Parts: join([]Part{
		colored(rect(Body, 1.5, 1.5, 11, 10), slate),
		colored(path(Detail, "M1.5 4.5H12.5M4.5 4.5V11.5"), slate),
	}, gear(11, 11, 5))},
	{ID: "choose-value", Color: blue, Accent: slate, Parts: []Part{
		rect(Body, 1.5, 1.5, 10, 3), rect(Body, 1.5, 6.5, 10, 3), rect(Body, 1.5, 11.5, 10, 3),
		colored(path(Plate, pointer), slate),
	}},
	{ID: "set-date-interval", Color: navy, Accent: navy, Parts: []Part{
		path(Detail, "M4 2Q0.5 8 4 14M12 2Q15.5 8 12 14"),
		path(Accent, "M7 5L4 8L7 11Z"), path(Accent, "M9 5L12 8L9 11Z"),
	}},
	{ID: "create-folder", Color: groupColor, Accent: yellow, Parts: join([]Part{
		path(Body, "M1.5 3.5H5.5L7.5 5.5H13.5V12.5H1.5Z"),
		rect(Accent, 2, 7, 11, 2),
	}, []Part{colored(path(Accent, plus(12, 12, 3)), green)})},
	{ID: "expand-all", Color: gray, Parts: listOrder("M2.5 1.5V11", head(2.5, 14.5, 0, 1, 2))},
	{ID: "collapse-all", Color: gray, Parts: listOrder("M2.5 14.5V5", head(2.5, 1.5, 0, -1, 2))},
	{ID: "close", Color: black, Parts: []Part{
		path(InkMark, "M3.5 3.5L12.5 12.5M12.5 3.5L3.5 12.5"),
	}},
	{ID: "sort-list-asc", Color: black, Parts: sortList(letterA, letterYa)},
	{ID: "sort-list-desc", Color: black, Parts: sortList(letterYa, letterA)},
	{ID: "show-data", Color: slate, Parts: join([]Part{
		rect(Body, 7.5, 7.5, 7, 7), path(Detail, "M9.5 10.5H12.5M9.5 12.5H12.5"),
	}, tint(heldLens(9.5, 6.5, 5, -1), gray))},
	// Кнопки поля ввода и то, что они открывают. The calendar and the
	// calculator of the library are grey: the configurator's own, on the menu
	// «Сервис», are painted.
	{ID: "input-field-select", Color: gray, Accent: gray, Parts: []Part{
		circle(Accent, 3, 11, 1), circle(Accent, 8, 11, 1), circle(Accent, 13, 11, 1),
	}},
	{ID: "input-field-clear", Color: gray, Accent: gray, Parts: []Part{
		path(Mark, "M5.5 5.5L10.5 10.5M10.5 5.5L5.5 10.5"),
	}},
	{ID: "input-field-open", Color: gray, Parts: []Part{
		rect(Body, 6.5, 2.5, 7, 7),
		rect(Plate, 2.5, 6.5, 7, 7), rect(Body, 2.5, 6.5, 7, 7),
	}},
	{ID: "input-field-calendar", Color: gray, Accent: gray, Parts: []Part{
		rect(Body, 2.5, 2.5, 11, 12),
		rect(Accent, 3, 3, 10, 3),
		path(Detail, "M5.5 1V3.5M10.5 1V3.5"),
		path(Detail, "M4.5 8.5H5.5M7.5 8.5H8.5M10.5 8.5H11.5M4.5 11.5H5.5M7.5 11.5H8.5M10.5 11.5H11.5"),
	}},
	{ID: "input-field-calculator", Color: gray, Accent: gray, Parts: calculator()},
	{ID: "input-field-choose-type", Color: black, Parts: []Part{
		path(InkMark, "M3 4V3H13V4M8 3V13M5.5 13H10.5"),
	}},
	// Виды объектов и данных. A kind is drawn by its icon of the tree: the
	// library shows the same picture, and its «objects» of the compatibility
	// mode of 8.1 too - a reference, an object and a manager of one kind share
	// one icon (the owner's decision of 16.10.2026). Drawn here is only what
	// the tree has not: the debit and credit of an entry and the actions of a
	// task and a business process.
	{ID: "debit", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M1.5 14.5V12.5H9.5V14.5M3.5 12.5L4.5 10.5V3.5H8.5V12.5M10.5 7.5H14.5M12.5 7.5V12.5"),
	}},
	{ID: "credit", Color: red, Accent: red, Parts: []Part{
		path(Line, "M2.5 3.5V12.5M2.5 8.5H4.5L8.5 3.5M4.5 8.5L8.5 12.5M10.5 7.5H14.5M12.5 7.5V12.5"),
	}},
	{ID: "debit-credit", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M1.5 7.5V5.5H6.5V7.5M2.5 5.5L3.5 4.5V1.5H5.5V5.5M7 3.5H10M8.5 3.5V5.5"),
		colored(path(Line, "M7.5 9V14.5M7.5 11.5H8.5L10.5 9M8.5 11.5L10.5 14.5M11.5 10.5H15.5M13.5 10.5V14.5"), red),
	}},
	{ID: "activate-task", Color: brown, Accent: gray, Parts: taskBoard(
		colored(rect(Plate, 8.5, 8.5, 7, 6), slate), colored(rect(Body, 8.5, 8.5, 7, 6), slate), colored(rect(Accent, 9, 9, 6, 2), blue),
	)},
	{ID: "execute-task", Color: brown, Accent: gray, Parts: taskBoard(
		colored(path(Mark, "M8.5 11.5L11 14L15 9.5"), green),
	)},
	{ID: "business-process-start", Color: blue, Accent: blue, Parts: []Part{
		colored(rect(Accent, 1, 4, 2, 8), "#8fb8e0"), colored(rect(Accent, 4, 4, 2, 8), "#8fb8e0"),
		path(Accent, "M7 2.5L14.5 8L7 13.5Z"),
	}},
	// Действия списков: поиск, режимы просмотра, уровни, масштаб, значения и
	// настройки. The prototype draws «Управление поиском» with the glass of
	// «Найти» and «Параметры» with the wrench of «Сервис → Параметры»: those
	// share icons. «Печать сразу» is the printer with a large check over it,
	// where the configurator's «Печать текущего» shows a sheet.
	{ID: "cancel-search", Color: slate, Accent: red, Parts: join(lens(9, 6, 4.5), []Part{
		path(Mark, "M2 10L6.5 14.5M6.5 10L2 14.5"),
	})},
	{ID: "data-search", Color: ink, Parts: lens(7.5, 7.5, 4)},
	{ID: "zoom", Color: ink, Parts: zoomLens(plus(6, 6, 3))},
	{ID: "zoom-in", Color: gray, Parts: zoomLens(plus(6, 6, 3))},
	{ID: "zoom-out", Color: gray, Parts: zoomLens(minus(6, 6, 3))},
	{ID: "show-in-list", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 4.5, 1.5, 10, 13),
		path(Detail, "M4.5 4.5H14.5M9.5 1.5V14.5M11 6.5H13M11 9.5H13M11 12.5H13"),
		path(Accent, "M1 6H4V3.5L8.5 8L4 12.5V10H1Z"),
	}},
	{ID: "list-view-mode", Color: slate, Accent: yellow, Parts: []Part{
		colored(path(Body, "M1.5 1.5H4.5L5.5 2.5H9.5V7.5H1.5Z"), groupColor),
		rect(Accent, 2, 4, 7, 2),
		path(Detail, "M3.5 9.5H14.5"),
		colored(bar(5, 12, 10, 3), blue),
	}},
	{ID: "list-view-mode-tree", Color: gray, Accent: green, Parts: listFrame(
		bar(3, 3, 4, 2), bar(3, 11, 4, 2),
		path(Detail, "M4.5 5V11M4.5 6.5H7M4.5 9.5H7M8.5 6.5H12.5M8.5 9.5H12.5"),
	)},
	{ID: "list-view-mode-hierarchical-list", Color: gray, Accent: green, Parts: listFrame(
		bar(3, 3, 4, 2),
		path(Detail, "M4.5 5V12.5M4.5 6.5H7M4.5 9.5H7M4.5 12.5H7M8.5 6.5H12.5M8.5 9.5H12.5M8.5 12.5H12.5"),
	)},
	{ID: "list-view-mode-list", Color: gray, Accent: orange, Parts: listFrame(
		path(Accent, bullets(3, 3, 4)),
		path(Detail, "M6.5 3.5H12.5M6.5 6.5H12.5M6.5 9.5H12.5M6.5 12.5H12.5"),
	)},
	{ID: "level-up", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, "M1.5 4.5H5.5L7.5 6.5H11.5V13.5H1.5Z"), rect(Accent, 2, 8, 9, 2),
		colored(path(Line, "M13.5 10V3.5"), blue), colored(path(Accent, "M11.5 4L13.5 1L15.5 4Z"), blue),
	}},
	{ID: "level-down", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, "M1.5 4.5H5.5L7.5 6.5H11.5V13.5H1.5Z"), rect(Accent, 2, 8, 9, 2),
		colored(path(Line, "M13.5 1.5V8"), blue), colored(path(Accent, "M11.5 8L13.5 11L15.5 8Z"), blue),
	}},
	{ID: "rename", Color: blue, Parts: join([]Part{
		rect(Body, 1.5, 5.5, 13, 5),
	}, []Part{colored(path(InkMark, "M8 3V13M6.5 3H9.5M6.5 13H9.5"), black)})},
	{ID: "restore-values", Color: slate, Accent: green, Parts: join(valuesSheet(6.5), []Part{
		mound(0),
		path(Accent, "M2 10V5Q2 3 4 3H6V1L9 4L6 7V5H4V10Z"),
	})},
	{ID: "save-values", Color: slate, Accent: green, Parts: join(valuesSheet(1.5), []Part{
		mound(9),
		path(Accent, "M10 3H12Q14 3 14 5V7H16L13 10L10 7H12V5H10Z"),
	})},
	{ID: "customize-list", Color: blue, Parts: join([]Part{
		rect(Body, 1.5, 3.5, 9, 11),
		path(Detail, "M3.5 6.5H8.5M3.5 9.5H8.5M3.5 12.5H8.5"),
	}, filterOver(5.5, 1.5, 9))},
	{ID: "customize-form", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), rect(Accent, 2, 2, 12, 2),
		colored(circle(Body, 8, 9, 3.5), gray), colored(circle(Accent, 8, 9, 1), gray),
	}},
	{ID: "setting", Color: gray, Parts: gear(8, 8, 7)},
	{ID: "load-report-settings", Color: gray, Parts: join([]Part{
		colored(path(Body, "M1.5 2.5H5.5L7.5 4.5H12.5V11.5H1.5Z"), groupColor),
		colored(rect(Accent, 2, 6, 10, 2), yellow),
	}, gear(11, 11, 5))},
	{ID: "save-report-settings", Color: gray, Parts: join(tint(floppy(), blue), gear(11, 11, 5))},
	// The floppy and at its lower right the button «OK» of a form, small,
	// white, its edge and its letters in ink, so that on a dark theme both
	// are as light (the owner's choices of 17.10.2026).
	{ID: "end-edit", Color: blue, Accent: slate, Parts: join(floppy(),
		[]Part{colored(rect(Plate, 5.5, 8.5, 10, 7), black)}, tint(okButton(5.5, 8.5, 10, 7, 2, 3, InkLine, InkLine), black))},
	{ID: "grayed-all", Color: slate, Accent: gray, Parts: append(stackedWindows(), rect(Accent, 3, 6, 7, 7))},
	{ID: "fix-table", Color: gray, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, "M1.5 8.5H14.5M1.5 11.5H14.5M10.5 2.5V13.5"),
		colored(path(Detail, "M1.5 5.5H14.5M5.5 2.5V13.5"), ink),
	}},
	{ID: "move-item", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, "M1.5 5.5H4.5L5.5 6.5H9.5V11.5H1.5Z"), rect(Accent, 2, 8, 7, 1),
		colored(path(Line, "M12.5 12.5V2.5Q12.5 1.5 11.5 1.5H6.5Q5.5 1.5 5.5 2.5V3"), navy),
		colored(path(Accent, head(5.5, 6.5, 0, 1, 2)), navy),
		colored(bar(8, 12, 7, 3), blue),
	}},
	{ID: "switch-activity", Color: blue, Accent: blue, Parts: []Part{
		path(Accent, "M1 5H10L7.5 11H1Z"),
		colored(path(Accent, "M11 5H15V11H8.5Z"), gray),
	}},
	{ID: "print-immediately", Color: gray, Accent: green, Parts: append(printer(),
		path(Mark, "M2.5 3.5L5 6L10 1"),
	)},
	// The whole list behind and in front of it the printer of «Print», ten
	// pixels wide, at the lower right, cleared from the list (the owner's
	// choice of 17.10.2026: half a list and a box of a printer read as
	// neither).
	{ID: "output-list", Color: blue, Parts: join(list(false), cleared(tint(printerAt(5.5, 5.5, 10, 4, 2, 4), gray)...))},
	// Скрытые из окна выбора: команды списков обычного приложения. Fourteen
	// of the 29 hidden pictures are drawn by the prototype as a visible one
	// and share its icon (docs/block-00/standartnye-kartinki.md); the rest
	// are drawn here from the form that shows them.
	{ID: "input-on-basis", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 8, 10),
		rect(Plate, 6.5, 4.5, 8, 10), rect(Body, 6.5, 4.5, 8, 10),
		path(Line, "M3.5 5.5V9.5Q3.5 11.5 5.5 11.5H8.5"),
		path(Accent, "M8 9.25L11 11.5L8 13.75Z"),
	}},
	{ID: "find-in-tree-sheet", Color: slate, Parts: join([]Part{rect(Body, 1.5, 1.5, 10, 13)}, tint(colouredRows(3.5), gray), []Part{
		path(Plate, pointer), path(Body, pointer),
	})},
	{ID: "find-in-list", Color: slate, Accent: red, Parts: join([]Part{
		rect(Body, 1.5, 1.5, 10, 13),
		path(Accent, bullets(3, 3, 3)),
		path(Detail, "M6.5 3.5H9.5M6.5 6.5H9.5M6.5 9.5H9.5"),
	}, []Part{colored(path(Detail, "M15 12.5H9"), ink), colored(path(Tip, head(5.5, 12.5, -1, 0, 2)), ink)})},
	{ID: "find-by-number", Color: navy, Parts: join(lens(6.5, 9.5, 4), []Part{
		colored(path(Detail, "M10.5 5.5V1.5L13.5 5.5V1.5"), black),
	})},
	{ID: "list-settings", Color: blue, Parts: join([]Part{
		rect(Body, 1.5, 1.5, 9, 3), rect(Body, 1.5, 6.5, 9, 3), rect(Body, 1.5, 11.5, 9, 3),
	}, gear(11, 5, 5))},
	{ID: "filter-history", Color: slate, Parts: join([]Part{
		rect(Body, 4.5, 4.5, 10, 10),
		path(Detail, "M10.5 6.5H13M10.5 9.5H13M6 12.5H13"),
	}, filterOver(1.5, 1.5, 9))},
	{ID: "filter-and-sort", Color: slate, Parts: join([]Part{
		rect(Body, 1.5, 2.5, 11, 12), colored(rect(Accent, 2, 3, 10, 2), blue),
		path(Detail, "M3 8.5H4.5M3 10.5H4.5M3 12.5H4.5M9.5 8.5H10.5M9.5 10.5H10.5"),
	}, filterOver(1.5, 2.5, 11), []Part{sortArrow(13, 6, 15)})},
	{ID: "filter-by-current-value", Color: slate, Accent: blue, Parts: join(filterOver(1.5, 1.5, 9), []Part{
		path(Plate, "M9.5 3.5H14.5V14.5H9.5Z"), path(Body, "M9.5 3.5H14.5V14.5H9.5Z"), path(Detail, "M9.5 8.5H14.5"),
		path(Line, "M10.75 11.75L11.75 12.75L13.25 11"),
	})},
	{ID: "clear-filter", Color: gray, Accent: red, Parts: join(filterOver(1.5, 1.5, 11), []Part{
		path(Mark, "M10 10L14.5 14.5M14.5 10L10 14.5"),
	})},
	{ID: "filter-by-type", Color: navy, Parts: []Part{
		colored(bar(2, 1, 9, 2), red), colored(bar(3, 4, 8, 2), red),
		colored(bar(2, 7, 9, 2), blue), colored(bar(3, 10, 8, 2), blue),
		colored(bar(2, 13, 9, 2), red),
		path(Detail, "M12 1.5Q13.5 1.5 13.5 3V6.5Q13.5 8 15 8Q13.5 8 13.5 9.5V13Q13.5 14.5 12 14.5"),
	}},
	{ID: "view-by-owner", Color: navy, Parts: []Part{
		colored(bar(2, 2, 8, 3), blue),
		path(Detail, "M1.5 7.5H11.5"),
		colored(bar(5, 10, 10, 4), blue),
	}},
	{ID: "edit-in-dialog", Color: slate, Accent: red, Parts: []Part{
		rect(Body, 2.5, 2.5, 12, 11),
		path(Detail, "M2.5 5.5H14.5M8.5 5.5V13.5M4 7.5H7M4 10.5H7M10 7.5H13M10 10.5H13"),
		path(Mark, "M1.5 14.5L14.5 1.5"),
	}},
	// The bars alone over the whole grid: under them the grey grid of the
	// pivot table took a third of the picture (the owner's remark of
	// 17.10.2026).
	{ID: "pivot-chart", Color: gray, Parts: chartBars(15, 14)},
	{ID: "sorting", Color: slate, Accent: navy, Parts: join([]Part{rect(Body, 1.5, 1.5, 9, 13)}, colouredRows(3.5), []Part{
		path(Line, "M12 3.5Q14.5 3.5 14.5 8Q14.5 12.5 12 12.5"),
		path(Accent, "M13 1.5L10.5 3.5L13 5.5Z"), path(Accent, "M13 10.5L10.5 12.5L13 14.5Z"),
	})},
	{ID: "set-time", Color: steel, Accent: red, Parts: []Part{
		circle(Body, 7.5, 7.5, 6),
		colored(path(Detail, "M7.5 3.5V7.5L5 10"), navy),
		path(Line, "M10.5 7.5H11.5M7.5 10.5V11.5M3.5 7.5H4.5"),
	}},
}
