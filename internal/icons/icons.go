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

import "slices"

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
)

// Part is one shape of an icon in its role. Exactly one of the geometries is
// set: a rectangle, a circle or a path in the 16×16 grid.
type Part struct {
	Role   Role
	Rect   *[4]float64 // x, y, width, height
	Circle *[3]float64 // cx, cy, r
	Path   string
	// Color overrides the accent colour of the icon for this part: the bars
	// of a report are red, green and orange.
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

func colored(part Part, color string) Part {
	part.Color = color
	return part
}

// The colours of the families of kinds, as the prototype paints them.
const (
	slate  = "#7b90aa" // pages and windows: documents, reports, modules, forms
	steel  = "#5f86b3" // frames of registers and catalogs
	blue   = "#3d7bc4" // values: constants, attributes, parameters
	green  = "#4f9a3a" // journals, information registers, calculation
	yellow = "#e8bf3f" // the header of a catalog
	gold   = "#e3a826" // roles, commands, the configuration itself
	amber  = "#d99a2b" // groups of commands, subsystems
	coin   = "#f0b429" // accumulation
	red    = "#d64b3f"
	orange = "#e39b2d"
	brown  = "#a0704a"
	gray   = "#8b939c"
	sea    = "#2f8fb0" // the globe of services and languages
	code   = "#c8463d" // the brackets of code
)

// The shapes several icons share.
const (
	page     = "M2.5 1.5H9.5L13.5 5.5V14.5H2.5Z"
	pageFold = "M9.5 1.5V5.5H13.5"
	window   = "M1.5 5.5H14.5"
	cylinder = "M3.5 4V12Q3.5 14 8 14Q12.5 14 12.5 12V4"
	globe    = "M2 8H14M8 2Q4.5 8 8 14M8 2Q11.5 8 8 14"
)

var table = []Icon{
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
		rect(Body, 1.5, 5.5, 13, 5),
		rect(Accent, 3, 7, 3, 2),
	}},
	{ID: "attribute", Color: slate, Parts: []Part{
		rect(Body, 2.5, 6.5, 11, 3),
	}},
	{ID: "dimension", Color: blue, Accent: red, Parts: []Part{
		path(Line, "M3.5 2.5V12.5H13.5"),
		path(Detail, "M3.5 2.5L2 4M3.5 2.5L5 4M13.5 12.5L12 11M13.5 12.5L12 14"),
		rect(Body, 6.5, 5.5, 5, 4),
	}},
	{ID: "resource", Color: green, Accent: green, Parts: []Part{
		path(Body, "M5.5 14Q2.5 9 6 5Q8.5 2.5 12.5 2Q13 7 10.5 10Q8.5 12.5 5.5 14Z"),
		path(Detail, "M5.5 14L10 6.5"),
	}},
	{ID: "tabular-section", Color: steel, Accent: steel, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M1.5 8.5H14.5M1.5 11H14.5M5.5 5.5V13.5"),
	}},
	{ID: "exchange-plan", Color: blue, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 6, 6),
		rect(Body, 8.5, 8.5, 6, 6),
		path(Line, "M9.5 3.5H12.5V6.5M11 5L12.5 6.5L14 5M6.5 12.5H3.5V9.5M2 11L3.5 9.5L5 11"),
	}},
	{ID: "filter-criterion", Color: slate, Accent: red, Parts: []Part{
		path(Body, "M1.5 2.5H14.5L9.5 8.5V14L6.5 12.5V8.5Z"),
		path(Line, "M4.5 5H11.5"),
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
	{ID: "settings-storage", Color: blue, Accent: gray, Parts: []Part{
		path(Body, cylinder+"Q12.5 2 8 2Q3.5 2 3.5 4Z"),
		path(Detail, "M3.5 4Q3.5 6 8 6Q12.5 6 12.5 4"),
		circle(Accent, 11.5, 11.5, 3),
		circle(OnAccent, 11.5, 11.5, 1),
	}},
	{ID: "command", Color: gold, Accent: gold, Parts: []Part{
		circle(Body, 8, 8, 6.5),
		path(Accent, "M6.5 5L11 8L6.5 11Z"),
	}},
	{ID: "command-group", Color: amber, Accent: gold, Parts: []Part{
		path(Body, "M1.5 3.5H6L7.5 5H14.5V13.5H1.5Z"),
		path(Accent, "M6.5 7L10.5 9.25L6.5 11.5Z"),
	}},
	{ID: "form", Color: slate, Accent: steel, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M3.5 8H6.5M3.5 10.5H6.5M8.5 8H12.5M8.5 10.5H12.5"),
	}},
	{ID: "template", Color: slate, Accent: orange, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		rect(Accent, 2, 3, 12, 2.5),
		path(Detail, "M1.5 5.5H14.5M1.5 9.5H14.5M5.5 5.5V13.5M10 5.5V13.5"),
	}},
	{ID: "picture", Color: steel, Accent: green, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Accent, "M2 13L6 8L9 11L11 9L14 12V13Z"),
		colored(circle(Accent, 11, 6, 1.5), "#efc23a"),
	}},
	{ID: "xdto-package", Color: brown, Parts: []Part{
		path(Body, "M2.5 5L8 2.5L13.5 5V11.5L8 14L2.5 11.5Z"),
		path(Detail, "M2.5 5L8 7.5L13.5 5M8 7.5V14M5.25 3.75L10.75 6.25"),
	}},
	{ID: "web-service", Color: sea, Accent: brown, Parts: []Part{
		circle(Body, 8, 8, 6),
		path(Detail, globe),
		circle(Accent, 5, 11.5, 3),
	}},
	{ID: "http-service", Color: sea, Accent: blue, Parts: []Part{
		circle(Body, 8, 7, 5.5),
		path(Detail, "M2.5 7H13.5M8 1.5Q5 7 8 12.5M8 1.5Q11 7 8 12.5"),
		rect(Accent, 1.5, 11.5, 13, 3),
	}},
	{ID: "ws-reference", Color: sea, Accent: gray, Parts: []Part{
		circle(Body, 8, 8, 6),
		path(Detail, globe),
		circle(Accent, 5, 11.5, 3),
		path(OnAccent, "M4 11.5H6"),
	}},
	{ID: "websocket-client", Color: sea, Accent: green, Parts: []Part{
		circle(Body, 8, 8, 6),
		path(Detail, globe),
		path(Line, "M1.5 13.5H7.5M5.5 11.5L7.5 13.5L5.5 15"),
	}},
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
	{ID: "language", Color: sea, Accent: blue, Parts: []Part{
		circle(Body, 6, 6, 4.5),
		path(Detail, "M1.5 6H10.5M6 1.5Q3.5 6 6 10.5M6 1.5Q8.5 6 6 10.5"),
		path(Line, "M9.5 15L11.75 9.5L14 15M10.4 13H13.1"),
	}},
	{ID: "constant", Color: blue, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 7.5, 6, 6),
		rect(Body, 8.5, 7.5, 6, 6),
		rect(Accent, 9, 1.5, 5, 5),
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
		colored(rect(Accent, 1.5, 2.5, 10, 2.5), red),
		colored(rect(Accent, 4.5, 6.75, 10, 2.5), green),
		colored(rect(Accent, 1.5, 11, 13, 2.5), blue),
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
	{ID: "accumulation-register", Color: steel, Accent: coin, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		circle(Accent, 8, 8, 4),
		path(OnAccent, "M6.3 8.2L7.5 9.4L9.8 6.8"),
	}},
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
	{ID: "external-data-source", Color: blue, Accent: orange, Parts: []Part{
		path(Body, cylinder+"Q12.5 2 8 2Q3.5 2 3.5 4Z"),
		path(Detail, "M3.5 4Q3.5 6 8 6Q12.5 6 12.5 4"),
		circle(Accent, 11, 10.5, 1.5),
		circle(Accent, 13.5, 13, 1.5),
		circle(Accent, 10, 13.75, 1.5),
	}},
	{ID: "external-table", Color: slate, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Detail, "M1.5 5.5H14.5M5.5 2.5V13.5M10.5 2.5V13.5"),
	}},
	{ID: "external-cube", Color: "#6f8399", Parts: []Part{
		path(Body, "M8 1.5L14 4.5V11.5L8 14.5L2 11.5V4.5Z"),
		path(Detail, "M2 4.5L8 7.5L14 4.5M8 7.5V14.5"),
	}},
	{ID: "external-dimension-table", Color: slate, Accent: red, Parts: []Part{
		rect(Body, 5.5, 2.5, 9, 11),
		path(Detail, "M5.5 5.5H14.5M10 2.5V13.5"),
		path(Line, "M2.5 1.5V13.5H4.5"),
	}},
	{ID: "external-function", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M2.5 2.5V13.5"),
		path(Detail, "M2.5 3.5H12.5M4.5 6.5H13.5M4.5 9.5H11.5M4.5 12.5H9.5"),
	}},
}

// All is every icon, in the order of the table.
func All() []Icon {
	return slices.Clone(table)
}

// Has reports whether an icon of that name is drawn.
func Has(id string) bool {
	return slices.ContainsFunc(table, func(icon Icon) bool { return icon.ID == id })
}
