package icons

import (
	"fmt"
	"math"
)

// The commands of Studio's windows, menus and panels, drawn as the
// prototype's main menu, toolbars and the action panel of its tree show
// them (docs/block-00/obolochka.md). The same command has the same picture
// wherever it stands.

func round(value float64) float64 {
	return math.Round(value*100) / 100
}

// lens is a magnifying glass: the glass a body, the handle brown and solid,
// leaving it to the lower right.
func lens(cx, cy, r float64) []Part {
	return heldLens(cx, cy, r, 1)
}

// heldLens is the glass with its handle to the lower right (side 1) or to
// the lower left (side -1), so that the handle keeps clear of what the glass
// is laid over.
func heldLens(cx, cy, r, side float64) []Part {
	const k, w = math.Sqrt2 / 2, 0.9
	near, far := r+0.25, r+3.75
	point := func(along, across float64) string {
		return fmt.Sprintf("%s %s", number(round(cx+side*(along*k-across*k))), number(round(cy+along*k+across*k)))
	}
	return []Part{
		circle(Body, cx, cy, r),
		colored(path(Accent, "M"+point(near, -w)+"L"+point(far, -w)+"L"+point(far, w)+"L"+point(near, w)+"Z"), brown),
	}
}

// person is a head over shoulders, the head solid in skin, the shoulders a
// body in the colour of the icon, standing on y bottom.
func person(cx, bottom, size float64) []Part {
	head := size * 0.42
	return []Part{
		colored(circle(Accent, cx, bottom-size-head+0.5, head), skin),
		path(Body, fmt.Sprintf("M%s %sQ%s %s %s %sQ%s %s %s %sZ",
			number(cx-size*0.8), number(bottom), number(cx-size*0.8), number(bottom-size+0.5), number(cx), number(bottom-size+0.5),
			number(cx+size*0.8), number(bottom-size+0.5), number(cx+size*0.8), number(bottom))),
	}
}

// printer is a printer with the paper going in at the top and the printed
// sheet coming out at the bottom: «print», and with a check on its paper
// «print the current one».
func printer() []Part {
	return []Part{
		path(Detail, "M4.5 6.5V1.5H11.5V6.5"),
		rect(Body, 1.5, 6.5, 13, 6),
		path(Detail, "M4.5 10.5H11.5V14.5H4.5Z"),
	}
}

// lockedPerson is a user with a padlock laid over the lower right of him, in
// the colour of the lock: a temporary lock grey, the locks of
// authentication gold. The person keeps the size of «users», as the
// prototype keeps it under every sign.
func lockedPerson(lock string) []Part {
	return join(person(6.5, 14.5, 6), over(
		colored(path(Detail, padlock), lock),
		colored(path(Plate, "M9.5 11.5H14.5V15H9.5Z"), lock),
		colored(rect(Accent, 9.5, 11.5, 5, 3.5), lock),
	))
}

// editingPencil is the pencil of «change», alone, under the clock of the
// history of data and across the sheet of a spreadsheet being edited.
func editingPencil() []Part {
	return []Part{path(Body, pencil), path(Detail, "M3 11.5L4.5 13M9.25 4.5L11.5 6.75")}
}

// floppy is the diskette of writing: «save», and under the sign of what is
// written - the gear of a report's settings, the «OK» of ending an edit.
func floppy() []Part {
	return []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M4.5 1.5V5.5H11.5V1.5M4.5 14.5V9.5H11.5V14.5"),
		rect(Accent, 9, 2.5, 1.5, 2),
	}
}

