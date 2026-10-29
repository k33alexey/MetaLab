// Package kinds lists the kinds of metadata object and what each is called.
//
// The prototype has one class of metadata object per kind, and the class
// carries the kind's names to every layer at once: the full name of an object
// (Справочник.Банки), the source of a query, the collection of Метаданные and
// the manager of the global context (Справочники), the tag and the folder of
// a configurator export (Catalog, Catalogs). Nothing beside the class keeps a
// word list of its own. This package is that class for the words: it depends
// on nothing, so the compiler and the project layout read it as well as the
// model. Where the objects of a kind lie in the project, and which modules and
// forms each keeps, is said here too (Layout): the project layout and Studio
// read it. What a kind does - how it is read, its rights, its registration -
// the metadata package adds to the same kind.
package kinds

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind identifies a kind of metadata object. It is the name of the folder its
// objects are kept in.
type Kind string

const (
	Role                       Kind = "roles"
	Subsystem                  Kind = "subsystems"
	Constant                   Kind = "constants"
	SessionParameter           Kind = "session-parameters"
	CommonAttribute            Kind = "common-attributes"
	CommonModule               Kind = "common-modules"
	EventSubscription          Kind = "event-subscriptions"
	Enumeration                Kind = "enumerations"
	ScheduledJob               Kind = "scheduled-jobs"
	CommandGroup               Kind = "command-groups"
	CommonCommand              Kind = "common-commands"
	CommonForm                 Kind = "common-forms"
	CommonTemplate             Kind = "common-templates"
	XDTOPackage                Kind = "xdto-packages"
	WebService                 Kind = "web-services"
	WSReference                Kind = "ws-references"
	WebSocketClient            Kind = "websocket-clients"
	ExternalDataSource         Kind = "external-data-sources"
	ExternalDataSourceTable    Kind = "external-data-source-tables"
	ExternalCube               Kind = "external-data-source-cubes"
	ExternalDimensionTable     Kind = "external-data-source-dimension-tables"
	ExternalFunction           Kind = "external-data-source-functions"
	HTTPService                Kind = "http-services"
	CommonPicture              Kind = "common-pictures"
	StyleItem                  Kind = "style-items"
	Style                      Kind = "styles"
	FunctionalOption           Kind = "functional-options"
	FunctionalOptionParameter  Kind = "functional-options-parameters"
	SettingsStorage            Kind = "settings-storages"
	FilterCriterion            Kind = "filter-criteria"
	DefinedType                Kind = "defined-types"
	Language                   Kind = "languages"
	Catalog                    Kind = "catalogs"
	Document                   Kind = "documents"
	DocumentJournal            Kind = "document-journals"
	Numerator                  Kind = "document-numerators"
	Sequence                   Kind = "sequences"
	Report                     Kind = "reports"
	DataProcessor              Kind = "data-processors"
	ChartOfCharacteristicTypes Kind = "charts-of-characteristic-types"
	ChartOfAccounts            Kind = "charts-of-accounts"
	ChartOfCalculationTypes    Kind = "charts-of-calculation-types"
	InformationRegister        Kind = "information-registers"
	AccumulationRegister       Kind = "accumulation-registers"
	AccountingRegister         Kind = "accounting-registers"
	CalculationRegister        Kind = "calculation-registers"
	Recalculation              Kind = "recalculations"
	BusinessProcess            Kind = "business-processes"
	Task                       Kind = "tasks"
	ExchangePlan               Kind = "exchange-plans"
)

// Words is a name in the two languages code is written in.
type Words struct {
	Russian string `json:"russian"`
	English string `json:"english"`
}

// Is reports whether a word of code is this name in either language. Case
// does not matter, as it does not anywhere in the prototype's language.
func (words Words) Is(word string) bool {
	return strings.EqualFold(word, words.Russian) || strings.EqualFold(word, words.English)
}

// Descriptor is what a kind is called.
type Descriptor struct {
	Kind Kind
	// Name names one object of the kind: the first part of its full name and
	// of a source of a query, the class of its metadata object, the tag of
	// its export (help: ОбъектМетаданных: Справочник, ПолноеИмя).
	Name Words
	// Collection names all objects of the kind: the collection of Метаданные
	// or of the object that owns them, the manager of the global context
	// where the kind has one, the folder of the export.
	Collection Words
	// Genitive is what the modules and forms of an object of the kind are
	// named after: МодульОбъектаСправочника, МодульФормыСправочника. The
	// prototype names a module by the object instead (Справочник.Банки.
	// МодульОбъекта); the names are the platform's own, and a subordinate
	// object's says whose it is.
	Genitive string
	// Title and Titles are the captions of one object and of the kind as a
	// whole in the interface.
	Title, Titles string
	// Icon names the picture of the kind in the icon set (internal/icons):
	// its objects and its branch of the tree are drawn with it, as the
	// prototype draws them with one.
	Icon string
}

