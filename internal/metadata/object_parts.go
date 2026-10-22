package metadata

import (
	"reflect"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PartGroup is a sort of part of an object: what the configuration tree shows
// the parts of one sort under, and what decides the rights a role gives on
// such a part.
type PartGroup string

const (
	StandardAttributesPart          PartGroup = "standard-attributes"
	DimensionsPart                  PartGroup = "dimensions"
	ResourcesPart                   PartGroup = "resources"
	AttributesPart                  PartGroup = "attributes"
	AddressingAttributesPart        PartGroup = "addressing-attributes"
	AccountingFlagsPart             PartGroup = "accounting-flags"
	ExtDimensionAccountingFlagsPart PartGroup = "ext-dimension-accounting-flags"
	TablePartsPart                  PartGroup = "table-parts"
	StandardTablePartsPart          PartGroup = "standard-table-parts"
	FieldsPart                      PartGroup = "fields"
	CommandsPart                    PartGroup = "commands"
	OperationsPart                  PartGroup = "operations"
	URLTemplatesPart                PartGroup = "url-templates"
	MethodsPart                     PartGroup = "methods"
)

// partGroupInfo is one sort of part: the word the prototype names a part of
// it by in a role (Catalog.Товары.TabularSection.Состав), the caption of its
// folder, the rights the configurator offers on such a part, the field of a
// description that lists the parts of the sort (describedParts) - empty for
// a sort the platform gives or a kind reads in its own way - and the icon its
// branch and its parts are shown by in the tree (ICONS.md), empty for a sort
// the tree shows no branch of.
type partGroupInfo struct {
	group     PartGroup
	prototype string
	title     string
	rights    []PermissionOperation
	field     string
	icon      string
}

// partGroups lists every sort of part in the order the tree shows them. The
// rights are the exports': every Rights.xml of every role under docs/ (2526,
// survey of 12.10.2026) writes View and Edit on a field of any sort, View on
// a command and Use on an operation of a web service and a method of an HTTP
// service. A template of an HTTP service has no rights: the exports write
// them on its methods only.
var partGroups = []partGroupInfo{
	{StandardAttributesPart, "StandardAttribute", "Стандартные реквизиты", fieldRights, "", "standard-attribute"},
	{DimensionsPart, "Dimension", "Измерения", fieldRights, "Dimensions", "dimension"},
	{ResourcesPart, "Resource", "Ресурсы", fieldRights, "Resources", "resource"},
	{AttributesPart, "Attribute", "Реквизиты", fieldRights, "Attributes", "attribute"},
	{AddressingAttributesPart, "AddressingAttribute", "Реквизиты адресации", fieldRights, "", "addressing-attribute"},
	{AccountingFlagsPart, "AccountingFlag", "Признаки учета", fieldRights, "AccountingFlags", "accounting-flag"},
	{ExtDimensionAccountingFlagsPart, "ExtDimensionAccountingFlag", "Признаки учета субконто", fieldRights, "ExtDimensionAccountingFlags", "ext-dimension-accounting-flag"},
	{TablePartsPart, "TabularSection", "Табличные части", fieldRights, "TableParts", "tabular-section"},
	{StandardTablePartsPart, "StandardTabularSection", "Стандартные табличные части", fieldRights, "", "standard-tabular-section"},
	{FieldsPart, "Field", "Поля", fieldRights, "Fields", "attribute"},
	{CommandsPart, "Command", "Команды", []PermissionOperation{PermissionView}, "Commands", "command"},
	{OperationsPart, "Operation", "Операции", []PermissionOperation{PermissionUse}, "", "web-service-operation"},
	{URLTemplatesPart, "URLTemplate", "Шаблоны URL", nil, "", "url-template"},
	{MethodsPart, "Method", "Методы", []PermissionOperation{PermissionUse}, "", "http-method"},
}

// fieldRights are a field's two rights, whatever the field is.
var fieldRights = []PermissionOperation{PermissionView, PermissionEdit}

// partGroupOf is what partGroups says of the group; nothing for a group it
// does not list.
func partGroupOf(group PartGroup) partGroupInfo {
	for _, item := range partGroups {
		if item.group == group {
			return item
		}
	}
	return partGroupInfo{}
}

// partRightsOf is the rights the configurator offers on a part of the group.
func partRightsOf(group PartGroup) []PermissionOperation {
	return partGroupOf(group).rights
}

// offeredPartRights is the rights the configurator offers on a part of the
// group in an object of the kind: those the window of a role was seen to
// offer on the kind's parts (kindPartRightsOf), those of the group where the
// exports give rights on the parts of the kind, and none where they never do
// - a right given there all the same is carried and noted
// (NoteRightWithoutSample).
func offeredPartRights(kind Kind, group PartGroup) []PermissionOperation {
	if offered := usesOf(kind).partsOffered; offered != nil {
		return offered[group]
	}
	if usesOf(kind).parts != present {
		return nil
	}
	return partRightsOf(group)
}

// objectPart is one part of an object a role may name: a field, a table part,
// a command, an operation, or a template that holds methods.
type objectPart struct {
	// key is the part's identifier, or for a standard one its prototype name
	// in lower case, and for a field of a standard table part the part's name
	// before it - the key a role keeps the part's rights under.
	key  string
	name string
	// english is the English name of a part the platform gives; a part the
	// object declares has one name in both languages, and this is empty.
	english string
	title   LocalizedText
	group   PartGroup
	// parent is the key of the part this one lies in: a table part, a
	// standard table part, a template of an HTTP service.
	parent string
	// attribute is the field itself where the part holds a value with its
	// types, for what has to know what the field can hold.
	attribute *Attribute
}

// kindComposition is what an object of a kind holds, kept beside kindInfos
// as kindUsesOf is.
type kindComposition struct {
	// described is a kind whose objects declare their parts in the fields of
	// their description that partGroups names (describedParts); the standard
	// ones every object of the kind has are added by partsOf.
	described bool
	// parts lists the declared parts of an object of a kind that reads them
	// in its own way - a task, a criterion of filter, the services. Nil, and
	// not described, is a kind whose objects declare no parts.
	parts func(catalog *Catalog, id uuid.UUID) []objectPart
	// branches are the sorts of part an object of a kind that reads its
	// parts in its own way shows a branch of in the tree; a described kind
	// has the branch of every sort its description keeps (branchesOf).
	branches []PartGroup
	// inline is a kind whose parts the tree shows right under the object,
	// with no branch of their own: the templates of an HTTP service with
	// their methods, the operations of a web service (inventory of block
	// 00, «Ветки под объектом», snapshots derevo/11a, 11b).
	inline bool
	// held are the lists of what an object keeps beside its parts that the
	// tree shows a branch of after the branches of its parts: the values of
	// an enumeration, the columns of a journal, the functions of a source
	// (snapshots derevo/07, 12b, 10). Nil where there are none.
	held []heldList
	// beneath is what the tree shows under a part of an object, or under an
	// item of a held list, that is no part itself, by the part or the item:
	// the parameters of an operation of a web service, the dimensions of a
	// recalculation (snapshots derevo/11b, 05b). Nil where there is nothing.
	beneath func(catalog *Catalog, id uuid.UUID) map[uuid.UUID][]BranchPart
	// dataVersion is a kind whose objects keep the version of their data
	// (help, the tables of the query language; the table of a chart of
	// calculation types does not list it, its object - ПланВидовРасчетаОбъект
	// - has it).
	dataVersion bool
	// identities is what else of the object has an identifier of its own and
	// is no part a role may name, beside its held lists; nil where there is
	// nothing.
	identities func(catalog *Catalog, id uuid.UUID) []heldIdentity
	// valueFields are the fields the manager of the value holds beside the
	// value, for a kind whose object is one value and no parts: a constant's
	// common attributes (help: КонстантаМенеджерЗначения.<Имя>.<Имя общего
	// реквизита>). Nil where there are none.
	valueFields func(catalog *Catalog, id uuid.UUID) []Attribute
	// types is the types of the value of a kind whose object is one value
	// and declares no fields: a constant, a session parameter, a common
	// attribute, a defined type. Nil where the object is no such value.
	types func(catalog *Catalog, id uuid.UUID) []Type
	// files is what the folder of an object of the kind keeps beside its
	// modules and declares in its description - the form roles, the commands,
	// the templates, the folders of its own content; the load fills in the
	// rest (validateObjectsAsLoaded). Nil is a kind whose objects keep no
	// such folder, or whose files are checked by code of their own.
	files func(catalog *Catalog, id uuid.UUID) objectFiles
	// queryTable is what an object of the kind decides of the table a query
	// reads of it, where its settings decide which of the fields it keeps
	// (objectFieldsOf) the table has and the window of a restriction lists
	// them in an order of its own (referenceTable); nil is a kind whose
	// table holds every field of its objects.
	queryTable func(catalog *Catalog, id uuid.UUID) referenceTable
	// rightsFolders are the folders the tree of rights of the window of a
	// role shows under an object of the kind, in its order, every one even
	// when empty (pass of 20.10.2026, block-00/rol, «Папки частей»): its own
	// order, not the configuration tree's - the standard attributes stand
	// after the attributes and before the commands, a register of
	// information has them before its attributes, a cube its tables of
	// dimensions first. A folder holds the parts of a sort or the objects of
	// a kind lying inside the object. Nil is a kind whose objects show none.
	rightsFolders []rightsFolder
}

// rightsFolder is one folder of the tree of rights: the parts of the sort
// group, or the objects of the kind lying inside the object.
type rightsFolder struct {
	group PartGroup
	kind  Kind
}

// partFolders are the folders of the parts of the sorts, in their order.
func partFolders(groups ...PartGroup) []rightsFolder {
	result := make([]rightsFolder, len(groups))
	for index, group := range groups {
		result[index] = rightsFolder{group: group}
	}
	return result
}

// The folders of the tree of rights the kinds share (block-00/rol, «Папки
// частей»): a catalog, a document, a business process; a report; the
// registers of accumulation and of accounting.
var (
	referenceFolders = partFolders(AttributesPart, TablePartsPart, StandardAttributesPart, CommandsPart)
	reportFolders    = partFolders(AttributesPart, TablePartsPart, CommandsPart)
	registerFolders  = partFolders(DimensionsPart, ResourcesPart, AttributesPart, StandardAttributesPart, CommandsPart)
	tableFolders     = partFolders(FieldsPart, CommandsPart)
)

// heldList is one list of what an object keeps beside its parts, as the tree
// shows it: the key, caption and icon of its branch - the icon its items are
// shown by too - and its items, named as heldIdentity names them. The word of
// the items says their identifiers are the object's own, checked with the
// rest of what it holds (heldIdentities); empty where the items are objects
// of a kind of their own, which the catalog knows already.
type heldList struct {
	key, title, icon string
	items            func(catalog *Catalog, id uuid.UUID) []heldIdentity
	// kind is the kind of the items where they are objects of a kind of
	// their own (objectsHeld); empty where they are the object's own.
	kind Kind
}

// objectsHeld is the list of the objects of a kind an object keeps inside
// itself, shown by the caption and icon the tree shows the kind by: the
// functions of a source, the recalculations of a calculation register. The
// items are objects the catalog knows already, so they name no identity of
// the object's own.
func objectsHeld(key string, kind Kind, items func(catalog *Catalog, id uuid.UUID) []heldIdentity) heldList {
	descriptor := kinds.Must(kind)
	return heldList{key: key, title: descriptor.Titles, icon: descriptor.Icon, items: items, kind: kind}
}

// formRoles is the forms of a description: every kind keeps its own set of
// roles and says which it has (formSlot).
type formRoles interface{ slots() []formSlot }

// declaredFilesOf is what the folder of an object keeps because its
// description declares it: the forms of its roles, its commands and its
// templates - as many of these as a description of its kind has. The kind
// says which by the description itself, so a kind that gains templates is
// checked for them without a word here.
func declaredFilesOf(catalog *Catalog, id uuid.UUID) objectFiles {
	var files objectFiles
	description, ok := descriptionOf(catalog, catalog.objects[id].kind, id)
	if !ok {
		return files
	}
	if forms := description.FieldByName("Forms"); forms.IsValid() {
		files.formSlots = forms.Interface().(formRoles).slots()
	}
	if commands := description.FieldByName("Commands"); commands.IsValid() {
		files.commands = commands.Interface().([]ObjectCommand)
	}
	if templates := description.FieldByName("Templates"); templates.IsValid() {
		files.templates = templates.Interface().([]ObjectTemplate)
	}
	return files
}

// descriptionOf is the description of an object of the kind, found by the
// kind (kindInfo.object); false where the kind keeps none to find.
func descriptionOf(catalog *Catalog, kind Kind, id uuid.UUID) (reflect.Value, bool) {
	info, _ := kindInfoOf(kind)
	if info.object == nil {
		return reflect.Value{}, false
	}
	found, ok := info.object(catalog, id)
	if !ok {
		return reflect.Value{}, false
	}
	return reflect.ValueOf(found), true
}

// describedParts is the parts an object declares in its description, read
// off the description itself: for every sort of part in the order of
// partGroups, the list its field names (partGroupInfo.field) - fields, with
// the fields of each table part after it, and commands. A kind whose
// description has the field has the parts, so a kind that gains table parts
// is read for them without a word here.
func describedParts(catalog *Catalog, kind Kind, id uuid.UUID) []objectPart {
	description, ok := descriptionOf(catalog, kind, id)
	if !ok {
		return nil
	}
	var parts []objectPart
	for _, sort := range partGroups {
		list := reflect.Value{}
		if sort.field != "" {
			list = description.FieldByName(sort.field)
		}
		for index := 0; list.IsValid() && index < list.Len(); index++ {
			item := list.Index(index).Addr().Interface()
			switch sort.group {
			case TablePartsPart:
				part := item.(*TablePart)
				parts = append(parts, objectPart{key: part.ID.String(), name: part.Name, title: part.Title, group: TablePartsPart})
				for field := range part.Attributes {
					parts = append(parts, fieldPart(&part.Attributes[field], AttributesPart, part.ID.String()))
				}
			case CommandsPart:
				command := item.(*ObjectCommand)
				parts = append(parts, objectPart{key: command.ID.String(), name: command.Name, title: command.Title, group: CommandsPart})
			default:
				parts = append(parts, fieldPart(attributeIn(item), sort.group, ""))
			}
		}
	}
	return parts
}

// attributeIn is the field an item of a list of fields is: the item itself,
// or the Attribute a dimension, a resource or a flag holds beside what is its
// own.
func attributeIn(item any) *Attribute {
	if attribute, ok := item.(*Attribute); ok {
		return attribute
	}
	return reflect.ValueOf(item).Elem().FieldByName("Attribute").Addr().Interface().(*Attribute)
}

// heldIdentity is something an object holds with an identifier of its own,
// named for a message: what it is and its name within the object.
type heldIdentity struct {
	id   uuid.UUID
	word string
	name string
}

// identitiesOf names the items of a list the way heldIdentity does, by their
// ID and Name: every list it is given is of descriptions that have both.
func identitiesOf[T any](word string, items []T) []heldIdentity {
	return identitiesIn(word, reflect.ValueOf(items))
}

// identitiesIn names the items of a list read off a description, as
// identitiesOf does.
func identitiesIn(word string, items reflect.Value) []heldIdentity {
	result := make([]heldIdentity, 0, items.Len())
	for index := range items.Len() {
		item := items.Index(index)
		result = append(result, heldIdentity{id: item.FieldByName("ID").Interface().(uuid.UUID), word: word,
			name: item.FieldByName("Name").String()})
	}
	return result
}

// itemIDs is the identifiers of the items of a list, in its order; nil for
// an empty one.
func itemIDs[T any](items []T) []uuid.UUID {
	if len(items) == 0 {
		return nil
	}
	result := make([]uuid.UUID, 0, len(items))
	for _, item := range identitiesOf("", items) {
		result = append(result, item.id)
	}
	return result
}

// kindCompositionOf is every kind's composition. A test requires every kind
// of the model to be here, so a new kind cannot be left without one.
var kindCompositionOf = map[Kind]kindComposition{
	RoleKind:      {},
	SubsystemKind: {},
	ConstantKind: {valueFields: func(c *Catalog, id uuid.UUID) []Attribute {
		return c.Constants[c.constantByID[id]].CommonAttributeFields
	}, types: func(c *Catalog, id uuid.UUID) []Type {
		return c.Constants[c.constantByID[id]].Types
	}},
	SessionParameterKind: {types: func(c *Catalog, id uuid.UUID) []Type {
		return c.SessionParameters[c.sessionParameterByID[id]].Types
	}},
	CommonAttributeKind: {types: func(c *Catalog, id uuid.UUID) []Type {
		return c.CommonAttributes[c.commonAttributeByID[id]].Types
	}},
	CommonModuleKind:      {},
	EventSubscriptionKind: {},
	EnumerationKind: {described: true, held: []heldList{{key: "values", title: "Значения", icon: "enumeration-value", items: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("value", c.Enumerations[c.enumerationByID[id]].Values)
	}}}, files: declaredFilesOf, rightsFolders: partFolders(CommandsPart)},
	ScheduledJobKind:   {},
	CommandGroupKind:   {},
	CommonCommandKind:  {},
	CommonFormKind:     {},
	CommonTemplateKind: {},
	XDTOPackageKind:    {},
	WebServiceKind: {branches: []PartGroup{OperationsPart}, inline: true, rightsFolders: partFolders(OperationsPart), parts: func(c *Catalog, id uuid.UUID) []objectPart {
		var parts []objectPart
		for _, operation := range c.WebServices[c.webServiceByID[id]].Operations {
			parts = append(parts, objectPart{key: operation.ID.String(), name: operation.Name, title: operation.Title, group: OperationsPart})
		}
		return parts
	}, beneath: func(c *Catalog, id uuid.UUID) map[uuid.UUID][]BranchPart {
		result := map[uuid.UUID][]BranchPart{}
		for _, operation := range c.WebServices[c.webServiceByID[id]].Operations {
			for _, parameter := range operation.Parameters {
				result[operation.ID] = append(result[operation.ID], BranchPart{ID: parameter.ID, Name: parameter.Name, Icon: "parameter"})
			}
		}
		return result
	}},
	WSReferenceKind:     {},
	WebSocketClientKind: {},
	// The functions of a source lie in its description and are objects of
	// their own kind (kinds.ExternalFunction), shown by its caption and icon.
	ExternalDataSourceKind: {held: []heldList{objectsHeld("functions", ExternalFunctionKind, func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("", c.ExternalDataSources[c.externalDataSourceByID[id]].Functions)
	})}, rightsFolders: []rightsFolder{{kind: ExternalDataSourceTableKind}, {kind: ExternalCubeKind}, {kind: ExternalFunctionKind}}},
	ExternalDataSourceTableKind: {described: true, rightsFolders: tableFolders, files: func(c *Catalog, id uuid.UUID) objectFiles {
		files := declaredFilesOf(c, id)
		// The kind has both modules of data; a table keeps the one of its
		// type.
		other := kinds.ObjectModule
		if location := c.externalTableByID[id]; c.ExternalDataSources[location.source].Tables[location.table].ObjectTable() {
			other = kinds.RecordSetModule
		}
		files.modules = slices.DeleteFunc(kinds.Must(ExternalDataSourceTableKind).Layout().Modules, func(module kinds.Module) bool { return module == other })
		return files
	}},
	// A cube is read as a register is: its dimensions and resources are the
	// fields of its records (help: ВнешнийИсточникДанныхКубЗапись.<Имя
	// измерения>, <Имя ресурса>).
	ExternalCubeKind: {described: true, files: declaredFilesOf,
		rightsFolders: append([]rightsFolder{{kind: ExternalDimensionTableKind}}, partFolders(DimensionsPart, ResourcesPart, CommandsPart)...)},
	// A dimension table holds fields as a table of a source does (help:
	// ВнешнийИсточникДанныхКубТаблицаИзмеренияОбъект.<Имя поля>).
	ExternalDimensionTableKind: {described: true, files: declaredFilesOf, rightsFolders: tableFolders},
	ExternalFunctionKind:       {},
	HTTPServiceKind: {branches: []PartGroup{URLTemplatesPart}, inline: true, rightsFolders: partFolders(URLTemplatesPart), parts: func(c *Catalog, id uuid.UUID) []objectPart {
		var parts []objectPart
		for _, template := range c.HTTPServices[c.httpServiceByID[id]].Templates {
			parts = append(parts, objectPart{key: template.ID.String(), name: template.Name, title: template.Title, group: URLTemplatesPart})
			for _, method := range template.Methods {
				parts = append(parts, objectPart{key: method.ID.String(), name: method.Name, title: method.Title,
					group: MethodsPart, parent: template.ID.String()})
			}
		}
		return parts
	}},
	CommonPictureKind:             {},
	StyleItemKind:                 {},
	StyleKind:                     {},
	FunctionalOptionKind:          {},
	FunctionalOptionParameterKind: {},
	SettingsStorageKind:           {files: declaredFilesOf},
	// The fields of a criterion are what it searches by, not parts of its
	// own: only its commands are.
	FilterCriterionKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.FilterCriteria[c.filterCriterionByID[id]].Commands)
	}, files: declaredFilesOf, rightsFolders: partFolders(CommandsPart)},
	DefinedTypeKind: {types: func(c *Catalog, id uuid.UUID) []Type {
		return c.DefinedTypes[c.definedTypeByID[id]].Types
	}},
	LanguageKind: {},
	CatalogKind: withQueryTable(referenceKind(CatalogKind, referenceFolders), func(c *Catalog, id uuid.UUID) referenceTable {
		definition := c.Catalogs[c.catalogByID[id]]
		return referenceTable{hierarchy: definition.Hierarchy, owned: len(definition.Owners) > 0,
			codeLength: definition.Code.Length, descriptionLength: definition.DescriptionLength}
	}),
	ChartOfCharacteristicTypesKind: withQueryTable(referenceKind(ChartOfCharacteristicTypesKind, referenceFolders), func(c *Catalog, id uuid.UUID) referenceTable {
		definition := c.ChartsOfCharacteristicTypes[c.chartOfCharacteristicTypesByID[id]]
		return referenceTable{hierarchy: definition.Hierarchy, codeLength: definition.Code.Length, descriptionLength: definition.DescriptionLength}
	}),
	ChartOfAccountsKind: withQueryTable(referenceKind(ChartOfAccountsKind, partFolders(AttributesPart, AccountingFlagsPart, ExtDimensionAccountingFlagsPart,
		TablePartsPart, StandardAttributesPart, StandardTablePartsPart, CommandsPart)), func(c *Catalog, id uuid.UUID) referenceTable {
		definition := c.ChartsOfAccounts[c.chartOfAccountsByID[id]]
		return referenceTable{hierarchy: accountsHierarchy, codeLength: definition.Code.Length, descriptionLength: definition.DescriptionLength}
	}),
	ChartOfCalculationTypesKind: withQueryTable(referenceKind(ChartOfCalculationTypesKind, partFolders(AttributesPart, TablePartsPart, StandardAttributesPart,
		StandardTablePartsPart, CommandsPart)), func(c *Catalog, id uuid.UUID) referenceTable {
		definition := c.ChartsOfCalculationTypes[c.chartOfCalculationTypesByID[id]]
		return referenceTable{codeLength: definition.Code.Length, descriptionLength: definition.DescriptionLength,
			actionPeriod: definition.ActionPeriodUse, base: takesABase(definition)}
	}),
	// The addressing attributes of a task are no fields of its own sort:
	// they stand between its attributes and its table parts.
	TaskKind: {dataVersion: true, branches: []PartGroup{AddressingAttributesPart, AttributesPart, TablePartsPart},
		rightsFolders: partFolders(AddressingAttributesPart, AttributesPart, TablePartsPart, StandardAttributesPart, CommandsPart), parts: func(c *Catalog, id uuid.UUID) []objectPart {
			item := c.Tasks[c.taskByID[id]]
			parts := declaredParts(item.Attributes, nil, nil)
			for _, attribute := range addressingAsAttributes(item) {
				parts = append(parts, fieldPart(&attribute, AddressingAttributesPart, ""))
			}
			return append(parts, declaredParts(nil, item.TableParts, item.Commands)...)
		}, files: declaredFilesOf},
	AccountingRegisterKind: {described: true, files: declaredFilesOf, rightsFolders: registerFolders},
	// The recalculations stand after the branches of parts, each with its
	// dimensions right under it, and the branch stands even when there are
	// none (snapshots derevo/05, 05b).
	CalculationRegisterKind: {dataVersion: true, described: true, files: declaredFilesOf, rightsFolders: []rightsFolder{{group: DimensionsPart},
		{group: ResourcesPart}, {group: AttributesPart}, {group: StandardAttributesPart}, {kind: RecalculationKind}, {group: CommandsPart}},
		held: []heldList{objectsHeld("recalculations", RecalculationKind, func(c *Catalog, id uuid.UUID) []heldIdentity {
			return identitiesOf("", c.CalculationRegisters[c.calculationRegisterByID[id]].Recalculations)
		})}, beneath: func(c *Catalog, id uuid.UUID) map[uuid.UUID][]BranchPart {
			result := map[uuid.UUID][]BranchPart{}
			for _, recalculation := range c.CalculationRegisters[c.calculationRegisterByID[id]].Recalculations {
				for _, dimension := range recalculation.Dimensions {
					result[recalculation.ID] = append(result[recalculation.ID], BranchPart{ID: dimension.ID, Name: dimension.Name, Icon: partGroupOf(DimensionsPart).icon})
				}
			}
			return result
		}},
	RecalculationKind: {identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		recalculation, _ := c.recalculationOf(id)
		return identitiesOf("dimension", recalculation.Dimensions)
	}},
	ReportKind:        {described: true, files: declaredFilesOf, rightsFolders: reportFolders},
	DataProcessorKind: {described: true, files: declaredFilesOf, rightsFolders: reportFolders},
	NumeratorKind:     {},
	SequenceKind: {identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("dimension", c.Sequences[c.sequenceByID[id]].Dimensions)
	}, files: declaredFilesOf},
	DocumentJournalKind: {described: true, held: []heldList{{key: "columns", title: "Графы", icon: "journal-column", items: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("column", c.DocumentJournals[c.documentJournalByID[id]].Columns)
	}}}, files: declaredFilesOf, rightsFolders: partFolders(StandardAttributesPart, CommandsPart)},
	ExchangePlanKind: {dataVersion: true, described: true, files: declaredFilesOf, rightsFolders: referenceFolders},
	BusinessProcessKind: {dataVersion: true, described: true, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("route point", c.BusinessProcesses[c.businessProcessByID[id]].Route.Points)
	}, rightsFolders: referenceFolders, files: func(c *Catalog, id uuid.UUID) objectFiles {
		// The route map keeps the pictures its items draw in a folder of
		// the process.
		files := declaredFilesOf(c, id)
		files.collections = []string{project.RouteDirectory}
		files.route = &c.BusinessProcesses[c.businessProcessByID[id]].Route
		return files
	}},
	DocumentKind: {dataVersion: true, described: true, files: declaredFilesOf, rightsFolders: referenceFolders},
	InformationRegisterKind: {described: true, files: declaredFilesOf,
		rightsFolders: partFolders(DimensionsPart, ResourcesPart, StandardAttributesPart, AttributesPart, CommandsPart)},
	AccumulationRegisterKind: {described: true, files: declaredFilesOf, rightsFolders: registerFolders},
}