// blockArrow is the solid arrow of moving an item, turned by angle degrees
// clockwise from pointing up: a wide head on a narrower shaft, through the
// middle of the grid. Moving up, down, left and right and the arrows of
// appearance, straight and inclined, are this one arrow (the prototype draws
// them alike).
func blockArrow(angle float64) string {
	turn := angle * math.Pi / 180
	corners := [][2]float64{{0, -6.5}, {6, -0.5}, {2.5, -0.5}, {2.5, 6.5}, {-2.5, 6.5}, {-2.5, -0.5}, {-6, -0.5}}
	points := make([][2]float64, len(corners))
	for index, corner := range corners {
		x, y := corner[0], corner[1]
		points[index] = [2]float64{round(8 + x*math.Cos(turn) - y*math.Sin(turn)), round(8 + x*math.Sin(turn) + y*math.Cos(turn))}
	}
	return polygon(points...)
}

func join(groups ...[]Part) []Part {
	var parts []Part
	for _, group := range groups {
		parts = append(parts, group...)
	}
	return parts
}

const (
	skin    = "#e8a46a"
	purple  = "#8a6bbf"
	cross   = "M4 4L12 12M12 4L4 12"
	plus    = "M8 4.5V11.5M4.5 8H11.5"
	tree    = "M2.5 2.5V10.5M2.5 4.5H5.5M2.5 7.5H5.5M2.5 10.5H5.5"
	pencil  = "M2 14L2.75 11L10.5 3.25L12.75 5.5L5 13.25Z"
	padlock = "M10.5 11.5V10Q10.5 8.5 12 8.5Q13.5 8.5 13.5 10V11.5"
	// editPencil is the pencil laid over what it edits, to the lower right.
	editPencil = "M7.5 15L8.1 12.6L12.75 7.95L14.55 9.75L9.9 14.4Z"
)

