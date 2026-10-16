package icons

import (
	"fmt"
	"math"
)

// The pictures of the library that serve the user rather than the data:
// moving about and back in history, help, the messages and discussions of
// the system of interaction, its users, the dialogs of a question, a
// warning, information and a stop, the password, the references and the
// exchange of a distributed base - drawn from the window «Выбор картинки» as
// the inventory photographed it (docs/materials/block-00/standartnye-kartinki/).
// The prototype draws the system of interaction in thin dark lines and the
// dialogs in thin pale ones; they keep that, in the colour of their outline.

// pale is the light blue of the dialogs of a question and of information.
const pale = "#6a98d8"

// personContour is a user drawn by his outline - a head over shoulders, the
// shoulders from top to bottom half wide - in the role given: the lines of
// the system of interaction, a plate hiding what lies behind, a solid spot.
func personContour(role Role, cx, cy, r, bottom, half float64) []Part {
	top := cy + r + 1.5
	return []Part{
		circle(role, cx, cy, r),
		path(role, fmt.Sprintf("M%s %sQ%s %s %s %sQ%s %s %s %sZ",
			number(cx-half), number(bottom), number(cx-half), number(top), number(cx), number(top),
			number(cx+half), number(top), number(cx+half), number(bottom))),
	}
}

// solidPerson is a small user in black ink, hiding what lies under him by
// his own shape: the external user of the system of interaction inside his
// ring, the user of an integration over its globe.
func solidPerson(cx, cy, r, bottom, half float64) []Part {
	return join(tint(personContour(Plate, cx, cy, r, bottom, half), black), tint(personContour(InkSpot, cx, cy, r, bottom, half), black))
}

// bubble is the cloud of a message: a box with rounded corners from left,
// top to right, bottom and its tail going down from tail; with no tail, the
// back cloud of the discussions.
func bubble(left, top, right, bottom, tail float64) string {
	l, t, r, b := number(left), number(top), number(right), number(bottom)
	d := fmt.Sprintf("M%s %sH%sQ%s %s %s %sV%sQ%s %s %s %s", number(left+2), t, number(right-2), r, t, r, number(top+2),
		number(bottom-2), r, b, number(right-2), b)
	if tail > 0 {
		d += fmt.Sprintf("H%sL%s %sV%s", number(tail+1), number(tail-1), number(bottom+2.5), b)
	}
	return d + fmt.Sprintf("H%sQ%s %s %s %sV%sQ%s %s %s %sZ", number(left+2), l, b, l, number(bottom-2), number(top+2), l, t, number(left+2), t)
}

// bell is the bell of notifications standing at cx: alone it notifies,
// struck through it does not, a cross beside it asks not to disturb.
func bell(cx float64) Part {
	return path(Detail, fmt.Sprintf("M%s 12.5Q%s 11.5 %s 9.5V7.5Q%s 4 %s 4Q%s 4 %s 7.5V9.5Q%s 11.5 %s 12.5ZM%s 2.5V4M%s 14.5H%s",
		number(cx-4.5), number(cx-3.5), number(cx-3.5), number(cx-3.5), number(cx), number(cx+3.5), number(cx+3.5),
		number(cx+3.5), number(cx+4.5), number(cx), number(cx-1.5), number(cx+1.5)))
}

// turnArrow is an arrow round a circle, from the angle from to the angle to
// in degrees (0 to the right, 90 down), clockwise or back, its head at the
// end: the history going back, a picture turned either way. A thin line with
// a small head, as every arrow is.
func turnArrow(cx, cy, r, from, to float64, clockwise bool) []Part {
	at := func(angle float64) (float64, float64) {
		radians := angle * math.Pi / 180
		return round(cx + r*math.Cos(radians)), round(cy + r*math.Sin(radians))
	}
	x1, y1 := at(from)
	x2, y2 := at(to)
	sweep, way := 0, -1.0
	if clockwise {
		sweep, way = 1, 1
	}
	large := 0
	if math.Abs(to-from) > 180 {
		large = 1
	}
	radians := to * math.Pi / 180
	tx, ty := -way*math.Sin(radians), way*math.Cos(radians)
	const length, half = 2.75, 2
	head := fmt.Sprintf("M%s %sL%s %sL%s %sZ",
		number(round(x2+ty*half)), number(round(y2-tx*half)),
		number(round(x2+tx*length)), number(round(y2+ty*length)),
		number(round(x2-ty*half)), number(round(y2+tx*half)))
	return []Part{
		path(Detail, fmt.Sprintf("M%s %sA%s %s 0 %d %d %s %s", number(x1), number(y1), number(r), number(r), large, sweep, number(x2), number(y2))),
		path(Detail, head),
	}
}

