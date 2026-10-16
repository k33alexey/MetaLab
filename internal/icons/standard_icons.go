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
	return over(colored(circle(Plate, cx, cy, r), color), colored(circle(Body, cx, cy, r), color), colored(path(Line, sign), color))
}

// gear is a wheel with six teeth and a hole, outlined, hiding what lies
// under it by its own shape and no disc around it: the settings of a report,
// a storage of settings (the owner's choice of 16.10.2026 - eight teeth on a
// grey disc read as a spot at 16 px).
func gear(cx, cy, r float64) []Part {
	const teeth = 6
	inner := r - 1.5
	d := ""
	for index := range teeth * 4 {
		angle := (float64(index) - 1.5) * math.Pi / (teeth * 2)
		radius := r
		if index%4 >= 2 {
			radius = inner
		}
		d += map[bool]string{true: "M", false: "L"}[index == 0] + number(round(cx+radius*math.Cos(angle))) + " " + number(round(cy+radius*math.Sin(angle)))
	}
	return []Part{path(Plate, d+"Z"), path(Body, d+"Z"), circle(Detail, cx, cy, 1.25)}
}

// postedDocument is a document with its lines and coins beneath it, which
// posting moves: the arrow over it says which way.
func postedDocument(arrow []Part) []Part {
	return join([]Part{
		rect(Body, 1.5, 1.5, 9, 11),
		path(Detail, "M3.5 4.5H6.5M3.5 6.5H8.5M3.5 8.5H8.5"),
	}, coinStack(10.5, 11.5, 1), arrow)
}

// listOrder is a list with the arrow of what is done to all its levels:
// expanded down, collapsed up.
func listOrder(arrow string) []Part {
	return []Part{
		path(Detail, "M8 3.5H14.5M8 6.5H14.5M8 9.5H14.5M8 12.5H14.5"),
		path(Accent, "M5.5 3H6.5V4H5.5ZM5.5 6H6.5V7H5.5ZM5.5 9H6.5V10H5.5ZM5.5 12H6.5V13H5.5Z"),
		colored(path(Line, arrow), orange),
	}
}

// letterA and letterYa are the first and last letters of the alphabet the
// sorting of a list goes by, standing at the top or the bottom.
func letterA(top float64) Part {
	return colored(path(Line, fmt.Sprintf("M1.5 %sL3.75 %sL6 %sM2.4 %sH5.1",
		number(top+5.5), number(top), number(top+5.5), number(top+3.75))), blue)
}

func letterYa(top float64) Part {
	return colored(path(Line, fmt.Sprintf("M5.5 %sV%sH3.5Q2 %s 2 %sQ2 %s 3.5 %sH5.5M3.75 %sL2 %s",
		number(top+5.5), number(top), number(top), number(top+1.5), number(top+3), number(top+3), number(top+3), number(top+5.5))), red)
}

func sortList(first, last func(float64) Part) []Part {
	return []Part{first(1.5), last(9), sortArrow(11.75, 1.5, 14.5)}
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
		number(x-0.75), number(top), number(x+0.75), number(tip-4), number(x+2.5),
		number(x), number(tip), number(x-2.5), number(tip-4), number(x-0.75))
}

// crossedDisc is a disc with a cross in it: «delete» and «stop» of the
// library in red, «delete directly» grey with the cross red.
func crossedDisc() []Part {
	return []Part{circle(Body, 8, 8, 6.5), path(Mark, "M5.5 5.5L10.5 10.5M10.5 5.5L5.5 10.5")}
}

// taskBoard is the board of a task with the lines of what it asks, the
// sign of what is done to it in the lower right corner.
func taskBoard(sign ...Part) []Part {
	return join(board(), []Part{path(Detail, "M3.5 6.5H9.5M3.5 8.5H9.5M3.5 10.5H7.5")}, over(sign...))
}

// listFrame is a list in its frame, as the modes of viewing a list show it:
// what stands in it says which mode.
func listFrame(inside ...Part) []Part {
	return append([]Part{rect(Body, 1.5, 1.5, 13, 13)}, inside...)
}