var commandIcons = []Icon{
	// Файл.
	{ID: "file-new", Color: slate, Parts: []Part{
		path(Body, page), path(Detail, pageFold),
	}},
	{ID: "file-open", Color: groupColor, Accent: blue, Parts: []Part{
		path(Body, "M1.5 6.5H5.5L7 8H14.5V14.5H1.5Z"),
		path(Line, "M10.5 6.5V1.5M8.5 3.5L10.5 1.5L12.5 3.5"),
	}},
	{ID: "file-close", Color: gray, Accent: gray, Parts: []Part{
		path(Body, "M1.5 6.5H5.5L7 8H14.5V14.5H1.5Z"),
		path(Line, "M10.5 1.5V6.5M8.5 4.5L10.5 6.5L12.5 4.5"),
	}},
	{ID: "save", Color: blue, Accent: slate, Parts: floppy()},
	{ID: "compare-files", Color: slate, Parts: join([]Part{
		rect(Body, 1.5, 1.5, 7, 9), path(Detail, "M3.5 4.5H6.5M3.5 6.5H6.5"),
	}, lens(10, 10, 3)),
	},
	{ID: "print", Color: gray, Parts: printer()},
	{ID: "print-current", Color: gray, Accent: green, Parts: append(printer(),
		path(Mark, "M5.75 4L7.25 5.5L10.25 2.5"),
	)},
	{ID: "print-preview", Color: slate, Parts: join([]Part{
		path(Body, "M1.5 1.5H7.5L10.5 4.5V13.5H1.5Z"), path(Detail, "M7.5 1.5V4.5H10.5"),
	}, lens(10, 10.5, 3)),
	},
	{ID: "page-setup", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		path(Line, "M5 1.5V14.5M11 1.5V14.5M2.5 4H13.5M2.5 12H13.5"),
	}},
	// Правка.
	{ID: "undo", Color: steel, Accent: steel, Parts: []Part{
		path(Line, "M13.5 12.5Q13.5 6 7 6H3M5.5 3.5L3 6L5.5 8.5"),
	}},
	{ID: "redo", Color: steel, Accent: steel, Parts: []Part{
		path(Line, "M2.5 12.5Q2.5 6 9 6H13M10.5 3.5L13 6L10.5 8.5"),
	}},
	{ID: "cut", Color: slate, Parts: scissors()},
	{ID: "copy", Color: slate, Parts: twoPages()},
	{ID: "paste", Color: gray, Accent: slate, Parts: clipboard()},
	{ID: "find", Color: slate, Parts: lens(6.5, 6.5, 4.5)},
	{ID: "find-next", Color: slate, Accent: steel, Parts: join(lens(7, 9, 3.5), []Part{
		path(Line, "M2.5 4Q7.5 0.5 13 4M13 1.5V4H10.5"),
	})},
	{ID: "find-previous", Color: slate, Accent: steel, Parts: join(lens(7, 9, 3.5), []Part{
		path(Line, "M13 4Q8 0.5 2.5 4M2.5 1.5V4H5"),
	})},
	// The same picture stands on the menu «Правка» and in the library: the
	// letter A found and replaced by B.
	{ID: "replace", Color: slate, Accent: blue, Parts: join(lens(7.5, 9, 3.5), []Part{
		colored(path(Line, "M1 6.5L2.75 1.5L4.5 6.5M1.6 4.75H3.9"), red),
		path(Line, "M11.5 6.5V1.5H13.25Q14.75 1.5 14.75 2.75Q14.75 4 13.25 4H11.5M13.25 4Q15 4 15 5.25Q15 6.5 13.25 6.5H11.5"),
	})},
	{ID: "global-search", Color: groupColor, Parts: join([]Part{
		path(Body, "M1.5 2.5H5.5L7 4H13.5V12.5H1.5Z"),
	}, over(tint(lens(10, 10, 3), slate)...))},
	{ID: "bookmarks", Color: gray, Accent: "#5b636d", Parts: flag()},
	{ID: "interface-texts", Color: green, Accent: red, Parts: []Part{
		path(Line, "M1.5 2.5H9.5M5.5 2.5V11.5"),
		path(Body, "M7.5 14.5L8.25 12L13 7.25L14.75 9L10 13.75Z"),
	}},
	{ID: "find-in-tree", Color: slate, Parts: []Part{
		path(Detail, tree),
		path(Body, "M7.5 5.5V14L9.5 12L11 15L12.5 14.25L11 11.5H13.5Z"),
	}},
	{ID: "additional", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 3.5, 3.5, 9, 7), path(Detail, "M3.5 5.5H12.5"),
		rect(Body, 5.5, 7.5, 9, 7), path(Detail, "M5.5 9.5H14.5"),
		path(Mark, "M3.75 1V6.5M1 3.75H6.5"),
	}},
	// The sheet of properties, as the palette shows them - the names at the
	// left, the fields of the values at the right - with the cube of the
	// object they describe in its lower right corner. The prototype draws a
	// hand holding a letter; the owner chose the sheet and the cube
	// (16.10.2026). The cube is a badge only: alone it is the cube of an
	// external source, and boxed with tape the package of XDTO.
	{ID: "properties", Color: slate, Accent: "#8fb8e0", Parts: join([]Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M1.5 4.5H14.5M7.5 4.5V14.5M3.5 7H6M3.5 9.5H6M3.5 12H6"),
		rect(Accent, 8.5, 6.25, 4.5, 1.5), rect(Accent, 8.5, 8.75, 3, 1.5),
	}, over(objectBadge()...))},
	// Конфигурация.
	{ID: "configuration-open", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window+"M4.5 7.5V11.5M4.5 9.5H7.5M4.5 11.5H7.5"),
	}},
	{ID: "configuration-close", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 4.5, 11, 10),
		path(Detail, "M1.5 7.5H12.5M4.5 9.5V13M4.5 11H7M4.5 13H7"),
		rect(Accent, 9.5, 1, 5.5, 5.5),
		path(OnAccent, "M11 2.5L13.5 5M13.5 2.5L11 5"),
	}},
	{ID: "configuration-save", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window),
		colored(rect(Body, 6.5, 7.5, 8, 7), blue),
		path(Line, "M8.5 7.5V10H12.5V7.5"),
	}},
	{ID: "database-update", Color: blue, Accent: green, Parts: []Part{
		path(Body, "M5.5 4V12.5Q5.5 14.5 10 14.5Q14.5 14.5 14.5 12.5V4Q14.5 2 10 2Q5.5 2 5.5 4Z"),
		path(Detail, "M5.5 4Q5.5 6 10 6Q14.5 6 14.5 4"),
		path(Line, "M2 14.5V9.5Q2 7.5 4 7.5H6M4.5 6L6 7.5L4.5 9"),
	}},
	{ID: "compare-merge", Color: slate, Accent: groupColor, Parts: join([]Part{
		path(Detail, tree),
		path(Accent, "M9.5 1.5H11.5L12.5 2.5H15V5.5H9.5Z"),
	}, lens(9.5, 9.5, 3))},
	{ID: "compare-configurations", Color: slate, Parts: join([]Part{
		path(Detail, tree),
	}, lens(9.5, 8, 3.5))},
	{ID: "configuration-report", Color: slate, Accent: red, Parts: []Part{
		path(Body, page), path(Detail, pageFold+"M6.5 8H11.5M6.5 10.5H11.5"),
		rect(Accent, 4, 7.25, 1.5, 1.5),
		colored(rect(Accent, 4, 9.75, 1.5, 1.5), blue),
	}},
	{ID: "check-configuration", Color: slate, Accent: red, Parts: []Part{
		path(Detail, "M1.5 3.5H8.5M3.5 6.5H8.5M3.5 9.5H6.5M3.5 12.5H6.5"),
		path(Mark, "M8.5 9.5L10.75 12.5L14.5 5"),
	}},
	{ID: "configuration-window", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window+"M6.5 5.5V13.5M9 8.5H12.5M9 11H12.5"),
		path(Line, "M3.5 8V12M3.5 10H5"),
	}},
	// Отладка и Сервис.
	{ID: "run", Color: gold, Accent: blue, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		path(Accent, "M6.5 5L11 8L6.5 11Z"),
	}},
	{ID: "performance-measurement", Color: steel, Accent: steel, Parts: []Part{
		circle(Body, 8, 9, 5.5),
		path(Detail, "M8 9V6M8 9L10 10.5M6.5 1.5H9.5M8 1.5V3.5M12.5 3.5L13.5 4.5"),
	}},
	{ID: "record-actions", Color: gold, Accent: red, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		circle(Accent, 8, 8, 3.5),
	}},
	{ID: "calculator", Color: brown, Accent: green, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		rect(Accent, 4, 3, 8, 2.5),
		path(Detail, "M4.5 8H5.5M7.5 8H8.5M10.5 8H11.5M4.5 10.5H5.5M7.5 10.5H8.5M10.5 10.5H11.5M4.5 13H5.5M7.5 13H8.5M10.5 13H11.5"),
	}},
	{ID: "calendar", Color: slate, Accent: red, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 12),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M5 1V3.5M11 1V3.5M4 8.5H5M7.5 8.5H8.5M11 8.5H12M4 11.5H5M7.5 11.5H8.5"),
		rect(Accent, 10.5, 10.5, 2, 2),
	}},
	{ID: "text-templates", Color: brown, Accent: blue, Parts: []Part{
		path(Body, "M3.5 1.5H13.5V14.5H3.5Q2.5 14.5 2.5 13.5V2.5Q2.5 1.5 3.5 1.5Z"),
		path(Detail, "M4.5 1.5V14.5"),
		path(Line, "M7 5H11.5M7 7.5H11.5M7 10H10"),
	}},
	{ID: "temporary-lock", Color: blue, Accent: gray, Parts: lockedPerson(gray)},
	{ID: "settings", Color: gray, Parts: []Part{
		path(Body, "M10.5 1.5A4 4 0 0 0 7 7L1.75 12.25A1.5 1.5 0 0 0 3.75 14.25L9 9A4 4 0 0 0 14.5 5.5L12.25 7.25L10 6L9.25 3.75Z"),
	}},
	// Администрирование.
	{ID: "users", Color: blue, Parts: person(8, 14.5, 6)},
	// A user online: the green dot of a messenger at the person's side, where
	// the prototype has a monitor (the owner's choice of 16.10.2026).
	{ID: "active-users", Color: blue, Accent: green, Parts: join(person(6.5, 14.5, 6), over(
		colored(circle(Plate, 12, 12, 2.5), green), circle(Accent, 12, 12, 2),
	))},
	{ID: "event-log", Color: blue, Parts: []Part{
		rect(Body, 1.5, 3.5, 13, 9),
		circle(Detail, 5, 8, 1.75), circle(Detail, 11, 8, 1.75),
		path(Detail, "M5 9.75H11M3.5 3.5V5.5M12.5 3.5V5.5"),
	}},
	{ID: "authentication-locks", Color: blue, Accent: gold, Parts: lockedPerson(gold)},
	// Окна.
	{ID: "window-new", Color: slate, Accent: green, Parts: join([]Part{
		rect(Body, 1.5, 2.5, 13, 11), path(Detail, window),
	}, over(path(Mark, "M8 7.5V12M5.75 9.75H10.25")))},
	{ID: "split-horizontal", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11), path(Detail, "M1.5 8H14.5"),
	}},
	{ID: "split-vertical", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11), path(Detail, "M8 2.5V13.5"),
	}},
	{ID: "service-messages", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 8, 13), path(Detail, "M3.5 4.5H7.5M3.5 7.5H7.5M3.5 10.5H6"),
		path(Accent, "M11 6.5H13.25L12 12.25Q11.85 13 12.4 13H13.25L13 14.25H11.4Q9.85 14.25 10.15 12.75L11.15 7.75H10.75Z"),
		circle(Accent, 12.9, 4.25, 1.25),
	}},
	{ID: "navigate-back", Color: slate, Accent: "#2f6fb5", Parts: []Part{
		rect(Body, 3.5, 1.5, 11, 8), path(Detail, "M3.5 4H14.5"),
		path(Accent, "M1 12L5.5 8V10.5H12V13.5H5.5V16Z"),
	}},
	{ID: "navigate-forward", Color: slate, Accent: "#2f6fb5", Parts: []Part{
		rect(Body, 1.5, 1.5, 11, 8), path(Detail, "M1.5 4H12.5"),
		path(Accent, "M15 12L10.5 8V10.5H4V13.5H10.5V16Z"),
	}},
	{ID: "windows", Color: slate, Parts: []Part{
		rect(Body, 1.5, 1.5, 9, 8), path(Detail, "M1.5 4H10.5"),
		rect(Body, 5.5, 6.5, 9, 8), path(Detail, "M5.5 9H14.5"),
	}},
	// Справка.
	{ID: "help", Color: gold, Accent: "#efc23a", Parts: []Part{
		circle(Accent, 8, 8, 6.5),
		path(OnAccent, "M5.75 6Q5.75 3.75 8 3.75Q10.25 3.75 10.25 5.75Q10.25 7.25 8 8.25V9.5M8 12V12.25"),
	}},
	{ID: "syntax-assistant", Color: gold, Accent: "#2f4f7f", Parts: []Part{
		colored(circle(Accent, 8, 7.5, 2.5), skin),
		path(Body, "M3 15Q3 10.5 8 10.5Q13 10.5 13 15Z"),
		path(Accent, "M1.5 4L8 1.5L14.5 4L8 6.5Z"),
		path(Line, "M12.5 4.5V8"),
	}},
	{ID: "syntax-search", Color: slate, Accent: blue, Parts: join([]Part{
		path(Line, "M1.5 3.5Q1.5 1.5 3.5 1.5Q5.5 1.5 5.5 3.25Q5.5 4.5 3.5 5.5V7M3.5 9.25V9.5"),
	}, lens(9.5, 8, 4))},
	{ID: "about", Color: gold, Accent: "#efc23a", Parts: []Part{
		circle(Accent, 8, 8, 6.5),
		path(OnAccent, "M8 7V11.5M8 4.5V4.75"),
	}},
	// Действия дерева и его контекстного меню.
	{ID: "add", Color: green, Accent: green, Parts: join([]Part{circle(Body, 8, 8, 6.5)}, over(path(Mark, plus)))},
	{ID: "change", Color: "#6f9a2a", Accent: "#efc23a", Parts: editingPencil()},
	{ID: "clone", Color: slate, Accent: green, Parts: join([]Part{
		path(Body, "M2.5 1.5H8L10.5 4V12.5H2.5Z"),
	}, plusBadge(11, 11))},
	{ID: "delete", Color: red, Accent: red, Parts: []Part{
		path(Mark, cross),
	}},
	{ID: "move-up", Color: blue, Parts: []Part{
		path(Body, blockArrow(0)),
	}},
	{ID: "move-down", Color: blue, Parts: []Part{
		path(Body, blockArrow(180)),
	}},
	{ID: "sort", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 9, 13), path(Detail, "M3.5 4.5H8.5M3.5 7.5H8.5M3.5 10.5H8.5"),
		path(Line, "M13 2.5V13.5M11.25 11.75L13 13.5L14.75 11.75"),
	}},
	{ID: "references-to", Color: slate, Accent: red, Parts: join([]Part{
		colored(rect(Accent, 10, 1.5, 4.5, 3.5), blue),
		path(Line, "M1.5 3.25H8.5M6.75 1.5L8.5 3.25L6.75 5"),
	}, lens(6, 10, 3.5))},
	{ID: "references-in", Color: slate, Accent: green, Parts: join([]Part{
		colored(rect(Accent, 1.5, 1.5, 4.5, 3.5), blue),
		path(Line, "M7.5 3.25H14.5M12.75 1.5L14.5 3.25L12.75 5"),
	}, lens(6, 10, 3.5))},
	{ID: "filter-by-subsystems", Color: gray, Accent: amber, Parts: []Part{
		path(Body, "M1.5 2.5H11.5L7.75 7V12.5L5.25 11V7Z"),
		path(Line, "M10.5 9.5H14.5M12 12H14.5M12 14.5H14.5M12 9.5V14.5"),
	}},
	{ID: "check-package", Color: brown, Accent: green, Parts: []Part{
		path(Body, "M1.5 4.5L6.5 2L11.5 4.5V10.5L6.5 13L1.5 10.5Z"),
		path(Detail, "M1.5 4.5L6.5 7L11.5 4.5M6.5 7V13"),
		path(Mark, "M9 12L11 14L14.5 9.5"),
	}},
	{ID: "open-module", Color: slate, Parts: join([]Part{
		rect(Body, 1.5, 1.5, 11, 10), path(Detail, "M1.5 4.5H12.5M3.5 7H8M3.5 9H6.5"),
	}, over(colored(path(Body, editPencil), "#6f9a2a")))},
	{ID: "open-predefined-data", Color: blue, Parts: []Part{
		colored(bar(1.5, 1.5, 6, 2.5), "#5b9be0"), colored(bar(4.5, 5.25, 6, 2.5), "#5b9be0"), colored(bar(4.5, 9, 4, 2.5), "#5b9be0"),
		path(Detail, "M3 4V10.25H4.5M3 6.5H4.5"),
		colored(path(Body, editPencil), "#6f9a2a"),
	}},
	{ID: "characteristics", Color: purple, Accent: purple, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "additional-indexes", Color: blue, Accent: green, Parts: []Part{
		colored(rect(Accent, 5.5, 1.5, 5, 4), blue),
		path(Detail, "M8 5.5V11M2.75 11V8.5H13.25V11"),
		rect(Accent, 1, 11, 3.5, 3.5),
		rect(Accent, 6.25, 11, 3.5, 3.5),
		rect(Accent, 11.5, 11, 3.5, 3.5),
	}},
	{ID: "help-information", Color: slate, Accent: "#efc23a", Parts: []Part{
		rect(Body, 1.5, 1.5, 11, 10), path(Detail, "M1.5 4.5H12.5M3.5 7H9M3.5 9H7"),
		circle(Accent, 11.5, 11.5, 3.5),
		path(OnAccent, "M10.25 10.5Q10.25 9.25 11.5 9.25Q12.75 9.25 12.75 10.25Q12.75 11 11.5 11.5V12M11.5 13.5V13.6"),
	}},
}