// referenceKind is the composition of a kind of reference with predefined
// items - a catalog, the three charts: its parts read off its description,
// the version of its data, its predefined items and the files it declares,
// with the folders of its tree of rights.
func referenceKind(kind Kind, folders []rightsFolder) kindComposition {
	return kindComposition{dataVersion: true, described: true, rightsFolders: folders, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		description, _ := descriptionOf(c, kind, id)
		return identitiesIn("predefined item", description.FieldByName("Predefined"))
	}, files: declaredFilesOf}
}

// withQueryTable gives a composition what its objects decide of the table a
// query reads of them.
func withQueryTable(composition kindComposition, queryTable func(*Catalog, uuid.UUID) referenceTable) kindComposition {
	composition.queryTable = queryTable
	return composition
}

// referenceTable is what the settings of a catalog or one of the three
// charts decide of the table a query reads of it, as the window of a
// restriction lists its fields (passes of 22.10.2026,
// block-00/vse-ogranicheniya-dostupa.md, the first, the second and the
// fourth; ACCESS-RIGHTS.md, «Состав и порядок полей ограничения»). The help
// of the tables says the parent is there only in a multi-level object, the
// owner only in a subordinate catalog, БазовыеВидыРасчета only in a chart of
// calculation types depending on a base and ВытесняющиеВидыРасчета only in
// one using the period of action (Справочник.<Имя справочника>,
// ПланВидовХарактеристик.<Имя плана видов характеристик>, ПланВидовРасчета.
// <Имя плана видов расчета>); the window shows ЭтоГруппа only with a
// hierarchy of folders and items, as the table keeps the flag of a folder
// (appendHierarchyColumns), the code and the description only where they
// have a length, and ПериодДействияБазовый only with the period of action
// (pola-23, pola-24).
type referenceTable struct {
	hierarchy Hierarchy
	// owned is a catalog subordinate to others.
	owned                         bool
	codeLength, descriptionLength int
	// actionPeriod and base are a chart of calculation types using the
	// period of action and depending on a base; no other kind has the
	// fields they keep.
	actionPeriod, base bool
}

