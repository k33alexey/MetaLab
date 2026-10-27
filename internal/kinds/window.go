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
	// docs/block-00/http-servis.md, «Окно»; snapshots http-servis/01, 09,
	// 17, 18.
	HTTPService: {
		{"main", "Основные"}, {"subsystems", "Подсистемы"}, {"templates", "Шаблоны URL"}, {"sessions", "Управление сеансами"}, {"other", "Прочее"},
	},
	// docs/block-00/podsistema.md, «Окно»; snapshots podsistema/04-07. «Состав»
	// comes with its item.
	Subsystem: {{"main", "Основные"}, {"functional-options", "Функциональные опции"}, {"other", "Прочее"}},
	// docs/block-00/web-servis.md, «Окно»; snapshots web-servis/01-04.
	WebService: {
		{"main", "Основные"}, {"subsystems", "Подсистемы"}, {"operations", "Операции"}, {"sessions", "Управление сеансами"}, {"other", "Прочее"},
	},
	// docs/block-00/spravochnik.md, «Окно редактирования», fifteen pages.
	Catalog: {
		{"main", "Основные"}, {"subsystems", "Подсистемы"}, {"functional-options", "Функциональные опции"},
		{"hierarchy", "Иерархия"}, {"owners", "Владельцы"}, {"data", "Данные"}, {"numbering", "Нумерация"},
		{"forms", "Формы"}, {"input", "Поле ввода"}, {"commands", "Команды"}, {"templates", "Макеты"},
		{"based-on", "Ввод на основании"}, {"rights", "Права"}, {"exchange", "Обмен данными"}, {"other", "Прочее"},
	},
}

// serviceIdentityRows and serviceSessionRows are the pages «Основные» and
// «Управление сеансами» of the window of a web and an HTTP service, the
// same in both (web-servis.md, http-servis.md, «Окно»): the captions end in
// a colon, the reuse of sessions stands above its list.
var (
	serviceIdentityRows = []WindowRow{{Key: "name", Title: "Имя:"}, {Key: "title", Title: "Синоним:"}, {Key: "comment", Title: "Комментарий:"}}
	serviceSessionRows  = []WindowRow{
		{Key: "reuseSessions", Title: "Повторное использование сеансов:", Above: true},
		{Key: "sessionMaxAge", Title: "Время жизни сеанса:"},
	}
)

// WindowRow is a row of a page of the window of an object built of the
// palette's row of the same property (Key): its field, values and what it
// is available by are the palette's; the window gives it its own caption,
// the frame it stands in, whether the caption stands above the field, and
// whether a module or a window opens by a button rather than «Открыть».
type WindowRow struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Frame  string `json:"frame,omitempty"`
	Above  bool   `json:"above,omitempty"`
	Button bool   `json:"button,omitempty"`
}

// windowRows are the pages of a window built of the palette's rows, by
// page (docs/block-00/<вид>.md, «Окно редактирования»).
var windowRows = map[Kind]map[string][]WindowRow{
	// docs/block-00/http-servis.md, «Окно»; snapshots http-servis/01, 17,
	// 18: the root after the comment, the sessions as a web service's, the
	// module alone on «Прочее».
	HTTPService: {
		"main":     append(slices.Clone(serviceIdentityRows), WindowRow{Key: "rootUrl", Title: "Корневой URL:"}),
		"sessions": serviceSessionRows,
		"other":    {{Key: string(ServiceModule), Title: "Модуль", Button: true}},
	},
	// docs/block-00/podsistema.md, «Окно»; snapshots podsistema/04, 05, 07:
	// the flags under the comment, «Командный интерфейс» a button, the
	// explanation and the picture; the help a button over its flag. The
	// page of the options is the marks of the options (objectWindowEditor).
	Subsystem: {
		"main": append(slices.Clone(serviceIdentityRows), WindowRow{Key: "includeInCommandInterface", Title: "Включать в командный интерфейс"},
			WindowRow{Key: "useOneCommand", Title: "Подсистема с одной командой"}, WindowRow{Key: "commandInterface", Title: "Командный интерфейс", Button: true},
			WindowRow{Key: "explanation", Title: "Пояснение:", Above: true}, WindowRow{Key: "picture", Title: "Картинка:"}),
		"other": {{Key: "help", Title: "Справочная информация", Button: true}, {Key: "includeHelpInContents", Title: "Включать в содержание справки"}},
	},
	// docs/block-00/web-servis.md, «Окно»; snapshots web-servis/01, 03, 04:
	// the captions end in a colon, the reuse of sessions stands above its
	// list, the module opens by a button.
	WebService: {
		"main":     serviceIdentityRows,
		"sessions": serviceSessionRows,
		"other": {
			{Key: string(ServiceModule), Title: "Модуль", Button: true},
			{Key: "namespace", Title: "URI пространства имен:"},
			{Key: "packages", Title: "Пакеты XDTO:"},
			{Key: "descriptorFile", Title: "Имя файла публикации:"},
		},
	},
	// docs/block-00/spravochnik.md, pages 8, 9, 10 and 15; snapshots
	// okno-08, okno-09, okno-10, okno-15.
	Catalog: {
		"forms": {
			{Key: "forms.object", Title: "Элемента:", Frame: "Формы"},
			{Key: "forms.folder", Title: "Группы:", Frame: "Формы"},
			{Key: "forms.list", Title: "Списка:", Frame: "Формы"},
			{Key: "forms.choice", Title: "Выбора:", Frame: "Формы"},
			{Key: "forms.folderChoice", Title: "Выбора группы:", Frame: "Формы"},
		},
		"commands": {
			{Key: "useStandardCommands", Title: "Использовать стандартные команды"},
		},
		"input": {
			{Key: "quickChoice", Title: "Быстрый выбор"},
			{Key: "createOnInput", Title: "Создание при вводе:"},
			{Key: "inputByString", Title: "Ввод по строке:", Frame: "Ввод по строке"},
			{Key: "searchStringMode", Title: "Способ поиска строки:", Frame: "Ввод по строке"},
			{Key: "fullTextSearchOnInput", Title: "Полнотекстовый поиск:", Frame: "Ввод по строке"},
			{Key: "choiceDataGetMode", Title: "Режим получения данных выбора:", Frame: "Ввод по строке"},
			{Key: "choiceHistoryOnInput", Title: "История выбора при вводе:"},
		},
		"other": {
			{Key: string(ObjectModule), Title: "Модуль объекта", Button: true},
			{Key: string(ManagerModule), Title: "Модуль менеджера", Button: true},
			{Key: "predefined", Title: "Предопределенные", Button: true},
			{Key: "predefinedDataUpdate", Title: "Обновление предопределенных данных:", Above: true},
			{Key: "dataLockFields", Title: "Поля блокировки данных:"},
			{Key: "fullTextSearch", Title: "Полнотекстовый поиск:"},
			{Key: "dataHistory", Title: "История данных:"},
			{Key: "updateDataHistoryImmediatelyAfterWrite", Title: "Обновлять историю данных сразу после записи"},
			{Key: "executeAfterDataHistoryVersionWriteProcessing", Title: "Выполнять обработку после записи версии истории данных"},
			{Key: "help", Title: "Справочная информация", Button: true},
			{Key: "includeHelpInContents", Title: "Включать в содержание справки"},
		},
	},
}

