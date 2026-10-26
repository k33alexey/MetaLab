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
