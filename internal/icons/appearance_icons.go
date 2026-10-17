package icons

import "fmt"

// The pictures of appearance, which a report or a list sets on a cell by a
// condition to show a state at a glance: circles, boxes and stars filled in
// part, arrows of a trend, flags, signs in a disc - drawn from the window
// «Выбор картинки» (docs/materials/block-00/standartnye-kartinki/). Their
// colour is what they say, so the prototype draws them solid with a darker
// rim, not thinly filled as a command; a filled part is solid, an empty part
// an outline, and the pictures of one row stand in one place, so that a
// column of them lines up.

// The colours of appearance, as the prototype paints its signals.
const (
	signalYellow = "#f2c230"
	signalGreen  = "#5bb84a"
	signalRed    = "#ee5a3a"
	signalBlue   = "#5d9de0"
	signalGray   = "#a3a7ac"
	signalBlack  = "#3d3f42"
	circleRing   = "#6f8fb3" // the ring of a circle filled in part
)

// signal is a solid shape of appearance in its colour with a darker rim -
// a disc, a box, an arrow, a star, a flag.
func signal(shape Part, color string) []Part {
	rim := shape
	rim.Role = Rim
	shape.Role = Accent
	return []Part{colored(shape, color), colored(rim, color)}
}

// filledCircle is the circle filled by quarters clockwise from its top:
// none is the empty circle, four the filled one. The ring stays over the
// part filled, so that every circle of the row has one edge.
func filledCircle(quarters int) []Part {
	const cx, cy, r = 8, 8, 6.5
	parts := []Part{}
	switch quarters {
	case 4:
		parts = append(parts, colored(circle(Accent, cx, cy, r), signalBlue))
	case 1, 2, 3:
		end := [][2]float64{{cx + r, cy}, {cx, cy + r}, {cx - r, cy}}[quarters-1]
		parts = append(parts, colored(path(Accent, fmt.Sprintf("M%s %sV%sA%s %s 0 %d 1 %s %sZ",
			number(cx), number(cy), number(cy-r), number(r), number(r), quarters/3, number(end[0]), number(end[1]))), signalBlue))
	}
	return append(parts, colored(circle(Detail, cx, cy, r), circleRing))
}

// boxes are four boxes, two over two, the first filled ones blue, filled
// from the lower left: then the lower right, the upper left, the upper right.
func boxes(filled int) []Part {
	corners := [][2]float64{{1.5, 9.5}, {9.5, 9.5}, {1.5, 1.5}, {9.5, 1.5}}
	var parts []Part
	for index, corner := range corners {
		box := bar(corner[0], corner[1], 5, 5)
		if index < filled {
			parts = append(parts, signal(box, signalBlue)...)
		} else {
			box.Role = Body
			parts = append(parts, colored(box, signalGray))
		}
	}
	return parts
}

// appearanceStar is the star of appearance, of adding to the favourites
// too: solid, filled on its left half, or outlined.
func appearanceStar(filled float64) []Part {
	points := starPoints(8, 8.75, 7, 3)
	switch filled {
	case 1:
		return signal(path(Accent, polygon(points...)), signalYellow)
	case 0.5:
		left := append(append([][2]float64{}, points[5:]...), points[0])
		return []Part{
			colored(path(Accent, polygon(left...)), signalYellow),
			colored(path(Detail, polygon(points...)), signalGray),
			colored(path(Rim, polyline(left...)), signalYellow),
		}
	default:
		return []Part{colored(path(Detail, polygon(points...)), signalGray)}
	}
}

// signDisc is a solid disc with a sign in it, in ink: white on red and on
// green, dark on yellow, where white reads poorly (the owner's choice of
// 16.10.2026).
func signDisc(color, ink string, sign ...Part) []Part {
	return append(signal(circle(Accent, 8, 8, 6.5), color), tint(sign, ink)...)
}

// pennant is a flag on its dark pole, the cloth waving to the right.
func pennant(color string) []Part {
	return append(signal(path(Accent, "M4.5 2Q7.5 1 9.75 2.75Q11.75 4.25 14.5 5Q12.25 7.25 9.25 7.25Q6.5 7.25 4.5 8.75Z"), color),
		colored(path(Detail, "M3.5 1.5V14.5"), "#5b636d"))
}

