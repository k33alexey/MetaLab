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