// referenceTableOrder is the window's order of the standard fields of the
// four kinds: the reference, the version of the data, the mark of
// deletion, the owner, the parent, ЭтоГруппа, the code, the description,
// Порядок, Вид and Забалансовый of a chart of accounts, ПериодДействияБазовый
// and the three lists of a chart of calculation types, then the rest in the
// order the object keeps them - ТипЗначения of a chart of characteristic
// types, the last of its standard attributes, before what the object
// declares with the table parts, ВидыСубконто of a chart of accounts after
// them -, and last the predefined and its name. A field of a standard table
// part stands where its part does.
var referenceTableOrder = map[string]int{"ref": 0, "dataversion": 1, "deletionmark": 2, "owner": 3, "parent": 4, "isfolder": 5,
	"code": 6, "description": 7, "order": 8, "type": 9, "offbalance": 10, "actionperiodisbasic": 11,
	"basecalculationtypes": 12, "leadingcalculationtypes": 13, "displacingcalculationtypes": 14,
	"predefined": 16, "predefineddataname": 17}

// declaredOrder is the place of every field referenceTableOrder does not
// name.
const declaredOrder = 15

// standardPartFieldsOrder is the window's order of the fields of a standard
// table part: the line number, the kind of ext dimension or of calculation,
// the predefined flag, ТолькоОбороты, then the accounting flags of the ext
// dimensions (pass of 22.10.2026, the fourth, pola-16, pola-20) - not the
// order the prototype describes them in, which standardTablePartsOfKind
// keeps.
var standardPartFieldsOrder = map[string]int{"linenumber": 0, "extdimensiontype": 1, "calculationtype": 1, "predefined": 2,
	"turnoversonly": 3}