// flag is the chequered flag of bookmarks; what is done to them stands at its
// lower right.
func flag() []Part {
	return []Part{
		path(Detail, "M3.5 1.5V14.5"),
		rect(Body, 3.5, 2.5, 10, 7),
		rect(Accent, 3.5, 2.5, 2.5, 2.33), rect(Accent, 8.5, 2.5, 2.5, 2.33),
		rect(Accent, 6, 4.83, 2.5, 2.33), rect(Accent, 11, 4.83, 2.5, 2.33),
		rect(Accent, 3.5, 7.17, 2.5, 2.33), rect(Accent, 8.5, 7.17, 2.5, 2.33),
	}
}

// scissors, twoPages and clipboard are cut, copy and paste: the same
// pictures stand under a breakpoint's dot in the debugger. The board of the
// clipboard is also the board of a task the library activates or executes.
func scissors() []Part {
	return []Part{
		path(Detail, "M5.5 10.5L11 1.5M10.5 10.5L5 1.5"),
		circle(Body, 4.5, 12.5, 2),
		circle(Body, 11.5, 12.5, 2),
	}
}

func twoPages() []Part {
	return []Part{
		rect(Body, 1.5, 1.5, 8, 10),
		rect(Body, 6.5, 4.5, 8, 10),
	}
}

