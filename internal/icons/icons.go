// Package icons draws the icons of Studio and the application: one table of
// shapes and one rule that colours them (docs/requirements/ICONS.md, «Визуальный
// язык»; the approved sample is docs/requirements/assets/icons-style.html).
//
// An icon is a few shapes on a 16×16 grid, each in a role. The roles, not the
// icons, decide how a shape is painted: the body is the colour of the kind,
// outlined and filled thinly; details are lines of that colour; an accent is a
// solid spot of its own colour, the one a kind is told by at a glance. So an
// icon carries only its colours and its geometry, and the whole set changes
// style in one place - the rule in sprite.go.
//
// Metaphor and colour come from the prototype's tree as the inventory of block
// 00 photographed it; the drawing is our own. An icon the inventory has no
// picture of is drawn by its meaning, by the owner's decision of 15.10.2026.
package icons

import (
	"fmt"
	"math"
	"slices"
)

// Role is how a shape is painted.
type Role int

const (
	// Body is the shape of the thing: outlined in the colour of the kind and
	// filled with it thinly.
	Body Role = iota
	// Detail is a line inside or beside the body, in the colour of the kind.
	Detail
	// Accent is a solid spot of the accent colour.
	Accent
	// Line is a line in the accent colour.
	Line
	// OnAccent is a dark mark drawn over an accent.
	OnAccent
	// Mark is a sign standing on its own - a check, a cross, a plus: a thick
	// line in the accent colour, of one weight in every icon. Arrows stay
	// lines.
	Mark
	// Plate is a body that hides what lies under it: filled with the
	// background of the page, outlined in the colour of the kind - the badge
	// of a reference laid over the icon of a kind.
	Plate
	// Rim is the edge of a solid accent - the rim of a coin: a line in a
	// slightly darker shade of the accent, lighter than OnAccent.
	Rim
	// InkMark is a mark in the colour of the icon, as thick as Mark, that
	// lightens on a dark theme, more than an outline does: the black cross of
	// «close», the letter of «choose type», which would vanish on a dark page.
	InkMark
	// InkSpot is a solid spot in the colour of the icon that lightens on a
	// dark theme as an ink mark does: the arrow of sorting, dark on a light
	// page and nearly white on a dark one.
	InkSpot
)

// Part is one shape of an icon in its role. Exactly one of the geometries is
// set: a rectangle, a circle or a path in the 16×16 grid.
type Part struct {
	Role   Role
	Rect   *[4]float64 // x, y, width, height
	Circle *[3]float64 // cx, cy, r
	Path   string
	// Color overrides the colour of the icon for this part, in its role: the
	// bars of a report are red, green and orange; the folder of a group beside
	// a report keeps the colour of a group.
	Color string
}

// Icon is one picture: the colour of what it shows, the colour of its
// accent and its shapes, drawn in order.
type Icon struct {
	ID     string
	Color  string
	Accent string
	Parts  []Part
}

func rect(role Role, x, y, width, height float64) Part {
	return Part{Role: role, Rect: &[4]float64{x, y, width, height}}
}

func circle(role Role, cx, cy, r float64) Part {
	return Part{Role: role, Circle: &[3]float64{cx, cy, r}}
}

func path(role Role, d string) Part {
	return Part{Role: role, Path: d}
}

// bar is the dash of an attribute and of everything drawn like one - a
// resource, a characteristic, a standard attribute, an enumeration value: one
// solid shape with the same rounding, told apart by its colour alone (the
// owner's decision of 15.10.2026), standing or lying.
func bar(x, y, width, height float64) Part {
	const r = 1
	return path(Accent, fmt.Sprintf("M%s %sH%sQ%s %s %s %sV%sQ%s %s %s %sH%sQ%s %s %s %sV%sQ%s %s %s %sZ",
		number(x+r), number(y), number(x+width-r), number(x+width), number(y), number(x+width), number(y+r),
		number(y+height-r), number(x+width), number(y+height), number(x+width-r), number(y+height),
		number(x+r), number(x), number(y+height), number(x), number(y+height-r),
		number(y+r), number(x), number(y), number(x+r), number(y)))
}

// barStack is three dashes one over another: a standard tabular section,
// and the tab of a list of elements, attributes, commands or parameters of a
// form, in their colour.
func barStack() []Part {
	return []Part{bar(2, 1.5, 12, 3.5), bar(2, 6.25, 12, 3.5), bar(2, 11, 12, 3.5)}
}

