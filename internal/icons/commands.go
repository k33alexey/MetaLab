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

// person is the user: a round head in skin, six pixels across, a pixel
// above solid light blue shoulders as wide as the grid allows with rounded
// corners, standing on the bottom of the grid; cx is a whole point, so that
// every edge is a border of pixels. Light, so that the user is plain and a
// sign laid over him reads on him without a halo (the owner's choice of
// 17.10.2026 - a head of five pixels read as a cross and outlined shoulders
// sank on a dark theme, and on dark blue shoulders the signs were lost).
func person(cx float64) []Part {
	return []Part{
		colored(circle(Accent, cx, 5, 3), skin),
		colored(path(Accent, fmt.Sprintf("M%s 15V12Q%s 9 %s 9H%sQ%s 9 %s 12V15Z",
			number(cx-6), number(cx-6), number(cx-3), number(cx+3), number(cx+6), number(cx+6))), personBlue),
	}
}

// printer is a printer with the paper going in at the top and the printed
// sheet coming out at the bottom: «print», and with a check on its paper
// «print the current one». printerAt is the same printer of body points
// wide and body high from left, top, the paper inset from its sides, paper
// high over it, the sheet coming two points below it: smaller, over the
// list it outputs.
func printer() []Part {
	return printerAt(1.5, 1.5, 13, 6, 3, 5)
}

func printerAt(left, top, width, body, inset, paper float64) []Part {
	right, bodyTop := left+width, top+paper
	return []Part{
		path(Detail, fmt.Sprintf("M%s %sV%sH%sV%s", number(left+inset), number(bodyTop), number(top), number(right-inset), number(bodyTop))),
		rect(Body, left, bodyTop, width, body),
		path(Detail, fmt.Sprintf("M%s %sH%sV%sH%sZ", number(left+inset), number(bodyTop+body-2), number(right-inset), number(bodyTop+body+2), number(left+inset))),
	}
}

// lockedPerson is a user with a padlock laid over the lower right of him, in
// the colour of the lock: a temporary lock grey, the locks of
// authentication gold. The person keeps the size of «users», as the
// prototype keeps it under every sign.
func lockedPerson(lock string) []Part {
	return join(person(7), []Part{
		colored(path(Detail, padlock), lock),
		colored(path(Plate, "M9.5 11.5H14.5V14.5H9.5Z"), lock),
		colored(rect(Accent, 9, 11, 6, 4), lock),
	})
}

// editingPencil is the pencil of «change», alone, filling the grid from
// corner to corner, and across the sheet of a spreadsheet being edited
// (the owner's choice of 17.10.2026: smaller it left three pixels empty).
func editingPencil() []Part {
	return pencilAt(1, 15, 17.5, 2.1, 1.3)
}

// pencilAt is a pencil as the prototype draws it, its point at x, y, lying
// up and to the right length points long and twice half wide: a dark lead,
// a cone of bare wood, a green body and a dark cap, all of one width and no
// outline, solid, so that nothing shows through it (the owner's choice of
// 16.10.2026, for every pencil of the set). The lead and the cap lighten on
// a dark theme as ink does, else they vanish on a dark page. ends scales
// the lead, the cone and the cap, so that a larger pencil keeps their
// proportions.
func pencilAt(x, y, length, half, ends float64) []Part {
	cone, lead, cap := 3.2*ends, 1.3*ends, 2.2*ends
	at := func(along, across float64) [2]float64 {
		return [2]float64{round(x + (along+across)*math.Sqrt2/2), round(y + (across-along)*math.Sqrt2/2)}
	}
	band := func(from, to float64) string {
		return polygon(at(from, -half), at(to, -half), at(to, half), at(from, half))
	}
	tip := half * lead / cone
	return []Part{
		colored(path(Accent, polygon(at(0, 0), at(cone, -half), at(cone, half))), "#e9b97a"),
		colored(path(InkSpot, polygon(at(0, 0), at(lead, -tip), at(lead, tip))), black),
		colored(path(Accent, band(cone, length-cap)), "#43b04f"),
		colored(path(InkSpot, band(length-cap, length)), "#4f5963"),
	}
}

// floppy is the diskette of writing: «save», and under the sign of what is
// written - the gear of a report's settings, the «OK» of ending an edit.
func floppy() []Part {
	return []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M4.5 1.5V5.5H11.5V1.5M4.5 14.5V9.5H11.5V14.5"),
		rect(Accent, 8, 2, 2, 3),
	}
}