// descriptors is every kind, in the order of the configuration tree (see
// Branches); a kind whose objects lie inside another's stands after its owner.
// The names are the help's (8.3.27: the classes of metadata objects and the
// collections of ОбъектМетаданныхКонфигурация, ВнешнийИсточникДанных, Куб,
// РегистрРасчета) and the export's tags and folders; Russian names are
// written without ё, as the help writes them.
var descriptors = []Descriptor{
	{Subsystem, Words{"Подсистема", "Subsystem"}, Words{"Подсистемы", "Subsystems"}, "Подсистемы", "Подсистема", "Подсистемы", "subsystem"},
	{CommonModule, Words{"ОбщийМодуль", "CommonModule"}, Words{"ОбщиеМодули", "CommonModules"}, "ОбщегоМодуля", "Общий модуль", "Общие модули", "common-module"},
	{SessionParameter, Words{"ПараметрСеанса", "SessionParameter"}, Words{"ПараметрыСеанса", "SessionParameters"}, "ПараметраСеанса", "Параметр сеанса", "Параметры сеанса", "session-parameter"},
	{Role, Words{"Роль", "Role"}, Words{"Роли", "Roles"}, "Роли", "Роль", "Роли", "role"},
	{CommonAttribute, Words{"ОбщийРеквизит", "CommonAttribute"}, Words{"ОбщиеРеквизиты", "CommonAttributes"}, "ОбщегоРеквизита", "Общий реквизит", "Общие реквизиты", "common-attribute"},
	{ExchangePlan, Words{"ПланОбмена", "ExchangePlan"}, Words{"ПланыОбмена", "ExchangePlans"}, "ПланаОбмена", "План обмена", "Планы обмена", "exchange-plan"},
	{FilterCriterion, Words{"КритерийОтбора", "FilterCriterion"}, Words{"КритерииОтбора", "FilterCriteria"}, "КритерияОтбора", "Критерий отбора", "Критерии отбора", "filter-criterion"},
	{EventSubscription, Words{"ПодпискаНаСобытие", "EventSubscription"}, Words{"ПодпискиНаСобытия", "EventSubscriptions"}, "ПодпискиНаСобытие", "Подписка на событие", "Подписки на события", "event-subscription"},
	{ScheduledJob, Words{"РегламентноеЗадание", "ScheduledJob"}, Words{"РегламентныеЗадания", "ScheduledJobs"}, "РегламентногоЗадания", "Регламентное задание", "Регламентные задания", "scheduled-job"},
	{FunctionalOption, Words{"ФункциональнаяОпция", "FunctionalOption"}, Words{"ФункциональныеОпции", "FunctionalOptions"}, "ФункциональнойОпции", "Функциональная опция", "Функциональные опции", "functional-option"},
	{FunctionalOptionParameter, Words{"ПараметрФункциональныхОпций", "FunctionalOptionsParameter"}, Words{"ПараметрыФункциональныхОпций", "FunctionalOptionsParameters"},
		"ПараметраФункциональныхОпций", "Параметр функциональных опций", "Параметры функциональных опций", "functional-options-parameter"},
	{DefinedType, Words{"ОпределяемыйТип", "DefinedType"}, Words{"ОпределяемыеТипы", "DefinedTypes"}, "ОпределяемогоТипа", "Определяемый тип", "Определяемые типы", "defined-type"},
	{SettingsStorage, Words{"ХранилищеНастроек", "SettingsStorage"}, Words{"ХранилищаНастроек", "SettingsStorages"}, "ХранилищаНастроек", "Хранилище настроек", "Хранилища настроек", "settings-storage"},
	{CommonCommand, Words{"ОбщаяКоманда", "CommonCommand"}, Words{"ОбщиеКоманды", "CommonCommands"}, "ОбщейКоманды", "Общая команда", "Общие команды", "command"},
	{CommandGroup, Words{"ГруппаКоманд", "CommandGroup"}, Words{"ГруппыКоманд", "CommandGroups"}, "ГруппыКоманд", "Группа команд", "Группы команд", "command-group"},
	{CommonForm, Words{"ОбщаяФорма", "CommonForm"}, Words{"ОбщиеФормы", "CommonForms"}, "ОбщейФормы", "Общая форма", "Общие формы", "form"},
	{CommonTemplate, Words{"ОбщийМакет", "CommonTemplate"}, Words{"ОбщиеМакеты", "CommonTemplates"}, "ОбщегоМакета", "Общий макет", "Общие макеты", "template"},
	{CommonPicture, Words{"ОбщаяКартинка", "CommonPicture"}, Words{"ОбщиеКартинки", "CommonPictures"}, "ОбщейКартинки", "Общая картинка", "Общие картинки", "picture"},
	{XDTOPackage, Words{"ПакетXDTO", "XDTOPackage"}, Words{"ПакетыXDTO", "XDTOPackages"}, "ПакетаXDTO", "XDTO-пакет", "XDTO-пакеты", "xdto-package"},
	{WebService, Words{"WebСервис", "WebService"}, Words{"WebСервисы", "WebServices"}, "WebСервиса", "Web-сервис", "Web-сервисы", "web-service"},
	{HTTPService, Words{"HTTPСервис", "HTTPService"}, Words{"HTTPСервисы", "HTTPServices"}, "HTTPСервиса", "HTTP-сервис", "HTTP-сервисы", "http-service"},
	{WSReference, Words{"WSСсылка", "WSReference"}, Words{"WSСсылки", "WSReferences"}, "WSСсылки", "WS-ссылка", "WS-ссылки", "ws-reference"},
	{WebSocketClient, Words{"WebSocketКлиент", "WebSocketClient"}, Words{"WebSocketКлиенты", "WebSocketClients"}, "WebSocketКлиента", "WebSocket-клиент", "WebSocket-клиенты", "websocket-client"},
	{StyleItem, Words{"ЭлементСтиля", "StyleItem"}, Words{"ЭлементыСтиля", "StyleItems"}, "ЭлементаСтиля", "Элемент стиля", "Элементы стиля", "style-item"},
	{Style, Words{"Стиль", "Style"}, Words{"Стили", "Styles"}, "Стиля", "Стиль", "Стили", "style"},
	{Language, Words{"Язык", "Language"}, Words{"Языки", "Languages"}, "Языка", "Язык", "Языки", "language"},
	{Constant, Words{"Константа", "Constant"}, Words{"Константы", "Constants"}, "Константы", "Константа", "Константы", "constant"},
	{Catalog, Words{"Справочник", "Catalog"}, Words{"Справочники", "Catalogs"}, "Справочника", "Справочник", "Справочники", "catalog"},
	{Document, Words{"Документ", "Document"}, Words{"Документы", "Documents"}, "Документа", "Документ", "Документы", "document"},
	{Numerator, Words{"Нумератор", "DocumentNumerator"}, Words{"НумераторыДокументов", "DocumentNumerators"}, "Нумератора", "Нумератор", "Нумераторы", "numerator"},
	{Sequence, Words{"Последовательность", "Sequence"}, Words{"Последовательности", "Sequences"}, "Последовательности", "Последовательность", "Последовательности", "sequence"},
	{DocumentJournal, Words{"ЖурналДокументов", "DocumentJournal"}, Words{"ЖурналыДокументов", "DocumentJournals"}, "ЖурналаДокументов", "Журнал документов", "Журналы документов", "document-journal"},
	{Enumeration, Words{"Перечисление", "Enum"}, Words{"Перечисления", "Enums"}, "Перечисления", "Перечисление", "Перечисления", "enumeration"},
	{Report, Words{"Отчет", "Report"}, Words{"Отчеты", "Reports"}, "Отчета", "Отчёт", "Отчёты", "report"},
	{DataProcessor, Words{"Обработка", "DataProcessor"}, Words{"Обработки", "DataProcessors"}, "Обработки", "Обработка", "Обработки", "data-processor"},
	{ChartOfCharacteristicTypes, Words{"ПланВидовХарактеристик", "ChartOfCharacteristicTypes"}, Words{"ПланыВидовХарактеристик", "ChartsOfCharacteristicTypes"},
		"ПланаВидовХарактеристик", "План видов характеристик", "Планы видов характеристик", "chart-of-characteristic-types"},
	{ChartOfAccounts, Words{"ПланСчетов", "ChartOfAccounts"}, Words{"ПланыСчетов", "ChartsOfAccounts"}, "ПланаСчетов", "План счетов", "Планы счетов", "chart-of-accounts"},
	{ChartOfCalculationTypes, Words{"ПланВидовРасчета", "ChartOfCalculationTypes"}, Words{"ПланыВидовРасчета", "ChartsOfCalculationTypes"},
		"ПланаВидовРасчета", "План видов расчёта", "Планы видов расчёта", "chart-of-calculation-types"},
	{InformationRegister, Words{"РегистрСведений", "InformationRegister"}, Words{"РегистрыСведений", "InformationRegisters"}, "РегистраСведений", "Регистр сведений", "Регистры сведений", "information-register"},
	{AccumulationRegister, Words{"РегистрНакопления", "AccumulationRegister"}, Words{"РегистрыНакопления", "AccumulationRegisters"}, "РегистраНакопления", "Регистр накопления", "Регистры накопления", "accumulation-register"},
	{AccountingRegister, Words{"РегистрБухгалтерии", "AccountingRegister"}, Words{"РегистрыБухгалтерии", "AccountingRegisters"}, "РегистраБухгалтерии", "Регистр бухгалтерии", "Регистры бухгалтерии", "accounting-register"},
	{CalculationRegister, Words{"РегистрРасчета", "CalculationRegister"}, Words{"РегистрыРасчета", "CalculationRegisters"}, "РегистраРасчета", "Регистр расчёта", "Регистры расчёта", "calculation-register"},
	{Recalculation, Words{"Перерасчет", "Recalculation"}, Words{"Перерасчеты", "Recalculations"}, "Перерасчета", "Перерасчёт", "Перерасчёты", "recalculation"},
	{BusinessProcess, Words{"БизнесПроцесс", "BusinessProcess"}, Words{"БизнесПроцессы", "BusinessProcesses"}, "БизнесПроцесса", "Бизнес-процесс", "Бизнес-процессы", "business-process"},
	{Task, Words{"Задача", "Task"}, Words{"Задачи", "Tasks"}, "Задачи", "Задача", "Задачи", "task"},
	{ExternalDataSource, Words{"ВнешнийИсточникДанных", "ExternalDataSource"}, Words{"ВнешниеИсточникиДанных", "ExternalDataSources"},
		"ВнешнегоИсточникаДанных", "Внешний источник данных", "Внешние источники данных", "external-data-source"},
	{ExternalDataSourceTable, Words{"Таблица", "Table"}, Words{"Таблицы", "Tables"}, "ТаблицыВнешнегоИсточника", "Таблица", "Таблицы", "external-table"},
	{ExternalCube, Words{"Куб", "Cube"}, Words{"Кубы", "Cubes"}, "КубаВнешнегоИсточника", "Куб", "Кубы", "external-cube"},
	{ExternalDimensionTable, Words{"ТаблицаИзмерения", "DimensionTable"}, Words{"ТаблицыИзмерений", "DimensionTables"},
		"ТаблицыИзмеренияВнешнегоИсточника", "Таблица измерения", "Таблицы измерений", "external-dimension-table"},
	{ExternalFunction, Words{"Функция", "Function"}, Words{"Функции", "Functions"}, "ФункцииВнешнегоИсточника", "Функция", "Функции", "external-function"},
}