// WindowRows are the pages of the window of an object of the kind built of
// the palette's rows, by the key of the page.
func (descriptor Descriptor) WindowRows() map[string][]WindowRow {
	return windowRows[descriptor.Kind]
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
	// Max is the greatest value of a number the kind bounds itself.
	Max int `json:"max,omitempty"`
	// Default is the value a list shows where the description writes none,
	// the model's meaning of an absent one (not indexed, not checked).
	Default string `json:"default,omitempty"`
	// Fixed is a property set once, when the object is made, and shown
	// unavailable after: the kind of a template (obshchij-maket.md,
	// «Палитра»).
	Fixed bool `json:"fixed,omitempty"`
	// Optional is a value a node may leave not said, as the palette of a
	// node of the editor of an XDTO package shows it (paket-xdto.md,
	// «Палитра узла»): not said, the field shows Default greyed with ✓
	// beside it, which says it; said, ✕ takes it back.
	Optional bool `json:"optional,omitempty"`
	// Shown are what the row stands by at all, unlike Needs, which leave it
	// standing unavailable: type=<kind> - the kind of the one type of the
	// part (string, number, reference), owner=<kind> - the kind of the
	// object the part belongs to.
	Shown []string `json:"shown,omitempty"`
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
	// PasswordField is a password kept apart from the project - shown by dots
	// when there is one - and HeadersField the headers of a WebSocket
	// client - «Имя(значение)» by commas, «…» opening «Настройка заголовков»,
	// the cross clearing them (websocket-klient.md).
	PasswordField = "password"
	HeadersField  = "headers"
	// XMLTypeField is a type named by the schema - the type of an operation
	// of a web service and of its parameter: its name and namespace shown,
	// «…» choosing it in the window of the types of the packages.
	XMLTypeField = "xml-type"
	// XMLTypesField is a list of types named by the schema - the members of
	// a union of an XDTO package.
	XMLTypesField = "xml-types"
	// PictureField is a picture - of the platform by its name, of the
	// project by its own - chosen by the window «Выбор картинки».
	PictureField = "picture"
	// PackagesField is the XDTO packages a web service uses: the namespaces
	// shown, «…» choosing them, the cross clearing them (web-servis.md).
	PackagesField = "packages"
	// FormField is a main form of the object, chosen among its forms,
	// cleared, and opened by the magnifier.
	FormField = "form"
	// LinkField is an address shown as a link and not edited: the source of
	// a WS reference, which its import alone writes (ws-ssylka.md).
	LinkField = "link"
	// TypeField is the type of a part with «…» of the window «Редактирование
	// типа данных»; ValueField a value of the part's type.
	TypeField  = "type"
	ValueField = "value"
	// modulesRow stands in a palette for a row of every module the kind
	// keeps (Layout.Modules), each its own caption and explanation.
	modulesRow = "modules"
)

