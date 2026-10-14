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
}

// descriptors is every kind, in the order of the configuration tree (see
// Branches); a kind whose objects lie inside another's stands after its owner.
// The names are the help's (8.3.27: the classes of metadata objects and the
// collections of ОбъектМетаданныхКонфигурация, ВнешнийИсточникДанных, Куб,
// РегистрРасчета) and the export's tags and folders; Russian names are
// written without ё, as the help writes them.
var descriptors = []Descriptor{
	{Subsystem, Words{"Подсистема", "Subsystem"}, Words{"Подсистемы", "Subsystems"}, "Подсистемы", "Подсистема", "Подсистемы"},
	{CommonModule, Words{"ОбщийМодуль", "CommonModule"}, Words{"ОбщиеМодули", "CommonModules"}, "ОбщегоМодуля", "Общий модуль", "Общие модули"},
	{SessionParameter, Words{"ПараметрСеанса", "SessionParameter"}, Words{"ПараметрыСеанса", "SessionParameters"}, "ПараметраСеанса", "Параметр сеанса", "Параметры сеанса"},
	{Role, Words{"Роль", "Role"}, Words{"Роли", "Roles"}, "Роли", "Роль", "Роли"},
	{CommonAttribute, Words{"ОбщийРеквизит", "CommonAttribute"}, Words{"ОбщиеРеквизиты", "CommonAttributes"}, "ОбщегоРеквизита", "Общий реквизит", "Общие реквизиты"},
	{ExchangePlan, Words{"ПланОбмена", "ExchangePlan"}, Words{"ПланыОбмена", "ExchangePlans"}, "ПланаОбмена", "План обмена", "Планы обмена"},
	{FilterCriterion, Words{"КритерийОтбора", "FilterCriterion"}, Words{"КритерииОтбора", "FilterCriteria"}, "КритерияОтбора", "Критерий отбора", "Критерии отбора"},
	{EventSubscription, Words{"ПодпискаНаСобытие", "EventSubscription"}, Words{"ПодпискиНаСобытия", "EventSubscriptions"}, "ПодпискиНаСобытие", "Подписка на событие", "Подписки на события"},
	{ScheduledJob, Words{"РегламентноеЗадание", "ScheduledJob"}, Words{"РегламентныеЗадания", "ScheduledJobs"}, "РегламентногоЗадания", "Регламентное задание", "Регламентные задания"},
	{FunctionalOption, Words{"ФункциональнаяОпция", "FunctionalOption"}, Words{"ФункциональныеОпции", "FunctionalOptions"}, "ФункциональнойОпции", "Функциональная опция", "Функциональные опции"},
	{FunctionalOptionParameter, Words{"ПараметрФункциональныхОпций", "FunctionalOptionsParameter"}, Words{"ПараметрыФункциональныхОпций", "FunctionalOptionsParameters"},
		"ПараметраФункциональныхОпций", "Параметр функциональных опций", "Параметры функциональных опций"},
	{DefinedType, Words{"ОпределяемыйТип", "DefinedType"}, Words{"ОпределяемыеТипы", "DefinedTypes"}, "ОпределяемогоТипа", "Определяемый тип", "Определяемые типы"},
	{SettingsStorage, Words{"ХранилищеНастроек", "SettingsStorage"}, Words{"ХранилищаНастроек", "SettingsStorages"}, "ХранилищаНастроек", "Хранилище настроек", "Хранилища настроек"},
	{CommonCommand, Words{"ОбщаяКоманда", "CommonCommand"}, Words{"ОбщиеКоманды", "CommonCommands"}, "ОбщейКоманды", "Общая команда", "Общие команды"},
	{CommandGroup, Words{"ГруппаКоманд", "CommandGroup"}, Words{"ГруппыКоманд", "CommandGroups"}, "ГруппыКоманд", "Группа команд", "Группы команд"},
	{CommonForm, Words{"ОбщаяФорма", "CommonForm"}, Words{"ОбщиеФормы", "CommonForms"}, "ОбщейФормы", "Общая форма", "Общие формы"},
	{CommonTemplate, Words{"ОбщийМакет", "CommonTemplate"}, Words{"ОбщиеМакеты", "CommonTemplates"}, "ОбщегоМакета", "Общий макет", "Общие макеты"},
	{CommonPicture, Words{"ОбщаяКартинка", "CommonPicture"}, Words{"ОбщиеКартинки", "CommonPictures"}, "ОбщейКартинки", "Общая картинка", "Общие картинки"},
	{XDTOPackage, Words{"ПакетXDTO", "XDTOPackage"}, Words{"ПакетыXDTO", "XDTOPackages"}, "ПакетаXDTO", "XDTO-пакет", "XDTO-пакеты"},
	{WebService, Words{"WebСервис", "WebService"}, Words{"WebСервисы", "WebServices"}, "WebСервиса", "Web-сервис", "Web-сервисы"},
	{HTTPService, Words{"HTTPСервис", "HTTPService"}, Words{"HTTPСервисы", "HTTPServices"}, "HTTPСервиса", "HTTP-сервис", "HTTP-сервисы"},
	{WSReference, Words{"WSСсылка", "WSReference"}, Words{"WSСсылки", "WSReferences"}, "WSСсылки", "WS-ссылка", "WS-ссылки"},
	{WebSocketClient, Words{"WebSocketКлиент", "WebSocketClient"}, Words{"WebSocketКлиенты", "WebSocketClients"}, "WebSocketКлиента", "WebSocket-клиент", "WebSocket-клиенты"},
	{StyleItem, Words{"ЭлементСтиля", "StyleItem"}, Words{"ЭлементыСтиля", "StyleItems"}, "ЭлементаСтиля", "Элемент стиля", "Элементы стиля"},
	{Style, Words{"Стиль", "Style"}, Words{"Стили", "Styles"}, "Стиля", "Стиль", "Стили"},
	{Language, Words{"Язык", "Language"}, Words{"Языки", "Languages"}, "Языка", "Язык", "Языки"},
	{Constant, Words{"Константа", "Constant"}, Words{"Константы", "Constants"}, "Константы", "Константа", "Константы"},
	{Catalog, Words{"Справочник", "Catalog"}, Words{"Справочники", "Catalogs"}, "Справочника", "Справочник", "Справочники"},
	{Document, Words{"Документ", "Document"}, Words{"Документы", "Documents"}, "Документа", "Документ", "Документы"},
	{Numerator, Words{"Нумератор", "DocumentNumerator"}, Words{"НумераторыДокументов", "DocumentNumerators"}, "Нумератора", "Нумератор", "Нумераторы"},
	{Sequence, Words{"Последовательность", "Sequence"}, Words{"Последовательности", "Sequences"}, "Последовательности", "Последовательность", "Последовательности"},
	{DocumentJournal, Words{"ЖурналДокументов", "DocumentJournal"}, Words{"ЖурналыДокументов", "DocumentJournals"}, "ЖурналаДокументов", "Журнал документов", "Журналы документов"},
	{Enumeration, Words{"Перечисление", "Enum"}, Words{"Перечисления", "Enums"}, "Перечисления", "Перечисление", "Перечисления"},
	{Report, Words{"Отчет", "Report"}, Words{"Отчеты", "Reports"}, "Отчета", "Отчёт", "Отчёты"},
	{DataProcessor, Words{"Обработка", "DataProcessor"}, Words{"Обработки", "DataProcessors"}, "Обработки", "Обработка", "Обработки"},
	{ChartOfCharacteristicTypes, Words{"ПланВидовХарактеристик", "ChartOfCharacteristicTypes"}, Words{"ПланыВидовХарактеристик", "ChartsOfCharacteristicTypes"},
		"ПланаВидовХарактеристик", "План видов характеристик", "Планы видов характеристик"},
	{ChartOfAccounts, Words{"ПланСчетов", "ChartOfAccounts"}, Words{"ПланыСчетов", "ChartsOfAccounts"}, "ПланаСчетов", "План счетов", "Планы счетов"},
	{ChartOfCalculationTypes, Words{"ПланВидовРасчета", "ChartOfCalculationTypes"}, Words{"ПланыВидовРасчета", "ChartsOfCalculationTypes"},
		"ПланаВидовРасчета", "План видов расчёта", "Планы видов расчёта"},
	{InformationRegister, Words{"РегистрСведений", "InformationRegister"}, Words{"РегистрыСведений", "InformationRegisters"}, "РегистраСведений", "Регистр сведений", "Регистры сведений"},
	{AccumulationRegister, Words{"РегистрНакопления", "AccumulationRegister"}, Words{"РегистрыНакопления", "AccumulationRegisters"}, "РегистраНакопления", "Регистр накопления", "Регистры накопления"},
	{AccountingRegister, Words{"РегистрБухгалтерии", "AccountingRegister"}, Words{"РегистрыБухгалтерии", "AccountingRegisters"}, "РегистраБухгалтерии", "Регистр бухгалтерии", "Регистры бухгалтерии"},
	{CalculationRegister, Words{"РегистрРасчета", "CalculationRegister"}, Words{"РегистрыРасчета", "CalculationRegisters"}, "РегистраРасчета", "Регистр расчёта", "Регистры расчёта"},
	{Recalculation, Words{"Перерасчет", "Recalculation"}, Words{"Перерасчеты", "Recalculations"}, "Перерасчета", "Перерасчёт", "Перерасчёты"},
	{BusinessProcess, Words{"БизнесПроцесс", "BusinessProcess"}, Words{"БизнесПроцессы", "BusinessProcesses"}, "БизнесПроцесса", "Бизнес-процесс", "Бизнес-процессы"},
	{Task, Words{"Задача", "Task"}, Words{"Задачи", "Tasks"}, "Задачи", "Задача", "Задачи"},
	{ExternalDataSource, Words{"ВнешнийИсточникДанных", "ExternalDataSource"}, Words{"ВнешниеИсточникиДанных", "ExternalDataSources"},
		"ВнешнегоИсточникаДанных", "Внешний источник данных", "Внешние источники данных"},
	{ExternalDataSourceTable, Words{"Таблица", "Table"}, Words{"Таблицы", "Tables"}, "ТаблицыВнешнегоИсточника", "Таблица", "Таблицы"},
	{ExternalCube, Words{"Куб", "Cube"}, Words{"Кубы", "Cubes"}, "КубаВнешнегоИсточника", "Куб", "Кубы"},
	{ExternalDimensionTable, Words{"ТаблицаИзмерения", "DimensionTable"}, Words{"ТаблицыИзмерений", "DimensionTables"},
		"ТаблицыИзмеренияВнешнегоИсточника", "Таблица измерения", "Таблицы измерений"},
	{ExternalFunction, Words{"Функция", "Function"}, Words{"Функции", "Functions"}, "ФункцииВнешнегоИсточника", "Функция", "Функции"},
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