// fieldsOf is the fields of the table, of all the object keeps, in the
// window's order. The accounting flags of the ext dimensions are fields of
// the table of the kinds of ext dimensions (help, ПланСчетов.<Имя плана
// счетов>: ВидыСубконто holds <Признак учета доп. измерения>), and the
// window shows them there, after its own, not among the account's
// (snapshot pola-12).
func (table referenceTable) fieldsOf(fields []objectPart) []objectPart {
	absent := map[string]bool{
		"parent": !table.hierarchy.Enabled, "isfolder": !table.hierarchy.Enabled || table.hierarchy.Kind != FoldersAndItemsHierarchy,
		"owner": !table.owned, "code": table.codeLength == 0, "description": table.descriptionLength == 0,
		"actionperiodisbasic": !table.actionPeriod, "displacingcalculationtypes": !table.actionPeriod,
		"basecalculationtypes": !table.base,
	}
	// A standard field or part is ranked by its key, a field of a standard
	// part by its part's; what the object declares is keyed by identifiers
	// and ranked as declared.
	standard := func(field objectPart) bool {
		return field.group == StandardAttributesPart || field.group == StandardTablePartsPart
	}
	keyOf := func(field objectPart) string {
		if field.parent != "" {
			return field.parent
		}
		return field.key
	}
	rankOf := func(field objectPart) int {
		if order, ok := referenceTableOrder[keyOf(field)]; ok && standard(field) {
			return order
		}
		return declaredOrder
	}
	result := slices.DeleteFunc(slices.Clone(fields), func(field objectPart) bool { return standard(field) && absent[keyOf(field)] })
	extDimensions := strings.ToLower(extDimensionTypesPart.en)
	var flags []objectPart
	result = slices.DeleteFunc(result, func(field objectPart) bool {
		if field.group != ExtDimensionAccountingFlagsPart {
			return false
		}
		field.parent = extDimensions
		flags = append(flags, field)
		return true
	})
	slices.SortStableFunc(result, func(a, b objectPart) int { return rankOf(a) - rankOf(b) })
	for part := range result {
		if result[part].group != StandardTablePartsPart {
			continue
		}
		key := result[part].key
		last := part
		for last+1 < len(result) && result[last+1].parent == key {
			last++
		}
		slices.SortStableFunc(result[part+1:last+1], func(a, b objectPart) int {
			return standardPartFieldsOrder[strings.TrimPrefix(a.key, key+".")] - standardPartFieldsOrder[strings.TrimPrefix(b.key, key+".")]
		})
		if key == extDimensions {
			result = slices.Insert(result, last+1, flags...)
		}
	}
	return result
}

