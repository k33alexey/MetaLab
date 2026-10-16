package icons

import "fmt"

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
	textPage      = "M1.5 1.5H7.5L11 5V14.5H1.5Z"
	textPageFold  = "M7.5 1.5V5H11"
	textPageLines = "M3.5 7H7M3.5 9.5H7M3.5 12H7"
	markPage      = "M4.5 3.5H11L14.5 7V14.5H4.5Z"
	markPageFold  = "M11 3.5V7H14.5"
	markPageLines = "M7 9.5H12M7 12H12"
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

// headDown is the solid head of a step's arrow, its point at x, y.
func headDown(x, y float64) Part {
	return path(Accent, fmt.Sprintf("M%s %sH%sL%s %sZ", number(x-2), number(y-2.5), number(x+2), number(x), number(y)))
}

// executionPoint is the arrow of the line being executed - in the margin of
// a module, in the call stack and over the text of a module.
func executionPoint(x, y, size float64) Part {
	return colored(path(Body, arrowRight(x, y, size)), orange)
}

// executionOverBreakpoint is the same arrow standing on a breakpoint's dot,
// when the debugger has stopped on a line with a breakpoint: filled solid and
// light, so that the red of the dot does not show through it, and outlined
// dark, as the prototype draws it in the margin.
func executionOverBreakpoint(x, y, size float64) []Part {
	arrow := arrowRight(x, y, size)
	return []Part{colored(path(Accent, arrow), "#fcdcb0"), colored(path(Detail, arrow), brown)}
}

