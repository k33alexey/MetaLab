package metadata

import (
	"reflect"
	"strings"

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
// folder, and the rights the configurator offers on such a part.
type partGroupInfo struct {
	group     PartGroup
	prototype string
	title     string
	rights    []PermissionOperation
}

// partGroups lists every sort of part in the order the tree shows them. The
// rights are the exports': every Rights.xml of every role under docs/ (2526,
// survey of 12.10.2026) writes View and Edit on a field of any sort, View on
// a command and Use on an operation of a web service and a method of an HTTP
// service. A template of an HTTP service has no rights: the exports write
// them on its methods only.
var partGroups = []partGroupInfo{
	{StandardAttributesPart, "StandardAttribute", "Стандартные реквизиты", fieldRights},
	{DimensionsPart, "Dimension", "Измерения", fieldRights},
	{ResourcesPart, "Resource", "Ресурсы", fieldRights},
	{AttributesPart, "Attribute", "Реквизиты", fieldRights},
	{AddressingAttributesPart, "AddressingAttribute", "Реквизиты адресации", fieldRights},
	{AccountingFlagsPart, "AccountingFlag", "Признаки учета", fieldRights},
	{ExtDimensionAccountingFlagsPart, "ExtDimensionAccountingFlag", "Признаки учета субконто", fieldRights},
	{TablePartsPart, "TabularSection", "Табличные части", fieldRights},
	{StandardTablePartsPart, "StandardTabularSection", "Стандартные табличные части", fieldRights},
	{FieldsPart, "Field", "Поля", fieldRights},
	{CommandsPart, "Command", "Команды", []PermissionOperation{PermissionView}},
	{OperationsPart, "Operation", "Операции", []PermissionOperation{PermissionUse}},
	{URLTemplatesPart, "URLTemplate", "Шаблоны URL", nil},
	{MethodsPart, "Method", "Методы", []PermissionOperation{PermissionUse}},
}

// fieldRights are a field's two rights, whatever the field is.
var fieldRights = []PermissionOperation{PermissionView, PermissionEdit}

// partRightsOf is the rights the configurator offers on a part of the group.
func partRightsOf(group PartGroup) []PermissionOperation {
	for _, item := range partGroups {
		if item.group == group {
			return item.rights
		}
	}
	return nil
}

// offeredPartRights is the rights the configurator offers on a part of the
// group in an object of the kind: those of the group where the exports give
// rights on the parts of the kind, and none where they never do - a right
// given there all the same is carried and noted (NoteRightWithoutSample).
func offeredPartRights(kind Kind, group PartGroup) []PermissionOperation {
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
	// parts lists the declared parts of the object, in the order of its
	// description; the standard ones every object of the kind has are added
	// by partsOf. Nil is a kind whose objects declare no parts.
	parts func(catalog *Catalog, id uuid.UUID) []objectPart
	// dataVersion is a kind whose objects keep the version of their data
	// (help, the tables of the query language; the table of a chart of
	// calculation types does not list it, its object - ПланВидовРасчетаОбъект
	// - has it).
	dataVersion bool
	// identities is what else of the object has an identifier of its own and
	// is no part a role may name; nil where there is nothing.
	identities func(catalog *Catalog, id uuid.UUID) []heldIdentity
	// valueFields are the fields the manager of the value holds beside the
	// value, for a kind whose object is one value and no parts: a constant's
	// common attributes (help: КонстантаМенеджерЗначения.<Имя>.<Имя общего
	// реквизита>). Nil where there are none.
	valueFields func(catalog *Catalog, id uuid.UUID) []Attribute
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
	result := make([]heldIdentity, 0, len(items))
	for index := range items {
		item := reflect.ValueOf(&items[index]).Elem()
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
	}},
	SessionParameterKind:  {},
	CommonAttributeKind:   {},
	CommonModuleKind:      {},
	EventSubscriptionKind: {},
	EnumerationKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.Enumerations[c.enumerationByID[id]].Commands)
	}, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("value", c.Enumerations[c.enumerationByID[id]].Values)
	}},
	ScheduledJobKind:   {},
	CommandGroupKind:   {},
	CommonCommandKind:  {},
	CommonFormKind:     {},
	CommonTemplateKind: {},
	XDTOPackageKind:    {},
	WebServiceKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		var parts []objectPart
		for _, operation := range c.WebServices[c.webServiceByID[id]].Operations {
			parts = append(parts, objectPart{key: operation.ID.String(), name: operation.Name, title: operation.Title, group: OperationsPart})
		}
		return parts
	}},
	WSReferenceKind:        {},
	WebSocketClientKind:    {},
	ExternalDataSourceKind: {},
	ExternalDataSourceTableKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		location := c.externalTableByID[id]
		table := c.ExternalDataSources[location.source].Tables[location.table]
		return externalFieldParts(table.Fields, table.Commands)
	}},
	// A cube is read as a register is: its dimensions and resources are the
	// fields of its records (help: ВнешнийИсточникДанныхКубЗапись.<Имя
	// измерения>, <Имя ресурса>).
	ExternalCubeKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		location := c.externalCubeByID[id]
		cube := c.ExternalDataSources[location.source].Cubes[location.cube]
		return registerParts(cube.dimensionAttributes(), cube.resourceAttributes(), nil, cube.Commands)
	}},
	// A dimension table holds fields as a table of a source does (help:
	// ВнешнийИсточникДанныхКубТаблицаИзмеренияОбъект.<Имя поля>).
	ExternalDimensionTableKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		location := c.externalDimensionTableByID[id]
		table := c.ExternalDataSources[location.source].Cubes[location.cube].DimensionTables[location.table]
		return externalFieldParts(table.Fields, table.Commands)
	}},
	ExternalFunctionKind: {},
	HTTPServiceKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
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
	SettingsStorageKind:           {},
	FilterCriterionKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.FilterCriteria[c.filterCriterionByID[id]].Commands)
	}},
	DefinedTypeKind: {},
	LanguageKind:    {},
	CatalogKind: referenceKind(func(c *Catalog, id uuid.UUID) ([]Attribute, []TablePart, []ObjectCommand, []PredefinedCatalogItem) {
		item := c.Catalogs[c.catalogByID[id]]
		return item.Attributes, item.TableParts, item.Commands, item.Predefined
	}),
	ChartOfCharacteristicTypesKind: referenceKind(func(c *Catalog, id uuid.UUID) ([]Attribute, []TablePart, []ObjectCommand, []PredefinedCharacteristic) {
		item := c.ChartsOfCharacteristicTypes[c.chartOfCharacteristicTypesByID[id]]
		return item.Attributes, item.TableParts, item.Commands, item.Predefined
	}),
	ChartOfAccountsKind: {dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.ChartsOfAccounts[c.chartOfAccountsByID[id]]
		parts := declaredParts(item.Attributes, nil, nil)
		for index := range item.AccountingFlags {
			parts = append(parts, fieldPart(&item.AccountingFlags[index].Attribute, AccountingFlagsPart, ""))
		}
		for index := range item.ExtDimensionAccountingFlags {
			parts = append(parts, fieldPart(&item.ExtDimensionAccountingFlags[index].Attribute, ExtDimensionAccountingFlagsPart, ""))
		}
		return append(parts, declaredParts(nil, item.TableParts, item.Commands)...)
	}, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("predefined item", c.ChartsOfAccounts[c.chartOfAccountsByID[id]].Predefined)
	}},
	ChartOfCalculationTypesKind: referenceKind(func(c *Catalog, id uuid.UUID) ([]Attribute, []TablePart, []ObjectCommand, []PredefinedCalculationType) {
		item := c.ChartsOfCalculationTypes[c.chartOfCalculationTypesByID[id]]
		return item.Attributes, item.TableParts, item.Commands, item.Predefined
	}),
	TaskKind: {dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Tasks[c.taskByID[id]]
		parts := declaredParts(item.Attributes, nil, nil)
		for _, attribute := range addressingAsAttributes(item) {
			parts = append(parts, fieldPart(&attribute, AddressingAttributesPart, ""))
		}
		return append(parts, declaredParts(nil, item.TableParts, item.Commands)...)
	}},
	AccountingRegisterKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.AccountingRegisters[c.accountingRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), accountingResourceAttributes(item.Resources), item.Attributes, item.Commands)
	}},
	CalculationRegisterKind: {dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.CalculationRegisters[c.calculationRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), item.Resources, item.attributeFields(), item.Commands)
	}},
	RecalculationKind: {identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		recalculation, _ := c.recalculationOf(id)
		return identitiesOf("dimension", recalculation.Dimensions)
	}},
	ReportKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Reports[c.reportByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	DataProcessorKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.DataProcessors[c.dataProcessorByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	NumeratorKind: {},
	SequenceKind: {identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("dimension", c.Sequences[c.sequenceByID[id]].Dimensions)
	}},
	DocumentJournalKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.DocumentJournals[c.documentJournalByID[id]].Commands)
	}, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("column", c.DocumentJournals[c.documentJournalByID[id]].Columns)
	}},
	ExchangePlanKind: {dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.ExchangePlans[c.exchangePlanByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	BusinessProcessKind: {dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.BusinessProcesses[c.businessProcessByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("route point", c.BusinessProcesses[c.businessProcessByID[id]].Route.Points)
	}},
	DocumentKind: {dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Documents[c.documentByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	InformationRegisterKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.InformationRegisters[c.informationRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), item.Resources, item.Attributes, item.Commands)
	}},
	AccumulationRegisterKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.AccumulationRegisters[c.accumulationRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), item.Resources, item.Attributes, item.Commands)
	}},
}