// palettes are the tabs of the palette of an object of a kind and their
// rows, in the prototype's order (docs/block-00/<вид>.md, «Палитра
// свойств»). A tab whose rows are not built yet stands empty.
var palettes = map[Kind][]PaletteTab{
	// docs/block-00/websocket-klient.md, «Палитра»; snapshots
	// websocket-klient/09, 11: one tab - two flags, the module, the address,
	// the user, the password by dots, the headers, two flags of the system,
	// the timeout in seconds.
	WebSocketClient: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("клиента", "о клиенте"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "predefined", Title: "Предопределенный", Field: FlagField, Important: true,
				Description: "Платформа сама создаёт клиент по этому объекту."},
			{Key: "autoConnect", Title: "Подключать автоматически", Field: FlagField, Important: true,
				Description: "Клиент подключается при запуске приложения и снова — когда соединение обрывается."},
			{Key: string(ClientModule), Title: "Модуль", Field: ModuleField, Important: true,
				Description: "Код клиента: обработчики событий соединения."},
			{Key: "serverUrl", Title: "URL сервера", Field: TextField, Important: true, Description: "Адрес сервера, к которому подключается клиент."},
			{Key: "user", Title: "Пользователь", Field: TextField, Important: true, Description: "Имя, с которым клиент входит на сервер."},
			{Key: "passwordSet", Title: "Пароль", Field: PasswordField, Important: true,
				Description: "Пароль входа на сервер; хранится в защищённом хранилище системы, а не в файлах проекта."},
			{Key: "headers", Title: "Заголовки", Field: HeadersField, Important: true,
				Description: "Заголовки запроса, которым открывается соединение."},
			{Key: "useOsProxy", Title: "Использовать прокси ОС", Field: FlagField, Important: true,
				Description: "Соединение идёт через прокси, настроенный в системе."},
			{Key: "useOsAuthentication", Title: "Использовать аутентификацию ОС", Field: FlagField, Important: true,
				Description: "Клиент входит на сервер учётной записью системы."},
			{Key: "timeout", Title: "Таймаут (секунд)", Field: NumberField, Important: true,
				Description: "Сколько секунд ждать подключения."},
		})},
	},
	// docs/block-00/web-servis.md, «Палитра»: one tab - the packages after
	// the comment, then the namespace, the module, the file of publication,
	// the reuse of sessions and their lifetime.
	WebService: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("web-сервиса", "о web-сервисе"), []PaletteRow{
			{Key: "packages", Title: "Пакеты XDTO", Field: PackagesField, Important: true,
				Description: "Пакеты XDTO, типы которых сервис принимает и возвращает."},
			{Field: SeparatorField, Important: true},
			{Key: "namespace", Title: "URI пространства имен", Field: TextField, Important: true,
				Description: "Пространство имён сервиса в его описании WSDL."},
			{Key: string(ServiceModule), Title: "Модуль", Field: ModuleField, Important: true,
				Description: "Код сервиса: процедуры, которые исполняют его операции."},
			{Key: "descriptorFile", Title: "Имя файла публикации", Field: TextField, Important: true,
				Description: "Имя файла, по которому сервис публикуется на веб-сервере."},
		}, sessionPaletteRows)},
	},
	// docs/block-00/http-servis.md, «Палитра»; snapshot http-servis/01:
	// one tab - the root, the module, the reuse of sessions and their
	// lifetime after the comment.
	HTTPService: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("HTTP-сервиса", "об HTTP-сервисе"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "rootUrl", Title: "Корневой URL", Field: TextField, Important: true,
				Description: "Первая часть адреса, по которому отвечает сервис: буквы, цифры и знаки - ~ $ ( ) _ ."},
			{Key: string(ServiceModule), Title: "Модуль", Field: ModuleField, Important: true,
				Description: "Код сервиса: процедуры, которые обрабатывают его методы."},
		}, sessionPaletteRows)},
	},
	// docs/block-00/ws-ssylka.md, «Палитра»; snapshot ws-ssylka/03-palitra:
	// one tab, «URL источника» a link the import writes, «WS-ссылка:
	// Открыть» the window viewing the description.
	WSReference: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("WS-ссылки", "о WS-ссылке"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "locationUrl", Title: "URL источника", Field: LinkField, Important: true,
				Description: "Откуда импортировано описание сервиса: адрес или файл; меняется «Импорт WSDL…»."},
			{Key: "definition", Title: "WS-ссылка", Field: WindowField, Important: true, Description: "Описание сервиса: открывается окном просмотра."},
		})},
	},
	// docs/block-00/obshchij-maket.md, «Палитра»; snapshot
	// obshchij-maket/07-novyj-maket-palitra: one tab, «Тип макета»
	// unavailable once the template is made, «Макет: Открыть» opening the
	// editor of its content.
	CommonTemplate: templatePalette,
	// docs/block-00/obshchaya-kartinka.md, «Палитра»; snapshot
	// obshchaya-kartinka/01-palitra: one tab, «Картинка: Открыть» opens the
	// window of the picture, «Доступность для оформления» is unavailable
	// until the picture is available for choice. The palette has no other
	// tab for «только важные» to leave out, so this one keeps everything.
	CommonPicture: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("картинки", "о картинке"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "picture", Title: "Картинка", Field: WindowField, Important: true, Description: "Изображение картинки: открывается её окном."},
			{Key: "availableForChoice", Title: "Доступность для выбора", Field: FlagField, Important: true,
				Description: "Картинку можно выбрать в приложении, задавая картинку ячейки, рисунка или колонтитула табличного документа."},
			{Key: "availableForAppearance", Title: "Доступность для оформления", Field: FlagField, Important: true, Needs: []string{"availableForChoice"},
				Description: "Картинку можно выбрать в приложении и для элемента условного оформления."},
		})},
	},
	// docs/block-00/podsistema.md, «Палитра»; snapshots podsistema/17, 18:
	// «Основные» with «Состав: Открыть» and the two flags, «Представление»
	// with «Командный интерфейс: Открыть», the explanation and the picture,
	// «Справочная информация» with its flag and «Открыть».
	Subsystem: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("подсистемы", "о подсистеме"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "members", Title: "Состав", Field: WindowField, Important: true, Description: "Объекты, которые входят в подсистему."},
			{Key: "includeInCommandInterface", Title: "Включать в командный интерфейс", Field: FlagField, Important: true,
				Description: "Становится ли подсистема разделом командного интерфейса."},
			{Key: "useOneCommand", Title: "Подсистема с одной командой", Field: FlagField, Important: true,
				Description: "Открывается ли раздел одной командой вместо панели своих команд."},
		})},
		{Key: "presentation", Title: "Представление", Important: true, Rows: []PaletteRow{
			{Key: "commandInterface", Title: "Командный интерфейс", Field: WindowField, Important: true, Description: "Команды раздела, их группы и видимость."},
			{Key: "explanation", Title: "Пояснение", Field: LocalizedField, Lines: 3, Important: true, Description: "Текст, которым раздел пояснён пользователю."},
			{Key: "picture", Title: "Картинка", Field: PictureField, Important: true, Description: "Картинка раздела в панели разделов."},
		}},
		{Key: "help", Title: "Справочная информация", Important: true, Rows: []PaletteRow{
			{Key: "includeHelpInContents", Title: "Включать в содержание справки", Field: FlagField, Important: true,
				Description: "Входит ли справка подсистемы в содержание справки приложения."},
			{Key: "help", Title: "Справочная информация", Field: WindowField, Important: true, Description: "Справка подсистемы."},
		}},
	},
	// docs/block-00/paket-xdto.md, «Палитра»; snapshot paket-xdto/01: one
	// tab - the name, the synonym, the comment, the namespace and «Пакет:
	// Открыть», the editor of the package.
	XDTOPackage: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("пакета", "о пакете"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			xdtoNamespaceRow,
			{Key: "content", Title: "Пакет", Field: WindowField, Important: true, Description: "Типы пакета: открываются его редактором."},
		})},
	},
	// docs/block-00/stil.md, «Палитра»; snapshot stil/01-palitra: one tab,
	// «Стиль: Открыть» opening the values of the style in a window of
	// their own with OK and «Отмена».
	Style: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("стиля", "о стиле"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "items", Title: "Стиль", Field: WindowField, Important: true, Description: "Значения элементов стиля в этом стиле: открываются отдельным окном."},
		})},
	},
	// docs/block-00/konstanta.md, «Палитра свойств»; snapshots
	// konstanta/palitra-*: three tabs, «только важные» keeps «Основные» and
	// «Данные»; the modules are named as the palette of a constant names
	// them, the manager first.
	Constant: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("константы", "о константе"), []PaletteRow{
			{Field: SeparatorField, Important: true},
		}, typeRows, []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: string(ManagerModule), Title: "Модуль менеджера", Field: ModuleField, Important: true,
				Description: "Код менеджера константы: процедуры и функции, общие для константы."},
			{Key: string(ValueModule), Title: "Модуль менеджера значения", Field: ModuleField, Important: true,
				Description: "Код, который исполняет значение константы: обработчики его записи и проверки."},
		})},
		{Key: "data", Title: "Данные", Important: true, Rows: []PaletteRow{
			{Key: "dataLock", Title: "Режим управления блокировкой данных", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"automatic", "Автоматический"}, {"managed", "Управляемый"}},
				Description: "Как блокируется значение при записи: платформой сама или кодом."},
			{Key: "dataHistory", Title: "История данных", Field: ChoiceField, Important: true,
				Choices: [][2]string{{"dont-use", "Не использовать"}, {"use", "Использовать"}}, Description: "Хранятся ли версии значения константы."},
			{Key: "updateDataHistoryImmediatelyAfterWrite", Title: "Обновлять историю данных сразу после записи", Field: FlagField, Important: true,
				Needs: []string{"dataHistory=use"}, Description: "Версия строится сразу после записи, а не по расписанию."},
			{Key: "executeAfterDataHistoryVersionWriteProcessing", Title: "Выполнять обработку после записи версии истории данных", Field: FlagField,
				Important: true, Needs: []string{"dataHistory=use"}, Description: "Модуль менеджера узнаёт о каждой записанной версии."},
		}},
		{Key: "presentation", Title: "Представление", Rows: rows([]PaletteRow{
			{Key: "useStandardCommands", Title: "Использовать стандартные команды", Field: FlagField,
				Description: "Платформа сама даёт команду, открывающую константу."},
			{Key: "defaultForm", Title: "Основная форма", Field: FormField, Description: "Общая форма, которой открывается константа."},
		}, referenceChoice, []PaletteRow{
			{Field: SeparatorField},
			{Key: "extendedPresentation", Title: "Расширенное представление", Field: LocalizedField,
				Description: "Название константы там, где для него есть место: заголовок её формы."},
			{Key: "explanation", Title: "Пояснение", Field: LocalizedField, Lines: 3,
				Description: "Фраза, которой константа предлагается пользователю рядом с командой, открывающей её."},
			{Field: SeparatorField},
		}, typePresentation, []PaletteRow{
			{Field: SeparatorField},
			{Key: "fillChecking", Title: "Проверка заполнения", Field: ChoiceField, Default: "dont-check", Choices: fillChecks,
				Description: "Отказывает ли запись незаполненного значения."},
			{Field: SeparatorField},
			{Key: "choice.quickChoice", Title: "Быстрый выбор", Field: ChoiceField, Choices: autoUsage,
				Description: "Выбирается ли значение из выпадающего списка вместо формы выбора."},
			{Key: "choice.historyOnInput", Title: "История выбора при вводе", Field: ChoiceField, Choices: autoHistory,
				Description: "Предлагаются ли первыми значения, которые пользователь уже выбирал."},
			{Field: SeparatorField},
			{Key: "choice.linkByType", Title: "Связь по типу", Field: FieldsField,
				Description: "Значение, от которого зависит тип значения константы."},
		})},
	},
	// docs/block-00/rol.md, «Палитра» (snapshot rol/02-novaya-rol): one tab,
	// the name, the synonym, the comment and, apart, «Права: Открыть» - the
	// window of the role's rights.
	Role: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("роли", "о роли"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "rights", Title: "Права", Field: WindowField, Important: true,
				Description: "Окно прав роли: права на объекты проекта и ограничения доступа к данным."},
		})},
	},
	// docs/block-00/spravochnik.md, «Палитра свойств»: four tabs, «только
	// важные» takes «Справочная информация» away.
	Catalog: {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("справочника", "о справочнике"), []PaletteRow{
			{Field: modulesRow, Important: true},
		})},
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