// references are the names of the reference types, kept beside descriptors
// rather than in each line of it: eleven kinds of fifty have them, twelve
// types in all. The help (8.3.27) has a page of each and of no other.
var references = map[Kind][]Words{
	Catalog:                    {{"СправочникСсылка", "CatalogRef"}},
	Document:                   {{"ДокументСсылка", "DocumentRef"}},
	Enumeration:                {{"ПеречислениеСсылка", "EnumRef"}},
	ChartOfCharacteristicTypes: {{"ПланВидовХарактеристикСсылка", "ChartOfCharacteristicTypesRef"}},
	ChartOfAccounts:            {{"ПланСчетовСсылка", "ChartOfAccountsRef"}},
	ChartOfCalculationTypes:    {{"ПланВидовРасчетаСсылка", "ChartOfCalculationTypesRef"}},
	BusinessProcess:            {{"БизнесПроцессСсылка", "BusinessProcessRef"}, {"ТочкаМаршрутаБизнесПроцессаСсылка", "BusinessProcessRoutePointRef"}},
	Task:                       {{"ЗадачаСсылка", "TaskRef"}},
	ExchangePlan:               {{"ПланОбменаСсылка", "ExchangePlanRef"}},
	ExternalDataSourceTable:    {{"ВнешнийИсточникДанныхТаблицаСсылка", "ExternalDataSourceTableRef"}},
	ExternalDimensionTable:     {{"ВнешнийИсточникДанныхКубТаблицаИзмеренияСсылка", "ExternalDataSourceCubeDimensionTableRef"}},
}