// referenceKind is the composition of a kind whose objects declare
// attributes, table parts and commands and have predefined items, read by
// object.
func referenceKind[P any](object func(c *Catalog, id uuid.UUID) ([]Attribute, []TablePart, []ObjectCommand, []P)) kindComposition {
	return kindComposition{dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		attributes, tableParts, commands, _ := object(c, id)
		return declaredParts(attributes, tableParts, commands)
	}, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		_, _, _, predefined := object(c, id)
		return identitiesOf("predefined item", predefined)
	}}
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

// externalFieldParts is the fields and the commands of a table of a source or
// of a cube's dimension.
func externalFieldParts(fields []ExternalField, commands []ObjectCommand) []objectPart {
	parts := make([]objectPart, 0, len(fields)+len(commands))
	for index := range fields {
		parts = append(parts, fieldPart(&fields[index].Attribute, FieldsPart, ""))
	}
	return append(parts, declaredParts(nil, nil, commands)...)
}

// registerParts is the dimensions, the resources, the attributes and the
// commands of a register.
func registerParts(dimensions, resources, attributes []Attribute, commands []ObjectCommand) []objectPart {
	var parts []objectPart
	for index := range dimensions {
		parts = append(parts, fieldPart(&dimensions[index], DimensionsPart, ""))
	}
	for index := range resources {
		parts = append(parts, fieldPart(&resources[index], ResourcesPart, ""))
	}
	return append(parts, declaredParts(attributes, nil, commands)...)
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
	if declared := kindCompositionOf[kind].parts; declared != nil {
		parts = append(parts, declared(catalog, id)...)
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

// heldIdentities is everything of an object with an identifier of its own:
// the parts the one walk of its composition finds, and the rest
// (kindComposition.identities). A common attribute taken into the object is
// the common attribute's, not the object's.
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
		word := strings.ReplaceAll(strings.TrimSuffix(string(part.group), "s"), "-", " ")
		result = append(result, heldIdentity{id: partID, word: word, name: name})
	}
	if more := kindCompositionOf[kind].identities; more != nil {
		result = append(result, more(catalog, id)...)
	}
	return result
}

