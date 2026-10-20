package kinds

import "slices"

// Storage is how the objects of a kind lie in the project.
type Storage int

const (
	// InFile is one description per object, named by its identifier:
	// metadata/subsystems/<id>.yaml.
	InFile Storage = iota + 1
	// InFolder is a folder named after the object holding its description,
	// its modules and its forms, commands and templates - everything the
	// prototype's export keeps under the object, kept together the same way.
	InFolder
	// InFilesFolder is a folder named after the object holding its
	// description, its modules and its content, and no subordinate entities.
	InFilesFolder
	// InFormFolder is a folder named after the object laid out as a form is:
	// a common form.
	InFormFolder
	// InConfiguration is a list inside the description of the configuration
	// root: the languages.
	InConfiguration
	// InOwner is described inside its owner, which keeps only the object's
	// modules on disk, in a folder of its own: a recalculation.
	InOwner
	// InOwnerDescription is a list inside the description of its owner, with
	// nothing of its own on disk: a function of an external data source,
	// which the export writes among the child objects of the source and which
	// has no modules, forms or templates.
	InOwnerDescription
)

// Branch is where the configuration tree shows the objects of a kind.
type Branch int

const (
	// NoBranch is a kind whose objects the tree shows under their owner.
	NoBranch Branch = iota
	// CommonBranch is the group «Общие».
	CommonBranch
	// TopBranch stands beside «Общие».
	TopBranch
	// DocumentsBranch stands inside «Документы», before the documents
	// themselves: numerators and sequences.
	DocumentsBranch
)

// Module is the role a module plays in the folder of what owns it, and the
// name of its file there.
type Module string

const (
	ObjectModule    Module = "МодульОбъекта"
	ManagerModule   Module = "МодульМенеджера"
	RecordSetModule Module = "МодульНабораЗаписей"
	// ValueModule is the prototype's value manager module of a constant
	// (ValueManagerModule in the export), named by what it is called around.
	ValueModule   Module = "МодульЗначения"
	CommandModule Module = "МодульКоманды"
	// ServiceModule is the one module of a web or HTTP service (Module in the
	// export): the procedures its operations name.
	ServiceModule Module = "МодульСервиса"
	ClientModule  Module = "МодульКлиента"
	FormModule    Module = "МодульФормы"
)

// moduleTitles are the captions of the modules in the interface.
var moduleTitles = map[Module]string{
	ObjectModule: "Модуль объекта", ManagerModule: "Модуль менеджера", RecordSetModule: "Модуль набора записей",
	ValueModule: "Модуль значения", CommandModule: "Модуль команды", ServiceModule: "Модуль сервиса",
	ClientModule: "Модуль клиента", FormModule: "Модуль формы",
}

// moduleCommands are the words of the command of its owner's menu that
// opens the module, as the prototype's menu of each kind names it (inventory
// of block 00: spravochnik.md, konstanta.md, registr-svedenij.md,
// http-servis.md, websocket-klient.md, obshchaya-komanda.md and the rest): a
// constant's module of value is its «модуль менеджера значения», the one
// module of a service or a client is «Открыть модуль». A form's module has
// none - a form opens by «Открыть форму», its module inside it.
var moduleCommands = map[Module]string{
	ObjectModule: "Открыть модуль объекта", ManagerModule: "Открыть модуль менеджера",
	RecordSetModule: "Открыть модуль набора записей", ValueModule: "Открыть модуль менеджера значения",
	CommandModule: "Открыть модуль команды", ServiceModule: "Открыть модуль", ClientModule: "Открыть модуль",
}

// Command is the words of the command of its owner's menu that opens the
// module; empty for a module no menu opens.
func (module Module) Command() string { return moduleCommands[module] }

// OwnAction says opening the module is the action of its owner itself - bold
// in its menu and done by a double click: a command is the module that runs
// it («Открыть модуль команды» bold, inventory of block 00, komanda.md,
// obshchaya-komanda.md); every other owner's own action is «Изменить».
func (module Module) OwnAction() bool { return module == CommandModule }

// File is the name of the module's file.
func (module Module) File() string { return string(module) + ".bsl" }

// Title is the caption of the module in the interface.
func (module Module) Title() string { return moduleTitles[module] }

// ModuleOfFile reads a file name as the module it is.
func ModuleOfFile(name string) (Module, bool) {
	for module := range moduleTitles {
		if module.File() == name {
			return module, true
		}
	}
	return "", false
}

