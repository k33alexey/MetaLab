package metadata

import (
	"fmt"
	"io"
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
	Format             int                            `yaml:"format"`
	ID                 uuid.UUID                      `yaml:"id"`
	Name               string                         `yaml:"name"`
	Title              LocalizedText                  `yaml:"title"`
	WriteMode          InformationRegisterWriteMode   `yaml:"write_mode"`
	Periodicity        InformationRegisterPeriodicity `yaml:"periodicity"`
	Dimensions         []Attribute                    `yaml:"dimensions,omitempty"`
	Resources          []Attribute                    `yaml:"resources,omitempty"`
	Attributes         []Attribute                    `yaml:"attributes,omitempty"`
	StandardAttributes []StandardAttribute            `yaml:"standard_attributes,omitempty"`
	Forms              InformationRegisterForms       `yaml:"forms,omitempty"`
	Commands           []ObjectCommand                `yaml:"commands,omitempty"`
	Templates          []ObjectTemplate               `yaml:"templates,omitempty"`
}

func DecodeInformationRegister(source string, reader io.Reader, configuration project.Project) (InformationRegisterDefinition, error) {
	var value InformationRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return InformationRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
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
	issues = append(issues, validateAttributes("dimensions", value.Dimensions, configuration, reservedInformationRegisterName)...)
	issues = append(issues, validateAttributes("resources", value.Resources, configuration, reservedInformationRegisterName)...)
	issues = append(issues, validateAttributes("attributes", value.Attributes, configuration, reservedInformationRegisterName)...)
	registerFields := []fieldGroup{
		{"dimensions", value.Dimensions}, {"resources", value.Resources}, {"attributes", value.Attributes},
	}
	issues = append(issues, validateStandardAttributes("standard_attributes", value.StandardAttributes, standardFieldsOfKind(InformationRegisterKind), configuration)...)
	issues = append(issues, validateFieldLinks(registerFields, nil, standardAttributeChoices("standard_attributes", value.StandardAttributes)...)...)
	issues = append(issues, validateAttributeUse(registerFields, nil, false, false)...)
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
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return InformationRegisterDefinition{}, err
	}
	return value, nil
}

func reservedInformationRegisterName(name string) bool {
	switch foldStandardName(name) {
	case "recordid":
		return true
	default:
		return reservedStandardName(InformationRegisterKind, name)
	}
}

func cloneInformationRegisterDefinition(value InformationRegisterDefinition) InformationRegisterDefinition {
	value.Title = cloneTitle(value.Title)
	value.Dimensions = cloneAttributes(value.Dimensions)
	value.Resources = cloneAttributes(value.Resources)
	value.Attributes = cloneAttributes(value.Attributes)
	value.Forms = cloneFormSet(value.Forms)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.StandardAttributes = cloneStandardAttributes(value.StandardAttributes)
	return value
}

func informationRegisterFields(value InformationRegisterDefinition) []Attribute {
	result := make([]Attribute, 0, len(value.Dimensions)+len(value.Resources)+len(value.Attributes))
	result = append(result, value.Dimensions...)
	result = append(result, value.Resources...)
	result = append(result, value.Attributes...)
	return result
}
