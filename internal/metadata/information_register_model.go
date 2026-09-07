package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// InformationRegisterPeriodicity controls how record periods participate in a register key.
type InformationRegisterPeriodicity string

const (
	InformationRegisterPeriodNone             InformationRegisterPeriodicity = "none"
	InformationRegisterPeriodSecond           InformationRegisterPeriodicity = "second"
	InformationRegisterPeriodDay              InformationRegisterPeriodicity = "day"
	InformationRegisterPeriodMonth            InformationRegisterPeriodicity = "month"
	InformationRegisterPeriodQuarter          InformationRegisterPeriodicity = "quarter"
	InformationRegisterPeriodYear             InformationRegisterPeriodicity = "year"
	InformationRegisterPeriodRecorderPosition InformationRegisterPeriodicity = "recorder-position"
)

// InformationRegisterWriteMode selects independent or document-recorder-owned records.
type InformationRegisterWriteMode string

const (
	InformationRegisterIndependent InformationRegisterWriteMode = "independent"
	InformationRegisterRecorder    InformationRegisterWriteMode = "recorder"
)

// InformationRegisterDefinition describes one ML information register.
type InformationRegisterDefinition struct {
	Format          int                            `yaml:"format"`
	ID              uuid.UUID                      `yaml:"id"`
	Name            string                         `yaml:"name"`
	Title           LocalizedText                  `yaml:"title"`
	WriteMode       InformationRegisterWriteMode   `yaml:"write_mode"`
	Periodicity     InformationRegisterPeriodicity `yaml:"periodicity"`
	Dimensions      []Attribute                    `yaml:"dimensions,omitempty"`
	Resources       []Attribute                    `yaml:"resources,omitempty"`
	Attributes      []Attribute                    `yaml:"attributes,omitempty"`
	Recorders       []uuid.UUID                    `yaml:"recorders,omitempty"`
	RecordSetModule *uuid.UUID                     `yaml:"record_set_module,omitempty"`
	ManagerModule   *uuid.UUID                     `yaml:"manager_module,omitempty"`
	Forms           ObjectForms                    `yaml:"forms,omitempty"`
}

func DecodeInformationRegister(source string, reader io.Reader, manifest project.Project) (InformationRegisterDefinition, error) {
	var value InformationRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return InformationRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	switch value.WriteMode {
	case InformationRegisterIndependent, InformationRegisterRecorder:
	default:
		issues = append(issues, "write_mode must be independent or recorder")
	}
	switch value.Periodicity {
	case InformationRegisterPeriodNone, InformationRegisterPeriodSecond, InformationRegisterPeriodDay,
		InformationRegisterPeriodMonth, InformationRegisterPeriodQuarter, InformationRegisterPeriodYear:
	case InformationRegisterPeriodRecorderPosition:
		if value.WriteMode != InformationRegisterRecorder {
			issues = append(issues, "recorder-position periodicity requires recorder write mode")
		}
	default:
		issues = append(issues, "periodicity must be none, second, day, month, quarter, year or recorder-position")
	}
	issues = append(issues, validateAttributes("dimensions", value.Dimensions, manifest, reservedInformationRegisterName)...)
	issues = append(issues, validateAttributes("resources", value.Resources, manifest, reservedInformationRegisterName)...)
	issues = append(issues, validateAttributes("attributes", value.Attributes, manifest, reservedInformationRegisterName)...)
	if len(value.Dimensions)+len(value.Resources)+len(value.Attributes) == 0 {
		issues = append(issues, "dimensions, resources or attributes must contain at least one item")
	}
	if len(value.Dimensions) > 32 {
		issues = append(issues, "dimensions must not contain more than 32 items")
	}
	if len(value.Dimensions)+len(value.Resources)+len(value.Attributes) > 1500 {
		issues = append(issues, "dimensions, resources and attributes must not contain more than 1500 items in total")
	}
	fieldNames := map[string]string{}
	fieldIDs := map[uuid.UUID]string{}
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
	if len(value.Recorders) > 128 {
		issues = append(issues, "recorders must not contain more than 128 items")
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
	if value.WriteMode == InformationRegisterRecorder && len(value.Recorders) == 0 {
		issues = append(issues, "recorders must contain at least one document for recorder write mode")
	}
	if value.WriteMode == InformationRegisterIndependent && len(value.Recorders) != 0 {
		issues = append(issues, "recorders are only allowed for recorder write mode")
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
		return InformationRegisterDefinition{}, err
	}
	return value, nil
}

func reservedInformationRegisterName(name string) bool {
	switch strings.ToLower(name) {
	case "период", "period", "регистратор", "recorder", "номерстроки", "linenumber", "активность", "active", "recordid":
		return true
	default:
		return false
	}
}

func cloneInformationRegisterDefinition(value InformationRegisterDefinition) InformationRegisterDefinition {
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

func informationRegisterFields(value InformationRegisterDefinition) []Attribute {
	result := make([]Attribute, 0, len(value.Dimensions)+len(value.Resources)+len(value.Attributes))
	result = append(result, value.Dimensions...)
	result = append(result, value.Resources...)
	result = append(result, value.Attributes...)
	return result
}
