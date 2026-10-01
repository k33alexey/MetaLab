package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TaskNumberPrefix says where the start of a task number comes from.
type TaskNumberPrefix string

const (
	NoNumberPrefix              TaskNumberPrefix = "none"
	BusinessProcessNumberPrefix TaskNumberPrefix = "business-process-number"
)

// AddressingAttribute is what a task is addressed by: a role, a department, an
// object - whatever the application decided stands between a task and the
// people who may do it.
//
// Each of them is bound to a dimension of the addressing register: that binding
// is the whole mechanism, because it is how the platform turns "this task is
// for the role of accountant" into the people who hold that role.
//
// It is a field, and it carries a field's whole palette. A name, a synonym, a
// type and the dimension were all we had; the export writes twenty-nine
// properties on an addressing attribute - everything a catalog attribute has,
// plus the dimension - and «ОбъектМетаданных: РеквизитАдресации» of the syntax
// assistant lists the same set. Only «использование» is missing on both counts,
// and it is refused here: it divides items from folders, and a task has neither.
type AddressingAttribute struct {
	Attribute `yaml:",inline" json:",inline"`
	// Dimension is the dimension of the addressing register this attribute is
	// matched against.
	Dimension *uuid.UUID `yaml:"dimension,omitempty" json:"dimension,omitempty"`
}

