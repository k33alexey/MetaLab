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
	// Choices are the values of a list and their captions, the prototype's;
	// of a switch, the caption of false and of true.
	Choices [][2]string `json:"choices,omitempty"`
	// Needs are what the row is available by: the key of a property that is
	// on - not false, zero or empty - or key=value; the row shows its value
	// unavailable while one of them is not so.
	Needs []string `json:"needs,omitempty"`
	// Limit names the model's ceiling of a number (metadata.EditorLimits)
	// and Min its least value.
	Limit string `json:"limit,omitempty"`
	Min   int    `json:"min,omitempty"`
	// Lines is how many lines a text of several shows - the explanation.
	Lines int `json:"lines,omitempty"`
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
// every language of the project, a plain text, the link «Открыть» of a
// module (the rows of modulesRow); a flag, a list of choices, a number, a
// flag shown as a list of two (a switch); objects of the project chosen in
// «Выбор объекта», fields of the object, the link «Открыть» of a window of
// the object; and the line between groups of rows.
const (
	NameField      = "name"
	LocalizedField = "localized"
	TextField      = "text"
	ModuleField    = "module"
	FlagField      = "flag"
	ChoiceField    = "choice"
	NumberField    = "number"
	SwitchField    = "switch"
	ObjectsField   = "objects"
	FieldsField    = "fields"
	WindowField    = "window"
	SeparatorField = "separator"
	// FormField is a main form of the object, chosen among its forms,
	// cleared, and opened by the magnifier.
	FormField = "form"
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
		{Key: "data", Title: "Данные", Important: true, Rows: []PaletteRow{
			{Key: "hierarchy.enabled", Title: "Иерархический", Field: FlagField, Important: true,
				Description: "Элементы справочника складываются в дерево: у элемента может быть родитель."},
			{Key: "hierarchy.kind", Title: "Вид иерархии", Field: ChoiceField, Important: true, Needs: []string{"hierarchy.enabled"},
				Choices:     [][2]string{{"folders-and-items", "Иерархия групп и элементов"}, {"items", "Иерархия элементов"}},
				Description: "Кто бывает родителем: только группа или любой элемент."},
			{Key: "hierarchy.limitLevels", Title: "Ограничивать кол-во уровней", Field: FlagField, Important: true, Needs: []string{"hierarchy.enabled"},
				Description: "Дерево не глубже заданного числа уровней."},
			{Key: "hierarchy.levelCount", Title: "Количество уровней", Field: NumberField, Limit: "levels", Min: 1, Important: true,
				Needs:       []string{"hierarchy.enabled", "hierarchy.limitLevels"},
				Description: "Сколько уровней может быть в дереве, считая уровень элементов."},
			{Key: "hierarchy.foldersOnTop", Title: "Размещать группы сверху", Field: FlagField, Important: true, Needs: []string{"hierarchy.enabled"},
				Description: "В иерархическом списке группы стоят перед элементами."},
			{Field: SeparatorField, Important: true},
			{Key: "owners", Title: "Владельцы", Field: ObjectsField, Important: true,
				Description: "Объекты, которым подчинены элементы справочника: у каждого элемента — свой владелец."},
			{Key: "subordination", Title: "Использование подчинения", Field: ChoiceField, Important: true, Needs: []string{"owners"},
				Choices:     [][2]string{{"to-items", "Элементам"}, {"to-folders", "Группам"}, {"to-folders-and-items", "Группам и элементам"}},
				Description: "Чему из владельца подчиняется элемент: элементу, группе или тому и другому."},
			{Key: "code.length", Title: "Длина кода", Field: NumberField, Limit: "code", Important: true,
				Description: "Сколько знаков в коде; ноль — у справочника нет кода."},
			{Key: "descriptionLength", Title: "Длина наименования", Field: NumberField, Limit: "description", Important: true,
				Description: "Сколько знаков в наименовании; ноль — у справочника нет наименования."},
			{Key: "code.type", Title: "Тип кода", Field: ChoiceField, Important: true, Needs: []string{"code.length"},
				Choices: [][2]string{{"number", "Число"}, {"string", "Строка"}}, Description: "Код — число или строка."},
			{Key: "code.fixedLength", Title: "Допустимая длина кода", Field: SwitchField, Important: true,
				Choices:     [][2]string{{"false", "Переменная"}, {"true", "Фиксированная"}},
				Description: "Строковый код хранится как набран или дополняется пробелами до полной длины."},
			{Key: "code.series", Title: "Серии кодов", Field: ChoiceField, Important: true, Needs: []string{"code.length"},
				Choices:     [][2]string{{"whole", "Во всем справочнике"}, {"within-subordination", "В пределах подчинения"}, {"within-owner-subordination", "В пределах подчинения владельцу"}},
				Description: "Среди каких элементов код уникален и продолжается при автонумерации."},
			{Key: "code.unique", Title: "Контроль уникальности", Field: FlagField, Important: true, Needs: []string{"code.length"},
				Description: "Запись элемента с повторным кодом отказывается."},
			{Key: "code.auto", Title: "Автонумерация", Field: FlagField, Important: true, Needs: []string{"code.length"},
				Description: "Новый элемент получает следующий код сам."},
			{Key: "standardAttributes", Title: "Стандартные реквизиты", Field: WindowField, Important: true,
				Description: "Окно стандартных реквизитов: код, наименование, родитель, владелец и другие поля, которые платформа даёт справочнику."},
			{Key: "characteristics", Title: "Характеристики", Field: WindowField,
				Description: "Окно характеристик: дополнительные свойства элементов, которые хранятся в другом объекте."},
			{Key: "predefined", Title: "Предопределенные", Field: WindowField, Important: true,
				Description: "Окно предопределённых элементов: элементы, которые есть в любой базе с этим проектом."},
			{Key: "predefinedDataUpdate", Title: "Обновление предопределенных данных", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"auto", "Авто"}, {"auto-update", "Обновлять автоматически"}, {"dont-auto-update", "Не обновлять автоматически"}},
				Description: "Что делать с предопределёнными элементами в базе при обновлении проекта."},
			{Key: "basedOn", Title: "Вводится на основании", Field: ObjectsField, Important: true,
				Description: "Объекты, на основании которых можно создать элемент справочника."},
			{Key: "dataLockFields", Title: "Поля блокировки данных", Field: FieldsField,
				Description: "Поля, по которым блокируется элемент, пока его записывают."},
			{Key: "dataLock", Title: "Режим управления блокировкой данных", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"automatic", "Автоматический"}, {"managed", "Управляемый"}, {"automatic-and-managed", "Автоматический и управляемый"}},
				Description: "Как блокируются данные справочника при записи: платформой сама, кодом или и так и так."},
			{Key: "fullTextSearch", Title: "Полнотекстовый поиск", Field: ChoiceField, Important: true,
				Choices: [][2]string{{"use", "Использовать"}, {"dont-use", "Не использовать"}}, Description: "Входит ли справочник в полнотекстовый индекс."},
			{Key: "dataHistory", Title: "История данных", Field: ChoiceField, Important: true,
				Choices: [][2]string{{"use", "Использовать"}, {"dont-use", "Не использовать"}}, Description: "Хранятся ли версии элементов справочника."},
			{Key: "updateDataHistoryImmediatelyAfterWrite", Title: "Обновлять историю данных сразу после записи", Field: FlagField, Important: true,
				Needs: []string{"dataHistory=use"}, Description: "Версия строится сразу после записи, а не по расписанию."},
			{Key: "executeAfterDataHistoryVersionWriteProcessing", Title: "Выполнять обработку после записи версии истории данных", Field: FlagField,
				Important: true, Needs: []string{"dataHistory=use"}, Description: "Модуль менеджера узнаёт о каждой записанной версии."},
			{Key: "additionalIndexes", Title: "Дополнительные индексы", Field: WindowField, Important: true,
				Description: "Окно дополнительных индексов: индексы базы сверх тех, что платформа строит сама."},
		}},
		// «только важные» keeps three rows of the input by string
		// (snapshot vazhnye-predstavlenie).
		{Key: "presentation", Title: "Представление", Important: true, Rows: []PaletteRow{
			{Key: "useStandardCommands", Title: "Использовать стандартные команды", Field: FlagField,
				Description: "Платформа сама даёт команды справочника: открыть список, создать элемент и другие."},
			{Key: "defaultPresentation", Title: "Основное представление", Field: ChoiceField,
				Choices:     [][2]string{{"as-code", "В виде кода"}, {"as-description", "В виде наименования"}},
				Description: "Чем элемент представлен в одну строку: кодом или наименованием."},
			{Key: "quickChoice", Title: "Быстрый выбор", Field: FlagField,
				Description: "Элемент выбирается из выпадающего списка, а не из формы выбора."},
			{Key: "choiceMode", Title: "Способ выбора", Field: ChoiceField,
				Choices:     [][2]string{{"both-ways", "Обоими способами"}, {"from-form", "Из формы"}, {"quick-choice", "Быстрый выбор"}},
				Description: "Как выбирается элемент в поле: из формы, быстрым выбором или обоими способами."},
			{Key: "inputByString", Title: "Ввод по строке", Field: FieldsField,
				Description: "Поля, по которым элемент находится по набранному в поле тексту, в порядке поиска."},
			{Key: "searchStringMode", Title: "Способ поиска строки при вводе по строке", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"begin", "Начало"}, {"any-part", "Любая часть"}},
				Description: "Где в поле ищется набранный текст: в начале или в любой части."},
			{Key: "fullTextSearchOnInput", Title: "Полнотекстовый поиск при вводе по строке", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"use", "Использовать"}, {"dont-use", "Не использовать"}},
				Description: "Набранный текст ищется по полнотекстовому индексу, а не сравнением полей."},
			{Key: "choiceDataGetMode", Title: "Режим получения данных выбора при вводе по строке", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"directly", "Непосредственно"}, {"background", "Фоновый"}},
				Description: "Ждёт ли пользователь, пока ищутся элементы по набранному тексту, или поиск идёт в фоне."},
			{Key: "createOnInput", Title: "Создание при вводе", Field: ChoiceField,
				Choices:     [][2]string{{"auto", "Авто"}, {"use", "Использовать"}, {"dont-use", "Не использовать"}},
				Description: "Можно ли создать новый элемент из поля, в котором его не нашли."},
			{Key: "choiceHistoryOnInput", Title: "История выбора при вводе", Field: ChoiceField,
				Choices:     [][2]string{{"auto", "Авто"}, {"dont-use", "Не использовать"}},
				Description: "Предлагаются ли первыми элементы, которые пользователь уже выбирал."},
			{Field: SeparatorField},
			{Key: "forms.object", Title: "Основная форма объекта", Field: FormField, Description: "Форма, которой открывается элемент."},
			{Key: "forms.folder", Title: "Основная форма группы", Field: FormField, Needs: []string{"hierarchy.enabled", "hierarchy.kind=folders-and-items"},
				Description: "Форма, которой открывается группа."},
			{Key: "forms.list", Title: "Основная форма списка", Field: FormField, Description: "Форма, которой открывается список справочника."},
			{Key: "forms.choice", Title: "Основная форма выбора", Field: FormField, Description: "Форма, из которой выбирается элемент."},
			{Key: "forms.folderChoice", Title: "Основная форма выб. группы", Field: FormField, Needs: []string{"hierarchy.enabled", "hierarchy.kind=folders-and-items"},
				Description: "Форма, из которой выбирается группа."},
			{Field: SeparatorField},
			{Key: "objectPresentation", Title: "Представление объекта", Field: LocalizedField,
				Description: "Как называется один элемент: «Товар», а не «Товары»."},
			{Key: "extendedObjectPresentation", Title: "Расширенное представление объекта", Field: LocalizedField,
				Description: "Более длинное название элемента, там, где для него есть место: заголовок формы элемента."},
			{Key: "listPresentation", Title: "Представление списка", Field: LocalizedField, Description: "Как называется весь список: «Товары»."},
			{Key: "extendedListPresentation", Title: "Расширенное представление списка", Field: LocalizedField,
				Description: "Более длинное название списка: заголовок формы списка."},
			{Key: "explanation", Title: "Пояснение", Field: LocalizedField, Lines: 3,
				Description: "Фраза, которой справочник предлагается пользователю рядом с командой, открывающей его."},
		}},
		// The caption of the tab is «Справка», not the prototype's «Справочная
		// информация» (the owner's decision of 26.10.2026, ML-STUDIO.md,
		// «закладка „Справка" палитры»); the property on it keeps its name.
		{Key: "help", Title: "Справка", Rows: []PaletteRow{
			{Key: "includeHelpInContents", Title: "Включать в содержание справки", Field: FlagField,
				Description: "Справка справочника входит в оглавление справки проекта."},
			{Key: "help", Title: "Справочная информация", Field: WindowField, Description: "Текст справки справочника."},
		}},
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