// zoomLens is the glass of the scale with the sign of the way it goes in it.
func zoomLens(sign string) []Part {
	return append(lens(6.5, 6.5, 4.5), colored(path(Mark, sign), ink))
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
		path(Detail, fmt.Sprintf("M%s 4.25V12.75M%s 7.25H%sM%s 10.25H%sM%s 12.75H%s",
			number(left), number(left), number(left+1.5), number(left), number(left+1.5), number(left), number(left+1.5))),
		colored(rect(Accent, left, 3.5, 3, 1.5), red), colored(rect(Accent, left+1.5, 6.5, 3, 1.5), yellow),
		colored(rect(Accent, left+1.5, 9.5, 3, 1.5), blue), colored(rect(Accent, left+1.5, 12, 3, 1.5), green),
	}
}

// pointer is the arrow of the mouse that picks a row: the top level of a
// tree, a row found in it.
const pointer = "M8 4.5V15L10.5 12.75L12 15.75L13.75 14.9L12.25 12H15.5Z"

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
		path(Body, "M5.5 1.5H11L14.5 5V14.5H5.5Z"), path(Detail, "M11 1.5V5H14.5"),
	}, badge(5, 11, red, "M3.25 9.25L6.75 12.75M6.75 9.25L3.25 12.75"))},
	{ID: "delete-directly", Color: gray, Accent: red, Parts: crossedDisc()},
	{ID: "delete-circle", Color: red, Accent: red, Parts: crossedDisc()},
	{ID: "write-and-close", Color: slate, Accent: green, Parts: []Part{
		path(Body, page), path(Detail, pageFold),
		path(Mark, "M4.5 10L7 12.5L11.5 8"),
	}},
	{ID: "post", Color: slate, Accent: coin, Parts: postedDocument([]Part{
		colored(path(Line, "M7.5 3Q12.5 2.5 12.5 7.5"), green),
		colored(headDown(12.5, 9.5), green),
	})},
	{ID: "undo-posting", Color: slate, Accent: coin, Parts: postedDocument([]Part{
		colored(path(Line, "M12.5 8.5Q12.5 3 9 3"), red),
		colored(path(Accent, "M9.5 0.5L7 3L9.5 5.5Z"), red),
	})},
	{ID: "move-left", Color: blue, Parts: []Part{
		path(Body, "M1.5 8L7.5 2V5.5H14.5V10.5H7.5V14Z"),
	}},
	{ID: "move-right", Color: blue, Parts: []Part{
		path(Body, "M14.5 8L8.5 2V5.5H1.5V10.5H8.5V14Z"),
	}},
	{ID: "report-settings", Color: gray, Parts: join([]Part{
		colored(rect(Body, 1.5, 1.5, 11, 10), slate),
		colored(path(Detail, "M1.5 4.5H12.5M5 4.5V11.5"), slate),
	}, gear(11, 11, 4.75))},
	{ID: "choose-value", Color: blue, Accent: slate, Parts: []Part{
		rect(Body, 1.5, 1.5, 10, 3.5), rect(Body, 1.5, 6, 10, 3.5), rect(Body, 1.5, 10.5, 10, 3.5),
		colored(path(Plate, pointer), slate),
	}},
	{ID: "set-date-interval", Color: navy, Accent: navy, Parts: []Part{
		path(Detail, "M4 2Q0.5 8 4 14M12 2Q15.5 8 12 14"),
		path(Accent, "M7.25 5L4.5 8L7.25 11Z"), path(Accent, "M8.75 5L11.5 8L8.75 11Z"),
	}},
	{ID: "create-folder", Color: groupColor, Accent: yellow, Parts: join([]Part{
		path(Body, "M1.5 3.5H5.5L7 5H13.5V12.5H1.5Z"),
		rect(Accent, 2, 7, 11, 1.5),
	}, over(colored(path(Mark, "M12.5 9.5V15M9.75 12.25H15.25"), green)))},
	{ID: "expand-all", Color: gray, Parts: listOrder("M2.5 1.5V14M0.75 12.25L2.5 14L4.25 12.25")},
	{ID: "collapse-all", Color: gray, Parts: listOrder("M2.5 14.5V2M0.75 3.75L2.5 2L4.25 3.75")},
	{ID: "close", Color: black, Parts: []Part{
		path(InkMark, "M3.5 3.5L12.5 12.5M12.5 3.5L3.5 12.5"),
	}},
	{ID: "sort-list-asc", Color: black, Parts: sortList(letterA, letterYa)},
	{ID: "sort-list-desc", Color: black, Parts: sortList(letterYa, letterA)},
	{ID: "show-data", Color: slate, Parts: join([]Part{
		rect(Body, 7.5, 7.5, 7, 7), path(Detail, "M9.5 11H12.5M9.5 13H12.5"),
	}, tint(heldLens(9, 6.5, 5, -1), gray))},
	// Кнопки поля ввода и то, что они открывают. The calendar and the
	// calculator of the library are grey: the configurator's own, on the menu
	// «Сервис», are painted.
	{ID: "input-field-select", Color: gray, Accent: gray, Parts: []Part{
		circle(Accent, 3.5, 11, 1.1), circle(Accent, 8, 11, 1.1), circle(Accent, 12.5, 11, 1.1),
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
		rect(Accent, 3, 3, 10, 2.5),
		path(Detail, "M5.5 1V3.5M10.5 1V3.5"),
		path(Detail, "M5 8.5H6M7.5 8.5H8.5M10 8.5H11M5 11H6M7.5 11H8.5M10 11H11"),
	}},
	{ID: "input-field-calculator", Color: gray, Accent: gray, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		rect(Accent, 4, 3, 8, 2.5),
		path(Detail, "M4.5 8H5.5M7.5 8H8.5M10.5 8H11.5M4.5 10.5H5.5M7.5 10.5H8.5M10.5 10.5H11.5M4.5 13H5.5M7.5 13H8.5M10.5 13H11.5"),
	}},
	{ID: "input-field-choose-type", Color: black, Parts: []Part{
		path(InkMark, "M3 3.5V2.5H13V3.5M8 2.5V13.5M5.5 13.5H10.5"),
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
		path(Line, "M2.5 3.5V12.5M2.5 8H4L8 3.5M4 8L8.5 12.5M10.5 7.5H14.5M12.5 7.5V12.5"),
	}},
	{ID: "debit-credit", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M1.5 7.5V5.5H6.5V7.5M2.5 5.5L3.5 4.5V1.5H5.5V5.5M7 3.5H10M8.5 3.5V5.5"),
		colored(path(Line, "M7.5 9V14.5M7.5 11.5H8.5L10.5 9M8.5 11.5L10.5 14.5M11.5 10.5H15.5M13.5 10.5V14.5"), red),
	}},
	{ID: "activate-task", Color: brown, Accent: gray, Parts: taskBoard(
		colored(rect(Plate, 8.5, 8.5, 7, 6), slate), colored(rect(Body, 8.5, 8.5, 7, 6), slate), colored(rect(Accent, 9, 9, 6, 1.5), blue),
	)},
	{ID: "execute-task", Color: brown, Accent: gray, Parts: taskBoard(
		colored(path(Mark, "M8.5 11.5L11 14L15 9.5"), green),
	)},
	{ID: "business-process-start", Color: blue, Accent: blue, Parts: []Part{
		colored(rect(Accent, 1.5, 4, 1.5, 8), "#8fb8e0"), colored(rect(Accent, 4, 4, 1.5, 8), "#8fb8e0"),
		path(Accent, "M7 2.5L14.5 8L7 13.5Z"),
	}},
	// Действия списков: поиск, режимы просмотра, уровни, масштаб, значения и
	// настройки. The prototype draws «Управление поиском» with the glass of
	// «Найти» and «Параметры» with the wrench of «Сервис → Параметры»: those
	// share icons. «Печать сразу» is the printer with a large check over it,
	// where the configurator's «Печать текущего» shows a sheet.
	{ID: "cancel-search", Color: slate, Accent: red, Parts: join(lens(9.5, 6, 4.5), []Part{
		path(Mark, "M2 10L6.5 14.5M6.5 10L2 14.5"),
	})},
	{ID: "data-search", Color: ink, Parts: lens(7, 7, 4)},
	{ID: "zoom", Color: ink, Parts: zoomLens("M6.5 4.5V8.5M4.5 6.5H8.5")},
	{ID: "zoom-in", Color: gray, Parts: zoomLens("M6.5 4.5V8.5M4.5 6.5H8.5")},
	{ID: "zoom-out", Color: gray, Parts: zoomLens("M4.5 6.5H8.5")},
	{ID: "show-in-list", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 4.5, 1.5, 10, 13),
		path(Detail, "M4.5 4.5H14.5M9.5 1.5V14.5M11 7H13M11 9.5H13M11 12H13"),
		path(Accent, "M1 6.5H4V4L8.5 8L4 12V9.5H1Z"),
	}},
	{ID: "list-view-mode", Color: slate, Accent: yellow, Parts: []Part{
		colored(path(Body, "M1.5 1.5H4.5L5.5 2.5H9.5V7.5H1.5Z"), groupColor),
		rect(Accent, 2, 4, 7, 1),
		path(Detail, "M3.5 10H14.5"),
		colored(bar(5, 12, 10, 3), blue),
	}},
	{ID: "list-view-mode-tree", Color: gray, Accent: green, Parts: listFrame(
		bar(3.5, 3, 4, 2), bar(3.5, 11, 4, 2),
		path(Detail, "M4.5 5V11M4.5 7.25H7M4.5 9.25H7M8.5 7.25H12.5M8.5 9.25H12.5"),
	)},
	{ID: "list-view-mode-hierarchical-list", Color: gray, Accent: green, Parts: listFrame(
		bar(3.5, 3, 4, 2),
		path(Detail, "M4.5 5V12M4.5 7H7M4.5 9.5H7M4.5 12H7M8.5 7H12.5M8.5 9.5H12.5M8.5 12H12.5"),
	)},
	{ID: "list-view-mode-list", Color: gray, Accent: orange, Parts: listFrame(
		path(Accent, "M3.5 4H5V5.5H3.5ZM3.5 6.5H5V8H3.5ZM3.5 9H5V10.5H3.5ZM3.5 11.5H5V13H3.5Z"),
		path(Detail, "M6.5 4.75H12.5M6.5 7.25H12.5M6.5 9.75H12.5M6.5 12.25H12.5"),
	)},
	{ID: "level-up", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, "M1.5 4.5H5.5L7 6H11.5V13.5H1.5Z"), rect(Accent, 2, 8, 9, 1.5),
		colored(path(Line, "M13.5 10V3"), blue), colored(path(Accent, "M11.25 4.25L13.5 1.25L15.75 4.25Z"), blue),
	}},
	{ID: "level-down", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, "M1.5 4.5H5.5L7 6H11.5V13.5H1.5Z"), rect(Accent, 2, 8, 9, 1.5),
		colored(path(Line, "M13.5 1.5V8.5"), blue), colored(path(Accent, "M11.25 7.75L13.5 10.75L15.75 7.75Z"), blue),
	}},
	{ID: "rename", Color: blue, Parts: join([]Part{
		rect(Body, 1.5, 5.5, 13, 5),
	}, over(colored(path(InkMark, "M8 2.5V13.5M6.5 2.5H9.5M6.5 13.5H9.5"), black)))},
	{ID: "restore-values", Color: slate, Accent: green, Parts: join(valuesSheet(7), []Part{
		mound(0),
		path(Accent, "M2.5 10V5.5Q2.5 3.5 4.5 3.5H6V1.5L9.5 4.5L6 7.5V5.5H4.5V10Z"),
	})},
	{ID: "save-values", Color: slate, Accent: green, Parts: join(valuesSheet(1.5), []Part{
		mound(9),
		path(Accent, "M9.5 3.5H11.5Q13.5 3.5 13.5 5.5V7H15.5L12.5 10.25L9.5 7H11.5V5.5H9.5Z"),
	})},
	{ID: "customize-list", Color: blue, Parts: join([]Part{
		rect(Body, 1.5, 3.5, 9, 11),
		path(Detail, "M3.5 6.5H8.5M3.5 9H8.5M3.5 11.5H8.5"),
	}, over(filterOver(6, 1.5, 9)...))},
	{ID: "customize-form", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), rect(Accent, 2, 2, 12, 2.5),
		colored(circle(Body, 8, 9.25, 3.25), gray), colored(circle(Accent, 8, 9.25, 1.25), gray),
	}},
	{ID: "setting", Color: gray, Parts: gear(8, 8, 6.5)},
	{ID: "load-report-settings", Color: gray, Parts: join([]Part{
		colored(path(Body, "M1.5 2.5H5L6.5 4H12.5V11.5H1.5Z"), groupColor),
		colored(rect(Accent, 2, 6, 10, 1.5), yellow),
	}, gear(11, 11, 4.75))},
	{ID: "save-report-settings", Color: gray, Parts: join(tint(floppy(), blue), gear(11, 11, 4.75))},
	{ID: "end-edit", Color: blue, Accent: slate, Parts: join(floppy(), []Part{
		colored(rect(Plate, 6.5, 9.5, 9, 6), black),
		colored(path(Detail, "M8.75 11Q7.5 11 7.5 12.5Q7.5 14 8.75 14Q10 14 10 12.5Q10 11 8.75 11ZM11.5 11V14M14.5 11L11.5 12.5L14.5 14"), black),
	})},
	{ID: "grayed-all", Color: slate, Accent: gray, Parts: append(stackedWindows(), rect(Accent, 3.5, 6.5, 6, 6))},
	{ID: "fix-table", Color: gray, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, "M1.5 8.5H14.5M1.5 11H14.5M10.5 2.5V13.5"),
		colored(path(Detail, "M1.5 5.5H14.5M6 2.5V13.5"), ink),
	}},
	{ID: "move-item", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, "M1.5 5.5H4.5L5.5 6.5H9.5V11.5H1.5Z"), rect(Accent, 2, 8, 7, 1),
		colored(path(Line, "M12.5 12.5V2.5Q12.5 1.5 11.5 1.5H6.5Q5.5 1.5 5.5 2.5V3"), navy),
		colored(headDown(5.5, 5.5), navy),
		colored(bar(8, 12.5, 7, 2.5), blue),
	}},
	{ID: "switch-activity", Color: blue, Accent: blue, Parts: []Part{
		path(Accent, "M1.5 5.5H9.5L7 10.5H1.5Z"),
		colored(path(Accent, "M10.5 5.5H14.5V10.5H8Z"), gray),
	}},
	{ID: "print-immediately", Color: gray, Accent: green, Parts: append(printer(),
		path(Mark, "M2.5 3.5L5 6L10 1"),
	)},
	{ID: "output-list", Color: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 8, 13),
		path(Detail, "M3.5 4.5H7.5M3.5 7H7.5"),
		colored(path(Detail, "M8.5 7.5V3.5H13.5V7.5"), gray),
		colored(rect(Plate, 6.5, 7.5, 9, 5), gray), colored(rect(Body, 6.5, 7.5, 9, 5), gray),
		colored(path(Detail, "M8.5 10.5H13.5V14.5H8.5Z"), gray),
	}},
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
		path(Accent, "M3.5 4H5V5.5H3.5ZM3.5 6.5H5V8H3.5ZM3.5 9H5V10.5H3.5Z"),
		path(Detail, "M6.5 4.75H9.5M6.5 7.25H9.5M6.5 9.75H9.5"),
	}, over(colored(path(Detail, "M15 12H8.5M8.5 9.75L5.5 12L8.5 14.25Z"), ink)))},
	{ID: "find-by-number", Color: navy, Parts: join(lens(6, 9, 4), []Part{
		colored(path(Detail, "M10.5 5.5V1.5L14 5.5V1.5"), black),
	})},
	{ID: "list-settings", Color: blue, Parts: join([]Part{
		rect(Body, 1.5, 3.5, 9, 3.5), rect(Body, 1.5, 7.5, 9, 3.5), rect(Body, 1.5, 11.5, 9, 3),
	}, over(tint(gear(11.25, 4.75, 4.25), gray)...))},
	{ID: "filter-history", Color: slate, Parts: join([]Part{
		rect(Body, 4.5, 4.5, 10, 10),
		path(Detail, "M10.5 7.5H13M10.5 10H13M6 12.5H13"),
	}, filterOver(1.5, 1.5, 9))},
	{ID: "filter-and-sort", Color: slate, Parts: join([]Part{
		rect(Body, 1.5, 2.5, 11, 12), colored(rect(Accent, 2, 3, 10, 1.5), blue),
		path(Detail, "M3 8H4.5M3 10H4.5M3 12H4.5M9.5 8H10.5M9.5 10H10.5"),
	}, filterOver(1.5, 2.5, 11), []Part{sortArrow(12.75, 6, 15)})},
	{ID: "filter-by-current-value", Color: slate, Accent: blue, Parts: join(filterOver(1.5, 1.5, 10), []Part{
		path(Plate, "M9.5 3.5H14.5V14.5H9.5Z"), path(Body, "M9.5 3.5H14.5V14.5H9.5Z"), path(Detail, "M9.5 9H14.5"),
		path(Line, "M10.75 11.75L11.75 12.75L13.25 11"),
	})},
	{ID: "clear-filter", Color: gray, Accent: red, Parts: join(filterOver(1.5, 1.5, 11), []Part{
		path(Mark, "M10 10L14.5 14.5M14.5 10L10 14.5"),
	})},
	{ID: "filter-by-type", Color: navy, Parts: []Part{
		colored(bar(1.5, 1.5, 9, 2), red), colored(bar(3, 4.5, 8, 2), red),
		colored(bar(1.5, 7.5, 9, 2), blue), colored(bar(3, 10.5, 8, 2), blue),
		colored(bar(1.5, 13.5, 9, 2), red),
		path(Detail, "M12 1.5Q13.5 1.5 13.5 3V6.5Q13.5 8 15 8Q13.5 8 13.5 9.5V13Q13.5 14.5 12 14.5"),
	}},
	{ID: "view-by-owner", Color: navy, Parts: []Part{
		colored(bar(1.5, 2, 8, 3), blue),
		path(Detail, "M1.5 7.5H11.5"),
		colored(bar(5.5, 10, 9.5, 4), blue),
	}},
	{ID: "edit-in-dialog", Color: slate, Accent: red, Parts: []Part{
		rect(Body, 2.5, 2.5, 12, 11),
		path(Detail, "M2.5 5.5H14.5M8.5 5.5V13.5M4 8H7M4 10.5H7M10 8H13M10 10.5H13"),
		path(Mark, "M1.5 14.5L14.5 1.5"),
	}},
	{ID: "pivot-chart", Color: gray, Parts: append([]Part{
		rect(Body, 1.5, 10.5, 13, 4),
		path(Detail, "M1.5 12.5H14.5M5 10.5V12.5M9.5 10.5V12.5M3.5 12.5V14.5M7.5 12.5V14.5M12 12.5V14.5"),
	}, chartBars(10.5, 8.5)...)},
	{ID: "sorting", Color: slate, Accent: navy, Parts: join([]Part{rect(Body, 1.5, 1.5, 9, 13)}, colouredRows(3.5), []Part{
		path(Line, "M12 3.5Q15 3.5 15 8Q15 12.5 12 12.5"),
		path(Accent, "M12.75 1.5L10.25 3.5L12.75 5.5Z"), path(Accent, "M12.75 10.5L10.25 12.5L12.75 14.5Z"),
	})},
	{ID: "set-time", Color: steel, Accent: red, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		colored(path(Detail, "M8 3.5V8L5.5 11"), navy),
		circle(Accent, 12, 8, 0.75), circle(Accent, 8, 12, 0.75), circle(Accent, 4, 8, 0.75),
	}},
}