// calculator is a calculator with its display and nine keys: the
// configurator's own, painted, and the grey one of an input field.
func calculator() []Part {
	return []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		rect(Accent, 4, 3, 8, 3),
		path(Detail, "M4.5 8.5H5.5M7.5 8.5H8.5M10.5 8.5H11.5M4.5 10.5H5.5M7.5 10.5H8.5M10.5 10.5H11.5M4.5 12.5H5.5M7.5 12.5H8.5M10.5 12.5H11.5"),
	}
}

// blockArrow is the solid arrow of moving an item, turned by angle degrees
// clockwise from pointing up, in steps of 45: a wide head on a narrower
// shaft, through the middle of the grid. Moving up, down, left and right and
// the arrows of appearance, straight and inclined, are this one arrow (the
// prototype draws them alike). The inclined one is drawn as its own shape
// and not turned from the straight one: turned by 45 degrees the straight
// arrow's corners fall between pixels, here the back of its head is a row
// and a column of pixels and its sides go through the middles of pixels.
// Either is then turned by quarters round the middle of the grid, which
// keeps a corner on the half-pixel grid.
// solidArrow is the arrow of moving: a block arrow filled solid with a
// darker rim, which reads at 16 pixels where an outline did not (the
// owner's remark of 19.10.2026 on «Переместить вверх/вниз»).
func solidArrow(angle float64) []Part {
	return solidPath(blockArrow(angle))
}

// solidPath is a shape filled solid in the accent with a darker rim.
func solidPath(shape string) []Part {
	return []Part{path(Accent, shape), path(Rim, shape)}
}

func blockArrow(angle float64) string {
	corners := [][2]float64{{8, 1.5}, {14, 7.5}, {10.5, 7.5}, {10.5, 14.5}, {5.5, 14.5}, {5.5, 7.5}, {2, 7.5}}
	if math.Mod(angle, 90) != 0 {
		corners = [][2]float64{{12.5, 3.5}, {12.5, 11.5}, {10.5, 9.5}, {5.5, 14.5}, {1.5, 10.5}, {6.5, 5.5}, {4.5, 3.5}}
		angle -= 45
	}
	for range int(math.Mod(angle+360, 360) / 90) {
		for index, corner := range corners {
			corners[index] = [2]float64{16 - corner[1], corner[0]}
		}
	}
	return polygon(corners...)
}

// wideArrow is the wide arrow of «Back» and «Forward», also of sorting a
// list (the owner's choice of 17.10.2026: a thin arrow beside the list was
// lost): its point at x, y, pointing along dx or dy, each -1, 0 or 1, a
// shaft two pixels thick and a head of 45° six wide and three deep, length
// points from the point to the end of the shaft. wideArrows is the same
// arrow with a head at both ends, length points from point to point.
func wideArrow(x, y, dx, dy, length float64) string {
	return alongArrow(x, y, dx, dy, [][2]float64{{0, 0}, {3, -3}, {3, -1}, {length, -1}, {length, 1}, {3, 1}, {3, 3}})
}

func wideArrows(x, y, dx, dy, length float64) string {
	back := length - 3
	return alongArrow(x, y, dx, dy, [][2]float64{{0, 0}, {3, -3}, {3, -1}, {back, -1}, {back, -3}, {length, 0}, {back, 3}, {back, 1}, {3, 1}, {3, 3}})
}

// alongArrow lays the points of an arrow, given back from its point and
// across its shaft, from its point at x, y pointing along dx or dy.
func alongArrow(x, y, dx, dy float64, points [][2]float64) string {
	for index, point := range points {
		points[index] = [2]float64{x - dx*point[0] + dy*point[1], y - dy*point[0] + dx*point[1]}
	}
	return polygon(points...)
}

// head is the solid head of an arrow, its point at x, y, pointing along dx,
// dy, each -1, 0 or 1 (the prototype ends every arrow so, and a head of two
// thin strokes was lost at 16 px - the owner's remark of 16.10.2026, the
// arrow of «Run to cursor» the sample). Along an axis it is size points to
// either side of the shaft and 2×size-0.5 long, so that with its point in
// the middle of a pixel its back is on the border of two; slanting, it is a
// right triangle with its sides of 2×size-1 along the axes from the point.
func head(x, y, dx, dy, size float64) string {
	switch {
	case dx != 0 && dy != 0:
		legs := 2*size - 1
		return polygon([2]float64{x, y}, [2]float64{x - dx*legs, y}, [2]float64{x, y - dy*legs})
	case dx != 0:
		back := x - dx*(2*size-0.5)
		return polygon([2]float64{x, y}, [2]float64{back, y - size}, [2]float64{back, y + size})
	default:
		back := y - dy*(2*size-0.5)
		return polygon([2]float64{x, y}, [2]float64{x - size, back}, [2]float64{x + size, back})
	}
}

