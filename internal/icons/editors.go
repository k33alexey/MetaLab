package icons

import "fmt"

// The pictures inside the editors' windows: the property palette, the window
// of a type, the module editor with its completion list and its panel, as the
// inventory of block 00 photographed them (obolochka/, konstanta/,
// redaktor-modulya/).

// brackets are the round brackets of a call, left and right on the
// half-pixel grid from top to bottom: bent at their ends and straight in the
// middle, so that each is one column of pixels and does not merge with the
// letter before it (the owner's choice of 16.10.2026 for P() of the list of
// procedures, and so every call the set draws).
func brackets(left, right, top, bottom float64) string {
	bracket := func(x, side float64) string {
		return fmt.Sprintf("M%s %sQ%s %s %s %sV%sQ%s %s %s %s",
			number(x+side*0.5), number(top), number(x), number(top+0.5), number(x), number(top+1.25),
			number(bottom-1.25), number(x), number(bottom-0.5), number(x+side*0.5), number(bottom))
	}
	return bracket(left, 1) + bracket(right, -1)
}

// funnel is the sign of a selection: a cone as deep as half its width
// narrowing to a neck of 3 points with a flat end, as the prototype's library
// draws it (a slanted end read as bitten at 16 px, the owner's remark of
// 16.10.2026) - the filter of the palette's important properties, a filter
// criterion before its table, the selections of a list.
func funnel(left, top, width float64) string {
	neck, cone := left+width/2, top+(width-1)/2
	return fmt.Sprintf("M%s %sH%sL%s %sV%sH%sV%sZ", number(left), number(top), number(left+width),
		number(neck+1.5), number(cone), number(cone+5), number(neck-1.5), number(cone))
}

// slashes are the «//» of a comment in a module, from the bottom of the grid
// to its top.
func slashes() Part {
	return path(Mark, "M2.5 14.5L7 1.5M8.5 14.5L13 1.5")
}

