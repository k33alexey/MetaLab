package metadata

import (
	"io"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ReportDefinition describes one report: what it is built from and how it is
// shown. A report keeps no data of its own - its attributes and table parts
// live only while it runs, and that is why it has no table.
type ReportDefinition struct {
	Format     int           `yaml:"format" json:"format"`
	ID         uuid.UUID     `yaml:"id" json:"id"`
	Name       string        `yaml:"name" json:"name"`
	Title      LocalizedText `yaml:"title" json:"title"`
	Attributes []Attribute   `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts []TablePart   `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	// MainSchema is the composition schema the report is built by. It names
	// one of the report's own templates: a schema is a kind of template, not a
	// thing beside them.
	MainSchema *uuid.UUID `yaml:"main_schema,omitempty" json:"mainSchema,omitempty"`
	// VariantsStorage and SettingsStorage are where saved variants and
	// settings of this report are kept, when it does not use the common ones.
	VariantsStorage *uuid.UUID `yaml:"variants_storage,omitempty" json:"variantsStorage,omitempty"`
	SettingsStorage *uuid.UUID `yaml:"settings_storage,omitempty" json:"settingsStorage,omitempty"`
	// Forms of a report may be its own or common to the configuration, and
	// the second is the usual case rather than the exception.
	Forms     ReportForms      `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands  []ObjectCommand  `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// ReportForms are the roles a report shows itself through: the report itself,
// its settings and one of its variants. The first two have an auxiliary form
// beside them; the variant has none, and that asymmetry is the platform's own -
// the syntax assistant lists an auxiliary form for the report and for the
// settings, and none for the variant.
type ReportForms struct {
	Main     string `yaml:"main,omitempty" json:"main,omitempty"`
	Settings string `yaml:"settings,omitempty" json:"settings,omitempty"`
	Variant  string `yaml:"variant,omitempty" json:"variant,omitempty"`

	Auxiliary         string `yaml:"auxiliary,omitempty" json:"auxiliary,omitempty"`
	AuxiliarySettings string `yaml:"auxiliary_settings,omitempty" json:"auxiliarySettings,omitempty"`
}

func (forms ReportForms) slots() []formSlot {
	return []formSlot{
		{"forms.main", forms.Main},
		{"forms.settings", forms.Settings},
		{"forms.variant", forms.Variant},
		{"forms.auxiliary", forms.Auxiliary},
		{"forms.auxiliary_settings", forms.AuxiliarySettings},
	}
}

// DataProcessorDefinition describes one data processor. It is a report without
// the parts that exist for showing numbers: no composition schema, no variants
// and no settings to store.
type DataProcessorDefinition struct {
	Format     int              `yaml:"format" json:"format"`
	ID         uuid.UUID        `yaml:"id" json:"id"`
	Name       string           `yaml:"name" json:"name"`
	Title      LocalizedText    `yaml:"title" json:"title"`
	Attributes []Attribute      `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts []TablePart      `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
	Forms      SingleRoleForms  `yaml:"forms,omitempty" json:"forms,omitempty"`
	Commands   []ObjectCommand  `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates  []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// DecodeReport reads and validates one report.
func DecodeReport(source string, reader io.Reader, configuration project.Project) (ReportDefinition, error) {
	var value ReportDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ReportDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateRunningObjectShape(value.Attributes, value.TableParts, configuration, reservedReportName)...)
	for name, id := range map[string]*uuid.UUID{
		"main_schema": value.MainSchema, "variants_storage": value.VariantsStorage,
		"settings_storage": value.SettingsStorage,
	} {
		if id != nil && id.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return ReportDefinition{}, err
	}
	return value, nil
}

// DecodeDataProcessor reads and validates one data processor.
func DecodeDataProcessor(source string, reader io.Reader, configuration project.Project) (DataProcessorDefinition, error) {
	var value DataProcessorDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return DataProcessorDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateRunningObjectShape(value.Attributes, value.TableParts, configuration, reservedReportName)...)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return DataProcessorDefinition{}, err
	}
	return value, nil
}

// validateRunningObjectShape checks what a report and a data processor share:
// attributes and table parts that exist only while the object runs. They are
// checked like any others - a name is a name and a type is a type whether or
// not the value is ever written down.
func validateRunningObjectShape(attributes []Attribute, parts []TablePart, configuration project.Project, reserved func(string) bool) []string {
	issues := validateAttributes("attributes", attributes, configuration, reserved)
	names := map[string]bool{}
	for _, attribute := range attributes {
		names[strings.ToLower(attribute.Name)] = true
	}
	issues = append(issues, validateTableParts(parts, names, configuration, reserved)...)
	issues = append(issues, validateFieldLinks([]fieldGroup{{"attributes", attributes}}, parts)...)
	return append(issues, validateAttributeUse([]fieldGroup{{"attributes", attributes}}, parts, false, false)...)
}

// reservedReportName keeps the one standard attribute a running object has.
func reservedReportName(name string) bool {
	switch strings.ToLower(name) {
	case "ссылка", "ref", "номерстроки", "linenumber":
		return true
	default:
		return false
	}
}

func cloneReport(value ReportDefinition) ReportDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	for _, id := range []**uuid.UUID{&value.MainSchema, &value.VariantsStorage, &value.SettingsStorage} {
		if *id != nil {
			copied := **id
			*id = &copied
		}
	}
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	return value
}

func cloneDataProcessor(value DataProcessorDefinition) DataProcessorDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.Forms = cloneFormSet(value.Forms)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	return value
}

// Report returns one report by name, folded case.
func (catalog *Catalog) Report(name string) (ReportDefinition, bool) {
	index, ok := catalog.reportByName[strings.ToLower(name)]
	if !ok {
		return ReportDefinition{}, false
	}
	return cloneReport(catalog.Reports[index]), true
}

// DataProcessor returns one data processor by name, folded case.
func (catalog *Catalog) DataProcessor(name string) (DataProcessorDefinition, bool) {
	index, ok := catalog.dataProcessorByName[strings.ToLower(name)]
	if !ok {
		return DataProcessorDefinition{}, false
	}
	return cloneDataProcessor(catalog.DataProcessors[index]), true
}

// runningObject is a report or a data processor seen only through the fields
// their identity is checked by. Both keep no data, and both are checked the
// same way, so one loop covers them instead of two that drift apart.
type runningObject struct {
	id         uuid.UUID
	name       string
	attributes []Attribute
	parts      []TablePart
}

func reportsAsRunning(items []ReportDefinition) []runningObject {
	result := make([]runningObject, 0, len(items))
	for _, item := range items {
		result = append(result, runningObject{item.ID, item.Name, item.Attributes, item.TableParts})
	}
	return result
}

func dataProcessorsAsRunning(items []DataProcessorDefinition) []runningObject {
	result := make([]runningObject, 0, len(items))
	for _, item := range items {
		result = append(result, runningObject{item.ID, item.Name, item.Attributes, item.TableParts})
	}
	return result
}