// newNames are the names «Добавить» gives a new object of a kind where it is
// not the kind's word for one object numbered: a template is «Макет», as
// the constructor of a template offers it (docs/block-00/obshchij-maket.md,
// maket.md).
var newNames = map[Kind]string{CommonTemplate: "Макет"}

// NewName is the name «Добавить» gives a new object of the kind by itself,
// and false for a kind whose new object is its word numbered.
func (descriptor Descriptor) NewName() (string, bool) {
	name, ok := newNames[descriptor.Kind]
	return name, ok
}

// memberWords are the words a caption of the window of an object names one
// of its kind by where it is not the kind's word for one object in small
// letters: «Подсистемы, в которых участвует вид расчета» of a chart of
// calculation types (docs/block-00/obshchee.md, «Окно объекта», snapshots
// configurator-2026-10-27/series-2.584/).
var memberWords = map[Kind]string{ChartOfCalculationTypes: "вид расчета"}

// MemberWord is how the captions «Подсистемы, в которых участвует …» and
// «Функциональные опции, в которых участвует …» name an object of the kind:
// its word in small letters, but a word of Latin letters as it is written -
// the prototype writes «Web-сервис», «HTTP-сервис» there.
func (descriptor Descriptor) MemberWord() string {
	if word, ok := memberWords[descriptor.Kind]; ok {
		return word
	}
	first, size := utf8.DecodeRuneInString(descriptor.Title)
	if first < unicode.MaxASCII {
		return descriptor.Title
	}
	return string(unicode.ToLower(first)) + descriptor.Title[size:]
}