// sessionPaletteRows are the rows of the sessions of a web and an HTTP
// service, the same in both palettes.
var sessionPaletteRows = []PaletteRow{
	{Key: "reuseSessions", Title: "Повторное использование сеансов", Field: ChoiceField, Important: true, Default: "do-not-use",
		Choices:     [][2]string{{"do-not-use", "Не использовать"}, {"use", "Использовать"}, {"auto", "Использовать автоматически"}},
		Description: "Отвечает ли сервис нескольким вызовам в одном сеансе."},
	{Key: "sessionMaxAge", Title: "Время жизни сеанса", Field: NumberField, Important: true,
		Description: "Сколько секунд сеанс может бездействовать, прежде чем завершится; действует при повторном использовании сеансов."},
}

// templatePalette is the palette of a template, common or of an object -
// the prototype's are the same (docs/block-00/maket.md): «Тип макета»
// unavailable once the template is made, «Макет: Открыть» opening the editor
// of its content.
var templatePalette = []PaletteTab{
	{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("макета", "о макете"), []PaletteRow{
		{Key: "kind", Title: "Тип макета", Field: ChoiceField, Important: true, Fixed: true, Choices: TemplateKinds(),
			Description: "Что хранит макет; выбирается в конструкторе макета и после не меняется."},
		{Field: SeparatorField, Important: true},
		{Key: "content", Title: "Макет", Field: WindowField, Important: true, Description: "Содержимое макета: открывается его редактором."},
	})},
}

// httpMethods are the HTTP methods of a method of an HTTP service, in the
// order of the prototype's list (http-servis.md, «Палитра»; snapshot
// http-servis/14), «Любой» last.
var httpMethods = [][2]string{
	{"GET", "GET"}, {"HEAD", "HEAD"}, {"PUT", "PUT"}, {"POST", "POST"}, {"DELETE", "DELETE"}, {"PATCH", "PATCH"}, {"MERGE", "MERGE"},
	{"OPTIONS", "OPTIONS"}, {"TRACE", "TRACE"}, {"CONNECT", "CONNECT"}, {"PROPFIND", "PROPFIND"}, {"PROPPATCH", "PROPPATCH"},
	{"MOVE", "MOVE"}, {"COPY", "COPY"}, {"LOCK", "LOCK"}, {"UNLOCK", "UNLOCK"}, {"MKCOL", "MKCOL"}, {"ANY", "Любой"},
}

// templateKinds are the kinds of a template with the prototype's captions,
// in the order the constructor of a template lists them (obshchij-maket.md,
// «Команды»; snapshot obshchij-maket/03-konstruktor).
var templateKinds = [][2]string{
	{"spreadsheet", "Табличный документ"}, {"text", "Текстовый документ"}, {"binary", "Двоичные данные"},
	{"active-document", "Active document"}, {"html", "HTML документ"}, {"geographical-schema", "Географическая схема"},
	{"graphical-schema", "Графическая схема"}, {"composition-schema", "Схема компоновки данных"},
	{"composition-appearance", "Макет оформления компоновки данных"}, {"add-in", "Внешняя компонента"},
}