// The words a module's code finds defined without declaring them, as the help
// names the properties behind them (8.3.27).
var (
	// ThisObject is the object, record set or value manager a module runs
	// around (СправочникОбъект.ЭтотОбъект, РегистрСведенийНаборЗаписей.
	// ЭтотОбъект, КонстантаМенеджерЗначения.ЭтотОбъект, ...), the form of a
	// form's module (ФормаКлиентскогоПриложения.ЭтотОбъект), the command of a
	// command's module (КомандаКомандногоИнтерфейса.ЭтотОбъект) and the common
	// module itself (ОбщийМодуль.ЭтотОбъект).
	ThisObject = Words{"ЭтотОбъект", "ThisObject"}
	// ThisForm is the form of a form's module. ФормаКлиентскогоПриложения
	// has no page of it in the help; the forms of the exports use it all the
	// same - in 908 modules of acc, 2466 of erp, 390 of sb - and the
	// prototype compiles them.
	ThisForm = Words{"ЭтаФорма", "ThisForm"}
	// Items are the elements of a form (ФормаКлиентскогоПриложения.
	// Элементы).
	Items = Words{"Элементы", "Items"}
	// RegisterRecords is the record sets a document writes into
	// (ДокументОбъект.Движения).
	RegisterRecords = Words{"Движения", "RegisterRecords"}
)

// moduleWords are the words of a module by its role. A manager, a service and
// a WebSocket client have none: the help gives ЭтотОбъект to no manager
// (СправочникМенеджер and the rest), to no service and to no client.
var moduleWords = map[Module][]Words{
	ObjectModule: {ThisObject}, RecordSetModule: {ThisObject}, ValueModule: {ThisObject},
	CommandModule: {ThisObject}, FormModule: {ThisForm, ThisObject, Items},
}

// objectModuleWords are the words an object module of a kind has beyond
// those of its role: only a document's object has its register records.
var objectModuleWords = map[Kind][]Words{Document: {RegisterRecords}}

// ModuleWords are the words the code of a module of an object of the kind
// finds defined: ЭтотОбъект, ЭтаФорма, Движения. A common module keeps its one
// module apart from any role and has ЭтотОбъект all the same.
func (descriptor Descriptor) ModuleWords(module Module) []Words {
	if descriptor.Kind == CommonModule {
		return []Words{ThisObject}
	}
	result := slices.Clone(moduleWords[module])
	if module == ObjectModule {
		result = append(result, objectModuleWords[descriptor.Kind]...)
	}
	return result
}

// Layout is where the objects of a kind lie and what each keeps there.
type Layout struct {
	Storage Storage
	// Owner and Folder place an object that lies inside another: in the
	// folder Folder of an object of the kind Owner. Empty for an object of
	// the configuration itself.
	Owner  Kind
	Folder string
	// Modules are the roles of the modules an object of the kind may keep in
	// its folder, as the prototype's export keeps them (Ext/ObjectModule.bsl,
	// ManagerModule.bsl, RecordSetModule.bsl, ValueManagerModule.bsl,
	// CommandModule.bsl, Module.bsl of a service). A common module keeps its
	// one module apart, in modules/, by its identifier.
	Modules []Module
	// Forms, Commands and Templates say an object of the kind keeps forms,
	// commands and templates of its own (help: ОбъектМетаданных: <Вид>.Формы,
	// .Команды, .Макеты): a criterion of filter has no templates, a storage
	// of settings no commands.
	Forms, Commands, Templates bool
	Branch                     Branch
}

var (
	objectModules    = []Module{ObjectModule, ManagerModule}
	recordSetModules = []Module{RecordSetModule, ManagerModule}
	managerModules   = []Module{ManagerModule}
)