// rootAddOrder is the submenu «Добавить ▸» of the menu of the root, in the
// prototype's order (docs/block-00/konfiguraciya.md, «Команды»; snapshot
// configurator-2026-10-27/series-2.595/35) - not the order of the tree:
// the common kinds from «Язык» to «Определяемый тип». The prototype's
// «Бот» is not carried (owner, 30.09.2026); a common form, a common
// command, a group of commands, a style and the applied kinds it does not
// offer there.
var rootAddOrder = []Kind{
	Language, Subsystem, StyleItem, CommonPicture, SessionParameter, Role, CommonTemplate, FilterCriterion, CommonModule,
	CommonAttribute, ExchangePlan, XDTOPackage, WebService, HTTPService, WSReference, WebSocketClient, EventSubscription,
	ScheduledJob, SettingsStorage, FunctionalOption, FunctionalOptionParameter, DefinedType,
}

// RootAddKinds are the kinds «Добавить ▸» of the menu of the root offers, in
// its order (rootAddOrder).
func RootAddKinds() []Kind {
	return slices.Clone(rootAddOrder)
}

// referenceOrder is the order the prototype's window «Редактирование типа
// данных» lists the reference types of the configuration in - not the order
// of the tree, where an exchange plan stands among the common objects and the
// charts after the reports (docs/block-00/obshchee.md, «Окно «Редактирование
// типа данных»», 23.10.2026). A business process gives two, its own and its
// route point's, in the order References names them.
var referenceOrder = []Kind{
	Catalog, Document, Enumeration, ChartOfCharacteristicTypes, ChartOfAccounts, ChartOfCalculationTypes,
	BusinessProcess, Task, ExchangePlan,
}

// ReferenceKinds are the kinds whose references a type is chosen among, in the
// order the type editor lists them (referenceOrder). The tables of an external
// source have references too and are not among them: the window offers them,
// those of every source, only in the places of a source - a field, a function,
// a dimension and a field of a dimension table.
func ReferenceKinds() []Kind {
	return slices.Clone(referenceOrder)
}

// References name the types of a reference to an object of the kind (help:
// СправочникСсылка.<Имя справочника>), the reference to the object itself
// first; a business process has a second, to a point of its route. A kind
// whose objects have no reference has none.
func (descriptor Descriptor) References() []Words {
	return slices.Clone(references[descriptor.Kind])
}

// Of finds what a kind is called.
func Of(kind Kind) (Descriptor, bool) {
	index := slices.IndexFunc(descriptors, func(item Descriptor) bool { return item.Kind == kind })
	if index < 0 {
		return Descriptor{}, false
	}
	return descriptors[index], true
}

// Must is Of for a kind the caller names by its constant: one missing here is
// a mistake of this package, not of the input.
func Must(kind Kind) Descriptor {
	descriptor, ok := Of(kind)
	if !ok {
		panic("kinds: no descriptor of " + string(kind))
	}
	return descriptor
}

// ByCollection finds the kind a word of code names by its collection, in
// either language and whatever its case: Справочники, catalogs.
func ByCollection(word string) (Kind, bool) {
	index := slices.IndexFunc(descriptors, func(item Descriptor) bool { return item.Collection.Is(word) })
	if index < 0 {
		return "", false
	}
	return descriptors[index].Kind, true
}

// All is every kind.
func All() []Descriptor {
	return slices.Clone(descriptors)
}