// dataVersionField is the version of an object's data: a field of the table
// a query reads where the kind keeps one (kindComposition.dataVersion), and
// not a standard attribute - no description of the prototype describes it.
var dataVersionField = standardField{"ВерсияДанных", "DataVersion"}

// restrictionFieldsOf is what a restriction of reading the object may name:
// the fields of the table a query reads (help, «Ограничение доступа к данным»;
// the tables of the query language). That is every part keeping a value - the
// standard attributes, what the object declares, the standard table parts and
// their fields - the version of the data where the kind has one, and the line
// number of every table part, which the prototype writes as ОС.LineNumber.
// Commands, operations and methods keep no value.
func (catalog *Catalog) restrictionFieldsOf(kind Kind, id uuid.UUID) []objectPart {
	parts, _ := catalog.partsOf(kind, id)
	language := catalog.Project.DefaultLanguage
	standard := func(field standardField, key, parent string) objectPart {
		return objectPart{key: key, name: field.ru, english: field.en, title: LocalizedText{language: nameInWords(field.ru)},
			group: StandardAttributesPart, parent: parent}
	}
	var fields []objectPart
	if kindCompositionOf[kind].dataVersion {
		fields = append(fields, standard(dataVersionField, strings.ToLower(dataVersionField.en), ""))
	}
	for _, part := range parts {
		switch part.group {
		case CommandsPart, OperationsPart, URLTemplatesPart, MethodsPart:
			continue
		}
		fields = append(fields, part)
		if part.group == TablePartsPart {
			for _, field := range tablePartStandardFields {
				fields = append(fields, standard(field, part.key+"."+strings.ToLower(field.en), part.key))
			}
		}
	}
	return fields
}
