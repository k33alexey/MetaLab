package icons

import "fmt"

// The form editor: the tree of elements, the attributes, commands and
// parameters of a form, the tabs and the panels around them, as the
// inventory photographed them (docs/block-00/redaktor-formy.md). The
// prototype draws every field one red dash and every group one folder - the
// kind of an element is told by its properties, not by its icon - and so do
// we: a field is form-field, a group is catalog-group.

// ink is the dark grey of the arrows of the preview's panel and of the
// cursor in a search string; drawn as details, it lightens on a dark theme.
const ink = "#4a5560"

// rose is the colour of a form's parameters and of the face of a button.
const rose = "#c4846f"

// columns is a table seen as its columns: the table of a form and the
// attribute holding a collection.
func columns(top float64) []Part {
	return gridTable(1.5, top, 13, 14.5-top)
}

// gridTable is a table of three columns under a header in the accent
// colour, at x, y of width by height: the table of a form, the tables of a
// query and the table the settings of data composition are laid over.
func gridTable(x, y, width, height float64) []Part {
	first, second := thirds(x, width)
	return []Part{
		box(Body, x, y, width, height),
		box(Accent, x, y, width, 2.5),
		path(Detail, fmt.Sprintf("M%s %sV%sM%s %sV%s", number(first), number(y), number(y+height),
			number(second), number(y), number(y+height))),
	}
}

// middleColumn is the column of a table of the whole grid picked out in a
// colour, filling it between its lines.
func middleColumn(top float64, color string) Part {
	first, second := thirds(1.5, 13)
	return colored(rect(Accent, first+0.5, top+2.5, second-first-1, 11.5-top), color)
}

// monitor is the screen the form is previewed on, on a neck two pixels
// wide: one line on the middle of the grid would fall between two pixels.
func monitor() []Part {
	return []Part{rect(Body, 1.5, 1.5, 13, 10), path(Detail, "M7.5 12V13.5M8.5 12V13.5M4.5 14.5H11.5")}
}

// okButton is the button of a form with «OK» on it, from left, top width
// points wide and height high, its frame in role frame; its letters, the
// «O» w points wide between its sides and both letters h points high, stand
// in the middle of it in role letters: the button of the form, and small at
// the lower right of the floppy the «OK» ending an edit (the owner's
// choices of 17.10.2026).
func okButton(left, top, width, height, w, h float64, frame, letters Role) []Part {
	x := left + 1 + (width-1-(2*w+3))/2
	y := top + 1 + (height-1-(h+1))/2
	return []Part{rect(frame, left, top, width, height), path(letters, okLetters(x, y, w, h))}
}

// okLetters are the letters «OK», curved as letters are, the stems on the
// pixel grid: the «O» from x to x+w between the middles of its sides, the
// stem of the «K» two points after it and its arms w-0.25 long, both from y
// to y+h.
func okLetters(x, y, w, h float64) string {
	at := func(dx, dy float64) string { return number(x+dx) + " " + number(y+dy) }
	mid, stem := h/2, w+2
	return "M" + at(w/2, 0) + "Q" + at(0, 0) + " " + at(0, mid) + "Q" + at(0, h) + " " + at(w/2, h) +
		"Q" + at(w, h) + " " + at(w, mid) + "Q" + at(w, 0) + " " + at(w/2, 0) + "Z" +
		"M" + at(stem, 0) + "V" + number(y+h) + "M" + at(stem+w, 0) + "L" + at(stem+0.25, mid) + "L" + at(stem+w, h)
}