// clockFace is the clock of the history of data in the lower right corner
// of what was changed, hiding what lies under it.
func clockFace(cx, cy, r float64) []Part {
	return []Part{
		colored(circle(Plate, cx, cy, r), steel), colored(circle(Body, cx, cy, r), steel),
		colored(path(Detail, fmt.Sprintf("M%s %sV%sH%s", number(cx), number(cy-r+1.5), number(cy), number(cx+r-1.5))), black),
	}
}

// star is a star of five points round cx, cy: the favourites, outlined, and
// adding to them, solid gold.
func star(cx, cy, outer, inner float64) string {
	return polygon(starPoints(cx, cy, outer, inner)...)
}

// starPoints are the ten corners of a star from its top point clockwise, the
// outer and the inner in turn: the top point is the first, the inner corner
// under the middle the sixth, so the half of a star is its first and last
// five.
func starPoints(cx, cy, outer, inner float64) [][2]float64 {
	points := make([][2]float64, 10)
	for index := range points {
		radius := outer
		if index%2 == 1 {
			radius = inner
		}
		angle := (float64(index)*36 - 90) * math.Pi / 180
		points[index] = [2]float64{round(cx + radius*math.Cos(angle)), round(cy + radius*math.Sin(angle))}
	}
	return points
}

// polygon is the closed path through the points.
func polygon(points ...[2]float64) string {
	return polyline(points...) + "Z"
}

// polyline is the open path through the points.
func polyline(points ...[2]float64) string {
	d := ""
	for index, point := range points {
		d += map[bool]string{true: "M", false: "L"}[index == 0] + number(point[0]) + " " + number(point[1])
	}
	return d
}

// goArrow is the black arrow of going by a reference, in the lower right
// corner of the reference, hiding what lies under it.
func goArrow() []Part {
	const arrow = "M8.5 11.25H11.5V9L15 12.25L11.5 15.5V13.25H8.5Z"
	return []Part{colored(path(Plate, arrow), black), colored(path(InkSpot, arrow), black)}
}

// windowArrow is the window of the previous or the next one with its arrow
// over the corner, wide and dark so that it is not lost on the window's
// title, hiding what lies under it (the owner's choice of 16.10.2026).
func windowArrow(arrow string) []Part {
	return append([]Part{
		rect(Body, 1.5, 2.5, 11, 12), rect(Accent, 2, 3, 10, 2.5),
	}, over(colored(path(Plate, arrow), "#1f66c2"), colored(path(Accent, arrow), "#1f66c2"))...)
}

// exchangeBase is the base of a distributed base, the drum in the upper
// left that changes are written from and read into, the green arrow between
// it and what stands in the lower right.
func exchangeBase(arrow ...Part) []Part {
	return join(drum(1.5, 1, 6.5, 7.5, 1.25), arrow)
}

// fx is the f(x) of an expression, the letter, the brackets and the x in
// them, moved by dx, dy and drawn scale times its size: a custom expression
// over the whole grid, the user fields of data composition smaller over a
// table.
func fx(dx, dy, scale float64) string {
	points := [][2]float64{
		{3.5, 13}, {3.5, 5}, {3.5, 3}, {5.5, 3}, {2, 7}, {5, 7},
		{8, 4}, {6.25, 8.5}, {8, 13}, {14, 4}, {15.75, 8.5}, {14, 13},
		{9.5, 6.5}, {12.5, 10.5}, {12.5, 6.5}, {9.5, 10.5},
	}
	at := make([]any, len(points))
	for index, point := range points {
		at[index] = number(round(dx+point[0]*scale)) + " " + number(round(dy+point[1]*scale))
	}
	return fmt.Sprintf("M%sL%sQ%s %sM%sL%sM%sQ%s %sM%sQ%s %sM%sL%sM%sL%s", at...)
}

// envelope is the message of an exchange in the lower right corner.
func envelope() []Part {
	return []Part{rect(Body, 8.5, 9.5, 6, 5), path(Detail, "M8.5 10L11.5 12.5L14.5 10")}
}