// TaskDefinition describes assignments to people, addressed by role rather than
// by name.
type TaskDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Presentations is how this object is named to the person using it -
	// see object_presentation.go.
	Presentations `yaml:",inline" json:",inline"` // A task is edited and picked like a reference object, but it has no code:
	// its presentation is the number or the description - see object_choice.go.
	EditType            EditType         `yaml:"edit_type,omitempty" json:"editType,omitempty"`
	DefaultPresentation TaskPresentation `yaml:"default_presentation,omitempty" json:"defaultPresentation,omitempty"`
	ObjectInput         `yaml:",inline" json:",inline"`
	// BasedOn are the objects one of these may be made out of, the list the
	// command to make it offers - see based_on.go.
	BasedOn []uuid.UUID `yaml:"based_on,omitempty" json:"basedOn,omitempty"`
	// DataLock is how the platform locks a row of this object while it is
	// written, and DataLockFields are the fields it may be locked by - see
	// data_lock_settings.go.
	DataLock       project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	DataLockFields []ObjectField               `yaml:"data_lock_fields,omitempty" json:"dataLockFields,omitempty"`
	// FullTextSearch is whether this object is in the full-text index at all -
	// see full_text_search.go.
	FullTextSearch FullTextSearchMode `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	// DataHistorySettings is whether this object takes part in data history
	// and the two flags that go with it - see data_history.go.
	DataHistorySettings `yaml:",inline" json:",inline"`
	// AdditionalIndexes are the indexes this object asks the database for
	// beside the ones the platform builds - see additional_indexes.go.
	AdditionalIndexes []AdditionalIndex `yaml:"additional_indexes,omitempty" json:"additionalIndexes,omitempty"`

	Number            DocumentNumber `yaml:"number" json:"number"`
	DescriptionLength int            `yaml:"description_length" json:"descriptionLength"`
	// NumberPrefix takes the start of the number from the business process that
	// created the task, so the tasks of one process read as one process.
	NumberPrefix TaskNumberPrefix `yaml:"number_prefix,omitempty" json:"numberPrefix,omitempty"`
	// Addressing is the information register that matches addressing attributes
	// to the people who may execute the task.
	Addressing           *uuid.UUID            `yaml:"addressing,omitempty" json:"addressing,omitempty"`
	AddressingAttributes []AddressingAttribute `yaml:"addressing_attributes,omitempty" json:"addressingAttributes,omitempty"`
	// MainAddressingAttribute names the attribute that holds the performer
	// themselves - the one a list of "my tasks" is built on.
	MainAddressingAttribute string `yaml:"main_addressing_attribute,omitempty" json:"mainAddressingAttribute,omitempty"`
	// CurrentPerformer is the session parameter the platform reads to know
	// whose tasks to show, without any application code.
	CurrentPerformer   *uuid.UUID             `yaml:"current_performer,omitempty" json:"currentPerformer,omitempty"`
	Attributes         []Attribute            `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts         []TablePart            `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Characteristics    []ObjectCharacteristic `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	StandardAttributes []StandardAttribute    `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Forms              ObjectForms            `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands           []ObjectCommand        `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates          []ObjectTemplate       `yaml:"templates,omitempty" json:"templates,omitempty"`
	List               ListSettings           `yaml:"list,omitempty" json:"list,omitempty"`
}

// DecodeTask reads and validates one kind of task.
func DecodeTask(source string, reader io.Reader, configuration project.Project) (TaskDefinition, error) {
	var value TaskDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return TaskDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateNumberedObjectShape(numberedObjectShape{
		number:             value.Number,
		attributes:         value.Attributes,
		tableParts:         value.TableParts,
		forms:              value.Forms,
		list:               value.List,
		reservedName:       reservedTaskName,
		kind:               TaskKind,
		standardAttributes: value.StandardAttributes,
		presentation:       value.Presentations,
		input:              value.ObjectInput,
		basedOn:            value.BasedOn,
		dataLock:           value.DataLock,
		dataLockFields:     value.DataLockFields,
		fullTextSearch:     value.FullTextSearch,
		dataHistory:        value.DataHistorySettings,
		additionalIndexes:  value.AdditionalIndexes,
		extraFields:        []fieldGroup{{"addressing_attributes", addressingAttributeFields(value.AddressingAttributes)}},
	}, configuration)...)
	if value.DescriptionLength < 1 || value.DescriptionLength > maxDescriptionLength {
		issues = append(issues, fmt.Sprintf("description_length must be 1..%d", maxDescriptionLength))
	}
	// A task lives as long as the process that created it, and renumbering it
	// at the turn of a year would tear that process in two.
	if value.Number.Periodicity != "" && value.Number.Periodicity != NumberPeriodNone {
		issues = append(issues, "number.periodicity must be none: a task number does not restart with a period")
	}
	switch value.NumberPrefix {
	case "", NoNumberPrefix, BusinessProcessNumberPrefix:
	default:
		issues = append(issues, "number_prefix must be none or business-process-number")
	}
	if !validEditType(value.EditType) {
		issues = append(issues, "edit_type must be in-dialog, in-list or both-ways")
	}
	// A task has no code, so it stands for itself by its number or its
	// description - see object_choice.go.
	if !validTaskPresentation(value.DefaultPresentation) {
		issues = append(issues, "default_presentation must be as-number or as-description")
	}
	issues = append(issues, validateAddressing(value, configuration)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return TaskDefinition{}, err
	}
	return value, nil
}

// addressingAttributeFields is the addressing attributes as plain fields, for
// the checks that do not care which kind of field this is.
func addressingAttributeFields(attributes []AddressingAttribute) []Attribute {
	result := make([]Attribute, 0, len(attributes))
	for _, attribute := range attributes {
		result = append(result, attribute.Attribute)
	}
	return result
}

func validateAddressing(value TaskDefinition, configuration project.Project) []string {
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	attributeNames := map[string]bool{}
	for _, attribute := range value.Attributes {
		attributeNames[strings.ToLower(attribute.Name)] = true
	}
	for index, attribute := range value.AddressingAttributes {
		prefix := fmt.Sprintf("addressing_attributes[%d]", index)
		if attribute.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[attribute.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[attribute.ID] = true
		if !validIdentifier(attribute.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(attribute.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		if attributeNames[folded] {
			issues = append(issues, prefix+".name conflicts with an attribute")
		}
		if reservedTaskName(folded) {
			issues = append(issues, prefix+".name is reserved")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", attribute.Title, configuration)...)
		issues = append(issues, validateTypes(prefix+".types", attribute.Types, value.ID)...)
		issues = append(issues, validateFieldSettings(prefix, attribute.Attribute, configuration)...)
		issues = append(issues, validateFieldStorage(prefix, attribute.Attribute)...)
		// «Использование» divides the fields of items from the fields of
		// folders. A task has neither, so the setting would read as a division
		// nothing performs - the export writes it on no addressing attribute,
		// and the syntax assistant does not list it among their properties.
		if attribute.Use != "" {
			issues = append(issues, prefix+".use belongs to an attribute of a catalog or a chart of characteristic types, and this is neither")
		}
		if attribute.Dimension != nil && attribute.Dimension.IsZero() {
			issues = append(issues, prefix+".dimension must be a non-zero UUID")
		}
		// An addressing attribute matched against nothing addresses nothing:
		// the register is where the people behind a role are found.
		if attribute.Dimension != nil && value.Addressing == nil {
			issues = append(issues, prefix+".dimension needs an addressing register")
		}
	}
	if value.Addressing != nil && value.Addressing.IsZero() {
		issues = append(issues, "addressing must be a non-zero UUID")
	}
	if value.Addressing != nil && len(value.AddressingAttributes) == 0 {
		issues = append(issues, "an addressing register without addressing attributes matches nothing")
	}
	if len(value.AddressingAttributes) > 0 && value.Addressing == nil {
		issues = append(issues, "addressing attributes need the register that resolves them")
	}
	if value.MainAddressingAttribute != "" && !names[strings.ToLower(value.MainAddressingAttribute)] {
		issues = append(issues, "main_addressing_attribute names "+value.MainAddressingAttribute+", which is not an addressing attribute")
	}
	if value.MainAddressingAttribute == "" && len(value.AddressingAttributes) > 0 {
		issues = append(issues, "main_addressing_attribute is required: without it nobody knows which attribute holds the performer")
	}
	if value.CurrentPerformer != nil && value.CurrentPerformer.IsZero() {
		issues = append(issues, "current_performer must be a non-zero UUID")
	}
	return issues
}

// reservedTaskName keeps the standard attributes of a task: its number, date
// and description, whether it is executed, and where it stands - the business
// process and the point of its route.
func reservedTaskName(name string) bool {
	return reservedStandardName(TaskKind, name) || reservedRowVersionName(name)
}

func cloneTask(value TaskDefinition) TaskDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.AddressingAttributes = slices.Clone(value.AddressingAttributes)
	for index := range value.AddressingAttributes {
		value.AddressingAttributes[index].Attribute = cloneAttribute(value.AddressingAttributes[index].Attribute)
		if value.AddressingAttributes[index].Dimension != nil {
			id := *value.AddressingAttributes[index].Dimension
			value.AddressingAttributes[index].Dimension = &id
		}
	}
	for _, id := range []**uuid.UUID{&value.Addressing, &value.CurrentPerformer} {
		if *id != nil {
			copied := **id
			*id = &copied
		}
	}
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.Presentations = clonePresentations(value.Presentations)
	value.ObjectInput = cloneObjectInput(value.ObjectInput)
	value.BasedOn = slices.Clone(value.BasedOn)
	value.DataLockFields = cloneDataLockFields(value.DataLockFields)
	return value
}

// Task returns one kind of task by name, folded case.
func (catalog *Catalog) Task(name string) (TaskDefinition, bool) {
	index, ok := catalog.taskByName[strings.ToLower(name)]
	if !ok {
		return TaskDefinition{}, false
	}
	return cloneTask(catalog.Tasks[index]), true
}

// TaskByID returns one kind of task by its identifier.
func (catalog *Catalog) TaskByID(id uuid.UUID) (TaskDefinition, bool) {
	index, ok := catalog.taskByID[id]
	if !ok {
		return TaskDefinition{}, false
	}
	return cloneTask(catalog.Tasks[index]), true
}

func (catalog *Catalog) taskTables(definition TaskDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "date", Type: "timestamp with time zone", Nullable: false},
			{Name: "description", Type: fmt.Sprintf("character varying(%d)", definition.DescriptionLength), Nullable: false, Default: "''::character varying"},
			{Name: "executed", Type: "boolean", Nullable: false, Default: "false"},
			// Where the task stands: the process that created it and the point
			// of its route. The point is kept by name, because a point is not a
			// row of any table - it is part of the map the configuration draws.
			{Name: "business_process", Type: "uuid", Nullable: true},
			{Name: "route_point", Type: "character varying(128)", Nullable: true},
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}},
			// Открыть свои невыполненные задачи - то, ради чего этот вид и
			// существует, поэтому выполненность индексируется.
			{Name: physicalObjectName("ie", definition.ID), Method: "btree", Keys: []string{"executed"}},
		},
	}
	appendNumberColumn(&table, definition.ID, definition.Number)
	// Addressing attributes are columns of the task: a task carries the role it
	// is addressed to, and the register turns that into people.
	//
	// The index is the field's own answer now that the field carries one. It
	// used to be forced on here, because an addressing attribute had no
	// indexing of its own to read - and a setting the developer writes and the
	// schema overrules is worse than no setting at all. Nothing is lost by
	// reading it: every addressing attribute of the export asks for the index.
	for _, attribute := range definition.AddressingAttributes {
		if err := catalog.appendAttributeSchema(&table, attribute.Attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("task %s addressing attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("task %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Number", "Description"}, definition.Attributes, map[string]listColumn{
		"number": {name: "number", kind: definition.Number.Type}, "description": {name: "description", kind: StringType},
	})
	parts, err := catalog.tablePartTables("task", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	return table, parts, nil
}