// layouts is the layout of every kind. The modules are those of the exports
// (four configurations checked and energy): a reference object and a report
// keep an object module, a register a record set module, and what has no
// object to write - an enumeration, a journal, a criterion, a storage of
// settings - only a manager; a sequence has a record set and no manager.
var layouts = map[Kind]Layout{
	Subsystem:                  {Storage: InFile, Branch: CommonBranch},
	CommonModule:               {Storage: InFile, Branch: CommonBranch},
	SessionParameter:           {Storage: InFile, Branch: CommonBranch},
	Role:                       {Storage: InFile, Branch: CommonBranch},
	CommonAttribute:            {Storage: InFile, Branch: CommonBranch},
	ExchangePlan:               {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: CommonBranch},
	FilterCriterion:            {Storage: InFolder, Modules: managerModules, Forms: true, Commands: true, Branch: CommonBranch},
	EventSubscription:          {Storage: InFile, Branch: CommonBranch},
	ScheduledJob:               {Storage: InFile, Branch: CommonBranch},
	FunctionalOption:           {Storage: InFile, Branch: CommonBranch},
	FunctionalOptionParameter:  {Storage: InFile, Branch: CommonBranch},
	DefinedType:                {Storage: InFile, Branch: CommonBranch},
	SettingsStorage:            {Storage: InFolder, Modules: managerModules, Forms: true, Templates: true, Branch: CommonBranch},
	CommonCommand:              {Storage: InFilesFolder, Modules: []Module{CommandModule}, Branch: CommonBranch},
	CommandGroup:               {Storage: InFile, Branch: CommonBranch},
	CommonForm:                 {Storage: InFormFolder, Modules: []Module{FormModule}, Branch: CommonBranch},
	CommonTemplate:             {Storage: InFilesFolder, Branch: CommonBranch},
	CommonPicture:              {Storage: InFilesFolder, Branch: CommonBranch},
	XDTOPackage:                {Storage: InFilesFolder, Branch: CommonBranch},
	WebService:                 {Storage: InFilesFolder, Modules: []Module{ServiceModule}, Branch: CommonBranch},
	HTTPService:                {Storage: InFilesFolder, Modules: []Module{ServiceModule}, Branch: CommonBranch},
	WSReference:                {Storage: InFilesFolder, Branch: CommonBranch},
	WebSocketClient:            {Storage: InFilesFolder, Modules: []Module{ClientModule}, Branch: CommonBranch},
	StyleItem:                  {Storage: InFile, Branch: CommonBranch},
	Style:                      {Storage: InFile, Branch: CommonBranch},
	Language:                   {Storage: InConfiguration, Branch: CommonBranch},
	Constant:                   {Storage: InFilesFolder, Modules: []Module{ValueModule, ManagerModule}, Branch: TopBranch},
	Catalog:                    {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	Document:                   {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	Numerator:                  {Storage: InFile, Branch: DocumentsBranch},
	Sequence:                   {Storage: InFolder, Modules: []Module{RecordSetModule}, Branch: DocumentsBranch},
	DocumentJournal:            {Storage: InFolder, Modules: managerModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	Enumeration:                {Storage: InFolder, Modules: managerModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	Report:                     {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	DataProcessor:              {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	ChartOfCharacteristicTypes: {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	ChartOfAccounts:            {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	ChartOfCalculationTypes:    {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	InformationRegister:        {Storage: InFolder, Modules: recordSetModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	AccumulationRegister:       {Storage: InFolder, Modules: recordSetModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	AccountingRegister:         {Storage: InFolder, Modules: recordSetModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	CalculationRegister:        {Storage: InFolder, Modules: recordSetModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	Recalculation:              {Storage: InOwner, Owner: CalculationRegister, Folder: "recalculations", Modules: []Module{RecordSetModule}},
	BusinessProcess:            {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	Task:                       {Storage: InFolder, Modules: objectModules, Forms: true, Commands: true, Templates: true, Branch: TopBranch},
	ExternalDataSource:         {Storage: InFilesFolder, Branch: TopBranch},
	// A table is an object or a record set by its type (help:
	// ВнешнийИсточникДанныхТаблицаОбъект, ...НаборЗаписей); which one a table
	// keeps the model decides by the table.
	ExternalDataSourceTable: {Storage: InFolder, Owner: ExternalDataSource, Folder: "tables",
		Modules: []Module{ObjectModule, RecordSetModule, ManagerModule}, Forms: true, Commands: true, Templates: true},
	ExternalCube: {Storage: InFolder, Owner: ExternalDataSource, Folder: "cubes", Modules: recordSetModules, Forms: true, Commands: true, Templates: true},
	ExternalDimensionTable: {Storage: InFolder, Owner: ExternalCube, Folder: "dimension-tables",
		Modules: objectModules, Forms: true, Commands: true, Templates: true},
	ExternalFunction: {Storage: InOwnerDescription, Owner: ExternalDataSource},
}

// Layout is where the objects of the kind lie and what each keeps there.
func (descriptor Descriptor) Layout() Layout {
	layout := layouts[descriptor.Kind]
	layout.Modules = slices.Clone(layout.Modules)
	return layout
}

// KeepsFolder says an object of the kind keeps a folder named after it.
func (layout Layout) KeepsFolder() bool {
	return layout.Storage == InFolder || layout.Storage == InFilesFolder || layout.Storage == InFormFolder
}

// Branches is every kind the configuration tree shows in a branch of its
// own, in the order of the tree: «Общие» first, as the prototype's
// configurator shows it, numerators and sequences beside the documents.
// These are also the folders of metadata/.
func Branches() []Descriptor {
	var result []Descriptor
	for _, item := range descriptors {
		if layouts[item.Kind].Branch != NoBranch {
			result = append(result, item)
		}
	}
	return result
}
