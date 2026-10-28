package kinds

import "slices"

// SectionCommand is a standard command an object of the kind gives the
// command interface of a subsystem whose content it is in, unless its «Использовать
// стандартные команды» is off (docs/block-00/podsistema.md, «Досъёмка
// 28.10.2026»; snapshots series-2.603/42, 43): which command - the name the
// model writes (metadata.StandardCommand) - in which group of the section,
// whether it is visible there by default, and how it is titled. When says
// which objects of the kind give it: a register of information gives its
// list visible and a command to create a record only when it is
// independent, and its list unseen when a recorder writes it; a catalog
// gives a command to create a folder only when it has folders.
type SectionCommand struct {
	Standard string
	Group    string
	Visible  bool
	Title    SectionCommandTitle
	When     SectionCommandCondition
}

// SectionCommandTitle is how a standard command is titled: by the
// presentation of the list, or else the synonym («Список замовлень
// клієнтів», «Валюти»); by the presentation of an object, or else the
// synonym, with «: создать» or «: создать группу» after it by the command
// («Замовлення клієнта: создать», «Бригади: создать группу»); by the
// synonym.
type SectionCommandTitle string

const (
	ListTitle    SectionCommandTitle = "list"
	CreateTitle  SectionCommandTitle = "create"
	SynonymTitle SectionCommandTitle = "synonym"
)

// SectionCommandCondition is which objects of a kind give a command: every
// one, or only an independent register or only one a recorder writes, or
// only one with folders.
type SectionCommandCondition string

const (
	EveryObject       SectionCommandCondition = ""
	IndependentObject SectionCommandCondition = "independent"
	RecordedObject    SectionCommandCondition = "recorded"
	FolderObject      SectionCommandCondition = "folders"
)

// The groups a standard command stands in (metadata.InterfaceGroupKind).
const (
	navigationOrdinary = "navigation-panel-ordinary"
	actionsCreate      = "actions-panel-create"
	actionsReports     = "actions-panel-reports"
	actionsTools       = "actions-panel-tools"
)

var (
	listAndCreate = []SectionCommand{{Standard: "open-list", Group: navigationOrdinary, Visible: true, Title: ListTitle},
		{Standard: "create", Group: actionsCreate, Title: CreateTitle}}
	unseenList = []SectionCommand{{Standard: "open-list", Group: navigationOrdinary, Title: ListTitle}}
)

// sectionCommands are the standard commands of each kind; a kind not here
// gives a section none. A catalog of folders and items adds the creation of
// a folder, unseen, and a common form of standard commands is opened from
// «Сервис», visible, by its synonym (podsistema.md, «Досъёмка 28.10.2026,
// вечер»; snapshots seriya-2631/07, 08). The commands the inventory has not
// shot - a folder of another kind, a table of an external source - are not
// here yet (question В6 of series 2.631, В4 of series 2.636).
var sectionCommands = map[Kind][]SectionCommand{
	Catalog:  {listAndCreate[0], listAndCreate[1], {Standard: "create-folder", Group: actionsCreate, Title: CreateTitle, When: FolderObject}},
	Document: listAndCreate, ChartOfCharacteristicTypes: listAndCreate, ChartOfAccounts: listAndCreate,
	ChartOfCalculationTypes: listAndCreate, ExchangePlan: listAndCreate, BusinessProcess: listAndCreate, Task: listAndCreate,
	DocumentJournal: {{Standard: "open-list", Group: navigationOrdinary, Visible: true, Title: ListTitle}},
	InformationRegister: {{Standard: "open-list", Group: navigationOrdinary, Visible: true, Title: ListTitle, When: IndependentObject},
		{Standard: "open-list", Group: navigationOrdinary, Title: ListTitle, When: RecordedObject},
		{Standard: "create", Group: actionsCreate, Title: CreateTitle, When: IndependentObject}},
	AccumulationRegister: unseenList, AccountingRegister: unseenList, CalculationRegister: unseenList, Enumeration: unseenList,
	Constant:      {{Standard: "open", Group: actionsTools, Visible: true, Title: SynonymTitle}},
	Report:        {{Standard: "open", Group: actionsReports, Visible: true, Title: SynonymTitle}},
	DataProcessor: {{Standard: "open", Group: actionsTools, Visible: true, Title: SynonymTitle}},
	CommonForm:    {{Standard: "open", Group: actionsTools, Visible: true, Title: SynonymTitle}},
}

// withoutSectionCommands are the kinds whose objects give a section no
// standard command: said here so that every kind says what it gives.
var withoutSectionCommands = []Kind{Role, Subsystem, SessionParameter, CommonAttribute, CommonModule, EventSubscription, ScheduledJob,
	CommandGroup, CommonCommand, CommonTemplate, XDTOPackage, WebService, WSReference, WebSocketClient, ExternalDataSource,
	ExternalDataSourceTable, ExternalCube, ExternalDimensionTable, ExternalFunction, HTTPService, CommonPicture, StyleItem, Style,
	FunctionalOption, FunctionalOptionParameter, SettingsStorage, FilterCriterion, DefinedType, Language, Numerator, Sequence,
	Recalculation}

// SectionCommands are the standard commands an object of the kind gives a
// section, in the order the inventory lists them.
func (descriptor Descriptor) SectionCommands() []SectionCommand {
	return slices.Clone(sectionCommands[descriptor.Kind])
}
