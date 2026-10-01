package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// BaseDependency is how a calculation type takes its base from other types:
// not at all, or over the period an entry acts in, or over the period it was
// registered in. The choice decides which entries the base is gathered from,
// so it is not a display setting.
type BaseDependency string

const (
	NoBaseDependency       BaseDependency = "none"
	ActionPeriodBase       BaseDependency = "by-action-period"
	RegistrationPeriodBase BaseDependency = "by-registration-period"
)

// PredefinedCalculationType is a kind of accrual or deduction the configuration
// itself brings, together with the rules of how it competes with the others.
// Those rules are the point of the object: a calculation type carried over
// without its displacing and base lists computes different numbers while
// looking transferred.
type PredefinedCalculationType struct {
	ID          uuid.UUID `yaml:"id" json:"id"`
	Name        string    `yaml:"name" json:"name"`
	Code        string    `yaml:"code,omitempty" json:"code,omitempty"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	// ActionPeriodIsBase says the period this type acts in is the period its
	// base is gathered over.
	ActionPeriodIsBase bool `yaml:"action_period_is_base,omitempty" json:"actionPeriodIsBase,omitempty"`
	// Leading, Displacing and Base name calculation types of this chart; Base
	// may also name one of a base chart as "План.Вид".
	Leading    []string `yaml:"leading,omitempty" json:"leading,omitempty"`
	Displacing []string `yaml:"displacing,omitempty" json:"displacing,omitempty"`
	Base       []string `yaml:"base,omitempty" json:"base,omitempty"`
}

// ChartOfCalculationTypesDefinition describes kinds of accrual and deduction
// and how they influence one another.
//
// It repeats a catalog but never nests: the prototype has no hierarchy here at
// all, and inventing one would give the object a switch it must not have.
type ChartOfCalculationTypesDefinition struct {
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
	// ActionPeriodUse turns on competition over the period an entry acts in.
	// Without it there is nothing to displace.
	ActionPeriodUse bool           `yaml:"action_period_use,omitempty" json:"actionPeriodUse,omitempty"`
	BaseDependency  BaseDependency `yaml:"base_dependency,omitempty" json:"baseDependency,omitempty"`
	// BaseCharts are the charts a base may be taken from, this one included.
	BaseCharts           []uuid.UUID                 `yaml:"base_charts,omitempty" json:"baseCharts,omitempty"`
	Attributes           []Attribute                 `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts           []TablePart                 `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Characteristics      []ObjectCharacteristic      `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	StandardAttributes   []StandardAttribute         `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	StandardTableParts   []StandardTablePart         `yaml:"standard_table_parts,omitempty" json:"standardTableParts,omitempty"`
	Forms                ObjectForms                 `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands             []ObjectCommand             `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates            []ObjectTemplate            `yaml:"templates,omitempty" json:"templates,omitempty"`
	List                 ListSettings                `yaml:"list,omitempty" json:"list,omitempty"`
	Predefined           []PredefinedCalculationType `yaml:"predefined,omitempty" json:"predefined,omitempty"`
	PredefinedDataUpdate PredefinedDataUpdate        `yaml:"predefined_data_update,omitempty" json:"predefinedDataUpdate,omitempty"`
}

// DecodeChartOfCalculationTypes reads and validates one chart of calculation types.
func DecodeChartOfCalculationTypes(source string, reader io.Reader, configuration project.Project) (ChartOfCalculationTypesDefinition, error) {
	var value ChartOfCalculationTypesDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ChartOfCalculationTypesDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateReferenceObjectShape(referenceObjectShape{
		code:                 value.Code,
		descriptionLength:    value.DescriptionLength,
		attributes:           value.Attributes,
		tableParts:           value.TableParts,
		forms:                HierarchicalObjectForms{ObjectForms: value.Forms},
		list:                 value.List,
		reservedName:         reservedCalculationTypeName,
		codeAllowedLength:    true,
		codeType:             true,
		codeMayBeAbsent:      true,
		predefinedDataUpdate: value.PredefinedDataUpdate,
		presentation:         value.Presentations,
		choice:               value.ObjectChoice,
		basedOn:              value.BasedOn,
		dataLock:             value.DataLock,
		dataLockFields:       value.DataLockFields,
		fullTextSearch:       value.FullTextSearch,
		dataHistory:          value.DataHistorySettings,
		additionalIndexes:    value.AdditionalIndexes,
		kind:                 ChartOfCalculationTypesKind,
		standardAttributes:   value.StandardAttributes,
		standardTableParts:   value.StandardTableParts,
	}, configuration)...)
	switch value.BaseDependency {
	case "", NoBaseDependency, ActionPeriodBase, RegistrationPeriodBase:
	default:
		issues = append(issues, "base_dependency must be none, by-action-period or by-registration-period")
	}
	// A base that comes from nowhere and charts nobody takes a base from are
	// the same mistake seen from two sides.
	if value.BaseDependency != "" && value.BaseDependency != NoBaseDependency && len(value.BaseCharts) == 0 {
		issues = append(issues, "base_dependency needs base_charts to take the base from")
	}
	if len(value.BaseCharts) > 0 && (value.BaseDependency == "" || value.BaseDependency == NoBaseDependency) {
		issues = append(issues, "base_charts are useless while base_dependency is none")
	}
	seen := map[uuid.UUID]bool{}
	for index, chart := range value.BaseCharts {
		if chart.IsZero() {
			issues = append(issues, fmt.Sprintf("base_charts[%d] must be a non-zero UUID", index))
		}
		if seen[chart] {
			issues = append(issues, fmt.Sprintf("base_charts[%d] repeats", index))
		}
		seen[chart] = true
	}
	issues = append(issues, validatePredefinedCalculationTypes(value)...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	issues = append(issues, validateObjectCharacteristics(value.Characteristics)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return ChartOfCalculationTypesDefinition{}, err
	}
	return value, nil
}

func validatePredefinedCalculationTypes(value ChartOfCalculationTypesDefinition) []string {
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, item := range value.Predefined {
		prefix := fmt.Sprintf("predefined[%d]", index)
		if item.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[item.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[item.ID] = true
		if !validIdentifier(item.Name) || utf8.RuneCountInString(item.Name) > maxNameLength {
			issues = append(issues, prefix+".name must be a valid identifier of at most 255 characters")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		if value.Code.Length == 0 {
			if item.Code != "" {
				issues = append(issues, prefix+".code is given, and the code is switched off by a length of 0")
			}
		} else if item.Code == "" {
			if !value.Code.Auto {
				issues = append(issues, prefix+".code is required when automatic codes are disabled")
			}
		} else if _, err := normalizeCatalogCode(value.Code, item.Code); err != nil {
			issues = append(issues, prefix+".code is invalid: "+err.Error())
		}
		if utf8.RuneCountInString(item.Description) > value.DescriptionLength {
			issues = append(issues, fmt.Sprintf("%s.description must not exceed %d characters", prefix, value.DescriptionLength))
		}
		// The lists and the flag are carried whatever the settings say. The
		// prototype keeps all three lists and the flag on every chart, with
		// the setting that gives them meaning off as well - erp "Удержания"
		// has no action period and still writes the flag and the list of
		// displacing types - so a chart that switched a setting off after
		// filling them in is saved as it is, and it is the calculation that
		// does not read them.
	}
	// Leading and displacing name types of this chart, so they resolve here.
	for index, item := range value.Predefined {
		for role, list := range map[string][]string{"leading": item.Leading, "displacing": item.Displacing} {
			for _, name := range list {
				if !names[strings.ToLower(name)] {
					issues = append(issues, fmt.Sprintf("predefined[%d].%s names %s, which is not a calculation type of this chart", index, role, name))
				}
				if strings.EqualFold(name, item.Name) {
					issues = append(issues, fmt.Sprintf("predefined[%d].%s names the type itself", index, role))
				}
			}
		}
	}
	return issues
}

// reservedCalculationTypeName keeps the catalog's own names and the one standard
// attribute of a calculation type.
func reservedCalculationTypeName(name string) bool {
	switch foldStandardName(name) {
	case "actionperiodisbase":
		// Not the prototype's name for it - that is ActionPeriodIsBasic - but
		// close enough that a developer writes it and shadows the field.
		return true
	default:
		return reservedStandardName(ChartOfCalculationTypesKind, name) || reservedRowVersionName(name)
	}
}

func cloneChartOfCalculationTypes(value ChartOfCalculationTypesDefinition) ChartOfCalculationTypesDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.BaseCharts = slices.Clone(value.BaseCharts)
	value.Forms = cloneFormSet(value.Forms)
	value.List.SearchFields = slices.Clone(value.List.SearchFields)
	value.Predefined = slices.Clone(value.Predefined)
	for index := range value.Predefined {
		value.Predefined[index].Leading = slices.Clone(value.Predefined[index].Leading)
		value.Predefined[index].Displacing = slices.Clone(value.Predefined[index].Displacing)
		value.Predefined[index].Base = slices.Clone(value.Predefined[index].Base)
	}
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.Characteristics = cloneObjectCharacteristics(value.Characteristics)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.StandardTableParts = cloneStandardTableParts(value.StandardTableParts)
	value.Presentations = clonePresentations(value.Presentations)
	value.ObjectInput = cloneObjectInput(value.ObjectInput)
	value.BasedOn = slices.Clone(value.BasedOn)
	value.DataLockFields = cloneDataLockFields(value.DataLockFields)
	return value
}

// ChartOfCalculationTypes returns one chart by name, folded case.
func (catalog *Catalog) ChartOfCalculationTypes(name string) (ChartOfCalculationTypesDefinition, bool) {
	index, ok := catalog.chartOfCalculationTypesByName[strings.ToLower(name)]
	if !ok {
		return ChartOfCalculationTypesDefinition{}, false
	}
	return cloneChartOfCalculationTypes(catalog.ChartsOfCalculationTypes[index]), true
}

// ChartOfCalculationTypesByID returns one chart by its identifier.
func (catalog *Catalog) ChartOfCalculationTypesByID(id uuid.UUID) (ChartOfCalculationTypesDefinition, bool) {
	index, ok := catalog.chartOfCalculationTypesByID[id]
	if !ok {
		return ChartOfCalculationTypesDefinition{}, false
	}
	return cloneChartOfCalculationTypes(catalog.ChartsOfCalculationTypes[index]), true
}

// competitionTableName is one of the three standard tabular sections of a
// calculation type. They are the platform's own, not attributes of the
// configuration, so they have no UUID to be named by.
func competitionTableName(prefix string, id uuid.UUID) string { return physicalObjectName(prefix, id) }

func (catalog *Catalog) chartOfCalculationTypesTables(definition ChartOfCalculationTypesDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "description", Type: fmt.Sprintf("character varying(%d)", definition.DescriptionLength), Nullable: false, Default: "''::character varying"},
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "predefined_name", Type: "character varying(128)", Nullable: true},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"},
			{Name: physicalObjectName("up", definition.ID), Type: "unique", Definition: "UNIQUE (predefined_name)"},
		},
		Indexes: []schemadiff.Index{{Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}}},
	}
	// The action period is the base period of this type or it is not, and the
	// competition rules read that flag on every calculation. The column is
	// there with the action period off as well: the flag is a standard field
	// of every calculation type in the prototype, not of those that compete.
	table.Columns = append(table.Columns, schemadiff.Column{Name: "action_period_is_base", Type: "boolean", Nullable: false, Default: "false"})
	appendCodeColumn(&table, definition.ID, definition.Code)
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("chart of calculation types %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Description", "Code"}, definition.Attributes, codeAndDescriptionColumns(definition.Code, definition.DescriptionLength))
	parts, err := catalog.tablePartTables("chart of calculation types", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	// All three are always there, as they are in the prototype: a chart that
	// has no action period or no base still has its lists of displacing and
	// base types, and only the calculation leaves them unread.
	parts = append(parts, competitionTable("tl", definition.ID, tableName, tableName))
	parts = append(parts, competitionTable("tw", definition.ID, tableName, tableName))
	// A base may come from several charts, so the row says which chart the
	// type belongs to and the key stays off: one column cannot reference
	// several tables at once.
	base := competitionTable("tb", definition.ID, tableName, "")
	base.Columns = append(base.Columns, schemadiff.Column{Name: "base_chart", Type: "uuid", Nullable: false})
	parts = append(parts, base)
	return table, parts, nil
}

// competitionTable builds one standard tabular section of competition rules.
// target names the table the calculation type belongs to; an empty target
// leaves the reference unkeyed.
func competitionTable(prefix string, id uuid.UUID, ownerTable, target string) schemadiff.Table {
	table := schemadiff.Table{
		Name: competitionTableName(prefix, id),
		Columns: []schemadiff.Column{
			{Name: "owner_ref", Type: "uuid", Nullable: false},
			{Name: "line_no", Type: "integer", Nullable: false},
			{Name: "calculation_type", Type: "uuid", Nullable: false},
			// A line the configuration brought apart from one the user added -
			// the standard field "Предопределенный" of every such line.
			{Name: "predefined", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("p"+prefix, id), Type: "primary_key", Definition: "PRIMARY KEY (owner_ref, line_no)"},
			{Name: physicalObjectName("f"+prefix, id), Type: "foreign_key",
				Definition: "FOREIGN KEY (owner_ref) REFERENCES " + schemadiff.ApplicationSchema + "." + ownerTable + "(ref) ON DELETE CASCADE"},
			{Name: physicalObjectName("u"+prefix, id), Type: "unique", Definition: "UNIQUE (owner_ref, calculation_type)"},
		},
	}
	if target != "" {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("c"+prefix, id), Type: "foreign_key",
			Definition: "FOREIGN KEY (calculation_type) REFERENCES " + schemadiff.ApplicationSchema + "." + target + "(ref) DEFERRABLE INITIALLY DEFERRED",
		})
	}
	return table
}
