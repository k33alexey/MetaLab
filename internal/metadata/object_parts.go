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
// folder, the rights the configurator offers on such a part, and the field of
// a description that lists the parts of the sort (describedParts) - empty for
// a sort the platform gives or a kind reads in its own way.
type partGroupInfo struct {
	group     PartGroup
	prototype string
	title     string
	rights    []PermissionOperation
	field     string
}

// partGroups lists every sort of part in the order the tree shows them. The
// rights are the exports': every Rights.xml of every role under docs/ (2526,
// survey of 12.10.2026) writes View and Edit on a field of any sort, View on
// a command and Use on an operation of a web service and a method of an HTTP
// service. A template of an HTTP service has no rights: the exports write
// them on its methods only.
var partGroups = []partGroupInfo{
	{StandardAttributesPart, "StandardAttribute", "Стандартные реквизиты", fieldRights, ""},
	{DimensionsPart, "Dimension", "Измерения", fieldRights, "Dimensions"},
	{ResourcesPart, "Resource", "Ресурсы", fieldRights, "Resources"},
	{AttributesPart, "Attribute", "Реквизиты", fieldRights, "Attributes"},
	{AddressingAttributesPart, "AddressingAttribute", "Реквизиты адресации", fieldRights, ""},
	{AccountingFlagsPart, "AccountingFlag", "Признаки учета", fieldRights, "AccountingFlags"},
	{ExtDimensionAccountingFlagsPart, "ExtDimensionAccountingFlag", "Признаки учета субконто", fieldRights, "ExtDimensionAccountingFlags"},
	{TablePartsPart, "TabularSection", "Табличные части", fieldRights, "TableParts"},
	{StandardTablePartsPart, "StandardTabularSection", "Стандартные табличные части", fieldRights, ""},
	{FieldsPart, "Field", "Поля", fieldRights, "Fields"},
	{CommandsPart, "Command", "Команды", []PermissionOperation{PermissionView}, "Commands"},
	{OperationsPart, "Operation", "Операции", []PermissionOperation{PermissionUse}, ""},
	{URLTemplatesPart, "URLTemplate", "Шаблоны URL", nil, ""},
	{MethodsPart, "Method", "Методы", []PermissionOperation{PermissionUse}, ""},
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
	// described is a kind whose objects declare their parts in the fields of
	// their description that partGroups names (describedParts); the standard
	// ones every object of the kind has are added by partsOf.
	described bool
	// parts lists the declared parts of an object of a kind that reads them
	// in its own way - a task, a criterion of filter, the services. Nil, and
	// not described, is a kind whose objects declare no parts.
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
	EnumerationKind: {described: true, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("value", c.Enumerations[c.enumerationByID[id]].Values)
	}, files: declaredFilesOf},
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
	ExternalDataSourceTableKind: {described: true, files: func(c *Catalog, id uuid.UUID) objectFiles {
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
	ExternalCubeKind: {described: true, files: declaredFilesOf},
	// A dimension table holds fields as a table of a source does (help:
	// ВнешнийИсточникДанныхКубТаблицаИзмеренияОбъект.<Имя поля>).
	ExternalDimensionTableKind: {described: true, files: declaredFilesOf},
	ExternalFunctionKind:       {},
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
	SettingsStorageKind:           {files: declaredFilesOf},
	// The fields of a criterion are what it searches by, not parts of its
	// own: only its commands are.
	FilterCriterionKind: {parts: func(c *Catalog, id uuid.UUID) []objectPart {
		return declaredParts(nil, nil, c.FilterCriteria[c.filterCriterionByID[id]].Commands)
	}, files: declaredFilesOf},
	DefinedTypeKind: {types: func(c *Catalog, id uuid.UUID) []Type {
		return c.DefinedTypes[c.definedTypeByID[id]].Types
	}},
	LanguageKind:                   {},
	CatalogKind:                    referenceKind(CatalogKind),
	ChartOfCharacteristicTypesKind: referenceKind(ChartOfCharacteristicTypesKind),
	ChartOfAccountsKind:            referenceKind(ChartOfAccountsKind),
	ChartOfCalculationTypesKind:    referenceKind(ChartOfCalculationTypesKind),
	// The addressing attributes of a task are no fields of its own sort:
	// they stand between its attributes and its table parts.
	TaskKind: {dataVersion: true, parts: func(c *Catalog, id uuid.UUID) []objectPart {
		item := c.Tasks[c.taskByID[id]]
		parts := declaredParts(item.Attributes, nil, nil)
		for _, attribute := range addressingAsAttributes(item) {
			parts = append(parts, fieldPart(&attribute, AddressingAttributesPart, ""))
		}
		return append(parts, declaredParts(nil, item.TableParts, item.Commands)...)
	}, files: declaredFilesOf},
	AccountingRegisterKind:  {described: true, files: declaredFilesOf},
	CalculationRegisterKind: {dataVersion: true, described: true, files: declaredFilesOf},
	RecalculationKind: {identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		recalculation, _ := c.recalculationOf(id)
		return identitiesOf("dimension", recalculation.Dimensions)
	}},
	ReportKind:        {described: true, files: declaredFilesOf},
	DataProcessorKind: {described: true, files: declaredFilesOf},
	NumeratorKind:     {},
	SequenceKind: {identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("dimension", c.Sequences[c.sequenceByID[id]].Dimensions)
	}, files: declaredFilesOf},
	DocumentJournalKind: {described: true, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("column", c.DocumentJournals[c.documentJournalByID[id]].Columns)
	}, files: declaredFilesOf},
	ExchangePlanKind: {dataVersion: true, described: true, files: declaredFilesOf},
	BusinessProcessKind: {dataVersion: true, described: true, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		return identitiesOf("route point", c.BusinessProcesses[c.businessProcessByID[id]].Route.Points)
	}, files: func(c *Catalog, id uuid.UUID) objectFiles {
		// The route map keeps the pictures its items draw in a folder of
		// the process.
		files := declaredFilesOf(c, id)
		files.collections = []string{project.RouteDirectory}
		files.route = &c.BusinessProcesses[c.businessProcessByID[id]].Route
		return files
	}},
	DocumentKind:             {dataVersion: true, described: true, files: declaredFilesOf},
	InformationRegisterKind:  {described: true, files: declaredFilesOf},
	AccumulationRegisterKind: {described: true, files: declaredFilesOf},
}

// referenceKind is the composition of a kind of reference with predefined
// items - a catalog, the three charts: its parts read off its description,
// the version of its data, its predefined items and the files it declares.
func referenceKind(kind Kind) kindComposition {
	return kindComposition{dataVersion: true, described: true, identities: func(c *Catalog, id uuid.UUID) []heldIdentity {
		description, _ := descriptionOf(c, kind, id)
		return identitiesIn("predefined item", description.FieldByName("Predefined"))
	}, files: declaredFilesOf}
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
		result = append(result, heldIdentity{id: partID, word: partWord(part.group), name: name})
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
