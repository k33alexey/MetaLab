package icons

import (
	"fmt"
	"math"
)

// The debugger and Git: the menu «Отладка», the margin of a module, the
// list of breakpoints, the call stack and the watch window as the inventory
// photographed them (docs/block-00/otladka.md), and Git, which the prototype
// has not, drawn by its meaning.

// navy is the dark of the debugger's controls - play, stop, pause, the
// arrows of the steps.
const navy = "#3f6491"

// The module text the debugger acts on: a page with lines, standing to the
// left so that a control fits at its lower right, or to the right so that a
// breakpoint's sign fits at its upper left.
const (
	textPage      = "M1.5 1.5H7.5L11.5 5.5V14.5H1.5Z"
	textPageFold  = "M7.5 1.5V5.5H11.5"
	textPageLines = "M3.5 8.5H6.5M3.5 10.5H6.5M3.5 12.5H6.5"
	markPage      = "M4.5 3.5H10.5L14.5 7.5V14.5H4.5Z"
	markPageFold  = "M10.5 3.5V7.5H14.5"
	markPageLines = "M7.5 9.5H11.5M7.5 12.5H11.5"
)

func moduleText(page, fold, lines string) []Part {
	return []Part{path(Body, page), path(Detail, fold), path(Detail, lines)}
}

// braces are the { } of a procedure the steps go into, over and out of.
func braces(left, right, top, bottom float64) []Part {
	mid := (top + bottom) / 2
	brace := func(tip, side float64) string {
		in, end := tip+side, tip+2.5*side
		return fmt.Sprintf("M%s %sQ%s %s %s %sV%sQ%s %s %s %sQ%s %s %s %sV%sQ%s %s %s %s",
			number(end), number(top), number(in), number(top), number(in), number(top+1.5),
			number(mid-1), number(in), number(mid), number(tip), number(mid),
			number(in), number(mid), number(in), number(mid+1),
			number(bottom-1.5), number(in), number(bottom), number(end), number(bottom))
	}
	return []Part{path(Detail, brace(left, 1)), path(Detail, brace(right, -1))}
}

// executionPoint is the arrow of the line being executed - in the margin of
// a module, on a breakpoint's dot when the debugger has stopped on it, in
// the call stack and over the text of a module: solid yellow, outlined in
// ink - black on a light page, nearly white on a dark one - so that it
// stands out on the red and the blue of a dot as on a page (the owner's
// remarks of 16.10.2026: a pale arrow was lost on the dot, a brown edge
// too).
func executionPoint(x, y, size float64) []Part {
	arrow := arrowRight(x, y, size)
	return []Part{colored(path(Accent, arrow), "#ffcc33"), colored(path(InkLine, arrow), black)}
}

// inkedArrow is an arrow d of the debugger filled white, its edge in ink as
// the edge of the arrow of the current line: moving a breakpoint up or down.
func inkedArrow(d string) []Part {
	return []Part{colored(path(Accent, d), "#ffffff"), colored(path(InkLine, d), black)}
}

// marginArrow and marginDot are the arrow of the line being executed and the
// dot of a breakpoint in the margin of a module, each in its one place.
func marginArrow() []Part {
	return executionPoint(5.5, 3.5, 9)
}

func marginDot(color string) Part {
	return breakpointDot(6, 8, 4, color)
}

// arrowRight is the arrow of the line being executed, size points high,
// from x, y - a point on the half-pixel grid: its shaft and the base of its
// head are rounded to whole points from there, so that at 16 px its straight
// sides are sharp rows of pixels at any size, and its head is as deep as
// half its height, so that both sides of the point go at 45 degrees through
// the middles of pixels - steeper or flatter, every step of them was drawn
// as half a pixel and the point was soft. The shaft is a little more than a
// third of the height: wider, it made the arrow heavy (the owner's remarks
// of 16.10.2026).
func arrowRight(x, y, size float64) string {
	shaft, base := math.Round(size*3/11)+1, math.Round(size*5/11)
	d := ""
	for index, point := range [][2]float64{{0, shaft}, {base, shaft}, {base, 0}, {base + size/2, size / 2}, {base, size}, {base, size - shaft}, {0, size - shaft}} {
		d += map[bool]string{true: "M", false: "L"}[index == 0] + number(x+point[0]) + " " + number(y+point[1])
	}
	return d + "Z"
}

