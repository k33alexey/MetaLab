package kinds

import (
	_ "embed"
	"slices"
)

// managers is the type of the manager of an object of the kind, by its
// Russian name in managers.yaml (help: СправочникМенеджер.<Имя справочника>,
// ПерерасчетМенеджер.<Имя перерасчета>). A kind the code reaches no object of
// by a manager - a common module, a role, a session parameter, whose
// collection gives the value itself - has none.
var managers = map[Kind]string{
	Constant:                   "КонстантаМенеджер",
	Catalog:                    "СправочникМенеджер",
	Document:                   "ДокументМенеджер",
	DocumentJournal:            "ЖурналДокументовМенеджер",
	Enumeration:                "ПеречислениеМенеджер",
	Report:                     "ОтчетМенеджер",
	DataProcessor:              "ОбработкаМенеджер",
	ChartOfCharacteristicTypes: "ПланВидовХарактеристикМенеджер",
	ChartOfAccounts:            "ПланСчетовМенеджер",
	ChartOfCalculationTypes:    "ПланВидовРасчетаМенеджер",
	InformationRegister:        "РегистрСведенийМенеджер",
	AccumulationRegister:       "РегистрНакопленияМенеджер",
	AccountingRegister:         "РегистрБухгалтерииМенеджер",
	CalculationRegister:        "РегистрРасчетаМенеджер",
	Recalculation:              "ПерерасчетМенеджер",
	BusinessProcess:            "БизнесПроцессМенеджер",
	Task:                       "ЗадачаМенеджер",
	ExchangePlan:               "ПланОбменаМенеджер",
	Sequence:                   "ПоследовательностьМенеджер",
	FilterCriterion:            "КритерийОтбораМенеджер",
	SettingsStorage:            "ХранилищеНастроекМенеджер",
	ExternalDataSource:         "ВнешнийИсточникДанныхМенеджер",
	ExternalDataSourceTable:    "ВнешнийИсточникДанныхТаблицаМенеджер",
	ExternalCube:               "ВнешнийИсточникДанныхКубМенеджер",
	ExternalDimensionTable:     "ВнешнийИсточникДанныхКубТаблицаИзмеренияМенеджер",
	WSReference:                "WSСсылкаМенеджер",
}

// globalContext is the kinds whose collection is a property of the global
// context (help: Глобальный контекст - Справочники, ПараметрыСеанса). An
// object inside another is reached through its owner's manager instead
// (Descriptor.Inner).
var globalContext = map[Kind]bool{
	Constant: true, Catalog: true, Document: true, DocumentJournal: true, Enumeration: true, Report: true,
	DataProcessor: true, ChartOfCharacteristicTypes: true, ChartOfAccounts: true, ChartOfCalculationTypes: true,
	InformationRegister: true, AccumulationRegister: true, AccountingRegister: true, CalculationRegister: true,
	BusinessProcess: true, Task: true, ExchangePlan: true, Sequence: true, FilterCriterion: true,
	SettingsStorage: true, ExternalDataSource: true, WSReference: true, SessionParameter: true,
}

// ownerMethods is the kinds whose object is a method of the manager of the
// object it lies inside, and no manager of its own (help:
// ВнешнийИсточникДанныхМенеджер.<Имя внешнего источника>.<Имя функции>()).
var ownerMethods = map[Kind]bool{ExternalFunction: true}

//go:embed managers.yaml
var managersSource []byte

// managerTypes are the types of managers.yaml by their Russian name.
var managerTypes = readObjectTypes("managers.yaml", managersSource)

// Manager is the type of the manager of an object of the kind, the members
// it has whatever the object among them. False is a kind the code reaches no
// object of by a manager.
func (descriptor Descriptor) Manager() (ObjectType, bool) {
	name, ok := managers[descriptor.Kind]
	if !ok {
		return ObjectType{}, false
	}
	objectType := managerTypes[name]
	objectType.Members = slices.Clone(objectType.Members)
	return objectType, true
}

// InGlobalContext says the collection of the kind is a property of the
// global context: Справочники, ПараметрыСеанса.
func (descriptor Descriptor) InGlobalContext() bool {
	return globalContext[descriptor.Kind]
}

// Inner finds the kind of the objects the manager of an object of this kind
// holds under the collection the word names, in either language: Таблицы and
// Кубы of a source of data, Перерасчеты of a calculation register,
// ТаблицыИзмерений of a cube.
func (descriptor Descriptor) Inner(word string) (Descriptor, bool) {
	index := slices.IndexFunc(descriptors, func(item Descriptor) bool {
		_, managed := managers[item.Kind]
		return managed && item.Layout().Owner == descriptor.Kind && item.Collection.Is(word)
	})
	if index < 0 {
		return Descriptor{}, false
	}
	return descriptors[index], true
}

// OwnerMethod says an object of the kind is called as a method of the
// manager of the object it lies inside (Layout.Owner): a function of a
// source of data.
func (descriptor Descriptor) OwnerMethod() bool {
	return ownerMethods[descriptor.Kind]
}