const (
	eye   = "M1.5 8Q8 1.5 14.5 8Q8 14.5 1.5 8Z"
	cloud = "M4.5 12.5Q1.5 12.5 1.5 10Q1.5 7.5 4 7.5Q4.5 4 8 4Q11 4 11.75 6.5Q14.5 6.5 14.5 9.5Q14.5 12.5 11.5 12.5Z"
	// writtenArrow goes from the drum to what is written into, readArrow
	// back from it.
	writtenArrow = "M8.5 3.5Q11.5 3.5 11.5 6.5"
	readArrow    = "M11.5 9V6.5Q11.5 3.5 9.5 3.5"
)

var serviceIcons = []Icon{
	// Пользователи и система взаимодействия. «Пользователь», «Активные
	// пользователи», «Журнал регистрации» and the user with authentication -
	// the grey lock of «Временная блокировка» - are the pictures of the
	// configurator's commands; the user lacking his properties and the event
	// log of a user keep the same person under their sign.
	{ID: "user-without-properties", Color: blue, Parts: join(person(6.5, 14.5, 6), []Part{
		colored(path(InkMark, "M11.25 9Q11.25 7.5 13 7.5Q14.75 7.5 14.75 9Q14.75 10.25 13 11V11.75"), black),
		colored(circle(InkSpot, 13, 14, 1), black),
	})},
	{ID: "event-log-by-user", Color: blue, Parts: join(person(6.5, 14.5, 6), []Part{
		rect(Plate, 7.5, 9.5, 8, 5.5), rect(Body, 7.5, 9.5, 8, 5.5),
		circle(Detail, 9.75, 12.25, 1), circle(Detail, 13.25, 12.25, 1), path(Detail, "M9.75 13.25H13.25"),
	})},
	{ID: "collaboration-user", Color: ink, Parts: personContour(Detail, 8, 5, 3, 14.5, 5.5)},
	{ID: "collaboration-external-user", Color: "#a3aab2", Parts: join([]Part{
		circle(Detail, 8, 8, 6.5),
	}, solidPerson(8, 6.25, 1.75, 11.5, 3.25))},
	{ID: "collaboration-integration-user", Color: "#a3aab2", Parts: join(globe(6.5, 6.5, 5.5), over(solidPerson(12, 9.5, 1.75, 15, 3.25)...))},
	// Обсуждения и сообщения.
	{ID: "conversations", Color: ink, Parts: join([]Part{
		path(Detail, bubble(5.5, 1.5, 14.5, 8.5, 0)),
	}, over(path(Plate, bubble(1.5, 5.5, 10.5, 12, 5.5))))},
	{ID: "group-conversation", Color: ink, Parts: join(
		personContour(Detail, 10.5, 4, 2, 11, 4),
		over(personContour(Plate, 6, 7, 2.5, 14.5, 4.5)...),
	)},
	{ID: "new-conversation", Color: ink, Accent: green, Parts: join([]Part{
		path(Detail, bubble(4.5, 1.5, 14.5, 10, 12)),
	}, over(path(Mark, "M4 9.5V14.5M1.5 12H6.5")))},
	{ID: "message", Color: ink, Parts: []Part{path(Detail, bubble(1.5, 2.5, 14.5, 11.5, 7))}},
	{ID: "send-message", Color: ink, Parts: []Part{
		path(Detail, "M1.5 3.5H14.5L7 13.5L5.5 8.5ZM5.5 8.5L14.5 3.5"),
	}},
	{ID: "message-history", Color: ink, Parts: []Part{
		path(Detail, "M3 5.5Q4 2.5 7 2.5H12.5Q14.5 2.5 14.5 4.5V9.5Q14.5 11.5 12.5 11.5H11.5L9.5 14V11.5H7Q3.5 11.5 3 8.5"),
		path(Detail, "M1 4.5L3 6.75L5 4.5Z"),
		path(Detail, "M8.5 4.5V7.5H10.5"),
	}},
	{ID: "attach", Color: ink, Parts: []Part{
		path(Detail, "M10.5 5.5L5.75 10.25Q4.75 11.25 5.75 12.25Q6.75 13.25 7.75 12.25L13 7Q14.75 5.25 13 3.5Q11.25 1.75 9.5 3.5L3.75 9.25Q1.5 11.5 3.75 13.75Q6 16 8.25 13.75L12.5 9.5"),
	}},
	{ID: "videoconference", Color: ink, Parts: []Part{
		path(Detail, "M3 4.5H9Q10.5 4.5 10.5 6V10Q10.5 11.5 9 11.5H3Q1.5 11.5 1.5 10V6Q1.5 4.5 3 4.5ZM10.5 7L14.5 4.5V11.5L10.5 9"),
	}},
	{ID: "notify", Color: ink, Parts: []Part{bell(8)}},
	{ID: "dont-notify", Color: ink, Accent: red, Parts: join([]Part{bell(8)}, over(path(Mark, "M2.5 14.5L13.5 2.5")))},
	{ID: "dont-disturb", Color: ink, Accent: red, Parts: []Part{bell(6.5), path(Mark, "M11.5 5L15 8.5M15 5L11.5 8.5")}},
	// Диалоги: thin pale signs in a ring or a triangle, as the prototype's
	// windows of a question show them; «Информация» is the black one of a
	// form.
	{ID: "dialog-question", Color: pale, Parts: []Part{
		circle(Detail, 8, 8, 6.5),
		path(Detail, "M5.75 6Q5.75 3.75 8 3.75Q10.25 3.75 10.25 5.75Q10.25 7.25 8 8.25V9.5M8 11.75V12"),
	}},
	{ID: "dialog-exclamation", Color: orange, Parts: []Part{
		path(Detail, "M8 1.5L14.5 13.5H1.5Z"),
		path(Detail, "M8 5.5V9.5M8 11.25V11.5"),
	}},
	{ID: "dialog-information", Color: pale, Parts: []Part{
		circle(Detail, 8, 8, 6.5),
		path(Detail, "M7 6.5H8V11.5M6.5 11.5H9.5M8 4.25V4.5"),
	}},
	{ID: "dialog-stop", Color: red, Parts: []Part{
		circle(Detail, 8, 8, 6.5),
		path(Detail, "M5.5 5.5L10.5 10.5M10.5 5.5L5.5 10.5"),
	}},
	{ID: "information", Color: black, Parts: []Part{
		circle(Detail, 8, 8, 6.5),
		path(InkMark, "M8 7V11.5"),
		circle(InkSpot, 8, 4.75, 1.1),
	}},
	// Навигация: the solid blue triangles of moving through a list, the
	// window with its arrow of the previous and the next one, the history.
	{ID: "go-back", Color: navy, Accent: "#2f6fb5", Parts: []Part{path(Accent, "M11 3V13L4 8Z")}},
	{ID: "go-forward", Color: navy, Accent: "#2f6fb5", Parts: []Part{path(Accent, "M5 3V13L12 8Z")}},
	{ID: "go-to-begin", Color: navy, Accent: "#2f6fb5", Parts: []Part{
		rect(Accent, 2.5, 3, 1.5, 10), path(Accent, "M13 3V13L6 8Z"),
	}},
	{ID: "go-to-end", Color: navy, Accent: "#2f6fb5", Parts: []Part{
		path(Accent, "M3 3V13L10 8Z"), rect(Accent, 12, 3, 1.5, 10),
	}},
	{ID: "previous", Color: gray, Accent: "#7fb2e5", Parts: windowArrow("M12.5 0.5L16 4.75H14V11H11V4.75H9Z")},
	{ID: "next", Color: gray, Accent: "#7fb2e5", Parts: windowArrow("M11 1.5H14V7.75H16L12.5 12L9 7.75H11Z")},
	{ID: "history", Color: ink, Parts: append(turnArrow(8.5, 8, 5.5, 135, -180, false),
		path(Detail, "M8.5 5V8.5H11"),
	)},
	{ID: "data-history", Color: "#6f9a2a", Parts: join(editingPencil(), over(clockFace(11.75, 11.75, 3.5)...))},
	// Ссылки и избранное. «Получить навигационную ссылку» is the sign of a
	// reference; going by one adds the black arrow, to the outside of the
	// base the globe stands for the reference.
	{ID: "goto-url", Color: ink, Parts: join(link(6.5, 6.5, 5.5, 2.5, 1), goArrow())},
	{ID: "goto-external-url", Color: ink, Parts: join(globe(6.5, 6.5, 5.5), over(goArrow()...))},
	{ID: "favorites", Color: ink, Parts: []Part{path(Detail, star(8, 8.75, 6.5, 2.75))}},
	{ID: "add-to-favorites", Color: signalYellow, Parts: appearanceStar(1)},
	{ID: "function-menu", Color: gray, Accent: gray, Parts: []Part{
		path(Accent, "M2.5 3.5H13.5V5.5H2.5ZM2.5 7H13.5V9H2.5ZM2.5 10.5H13.5V12.5H2.5Z"),
	}},
	// Справка: the library's black question, of help and of the help of a
	// form alike, where the configurator's help is a gold disc.
	{ID: "help-question", Color: black, Parts: []Part{
		path(InkMark, "M5.25 5.5Q5.25 2.5 8 2.5Q10.75 2.5 10.75 5Q10.75 6.75 8 8V10"),
		circle(InkSpot, 8, 13, 1.25),
	}},
	{ID: "sync-contents", Color: slate, Accent: green, Parts: join([]Part{
		rect(Body, 1.5, 1.5, 10, 13),
		path(Detail, "M4.25 5V11.25M4.25 8H5.5M4.25 11.25H5.5"),
		rect(Accent, 3, 3.5, 3.5, 1.5), rect(Accent, 5.5, 7.25, 3.5, 1.5), rect(Accent, 5.5, 10.5, 3, 1.5),
	}, over(append([]Part{colored(circle(Plate, 10.5, 10.5, 3), slate)}, lens(10.5, 10.5, 3)...)...))},
	// Пароль.
	{ID: "show-password", Color: ink, Parts: []Part{path(Detail, eye), colored(circle(InkSpot, 8, 8, 2), black)}},
	{ID: "hide-password", Color: ink, Parts: []Part{
		path(Detail, eye), colored(circle(InkSpot, 8, 8, 2), black), path(Detail, "M3 13L13 3"),
	}},
	// Обмен распределённой базы: the base, the message of an exchange and the
	// initial image of a node.
	{ID: "write-changes", Color: blue, Accent: green, Parts: exchangeBase(join(envelope(), []Part{
		path(Line, writtenArrow), headDown(11.5, 9),
	})...)},
	{ID: "read-changes", Color: blue, Accent: green, Parts: exchangeBase(join(envelope(), []Part{
		path(Line, readArrow), path(Accent, "M9.75 1.25L7.25 3.5L9.75 5.75Z"),
	})...)},
	{ID: "create-initial-image", Color: blue, Accent: green, Parts: exchangeBase(join(drum(8.5, 8, 6, 7, 1.25), []Part{
		path(Line, writtenArrow), headDown(11.5, 8),
	})...)},
	{ID: "open-from-main-server", Color: ink, Parts: []Part{path(Detail, cloud)}},
	{ID: "open-from-standalone-server", Color: ink, Accent: red, Parts: join([]Part{
		path(Detail, cloud),
	}, over(path(Mark, "M2.5 14.5L13.5 2.5")))},
	// Прочее.
	{ID: "today", Color: slate, Accent: "#d9b84a", Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13), rect(Accent, 3, 2, 10, 2.5),
		path(Detail, "M4.5 7H7.5M4.5 9.25H7.5M4.5 11.5H7.5"),
		colored(rect(Accent, 9, 6.5, 3.5, 3.5), red), colored(rect(Accent, 10, 7.5, 1.5, 1.5), "#ffffff"),
	}},
	{ID: "rotate-clockwise", Color: ink, Parts: turnArrow(8, 8.5, 5, -20, 280, true)},
	{ID: "rotate-counterclockwise", Color: ink, Parts: turnArrow(8, 8.5, 5, 200, -100, false)},
	{ID: "char", Color: black, Parts: []Part{
		path(InkMark, "M2 13.5H5.5V12Q2.5 10.5 2.5 7Q2.5 2.5 8 2.5Q13.5 2.5 13.5 7Q13.5 10.5 10.5 12V13.5H14"),
	}},
	{ID: "formatted-string", Color: steel, Parts: []Part{
		colored(path(Line, "M2 3Q2.5 2 3.5 2Q5 2 5 3.5V6.5M5 4.5H3.5Q1.75 4.5 1.75 5.5Q1.75 6.5 3 6.5Q5 6.5 5 4.5"), gray),
		colored(path(Line, "M7 3.5V11M7 8Q7 6.5 8.5 6.5Q10 6.5 10 8.75Q10 11 8.5 11Q7 11 7 9.5"), steel),
		colored(path(Line, "M14.5 10Q13.75 9 12.75 9Q11 9 11 11.75Q11 14.5 12.75 14.5Q13.75 14.5 14.5 13.5"), red),
	}},
	{ID: "custom-expression", Color: steel, Parts: []Part{path(Detail, fx(0, 0, 1))}},
}
