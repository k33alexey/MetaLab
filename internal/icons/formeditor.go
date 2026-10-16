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
	return []Part{
		box(Body, x, y, width, height),
		box(Accent, x, y, width, 2),
		path(Detail, fmt.Sprintf("M%s %sV%sM%s %sV%s", number(round(x+width/3)), number(y), number(y+height),
			number(round(x+width*2/3)), number(y), number(y+height))),
	}
}

// middleColumn is the column of a table picked out in a colour.
func middleColumn(top float64, color string) Part {
	return colored(rect(Accent, 6.33, top+2.5, 3.33, 11.5-top), color)
}

// monitor is the screen the form is previewed on.
func monitor() []Part {
	return []Part{rect(Body, 1.5, 1.5, 13, 10), path(Detail, "M8 11.5V13.5M4.5 14.5H11.5")}
}

var formEditorIcons = []Icon{
	// Дерево элементов: поле (любого вида, и колонка таблицы), таблица,
	// командная панель, кнопка, декорация, контекстное меню, дополнения.
	{ID: "form-field", Color: red, Accent: red, Parts: []Part{bar(2, 6, 12, 4)}},
	{ID: "form-table", Color: slate, Accent: steel, Parts: columns(1.5)},
	{ID: "form-command-bar", Color: gray, Parts: []Part{
		rect(Body, 0.5, 5.5, 4.5, 5), rect(Body, 5.75, 5.5, 4.5, 5), rect(Body, 11, 5.5, 4.5, 5),
		colored(rect(Accent, 2, 7.25, 1.5, 1.5), blue), colored(rect(Accent, 7.25, 7.25, 1.5, 1.5), red),
		colored(rect(Accent, 12.5, 7.25, 1.5, 1.5), "#efc23a"),
	}},
	{ID: "form-button", Color: rose, Parts: join([]Part{rect(Body, 1.5, 3.5, 13, 9)}, over(
		colored(path(Detail, "M5.25 6Q3.75 6 3.75 8Q3.75 10 5.25 10Q6.75 10 6.75 8Q6.75 6 5.25 6ZM9 6V10M12 6L9.25 8L12 10"), brown),
	))},
	{ID: "form-decoration", Color: steel, Accent: green, Parts: join(landscape()[:2], over(
		landscape()[2], colored(path(Line, "M1 15L15 1"), red),
	))},
	{ID: "form-context-menu", Color: slate, Parts: join([]Part{
		rect(Body, 4.5, 4.5, 10, 10),
		colored(bar(1, 1.5, 8, 3), red),
	}, over(
		colored(rect(Accent, 6.5, 7.5, 2, 2), red), colored(rect(Accent, 6.5, 11, 2, 2), navy),
		path(Detail, "M10 8.5H12.5M10 12H12.5"),
	))},
	{ID: "form-addition", Color: groupColor, Accent: yellow, Parts: join([]Part{
		rect(Body, 9.5, 9.5, 5, 5),
		colored(circle(Accent, 4.5, 4.5, 3), blue),
	}, over(colored(path(Line, "M12.5 12.5L7 7M7 10V7H10"), green)))},
	{ID: "form-search-string", Color: gray, Parts: []Part{
		rect(Body, 2.5, 1.5, 11, 13), colored(path(Detail, "M5.5 4V12"), ink),
	}},
	{ID: "form-view-status", Color: groupColor, Accent: gray, Parts: []Part{
		rect(Body, 1.5, 4.5, 13, 7), path(Line, "M3.5 8H9M11 6.75L13 9.25M13 6.75L11 9.25"),
	}},
	{ID: "form-search-control", Color: slate, Parts: join([]Part{rect(Body, 1.5, 1.5, 13, 13)}, tint(lens(7, 7, 3.5), blue))},
	// Реквизиты формы: реквизит — черта реквизита (attribute), коллекция —
	// таблица колонками, таблица значений — колонками с реквизитом над ней,
	// колонка коллекции — красная колонка.
	{ID: "form-value-table", Color: slate, Accent: steel, Parts: join(columns(5.5), over(
		middleColumn(5.5, blue), colored(bar(4, 1, 8, 3), slate),
	))},
	{ID: "form-collection-column", Color: slate, Accent: steel, Parts: join(columns(1.5), over(middleColumn(1.5, red)))},
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
	}, over(path(Plate, "M6.5 6.75L11 9.25L6.5 11.75Z")))},
	{ID: "form-standard-commands", Color: slate, Accent: steel, Parts: join(formWindow(), over(
		colored(circle(Plate, 11.5, 11.5, 3.5), gold), colored(circle(Body, 11.5, 11.5, 3.5), gold),
		colored(path(Accent, "M10.5 9.75L13.25 11.5L10.5 13.25Z"), gold),
	))},
	// Панель дерева элементов: проверить, вид предпросмотра, повернуть,
	// разрешение, масштаб, ошибки размещения (добавить, изменить, удалить,
	// вверх, вниз — из пачки 1).
	{ID: "form-check", Color: slate, Accent: steel, Parts: join(formWindow(), over(
		colored(path(Accent, "M11 6.5L15.5 10L11 13.5Z"), brown),
	))},
	{ID: "preview-platform", Color: slate, Accent: "#efc23a", Parts: join(monitor(), []Part{rect(Accent, 2, 2, 12, 2.5)})},
	{ID: "preview-display", Color: slate, Accent: gray, Parts: []Part{
		rect(Body, 1.5, 1.5, 13, 13), rect(Accent, 3.5, 3.5, 9, 3.5), rect(Accent, 3.5, 9, 9, 3.5),
	}},
	{ID: "rotate-screen", Color: ink, Parts: []Part{
		path(Detail, "M3.5 3.5Q11 3.5 12 12.5M5.5 1.5L3.5 3.5L5.5 5.5M10 10.5L12 12.5L14 10.5"),
	}},
	{ID: "screen-resolution", Color: slate, Parts: join(monitor(), over(
		colored(path(Detail, "M5 9L11 4M5 6.5V9H7.5M8.5 4H11V6.5"), ink),
	))},
	{ID: "zoom-scale", Color: ink, Parts: []Part{
		path(Detail, "M2.5 13.5L13.5 2.5M2.5 9V13.5H7M9 2.5H13.5V7"),
	}},
	{ID: "placement-errors", Color: purple, Accent: purple, Parts: []Part{path(Mark, "M3 8.5L6.5 12L13 4")}},
	// Панель реквизитов: добавить реквизит, добавить колонку.
	{ID: "attribute-add", Color: slate, Accent: slate, Parts: join([]Part{bar(1, 3, 10, 3.5)}, plusBadge(11, 11))},
	{ID: "attribute-column-add", Color: slate, Accent: steel, Parts: join(columns(1.5), over(middleColumn(1.5, red)), plusBadge(11, 11))},
}
