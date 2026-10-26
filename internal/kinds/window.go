package kinds

import "slices"

// Page is a page of the window of an object: the key a window builds it by
// and its caption in the list of pages.
type Page struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// windowPages are the pages of the window of an object of a kind, in the
// order of the list at the left of the prototype's window (docs/block-00/
// obshchee.md, «Окно объекта»: the set of pages is the kind's). A kind
// whose window Studio does not build by them yet has none here; a kind the
// prototype opens no window of - a constant - never will.
var windowPages = map[Kind][]Page{
	// docs/block-00/spravochnik.md, «Окно редактирования», fifteen pages.
	Catalog: {
		{"main", "Основные"}, {"subsystems", "Подсистемы"}, {"functional-options", "Функциональные опции"},
		{"hierarchy", "Иерархия"}, {"owners", "Владельцы"}, {"data", "Данные"}, {"numbering", "Нумерация"},
		{"forms", "Формы"}, {"input", "Поле ввода"}, {"commands", "Команды"}, {"templates", "Макеты"},
		{"based-on", "Ввод на основании"}, {"rights", "Права"}, {"exchange", "Обмен данными"}, {"other", "Прочее"},
	},
}

// WindowPages are the pages of the window of an object of the kind, in
// their order; none for a kind whose window has no pages.
func (descriptor Descriptor) WindowPages() []Page {
	return slices.Clone(windowPages[descriptor.Kind])
}

// PaletteRow is a property of the palette of an object: the key it is read
// by - its name in the description, or the role of a module - its caption,
// the kind of its field, whether «Показывать только важные» keeps it, and the
// line the palette shows under the rows while it is chosen.
type PaletteRow struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Field       string `json:"field"`
	Important   bool   `json:"important"`
	Description string `json:"description"`
}

// PaletteTab is a tab of the palette - a category of its properties - and
// whether «Показывать только важные» keeps it.
type PaletteTab struct {
	Key       string       `json:"key"`
	Title     string       `json:"title"`
	Important bool         `json:"important"`
	Rows      []PaletteRow `json:"rows"`
}

// The fields of the rows of a palette: the name of the object, a text in
// every language of the project, a plain text, and the link «Открыть» of a
// module (the rows of modulesRow).
const (
	NameField      = "name"
	LocalizedField = "localized"
	TextField      = "text"
	ModuleField    = "module"
	// modulesRow stands in a palette for a row of every module the kind
	// keeps (Layout.Modules), each its own caption and explanation.
	modulesRow = "modules"
)

// palettes are the tabs of the palette of an object of a kind and their
// rows, in the prototype's order (docs/block-00/<вид>.md, «Палитра
// свойств»). A tab whose rows are not built yet stands empty.
var palettes = map[Kind][]PaletteTab{
	// docs/block-00/spravochnik.md, «Палитра свойств»: four tabs, «только
	// важные» takes «Справочная информация» away.
	Catalog: {
		{Key: "main", Title: "Основные", Important: true, Rows: []PaletteRow{
			{Key: "name", Title: "Имя", Field: NameField, Important: true,
				Description: "Имя объекта в коде и в запросах: одно слово, начинается с буквы."},
			{Key: "title", Title: "Синоним", Field: LocalizedField, Important: true,
				Description: "Название объекта для пользователя на каждом языке проекта."},
			{Key: "comment", Title: "Комментарий", Field: TextField, Important: true,
				Description: "Заметка разработчика об объекте; пользователю не показывается."},
			{Field: modulesRow, Important: true},
		}},
		{Key: "data", Title: "Данные", Important: true},
		{Key: "presentation", Title: "Представление", Important: true},
		{Key: "help", Title: "Справочная информация"},
	},
}

// moduleExplanations are the lines the palette shows under a row of a
// module.
var moduleExplanations = map[Module]string{
	ObjectModule:  "Код, который исполняет объект: обработчики его записи, удаления, заполнения и проверки.",
	ManagerModule: "Код менеджера: процедуры и функции, общие для всех объектов вида.",
}

// Palette is the tabs of the palette of an object of the kind with their
// rows, a row of each module of the kind where the palette lists them.
func (descriptor Descriptor) Palette() []PaletteTab {
	var result []PaletteTab
	for _, tab := range palettes[descriptor.Kind] {
		rows := []PaletteRow{}
		for _, row := range tab.Rows {
			if row.Field != modulesRow {
				rows = append(rows, row)
				continue
			}
			for _, module := range descriptor.Layout().Modules {
				rows = append(rows, PaletteRow{Key: string(module), Title: module.Title(), Field: ModuleField, Important: row.Important,
					Description: moduleExplanations[module]})
			}
		}
		tab.Rows = rows
		result = append(result, tab)
	}
	return result
}