// fieldPart is a field of an object or of its table part.
func fieldPart(attribute *Attribute, group PartGroup, parent string) objectPart {
	return objectPart{key: attribute.ID.String(), name: attribute.Name, title: attribute.Title, group: group,
		parent: parent, attribute: attribute}
}

// declaredParts is the attributes, the table parts with their attributes and
// the commands of an object, in that order.
func declaredParts(attributes []Attribute, tableParts []TablePart, commands []ObjectCommand) []objectPart {
	var parts []objectPart
	for index := range attributes {
		parts = append(parts, fieldPart(&attributes[index], AttributesPart, ""))
	}
	for _, part := range tableParts {
		parts = append(parts, objectPart{key: part.ID.String(), name: part.Name, title: part.Title, group: TablePartsPart})
		for index := range part.Attributes {
			parts = append(parts, fieldPart(&part.Attributes[index], AttributesPart, part.ID.String()))
		}
	}
	for _, command := range commands {
		parts = append(parts, objectPart{key: command.ID.String(), name: command.Name, title: command.Title, group: CommandsPart})
	}
	return parts
}

// partsOf is the one walk of what an object holds: its standard attributes,
// what it declares, and the standard table parts of its kind with their
// fields. The second result is false when the configuration has no such
// object.
func (catalog *Catalog) partsOf(kind Kind, id uuid.UUID) ([]objectPart, bool) {
	if !catalog.hasObject(kind, id) {
		return nil, false
	}
	language := catalog.Project.DefaultLanguage
	standard := func(field standardField, key, parent string, group PartGroup) objectPart {
		return objectPart{key: key, name: field.ru, english: field.en, title: LocalizedText{language: nameInWords(field.ru)}, group: group, parent: parent}
	}
	var parts []objectPart
	for _, field := range catalog.standardAttributeFields(kind, id) {
		parts = append(parts, standard(field, strings.ToLower(field.en), "", StandardAttributesPart))
	}
	if composition := kindCompositionOf[kind]; composition.described {
		parts = append(parts, describedParts(catalog, kind, id)...)
	} else if composition.parts != nil {
		parts = append(parts, composition.parts(catalog, id)...)
	}
	for _, part := range standardTablePartsOfKind(kind) {
		key := strings.ToLower(part.en)
		parts = append(parts, standard(standardField{part.ru, part.en}, key, "", StandardTablePartsPart))
		for _, field := range part.fields {
			parts = append(parts, standard(field, key+"."+strings.ToLower(field.en), key, StandardAttributesPart))
		}
	}
	return parts, true
}

