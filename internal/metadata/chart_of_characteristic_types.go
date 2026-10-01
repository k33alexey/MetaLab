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

// ChartOfCharacteristicTypesDefinition describes the kinds of characteristic a
// configuration lets its users invent: extra properties nobody foresaw, added
// without changing the configuration.
//
// Its shape repeats a catalog - code, description, attributes, table parts,
// modules, forms, predefined elements - and adds the two things that make it a
// chart of characteristic types rather than another catalog: what a value of a
// characteristic may be, and where the values that fit no existing type live.
type ChartOfCharacteristicTypesDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Presentations is how this object is named to the person using it -
	// see object_presentation.go.
	Presentations `yaml:",inline" json:",inline"` // ObjectChoice is how a value of this kind is entered and picked - see
	// object_choice.go.
	ObjectChoice `yaml:",inline" json:",inline"`
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

	Code              CatalogCode `yaml:"code" json:"code"`
	DescriptionLength int         `yaml:"description_length" json:"descriptionLength"`
	Hierarchy         Hierarchy   `yaml:"hierarchy,omitempty" json:"hierarchy,omitempty"`
	// ValueType is what a value of a characteristic of this chart may be. It
	// is the ceiling, not the value: each element narrows it further to its
	// own type, and an attribute typed by this chart accepts what the chart
	// allows.
	ValueType []Type `yaml:"value_type" json:"valueType"`
	// AdditionalValues is the catalog holding values that fit no existing
	// type - a characteristic whose values are a list the user writes
	// themselves has nowhere else to put them.
	AdditionalValues     *uuid.UUID              `yaml:"additional_values,omitempty" json:"additionalValues,omitempty"`
	Attributes           []Attribute             `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts           []TablePart             `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Characteristics      []ObjectCharacteristic  `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	StandardAttributes   []StandardAttribute     `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	Forms                HierarchicalObjectForms `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands             []ObjectCommand         `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates            []ObjectTemplate        `yaml:"templates,omitempty" json:"templates,omitempty"`
	List                 ListSettings            `yaml:"list,omitempty" json:"list,omitempty"`
	Predefined           []PredefinedCatalogItem `yaml:"predefined,omitempty" json:"predefined,omitempty"`
	PredefinedDataUpdate PredefinedDataUpdate    `yaml:"predefined_data_update,omitempty" json:"predefinedDataUpdate,omitempty"`
}

// DecodeChartOfCharacteristicTypes reads and validates one chart description.
// validateCharacteristicTypesHierarchy refuses what a chart of characteristic
// types does not have. Its hierarchy is a flag and folders on top, nothing
// more: no choice of a hierarchy of items, no limit of levels - the help gives
// the chart neither property, and the configurations being moved never write
// them on one. Its hierarchy is always one of folders and items.
func validateCharacteristicTypesHierarchy(hierarchy Hierarchy) []string {
	var issues []string
	if hierarchy.Kind == ItemsHierarchy {
		issues = append(issues, "hierarchy.kind of a chart of characteristic types is always folders-and-items")
	}
	if hierarchy.LimitLevels || hierarchy.LevelCount != 0 {
		issues = append(issues, "hierarchy of a chart of characteristic types has no limit of levels")
	}
	return issues
}