var appearanceIcons = []Icon{
	// Знаки.
	{ID: "appearance-exclamation", Color: signalYellow, Parts: join(
		signal(bar(6.5, 1.5, 3, 8), signalYellow), signal(circle(Accent, 8, 13, 1.5), signalYellow))},
	{ID: "appearance-dash", Color: signalYellow, Parts: signal(bar(2.5, 6.5, 11, 3), signalYellow)},
	{ID: "appearance-exclamation-disc", Color: signalYellow, Parts: signDisc(signalYellow, onAccentInk,
		path(Mark, "M8 4.5V8.5"), circle(Accent, 8, 11, 1))},
	{ID: "appearance-cross-disc", Color: signalRed, Parts: signDisc(signalRed, "#ffffff", path(Mark, "M5.5 5.5L10.5 10.5M10.5 5.5L5.5 10.5"))},
	{ID: "appearance-check-disc", Color: signalGreen, Parts: signDisc(signalGreen, "#ffffff", path(Mark, "M4.75 8.25L7 10.5L11.25 6"))},
	// Звёзды: заполненная — «Добавить в избранное». Крест и флажок оформления
	// — «Удалить» и «Применить»: в формах они стоят на кнопках рядом с
	// командами и в шагах помощников, а метка одной толщины во всём наборе
	// (выбор владельца 16.10.2026).
	{ID: "appearance-star-half", Color: signalYellow, Parts: appearanceStar(0.5)},
	{ID: "appearance-star-empty", Color: signalGray, Parts: appearanceStar(0)},
	// Квадраты.
	{ID: "appearance-boxes-filled", Color: signalBlue, Parts: boxes(4)},
	{ID: "appearance-boxes-three", Color: signalBlue, Parts: boxes(3)},
	{ID: "appearance-boxes-two", Color: signalBlue, Parts: boxes(2)},
	{ID: "appearance-boxes-one", Color: signalBlue, Parts: boxes(1)},
	{ID: "appearance-boxes-empty", Color: signalGray, Parts: boxes(0)},
	// Круги.
	{ID: "appearance-circle-filled", Color: signalBlue, Parts: filledCircle(4)},
	{ID: "appearance-circle-three-fourth", Color: signalBlue, Parts: filledCircle(3)},
	{ID: "appearance-circle-two-fourth", Color: signalBlue, Parts: filledCircle(2)},
	{ID: "appearance-circle-one-fourth", Color: signalBlue, Parts: filledCircle(1)},
	{ID: "appearance-circle-empty", Color: circleRing, Parts: filledCircle(0)},
	{ID: "appearance-circle-yellow", Color: signalYellow, Parts: signal(circle(Accent, 8, 8, 6.5), signalYellow)},
	{ID: "appearance-circle-green", Color: signalGreen, Parts: signal(circle(Accent, 8, 8, 6.5), signalGreen)},
	{ID: "appearance-circle-red", Color: signalRed, Parts: signal(circle(Accent, 8, 8, 6.5), signalRed)},
	// Чёрный круг обводится линией, которая на тёмной теме светлеет: кайма
	// темнее круга пропала бы на тёмной странице вместе с ним.
	{ID: "appearance-circle-black", Color: signalBlack, Parts: []Part{
		colored(circle(Accent, 8, 8, 6.5), signalBlack), colored(circle(Detail, 8, 8, 6.5), signalBlack),
	}},
	// Стрелки: одна стрелка «Переместить», повёрнутая.
	{ID: "appearance-up-green", Color: signalGreen, Parts: signal(path(Accent, blockArrow(0)), signalGreen)},
	{ID: "appearance-up-gray", Color: signalGray, Parts: signal(path(Accent, blockArrow(0)), signalGray)},
	{ID: "appearance-down-red", Color: signalRed, Parts: signal(path(Accent, blockArrow(180)), signalRed)},
	{ID: "appearance-down-gray", Color: signalGray, Parts: signal(path(Accent, blockArrow(180)), signalGray)},
	{ID: "appearance-right-yellow", Color: signalYellow, Parts: signal(path(Accent, blockArrow(90)), signalYellow)},
	{ID: "appearance-right-gray", Color: signalGray, Parts: signal(path(Accent, blockArrow(90)), signalGray)},
	{ID: "appearance-up-incline-yellow", Color: signalYellow, Parts: signal(path(Accent, blockArrow(45)), signalYellow)},
	{ID: "appearance-up-incline-green", Color: signalGreen, Parts: signal(path(Accent, blockArrow(45)), signalGreen)},
	{ID: "appearance-up-incline-gray", Color: signalGray, Parts: signal(path(Accent, blockArrow(45)), signalGray)},
	{ID: "appearance-down-incline-yellow", Color: signalYellow, Parts: signal(path(Accent, blockArrow(135)), signalYellow)},
	{ID: "appearance-down-incline-red", Color: signalRed, Parts: signal(path(Accent, blockArrow(135)), signalRed)},
	{ID: "appearance-down-incline-gray", Color: signalGray, Parts: signal(path(Accent, blockArrow(135)), signalGray)},
	{ID: "appearance-up-triangle", Color: signalGreen, Parts: signal(path(Accent, "M8 5L14.5 11.5H1.5Z"), signalGreen)},
	{ID: "appearance-down-triangle", Color: signalRed, Parts: signal(path(Accent, "M8 11L14.5 4.5H1.5Z"), signalRed)},

	// Флаги.
	{ID: "appearance-flag-yellow", Color: signalYellow, Parts: pennant(signalYellow)},
	{ID: "appearance-flag-green", Color: signalGreen, Parts: pennant(signalGreen)},
	{ID: "appearance-flag-red", Color: signalRed, Parts: pennant(signalRed)},
}