// breakpointDot is the dot of a breakpoint laid over a picture: red, or blue
// for a breakpoint with a condition.
func breakpointDot(cx, cy, r float64, color string) Part {
	return colored(circle(Accent, cx, cy, r), color)
}

// stackedWindows are the two windows of «check all» and «uncheck all» in
// the list of breakpoints, the front one hiding the back.
func stackedWindows() []Part {
	return []Part{
		rect(Body, 4.5, 1.5, 10, 10),
		rect(Plate, 1.5, 4.5, 10, 10), rect(Body, 1.5, 4.5, 10, 10),
	}
}

var debuggerIcons = []Icon{
	// Меню «Отладка»: исполнение.
	{ID: "debug-continue", Color: slate, Accent: navy, Parts: join(moduleText(textPage, textPageFold, textPageLines), []Part{
		path(Accent, "M9 7L15 11L9 15Z"),
	})},
	{ID: "debug-restart", Color: slate, Accent: navy, Parts: join(moduleText(textPage, textPageFold, textPageLines), []Part{
		rect(Accent, 8, 7, 2, 8), path(Accent, "M11 7L15 11L11 15Z"),
	})},
	{ID: "debug-finish", Color: slate, Accent: navy, Parts: join(moduleText(textPage, textPageFold, textPageLines), []Part{
		rect(Accent, 9, 8, 6, 6),
	})},
	{ID: "debug-pause", Color: slate, Accent: navy, Parts: join(moduleText(textPage, textPageFold, textPageLines), []Part{
		rect(Accent, 8, 7, 3, 8), rect(Accent, 12, 7, 3, 8),
	})},
	// Шаги: стрелка входит в скобки процедуры, обходит их, выходит из них.
	{ID: "step-into", Color: navy, Accent: navy, Parts: join(braces(2.5, 13.5, 6, 15), []Part{
		path(Line, "M3.5 2.5H6.5Q7.5 2.5 7.5 3.5V10"), path(Accent, head(7.5, 12.5, 0, 1, 2)),
	})},
	{ID: "step-over", Color: navy, Accent: navy, Parts: join(braces(1.5, 8.5, 6, 15), []Part{
		path(Line, "M4.5 6V4Q4.5 2.5 6 2.5H11Q12.5 2.5 12.5 4V10"), path(Accent, head(12.5, 12.5, 0, 1, 2)),
	})},
	{ID: "step-out", Color: navy, Accent: navy, Parts: join(braces(1.5, 8.5, 6, 15), []Part{
		path(Line, "M4.5 10V4Q4.5 2.5 6 2.5H11Q12.5 2.5 12.5 4V10"), path(Accent, head(12.5, 12.5, 0, 1, 2)),
	})},
	{ID: "run-to-cursor", Color: navy, Accent: navy, Parts: join(braces(7.5, 14.5, 3, 13), []Part{
		path(Line, "M1 5.5H4"), path(Accent, head(6.5, 5.5, 1, 0, 2)),
	})},
	{ID: "current-line", Color: slate, Parts: join([]Part{
		path(Body, markPage), path(Detail, markPageFold), path(Detail, markPageLines),
	}, executionPoint(1.5, 4.5, 8))},
	// Поле модуля и стек вызовов: текущая строка, точка останова.
	// In the margin the arrow and the dot each keep one place and one size,
	// so that stepping through a module nothing jumps: the arrow at the right
	// of the cell pointing at the text, the dot at its left; on a line with a
	// breakpoint the arrow lies over the dot (the owner's choice of
	// 16.10.2026 - an arrow over the middle of a dot was lost in it).
	{ID: "execution-point", Color: orange, Parts: marginArrow()},
	{ID: "breakpoint-mark", Color: red, Parts: []Part{marginDot(red)}},
	{ID: "breakpoint-mark-disabled", Color: red, Parts: []Part{circle(Body, 6, 8, 3.5)}},
	{ID: "breakpoint-mark-condition", Color: blue, Parts: []Part{marginDot(blue)}},
	// Отладчик стоит на строке с точкой останова: стрелка поверх кружка.
	{ID: "breakpoint-mark-current", Color: red, Parts: join([]Part{marginDot(red)}, marginArrow())},
	{ID: "breakpoint-mark-disabled-current", Color: red, Parts: join([]Part{circle(Body, 6, 8, 3.5)}, marginArrow())},
	{ID: "breakpoint-mark-condition-current", Color: blue, Parts: join([]Part{marginDot(blue)}, marginArrow())},
	// Меню «Отладка»: точки останова.
	{ID: "breakpoint", Color: slate, Accent: blue, Parts: join(moduleText(markPage, markPageFold, markPageLines), []Part{
		breakpointDot(4.5, 4.5, 3.5, red),
		path(Line, "M2.5 8.5V11.5H5"), path(Accent, head(8.5, 11.5, 1, 0, 2)),
	})},
	{ID: "breakpoint-condition", Color: slate, Accent: blue, Parts: join(moduleText(markPage, markPageFold, markPageLines), []Part{
		breakpointDot(4.5, 4.5, 3.5, blue),
		colored(path(Line, "M3.5 3.5Q3.5 2.5 4.5 2.5Q5.5 2.5 5.5 3.5Q5.5 4.25 4.5 4.5V5"), "#ffffff"),
		colored(circle(Accent, 4.5, 6.5, 0.5), "#ffffff"),
		path(Line, "M2.5 8.5V11.5H5"), path(Accent, head(8.5, 11.5, 1, 0, 2)),
	})},
	{ID: "breakpoint-toggle", Color: slate, Accent: gray, Parts: join(moduleText(markPage, markPageFold, markPageLines), []Part{
		path(Accent, "M6.97 8.53A3.5 3.5 0 0 0 2.03 13.47Z"),
		colored(path(Accent, "M6.97 8.53A3.5 3.5 0 0 1 2.03 13.47Z"), red),
	})},
	{ID: "breakpoints-delete", Color: slate, Accent: red, Parts: join(moduleText(markPage, markPageFold, markPageLines), []Part{
		path(Mark, "M1.5 1.5L6 6M6 1.5L1.5 6"),
	})},
	// The arrows of moving a breakpoint white with the edge in ink, as the
	// arrow of the current line: a brown edge was lost (the owner's remark
	// of 17.10.2026).
	{ID: "breakpoint-up", Color: brown, Parts: append([]Part{breakpointDot(10, 10, 5, red)},
		inkedArrow("M5 1.5L8.5 5.5H6.5V8.5H3.5V5.5H1.5Z")...)},
	{ID: "breakpoint-down", Color: brown, Parts: append([]Part{breakpointDot(10, 10, 5, red)},
		inkedArrow("M3.5 1.5H6.5V4.5H8.5L5 8.5L1.5 4.5H3.5Z")...)},
	{ID: "breakpoint-cut", Color: slate, Parts: join(scissors(), []Part{breakpointDot(13, 3, 3, red)})},
	{ID: "breakpoint-copy", Color: slate, Parts: join(twoPages(), []Part{breakpointDot(13, 3, 3, red)})},
	{ID: "breakpoint-paste", Color: gray, Accent: slate, Parts: join(clipboard(), []Part{breakpointDot(13, 3, 3, red)})},
	// Список точек останова: включить все, отключить все.
	{ID: "check-all", Color: slate, Accent: green, Parts: join(stackedWindows(), []Part{
		path(Mark, "M3.5 9.5L5.75 11.75L9.5 7.5"),
	})},
	{ID: "uncheck-all", Color: slate, Parts: stackedWindows()},
	// Меню «Отладка»: окна.
	{ID: "evaluate-expression", Color: slate, Parts: join(moduleText(markPage, markPageFold, markPageLines), lens(4, 4, 2.5))},
	{ID: "watch", Color: slate, Accent: navy, Parts: join([]Part{
		rect(Body, 3.5, 2.5, 11, 12), rect(Accent, 3.5, 2.5, 11, 2.5),
	}, lens(4, 4, 2.5))},
	// The call stack as modules one over another in a staircase: plain
	// sheets with square corners behind and in front the page of «Module», larger, with the
	// brackets of code - the procedures calling one another from module to
	// module (the owner's choice of 17.10.2026 out of some twenty drawings;
	// only the front sheet has its corner folded).
	{ID: "call-stack", Color: slate, Accent: code, Parts: join(
		[]Part{box(Body, 0.5, 0.5, 8, 9), box(Plate, 2.5, 2.5, 9, 9), box(Body, 2.5, 2.5, 9, 9)}, sheetAt(5.5, 5.5, 10, 10, 3),
		[]Part{path(Line, "M9.5 9.5L7.5 11.5L9.5 13.5M11.5 9.5L13.5 11.5L11.5 13.5")})},

	// Git: сам Git, ветка, фиксация, получение и отправка, различия,
	// конфликт. «Git» - знак Git (Jason Long, CC BY 3.0; NOTICE): ромб со
	// скруглёнными углами, ветка в нём белым, цвет - наш оранжевый (выбор
	// владельца 28.10.2026).
	{ID: "git", Color: orange, Parts: join(
		[]Part{path(Accent, "M8 0.9Q8.5 0.9 8.9 1.3L14.7 7.1Q15.1 7.5 15.1 8Q15.1 8.5 14.7 8.9L8.9 14.7Q8.5 15.1 8 15.1Q7.5 15.1 7.1 14.7L1.3 8.9Q0.9 8.5 0.9 8Q0.9 7.5 1.3 7.1L7.1 1.3Q7.5 0.9 8 0.9Z")},
		tint([]Part{path(Line, "M5.8 3.4L7.5 5.6V11.2M7.5 6.4L11.1 8.8"),
			circle(Accent, 7.5, 6.2, 1.3), circle(Accent, 7.5, 11.4, 1.3), circle(Accent, 11.3, 9, 1.3)}, "#ffffff"))},
	{ID: "git-branch", Color: orange, Parts: []Part{
		path(Detail, "M4.5 5.5V10.5M11.5 7.5Q11.5 10 4.5 10.5"),
		circle(Body, 4.5, 3.5, 2), circle(Body, 4.5, 12.5, 2), circle(Body, 11.5, 5.5, 2),
	}},
	{ID: "git-commit", Color: orange, Parts: []Part{
		path(Detail, "M0.5 7.5H4.5M10.5 7.5H14.5"), circle(Body, 7.5, 7.5, 3),
	}},
	{ID: "git-pull", Color: orange, Accent: blue, Parts: []Part{
		path(Detail, "M0.5 12.5H5.5M9.5 12.5H14.5"), circle(Body, 7.5, 12.5, 2),
		path(Line, "M7.5 1.5V6"), path(Accent, head(7.5, 9.5, 0, 1, 2)),
	}},
	{ID: "git-push", Color: orange, Accent: blue, Parts: []Part{
		path(Detail, "M0.5 12.5H5.5M9.5 12.5H14.5"), circle(Body, 7.5, 12.5, 2),
		path(Line, "M7.5 9V4"), path(Accent, head(7.5, 0.5, 0, -1, 2)),
	}},
	{ID: "git-diff", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M8.5 1.5V14.5"),
		path(Accent, plus(5, 8, 3)), colored(path(Accent, minus(11.5, 8, 2.5)), red),
	}},
	{ID: "git-conflict", Color: orange, Accent: red, Parts: []Part{
		path(Detail, "M3.5 1.5V4Q3.5 7 7.5 8.5M11.5 1.5V4Q11.5 7 7.5 8.5V14.5"),
		circle(Accent, 7.5, 9.5, 2.5),
	}},
}
