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

// badge is a disc at the lower corner of what it marks, hiding what lies
// under it, with its mark inside: the plus of a copy, the cross of a
// deletion mark.
func badge(cx, cy float64, color, mark string) []Part {
	return []Part{colored(circle(Plate, cx, cy, 3.5), color), colored(circle(Body, cx, cy, 3.5), color), colored(path(Mark, mark), color)}
}

// gear is a wheel with eight teeth and a hole: the settings of a report.
func gear(cx, cy, r float64) []Part {
	const teeth = 8
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
	return []Part{path(Body, d+"Z"), circle(Detail, cx, cy, 1.25)}
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
	return []Part{first(1.5), last(9), path(InkMark, "M11.5 1.5V13.5"), path(Detail, "M9.5 11.5L11.5 13.5L13.5 11.5")}
}

// crossedDisc is a disc with a cross in it: «delete» and «stop» of the
// library in red, «delete directly» grey with the cross red.
func crossedDisc() []Part {
	return []Part{circle(Body, 8, 8, 6.5), path(Mark, "M5.5 5.5L10.5 10.5M10.5 5.5L5.5 10.5")}
}

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
	}, badge(5, 11, red, "M3.5 9.5L6.5 12.5M6.5 9.5L3.5 12.5"))},
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
	}, []Part{colored(circle(Plate, 11, 11, 4.5), gray)}, gear(11, 11, 4.5))},
	{ID: "choose-value", Color: blue, Accent: slate, Parts: []Part{
		rect(Body, 1.5, 1.5, 10, 3.5), rect(Body, 1.5, 6, 10, 3.5), rect(Body, 1.5, 10.5, 10, 3.5),
		colored(path(Plate, "M9.5 7.5V15L11.25 13.25L12.5 15.5L13.75 14.75L12.5 12.5H14.75Z"), slate),
	}},
	{ID: "set-date-interval", Color: navy, Accent: navy, Parts: []Part{
		path(Detail, "M4 2Q0.5 8 4 14M12 2Q15.5 8 12 14"),
		path(Accent, "M7.25 5L4.5 8L7.25 11Z"), path(Accent, "M8.75 5L11.5 8L8.75 11Z"),
	}},
	{ID: "create-folder", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, "M1.5 3.5H5.5L7 5H13.5V12.5H1.5Z"),
		rect(Accent, 2, 7, 11, 1.5),
		colored(path(Mark, "M12.5 9.5V15M9.75 12.25H15.25"), green),
	}},
	{ID: "expand-all", Color: gray, Parts: listOrder("M2.5 1.5V14M0.75 12.25L2.5 14L4.25 12.25")},
	{ID: "collapse-all", Color: gray, Parts: listOrder("M2.5 14.5V2M0.75 3.75L2.5 2L4.25 3.75")},
	{ID: "close", Color: "#3a3f45", Parts: []Part{
		path(InkMark, "M3.5 3.5L12.5 12.5M12.5 3.5L3.5 12.5"),
	}},
	{ID: "sort-list-asc", Color: "#3a3f45", Parts: sortList(letterA, letterYa)},
	{ID: "sort-list-desc", Color: "#3a3f45", Parts: sortList(letterYa, letterA)},
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
	{ID: "input-field-choose-type", Color: "#3a3f45", Parts: []Part{
		path(InkMark, "M3 3.5V2.5H13V3.5M8 2.5V13.5M5.5 13.5H10.5"),
	}},
}