// coinStack is coins lying one on another, seen a little from above: the top
// of the upper one at top, each coin a solid gold disc with a rim. The rims
// lie on half pixels, so that at 16 px each is one sharp row of pixels
// instead of two faint ones (the owner's choice of 15.10.2026).
func coinStack(cx, top float64, count int) []Part {
	const rx, ry, thickness = 4.5, 1.75, 2
	var parts []Part
	for i := count - 1; i >= 0; i-- {
		y := top + float64(i)*thickness
		left, right := number(cx-rx), number(cx+rx)
		arc := fmt.Sprintf("A%s %s 0 0 0 ", number(rx), number(ry))
		edge := fmt.Sprintf("M%s %sV%s%s%s %sV%s", left, number(y), number(y+thickness), arc, right, number(y+thickness), number(y))
		parts = append(parts,
			path(Accent, edge+arc+left+" "+number(y)+"Z"),
			path(Rim, edge+arc+left+" "+number(y)+"Z"))
		if i == 0 {
			parts = append(parts, path(Rim, fmt.Sprintf("M%s %s%s%s %s", left, number(y), arc, right, number(y))))
		}
	}
	return parts
}

// drum is a cylinder standing upright, its top an ellipse of half-height
// ry: a source of data, a storage of settings, the nodes of an exchange.
func drum(left, top, width, height, ry float64) []Part {
	right, bottom, cx := number(left+width), number(top+height), number(left+width/2)
	l, upper, lower, rim := number(left), number(top), number(top+ry), number(top+2*ry)
	return []Part{
		path(Body, fmt.Sprintf("M%s %sV%sQ%s %s %s %sQ%s %s %s %sV%sQ%s %s %s %sQ%s %s %s %sZ",
			l, lower, number(top+height-ry), l, bottom, cx, bottom, right, bottom, right, number(top+height-ry),
			lower, right, upper, cx, upper, l, upper, l, lower)),
		path(Detail, fmt.Sprintf("M%s %sQ%s %s %s %sQ%s %s %s %s", l, lower, l, rim, cx, rim, right, rim, right, lower)),
	}
}

// axes are the two arrows of coordinates from a red origin at ox, oy - up to
// top, right to right: a dimension, alone or under the table of a cube's
// dimension. They are dark lines that lighten on a dark theme, heads too.
func axes(ox, oy, top, right float64) []Part {
	return []Part{
		colored(path(Detail, fmt.Sprintf("M%s %sV%sM%s %sH%s", number(ox), number(oy), number(top+2), number(ox), number(oy), number(right-2))), ink),
		colored(path(Detail, fmt.Sprintf("M%s %sL%s %sL%s %sZ", number(ox-1.5), number(top+3), number(ox), number(top), number(ox+1.5), number(top+3))), ink),
		colored(path(Detail, fmt.Sprintf("M%s %sL%s %sL%s %sZ", number(right-3), number(oy-1.5), number(right), number(oy), number(right-3), number(oy+1.5))), ink),
		colored(circle(Accent, ox, oy, 1.5), red),
	}
}

// globe is a ball with its equator and two meridians: the services, the
// references and clients of the web, a language. A sign of what the globe
// does stands in its lower right, the globe drawn up and left of it.
func globe(cx, cy, r float64) []Part {
	bend := r * 7 / 12
	return []Part{
		circle(Body, cx, cy, r),
		path(Detail, fmt.Sprintf("M%s %sH%sM%s %sQ%s %s %s %sM%s %sQ%s %s %s %s",
			number(cx-r), number(cy), number(cx+r),
			number(cx), number(cy-r), number(cx-bend), number(cy), number(cx), number(cy+r),
			number(cx), number(cy-r), number(cx+bend), number(cy), number(cx), number(cy+r))),
	}
}

// link is a reference, as the prototype draws it: an oval lying from the
// lower left to the upper right, broken at the middle of its two long sides,
// a dash through its middle - a reference alone and the sign of a WS
// reference. Dark lines that lighten on a dark theme.
func link(cx, cy, rx, ry, gap float64) []Part {
	const turn = -math.Pi / 4
	at := func(x, y float64) string {
		return number(round(cx+x*math.Cos(turn)-y*math.Sin(turn))) + " " + number(round(cy+x*math.Sin(turn)+y*math.Cos(turn)))
	}
	half := func(side float64) string {
		edge, corner := side*rx, side*(rx-ry)
		return "M" + at(side*gap, -ry) + "L" + at(corner, -ry) + "Q" + at(edge, -ry) + " " + at(edge, 0) +
			"Q" + at(edge, ry) + " " + at(corner, ry) + "L" + at(side*gap, ry)
	}
	return []Part{
		colored(path(Detail, half(-1)+half(1)), ink),
		colored(path(Detail, "M"+at(-rx/2, 0)+"L"+at(rx/2, 0)), ink),
	}
}