// plus is the plus of the set - of «Add», of a new window, of what is
// made anew, of the wizards: a solid cross two pixels thick with square
// ends, its arms arm points from its middle at cx, cy, whole points both,
// so that every edge is a border of pixels. The round ends of a mark ate
// half a pixel of each arm (the owner's remark of 16.10.2026); minus is its
// minus.
func plus(cx, cy, arm float64) string {
	return fmt.Sprintf("M%s %sH%sV%sH%sV%sH%sV%sH%sV%sH%sV%sH%sZ",
		number(cx-1), number(cy-arm), number(cx+1), number(cy-1), number(cx+arm), number(cy+1), number(cx+1),
		number(cy+arm), number(cx-1), number(cy+1), number(cx-arm), number(cy-1), number(cx-1))
}

func minus(cx, cy, arm float64) string {
	return fmt.Sprintf("M%s %sH%sV%sH%sZ", number(cx-arm), number(cy-1), number(cx+arm), number(cy+1), number(cx-arm))
}

// markedRows are the two rows of a sheet, each a coloured square and its
// line: the points of a task, the objects of the report of a configuration.
func markedRows(first, second string) []Part {
	return []Part{
		path(Detail, "M8.5 6.5H11.5M8.5 10.5H11.5"),
		colored(rect(Accent, 4, 5, 3, 3), first), colored(rect(Accent, 4, 9, 3, 3), second),
	}
}

func join(groups ...[]Part) []Part {
	var parts []Part
	for _, group := range groups {
		parts = append(parts, group...)
	}
	return parts
}

const (
	skin = "#e8a46a"
	// brightGreen is the green of an arrow over the lines of a list, where
	// the green of the set is too dark.
	brightGreen = "#2e9e2e"
	// sparkGold is the gold of the sparks of a wizard.
	sparkGold = "#e6a600"
	// personBlue is the shirt of a user, light, so that a sign over it reads.
	personBlue = "#9cc3ec"
	// litGreen is the green of the set made lighter for an arrow held in a
	// black edge on a blue cylinder: there the same green looks darker than
	// on a light page, and this one looks as the plus of a new window does
	// (the owner's choice of 16.10.2026).
	litGreen = "#68b552"
	// steelInk is the grey of the blades of the scissors.
	steelInk = "#6f7a86"
	purple   = "#8a6bbf"
	cross    = "M4 4L12 12M12 4L4 12"
	tree     = "M2.5 2.5V10.5M2.5 4.5H5.5M2.5 7.5H5.5M2.5 10.5H5.5"
	padlock  = "M10.5 11.5V10Q10.5 8.5 12 8.5Q13.5 8.5 13.5 10V11.5"
	// base, lid and inArrow are the cylinder of the database and the arrow
	// going down into it under its lid of «Update the configuration of the
	// database».
	base    = "M2.5 3.5V13A5.5 2 0 0 0 13.5 13V3.5A5.5 2 0 0 1 2.5 3.5Z"
	lid     = "M2.5 3.5A5.5 2 0 0 1 13.5 3.5A5.5 2 0 0 1 2.5 3.5Z"
	inArrow = "M6.5 3.5H9.5V8.5H11.5L8 12L4.5 8.5H6.5Z"
)