var editorIcons = []Icon{
	// Палитра свойств.
	{ID: "sort-alphabetical", Color: slate, Accent: blue, Parts: []Part{
		path(Detail, "M1.5 7L3.75 1.5L6 7M2.5 4.5H5M1.5 9.5H6L1.5 14.5H6"),
		path(Line, "M11.5 1.5V11"), path(Accent, head(11.5, 14.5, 0, 1, 2)),
	}},
	{ID: "sort-by-category", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		rect(Accent, 3, 3, 6, 2), rect(Accent, 3, 9, 6, 2),
		path(Detail, "M5 6.5H12.5M5 12.5H12.5"),
	}},
	{ID: "only-important", Color: steel, Accent: red, Parts: []Part{
		path(Body, funnel(1.5, 2.5, 11)),
		path(Line, "M10.5 10.5H14.5M10.5 12.5H14.5M10.5 14.5H14.5"),
	}},
	{ID: "cancel-edit", Color: steel, Accent: steel, Parts: []Part{
		path(Mark, cross),
	}},
	{ID: "apply", Color: green, Accent: green, Parts: []Part{
		path(Mark, "M2.5 8.5L6 12L13.5 4"),
	}},
	// The magnifier inside a localized field of the palette and of a window,
	// which opens «Строки на разных языках» (the owner's snapshot of
	// 19.10.2026, materials/block-00/konfiguraciya). It is not the library's
	// Лупа: that one is drawn as two windows (standartnye-kartinki.md). Grey,
	// as the buttons of an input field are.
	{ID: "field-translations", Color: gray, Parts: lens(6, 6, 4.5)},
	// Окно типа: примитивные типы, ссылка, любая ссылка. Дата — значок
	// календаря, характеристика и определяемый тип — папка группы.
	{ID: "type-number", Color: blue, Parts: []Part{
		path(Detail, "M1.5 5L3.5 3.5V12.5"),
		path(Detail, "M5.5 5.5Q5.5 3.5 7.25 3.5Q9 3.5 9 5.5Q9 7.5 5.5 12.5H9.5"),
		path(Detail, "M10.75 4Q11.5 3.5 12.5 3.5Q14.25 3.5 14.25 5.5Q14.25 7.75 12.25 7.75Q14.5 7.75 14.5 10.25Q14.5 12.5 12.5 12.5Q11.25 12.5 10.75 12"),
	}},
	{ID: "type-string", Color: slate, Accent: blue, Parts: []Part{
		path(Detail, "M4.5 2.5V6.5M4.5 4.5Q4.5 2.5 3.25 2.5Q2 2.5 2 4.5Q2 6.5 3.25 6.5Q4.5 6.5 4.5 5"),
		path(Detail, "M6.5 4V10M6.5 8Q6.5 6 7.75 6Q9 6 9 8Q9 10 7.75 10Q6.5 10 6.5 8.5"),
		path(Line, "M14 10.25Q13.5 9.5 12.5 9.5Q11 9.5 11 11.75Q11 14 12.5 14Q13.5 14 14 13.25"),
	}},
	{ID: "type-boolean", Color: green, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		path(Mark, "M4.5 8L7 10.5L11.5 5"),
	}},
	{ID: "type-value-storage", Color: gold, Accent: "#efc23a", Parts: []Part{
		path(Body, "M2.5 6.5H13.5V13Q13.5 14.5 12 14.5H4Q2.5 14.5 2.5 13Z"),
		rect(Accent, 1, 4, 14, 3),
		path(Detail, "M5.5 4V2.5Q5.5 1.5 6.5 1.5H9.5Q10.5 1.5 10.5 2.5V4M6.5 10.5H9.5"),
	}},
	{ID: "type-uuid", Color: gray, Parts: []Part{
		path(Detail, "M3.5 3.5V12.5M1.5 3.5H5.5M1.5 12.5H5.5"),
		path(Detail, "M7.5 3.5V12.5H9.5Q14.5 12.5 14.5 8Q14.5 3.5 9.5 3.5Z"),
	}},
	// reference is the sign of a reference, as the prototype draws it.
	{ID: "reference", Color: ink, Parts: link(8, 8, 7, 3, 1.25)},
	// ЛюбаяСсылка: three axes from one corner, x, y and z turned upside down.
	{ID: "type-any-reference", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M3.5 12V3.5H12M3.5 3.5L11 11"),
		path(Accent, "M11 1L15 3.5L11 6Z"),
		path(Accent, "M1 11L3.5 15L6 11Z"),
		path(Accent, "M14 9V14H9Z"),
	}},
	// Подсказка и список процедур редактора модуля.
	{ID: "property", Color: green, Accent: green, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "variable", Color: orange, Accent: orange, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "method-function", Color: green, Parts: []Part{
		path(Detail, "M5.5 13V5Q5.5 3.5 7 3.5M4 7.5H7"+brackets(9.5, 13.5, 5, 12)),
	}},
	{ID: "method-procedure", Color: green, Parts: []Part{
		path(Detail, "M3.5 6.5V14.5M3.5 8Q3.5 6.5 5 6.5Q6.5 6.5 6.5 8.5Q6.5 10.5 5 10.5Q3.5 10.5 3.5 9"+brackets(9.5, 13.5, 5, 12)),
	}},
	{ID: "procedure", Color: red, Accent: red, Parts: join([]Part{rect(Body, 1.5, 2.5, 13, 11)}, []Part{
		path(Line, "M3.5 11V5.5H5Q6.5 5.5 6.5 7Q6.5 8.5 5 8.5H3.5"+brackets(8.5, 12.5, 5, 11)),
	})},
	{ID: "function", Color: red, Accent: red, Parts: join([]Part{rect(Body, 1.5, 2.5, 13, 11)}, []Part{
		path(Line, "M3.5 11V5.5H6.5M3.5 8.5H5.5"+brackets(8.5, 12.5, 5, 11)),
	})},
	{ID: "keyword", Color: code, Accent: code, Parts: []Part{
		path(Line, "M2.5 3.5H9.5M2.5 7.5H13.5M2.5 11.5H7.5"),
	}},
	// Панель и меню «Текст» редактора модуля.
	{ID: "indent-increase", Color: slate, Accent: blue, Parts: []Part{
		path(Detail, "M7.5 3.5H14.5M7.5 6.5H14.5M7.5 9.5H14.5M1.5 12.5H14.5"),
		path(Accent, head(4.5, 6.5, 1, 0, 2)),
	}},
	{ID: "indent-decrease", Color: slate, Accent: blue, Parts: []Part{
		path(Detail, "M7.5 3.5H14.5M7.5 6.5H14.5M7.5 9.5H14.5M1.5 12.5H14.5"),
		path(Accent, head(1.5, 6.5, -1, 0, 2)),
	}},
	// «//» over the whole grid, two pixels thick; removing the comment the
	// same and at the lower right a red cross over the second stroke, a thin
	// white halo round it (the owner's remarks of 17.10.2026).
	{ID: "comment", Color: green, Accent: green, Parts: []Part{slashes()}},
	{ID: "uncomment", Color: green, Accent: green, Parts: append([]Part{slashes()},
		cleared(colored(path(Mark, "M7.5 9.5L12 14M12 9.5L7.5 14"), red))...)},
	// The module and over its lower right the green check, cleared by a
	// pixel (the owner's remark of 17.10.2026: a sheet of lines was not the
	// module).
	{ID: "check-module", Color: slate, Accent: code, Parts: join(modulePage(),
		thinlyCleared(colored(path(Mark, "M8.5 12L11 14.5L15 10"), green)))},
	// P() stands in the middle of the glass, as far from its rim on the left
	// as on the right, its brackets straight in the middle so that each is
	// one column of pixels and does not merge with the letter (the owner's
	// choice of 16.10.2026).
	{ID: "procedures-list", Color: slate, Accent: red, Parts: join(lens(7, 7, 5.5), []Part{
		path(Line, "M3.5 9V4.5H4.5Q6 4.5 6 6Q6 7.5 4.5 7.5H3.5"+brackets(7.5, 10.5, 5, 9)),
	})},
	{ID: "bookmark-next", Color: gray, Accent: "#5b636d", Parts: join(flag(), []Part{
		colored(path(Line, "M12.5 9.5V12"), green), colored(path(Accent, head(12.5, 15.5, 0, 1, 2)), green),
	})},
	{ID: "bookmark-previous", Color: gray, Accent: "#5b636d", Parts: join(flag(), []Part{
		colored(path(Line, "M12.5 15.5V13"), green), colored(path(Accent, head(12.5, 9.5, 0, -1, 2)), green),
	})},
	{ID: "bookmarks-delete", Color: gray, Accent: "#5b636d", Parts: join(flag(), []Part{
		colored(path(Mark, "M11 11L14 14M14 11L11 14"), red),
	})},
	// The query wizard, the configurator's command and the library's
	// picture alike: a list over the whole grid and the golden sparks of a
	// wizard, the most used command plain at 16 px (the owner's choices of
	// 16 and 17.10.2026).
	{ID: "query-designer", Color: slate, Parts: join(list(true), wizardSparks())},
	// With its result processed: the same and the wide arrow of «Forward»
	// in a brighter green, cleared from the lines of the list - a thin one
	// was lost in them (the owner's choice of 17.10.2026).
	{ID: "query-designer-processing", Color: slate, Parts: join(list(true), wizardSparks(),
		cleared(colored(path(Accent, wideArrow(8, 12, 1, 0, 7)), brightGreen)))},
}