func DecodeChartOfCharacteristicTypes(source string, reader io.Reader, configuration project.Project) (ChartOfCharacteristicTypesDefinition, error) {
	var value ChartOfCharacteristicTypesDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ChartOfCharacteristicTypesDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateCharacteristicTypesHierarchy(value.Hierarchy)...)
	issues = append(issues, validateReferenceObjectShape(referenceObjectShape{
		code:                 value.Code,
		descriptionLength:    value.DescriptionLength,
		attributes:           value.Attributes,
		tableParts:           value.TableParts,
		forms:                value.Forms,
		list:                 value.List,
		hierarchy:            value.Hierarchy,
		predefined:           value.Predefined,
		reservedName:         reservedChartOfCharacteristicTypesName,
		attributeUse:         true,
		codeSeries:           true,
		codeAllowedLength:    true,
		autonumbering:        true,
		checkUnique:          true,
		predefinedDataUpdate: value.PredefinedDataUpdate,
		presentation:         value.Presentations,
		choice:               value.ObjectChoice,
		basedOn:              value.BasedOn,
		dataLock:             value.DataLock,
		dataLockFields:       value.DataLockFields,
		fullTextSearch:       value.FullTextSearch,
		dataHistory:          value.DataHistorySettings,
		additionalIndexes:    value.AdditionalIndexes,
		kind:                 ChartOfCharacteristicTypesKind,
		standardAttributes:   value.StandardAttributes,
	}, configuration)...)
	// The value type is the point of the whole object: a chart that allows
	// nothing describes characteristics nobody can fill in.
	issues = append(issues, validateTypes("value_type", value.ValueType, value.ID)...)
	if value.AdditionalValues != nil && value.AdditionalValues.IsZero() {
		issues = append(issues, "additional_values must be a non-zero UUID")
	}
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return ChartOfCharacteristicTypesDefinition{}, err
	}
	return value, nil
}

// reservedChartOfCharacteristicTypesName keeps the catalog's own names and adds
// the value type: every element carries one, so an attribute of that name would
// collide with what the platform already put there.
func reservedChartOfCharacteristicTypesName(name string) bool {
	return reservedStandardName(ChartOfCharacteristicTypesKind, name) || reservedRowVersionName(name)
}

func cloneChartOfCharacteristicTypes(value ChartOfCharacteristicTypesDefinition) ChartOfCharacteristicTypesDefinition {
	value.Title = cloneTitle(value.Title)
	value.ValueType = cloneTypes(value.ValueType)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	if value.AdditionalValues != nil {
		id := *value.AdditionalValues
		value.AdditionalValues = &id
	}
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Predefined = slices.Clone(value.Predefined)
	for index := range value.Predefined {
		value.Predefined[index] = clonePredefinedCatalogItem(value.Predefined[index])
	}
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

// ChartOfCharacteristicTypes returns one chart by name, folded case.
func (catalog *Catalog) ChartOfCharacteristicTypes(name string) (ChartOfCharacteristicTypesDefinition, bool) {
	index, ok := catalog.chartOfCharacteristicTypesByName[strings.ToLower(name)]
	if !ok {
		return ChartOfCharacteristicTypesDefinition{}, false
	}
	return cloneChartOfCharacteristicTypes(catalog.ChartsOfCharacteristicTypes[index]), true
}

// ChartOfCharacteristicTypesByID returns one chart by its identifier.
func (catalog *Catalog) ChartOfCharacteristicTypesByID(id uuid.UUID) (ChartOfCharacteristicTypesDefinition, bool) {
	index, ok := catalog.chartOfCharacteristicTypesByID[id]
	if !ok {
		return ChartOfCharacteristicTypesDefinition{}, false
	}
	return cloneChartOfCharacteristicTypes(catalog.ChartsOfCharacteristicTypes[index]), true
}

func (catalog *Catalog) chartOfCharacteristicTypesTables(definition ChartOfCharacteristicTypesDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "code", Type: codeSQLType(definition.Code), Nullable: false},
			{Name: "description", Type: fmt.Sprintf("character varying(%d)", definition.DescriptionLength), Nullable: false, Default: "''::character varying"},
			// Every element carries its own value type, narrowed from what
			// the chart allows. It is a description of types, not a value, so
			// it is stored as one.
			{Name: "value_type", Type: "jsonb", Nullable: false, Default: "'[]'::jsonb"},
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "predefined_name", Type: "character varying(128)", Nullable: true},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"},
			{Name: physicalObjectName("up", definition.ID), Type: "unique", Definition: "UNIQUE (predefined_name)"},
		},
		Indexes: []schemadiff.Index{{Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}}},
	}
	if definition.Code.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (code)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"code"}})
	}
	appendHierarchyColumns(&table, definition.ID, definition.Hierarchy)
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("chart of characteristic types %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Description", "Code"}, definition.Attributes, map[string]listColumn{
		"code": {name: "code", kind: definition.Code.Type}, "description": {name: "description", kind: StringType},
	})
	parts, err := catalog.tablePartTables("chart of characteristic types", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	return table, parts, nil
}
