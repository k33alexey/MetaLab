package metadata

import (
	"fmt"
	"io"
	"slices"
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
	Format          int                           `yaml:"format"`
	ID              uuid.UUID                     `yaml:"id"`
	Name            string                        `yaml:"name"`
	Title           LocalizedText                 `yaml:"title"`
	Kind            AccumulationRegisterKindValue `yaml:"kind"`
	Dimensions      []Attribute                   `yaml:"dimensions,omitempty"`
	Resources       []Attribute                   `yaml:"resources"`
	Attributes      []Attribute                   `yaml:"attributes,omitempty"`
	Recorders       []uuid.UUID                   `yaml:"recorders"`
	RecordSetModule *uuid.UUID                    `yaml:"record_set_module,omitempty"`
	ManagerModule   *uuid.UUID                    `yaml:"manager_module,omitempty"`
	Forms           ObjectForms                   `yaml:"forms,omitempty"`
}

func DecodeAccumulationRegister(source string, reader io.Reader, manifest project.Project) (AccumulationRegisterDefinition, error) {
	var value AccumulationRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return AccumulationRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if value.Kind != AccumulationRegisterBalance && value.Kind != AccumulationRegisterTurnover {
		issues = append(issues, "kind must be balance or turnover")
	}
	issues = append(issues, validateAttributes("dimensions", value.Dimensions, manifest, reservedAccumulationRegisterName)...)
	issues = append(issues, validateAttributes("resources", value.Resources, manifest, reservedAccumulationRegisterName)...)
	issues = append(issues, validateAttributes("attributes", value.Attributes, manifest, reservedAccumulationRegisterName)...)
	if len(value.Resources) == 0 {
		issues = append(issues, "resources must contain at least one numeric item")
	}
	if len(value.Dimensions) > 32 {
		issues = append(issues, "dimensions must not contain more than 32 items")
	}
	if len(value.Dimensions)+len(value.Resources)+len(value.Attributes) > 1500 {
		issues = append(issues, "dimensions, resources and attributes must not contain more than 1500 items in total")
	}
	fieldNames, fieldIDs := map[string]string{}, map[uuid.UUID]string{}
	for _, group := range []struct {
		kind   string
		fields []Attribute
	}{{"dimensions", value.Dimensions}, {"resources", value.Resources}, {"attributes", value.Attributes}} {
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
		if len(resource.Types) != 1 || resource.Types[0].Kind != NumberType && resource.Types[0].Kind != DefinedType {
			issues = append(issues, fmt.Sprintf("resources[%d].types must contain exactly one number or numeric defined type", index))
		}
	}
	if len(value.Recorders) == 0 || len(value.Recorders) > 128 {
		issues = append(issues, "recorders must contain 1..128 documents")
	}
	seenRecorders := map[uuid.UUID]bool{}
	for index, recorder := range value.Recorders {
		if recorder.IsZero() {
			issues = append(issues, fmt.Sprintf("recorders[%d] must be a non-zero UUID", index))
		}
		if seenRecorders[recorder] {
			issues = append(issues, fmt.Sprintf("recorders[%d] must be unique", index))
		}
		seenRecorders[recorder] = true
	}
	for _, module := range []struct {
		name string
		id   *uuid.UUID
	}{{"record_set_module", value.RecordSetModule}, {"manager_module", value.ManagerModule}} {
		if module.id != nil && module.id.IsZero() {
			issues = append(issues, module.name+" must be a non-zero UUID")
		}
	}
	if value.RecordSetModule != nil && value.ManagerModule != nil && *value.RecordSetModule == *value.ManagerModule {
		issues = append(issues, "record_set_module and manager_module must be different")
	}
	issues = append(issues, validateObjectForms(value.Forms)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return AccumulationRegisterDefinition{}, err
	}
	return value, nil
}

func reservedAccumulationRegisterName(name string) bool {
	switch strings.ToLower(name) {
	case "период", "period", "регистратор", "recorder", "номерстроки", "linenumber", "активность", "active", "виддвижения", "movementkind", "recordid":
		return true
	default:
		return false
	}
}

func cloneAccumulationRegisterDefinition(value AccumulationRegisterDefinition) AccumulationRegisterDefinition {
	value.Title = cloneTitle(value.Title)
	value.Dimensions = cloneAttributes(value.Dimensions)
	value.Resources = cloneAttributes(value.Resources)
	value.Attributes = cloneAttributes(value.Attributes)
	value.Recorders = slices.Clone(value.Recorders)
	if value.RecordSetModule != nil {
		id := *value.RecordSetModule
		value.RecordSetModule = &id
	}
	if value.ManagerModule != nil {
		id := *value.ManagerModule
		value.ManagerModule = &id
	}
	value.Forms = cloneObjectForms(value.Forms)
	return value
}

func accumulationRegisterFields(value AccumulationRegisterDefinition) []Attribute {
	result := make([]Attribute, 0, len(value.Dimensions)+len(value.Resources)+len(value.Attributes))
	result = append(result, value.Dimensions...)
	result = append(result, value.Resources...)
	result = append(result, value.Attributes...)
	return result
}

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