// formWindow is a form: a window with its title and fields, standing for
// the form in the tree, in the form editor and on its tabs.
func formWindow() []Part {
	return []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M3.5 8H6.5M3.5 10.5H6.5M8.5 8H12.5M8.5 10.5H12.5"),
	}
}

// landscape is a picture: a frame with hills and the sun - a common picture
// and, struck through, a decoration of a form.
func landscape() []Part {
	return []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Accent, "M2 13L6 8L9 11L11 9L14 12V13Z"),
		colored(circle(Accent, 11, 6, 1.5), "#efc23a"),
	}
}

func colored(part Part, color string) Part {
	part.Color = color
	return part
}

// The colours of the families of kinds, as the prototype paints them.
const (
	slate      = "#7b90aa" // pages and windows: documents, reports, modules, forms
	steel      = "#5f86b3" // frames of registers and catalogs
	blue       = "#3d7bc4" // values: constants, attributes, parameters
	green      = "#4f9a3a" // journals, information registers, calculation
	yellow     = "#e8bf3f" // the header of a catalog
	groupColor = "#c99a1e" // the folder of a group
	gold       = "#e3a826" // roles, commands, the configuration itself
	amber      = "#d99a2b" // groups of commands, subsystems
	coin       = "#f0b429" // accumulation
	red        = "#d64b3f"
	orange     = "#e39b2d"
	brown      = "#a0704a"
	gray       = "#8b939c"
	sea        = "#2f8fb0" // the globe of services and languages
	code       = "#c8463d" // the brackets of code
)

// The shapes several icons share.
const (
	page     = "M2.5 1.5H9.5L13.5 5.5V14.5H2.5Z"
	pageFold = "M9.5 1.5V5.5H13.5"
	window   = "M1.5 5.5H14.5"
	folder   = "M1.5 2.5H5.5L7 4H13.5V11.5H1.5Z"
)