// partWord is what one part of the group is called in a message: an
// attribute, a dimension, a table part.
func partWord(group PartGroup) string {
	return strings.ReplaceAll(strings.TrimSuffix(string(group), "s"), "-", " ")
}

// heldIdentities is everything of an object with an identifier of its own:
// the parts the one walk of its composition finds, the items of its held
// lists named as its own, and the rest (kindComposition.identities). A common
// attribute taken into the object is the common attribute's, not the
// object's.
func (catalog *Catalog) heldIdentities(kind Kind, id uuid.UUID) []heldIdentity {
	parts, _ := catalog.partsOf(kind, id)
	names := make(map[string]string, len(parts))
	var result []heldIdentity
	for _, part := range parts {
		names[part.key] = part.name
		partID, err := uuid.Parse(part.key)
		if err != nil {
			continue
		}
		name := part.name
		if part.parent != "" {
			name = names[part.parent] + "." + name
		} else if _, common := catalog.commonAttributeByID[partID]; common {
			continue
		}
		result = append(result, heldIdentity{id: partID, word: partWord(part.group), name: name})
	}
	composition := kindCompositionOf[kind]
	for _, list := range composition.held {
		for _, item := range list.items(catalog, id) {
			if item.word != "" {
				result = append(result, item)
			}
		}
	}
	if composition.identities != nil {
		result = append(result, composition.identities(catalog, id)...)
	}
	return result
}

