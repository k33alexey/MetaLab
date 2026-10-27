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
	// Halo is the clearing under a sign laid over a picture that fills the
	// grid: the sign's own shape, filled and widely outlined in the page's
	// background, so that the lines of a table under an f(x), a plus or an
	// arrow do not run into it.
	Halo
	// ThinHalo is the halo of an outline - the disc of a badge, the body of
	// a folder laid over a picture: half a point clear of the line, where a
	// halo as wide as a spot's reads as a thick white ring (the owner's
	// remark on the plus of «clone», 16.10.2026).
	ThinHalo
	// Tip is the solid head of an arrow drawn in lines of the colour of the
	// icon: filled with that colour and lightening on a dark theme as its
	// lines do, so that the head and its shaft stay one arrow.
	Tip
	// InkLine is an outline in the colour of the icon, as thin as a line,
	// that lightens on a dark theme as ink does: black on a light page and
	// nearly white on a dark one - the edge of the arrow of the line being
	// executed, which must stand out on any dot.
	InkLine
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

// onLine is the nearest middle of a pixel: where a line of 1 px is drawn so
// that at 16 px it is one sharp row of pixels and not two faint ones
// (ICONS.md, «Визуальный язык»).
func onLine(value float64) float64 {
	return math.Round(value-0.5) + 0.5
}

// thirds are the two lines splitting a table from x of width into three
// columns: on the half-pixel grid, and the outer two columns equal.
func thirds(x, width float64) (float64, float64) {
	first := onLine(x + width/3)
	return first, x + width - (first - x)
}

func rect(role Role, x, y, width, height float64) Part {

	return Part{Role: role, Rect: &[4]float64{x, y, width, height}}
}

// box is a rectangle with sharp corners: a table, whose corners rounded by
// 2 would turn a narrow one into a capsule and a small one into a jar.
func box(role Role, x, y, width, height float64) Part {
	return path(role, fmt.Sprintf("M%s %sH%sV%sH%sZ", number(x), number(y), number(x+width), number(y+height), number(x)))
}

func circle(role Role, cx, cy, r float64) Part {
	return Part{Role: role, Circle: &[3]float64{cx, cy, r}}
}

func path(role Role, d string) Part {
	return Part{Role: role, Path: d}
}

// urlTemplate is the sign of a URL template: the letters http with the
// green spot of a method over the p (the owner's choice of 15.10.2026).
func urlTemplate() []Part {
	return append(httpLetters(0), circle(Accent, 13, 3, 2))
}

// httpLetters are the letters http of a URL template, moved down by dy: in
// the template's own sign they leave the top right to the method's spot, in
// adding a template they are raised to the top, leaving the lower right
// corner to the star of the new (the owner's choice of 27.10.2026).
func httpLetters(dy float64) []Part {
	y := func(at float64) string { return number(at + dy) }
	return []Part{
		path(Detail, fmt.Sprintf("M1.5 %sV%sM1.5 %sQ1.5 %s 3 %sQ4.5 %s 4.5 %sV%s", y(4), y(12), y(8.5), y(7), y(7), y(7), y(8.5), y(12))),
		path(Detail, fmt.Sprintf("M6.5 %sV%sQ6.5 %s 7.5 %sM5.5 %sH7.5M8.5 %sV%sQ8.5 %s 9.5 %sM8.5 %sH9.5",
			y(5), y(11), y(12), y(12), y(7.5), y(5), y(11), y(12), y(12), y(7.5))),
		path(Detail, fmt.Sprintf("M11.5 %sV%sM11.5 %sQ11.5 %s 12.75 %sQ14.5 %s 14.5 %sQ14.5 %s 12.75 %sQ11.5 %s 11.5 %s",
			y(7.5), y(14.5), y(8.5), y(7.5), y(7.5), y(7.5), y(9.75), y(12), y(12), y(12), y(11))),
	}
}

// httpMethod is the sign of an HTTP method: a green spot.
func httpMethod() []Part { return []Part{circle(Accent, 8, 8, 3)} }

// webServiceOperation is the sign of an operation of a web service: a brown
// ball with a green spot over it.
func webServiceOperation() []Part {
	return []Part{circle(Accent, 7, 9, 5), colored(circle(Accent, 12, 4, 2), green)}
}

// parameter is the sign of a parameter: a spot between brackets.
func parameter() []Part {
	return []Part{path(Detail, "M5.5 2.5Q2.5 8 5.5 13.5M10.5 2.5Q13.5 8 10.5 13.5"), circle(Accent, 8, 8, 2)}
}

