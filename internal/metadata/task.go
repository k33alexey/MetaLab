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
type AddressingAttribute struct {
	ID    uuid.UUID     `yaml:"id" json:"id"`
	Name  string        `yaml:"name" json:"name"`
	Title LocalizedText `yaml:"title" json:"title"`
	Types []Type        `yaml:"types" json:"types"`
	// Dimension is the dimension of the addressing register this attribute is
	// matched against.
	Dimension *uuid.UUID `yaml:"dimension,omitempty" json:"dimension,omitempty"`
}

// TaskDefinition describes assignments to people, addressed by role rather than
// by name.
type TaskDefinition struct {
	Format            int            `yaml:"format" json:"format"`
	ID                uuid.UUID      `yaml:"id" json:"id"`
	Name              string         `yaml:"name" json:"name"`
	Title             LocalizedText  `yaml:"title" json:"title"`
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
	CurrentPerformer *uuid.UUID       `yaml:"current_performer,omitempty" json:"currentPerformer,omitempty"`
	Attributes       []Attribute      `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts       []TablePart      `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Forms            ObjectForms      `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands         []ObjectCommand  `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates        []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
	List             ListSettings     `yaml:"list,omitempty" json:"list,omitempty"`
}

// DecodeTask reads and validates one kind of task.
func DecodeTask(source string, reader io.Reader, configuration project.Project) (TaskDefinition, error) {
	var value TaskDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return TaskDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateNumberedObjectShape(numberedObjectShape{
		number:       value.Number,
		attributes:   value.Attributes,
		tableParts:   value.TableParts,
		forms:        value.Forms,
		list:         value.List,
		reservedName: reservedTaskName,
	}, configuration)...)
	if value.DescriptionLength < 1 || value.DescriptionLength > 1_048_576 {
		issues = append(issues, "description_length must be 1..1048576")
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
	issues = append(issues, validateAddressing(value, configuration)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return TaskDefinition{}, err
	}
	return value, nil
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
	switch strings.ToLower(name) {
	case "ссылка", "ref", "номер", "number", "дата", "date", "наименование", "description",
		"выполнена", "executed", "бизнеспроцесс", "businessprocess", "точкамаршрута", "routepoint",
		"пометкаудаления", "deletionmark", "версия", "version":
		return true
	default:
		return false
	}
}

func cloneTask(value TaskDefinition) TaskDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = slices.Clone(value.TableParts)
	for index := range value.TableParts {
		value.TableParts[index].Title = cloneTitle(value.TableParts[index].Title)
		value.TableParts[index].Attributes = cloneAttributes(value.TableParts[index].Attributes)
	}
	value.AddressingAttributes = slices.Clone(value.AddressingAttributes)
	for index := range value.AddressingAttributes {
		value.AddressingAttributes[index].Title = cloneTitle(value.AddressingAttributes[index].Title)
		value.AddressingAttributes[index].Types = cloneTypes(value.AddressingAttributes[index].Types)
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
	value.Forms = cloneObjectForms(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
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
			{Name: "number", Type: documentNumberSQLType(definition.Number), Nullable: false},
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
	if definition.Number.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (number)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"number"}})
	}
	// Addressing attributes are columns of the task: a task carries the role it
	// is addressed to, and the register turns that into people.
	for _, attribute := range definition.AddressingAttributes {
		if err := catalog.appendAttributeSchema(&table, Attribute{ID: attribute.ID, Name: attribute.Name, Title: attribute.Title, Types: attribute.Types, Indexed: true}); err != nil {
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
