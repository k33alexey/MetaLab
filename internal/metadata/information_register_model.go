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
	Format int           `yaml:"format"`
	ID     uuid.UUID     `yaml:"id"`
	Name   string        `yaml:"name"`
	Title  LocalizedText `yaml:"title"` // ListPresentations and the help flag: a row of this kind is not an object a
	// person opens, so there is a list to name and no object - see
	// object_presentation.go. A record of an information register gets a name of
	// its own, which no other kind has.
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
	// DataHistorySettings is whether the records take part in data history and
	// the two flags that go with it - see data_history.go. Of the four
	// registers only this one carries them, and it is the one place where
	// data history and the full-text flag part company: the syntax assistant
	// gives data history to the information register alone, and the
	// demonstration configuration writes it on all 247 of them and on no
	// other register.
	DataHistorySettings `yaml:",inline" json:",inline"`
	RecordPresentations `yaml:",inline" json:",inline"`

	WriteMode          InformationRegisterWriteMode   `yaml:"write_mode"`
	Periodicity        InformationRegisterPeriodicity `yaml:"periodicity"`
	Dimensions         []Attribute                    `yaml:"dimensions,omitempty"`
	Resources          []Attribute                    `yaml:"resources,omitempty"`
	Attributes         []Attribute                    `yaml:"attributes,omitempty"`
	StandardAttributes []StandardAttribute            `yaml:"standard_attributes,omitempty"`
	// EditType is how a row is entered and edited; Totals asks for the extra
	// tables that answer a slice quickly; MainFilterOnPeriod puts the period
	// into the main filter. All three belong to this register and to no other -
	// see register_properties.go.
	EditType           EditType                  `yaml:"edit_type,omitempty"`
	Totals             InformationRegisterTotals `yaml:"totals,omitempty"`
	MainFilterOnPeriod bool                      `yaml:"main_filter_on_period,omitempty"`
	Forms              InformationRegisterForms  `yaml:"forms,omitempty"`
	Commands           []ObjectCommand           `yaml:"commands,omitempty"`
	Templates          []ObjectTemplate          `yaml:"templates,omitempty"`
}

func DecodeInformationRegister(source string, reader io.Reader, configuration project.Project) (InformationRegisterDefinition, error) {
	var value InformationRegisterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return InformationRegisterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateDataLockMode("data_lock", value.DataLock)...)
	issues = append(issues, validateFullTextSearch("full_text_search", value.FullTextSearch)...)
	issues = append(issues, validateDataHistory(value.DataHistorySettings)...)
	issues = append(issues, validateAdditionalIndexes(value.AdditionalIndexes, InformationRegisterKind,
		recordIndexTables(standardFieldsOfKind(InformationRegisterKind), attributeNames(value.Dimensions),
			attributeNames(value.Resources), attributeNames(value.Attributes)))...)
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
	issues = append(issues, validateListPresentations(value.ListPresentations, configuration)...)
	issues = append(issues, validateRecordPresentations(value.RecordPresentations, configuration)...)
	issues = append(issues, validateInformationRegisterProperties(value)...)
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
	value.ListPresentations = cloneListPresentations(value.ListPresentations)
	value.RecordPresentations = cloneRecordPresentations(value.RecordPresentations)
	value.AdditionalIndexes = cloneAdditionalIndexes(value.AdditionalIndexes)
	return value
}

func informationRegisterFields(value InformationRegisterDefinition) []Attribute {
	result := make([]Attribute, 0, len(value.Dimensions)+len(value.Resources)+len(value.Attributes))
	result = append(result, value.Dimensions...)
	result = append(result, value.Resources...)
	result = append(result, value.Attributes...)
	return result
}