var commandIcons = []Icon{
	// Файл.
	{ID: "file-new", Color: slate, Parts: []Part{
		path(Body, page), path(Detail, pageFold),
	}},
	{ID: "file-open", Color: groupColor, Accent: blue, Parts: []Part{
		path(Body, "M1.5 6.5H5.5L7.5 8.5H14.5V14.5H1.5Z"),
		path(Line, "M10.5 6.5V4"), path(Accent, head(10.5, 0.5, 0, -1, 2)),
	}},
	// The folder of «Открыть» with the arrow going down into it to its
	// bottom, in the colours of «Открыть»: the prototype's grey is its
	// unavailable state, which read as a disabled command (the owner's
	// remarks of 19.10.2026); the folder ten points high at its left, the
	// edge of the folder clearing round the arrow.
	{ID: "file-close", Color: groupColor, Accent: blue, Parts: append([]Part{path(Body, "M1.5 4.5H5.5L7.5 6.5H14.5V14.5H1.5Z")},
		thinlyCleared(path(Line, "M10.5 0.5V10"), path(Accent, head(10.5, 13.5, 0, 1, 2)))...)},
	// A white sheet of XML in a blue edge: the small black brackets of a
	// tag in its upper half and at its foot a bold green arrow down -
	// loading - outlined in white at its sides and below, across the edge
	// of the sheet, but not above, where it would wipe out the tag (the
	// owner's picture and choices of 19.10.2026, docs/загрузить из хмл.png,
	// drawn our own way; the band with the letters XML did not read at 16
	// points and is left out).
	{ID: "load-xml", Color: sea, Accent: sea, Parts: []Part{
		colored(path(Accent, "M2 1H10L12 3V14H2Z"), "#ffffff"),
		path(Line, "M1.5 0.5H9.5L12.5 3.5V14.5H1.5Z"), path(Line, "M9.5 0.5V3.5H12.5"),
		colored(path(Line, "M5 5.5L4 6.5L5 7.5M9 5.5L10 6.5L9 7.5M7.5 5L6.5 8"), "#222222"),
		colored(path(Accent, "M5 9H9V11H12L7 16L2 11H5Z"), "#ffffff"),
		colored(path(Accent, "M6 9H8V12H10L7 15L4 12H6Z"), green),
	}},

	{ID: "save", Color: blue, Accent: slate, Parts: floppy()},
	{ID: "compare-files", Color: slate, Parts: join([]Part{
		rect(Body, 1.5, 1.5, 7, 9), path(Detail, "M3.5 4.5H6.5M3.5 6.5H6.5"),
	}, lens(10.5, 10.5, 3)),
	},
	{ID: "print", Color: gray, Parts: printer()},
	{ID: "print-current", Color: gray, Accent: green, Parts: append(printer(),
		path(Mark, "M5.75 4L7.25 5.5L10.25 2.5"),
	)},
	{ID: "print-preview", Color: slate, Parts: join([]Part{
		path(Body, "M1.5 1.5H7.5L10.5 4.5V13.5H1.5Z"), path(Detail, "M7.5 1.5V4.5H10.5"),
	}, lens(10.5, 10.5, 3)),
	},
	{ID: "page-setup", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		path(Line, "M4.5 1.5V14.5M11.5 1.5V14.5M2.5 4.5H13.5M2.5 11.5H13.5"),
	}},
	// Правка.
	// «Отменить» and «Вернуть» are the prototype's arrows: a large wide head
	// and a short tail bending down from it and narrowing to its end, solid,
	// in the same blue; an unavailable command greys it (the owner's
	// snapshot and words of 19.10.2026).
	{ID: "undo", Color: steel, Accent: steel, Parts: []Part{path(Accent, "M1 7L7 1V4.5Q12 4.5 13.5 9Q14.5 12 13 15Q12 11 10 10Q8.5 9.5 7 9.5V13Z")}},
	{ID: "redo", Color: steel, Accent: steel, Parts: []Part{path(Accent, "M15 7L9 1V4.5Q4 4.5 2.5 9Q1.5 12 3 15Q4 11 6 10Q7.5 9.5 9 9.5V13Z")}},

	{ID: "cut", Color: slate, Parts: scissors()},
	{ID: "copy", Color: slate, Parts: twoPages()},
	{ID: "paste", Color: gray, Accent: slate, Parts: clipboard()},
	{ID: "find", Color: slate, Parts: lens(6, 6, 4.5)},
	{ID: "find-next", Color: slate, Accent: steel, Parts: join(lens(7, 9, 3.5), []Part{
		path(Line, "M2.5 4.5Q7.5 0.5 12.5 4.5"), path(Accent, head(13, 5, 1, 1, 2)),
	})},
	{ID: "find-previous", Color: slate, Accent: steel, Parts: join(lens(7, 9, 3.5), []Part{
		path(Line, "M12.5 4.5Q7.5 0.5 2.5 4.5"), path(Accent, head(2, 5, -1, 1, 2)),
	})},
	// The same picture stands on the menu «Правка» and in the library: the
	// letter A found and replaced by B.
	{ID: "replace", Color: slate, Accent: blue, Parts: join(lens(7, 9, 3.5), []Part{
		colored(path(Line, "M1 6.5L2.75 1.5L4.5 6.5M1.7 4.5H3.8"), red),
		path(Line, "M11.5 6.5V1.5H13.25Q14.5 1.5 14.5 2.5Q14.5 3.5 13.25 3.5H11.5M13.25 3.5Q15 3.5 15 5Q15 6.5 13.25 6.5H11.5"),
	})},
	{ID: "global-search", Color: groupColor, Parts: join([]Part{
		path(Body, "M1.5 2.5H5.5L7.5 4.5H13.5V12.5H1.5Z"),
	}, tint(lens(10.5, 10.5, 3), slate))},
	{ID: "bookmarks", Color: gray, Accent: "#5b636d", Parts: flag()},
	{ID: "interface-texts", Color: green, Accent: red, Parts: append([]Part{
		path(Line, "M1.5 2.5H9.5M5.5 2.5V11.5"),
	}, pencilAt(7.5, 14.5, 9.5, 1.3, 1)...)},
	{ID: "find-in-tree", Color: slate, Parts: []Part{
		path(Detail, tree),
		path(Body, "M7.5 5.5V14L9.5 12L11 15L12.5 14.25L11 11.5H13.5Z"),
	}},
	{ID: "additional", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 3.5, 3.5, 9, 7), path(Detail, "M3.5 5.5H12.5"),
		rect(Body, 5.5, 7.5, 9, 7), path(Detail, "M5.5 9.5H14.5"),
		path(Accent, plus(4, 4, 3)),
	}},
	// The sheet of properties, as the palette shows them - the names at the
	// left, the fields of the values at the right - with the cube of the
	// object they describe in its lower right corner. The prototype draws a
	// hand holding a letter; the owner chose the sheet and the cube
	// (16.10.2026). The cube is a badge only: alone it is the cube of an
	// external source, and boxed with tape the package of XDTO.
	{ID: "properties", Color: slate, Accent: "#8fb8e0", Parts: join([]Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M1.5 4.5H14.5M7.5 4.5V14.5M3.5 6.5H5.5M3.5 9.5H5.5M3.5 12.5H5.5"),
		rect(Accent, 9, 6, 4, 2), rect(Accent, 9, 9, 3, 2),
	}, objectBadge())},
	// Конфигурация.
	{ID: "configuration-open", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window+"M4.5 7.5V11.5M4.5 9.5H7.5M4.5 11.5H7.5"),
	}},
	{ID: "configuration-close", Color: slate, Parts: join([]Part{
		rect(Body, 1.5, 4.5, 11, 10),
		path(Detail, "M1.5 7.5H12.5M4.5 9.5V12.5M4.5 10.5H7.5M4.5 12.5H7.5"),
	}, []Part{colored(path(InkMark, "M11 1.5L14.5 5M14.5 1.5L11 5"), black)})},
	{ID: "configuration-save", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window),
		colored(rect(Body, 6.5, 7.5, 8, 7), blue),
		path(Line, "M8.5 7.5V10.5H12.5V7.5"),
	}},
	// Updating the configuration of the database - the button pressed most
	// often - is drawn louder than the set and our own way, not the
	// prototype's: a light cylinder with a white lid and a darker band at
	// its right, a clean blue edge, and a green arrow going down into it under
	// the lid, outlined in black on either theme so that it reads at 16 px
	// (the owner's choice of 16.10.2026).
	{ID: "database-update", Color: blue, Accent: green, Parts: []Part{
		colored(path(Accent, base), "#b5d6f6"), colored(path(Accent, "M11 5.18V14.68A5.5 2 0 0 0 13.5 13V3.5A5.5 2 0 0 1 11 5.18Z"), "#7fb3ea"),
		colored(path(Accent, lid), "#ffffff"), colored(path(Line, base), "#2f6db5"), colored(path(Line, lid), "#2f6db5"),
		colored(path(Accent, inArrow), litGreen), colored(path(Line, inArrow), black),
	}},
	{ID: "compare-merge", Color: slate, Accent: groupColor, Parts: join([]Part{
		path(Detail, tree),
		path(Accent, "M9 1H11L12 2H15V6H9Z"),
	}, lens(9.5, 9.5, 3))},
	{ID: "compare-configurations", Color: slate, Parts: join([]Part{
		path(Detail, tree),
	}, lens(9, 8, 3.5))},
	{ID: "configuration-report", Color: slate, Parts: join([]Part{path(Body, page), path(Detail, pageFold)}, markedRows(red, blue))},
	// The lines of the configuration and two checks, green above and red
	// below: what passed and what did not (the owner's choice of 17.10.2026).
	{ID: "check-configuration", Color: slate, Accent: red, Parts: []Part{
		path(Detail, "M1.5 3.5H6.5M3.5 6.5H6.5M1.5 9.5H6.5M3.5 12.5H6.5"),
		colored(path(Mark, "M8 4.5L10 6.5L14 2.5"), green), path(Mark, "M8 11.5L10 13.5L14 9.5"),
	}},
	{ID: "configuration-window", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window+"M6.5 5.5V13.5M9 8.5H12.5M9 11.5H12.5"),
		path(Line, "M3.5 8.5V12.5M3.5 10.5H5"),
	}},
	// Отладка и Сервис.
	{ID: "run", Color: gold, Accent: blue, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		path(Accent, "M6 5L11 8L6 11Z"),
	}},
	{ID: "performance-measurement", Color: steel, Accent: steel, Parts: []Part{
		circle(Body, 7.5, 9.5, 5),
		path(Detail, "M7.5 9.5V6.5M7.5 9.5L9.5 11M6 1.5H9M7.5 1.5V4.5M12 4L13 5"),
	}},
	{ID: "record-actions", Color: gold, Accent: red, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		circle(Accent, 8, 8, 3),
	}},
	{ID: "calculator", Color: brown, Accent: green, Parts: calculator()},
	{ID: "calendar", Color: slate, Accent: red, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 12),
		rect(Accent, 2, 3, 12, 3),
		path(Detail, "M4.5 1V3.5M11.5 1V3.5M4.5 8.5H5.5M7.5 8.5H8.5M10.5 8.5H11.5M4.5 11.5H5.5M7.5 11.5H8.5"),
		rect(Accent, 10, 10, 3, 3),
	}},
	{ID: "text-templates", Color: brown, Accent: blue, Parts: []Part{
		path(Body, "M3.5 1.5H13.5V14.5H3.5Q2.5 14.5 2.5 13.5V2.5Q2.5 1.5 3.5 1.5Z"),
		path(Detail, "M4.5 1.5V14.5"),
		path(Line, "M6.5 4.5H11.5M6.5 7.5H11.5M6.5 10.5H9.5"),
	}},
	{ID: "temporary-lock", Color: blue, Accent: gray, Parts: lockedPerson(gray)},
	{ID: "settings", Color: gray, Parts: []Part{
		path(Body, "M10.5 1.5A4 4 0 0 0 7 7L1.75 12.25A1.5 1.5 0 0 0 3.75 14.25L9 9A4 4 0 0 0 14.5 5.5L12.25 7.25L10 6L9.25 3.75Z"),
	}},
	// Администрирование.
	{ID: "users", Color: blue, Parts: person(8)},
	// A user online: the green dot of a messenger at the person's side, where
	// the prototype has a monitor (the owner's choice of 16.10.2026).
	{ID: "active-users", Color: blue, Accent: green, Parts: join(person(7), []Part{
		colored(circle(Plate, 12, 12, 2.5), green), circle(Accent, 12, 12, 2),
	})},
	{ID: "event-log", Color: blue, Parts: []Part{
		rect(Body, 1.5, 3.5, 13, 9),
		circle(Detail, 5, 8, 1.5), circle(Detail, 11, 8, 1.5),
		path(Detail, "M5 9.5H11M3.5 3.5V5.5M12.5 3.5V5.5"),
	}},
	{ID: "authentication-locks", Color: blue, Accent: gold, Parts: lockedPerson(gold)},
	// Окна.
	// The title of a new window is a row narrower than a window's, so that
	// the plus stands a pixel clear of it and of the bottom alike.
	{ID: "window-new", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11), path(Detail, "M1.5 4.5H14.5"), path(Accent, plus(8, 9, 3)),
	}},
	{ID: "split-horizontal", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 12), path(Detail, "M1.5 8.5H14.5"),
	}},
	{ID: "split-vertical", Color: slate, Parts: []Part{
		rect(Body, 2.5, 2.5, 12, 11), path(Detail, "M8.5 2.5V13.5"),
	}},
	{ID: "service-messages", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 8, 13), path(Detail, "M3.5 4.5H7.5M3.5 7.5H7.5M3.5 10.5H6"),
		path(Accent, "M11 6H13.25L12 12.25Q11.85 13 12.4 13H13.25L13 14H11.4Q9.85 14 10.15 12.75L11.15 7H10.75Z"),
		circle(Accent, 13, 4, 1),
	}},
	{ID: "navigate-back", Color: slate, Accent: "#2f6fb5", Parts: []Part{
		rect(Body, 3.5, 1.5, 11, 7), path(Detail, "M3.5 4.5H14.5"),
		path(Accent, wideArrow(1, 12, -1, 0, 11)),
	}},
	{ID: "navigate-forward", Color: slate, Accent: "#2f6fb5", Parts: []Part{
		rect(Body, 1.5, 1.5, 11, 7), path(Detail, "M1.5 4.5H12.5"),
		path(Accent, wideArrow(15, 12, 1, 0, 11)),
	}},
	{ID: "windows", Color: slate, Parts: []Part{
		rect(Body, 1.5, 1.5, 9, 8), path(Detail, "M1.5 4.5H10.5"),
		rect(Body, 5.5, 6.5, 9, 8), path(Detail, "M5.5 9.5H14.5"),
	}},
	// Справка.
	{ID: "help", Color: gold, Accent: "#efc23a", Parts: []Part{
		circle(Accent, 7.5, 7.5, 6.5),
		path(OnAccent, question),
	}},
	// The user of «users» in a cap of a graduate (the owner's remark of
	// 17.10.2026: the old figure was left behind).
	{ID: "syntax-assistant", Color: gold, Accent: "#2f4f7f", Parts: join(person(8), []Part{
		path(Accent, "M2.5 2L8 0L13.5 2L8 4Z"),
		path(Line, "M12.5 2.5V5.5"),
	})},
	{ID: "syntax-search", Color: slate, Accent: blue, Parts: join([]Part{
		path(Line, "M1.5 3.5Q1.5 1.5 3.5 1.5Q5.5 1.5 5.5 3.25Q5.5 4.5 3.5 5.5V7M3.5 9.25V9.5"),
	}, lens(9.5, 8.5, 4))},
	{ID: "about", Color: gold, Accent: "#efc23a", Parts: []Part{
		circle(Accent, 7.5, 7.5, 6.5),
		path(OnAccent, "M7.5 6.5V11M7.5 4V4.5"),
	}},
	// Действия дерева и его контекстного меню.
	{ID: "add", Color: green, Accent: green, Parts: []Part{circle(Body, 8, 8, 6.5), path(Accent, plus(8, 8, 4))}},
	{ID: "change", Color: "#6f9a2a", Accent: "#efc23a", Parts: editingPencil()},
	{ID: "clone", Color: slate, Accent: green, Parts: join([]Part{
		path(Body, "M2.5 1.5H8L10.5 4V12.5H2.5Z"),
	}, plusBadge(11.5, 11.5))},
	{ID: "delete", Color: red, Accent: red, Parts: []Part{
		path(Mark, cross),
	}},
	{ID: "move-up", Color: blue, Accent: blue, Parts: solidArrow(0)},
	{ID: "move-down", Color: blue, Accent: blue, Parts: solidArrow(180)},
	{ID: "sort", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 9, 13), path(Detail, "M3.5 4.5H8.5M3.5 7.5H8.5M3.5 10.5H8.5"),
		path(Accent, wideArrow(13, 15, 0, 1, 13)),
	}},
	{ID: "references-to", Color: slate, Accent: red, Parts: join([]Part{
		colored(rect(Accent, 10, 2, 5, 3), blue),
		path(Line, "M1.5 3.5H6"), path(Accent, head(9.5, 3.5, 1, 0, 2)),
	}, lens(6, 10, 3.5))},
	{ID: "references-in", Color: slate, Accent: green, Parts: join([]Part{
		colored(rect(Accent, 1, 2, 5, 3), blue),
		path(Line, "M7.5 3.5H12"), path(Accent, head(15.5, 3.5, 1, 0, 2)),
	}, lens(6, 10, 3.5))},
	// The funnel of every selection and, beside its neck, three short dashes
	// of the attribute, the form field and the characteristic: the list of
	// what it selects by, as the prototype draws it (the owner's choice of
	// 16.10.2026).
	{ID: "filter-by-subsystems", Color: gray, Accent: amber, Parts: append(filterOver(1.5, 1.5, 9),
		colored(bar(9, 7, 6, 2), slate), colored(bar(9, 10, 6, 2), red), colored(bar(9, 13, 6, 2), purple),
	)},
	{ID: "check-package", Color: brown, Accent: green, Parts: []Part{
		path(Body, "M1.5 4.5L6.5 2L11.5 4.5V10.5L6.5 13L1.5 10.5Z"),
		path(Detail, "M1.5 4.5L6.5 7L11.5 4.5M6.5 7V13"),
		path(Mark, "M9 12L11 14L14.5 9.5"),
	}},
	// The page of a module at its full size and the pencil over it, cleared
	// by a pixel: the prototype lays its pencil over its picture of a module
	// (the owner's choice of 17.10.2026).
	{ID: "open-module", Color: slate, Accent: code, Parts: join(modulePage(), thinlyCleared(pencilAt(7.5, 15, 9.5, 1.3, 1)...))},
	{ID: "open-predefined-data", Color: blue, Parts: append([]Part{
		colored(bar(2, 1, 6, 3), "#5b9be0"), colored(bar(5, 5, 6, 3), "#5b9be0"), colored(bar(5, 9, 4, 3), "#5b9be0"),
		path(Detail, "M3.5 4V10.5H5M3.5 6.5H5"),
	}, pencilAt(7.5, 15, 9.5, 1.3, 1)...)},
	{ID: "characteristics", Color: purple, Accent: purple, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "additional-indexes", Color: blue, Accent: green, Parts: []Part{
		colored(rect(Accent, 6, 1, 5, 4), blue),
		path(Detail, "M8.5 5V11M3.5 11V8.5H13.5V11"),
		rect(Accent, 2, 11, 3, 4),
		rect(Accent, 7, 11, 3, 4),
		rect(Accent, 12, 11, 3, 4),
	}},
	{ID: "help-information", Color: slate, Accent: "#efc23a", Parts: []Part{
		rect(Body, 1.5, 1.5, 11, 10), path(Detail, "M1.5 4.5H12.5M3.5 6.5H9M3.5 8.5H7"),
		circle(Accent, 11.5, 11.5, 3.5),
		path(OnAccent, "M10.25 10.5Q10.25 9.25 11.5 9.25Q12.75 9.25 12.75 10.25Q12.75 11 11.5 11.5V12M11.5 13.5V13.6"),
	}},
}

