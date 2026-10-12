package metadata

import (
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

// objectPart is one part of an object a role may name: a field, a table part,
// a command, an operation, or a template that holds methods.
type objectPart struct {
	// key is the part's identifier, or for a standard one its prototype name
	// in lower case, and for a field of a standard table part the part's name
	// before it - the key a role keeps the part's rights under.
	key   string
	name  string
	title LocalizedText
	group PartGroup
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
	// named is a filter criterion, a characteristic table and the content of
	// the standalone configuration naming the object by what it holds - the
	// kinds objectElementsOf took before the one walk of composition. Which
	// kinds the content of the standalone configuration may name is open
	// (BLOCKS, «один обход состава объекта»).
	named bool
}

// kindCompositionOf is every kind's composition. A test requires every kind
// of the model to be here, so a new kind cannot be left without one.
var kindCompositionOf = map[Kind]kindComposition{
	RoleKind:              {named: true},
	SubsystemKind:         {named: true},
	ConstantKind:          {named: true},
	SessionParameterKind:  {named: true},
	CommonAttributeKind:   {named: true},
	CommonModuleKind:      {named: true},
	EventSubscriptionKind: {named: true},
	EnumerationKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.Enumerations[c.enumerationByID[id]].Commands)
	}},
	ScheduledJobKind:   {},
	CommandGroupKind:   {},
	CommonCommandKind:  {named: true},
	CommonFormKind:     {named: true},
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
	ExternalDataSourceTableKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		location := c.externalTableByID[id]
		table := c.ExternalDataSources[location.source].Tables[location.table]
		parts := make([]objectPart, 0, len(table.Fields)+len(table.Commands))
		for index := range table.Fields {
			parts = append(parts, fieldPart(&table.Fields[index].Attribute, FieldsPart, ""))
		}
		return append(parts, declaredParts(nil, nil, table.Commands)...)
	}},
	ExternalCubeKind:           {},
	ExternalDimensionTableKind: {},
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
	FilterCriterionKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.FilterCriteria[c.filterCriterionByID[id]].Commands)
	}},
	DefinedTypeKind: {named: true},
	LanguageKind:    {},
	CatalogKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Catalogs[c.catalogByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	ChartOfCharacteristicTypesKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.ChartsOfCharacteristicTypes[c.chartOfCharacteristicTypesByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	ChartOfAccountsKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.ChartsOfAccounts[c.chartOfAccountsByID[id]]
		parts := declaredParts(item.Attributes, nil, nil)
		for index := range item.AccountingFlags {
			parts = append(parts, fieldPart(&item.AccountingFlags[index].Attribute, AccountingFlagsPart, ""))
		}
		for index := range item.ExtDimensionAccountingFlags {
			parts = append(parts, fieldPart(&item.ExtDimensionAccountingFlags[index].Attribute, ExtDimensionAccountingFlagsPart, ""))
		}
		return append(parts, declaredParts(nil, item.TableParts, item.Commands)...)
	}},
	ChartOfCalculationTypesKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.ChartsOfCalculationTypes[c.chartOfCalculationTypesByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	TaskKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Tasks[c.taskByID[id]]
		parts := declaredParts(item.Attributes, nil, nil)
		for _, attribute := range addressingAsAttributes(item) {
			parts = append(parts, fieldPart(&attribute, AddressingAttributesPart, ""))
		}
		return append(parts, declaredParts(nil, item.TableParts, item.Commands)...)
	}},
	AccountingRegisterKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.AccountingRegisters[c.accountingRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), accountingResourceAttributes(item.Resources), item.Attributes, item.Commands)
	}},
	CalculationRegisterKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.CalculationRegisters[c.calculationRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), item.Resources, item.attributeFields(), item.Commands)
	}},
	RecalculationKind: {},
	ReportKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Reports[c.reportByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	DataProcessorKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.DataProcessors[c.dataProcessorByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	NumeratorKind: {named: true},
	SequenceKind:  {named: true},
	DocumentJournalKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.DocumentJournals[c.documentJournalByID[id]].Commands)
	}},
	ExchangePlanKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.ExchangePlans[c.exchangePlanByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	BusinessProcessKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.BusinessProcesses[c.businessProcessByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	DocumentKind: {named: true, dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Documents[c.documentByID[id]]
		return declaredParts(item.Attributes, item.TableParts, item.Commands)
	}},
	InformationRegisterKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.InformationRegisters[c.informationRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), item.Resources, item.Attributes, item.Commands)
	}},
	AccumulationRegisterKind: {named: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.AccumulationRegisters[c.accumulationRegisterByID[id]]
		return registerParts(RegisterDimensionAttributes(item.Dimensions), item.Resources, item.Attributes, item.Commands)
	}},
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
		return objectPart{key: key, name: field.ru, title: LocalizedText{language: nameInWords(field.ru)}, group: group, parent: parent}
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
		return objectPart{key: key, name: field.ru, title: LocalizedText{language: nameInWords(field.ru)},
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