func clipboard() []Part {
	return append(board(), colored(rect(Body, 7.5, 6.5, 7, 8), slate))
}

func board() []Part {
	return []Part{
		rect(Body, 1.5, 2.5, 10, 12),
		rect(Accent, 4, 1, 5, 3),
	}
}

// plusBadge is the green plus in a circle on what is being added, hiding
// what lies under it: at the lower right of a copy, an attribute, a column,
// at the upper left of a nested query or a description of a temporary table
// the query wizard creates.
func plusBadge(cx, cy float64) []Part {
	return badge(cx, cy, green, fmt.Sprintf("M%s %sV%sM%s %sH%s", number(cx), number(cy-2.25), number(cy+2.25), number(cx-2.25), number(cy), number(cx+2.25)))
}

// objectBadge is the gold cube of an object in the lower right corner of
// what describes it, hiding what lies under it. It is drawn by its faces
// alone - a light top, a gold left side, a dark right one - with no edges:
// at 16 px lines merge the small cube into a spot, faces of three tones
// still read as a cube (the owner's choice of 16.10.2026).
func objectBadge() []Part {
	const (
		cube  = "M12 8.5L15.5 10.25V13.75L12 15.5L8.5 13.75V10.25Z"
		top   = "M12 8.5L15.5 10.25L12 12L8.5 10.25Z"
		right = "M12 12L15.5 10.25V13.75L12 15.5Z"
	)
	return []Part{
		colored(path(Plate, cube), gold),
		colored(path(Accent, cube), "#f0c050"),
		colored(path(Accent, top), "#fae29a"),
		colored(path(Accent, right), "#d39515"),
	}
}

// tint paints every part of a group in one colour: the glass of a lens
// laid over a folder.
func tint(parts []Part, color string) []Part {
	for index, part := range parts {
		if part.Color == "" {
			parts[index] = colored(part, color)
		}
	}
	return parts
}