// flag is the chequered flag of bookmarks; what is done to them stands at its
// lower right. Inside its frame it is five squares of two pixels wide and
// three high, so that no square lies over the frame, and white between the
// squares on a dark theme too, a chequered flag being black and white (the
// owner's remarks of 17.10.2026).
func flag() []Part {
	return []Part{
		path(Detail, "M3.5 1.5V14.5"),
		box(Body, 3.5, 1.5, 11, 7), colored(rect(Accent, 4, 2, 10, 6), "#ffffff"),
		rect(Accent, 4, 2, 2, 2), rect(Accent, 8, 2, 2, 2), rect(Accent, 12, 2, 2, 2),
		rect(Accent, 6, 4, 2, 2), rect(Accent, 10, 4, 2, 2),
		rect(Accent, 4, 6, 2, 2), rect(Accent, 8, 6, 2, 2), rect(Accent, 12, 6, 2, 2),
	}
}

// scissors, twoPages and clipboard are cut, copy and paste: the same
// pictures stand under a breakpoint's dot in the debugger. The board of the
// clipboard is also the board of a task the library activates or executes.
//
// The scissors are drawn as they are: two solid steel blades, wide at the
// handles and sharp at their points, crossing at a dark pivot, and the blue
// rings of the handles with a gap between them, a little over a pixel thick
// (the owner's choice of 17.10.2026 - blades of two crossed lines looked
// strange, rings of two pixels heavy). The blades lighten on a dark theme
// as ink does, the rings keep their blue, which reads on both.
func scissors() []Part {
	parts := []Part{
		colored(path(InkSpot, blade(5.5, 10.5, 12, 1, 1.3)), steelInk),
		colored(path(InkSpot, blade(10.5, 10.5, 4, 1, 1.3)), steelInk),
	}
	for _, cx := range []float64{4.5, 11.5} {
		parts = append(parts, colored(circle(Accent, cx, 12.5, 3), blue), colored(circle(Plate, cx, 12.5, 1.9), blue))
	}
	return append(parts, colored(circle(Accent, 8, 6.5, 1), "#3d4650"))
}

// blade is a solid wedge from its base, half points to either side of bx,
// by, to its point at tx, ty: a blade of the scissors.
func blade(bx, by, tx, ty, half float64) string {
	dx, dy := tx-bx, ty-by
	length := math.Hypot(dx, dy)
	nx, ny := -dy/length*half, dx/length*half
	return polygon([2]float64{round(bx + nx), round(by + ny)}, [2]float64{tx, ty}, [2]float64{round(bx - nx), round(by - ny)})
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
	return badge(cx, cy, green, fmt.Sprintf("M%s %sV%sM%s %sH%s", number(cx), number(cy-2), number(cy+2), number(cx-2), number(cy), number(cx+2)))
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