// TemplateKinds are the kinds of a template and their captions, in the
// constructor's order.
func TemplateKinds() [][2]string { return slices.Clone(templateKinds) }

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

// typeRows are the type of a value and its qualifiers by the kind of the
// type (obshchee.md, «Тип значения и квалификаторы»): the rows of the type
// of an attribute and of a constant alike.
var typeRows = []PaletteRow{
	{Key: "types", Title: "Тип", Field: TypeField, Important: true, Description: "Какие значения хранит реквизит."},
	{Key: "type.length", Title: "Длина", Field: NumberField, Important: true, Shown: []string{"type=string"},
		Description: "Сколько знаков в строке; ноль — длина не ограничена."},
	{Key: "type.fixedLength", Title: "Допустимая длина", Field: SwitchField, Important: true, Shown: []string{"type=string"},
		Choices: [][2]string{{"false", "Переменная"}, {"true", "Фиксированная"}}, Description: "Строка хранится как набрана или дополняется пробелами до полной длины."},
	{Key: "type.unlimited", Title: "Неограниченная длина", Field: FlagField, Important: true, Shown: []string{"type=string"},
		Description: "Строка любой длины; такую строку нельзя индексировать и сравнивать в запросе."},
	{Key: "type.precision", Title: "Длина", Field: NumberField, Important: true, Shown: []string{"type=number"},
		Description: "Сколько цифр в числе, вместе с дробной частью."},
	{Key: "type.scale", Title: "Точность", Field: NumberField, Important: true, Shown: []string{"type=number"},
		Description: "Сколько цифр после запятой."},
	{Key: "type.nonNegative", Title: "Неотрицательное", Field: FlagField, Important: true, Shown: []string{"type=number"},
		Description: "Отрицательное число не записывается."},
	{Key: "type.dateParts", Title: "Состав даты", Field: ChoiceField, Important: true, Shown: []string{"type=date"}, Choices: dateParts,
		Description: "Что хранит дата: дату, время или дату и время."},
}

// typePresentation are the rows of the presentation of a value by the kind
// of its type, in one order for every kind of type, so a composite one shows
// the union of its kinds' rows once each (docs/block-00/obshchee.md,
// «Представление по типу»): the format of a number, a date or a boolean, the
// modes of a string, the tooltip of every type, the marks of a number.
var typePresentation = []PaletteRow{
	{Key: "presentation.format", Title: "Формат", Field: LocalizedField, Important: true, Shown: formatted,
		Description: "Как значение показывается в поле."},
	{Key: "presentation.editFormat", Title: "Формат редактирования", Field: LocalizedField, Important: true, Shown: formatted,
		Description: "Как значение показывается, пока его правят."},
	{Key: "presentation.password", Title: "Режим пароля", Field: FlagField, Important: true, Shown: []string{"type=string"},
		Description: "Набранное в поле показывается звёздочками."},
	{Key: "presentation.tooltip", Title: "Подсказка", Field: LocalizedField, Lines: 3, Important: true,
		Description: "Текст, который пользователь видит, наведя указатель на поле."},
	{Key: "presentation.mask", Title: "Маска", Field: TextField, Important: true, Shown: []string{"type=string"},
		Description: "Шаблон набора: какие знаки и где можно набрать."},
	{Key: "presentation.multiLine", Title: "Многострочный режим", Field: FlagField, Important: true, Shown: []string{"type=string"},
		Description: "Поле показывает текст в несколько строк."},
	{Key: "presentation.extendedEdit", Title: "Расширенное редактирование", Field: FlagField, Important: true, Shown: []string{"type=string"},
		Description: "Текст правится с оформлением."},
	{Key: "presentation.markNegatives", Title: "Выделять отрицательные", Field: FlagField, Important: true, Shown: []string{"type=number"},
		Description: "Отрицательное число в поле выделяется цветом."},
	{Key: "presentation.minValue", Title: "Минимальное значение", Field: TextField, Important: true, Shown: []string{"type=number"},
		Description: "Меньше этого значения в поле не набрать."},
	{Key: "presentation.maxValue", Title: "Максимальное значение", Field: TextField, Important: true, Shown: []string{"type=number"},
		Description: "Больше этого значения в поле не набрать."},
}

// identityRows are the name, the synonym and the comment every palette
// opens with, said of the thing it shows: of whom (genitive) and about whom.
func identityRows(genitive, about string) []PaletteRow {
	return []PaletteRow{
		{Key: "name", Title: "Имя", Field: NameField, Important: true, Description: "Имя " + genitive + " в коде и в запросах: одно слово, начинается с буквы."},
		{Key: "title", Title: "Синоним", Field: LocalizedField, Important: true, Description: "Название " + genitive + " для пользователя на каждом языке проекта."},
		{Key: "comment", Title: "Комментарий", Field: TextField, Important: true, Description: "Заметка разработчика " + about + "; пользователю не показывается."},
	}
}

// rows joins groups of rows in their order.
func rows(groups ...[]PaletteRow) []PaletteRow {
	var result []PaletteRow
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}

// usage are the values of a setting used, not used or left to the platform,
// with the prototype's captions.
var (
	usage = [][2]string{{"use", "Использовать"}, {"dont-use", "Не использовать"}}
	// xdtoNamespaceRow is the namespace of an XDTO package, one row in the
	// palette of the package and of the root of its tree.
	xdtoNamespaceRow = PaletteRow{Key: "namespace", Title: "URI пространства имен", Field: TextField, Important: true,
		Description: "Пространство имён, которому принадлежат типы пакета; им их называет другая сторона обмена."}
	// xdtoFlag is a flag of a node of an XDTO package, as its palette
	// names it - a list, not a box (snapshot series-2.584/143).
	xdtoFlag     = [][2]string{{"false", "Ложь"}, {"true", "Истина"}}
	autoUsage    = [][2]string{{"auto", "Авто"}, {"use", "Использовать"}, {"dont-use", "Не использовать"}}
	autoHistory  = [][2]string{{"auto", "Авто"}, {"dont-use", "Не использовать"}}
	attributeUse = [][2]string{{"for-item", "Для элемента"}, {"for-folder", "Для группы"}, {"for-folder-and-item", "Для группы и элемента"}}
	fillChecks   = [][2]string{{"dont-check", "Не проверять"}, {"show-error", "Выдавать ошибку"}}
	dateParts    = [][2]string{{"date", "Дата"}, {"time", "Время"}, {"date-time", "Дата и время"}}
	// formatted are the kinds of type a format is given to.
	formatted = []string{"type=number", "type=date", "type=boolean"}
	// referenceChoice are the rows of choosing a reference: what may be
	// chosen in a hierarchy, the windows of the links and parameters of the
	// choice and the form it is chosen in.
	referenceChoice = []PaletteRow{
		{Key: "choice.foldersAndItems", Title: "Выбор групп и элементов", Field: ChoiceField, Important: true, Shown: []string{"type=reference"},
			Choices:     [][2]string{{"items", "Элементы"}, {"folders", "Группы"}, {"folders-and-items", "Группы и элементы"}},
			Description: "Что можно выбрать в поле из иерархического справочника: группы, элементы или то и другое."},
		{Key: "choiceParameterLinks", Title: "Связи параметров выбора", Field: WindowField, Important: true, Shown: []string{"type=reference"},
			Description: "Отбор выбираемых значений по значениям других реквизитов."},
		{Key: "choiceParameters", Title: "Параметры выбора", Field: WindowField, Important: true, Shown: []string{"type=reference"},
			Description: "Постоянный отбор выбираемых значений."},
		{Key: "choiceForm", Title: "Форма выбора", Field: WindowField, Important: true, Shown: []string{"type=reference"},
			Description: "Форма, из которой выбирается значение."},
	}
)