// newStar is the prototype's sign of adding a new part: a gold star of four
// rays over the lower right of the part's sign, cleared from what lies under
// it as the sparks of a wizard are.
func newStar() []Part { return newStarAt(12, 12, 3.5) }

// newStarAt is the star of the new centred at cx, cy, reaching r. Adding a URL
// template takes a smaller one in the very corner, below the letters raised
// to the top: the word http fills the width, and a star of the usual size
// covered its p (the owner's choice of 27.10.2026).
func newStarAt(cx, cy, r float64) []Part {
	return cleared(colored(path(Accent, sparkle(cx, cy, r)), sparkGold))
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
	return []Part{bar(2, 1, 12, 4), bar(2, 6, 12, 4), bar(2, 11, 12, 4)}
}

// coinStack is coins lying one on another, seen a little from above: the top
// of the upper one at top, each coin a solid gold disc rx wide on each side
// of cx, ry high and thickness thick, its edge drawn in the role edge - a
// rim, or a dark line where the coins of a narrow stack must not merge into
// one bar at 16 px (the owner's choice of 21.10.2026). The edges lie on half
// pixels, so that at 16 px each is one sharp row of pixels instead of two
// faint ones (the owner's choice of 15.10.2026).
func coinStack(cx, top, rx, ry, thickness float64, count int, edge Role) []Part {
	var parts []Part
	for i := count - 1; i >= 0; i-- {
		y := top + float64(i)*thickness
		left, right := number(cx-rx), number(cx+rx)
		arc := fmt.Sprintf("A%s %s 0 0 0 ", number(rx), number(ry))
		side := fmt.Sprintf("M%s %sV%s%s%s %sV%s", left, number(y), number(y+thickness), arc, right, number(y+thickness), number(y))
		parts = append(parts,
			path(Accent, side+arc+left+" "+number(y)+"Z"),
			path(edge, side+arc+left+" "+number(y)+"Z"))
		if i == 0 {
			parts = append(parts, path(edge, fmt.Sprintf("M%s %s%s%s %s", left, number(y), arc, right, number(y))))
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

// exchangePlan is two drums, the nodes of an exchange, with the green
// arrows between them: the plan itself and, with a pencil, the tables of
// its changes in a query.
func exchangePlan() []Part {
	return join(drum(1.5, 1, 6, 7, 1.25), drum(8.5, 8, 6, 7, 1.25),
		[]Part{path(Line, "M9.5 2.5H12.5V5M6.5 13.5H3.5V11"), path(Accent, head(12.5, 7.5, 0, 1, 1.5)), path(Accent, head(3.5, 8.5, 0, -1, 1.5))})
}

// cleared lays a sign over a picture that fills the grid, clearing a halo
// under it, where without one the sign merges with the lines it lies on:
// the f(x) of the user fields over the columns of a table, the «A» of a
// language over its globe, the sparks of a wizard over its list. Elsewhere
// a sign lies on its picture as it is - on the pixel grid it reads without a
// halo, and a halo round it only adds a white ring (the owner's decision of
// 16.10.2026 on the pass of 16 physical pixels).
// thinlyCleared lays a sign over a picture with the narrowest halo, a pixel
// wide: the plus of what is made anew and the pencil over a module, where a
// wider one was heavy (the owner's remarks of 17.10.2026).
func thinlyCleared(sign ...Part) []Part {
	parts := make([]Part, 0, 2*len(sign))
	for _, part := range sign {
		halo := part
		halo.Role, halo.Color = ThinHalo, ""
		parts = append(parts, halo)
	}
	return append(parts, sign...)
}

func cleared(sign ...Part) []Part {
	parts := make([]Part, 0, 2*len(sign))
	for _, part := range sign {
		halo := Halo
		if part.Role == Body || part.Role == Plate || part.Role == Detail {
			halo = ThinHalo
		}
		part.Role, part.Color = halo, ""
		parts = append(parts, part)
	}
	return append(parts, sign...)
}

// axes are the two arrows of coordinates from a red origin at ox, oy - up to
// top, right to right: a dimension, alone or under the table of a cube's
// dimension. They are dark lines that lighten on a dark theme, heads too.
func axes(ox, oy, top, right float64) []Part {
	return []Part{
		colored(path(Detail, fmt.Sprintf("M%s %sV%sM%s %sH%s", number(ox), number(oy), number(top+2), number(ox), number(oy), number(right-2))), ink),
		colored(path(Tip, head(ox, top, 0, -1, 1.5)), ink),
		colored(path(Tip, head(right, oy, 1, 0, 1.5)), ink),
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
// reference. Ink lines: dark on a light page and nearly white on a dark
// one, where lightened as an outline they stayed grey (the owner's remark of
// 21.10.2026).
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
		colored(path(InkLine, half(-1)+half(1)), ink),
		colored(path(InkLine, "M"+at(-rx/2, 0)+"L"+at(rx/2, 0)), ink),
	}
}

// formWindow is a form: a window with its title and fields, standing for
// the form in the tree, in the form editor and on its tabs.
func formWindow() []Part {
	return []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2),
		path(Detail, "M3.5 7.5H6.5M3.5 10.5H6.5M8.5 7.5H12.5M8.5 10.5H12.5"),
	}
}

// landscape is a picture: a frame with hills and the sun - a common picture
// and, struck through, a decoration of a form.
func landscape() []Part {
	return []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Accent, "M2 13L6 8L9 11L11 9L14 12V13Z"),
		colored(circle(Accent, 11.5, 5.5, 1.5), "#efc23a"),
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

// sheetAt is the page of «Module», its corner folded fold points, from
// left, top width points wide and height high, hiding what lies under it:
// the modules of the call stack, one over another.
func sheetAt(left, top, width, height, fold float64) []Part {
	right, bottom := left+width, top+height
	d := fmt.Sprintf("M%s %sH%sL%s %sV%sH%sZ", number(left), number(top), number(right-fold), number(right), number(top+fold), number(bottom), number(left))
	return []Part{path(Plate, d), path(Body, d), path(Detail, fmt.Sprintf("M%s %sV%sH%s", number(right-fold), number(top), number(top+fold), number(right)))}
}

// modulePage is a module: the page with its corner folded and the red
// brackets of code - a common module and the module of an object or of the
// configuration.
func modulePage() []Part {
	return []Part{path(Body, page), path(Detail, pageFold), path(Line, "M6.5 8L4.5 10L6.5 12M9.5 8L11.5 10L9.5 12")}
}

// crystal is a rhombus reaching r from the middle of the grid, its four
// facets lit from the upper left, and pressed into it a rhombus reaching
// dent, its upper walls dark and its lower ones light.
func crystal(r, dent float64) []Part {
	const light, mid, dark, deep = "#8cc4ec", "#4a97d6", "#2a6aa8", "#1f4f80"
	corners := func(r float64) (top, right, bottom, left [2]float64) {
		return [2]float64{8, 8 - r}, [2]float64{8 + r, 8}, [2]float64{8, 8 + r}, [2]float64{8 - r, 8}
	}
	top, right, bottom, left := corners(r)
	middle := [2]float64{8, 8}
	facet := func(a, b [2]float64, color string) Part { return colored(path(Accent, polygon(a, b, middle)), color) }
	t, rt, b, l := corners(dent)
	return []Part{
		facet(left, top, light), facet(top, right, mid), facet(left, bottom, mid), facet(bottom, right, dark),
		colored(path(Accent, polygon(t, rt, b, l)), mid),
		colored(path(Line, polyline(l, t, rt)), deep), colored(path(Line, polyline(l, b, rt)), light),
	}
}

// The shapes several icons share.
const (
	page     = "M2.5 1.5H9.5L13.5 5.5V14.5H2.5Z"
	pageFold = "M9.5 1.5V5.5H13.5"
	window   = "M1.5 5.5H14.5"
	folder   = "M1.5 2.5H5.5L7.5 4.5H13.5V11.5H1.5Z"
)

// treeIcons draw the configuration tree: the kinds, what an object holds
// and the schemas inside it.
var treeIcons = []Icon{
	// The configuration: a solid golden ball, a darker golden rim and a white
	// highlight, alike on both themes - on a pale ball the pale highlight was
	// lost on a light page (the owner's choice of 17.10.2026).
	{ID: "configuration", Color: gold, Parts: []Part{
		colored(circle(Accent, 8, 8, 7), "#f2c14e"),
		colored(circle(Line, 8, 8, 6.5), "#c48a12"),
		colored(circle(Accent, 6, 6, 2), "#ffffff"),
	}},
	{ID: "common", Color: slate, Parts: []Part{
		colored(circle(Accent, 6, 4, 3), blue),
		colored(circle(Accent, 4, 11, 3), green),
		colored(circle(Accent, 11, 10, 3), brown),
	}},
	{ID: "subsystem", Color: amber, Parts: []Part{
		rect(Body, 5.5, 1.5, 6, 4),
		path(Detail, "M8.5 5.5V8.5M4.5 10.5V8.5H12.5V10.5"),
		rect(Body, 2.5, 10.5, 4, 4),
		rect(Body, 10.5, 10.5, 4, 4),
	}},
	// A common module and a module are one picture, the page with the
	// brackets of code: the prototype draws the module alike for both too,
	// though as a window - the page is the owner's choice of 17.10.2026.
	{ID: "common-module", Color: slate, Accent: code, Parts: modulePage()},
	{ID: "module", Color: slate, Accent: code, Parts: modulePage()},
	// A session parameter as the prototype draws it: a blue crystal of four
	// facets lit from the upper left, a square pressed into its middle, its
	// upper walls in shadow and its lower ones lit (the owner's choice of
	// 17.10.2026: an outlined rhombus with a dash said nothing).
	{ID: "session-parameter", Color: blue, Parts: crystal(7, 2.5)},
	// The key of a role as the prototype draws it: a solid gold bow with its
	// gleam, a steel shank and a bit of two teeth, all on whole pixels so
	// that it reads as a key at 16 px (the owner's choice of 16.10.2026).
	{ID: "role", Color: gray, Accent: gold, Parts: []Part{
		circle(Accent, 8, 4, 3), colored(circle(Accent, 7, 3, 1), "#fbe3a0"),
		colored(path(Accent, "M7 7H9V15H7ZM9 10H11V11H9ZM9 12H11V14H9Z"), gray),
	}},
	{ID: "common-attribute", Color: blue, Accent: blue, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "attribute", Color: slate, Accent: slate, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "dimension", Color: ink, Accent: red, Parts: axes(3.5, 12.5, 1.5, 14.5)},
	{ID: "resource", Color: green, Accent: green, Parts: []Part{
		bar(6, 2, 4, 12),
	}},
	{ID: "tabular-section", Color: steel, Accent: steel, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M1.5 8.5H14.5M1.5 11.5H14.5M5.5 5.5V13.5"),
	}},
	// What the platform gives every object of a kind is green and solid, as
	// the prototype paints standard attributes and tabular sections; one the
	// object does not use is a grey dot.
	{ID: "standard-attribute", Color: green, Accent: green, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "standard-attribute-unused", Color: gray, Accent: gray, Parts: []Part{
		circle(Accent, 8, 8, 2),
	}},
	{ID: "standard-tabular-section", Color: green, Accent: green, Parts: barStack()},
	{ID: "enumeration-value", Color: "#6f8399", Accent: blue, Parts: []Part{
		colored(bar(5, 6, 10, 4), "#6f8399"),
		circle(Accent, 2, 8, 2),
	}},
	{ID: "journal-column", Color: steel, Accent: green, Parts: []Part{
		rect(Body, 4.5, 1.5, 7, 13),
		rect(Accent, 5, 2, 6, 2.5),
		path(Detail, "M4.5 4.5H11.5M6.5 6.5H9.5M6.5 9.5H9.5M6.5 12.5H9.5"),
	}},
	// An addressing attribute as the prototype draws it: the green dash of a
	// standard attribute (the owner's choice of 17.10.2026; a user beside
	// the dash, drawn here before, the prototype has not).
	{ID: "addressing-attribute", Color: green, Accent: green, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "accounting-flag", Color: slate, Accent: green, Parts: []Part{
		colored(bar(1, 2, 10, 4), slate),
		path(Mark, "M6.5 10L9.5 13L14.5 7.5"),
	}},
	{ID: "ext-dimension-accounting-flag", Color: green, Accent: green, Parts: join([]Part{
		path(Detail, "M1.5 2.5H8.5M1.5 5.5H8.5M1.5 8.5H5.5"),
	}, []Part{path(Mark, "M6.5 10L9.5 13L14.5 7.5")})},
	{ID: "url-template", Color: "#5d6b7a", Accent: green, Parts: urlTemplate()},
	{ID: "http-method", Color: green, Accent: green, Parts: httpMethod()},
	{ID: "web-service-operation", Color: brown, Accent: brown, Parts: webServiceOperation()},
	// One parameter, one sign: of a web service operation, of a data
	// composition schema.
	{ID: "parameter", Color: blue, Accent: green, Parts: parameter()},
	// Adding one of them: its sign with the gold star of the new over the
	// lower right, as the prototype's panels of the operations of a web
	// service and the URL templates of an HTTP service draw it
	// (web-servis/09, 14, http-servis/09, 13; the owner's answer В3 of
	// 27.10.2026). Adding a parameter is the prototype's green spot, the
	// same picture as adding a method, not the bracketed sign of a parameter
	// (the owner's choice of 27.10.2026).
	{ID: "url-template-add", Color: "#5d6b7a", Accent: green, Parts: join(httpLetters(-3), newStarAt(13, 13, 2.5))},
	{ID: "http-method-add", Color: green, Accent: green, Parts: join(httpMethod(), newStar())},
	{ID: "web-service-operation-add", Color: brown, Accent: brown, Parts: join(webServiceOperation(), newStar())},
	{ID: "parameter-add", Color: green, Accent: green, Parts: join(httpMethod(), newStar())},
	{ID: "document-movement", Color: slate, Accent: "#8fb8e0", Parts: []Part{
		path(Body, "M1.5 5.5H6L8.5 8V14.5H1.5Z"),
		rect(Accent, 9, 9, 6, 6),
		path(Detail, "M10.5 10.5H13.5M10.5 12.5H13.5"),
		colored(path(Line, "M3.5 3.5Q4 1.5 8 1.5Q11.5 1.5 11.5 4"), green), colored(path(Accent, head(11.5, 7.5, 0, 1, 2)), green),
	}},
	// A predefined element or group is drawn as an ordinary one with a small
	// yellow dot below its right end, as the prototype marks them.
	{ID: "predefined-item", Color: slate, Accent: yellow, Parts: []Part{
		colored(bar(2, 6, 12, 4), slate),
		circle(Accent, 13, 13, 2),
	}},
	{ID: "catalog-group", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, folder),
		rect(Accent, 2, 6, 11, 2),
	}},
	{ID: "predefined-group", Color: groupColor, Accent: yellow, Parts: []Part{
		path(Body, folder),
		rect(Accent, 2, 6, 11, 2),
		circle(Accent, 13, 13, 2),
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
		circle(Accent, 4.5, 4.5, 1.5),
	}},
	{ID: "route-condition", Color: red, Parts: []Part{
		path(Body, "M1.5 8L4.5 3.5H11.5L14.5 8L11.5 12.5H4.5Z"),
	}},
	{ID: "route-switch", Color: red, Parts: []Part{
		path(Body, "M2.5 4.5L4.5 1.5H11.5L13.5 4.5L11.5 7.5H4.5Z"),
		path(Detail, "M7.5 7.5V14M7.5 10.5H3.5V14M7.5 10.5H11.5V14"),
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
		rect(Body, 4.5, 6.5, 6, 6),
		path(Detail, "M7.5 8.5V10.5M6.5 9.5H8.5"),
	}},
	// The data composition schema and what it holds.
	{ID: "dcs-schema", Color: slate, Parts: []Part{
		path(Body, page),
		path(Detail, pageFold+"M5.5 8V11.5H8.5M5.5 9.5H8.5"),
		colored(rect(Accent, 4, 5, 3, 3), blue),
		colored(rect(Accent, 9, 8, 3, 2), green),
		colored(rect(Accent, 9, 11, 3, 2), orange),
	}},
	{ID: "dcs-dataset-query", Color: blue, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M5.5 5.5V13.5M10.5 5.5V13.5"),
	}},
	{ID: "dcs-dataset-object", Color: blue, Accent: orange, Parts: []Part{
		rect(Body, 1.5, 2.5, 10, 11),
		path(Detail, "M1.5 5.5H11.5M4.5 5.5V13.5M8.5 5.5V13.5"),
		circle(Accent, 12.5, 11.5, 2.5),
	}},
	{ID: "dcs-dataset-union", Color: blue, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 9, 8),
		rect(Body, 5.5, 6.5, 9, 8),
		path(Detail, "M5.5 9.5H14.5M9.5 9.5V14.5"),
	}},
	// The red link joins the two sets under their frames: it does not break
	// the blue of a border (the owner's remark of 16.10.2026).
	{ID: "dcs-link", Color: blue, Accent: red, Parts: []Part{
		path(Line, "M4.5 6.5V11.5H8.5"),
		rect(Body, 1.5, 1.5, 6, 5),
		rect(Body, 8.5, 9.5, 6, 5),
	}},
	{ID: "dcs-calculated-field", Color: slate, Accent: blue, Parts: []Part{
		colored(bar(1, 9, 10, 4), slate),
		path(Line, "M10.5 3.5H14.5M10.5 6.5H14.5"),
	}},
	// The folder of variants and in front of it the report, its page white
	// (the owner's remarks of 17.10.2026: a pale report was not seen, a
	// large one hid the folder).
	{ID: "dcs-settings-variant", Color: slate, Parts: join([]Part{
		colored(path(Body, folder), groupColor),
		colored(rect(Accent, 2, 6, 6, 2), yellow),
	}, sheetAt(7.5, 6.5, 8, 9, 3), []Part{
		colored(rect(Accent, 9, 11, 1, 3), red),
		colored(rect(Accent, 11, 9, 1, 5), green),
		colored(rect(Accent, 13, 10, 1, 4), orange),
	})},
	{ID: "exchange-plan", Color: blue, Accent: green, Parts: exchangePlan()},
	// The funnel of a selection stands in front of the table it selects
	// from, the red of the selected rows showing at its sides.
	{ID: "filter-criterion", Color: gray, Accent: red, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M1.5 4.5H14.5"),
		path(Line, "M3 7.5H4.5M3 10.5H4.5M11.5 7.5H13M11.5 10.5H13"),
		colored(path(Plate, funnel(3.5, 4.5, 9)), slate), colored(path(Body, funnel(3.5, 4.5, 9)), slate),
	}},
	{ID: "event-subscription", Color: slate, Accent: "#efc23a", Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		path(Accent, "M9.5 2L5 9H8L6.5 14L11.5 7H8.5L10 2Z"),
	}},
	{ID: "scheduled-job", Color: blue, Accent: red, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		path(Detail, "M7.5 2.5V7.5H12"),
		circle(Accent, 7.5, 7.5, 1.5),
	}},
	{ID: "functional-option", Color: gray, Accent: brown, Parts: []Part{
		rect(Accent, 2, 2, 4, 3),
		rect(Accent, 2, 7, 4, 3),
		rect(Accent, 2, 12, 4, 3),
		path(Detail, "M7.5 3.5H14.5M7.5 8.5H14.5M7.5 13.5H14.5"),
	}},
	{ID: "functional-options-parameter", Color: gray, Accent: brown, Parts: []Part{
		path(Detail, "M3.5 2.5H2.5V7.5H3.5M12.5 2.5H13.5V7.5H12.5"),
		colored(circle(Accent, 5, 5, 1), blue),
		colored(circle(Accent, 8, 5, 1), blue),
		colored(circle(Accent, 11, 5, 1), blue),
		rect(Accent, 2, 10, 4, 3),
		path(Detail, "M7.5 11.5H14.5"),
	}},
	{ID: "defined-type", Color: blue, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 4.5, 13, 7),
		rect(Accent, 3, 6, 2, 4),
		rect(Accent, 7, 6, 2, 4),
		rect(Accent, 11, 6, 2, 4),
	}},
	{ID: "settings-storage", Color: blue, Accent: gray, Parts: join(
		drum(1.5, 1.5, 9, 12, 2),
		gear(11, 11, 5),
	)},
	{ID: "command", Color: gold, Accent: gold, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		path(Accent, "M6 5L11 8L6 11Z"),
	}},
	{ID: "command-group", Color: amber, Accent: gold, Parts: []Part{
		path(Body, "M1.5 3.5H5.5L7.5 5.5H14.5V13.5H1.5Z"),
		path(Accent, "M6 7L10.5 9.5L6 12Z"),
	}},
	{ID: "form", Color: slate, Accent: steel, Parts: formWindow()},
	{ID: "template", Color: slate, Accent: orange, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M1.5 9.5H14.5M5.5 5.5V13.5M10.5 5.5V13.5"),
	}},
	{ID: "picture", Color: steel, Accent: green, Parts: landscape()},
	{ID: "xdto-package", Color: brown, Parts: []Part{
		path(Body, "M1.5 5L7.5 2L13.5 5V11L7.5 14L1.5 11Z"),
		path(Detail, "M1.5 5L7.5 8L13.5 5M7.5 8V14M4.5 3.5L10.5 6.5"),
	}},
	{ID: "web-service", Color: sea, Accent: brown, Parts: append(globe(6.5, 6.5, 5),
		circle(Accent, 12, 12, 3),
	)},
	// The globe of the services and under it «http» written as text in ink,
	// black on a light page and nearly white on a dark one (the owner's
	// choice of 17.10.2026 instead of a blue bar).
	{ID: "http-service", Color: sea, Accent: blue, Parts: append(globe(7.5, 5.5, 5),
		colored(path(InkLine, "M0.5 9.5V14.5M0.5 12.5Q0.5 11.5 1.5 11.5H2Q3.5 11.5 3.5 12.5V14.5"+
			"M5.5 11V13.5Q5.5 14.5 6.5 14.5H7M4.5 11.5H7M9.5 11V13.5Q9.5 14.5 10.5 14.5H11M8.5 11.5H11"+
			"M12.5 11.5V16M12.5 11.5H13.75Q15 11.5 15 13Q15 14.5 13.75 14.5H12.5"), black),
	)},
	{ID: "ws-reference", Color: sea, Parts: join(globe(6.5, 6.5, 5), link(11.5, 11.5, 4, 2, 1))},
	// The globe in the middle and under it the wide arrow both ways:
	// a socket talks both ways (the owner's choice of 17.10.2026).
	{ID: "websocket-client", Color: sea, Accent: green, Parts: append(globe(7.5, 5.5, 5),
		colored(path(Accent, wideArrows(14, 13, 1, 0, 12)), brightGreen),
	)},
	{ID: "style-item", Color: gray, Parts: []Part{
		colored(path(Accent, "M8 8V3A5 5 0 0 1 12.33 10.5Z"), red),
		colored(path(Accent, "M8 8L12.33 10.5A5 5 0 0 1 3.67 10.5Z"), "#efc23a"),
		colored(path(Accent, "M8 8L3.67 10.5A5 5 0 0 1 8 3Z"), blue),
	}},
	// The signs of the rows of the window of a style, by the kind of the
	// item (docs/block-00/stil.md, «Окно»; snapshots stil/06,
	// series-2.584/055): a colour - three discs, green above, orange and blue
	// below, laid over one another; a font - a slanted T with serifs; a
	// border - a frame of rounded corners.
	{ID: "style-item-color", Color: gray, Parts: []Part{
		colored(circle(Accent, 8, 5.5, 4), brightGreen),
		colored(circle(Accent, 5, 10.5, 4), orange),
		colored(circle(Accent, 11, 10.5, 4), blue),
	}},
	{ID: "style-item-font", Color: blue, Parts: []Part{
		path(Accent, "M5 2H15L14.4 5H13.4L13.2 4H10.5L8.5 12H10L9.6 14H4L4.4 12H6L8 4H5.6L5 5H4.4Z"),
	}},
	{ID: "style-item-border", Color: gray, Parts: []Part{
		rect(Body, 2.5, 2.5, 11, 11),
	}},
	{ID: "style", Color: slate, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Detail, "M9.5 3.5H13.5M11.5 3.5V8"),
		colored(path(Accent, "M6 10.5V7A3.5 3.5 0 0 1 9.03 12.25Z"), red),
		colored(path(Accent, "M6 10.5L9.03 12.25A3.5 3.5 0 0 1 2.97 12.25Z"), "#efc23a"),
		colored(path(Accent, "M6 10.5L2.97 12.25A3.5 3.5 0 0 1 6 7Z"), blue),
	}},
	// A language is a large globe with a thin «A» laid over its lower right,
	// cleared from the meridians under it, which it merged with (the owner's
	// choice of 16.10.2026).
	{ID: "language", Color: sea, Accent: blue, Parts: join(globe(7.5, 7.5, 6), cleared(path(Line, "M10 15L12.5 9L15 15M11 12.5H14")))},
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
		path(Line, "M2.5 13.5V3.5L8.5 13.5V3.5"),
		circle(Body, 12.5, 6.5, 2),
		path(Line, "M10.5 10.5H14.5"),
	}},
	{ID: "sequence", Color: gray, Parts: []Part{
		colored(path(Accent, "M6 3H11V1.5L14 4L11 6.5V5H6Z"), blue),
		colored(path(Accent, "M3 7H8V5.5L11 8L8 10.5V9H3Z"), green),
		colored(path(Accent, "M1 11H6V9.5L9 12L6 14.5V13H1Z"), red),
	}},
	{ID: "document-journal", Color: green, Accent: green, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13),
		rect(Accent, 2.5, 1.5, 3.5, 13),
		path(Detail, "M7.5 5.5H11.5M7.5 8.5H11.5M7.5 11.5H10.5"),
	}},
	{ID: "enumeration", Color: "#6f8399", Accent: blue, Parts: []Part{
		path(Detail, "M5.5 2.5H4.5Q3.5 2.5 3.5 3.5V6.5Q3.5 8 2 8Q3.5 8 3.5 9.5V12.5Q3.5 13.5 4.5 13.5H5.5M10.5 2.5H11.5Q12.5 2.5 12.5 3.5V6.5Q12.5 8 14 8Q12.5 8 12.5 9.5V12.5Q12.5 13.5 11.5 13.5H10.5"),
		circle(Accent, 8, 5, 1),
		circle(Accent, 8, 8, 1),
		circle(Accent, 8, 11, 1),
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
		path(Detail, "M5.5 7.5V3.5H9M5.5 9V11"), path(Tip, head(5.5, 13.5, 0, 1, 1.5)),
		path(Accent, "M12 1L15 4L12 7L9 4Z"),
	}},
	{ID: "chart-of-characteristic-types", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, window+"M6.5 8.5H12.5M6.5 11.5H12.5"),
		colored(circle(Accent, 4.5, 8.5, 1.5), green),
		colored(circle(Accent, 4.5, 11.5, 1.5), orange),
	}},
	{ID: "chart-of-accounts", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M1.5 3.5H9.5M5.5 3.5V13.5M8.5 7.5H14.5M11.5 7.5V14"),
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
	// the accumulation register: one stack in the middle of the sheet, two
	// thick coins with dark edges, which at 16 px read as coins and not as a
	// yellow bar (the owner's choice of 21.10.2026).
	{ID: "accumulation-register", Color: steel, Accent: coin, Parts: join(
		[]Part{rect(Body, 1.5, 1.5, 13, 13)},
		coinStack(8, 5, 3.5, 1.5, 3, 2, OnAccent),
	)},
	{ID: "accounting-register", Color: steel, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Line, "M3.5 4.5H7.5M5.5 4.5V11.5M8.5 7.5H12.5M10.5 7.5V12"),
	}},
	{ID: "calculation-register", Color: steel, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Accent, "M3.5 8L8 5.5L12.5 8L8 10.5Z"),
	}},
	{ID: "recalculation", Color: steel, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Line, "M11.5 6.5A3.5 3.5 0 1 0 11.5 9.5"), path(Accent, head(12, 7, 1, 1, 2)),
	}},
	{ID: "business-process", Color: blue, Accent: red, Parts: []Part{
		rect(Body, 5.5, 1.5, 6, 4),
		path(Detail, "M8.5 5.5V8.5H4.5V10M8.5 8.5H11.5V9"),
		colored(rect(Accent, 2, 10, 5, 4), blue),
		path(Accent, "M11.5 9L14.5 12L11.5 15L8.5 12Z"),
	}},
	{ID: "task", Color: slate, Parts: join([]Part{path(Body, page), path(Detail, pageFold)}, markedRows(red, blue))},

	{ID: "external-data-source", Color: blue, Accent: orange, Parts: join(drum(3.5, 2, 9, 12, 2), []Part{
		circle(Accent, 11.5, 10.5, 1.5),
		circle(Accent, 13.5, 13.5, 1.5),
		circle(Accent, 10.5, 13.5, 1.5),
	})},
	{ID: "external-table", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, "M1.5 5.5H14.5M5.5 2.5V13.5M10.5 2.5V13.5"),
	}},
	{ID: "external-cube", Color: "#6f8399", Parts: []Part{
		path(Body, "M7.5 1.5L13.5 4.5V11.5L7.5 14.5L1.5 11.5V4.5Z"),
		path(Detail, "M1.5 4.5L7.5 7.5L13.5 4.5M7.5 7.5V14.5"),
	}},
	{ID: "external-dimension-table", Color: slate, Accent: red, Parts: join([]Part{
		rect(Body, 5.5, 1.5, 9, 9), path(Detail, "M5.5 4.5H14.5M9.5 1.5V10.5"),
	}, axes(2.5, 13.5, 1.5, 14.5))},

	{ID: "external-function", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M2.5 2.5V13.5"),
		path(Detail, "M2.5 3.5H12.5M4.5 6.5H13.5M4.5 9.5H11.5M4.5 12.5H9.5"),
	}},
}

// table is every icon: the tree's, the commands' (commands.go), the
// editors' (editors.go), the library's (standard_icons.go, service_icons.go,
// composition_icons.go, spreadsheet_icons.go, appearance_icons.go).
var table = slices.Concat(treeIcons, commandIcons, editorIcons, debuggerIcons, formEditorIcons, standardIcons, serviceIcons, compositionIcons, spreadsheetIcons, appearanceIcons)

// All is every icon, in the order of the table.
func All() []Icon {
	return slices.Clone(table)
}

// Has reports whether an icon of that name is drawn.
func Has(id string) bool {
	return slices.ContainsFunc(table, func(icon Icon) bool { return icon.ID == id })
}