func arrowRight(x, y, size float64) string {
	scale := size / 11
	d := ""
	for index, point := range [][2]float64{{0, 3}, {5, 3}, {5, 0}, {11, 5.5}, {5, 11}, {5, 8}, {0, 8}} {
		d += map[bool]string{true: "M", false: "L"}[index == 0] + number(x+point[0]*scale) + " " + number(y+point[1]*scale)
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
		path(Accent, "M9 7L15 10.75L9 14.5Z"),
	})},
	{ID: "debug-restart", Color: slate, Accent: navy, Parts: join(moduleText(textPage, textPageFold, textPageLines), []Part{
		rect(Accent, 8.5, 7, 2, 7.5), path(Accent, "M11.5 7L15.5 10.75L11.5 14.5Z"),
	})},
	{ID: "debug-finish", Color: slate, Accent: navy, Parts: join(moduleText(textPage, textPageFold, textPageLines), []Part{
		rect(Accent, 8.5, 8, 6.5, 6.5),
	})},
	{ID: "debug-pause", Color: slate, Accent: navy, Parts: join(moduleText(textPage, textPageFold, textPageLines), []Part{
		rect(Accent, 8.5, 7.5, 2.5, 7), rect(Accent, 12.5, 7.5, 2.5, 7),
	})},
	// Шаги: стрелка входит в скобки процедуры, обходит их, выходит из них.
	{ID: "step-into", Color: navy, Accent: navy, Parts: join(braces(2.5, 13.5, 6, 15), []Part{
		path(Line, "M3.5 2.5H7Q8 2.5 8 3.5V10"), headDown(8, 12.5),
	})},
	{ID: "step-over", Color: navy, Accent: navy, Parts: join(braces(1.5, 8.5, 6, 15), []Part{
		path(Line, "M4 6V4Q4 2.5 5.5 2.5H11Q12.5 2.5 12.5 4V10"), headDown(12.5, 12.5),
	})},
	{ID: "step-out", Color: navy, Accent: navy, Parts: join(braces(1.5, 8.5, 6, 15), []Part{
		path(Line, "M5 10V4Q5 2.5 6.5 2.5H11Q12.5 2.5 12.5 4V10"), headDown(12.5, 12.5),
	})},
	{ID: "run-to-cursor", Color: navy, Accent: navy, Parts: join(braces(7.5, 14.5, 3, 13), []Part{
		path(Line, "M1 5.5H4"), path(Accent, "M3.5 3.5L6.5 5.5L3.5 7.5Z"),
	})},
	{ID: "current-line", Color: slate, Parts: join([]Part{
		path(Body, markPage), path(Detail, markPageFold), path(Detail, markPageLines),
	}, over(executionPoint(1, 4.5, 8)))},
	// Поле модуля и стек вызовов: текущая строка, точка останова.
	{ID: "execution-point", Color: orange, Parts: []Part{executionPoint(2, 2.5, 11)}},
	{ID: "breakpoint-mark", Color: red, Parts: []Part{breakpointDot(8, 8, 5, red)}},
	{ID: "breakpoint-mark-disabled", Color: red, Parts: []Part{circle(Body, 8, 8, 4.5)}},
	{ID: "breakpoint-mark-condition", Color: blue, Parts: []Part{breakpointDot(8, 8, 5, blue)}},
	// Отладчик стоит на строке с точкой останова: стрелка поверх кружка.
	{ID: "breakpoint-mark-current", Color: red, Parts: join([]Part{breakpointDot(8, 8, 5, red)}, executionOverBreakpoint(4.5, 4.5, 7))},
	{ID: "breakpoint-mark-disabled-current", Color: red, Parts: join([]Part{circle(Body, 8, 8, 4.5)}, executionOverBreakpoint(4.5, 4.5, 7))},
	{ID: "breakpoint-mark-condition-current", Color: blue, Parts: join([]Part{breakpointDot(8, 8, 5, blue)}, executionOverBreakpoint(4.5, 4.5, 7))},
	// Меню «Отладка»: точки останова.
	{ID: "breakpoint", Color: slate, Accent: blue, Parts: join(moduleText(markPage, markPageFold, markPageLines), over(
		breakpointDot(4.5, 4.5, 3.5, red),
		path(Line, "M2.5 8.5V11H7.5M6 9.5L7.5 11L6 12.5"),
	))},
	{ID: "breakpoint-condition", Color: slate, Accent: blue, Parts: join(moduleText(markPage, markPageFold, markPageLines), over(
		breakpointDot(4.5, 4.5, 3.5, blue),
		colored(path(Line, "M3.5 3.5Q3.5 2.5 4.5 2.5Q5.5 2.5 5.5 3.5Q5.5 4.25 4.5 4.5V5"), "#ffffff"),
		colored(circle(Accent, 4.5, 6.4, 0.5), "#ffffff"),
		path(Line, "M2.5 8.5V11H7.5M6 9.5L7.5 11L6 12.5"),
	))},
	{ID: "breakpoint-toggle", Color: slate, Accent: gray, Parts: join(moduleText(markPage, markPageFold, markPageLines), []Part{
		path(Accent, "M6.97 8.53A3.5 3.5 0 0 0 2.03 13.47Z"),
		colored(path(Accent, "M6.97 8.53A3.5 3.5 0 0 1 2.03 13.47Z"), red),
	})},
	{ID: "breakpoints-delete", Color: slate, Accent: red, Parts: join(moduleText(markPage, markPageFold, markPageLines), over(
		path(Mark, "M1.5 1.5L6 6M6 1.5L1.5 6"),
	))},
	{ID: "breakpoint-up", Color: brown, Parts: []Part{
		breakpointDot(9.5, 9.5, 5, red),
		path(Plate, "M5 1.5L8.5 5H6.5V8.5H3.5V5H1.5Z"),
	}},
	{ID: "breakpoint-down", Color: brown, Parts: []Part{
		breakpointDot(9.5, 9.5, 5, red),
		path(Plate, "M3.5 1.5H6.5V5H8.5L5 8.5L1.5 5H3.5Z"),
	}},
	{ID: "breakpoint-cut", Color: slate, Parts: join(scissors(), []Part{breakpointDot(12.5, 3.5, 3, red)})},
	{ID: "breakpoint-copy", Color: slate, Parts: join(twoPages(), []Part{breakpointDot(12.5, 3.5, 3, red)})},
	{ID: "breakpoint-paste", Color: gray, Accent: slate, Parts: join(clipboard(), []Part{breakpointDot(12.5, 3.5, 3, red)})},
	// Список точек останова: включить все, отключить все.
	{ID: "check-all", Color: slate, Accent: green, Parts: join(stackedWindows(), []Part{
		path(Mark, "M3.5 9.5L5.75 11.75L9.5 7.5"),
	})},
	{ID: "uncheck-all", Color: slate, Parts: stackedWindows()},
	// Меню «Отладка»: окна.
	{ID: "evaluate-expression", Color: slate, Parts: join(moduleText(markPage, markPageFold, markPageLines), lens(4, 4, 2.5))},
	{ID: "watch", Color: slate, Accent: navy, Parts: join([]Part{
		rect(Body, 3.5, 2.5, 11, 12), rect(Accent, 3.5, 2.5, 11, 2.5),
	}, lens(4, 4.5, 2.5))},
	{ID: "call-stack", Color: slate, Accent: navy, Parts: []Part{
		rect(Body, 3.5, 1.5, 11, 12), rect(Accent, 3.5, 1.5, 11, 2.5),
		colored(rect(Accent, 5.5, 5.5, 3, 3), groupColor), colored(rect(Accent, 8.5, 8, 3, 3), groupColor),
		colored(rect(Accent, 11.5, 10.5, 3, 3), groupColor),
		colored(path(Line, "M3 6.5Q0.5 11 5 14.5H8M6.5 13L8 14.5L6.5 16"), blue),
	}},

	// Git: ветка, фиксация, получение и отправка, различия, конфликт.
	{ID: "git-branch", Color: orange, Parts: []Part{
		path(Detail, "M4.5 5V11M11.5 7Q11.5 10 4.5 11"),
		circle(Body, 4.5, 3.5, 2), circle(Body, 4.5, 12.5, 2), circle(Body, 11.5, 5, 2),
	}},
	{ID: "git-commit", Color: orange, Parts: []Part{
		path(Detail, "M1 8H4.5M11.5 8H15"), circle(Body, 8, 8, 3.5),
	}},
	{ID: "git-pull", Color: orange, Accent: blue, Parts: []Part{
		path(Detail, "M1 12.5H5.5M10.5 12.5H15"), circle(Body, 8, 12.5, 2.5),
		path(Line, "M8 1.5V8M5.5 5.5L8 8L10.5 5.5"),
	}},
	{ID: "git-push", Color: orange, Accent: blue, Parts: []Part{
		path(Detail, "M1 12.5H5.5M10.5 12.5H15"), circle(Body, 8, 12.5, 2.5),
		path(Line, "M8 8V1.5M5.5 4L8 1.5L10.5 4"),
	}},
	{ID: "git-diff", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), path(Detail, "M8 1.5V14.5"),
		path(Mark, "M4.75 6V10M2.75 8H6.75"), colored(path(Mark, "M9.25 8H13.25"), red),
	}},
	{ID: "git-conflict", Color: orange, Accent: red, Parts: []Part{
		path(Detail, "M3.5 1.5V4Q3.5 7 8 8.5M12.5 1.5V4Q12.5 7 8 8.5V14.5"),
		circle(Accent, 8, 9, 2.5),
	}},
}
