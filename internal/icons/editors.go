package icons

import "fmt"

// The pictures inside the editors' windows: the property palette, the window
// of a type, the module editor with its completion list and its panel, as the
// inventory of block 00 photographed them (obolochka/, konstanta/,
// redaktor-modulya/).

const (
	parens  = "M10 5Q8.5 8.5 10 12M12.5 5Q14 8.5 12.5 12"
	wand    = "M5 4.5L13.5 13"
	sparkle = "M10.5 1.5V4.5M9 3H12M13.5 5V7M12.5 6H14.5"
)

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

var editorIcons = []Icon{
	// Палитра свойств.
	{ID: "sort-alphabetical", Color: slate, Accent: blue, Parts: []Part{
		path(Detail, "M1.5 7L3.75 1.5L6 7M2.4 5H5.1M1.5 9.5H6L1.5 14.5H6"),
		path(Line, "M11 1.5V14.5M9 12.5L11 14.5L13 12.5"),
	}},
	{ID: "sort-by-category", Color: slate, Accent: blue, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13),
		rect(Accent, 3, 3, 6, 2), rect(Accent, 3, 9, 6, 2),
		path(Detail, "M5 6.5H12.5M5 12.5H12.5"),
	}},
	{ID: "only-important", Color: steel, Accent: red, Parts: []Part{
		path(Body, funnel(1.5, 2.5, 11)),
		path(Line, "M10.5 9.5H14.5M10.5 12H14.5M10.5 14.5H14.5"),
	}},
	{ID: "cancel-edit", Color: steel, Accent: steel, Parts: []Part{
		path(Mark, cross),
	}},
	{ID: "apply", Color: green, Accent: green, Parts: []Part{
		path(Mark, "M2.5 8.5L6 12L13.5 4"),
	}},
	// Окно типа: примитивные типы, ссылка, любая ссылка. Дата — значок
	// календаря, характеристика и определяемый тип — папка группы.
	{ID: "type-number", Color: blue, Parts: []Part{
		path(Detail, "M1.5 4.5L3.25 3V13"),
		path(Detail, "M5.25 5Q5.25 3 7 3Q8.75 3 8.75 5Q8.75 7 5.25 13H9"),
		path(Detail, "M10.75 3.5Q11.5 3 12.5 3Q14.25 3 14.25 5Q14.25 7.75 12.25 7.75Q14.5 7.75 14.5 10.25Q14.5 13 12.5 13Q11.25 13 10.75 12.5"),
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
		rect(Accent, 1.5, 4, 13, 2.5),
		path(Detail, "M5.5 4V2.5Q5.5 1.5 6.5 1.5H9.5Q10.5 1.5 10.5 2.5V4M6.5 10H9.5"),
	}},
	{ID: "type-uuid", Color: gray, Parts: []Part{
		path(Detail, "M3 3.5V12.5M1.5 3.5H4.5M1.5 12.5H4.5"),
		path(Detail, "M7 3.5V12.5H9.5Q14 12.5 14 8Q14 3.5 9.5 3.5Z"),
	}},
	// reference is the sign of a reference, as the prototype draws it.
	{ID: "reference", Color: ink, Parts: link(8, 8, 7, 3, 1.25)},
	// ЛюбаяСсылка: three axes from one corner, x, y and z turned upside down.
	{ID: "type-any-reference", Color: blue, Accent: blue, Parts: []Part{
		path(Line, "M3.5 12V3.5H12M3.5 3.5L11 11"),
		path(Accent, "M11.5 1L15 3.5L11.5 6Z"),
		path(Accent, "M1 11.5L3.5 15L6 11.5Z"),
		path(Accent, "M13.5 9L13.5 13.5L9 13.5Z"),
	}},
	// Подсказка и список процедур редактора модуля.
	{ID: "property", Color: green, Accent: green, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "variable", Color: orange, Accent: orange, Parts: []Part{
		bar(2, 6, 12, 4),
	}},
	{ID: "method-function", Color: green, Parts: []Part{
		path(Detail, "M5 13V5Q5 3 7 3M3.5 7.5H6.5"+parens),
	}},
	{ID: "method-procedure", Color: green, Parts: []Part{
		path(Detail, "M3.5 6.5V14.5M3.5 8Q3.5 6.5 5.25 6.5Q7 6.5 7 8.75Q7 11 5.25 11Q3.5 11 3.5 9.5"+parens),
	}},
	{ID: "procedure", Color: red, Accent: red, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Line, "M3.75 11V5H5.5Q7.25 5 7.25 6.75Q7.25 8.5 5.5 8.5H3.75M9.75 5Q8.75 8 9.75 11M12 5Q13 8 12 11"),
	}},
	{ID: "function", Color: red, Accent: red, Parts: []Part{
		rect(Body, 1.5, 2.5, 13, 11),
		path(Line, "M3.75 11V5H7M3.75 8H6.25M9.75 5Q8.75 8 9.75 11M12 5Q13 8 12 11"),
	}},
	{ID: "keyword", Color: code, Accent: code, Parts: []Part{
		path(Line, "M2.5 4.5H9.5M2.5 8H13.5M2.5 11.5H7.5"),
	}},
	// Панель и меню «Текст» редактора модуля.
	{ID: "indent-increase", Color: slate, Accent: blue, Parts: []Part{
		path(Detail, "M7.5 3.5H14.5M7.5 6.5H14.5M7.5 9.5H14.5M1.5 12.5H14.5"),
		path(Line, "M1.5 4L4.5 6.5L1.5 9"),
	}},
	{ID: "indent-decrease", Color: slate, Accent: blue, Parts: []Part{
		path(Detail, "M7.5 3.5H14.5M7.5 6.5H14.5M7.5 9.5H14.5M1.5 12.5H14.5"),
		path(Line, "M4.5 4L1.5 6.5L4.5 9"),
	}},
	{ID: "comment", Color: green, Accent: green, Parts: []Part{
		path(Line, "M4.5 13.5L8 2.5M8.5 13.5L12 2.5"),
	}},
	{ID: "uncomment", Color: green, Accent: green, Parts: []Part{
		path(Line, "M2.5 12.5L6 1.5M6.5 12.5L10 1.5"),
		colored(path(Mark, "M11 11L14 14M14 11L11 14"), red),
	}},
	{ID: "check-module", Color: slate, Accent: green, Parts: []Part{
		rect(Body, 1.5, 1.5, 10, 11),
		path(Detail, "M3.5 4.5H5M3.5 7H5"),
		colored(path(Line, "M6.5 4.5H9.5M6.5 7H9.5"), red),
		path(Mark, "M7.5 12L10 14.5L14.5 9.5"),
	}},
	{ID: "procedures-list", Color: slate, Accent: red, Parts: join(lens(7, 7, 5.5), []Part{
		path(Line, "M4.5 9V5H5.75Q7 5 7 6.25Q7 7.5 5.75 7.5H4.5M9 5Q8.25 7 9 9M10.5 5Q11.25 7 10.5 9"),
	})},
	{ID: "bookmark-next", Color: gray, Accent: "#5b636d", Parts: join(flag(), []Part{
		colored(path(Line, "M12.5 9.5V14.5M10.5 12.5L12.5 14.5L14.5 12.5"), green),
	})},
	{ID: "bookmark-previous", Color: gray, Accent: "#5b636d", Parts: join(flag(), []Part{
		colored(path(Line, "M12.5 14.5V9.5M10.5 11.5L12.5 9.5L14.5 11.5"), green),
	})},
	{ID: "bookmarks-delete", Color: gray, Accent: "#5b636d", Parts: join(flag(), []Part{
		colored(path(Mark, "M11 11L14 14M14 11L11 14"), red),
	})},
	{ID: "query-designer", Color: slate, Accent: red, Parts: []Part{
		rect(Body, 1.5, 1.5, 10, 9),
		path(Detail, "M1.5 4H11.5M3.5 6.5H6"),
		colored(path(Line, wand), brown),
		path(Line, sparkle),
	}},
	{ID: "query-designer-processing", Color: slate, Accent: red, Parts: []Part{
		rect(Body, 1.5, 1.5, 10, 9),
		path(Detail, "M1.5 4H11.5"),
		colored(path(Line, wand), brown),
		path(Line, sparkle),
		colored(path(Line, "M1.5 13.5H6.5M5 12L6.5 13.5L5 15"), green),
	}},
}