var formEditorIcons = []Icon{
	// Дерево элементов: поле (любого вида, и колонка таблицы), таблица,
	// командная панель, кнопка, декорация, контекстное меню, дополнения.
	{ID: "form-field", Color: red, Accent: red, Parts: []Part{bar(2, 6, 12, 4)}},
	{ID: "form-table", Color: slate, Accent: steel, Parts: columns(1.5)},
	// The command bar is three round sockets side by side, each with its
	// colour inside, as the jacks of a television (the owner's choice of
	// 16.10.2026: buttons of four pixels with gaps read as dots).
	{ID: "form-command-bar", Color: gray, Parts: []Part{
		colored(circle(Accent, 3, 8, 2), blue), colored(circle(Accent, 8, 8, 2), red), colored(circle(Accent, 13, 8, 2), "#efc23a"),
		circle(Detail, 3, 8, 2.5), circle(Detail, 8, 8, 2.5), circle(Detail, 13, 8, 2.5),
	}},
	// The button with «OK»: the letters close together, two pixels free on
	// either side inside it (the owner's choice of 17.10.2026).
	{ID: "form-button", Color: rose, Parts: func() []Part {
		button := okButton(1.5, 3.5, 14, 9, 3, 5, Body, Detail)
		return []Part{button[0], colored(button[1], brown)}
	}()},
	{ID: "form-decoration", Color: steel, Accent: green, Parts: join(landscape()[:2], []Part{
		landscape()[2], colored(path(Line, "M1 15L15 1"), red),
	})},
	{ID: "form-context-menu", Color: slate, Parts: join([]Part{
		rect(Body, 4.5, 4.5, 10, 10),
		colored(bar(1, 1, 8, 3), red),
	}, []Part{
		colored(rect(Accent, 6, 6, 3, 3), red), colored(rect(Accent, 6, 10, 3, 3), navy),
		path(Detail, "M10.5 7.5H12.5M10.5 11.5H12.5"),
	})},
	{ID: "form-addition", Color: groupColor, Accent: yellow, Parts: join([]Part{
		rect(Body, 9.5, 9.5, 5, 5),
		colored(circle(Accent, 4, 4, 3), blue),
	}, []Part{colored(path(Line, "M12.5 12.5L8.5 8.5"), green), colored(path(Accent, head(7, 7, -1, -1, 2)), green)})},
	{ID: "form-search-string", Color: gray, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13), colored(path(Detail, "M5.5 4V12"), ink),
	}},
	{ID: "form-view-status", Color: groupColor, Accent: gray, Parts: []Part{
		rect(Body, 1.5, 4.5, 13, 8), path(Line, "M3.5 8.5H9M11 7.5L13 9.5M13 7.5L11 9.5"),
	}},
	{ID: "form-search-control", Color: slate, Parts: join([]Part{rect(Body, 1.5, 1.5, 13, 13)}, tint(lens(7, 7, 3.5), blue))},
	// Реквизиты формы: реквизит — черта реквизита (attribute), коллекция —
	// таблица колонками, таблица значений — колонками с реквизитом над ней,
	// колонка коллекции — красная колонка.
	{ID: "form-value-table", Color: slate, Accent: steel, Parts: join(columns(5.5), []Part{
		middleColumn(5.5, blue), colored(bar(4, 1, 8, 3), slate),
	})},
	{ID: "form-collection-column", Color: slate, Accent: steel, Parts: join(columns(1.5), []Part{middleColumn(1.5, red)})},
	{ID: "form-command", Color: gold, Accent: gold, Parts: []Part{bar(2, 6, 12, 4)}},
	{ID: "form-parameter", Color: rose, Accent: rose, Parts: []Part{bar(2, 6, 12, 4)}},
	// Отметка «выведено на форму» в списках реквизитов и команд.
	{ID: "form-used", Color: gray, Parts: []Part{rect(Body, 4.5, 4.5, 7, 7)}},
	// Закладки редактора: элементы, командный интерфейс, реквизиты, команды,
	// параметры; команды формы, стандартные; глобальные — значок команды.
	{ID: "form-elements-tab", Color: red, Accent: red, Parts: barStack()},
	{ID: "form-command-interface-tab", Color: green, Accent: green, Parts: barStack()},
	{ID: "form-attributes-tab", Color: slate, Accent: slate, Parts: barStack()},
	{ID: "form-commands-tab", Color: gold, Accent: gold, Parts: barStack()},
	{ID: "form-parameters-tab", Color: rose, Accent: rose, Parts: barStack()},
	{ID: "form-own-commands", Color: slate, Accent: steel, Parts: join([]Part{
		rect(Body, 1.5, 2.5, 13, 11), rect(Accent, 2, 3, 12, 2),
	}, []Part{path(Plate, "M6.5 6.75L11 9.25L6.5 11.75Z")})},
	{ID: "form-standard-commands", Color: slate, Accent: steel, Parts: join(formWindow(), []Part{
		colored(circle(Plate, 11.5, 11.5, 3), gold), colored(circle(Body, 11.5, 11.5, 3), gold),
		colored(path(Accent, "M10 9.5L13 11.5L10 13.5Z"), gold),
	})},
	// Панель дерева элементов: проверить, вид предпросмотра, повернуть,
	// разрешение, масштаб, ошибки размещения (добавить, изменить, удалить,
	// вверх, вниз — из пачки 1).
	{ID: "form-check", Color: slate, Accent: steel, Parts: join(formWindow(), []Part{
		colored(path(Accent, "M11 6.5L15.5 10L11 13.5Z"), brown),
	})},
	{ID: "preview-platform", Color: slate, Accent: "#efc23a", Parts: join(monitor(), []Part{rect(Accent, 2, 2, 12, 2)})},
	{ID: "preview-display", Color: slate, Accent: gray, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), rect(Accent, 3, 3, 10, 4), rect(Accent, 3, 9, 10, 4),
	}},
	{ID: "rotate-screen", Color: ink, Parts: []Part{
		path(Detail, "M4 3.5Q11 3.5 12 12"), path(Tip, head(0.5, 3.5, -1, 0, 2)), path(Tip, head(12, 15.5, 0, 1, 2)),
	}},
	{ID: "screen-resolution", Color: slate, Parts: join(monitor(), []Part{
		colored(path(Detail, "M5.5 8.5L9.5 4.5"), ink), colored(path(Tip, head(4, 10, -1, 1, 2)), ink), colored(path(Tip, head(11, 3, 1, -1, 2)), ink),
	})},
	{ID: "zoom-scale", Color: ink, Parts: []Part{
		path(Detail, "M4 12L12 4"), path(Tip, head(2, 14, -1, 1, 2.5)), path(Tip, head(14, 2, 1, -1, 2.5)),
	}},
	{ID: "placement-errors", Color: purple, Accent: purple, Parts: []Part{path(Mark, "M3 8.5L6.5 12L13 4")}},
	// Панель реквизитов: добавить реквизит, добавить колонку.
	{ID: "attribute-add", Color: slate, Accent: slate, Parts: join([]Part{bar(1, 3, 10, 4)}, plusBadge(11.5, 11.5))},
	{ID: "attribute-column-add", Color: slate, Accent: steel, Parts: join(columns(1.5), []Part{middleColumn(1.5, red)}, plusBadge(11.5, 11.5))},
}
