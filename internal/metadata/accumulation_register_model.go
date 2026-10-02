package metadata

import (
	"fmt"
	"io"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// AccumulationRegisterKindValue controls whether a register stores balances or only turnovers.
type AccumulationRegisterKindValue string

const (
	AccumulationRegisterBalance  AccumulationRegisterKindValue = "balance"
	AccumulationRegisterTurnover AccumulationRegisterKindValue = "turnover"
)

// AccumulationRegisterDefinition describes recorder-owned movements and rebuildable totals.
type AccumulationRegisterDefinition struct {
	Format int           `yaml:"format"`
	ID     uuid.UUID     `yaml:"id"`
	Name   string        `yaml:"name"`
	Title  LocalizedText `yaml:"title"` // ListPresentations and the help flag: a row of this kind is not an object a
	// person opens, so there is a list to name and no object - see
	// object_presentation.go.
	ListPresentations     `yaml:",inline" json:",inline"`
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
	// DataLock is how the platform locks records of this register while they
	// are written - see data_lock_settings.go. A register has the mode and no
	// fields: the prototype gives the list of fields only to the kinds that
	// have an object of their own.
	DataLock project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	// FullTextSearch is whether the records of this register are in the
	// full-text index - see full_text_search.go.
	FullTextSearch FullTextSearchMode `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	// AdditionalIndexes are the indexes this register asks the database for
	// beside the ones the platform builds - see additional_indexes.go.
	AdditionalIndexes []AdditionalIndex `yaml:"additional_indexes,omitempty" json:"additionalIndexes,omitempty"`

	Kind AccumulationRegisterKindValue `yaml:"kind"`
	// AllowTotalsSplitting permits the totals of concurrent writers to be kept
	// in rows of their own instead of making them queue on one.
	//
	// It permits and does not switch: the prototype calls the property
	// РазрешитьРазделениеИтогов and keeps the switch itself in the working
	// database - see register_totals_mode.go. The register used to split its
	// totals always and had no way to be asked about it at all.
	AllowTotalsSplitting bool `yaml:"allow_totals_splitting,omitempty"`
	// Aggregates are precomputed cuts of the movements, a subordinate entity
	// and not a property - see accumulation_aggregates.go. Declared here,
	// switched on in the working database, and an alternative to totals rather
	// than a layer above them.
	Aggregates         []AccumulationRegisterAggregate `yaml:"aggregates,omitempty" json:"aggregates,omitempty"`
	Dimensions         []RegisterDimension             `yaml:"dimensions,omitempty"`
	Resources          []Attribute                     `yaml:"resources"`
	Attributes         []Attribute                     `yaml:"attributes,omitempty"`
	StandardAttributes []StandardAttribute             `yaml:"standard_attributes,omitempty"`
	Forms              RegisterForms                   `yaml:"forms,omitempty"`
	Commands           []ObjectCommand                 `yaml:"commands,omitempty"`
	Templates          []ObjectTemplate                `yaml:"templates,omitempty"`
}

func DecodeAccumulationRegister(source string, reader io.Reader, configuration project.Project) (AccumulationRegisterDefinition, error) {
	var value AccumulationRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return AccumulationRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateDataLockMode("data_lock", value.DataLock)...)
	issues = append(issues, validateFullTextSearch("full_text_search", value.FullTextSearch)...)
	if value.Kind != AccumulationRegisterBalance && value.Kind != AccumulationRegisterTurnover {
		issues = append(issues, "kind must be balance or turnover")
	}
	issues = append(issues, validateAttributes("dimensions", RegisterDimensionAttributes(value.Dimensions), configuration, reservedAccumulationRegisterName)...)
	for index, dimension := range value.Dimensions {
		issues = append(issues, validateRegisterDimension(fmt.Sprintf("dimensions[%d]", index), dimension,
			accumulationRegisterDimensions(value.Kind == AccumulationRegisterBalance))...)
	}
	issues = append(issues, validateAttributes("resources", value.Resources, configuration, reservedAccumulationRegisterName)...)
	for index, resource := range value.Resources {
		issues = append(issues, validateResourceIndexing(fmt.Sprintf("resources[%d]", index), resource.Indexing)...)
	}
	for _, group := range []struct {
		path   string
		fields []Attribute
	}{{"dimensions", RegisterDimensionAttributes(value.Dimensions)}, {"resources", value.Resources}, {"attributes", value.Attributes}} {
		for index, field := range group.fields {
			issues = append(issues, validateMovementFieldStorage(fmt.Sprintf("%s[%d]", group.path, index), field)...)
		}
	}
	issues = append(issues, validateAttributes("attributes", value.Attributes, configuration, reservedAccumulationRegisterName)...)
	registerFields := []fieldGroup{
		{"dimensions", RegisterDimensionAttributes(value.Dimensions)}, {"resources", value.Resources}, {"attributes", value.Attributes},
	}
	issues = append(issues, validateListPresentations(value.ListPresentations, configuration)...)
	issues = append(issues, validateStandardAttributes("standard_attributes", value.StandardAttributes, accumulationStandardFields(value.Kind), configuration)...)
	issues = append(issues, validateFieldLinks(accumulationStandardFields(value.Kind), registerFields, nil, standardAttributeChoices("standard_attributes", value.StandardAttributes)...)...)
	issues = append(issues, validateAttributeUse(registerFields, nil, false, false)...)
	if len(value.Resources) == 0 {
		issues = append(issues, "resources must contain at least one numeric item")
	}
	fieldNames, fieldIDs := map[string]string{}, map[uuid.UUID]string{}
	for _, group := range []struct {
		kind   string
		fields []Attribute
	}{{"dimensions", RegisterDimensionAttributes(value.Dimensions)}, {"resources", value.Resources}, {"attributes", value.Attributes}} {
		for _, field := range group.fields {
			folded := strings.ToLower(field.Name)
			if previous, exists := fieldNames[folded]; exists {
				issues = append(issues, fmt.Sprintf("%s.%s conflicts with %s", group.kind, field.Name, previous))
			}
			fieldNames[folded] = group.kind + "." + field.Name
			if previous, exists := fieldIDs[field.ID]; exists {
				issues = append(issues, fmt.Sprintf("%s.%s UUID conflicts with %s", group.kind, field.Name, previous))
			}
			fieldIDs[field.ID] = group.kind + "." + field.Name
		}
	}
	for index, resource := range value.Resources {
		issues = append(issues, validateNumericResource(fmt.Sprintf("resources[%d]", index), resource)...)
	}
	issues = append(issues, validateAccumulationAggregates(value.Aggregates, RegisterDimensionAttributes(value.Dimensions))...)
	issues = append(issues, validateAdditionalIndexes(value.AdditionalIndexes, AccumulationRegisterKind,
		recordIndexTables(accumulationStandardFields(value.Kind), attributeNames(RegisterDimensionAttributes(value.Dimensions)),
			attributeNames(value.Resources), attributeNames(value.Attributes)))...)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return AccumulationRegisterDefinition{}, err
	}
	return value, nil
}

func reservedAccumulationRegisterName(name string) bool {
	switch foldStandardName(name) {
	case "movementkind", "recordid":
		// movementkind is not the prototype's name for the kind of movement -
		// that is RecordType - and recordid is the key of a stored row, which
		// is ours.
		return true
	default:
		return reservedStandardName(AccumulationRegisterKind, name)
	}
}

func cloneAccumulationRegisterDefinition(value AccumulationRegisterDefinition) AccumulationRegisterDefinition {
	value.Title = cloneTitle(value.Title)
	value.Aggregates = cloneAccumulationAggregates(value.Aggregates)
	value.Dimensions = cloneRegisterDimensions(value.Dimensions)
	value.Resources = cloneAttributes(value.Resources)
	value.Attributes = cloneAttributes(value.Attributes)
	value.Forms = cloneFormSet(value.Forms)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	value.ListPresentations = cloneListPresentations(value.ListPresentations)
	value.AdditionalIndexes = cloneAdditionalIndexes(value.AdditionalIndexes)
	return value
}

func accumulationRegisterFields(value AccumulationRegisterDefinition) []Attribute {
	result := make([]Attribute, 0, len(value.Dimensions)+len(value.Resources)+len(value.Attributes))
	result = append(result, RegisterDimensionAttributes(value.Dimensions)...)
	result = append(result, value.Resources...)
	result = append(result, value.Attributes...)
	return result
}

// validateNumericResource checks a resource of a register of accumulation,
// accounting or calculation where it is written: one number, or one defined
// type - what it stands for is known only beside the defined types, and is
// checked there by accumulationResourceType. A resource of these three holds
// an amount; the owner checked it on the platform for accounting and
// calculation (01.10.2026), and every resource of the materials and of the
// configurations being moved is a number or a defined money amount.
func validateNumericResource(prefix string, resource Attribute) []string {
	if single, ok := SingleType(resource.Types); !ok || single.Kind != NumberType && single.Kind != DefinedType {
		return []string{prefix + ".types must contain exactly one number or numeric defined type"}
	}
	return nil
}

// accumulationResourceType resolves a resource of a register of accumulation,
// accounting or calculation to the number it holds.
func (catalog *Catalog) accumulationResourceType(resource Attribute) (Type, error) {
	resolved, err := catalog.expandTypes(resource.Types, nil)
	if err != nil {
		return Type{}, err
	}
	if len(resolved) != 1 || resolved[0].Kind != NumberType {
		return Type{}, fmt.Errorf("resource %s must resolve to exactly one number type", resource.Name)
	}
	return resolved[0], nil
}