// dataVersionField is the version of an object's data: a field of the table
// a query reads where the kind keeps one (kindComposition.dataVersion), and
// not a standard attribute - no description of the prototype describes it.
var dataVersionField = standardField{"ВерсияДанных", "DataVersion"}

// restrictionFieldsOf is what a restriction of reading the object may name:
// the fields of the table a query reads (help, «Ограничение доступа к данным»;
// the tables of the query language), as the object has them
// (kindComposition.queryTable), in the order the window of a restriction
// lists them.
func (catalog *Catalog) restrictionFieldsOf(kind Kind, id uuid.UUID) []objectPart {
	fields := catalog.objectFieldsOf(kind, id)
	if queryTable := kindCompositionOf[kind].queryTable; queryTable != nil && catalog.hasObject(kind, id) {
		return queryTable(catalog, id).fieldsOf(fields)
	}
	return fields
}

// absentRestrictionFieldsOf is what the object keeps and the table a query
// reads of it has not by the object's settings now - the parent of a
// catalog with no hierarchy, the code of length 0 -, in the order of what
// it keeps. A restriction may still name one, as the prototype keeps it
// when the setting changes (ACCESS-RIGHTS.md, «Ограничение по полю,
// которого у объекта нет»): its list names it, its window does not show it.
func (catalog *Catalog) absentRestrictionFieldsOf(kind Kind, id uuid.UUID) []objectPart {
	present := map[string]bool{}
	for _, field := range catalog.restrictionFieldsOf(kind, id) {
		present[field.key] = true
	}
	return slices.DeleteFunc(catalog.objectFieldsOf(kind, id), func(field objectPart) bool { return present[field.key] })
}

// objectFieldsOf is every field an object of the kind keeps: every part
// keeping a value - the standard attributes, what the object declares, the
// standard table parts and their fields - the version of the data where the
// kind has one, and the line number of every table part, which the prototype
// writes as ОС.LineNumber. Commands, operations and methods keep no value.
func (catalog *Catalog) objectFieldsOf(kind Kind, id uuid.UUID) []objectPart {
	parts, _ := catalog.partsOf(kind, id)
	language := catalog.Project.DefaultLanguage
	var fields []objectPart
	if kindCompositionOf[kind].dataVersion {
		fields = append(fields, standardFieldPart(dataVersionField, strings.ToLower(dataVersionField.en), "", language))
	}
	return append(fields, valueFields(parts, language)...)
}

// standardFieldPart is a field the platform gives, under its key.
func standardFieldPart(field standardField, key, parent, language string) objectPart {
	return objectPart{key: key, name: field.ru, english: field.en, title: LocalizedText{language: nameInWords(field.ru)},
		group: StandardAttributesPart, parent: parent}
}