// partPalettes are the palettes of the parts of an object - an attribute, a
// table part, a template - by the kind of the part (docs/block-00/rekvizit.md,
// tablichnaya-chast.md, «Палитра»; snapshots spravochnik/rekvizit-*). The
// rows of an attribute depend on its type: Shown names the kind of its one
// type a row stands at (type=string, type=number, type=reference), and
// owner=<kind> the kind of the object the part belongs to.
var partPalettes = map[string][]PaletteTab{
	"template": templatePalette,
	// docs/block-00/web-servis.md, «Палитра»; snapshots web-servis/09, 10,
	// 14, 15: an operation on two tabs, its lock of data on «Данные» of two
	// values alone; a parameter on one.
	"webServiceOperation": {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("операции", "об операции"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "returnType", Title: "Тип возвращаемого значения", Field: XMLTypeField, Important: true,
				Description: "Тип значения, которое возвращает операция."},
			{Key: "nillable", Title: "Возможно пустое значение", Field: FlagField, Important: true,
				Description: "Операция может вернуть неопределённое значение."},
			{Key: "transactioned", Title: "В транзакции", Field: FlagField, Important: true,
				Description: "Процедура операции выполняется в транзакции."},
			{Key: "procedure", Title: "Имя процедуры", Field: TextField, Important: true,
				Description: "Процедура модуля сервиса, которая выполняется при вызове операции."},
		})},
		{Key: "data", Title: "Данные", Important: true, Rows: []PaletteRow{
			{Key: "dataLockControl", Title: "Режим управления блокировкой данных", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"automatic", "Автоматический"}, {"managed", "Управляемый"}},
				Description: "Как блокируются данные, которые меняет операция: платформой сама или кодом."},
		}},
	},
	// docs/block-00/http-servis.md, «Палитра»; snapshots http-servis/09, 13,
	// 14: a template of one row beside its name, a method of the HTTP method
	// - seventeen verbs and «Любой», in the order of the prototype's list -
	// and the handler.
	"httpTemplate": {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("шаблона", "о шаблоне"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "template", Title: "Шаблон", Field: TextField, Important: true,
				Description: "Часть адреса после корневого URL сервиса: сегменты, параметры в фигурных скобках, «*» последним."},
		})},
	},
	"httpMethod": {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("метода", "о методе"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "method", Title: "HTTP-метод", Field: ChoiceField, Important: true, Choices: httpMethods,
				Description: "На какой HTTP-метод запроса к шаблону отвечает метод."},
			{Key: "handler", Title: "Обработчик", Field: TextField, Important: true,
				Description: "Функция модуля сервиса, которая обрабатывает запрос."},
		})},
	},
	"webServiceParameter": {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("параметра", "о параметре"), []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "type", Title: "Тип значения", Field: XMLTypeField, Important: true, Description: "Тип значения параметра."},
			{Key: "nillable", Title: "Возможно пустое значение", Field: FlagField, Important: true,
				Description: "Параметр может получить неопределённое значение."},
			{Key: "direction", Title: "Направление передачи", Field: ChoiceField, Important: true, Default: "in",
				Choices:     [][2]string{{"in", "Входной"}, {"out", "Выходной"}, {"in-out", "Входной - Выходной"}},
				Description: "Передаётся ли значение параметра в операцию, из неё или в обе стороны."},
		})},
	},
	// docs/block-00/paket-xdto.md, «Окно — редактор пакета», «Узлы
	// редактора»; snapshots paket-xdto/03, series-2.584/141-170: one tab
	// «Другие» a node, every value but a name of a namespace said by ✓.
	"xdtoPackage": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			xdtoNamespaceRow,
			{Key: "elementFormQualified", Title: "Квалифицированная форма элемента", Field: SwitchField, Important: true, Optional: true,
				Choices: xdtoFlag, Default: "false", Description: "Пишутся ли элементы свойств пакета с пространством имён."},
			{Key: "attributeFormQualified", Title: "Квалифицированная форма атрибута", Field: SwitchField, Important: true, Optional: true,
				Choices: xdtoFlag, Default: "false", Description: "Пишутся ли атрибуты свойств пакета с пространством имён."},
		}},
	},
	"xdtoImport": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "namespace", Title: "Пространство имен", Field: TextField, Important: true,
				Description: "Пространство имён другого пакета, типы которого использует этот."},
		}},
	},
	"xdtoProperty": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "name", Title: "Имя", Field: TextField, Important: true, Optional: true, Description: "Имя свойства."},
			{Key: "ref", Title: "Ссылка", Field: XMLTypeField, Important: true, Optional: true,
				Description: "Глобальное свойство, которое это свойство повторяет."},
			{Key: "type", Title: "Тип", Field: XMLTypeField, Important: true, Optional: true, Description: "Тип значения свойства."},
			{Key: "lowerBound", Title: "Минимальное количество", Field: NumberField, Important: true, Optional: true, Default: "1", Min: 0,
				Description: "Сколько значений свойства самое меньшее."},
			{Key: "upperBound", Title: "Максимальное количество", Field: NumberField, Important: true, Optional: true, Default: "1", Min: -1,
				Description: "Сколько значений свойства самое большее; -1 — без ограничения."},
			{Key: "nillable", Title: "Возможно пустое", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag, Default: "false",
				Description: "Может ли значение свойства быть пустым."},
			{Key: "fixed", Title: "Фиксированное", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag, Default: "false",
				Description: "Всегда ли значение свойства равно значению по умолчанию."},
			{Key: "default", Title: "По умолчанию", Field: TextField, Important: true, Optional: true, Description: "Значение свойства по умолчанию."},
			{Key: "form", Title: "Форма", Field: ChoiceField, Important: true, Optional: true, Default: "Element",
				Choices:     [][2]string{{"Attribute", "Атрибут"}, {"Element", "Элемент"}, {"Text", "Текст"}},
				Description: "Чем свойство пишется в XML: атрибутом, элементом или текстом."},
			{Key: "localName", Title: "Локальное имя", Field: TextField, Important: true, Optional: true,
				Description: "Имя свойства в XML, когда оно не то, что «Имя»."},
			{Key: "qualified", Title: "Квалифицированное", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag,
				Description: "Пишется ли свойство в XML с пространством имён."},
		}},
	},
	"xdtoValueType": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "name", Title: "Имя", Field: TextField, Important: true, Optional: true, Description: "Имя типа значения."},
			{Key: "base", Title: "Базовый тип", Field: XMLTypeField, Important: true, Optional: true, Description: "Тип, который этот тип сужает."},
			{Key: "variety", Title: "Вариант", Field: ChoiceField, Important: true, Optional: true, Default: "Atomic",
				Choices:     [][2]string{{"Atomic", "Атомарный"}, {"List", "Список"}, {"Union", "Объединение"}},
				Description: "Одно значение, список значений или одно из значений нескольких типов."},
			{Key: "itemType", Title: "Тип элемента", Field: XMLTypeField, Important: true, Optional: true, Description: "Тип значений списка."},
			{Key: "memberTypes", Title: "Типы объединения", Field: XMLTypesField, Important: true, Optional: true,
				Description: "Типы, значения которых принимает объединение."},
			{Key: "length", Title: "Длина", Field: NumberField, Important: true, Optional: true, Default: "0", Description: "Длина значения."},
			{Key: "minLength", Title: "Минимальная длина", Field: NumberField, Important: true, Optional: true, Default: "0",
				Description: "Самая меньшая длина значения."},
			{Key: "maxLength", Title: "Максимальная длина", Field: NumberField, Important: true, Optional: true, Default: "0",
				Description: "Самая большая длина значения."},
			{Key: "whiteSpace", Title: "Пробельные символы", Field: ChoiceField, Important: true, Optional: true, Default: "preserve",
				Choices:     [][2]string{{"preserve", "preserve"}, {"replace", "replace"}, {"collapse", "collapse"}},
				Description: "Сохраняются ли пробелы, переводы строк и табуляции значения."},
			{Key: "minInclusive", Title: "Минимум включающий границу", Field: TextField, Important: true, Optional: true,
				Description: "Самое меньшее значение, оно само допустимо."},
			{Key: "minExclusive", Title: "Минимум не включающий границу", Field: TextField, Important: true, Optional: true,
				Description: "Граница снизу, само значение недопустимо."},
			{Key: "maxInclusive", Title: "Максимум включающий границу", Field: TextField, Important: true, Optional: true,
				Description: "Самое большее значение, оно само допустимо."},
			{Key: "maxExclusive", Title: "Максимум не включающий границу", Field: TextField, Important: true, Optional: true,
				Description: "Граница сверху, само значение недопустимо."},
			{Key: "totalDigits", Title: "Общее количество цифр", Field: NumberField, Important: true, Optional: true, Default: "0",
				Description: "Сколько цифр в числе самое большее."},
			{Key: "fractionDigits", Title: "Количество цифр дробной части", Field: NumberField, Important: true, Optional: true, Default: "0",
				Description: "Сколько цифр после запятой самое большее."},
		}},
	},
	"xdtoObjectType": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "name", Title: "Имя", Field: TextField, Important: true, Optional: true, Description: "Имя типа объекта."},
			{Key: "base", Title: "Базовый тип", Field: XMLTypeField, Important: true, Optional: true,
				Description: "Тип объекта, свойства которого этот тип наследует."},
			{Key: "open", Title: "Открытый", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag, Default: "false",
				Description: "Может ли объект нести свойства, не описанные в типе."},
			{Key: "abstract", Title: "Абстрактный", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag, Default: "false",
				Description: "Только основа других типов: объекта этого типа не бывает."},
			{Key: "mixed", Title: "Смешанный", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag, Default: "false",
				Description: "Может ли между свойствами объекта стоять текст."},
			{Key: "ordered", Title: "Упорядоченный", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag, Default: "true",
				Description: "Пишутся ли свойства объекта в порядке типа."},
			{Key: "sequenced", Title: "Последовательный", Field: SwitchField, Important: true, Optional: true, Choices: xdtoFlag, Default: "false",
				Description: "Хранит ли объект порядок своих свойств и текста."},
		}},
	},
	"xdtoPattern": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "value", Title: "Образец", Field: TextField, Important: true, Description: "Регулярное выражение, которому отвечает значение."},
		}},
	},
	"xdtoEnumeration": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "value", Title: "Перечисление", Field: TextField, Important: true, Description: "Одно из допустимых значений типа."},
		}},
	},
	// docs/block-00/ws-ssylka.md, «Окно»; snapshots ws-ssylka/04-09,
	// series-2.584/236-239: the nodes of «Web-сервисы» of the window viewing
	// a description, read only, each on one tab.
	"wsRoot": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "documentation", Title: "Описание определения Web-сервисов", Field: TextField, Important: true,
				Description: "Описание определения сервисов из описания WSDL."},
		}},
	},
	"wsService": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "namespace", Title: "Пространство имен", Field: TextField, Important: true, Description: "Пространство имён описания сервиса."},
			{Key: "name", Title: "Имя", Field: TextField, Important: true, Description: "Имя сервиса в описании."},
			{Key: "documentation", Title: "Описание Web-сервиса", Field: TextField, Important: true, Description: "Описание сервиса из описания WSDL."},
		}},
	},
	"wsPort": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "name", Title: "Имя", Field: TextField, Important: true, Description: "Имя точки подключения в описании."},
			{Key: "documentation", Title: "Описание порта", Field: TextField, Important: true, Description: "Описание точки подключения из описания WSDL."},
		}},
	},
	"wsOperation": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "name", Title: "Имя", Field: TextField, Important: true, Description: "Имя операции в описании."},
			{Key: "documentation", Title: "Описание операции", Field: TextField, Important: true, Description: "Описание операции из описания WSDL."},
			{Key: "returnType", Title: "Тип возвращаемого значения", Field: XMLTypeField, Important: true, Description: "Тип значения, которое возвращает операция."},
			{Key: "nillable", Title: "Возможно пустое", Field: SwitchField, Important: true, Choices: xdtoFlag, Description: "Может ли операция вернуть пустое значение."},
			{Key: "returnDocumentation", Title: "Описание возвращаемого значения", Field: TextField, Important: true,
				Description: "Описание возвращаемого значения из описания WSDL."},
		}},
	},
	"wsParameter": {
		{Key: "other", Title: "Другие", Important: true, Rows: []PaletteRow{
			{Key: "name", Title: "Имя", Field: TextField, Important: true, Description: "Имя параметра в описании."},
			{Key: "type", Title: "Тип", Field: XMLTypeField, Important: true, Description: "Тип значения параметра."},
			{Key: "direction", Title: "Направление", Field: ChoiceField, Important: true,
				Choices:     [][2]string{{"in", "Входной"}, {"out", "Выходной"}, {"in-out", "ВходнойВыходной"}},
				Description: "Передаётся ли значение параметра в операцию, из неё или в обе стороны."},
			{Key: "nillable", Title: "Возможно пустое", Field: SwitchField, Important: true, Choices: xdtoFlag, Description: "Может ли значение параметра быть пустым."},
			{Key: "documentation", Title: "Описание параметра операции", Field: TextField, Important: true,
				Description: "Описание параметра из описания WSDL."},
		}},
	},
	"attribute": {
		{Key: "main", Title: "Основные", Important: true, Rows: rows(identityRows("реквизита", "о реквизите"), []PaletteRow{
			{Field: SeparatorField, Important: true},
		}, typeRows)},
		{Key: "use", Title: "Использование", Important: true, Rows: []PaletteRow{
			{Key: "use", Title: "Использование", Field: ChoiceField, Important: true, Shown: []string{"owner=catalogs"}, Choices: attributeUse,
				Description: "Чей это реквизит: элемента, группы или обоих."},
			{Key: "indexing", Title: "Индексировать", Field: ChoiceField, Important: true, Default: "dont-index",
				Choices:     [][2]string{{"dont-index", "Не индексировать"}, {"index", "Индексировать"}, {"index-with-additional-order", "Индексировать с доп. упорядочиванием"}},
				Description: "Строит ли база индекс по реквизиту: отбор и упорядочивание по нему быстрее."},
			{Key: "fullTextSearch", Title: "Полнотекстовый поиск", Field: ChoiceField, Important: true, Choices: usage,
				Description: "Входит ли реквизит в полнотекстовый индекс."},
			{Key: "dataHistory", Title: "История данных", Field: ChoiceField, Important: true, Choices: usage,
				Description: "Хранятся ли изменения реквизита в истории данных."},
		}},
		{Key: "presentation", Title: "Представление", Important: true, Rows: rows(typePresentation, referenceChoice, []PaletteRow{
			{Field: SeparatorField, Important: true},
			{Key: "filling.fromFillingValue", Title: "Заполнять из данных заполнения", Field: FlagField, Important: true,
				Description: "Новый объект берёт значение реквизита из данных заполнения - например, из отбора списка."},
			{Key: "filling.value", Title: "Значение заполнения", Field: ValueField, Important: true,
				Description: "Значение реквизита у нового объекта."},
			{Key: "fillChecking", Title: "Проверка заполнения", Field: ChoiceField, Important: true, Default: "dont-check", Choices: fillChecks,
				Description: "Отказывает ли запись объекта с незаполненным реквизитом."},
			{Field: SeparatorField, Important: true},
			{Key: "choice.quickChoice", Title: "Быстрый выбор", Field: ChoiceField, Important: true, Choices: autoUsage,
				Description: "Выбирается ли значение из выпадающего списка вместо формы выбора."},
			{Key: "choice.createOnInput", Title: "Создание при вводе", Field: ChoiceField, Important: true, Choices: autoUsage,
				Description: "Можно ли создать значение прямо из поля."},
			{Key: "choice.historyOnInput", Title: "История выбора при вводе", Field: ChoiceField, Important: true, Choices: autoHistory,
				Description: "Предлагаются ли первыми значения, которые пользователь уже выбирал."},
			{Field: SeparatorField, Important: true},
			{Key: "choice.linkByType", Title: "Связь по типу", Field: FieldsField, Important: true,
				Description: "Реквизит, от значения которого зависит тип значения этого реквизита."},
		})},
	},
	"tablePart": {
		{Key: "main", Title: "Основные", Important: true, Rows: identityRows("табличной части", "о табличной части")},
		{Key: "data", Title: "Данные", Important: true, Rows: []PaletteRow{
			{Key: "standardAttributes", Title: "Стандартные реквизиты", Field: WindowField, Important: true,
				Description: "Окно стандартного реквизита табличной части — номера строки."},
			{Key: "lineNumberLength", Title: "Длина номера строки", Field: NumberField, Important: true, Min: 5, Max: 9,
				Description: "Сколько цифр в номере строки табличной части: от 5 до 9."},
		}},
		{Key: "use", Title: "Использование", Important: true, Rows: []PaletteRow{
			{Key: "use", Title: "Использование", Field: ChoiceField, Important: true, Shown: []string{"owner=catalogs"}, Choices: attributeUse,
				Description: "Чья это табличная часть: элемента, группы или обоих."},
		}},
		{Key: "presentation", Title: "Представление", Important: true, Rows: []PaletteRow{
			{Key: "tooltip", Title: "Подсказка", Field: LocalizedField, Lines: 3, Important: true,
				Description: "Текст, который пользователь видит, наведя указатель на табличную часть."},
			{Key: "fillChecking", Title: "Проверка заполнения", Field: ChoiceField, Important: true, Default: "dont-check", Choices: fillChecks,
				Description: "Отказывает ли запись объекта с пустой табличной частью."},
		}},
	},
}

// PartPalette is the palette of a part of an object - "attribute",
// "tablePart" - its tabs and rows, each row standing by what it is shown
// for.
func PartPalette(part string) []PaletteTab {
	return slices.Clone(partPalettes[part])
}