// treeIcons draw the configuration tree: the kinds, what an object holds
// and the schemas inside it.
var treeIcons = []Icon{
	{ID: "configuration", Color: gold, Accent: "#fff3c4", Parts: []Part{
		circle(Body, 8, 8, 6.5),
		circle(Accent, 6, 6, 2),
	}},
	{ID: "common", Color: slate, Parts: []Part{
		colored(circle(Accent, 6, 4.5, 2.75), blue),
		colored(circle(Accent, 4.5, 11.25, 2.75), green),
		colored(circle(Accent, 11.5, 10.5, 2.75), brown),
	}},
	{ID: "subsystem", Color: amber, Parts: []Part{
		rect(Body, 5.5, 1.5, 5, 4),
		path(Detail, "M8 5.5V8M3.5 10.5V8H12.5V10.5"),
		rect(Body, 1.5, 10.5, 4, 4),
		rect(Body, 10.5, 10.5, 4, 4),
	}},
	{ID: "common-module", Color: slate, Accent: code, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window),
		path(Line, "M6.5 7.5L4.5 9.5L6.5 11.5M9.5 7.5L11.5 9.5L9.5 11.5"),
	}},
	{ID: "module", Color: slate, Accent: code, Parts: []Part{
		path(Body, page),
		path(Detail, pageFold),
		path(Line, "M6.5 8L4.5 10L6.5 12M9.5 8L11.5 10L9.5 12"),
	}},
	{ID: "session-parameter", Color: blue, Parts: []Part{
		path(Body, "M8 1.5L14.5 8L8 14.5L1.5 8Z"),
		path(Detail, "M5.5 8H10.5"),
	}},
	{ID: "role", Color: gray, Accent: gold, Parts: []Part{
		path(Body, "M7 8.5H9V14.5H7Z"),
		path(Detail, "M9 11.5H11M9 13.5H10.5"),
		circle(Accent, 8, 5, 3.5),
		circle(OnAccent, 8, 4.2, 1),
	}},
	{ID: "common-attribute", Color: blue, Accent: blue, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "attribute", Color: slate, Accent: slate, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "dimension", Color: ink, Accent: red, Parts: axes(3.5, 12.5, 1.5, 14.5)},
	{ID: "resource", Color: green, Accent: green, Parts: []Part{
		bar(6, 1.5, 4, 13),
	}},
	{ID: "tabular-section", Color: steel, Accent: steel, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M1.5 8.5H14.5M1.5 11H14.5M5.5 5.5V13.5"),
	}},
	// What the platform gives every object of a kind is green and solid, as
	// the prototype paints standard attributes and tabular sections; one the
	// object does not use is a grey dot.
	{ID: "standard-attribute", Color: green, Accent: green, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "standard-attribute-unused", Color: gray, Accent: gray, Parts: []Part{
		circle(Accent, 8, 8, 1.5),
	}},
	{ID: "standard-tabular-section", Color: green, Accent: green, Parts: barStack()},
	{ID: "enumeration-value", Color: "#6f8399", Accent: blue, Parts: []Part{
		colored(bar(5, 6, 10, 4), "#6f8399"),
		circle(Accent, 2.75, 8, 1.25),
	}},
	{ID: "journal-column", Color: steel, Accent: green, Parts: []Part{
		rect(Body, 4.5, 1.5, 7, 13),
		rect(Accent, 5, 2, 6, 2.5),
		path(Detail, "M4.5 4.5H11.5M6.5 7.5H9.5M6.5 10H9.5M6.5 12.5H9.5"),
	}},
	{ID: "addressing-attribute", Color: slate, Accent: blue, Parts: []Part{
		colored(bar(1, 9, 9, 4), slate),
		circle(Accent, 12, 4, 2),
		path(Accent, "M9 11Q9 7 12 7Q15 7 15 11Z"),
	}},
	{ID: "accounting-flag", Color: slate, Accent: green, Parts: []Part{
		colored(bar(1, 2, 10, 4), slate),
		path(Mark, "M6.5 10L9.5 13L14.5 7.5"),
	}},
	{ID: "ext-dimension-accounting-flag", Color: green, Accent: green, Parts: []Part{
		path(Detail, "M1.5 2.5H8.5M1.5 5.5H8.5M1.5 8.5H5.5"),
		path(Mark, "M6.5 10L9.5 13L14.5 7.5"),
	}},
	{ID: "url-template", Color: "#5d6b7a", Accent: green, Parts: []Part{
		path(Detail, "M1.5 4V12M1.5 8.5Q1.5 7 3 7Q4.5 7 4.5 8.5V12"),
		path(Detail, "M6 5V11Q6 12 7 12M5 7.5H7M8.5 5V11Q8.5 12 9.5 12M7.5 7.5H9.5"),
		path(Detail, "M11 7.5V14.5M11 8.5Q11 7.5 12.5 7.5Q14.5 7.5 14.5 9.75Q14.5 12 12.5 12Q11 12 11 11"),
		circle(Accent, 13, 3.5, 2),
	}},
	{ID: "http-method", Color: green, Accent: green, Parts: []Part{
		circle(Accent, 8, 8, 3),
	}},
	{ID: "web-service-operation", Color: brown, Accent: brown, Parts: []Part{
		circle(Accent, 7, 9, 5),
		colored(circle(Accent, 11.75, 4.25, 2.25), green),
	}},
	// One parameter, one sign: of a web service operation, of a data
	// composition schema.
	{ID: "parameter", Color: blue, Accent: green, Parts: []Part{
		path(Detail, "M5.5 2.5Q2.5 8 5.5 13.5M10.5 2.5Q13.5 8 10.5 13.5"),
		circle(Accent, 8, 8, 2),
	}},
	{ID: "document-movement", Color: slate, Accent: "#8fb8e0", Parts: []Part{
		path(Body, "M1.5 5.5H6L8.5 8V14.5H1.5Z"),
		rect(Accent, 8.5, 8.5, 6, 6),
		path(Detail, "M10 10.5H13M10 12.5H13"),
		colored(path(Line, "M3.5 3.5Q4 1.5 8 1.5Q11.5 1.5 11.5 5.5M10 4L11.5 5.5L13 4"), green),
	}},
	// A predefined element or group is drawn as an ordinary one with a small
	// yellow dot below its right end, as the prototype marks them.
	{ID: "predefined-item", Color: slate, Accent: yellow, Parts: []Part{
		colored(bar(2, 6, 12, 4), slate),
		circle(Accent, 13.5, 12.5, 2),
	}},
	{ID: "catalog-group", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, folder),
		rect(Accent, 2, 6, 11, 1.5),
	}},
	{ID: "predefined-group", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, folder),
		rect(Accent, 2, 6, 11, 1.5),
		circle(Accent, 13.5, 13.5, 2),
	}},
	// The points of a route map, painted as the prototype's map paints them.
	{ID: "route-start", Color: green, Parts: []Part{
		path(Body, "M3.5 2.5H12.5V9.5L8 13.5L3.5 9.5Z"),
	}},
	{ID: "route-end", Color: blue, Parts: []Part{
		path(Body, "M8 2.5L12.5 6.5V13.5H3.5V6.5Z"),
		path(Detail, "M6 10.5H10"),
	}},
	{ID: "route-action", Color: "#c9a227", Accent: blue, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, "M1.5 6.5H14.5M5.5 9.5H11.5"),
		circle(Accent, 4, 4.5, 1),
	}},
	{ID: "route-condition", Color: red, Parts: []Part{
		path(Body, "M1.5 8L4.5 3.5H11.5L14.5 8L11.5 12.5H4.5Z"),
	}},
	{ID: "route-switch", Color: red, Parts: []Part{
		path(Body, "M2.5 4.5L4.5 1.5H11.5L13.5 4.5L11.5 7.5H4.5Z"),
		path(Detail, "M8 7.5V14M8 10.5H3.5V14M8 10.5H12.5V14"),
	}},
	{ID: "route-split", Color: orange, Parts: []Part{
		path(Body, "M1.5 4.5H14.5L8 12.5Z"),
	}},
	{ID: "route-merge", Color: green, Parts: []Part{
		path(Body, "M1.5 12.5H14.5L8 4.5Z"),
	}},
	{ID: "route-processing", Color: red, Accent: code, Parts: []Part{
		rect(Body, 1.5, 3.5, 13, 9),
		path(Line, "M6.5 6L4.5 8L6.5 10M9.5 6L11.5 8L9.5 10"),
	}},
	{ID: "route-subprocess", Color: red, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Body, 5.5, 8.5, 5, 4),
		path(Detail, "M8 9.5V11.5M7 10.5H9"),
	}},
	// The data composition schema and what it holds.
	{ID: "dcs-schema", Color: slate, Parts: []Part{
		path(Body, page),
		path(Detail, pageFold+"M5.5 9V12.5H8.5M5.5 10.5H8.5"),
		colored(rect(Accent, 4, 6.5, 3, 2.5), blue),
		colored(rect(Accent, 9, 9.5, 2.5, 2), green),
		colored(rect(Accent, 9, 12, 2.5, 1.5), orange),
	}},
	{ID: "dcs-dataset-query", Color: blue, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M5.5 5.5V13.5M10.5 5.5V13.5"),
	}},
	{ID: "dcs-dataset-object", Color: blue, Accent: orange, Parts: []Part{
		rect(Body, 1.5, 2.5, 10, 11),
		path(Detail, "M1.5 5.5H11.5M5 5.5V13.5M8 5.5V13.5"),
		circle(Accent, 12.5, 11.5, 2.5),
	}},
	{ID: "dcs-dataset-union", Color: blue, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 9, 8),
		rect(Body, 5.5, 6.5, 9, 8),
		path(Detail, "M5.5 9.5H14.5M10 9.5V14.5"),
	}},
	{ID: "dcs-link", Color: blue, Accent: red, Parts: []Part{
		rect(Body, 1.5, 1.5, 6, 5),
		rect(Body, 8.5, 9.5, 6, 5),
		path(Line, "M4.5 6.5V12H8.5"),
	}},
	{ID: "dcs-calculated-field", Color: slate, Accent: blue, Parts: []Part{
		colored(bar(1, 9, 10, 4), slate),
		path(Line, "M10.5 3.5H14.5M10.5 6H14.5"),
	}},
	{ID: "dcs-settings-variant", Color: slate, Parts: []Part{
		colored(path(Body, folder), groupColor),
		colored(rect(Accent, 2, 6, 6, 1.5), yellow),
		path(Body, "M8.5 7.5H12L14.5 10V14.5H8.5Z"),
		colored(rect(Accent, 9.5, 12, 1, 1.5), red),
		colored(rect(Accent, 11, 10.5, 1, 3), green),
		colored(rect(Accent, 12.5, 11.5, 1, 2), orange),
	}},
	{ID: "exchange-plan", Color: blue, Accent: green, Parts: join(
		drum(1.5, 1, 6, 7, 1.25), drum(8.5, 8, 6, 7, 1.25),
		[]Part{path(Line, "M9.5 3.5H12.5V6.5M11 5L12.5 6.5L14 5M6.5 12.5H3.5V9.5M2 11L3.5 9.5L5 11")},
	)},
	// The funnel of a selection stands in front of the table it selects
	// from, the red of the selected rows showing at its sides.
	{ID: "filter-criterion", Color: gray, Accent: red, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M1.5 4.5H14.5"),
		path(Line, "M3 7.5H4.5M3 10H4.5M11.5 7.5H13M11.5 10H13"),
		colored(path(Plate, funnel(3.5, 4.5, 9)), slate), colored(path(Body, funnel(3.5, 4.5, 9)), slate),
	}},
	{ID: "event-subscription", Color: slate, Accent: "#efc23a", Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		path(Accent, "M9.5 2.5L5 9H8L6.5 13.5L11.5 6.5H8.5L10 2.5Z"),
	}},
	{ID: "scheduled-job", Color: blue, Accent: red, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		path(Detail, "M8 4V8L10.5 9.5"),
		circle(Accent, 8, 8, 1),
	}},
	{ID: "functional-option", Color: gray, Accent: brown, Parts: []Part{
		rect(Accent, 1.5, 2.5, 4, 2.5),
		rect(Accent, 1.5, 6.75, 4, 2.5),
		rect(Accent, 1.5, 11, 4, 2.5),
		path(Detail, "M7.5 3.75H14.5M7.5 8H14.5M7.5 12.25H14.5"),
	}},
	{ID: "functional-options-parameter", Color: gray, Accent: brown, Parts: []Part{
		path(Detail, "M3.5 2.5H2.5V7.5H3.5M12.5 2.5H13.5V7.5H12.5"),
		colored(circle(Accent, 6, 5, 0.75), blue),
		colored(circle(Accent, 8, 5, 0.75), blue),
		colored(circle(Accent, 10, 5, 0.75), blue),
		rect(Accent, 1.5, 10.5, 4, 2.5),
		path(Detail, "M7.5 11.75H14.5"),
	}},
	{ID: "defined-type", Color: blue, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 4.5, 13, 7),
		rect(Accent, 3, 6, 2.5, 4),
		rect(Accent, 6.75, 6, 2.5, 4),
		rect(Accent, 10.5, 6, 2.5, 4),
	}},
	{ID: "settings-storage", Color: blue, Accent: gray, Parts: join(
		drum(1.5, 1.5, 9, 12, 2),
		tint(gear(11, 11, 4.75), gray),
	)},
	{ID: "command", Color: gold, Accent: gold, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		path(Accent, "M6.5 5L11 8L6.5 11Z"),
	}},
	{ID: "command-group", Color: amber, Accent: gold, Parts: []Part{
		path(Body, "M1.5 3.5H6L7.5 5H14.5V13.5H1.5Z"),
		path(Accent, "M6.5 7L10.5 9.25L6.5 11.5Z"),
	}},
	{ID: "form", Color: slate, Accent: steel, Parts: formWindow()},
	{ID: "template", Color: slate, Accent: orange, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M1.5 9.5H14.5M5.5 5.5V13.5M10 5.5V13.5"),
	}},
	{ID: "picture", Color: steel, Accent: green, Parts: landscape()},
	{ID: "xdto-package", Color: brown, Parts: []Part{
		path(Body, "M2.5 5L8 2.5L13.5 5V11.5L8 14L2.5 11.5Z"),
		path(Detail, "M2.5 5L8 7.5L13.5 5M8 7.5V14M5.25 3.75L10.75 6.25"),
	}},
	{ID: "web-service", Color: sea, Accent: brown, Parts: append(globe(6.5, 6.5, 5),
		circle(Accent, 11.5, 11.5, 3),
	)},
	{ID: "http-service", Color: sea, Accent: blue, Parts: []Part{
		circle(Body, 8, 7, 5.5),
		path(Detail, "M2.5 7H13.5M8 1.5Q5 7 8 12.5M8 1.5Q11 7 8 12.5"),
		rect(Accent, 1.5, 11.5, 13, 3),
	}},
	{ID: "ws-reference", Color: sea, Parts: join(globe(6.5, 6.5, 5), link(11.5, 11.5, 4, 2, 1))},
	{ID: "websocket-client", Color: sea, Accent: green, Parts: append(globe(6.5, 6.5, 5),
		path(Line, "M8.5 13.5H14.5M12.5 11.5L14.5 13.5L12.5 15.5"),
	)},
	{ID: "style-item", Color: gray, Parts: []Part{
		colored(path(Accent, "M8 8V3A5 5 0 0 1 12.33 10.5Z"), red),
		colored(path(Accent, "M8 8L12.33 10.5A5 5 0 0 1 3.67 10.5Z"), "#efc23a"),
		colored(path(Accent, "M8 8L3.67 10.5A5 5 0 0 1 8 3Z"), blue),
	}},
	{ID: "style", Color: slate, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M9 3.5H13M11 3.5V8"),
		colored(path(Accent, "M6 10.5V7A3.5 3.5 0 0 1 9.03 12.25Z"), red),
		colored(path(Accent, "M6 10.5L9.03 12.25A3.5 3.5 0 0 1 2.97 12.25Z"), "#efc23a"),
		colored(path(Accent, "M6 10.5L2.97 12.25A3.5 3.5 0 0 1 6 7Z"), blue),
	}},
	{ID: "language", Color: sea, Accent: blue, Parts: append(globe(6, 6, 4.5),
		path(Line, "M9.5 15L11.75 9.5L14 15M10.4 13H13.1"),
	)},
	// Four small squares, the upper right one set off up and right. Drawn
	// as a path: a body's rounded corners would make squares of 5 px discs.
	{ID: "constant", Color: blue, Accent: blue, Parts: []Part{
		path(Body, "M1.5 9.5H6.5V14.5H1.5ZM7.5 9.5H12.5V14.5H7.5ZM1.5 3.5H6.5V8.5H1.5ZM9.5 1.5H14.5V6.5H9.5Z"),
	}},
	{ID: "catalog", Color: steel, Accent: yellow, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M5.5 5.5V13.5M10.5 5.5V13.5"),
	}},
	{ID: "document", Color: slate, Parts: []Part{
		path(Body, page),
		path(Detail, pageFold+"M4.5 8.5H11.5M4.5 10.5H11.5M4.5 12.5H8.5"),
	}},
	{ID: "numerator", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M2.5 13.5V3.5L8 13.5V3.5"),
		circle(Body, 11.75, 6, 2),
		path(Line, "M10 10.5H13.5"),
	}},
	{ID: "sequence", Color: gray, Parts: []Part{
		colored(path(Accent, "M6 2.5H11V1L14.5 3.5L11 6V4.5H6Z"), blue),
		colored(path(Accent, "M3.5 7H8.5V5.5L12 8L8.5 10.5V9H3.5Z"), green),
		colored(path(Accent, "M1 11.5H6V10L9.5 12.5L6 15V13.5H1Z"), red),
	}},
	{ID: "document-journal", Color: green, Accent: green, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		rect(Accent, 2.5, 1.5, 3, 13),
		path(Detail, "M7.5 5.5H11.5M7.5 8.5H11.5M7.5 11.5H10.5"),
	}},
	{ID: "enumeration", Color: "#6f8399", Accent: blue, Parts: []Part{
		path(Detail, "M5.5 2.5H4.5Q3.5 2.5 3.5 3.5V6.5Q3.5 8 2 8Q3.5 8 3.5 9.5V12.5Q3.5 13.5 4.5 13.5H5.5M10.5 2.5H11.5Q12.5 2.5 12.5 3.5V6.5Q12.5 8 14 8Q12.5 8 12.5 9.5V12.5Q12.5 13.5 11.5 13.5H10.5"),
		circle(Accent, 6, 8, 1),
		circle(Accent, 8, 8, 1),
		circle(Accent, 10, 8, 1),
	}},
	{ID: "report", Color: slate, Parts: []Part{
		path(Body, page),
		path(Detail, pageFold),
		colored(rect(Accent, 4, 9, 2, 4), red),
		colored(rect(Accent, 7, 6, 2, 7), green),
		colored(rect(Accent, 10, 8, 2, 5), orange),
	}},
	{ID: "data-processor", Color: "#3f73b0", Accent: "#efc23a", Parts: []Part{
		rect(Body, 1.5, 7.5, 8, 7),
		path(Detail, "M5.5 7.5V3.5H9M5.5 9.5V12.5M4 11L5.5 12.5L7 11"),
		path(Accent, "M12 1L15 4L12 7L9 4Z"),
	}},
	{ID: "chart-of-characteristic-types", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window+"M6.5 8.5H12.5M6.5 11H12.5"),
		colored(circle(Accent, 4, 8.5, 1), green),
		colored(circle(Accent, 4, 11, 1), orange),
	}},
	{ID: "chart-of-accounts", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M1.5 3H9M5.25 3V13.5M8 7.5H14.5M11.25 7.5V14"),
	}},
	{ID: "chart-of-calculation-types", Color: green, Accent: green, Parts: []Part{
		path(Body, "M1.5 10L8 6.5L14.5 10L8 13.5Z"),
		path(Accent, "M1.5 6L8 2.5L14.5 6L8 9.5Z"),
	}},
	{ID: "information-register", Color: green, Accent: green, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M1.5 9.5H14.5M5.5 5.5V13.5M10.5 5.5V13.5"),
	}},
	// A plain sheet with a stack of gold coins on it, as the prototype draws
	// the accumulation register: two coins, so that with the face of the
	// upper one the stack shows three bands, as the prototype's does.
	{ID: "accumulation-register", Color: steel, Accent: coin, Parts: join(
		[]Part{rect(Body, 1.5, 1.5, 13, 13)},
		coinStack(8, 6.5, 2),
	)},
	{ID: "accounting-register", Color: steel, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Line, "M3.5 4.5H8.5M6 4.5V11.5M8 7.5H12.5M10.25 7.5V12"),
	}},
	{ID: "calculation-register", Color: steel, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Accent, "M3.5 8L8 5.5L12.5 8L8 10.5Z"),
	}},
	{ID: "recalculation", Color: steel, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Line, "M11 6A3.5 3.5 0 1 0 11.5 9.5M11 3.5V6H8.5"),
	}},
	{ID: "business-process", Color: blue, Accent: red, Parts: []Part{
		rect(Body, 5.5, 1.5, 5, 4),
		path(Detail, "M8 5.5V8H4.5V10.5M8 8H11.5V9"),
		colored(rect(Accent, 2, 10.5, 5, 3.5), blue),
		path(Accent, "M11.5 9L14.5 11.75L11.5 14.5L8.5 11.75Z"),
	}},
	{ID: "task", Color: slate, Accent: red, Parts: []Part{
		path(Body, page),
		path(Detail, pageFold+"M7 8.5H11.5M7 11.5H11.5"),
		rect(Accent, 4, 7.5, 2, 2),
		colored(rect(Accent, 4, 10.5, 2, 2), blue),
	}},
	{ID: "external-data-source", Color: blue, Accent: orange, Parts: join(drum(3.5, 2, 9, 12, 2), []Part{
		circle(Accent, 11, 10.5, 1.5),
		circle(Accent, 13.5, 13, 1.5),
		circle(Accent, 10, 13.75, 1.5),
	})},
	{ID: "external-table", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, "M1.5 5.5H14.5M5.5 2.5V13.5M10.5 2.5V13.5"),
	}},
	{ID: "external-cube", Color: "#6f8399", Parts: []Part{
		path(Body, "M8 1.5L14 4.5V11.5L8 14.5L2 11.5V4.5Z"),
		path(Detail, "M2 4.5L8 7.5L14 4.5M8 7.5V14.5"),
	}},
	{ID: "external-dimension-table", Color: slate, Accent: red, Parts: join([]Part{
		rect(Body, 5.5, 1.5, 9, 9), path(Detail, "M5.5 4H14.5M10 1.5V10.5"),
	}, axes(2.5, 13.5, 1.5, 15))},
	{ID: "external-function", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M2.5 2.5V13.5"),
		path(Detail, "M2.5 3.5H12.5M4.5 6.5H13.5M4.5 9.5H11.5M4.5 12.5H9.5"),
	}},
}

// table is every icon: the tree's, the commands' (commands.go), the
// editors' (editors.go), the library's (standard_icons.go, service_icons.go).
var table = slices.Concat(treeIcons, commandIcons, editorIcons, debuggerIcons, formEditorIcons, standardIcons, serviceIcons)

// All is every icon, in the order of the table.
func All() []Icon {
	return slices.Clone(table)
}

// Has reports whether an icon of that name is drawn.
func Has(id string) bool {
	return slices.ContainsFunc(table, func(icon Icon) bool { return icon.ID == id })
}