// valueFields is the parts that keep a value, each table part followed by
// its line number.
func valueFields(parts []objectPart, language string) []objectPart {
	var fields []objectPart
	for _, part := range parts {
		switch part.group {
		case CommandsPart, OperationsPart, URLTemplatesPart, MethodsPart:
			continue
		}
		fields = append(fields, part)
		if part.group == TablePartsPart {
			for _, field := range tablePartStandardFields {
				fields = append(fields, standardFieldPart(field, part.key+"."+strings.ToLower(field.en), part.key, language))
			}
		}
	}
	return fields
}

// DeclaredFieldKeys is the keys a restriction of a role names the fields an
// object declares by (RightRestriction.Fields): its attributes, its table
// parts, their attributes and line numbers.
func DeclaredFieldKeys(attributes []Attribute, tableParts []TablePart) []string {
	var keys []string
	for _, field := range valueFields(declaredParts(attributes, tableParts, nil), "") {
		keys = append(keys, field.key)
	}
	return keys
}

// branchesOf is the sorts of part an object of the kind shows a branch of
// under it in the configuration tree, in the order of the tree: for a kind
// read off its description, every sort whose field its description keeps and
// the tree shows (inventory of block 00, «Ветки под объектом», snapshots
// derevo/); for another, what its composition says. A branch stands whether
// it holds anything or not, as in the prototype's tree. The commands of an
// object are no branch read off its description: the tree shows them from
// their folders (studio, objectCommandNodes).
func branchesOf(kind Kind) []PartGroup {
	composition := kindCompositionOf[kind]
	if !composition.described {
		return composition.branches
	}
	info, _ := kindInfoOf(kind)
	var result []PartGroup
	for _, sort := range partGroups {
		if sort.group == CommandsPart || sort.field == "" || info.described == nil {
			continue
		}
		if _, ok := info.described.FieldByName(sort.field); ok {
			result = append(result, sort.group)
		}
	}
	return result
}

// ObjectBranch is one branch of the configuration tree under an object: its
// key, caption and icon, and what it holds. Group is the sort of part it
// holds, empty for a held list (kindComposition.held); Kind is the kind of
// the objects a held list holds, empty where they are the object's own. An
// inline branch has no node of its own: its parts stand right under the
// object.
type ObjectBranch struct {
	Key    string
	Group  PartGroup
	Title  string
	Icon   string
	Kind   Kind
	Inline bool
	Parts  []BranchPart
}

// ObjectModules are the modules an object of the kind keeps by its own
// description, in the order of its kind (kinds.Layout.Modules): a table of a
// source keeps the module of data of its type alone - an object table its
// object module, a table of no objects its record set module. A catalog that
// is not there - a project that does not load - or an object it does not
// have gives every module of the kind.
func (catalog *Catalog) ObjectModules(kind Kind, id uuid.UUID) []kinds.Module {
	modules := kinds.Must(kind).Layout().Modules
	if catalog == nil || !catalog.hasObject(kind, id) || kindCompositionOf[kind].files == nil {
		return modules
	}
	if kept := kindCompositionOf[kind].files(catalog, id).modules; kept != nil {
		return kept
	}
	return modules
}

// BranchPart is a part of an object in a branch, or an item of a held list,
// with the icon it is shown by and what lies in it: the fields of a table
// part, the methods of a template, the parameters of an operation, the
// dimensions of a recalculation.
type BranchPart struct {
	ID    uuid.UUID
	Name  string
	Icon  string
	Parts []BranchPart
}

// ObjectBranches is the branches of the configuration tree under an object of
// the kind, each with the parts the object declares of its sort, read by the
// one walk of what the object holds (partsOf), then the branches of its held
// lists. Everything stands in the order of the description, as in the
// prototype's tree (snapshots derevo/07, 11a, 11b: not by the alphabet). A
// common attribute taken into the object is the common attribute's, and is
// not among them. A catalog that is not there - a project that does not
// load - or an object it does not have gives every branch empty.
func (catalog *Catalog) ObjectBranches(kind Kind, id uuid.UUID) []ObjectBranch {
	composition := kindCompositionOf[kind]
	var parts []objectPart
	var beneath map[uuid.UUID][]BranchPart
	found := catalog != nil && catalog.hasObject(kind, id)
	if found {
		parts, _ = catalog.partsOf(kind, id)
		if composition.beneath != nil {
			beneath = composition.beneath(catalog, id)
		}
	}
	inside := map[string][]BranchPart{}
	for _, part := range parts {
		if partID, err := uuid.Parse(part.key); err == nil && part.parent != "" {
			inside[part.parent] = append(inside[part.parent], BranchPart{ID: partID, Name: part.name, Icon: partGroupOf(part.group).icon})
		}
	}
	var result []ObjectBranch
	for _, group := range branchesOf(kind) {
		info := partGroupOf(group)
		branch := ObjectBranch{Key: string(group), Group: group, Title: info.title, Icon: info.icon, Inline: composition.inline}
		for _, part := range parts {
			partID, err := uuid.Parse(part.key)
			if err != nil || part.group != group || part.parent != "" {
				continue
			}
			if _, common := catalog.commonAttributeByID[partID]; common {
				continue
			}
			branch.Parts = append(branch.Parts, BranchPart{ID: partID, Name: part.name, Icon: info.icon,
				Parts: append(inside[part.key], beneath[partID]...)})
		}
		result = append(result, branch)
	}
	for _, list := range composition.held {
		branch := ObjectBranch{Key: list.key, Title: list.title, Icon: list.icon, Kind: list.kind}
		if found {
			for _, item := range list.items(catalog, id) {
				branch.Parts = append(branch.Parts, BranchPart{ID: item.id, Name: item.name, Icon: list.icon, Parts: beneath[item.id]})
			}
		}
		result = append(result, branch)
	}
	return result
}
