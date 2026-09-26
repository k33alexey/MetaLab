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
	NoBaseDependency         BaseDependency = "none"
	ActionPeriodBase         BaseDependency = "by-action-period"
	RegistrationPeriodBase   BaseDependency = "by-registration-period"
	maxBaseChartsPerCalcPlan                = 8
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
	Format            int           `yaml:"format" json:"format"`
	ID                uuid.UUID     `yaml:"id" json:"id"`
	Name              string        `yaml:"name" json:"name"`
	Title             LocalizedText `yaml:"title" json:"title"`
	Code              CatalogCode   `yaml:"code" json:"code"`
	DescriptionLength int           `yaml:"description_length" json:"descriptionLength"`
	// ActionPeriodUse turns on competition over the period an entry acts in.
	// Without it there is nothing to displace.
	ActionPeriodUse bool           `yaml:"action_period_use,omitempty" json:"actionPeriodUse,omitempty"`
	BaseDependency  BaseDependency `yaml:"base_dependency,omitempty" json:"baseDependency,omitempty"`
	// BaseCharts are the charts a base may be taken from, this one included.
	BaseCharts         []uuid.UUID                 `yaml:"base_charts,omitempty" json:"baseCharts,omitempty"`
	Attributes         []Attribute                 `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts         []TablePart                 `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Characteristics    []ObjectCharacteristic      `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	StandardAttributes []StandardAttribute         `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
	StandardTableParts []StandardTablePart         `yaml:"standard_table_parts,omitempty" json:"standardTableParts,omitempty"`
	Forms              ObjectForms                 `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands           []ObjectCommand             `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates          []ObjectTemplate            `yaml:"templates,omitempty" json:"templates,omitempty"`
	List               ListSettings                `yaml:"list,omitempty" json:"list,omitempty"`
	Predefined         []PredefinedCalculationType `yaml:"predefined,omitempty" json:"predefined,omitempty"`
}

// DecodeChartOfCalculationTypes reads and validates one chart of calculation types.
func DecodeChartOfCalculationTypes(source string, reader io.Reader, configuration project.Project) (ChartOfCalculationTypesDefinition, error) {
	var value ChartOfCalculationTypesDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ChartOfCalculationTypesDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateReferenceObjectShape(referenceObjectShape{
		code:               value.Code,
		descriptionLength:  value.DescriptionLength,
		attributes:         value.Attributes,
		tableParts:         value.TableParts,
		forms:              HierarchicalObjectForms{ObjectForms: value.Forms},
		list:               value.List,
		reservedName:       reservedCalculationTypeName,
		kind:               ChartOfCalculationTypesKind,
		standardAttributes: value.StandardAttributes,
		standardTableParts: value.StandardTableParts,
	}, configuration)...)
	switch value.BaseDependency {
	case "", NoBaseDependency, ActionPeriodBase, RegistrationPeriodBase:
	default:
		issues = append(issues, "base_dependency must be none, by-action-period or by-registration-period")
	}
	if len(value.BaseCharts) > maxBaseChartsPerCalcPlan {
		issues = append(issues, fmt.Sprintf("base_charts must not contain more than %d charts", maxBaseChartsPerCalcPlan))
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
	if len(value.Predefined) > maxObjectsPerKind {
		return []string{fmt.Sprintf("predefined must not contain more than %d items", maxObjectsPerKind)}
	}
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
		if !validIdentifier(item.Name) || utf8.RuneCountInString(item.Name) > 128 {
			issues = append(issues, prefix+".name must be a valid identifier of at most 128 characters")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		if item.Code == "" {
			if !value.Code.Auto {
				issues = append(issues, prefix+".code is required when automatic codes are disabled")
			}
		} else if _, err := normalizeCatalogCode(value.Code, item.Code); err != nil {
			issues = append(issues, prefix+".code is invalid: "+err.Error())
		}
		if utf8.RuneCountInString(item.Description) > value.DescriptionLength {
			issues = append(issues, fmt.Sprintf("%s.description must not exceed %d characters", prefix, value.DescriptionLength))
		}
		// Each list exists only under the setting that gives it meaning.
		if len(item.Displacing) > 0 && !value.ActionPeriodUse {
			issues = append(issues, prefix+".displacing needs action_period_use: without an action period there is nothing to displace")
		}
		if len(item.Base) > 0 && (value.BaseDependency == "" || value.BaseDependency == NoBaseDependency) {
			issues = append(issues, prefix+".base needs base_dependency")
		}
		if item.ActionPeriodIsBase && !value.ActionPeriodUse {
			issues = append(issues, prefix+".action_period_is_base needs action_period_use")
		}
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
			{Name: "code", Type: codeSQLType(definition.Code), Nullable: false},
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
	if definition.ActionPeriodUse {
		// The action period is the base period of this type or it is not, and
		// the competition rules read that flag on every calculation.
		table.Columns = append(table.Columns, schemadiff.Column{Name: "action_period_is_base", Type: "boolean", Nullable: false, Default: "false"})
	}
	if definition.Code.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (code)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"code"}})
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("chart of calculation types %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Description", "Code"}, definition.Attributes, map[string]listColumn{
		"code": {name: "code", kind: definition.Code.Type}, "description": {name: "description", kind: StringType},
	})
	parts, err := catalog.tablePartTables("chart of calculation types", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	// Leading types are always there; displacing and base exist only under the
	// setting that gives them meaning, and a table for a rule that cannot
	// apply would be a column nobody may fill.
	parts = append(parts, competitionTable("tl", definition.ID, tableName, tableName))
	if definition.ActionPeriodUse {
		parts = append(parts, competitionTable("tw", definition.ID, tableName, tableName))
	}
	if definition.BaseDependency != "" && definition.BaseDependency != NoBaseDependency {
		// A base may come from several charts, so the row says which chart the
		// type belongs to and the key stays off: one column cannot reference
		// several tables at once.
		base := competitionTable("tb", definition.ID, tableName, "")
		base.Columns = append(base.Columns, schemadiff.Column{Name: "base_chart", Type: "uuid", Nullable: false})
		parts = append(parts, base)
	}
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
