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
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"` // RunningObjectPresentations: a report shows its own result, so there is no
	// list of rows to name - see object_presentation.go.
	RunningObjectPresentations `yaml:",inline" json:",inline"`

	Attributes []Attribute `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	TableParts []TablePart `yaml:"table_parts,omitempty" json:"tableParts,omitempty"`
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
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"` // RunningObjectPresentations: a report shows its own result, so there is no
	// list of rows to name - see object_presentation.go.
	RunningObjectPresentations `yaml:",inline" json:",inline"`

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
	issues = append(issues, validateRunningObjectShape(value.Attributes, value.TableParts, configuration)...)
	issues = append(issues, validateRunningObjectPresentations(value.RunningObjectPresentations, configuration)...)
	for name, id := range map[string]*uuid.UUID{
		"main_schema": value.MainSchema, "variants_storage": value.VariantsStorage,
		"settings_storage": value.SettingsStorage,
	} {
		if id != nil && id.IsZero() {
			issues = append(issues, name+" must be a non-zero UUID")
		}
	}
	issues = append(issues, validateMainSchema(value.MainSchema, value.Templates)...)
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
	issues = append(issues, validateRunningObjectShape(value.Attributes, value.TableParts, configuration)...)
	issues = append(issues, validateRunningObjectPresentations(value.RunningObjectPresentations, configuration)...)
	issues = append(issues, validateFormSlots(value.Forms.slots())...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return DataProcessorDefinition{}, err
	}
	return value, nil
}

// validateMainSchema checks that the main schema of a report is one of its
// own templates, and one that holds a composition schema. The designer's help
// says a report may have several schemas and one of them is chosen as the
// main one; the configurations being moved name one in 341, 92 and 157
// reports, every time a template of that same report holding a schema.
func validateMainSchema(schema *uuid.UUID, templates []ObjectTemplate) []string {
	if schema == nil || schema.IsZero() {
		return nil
	}
	for _, template := range templates {
		if template.ID != *schema {
			continue
		}
		if template.Kind != CompositionSchema {
			return []string{"main_schema names template " + template.Name + ", which holds " + string(template.Kind) + ", not a composition schema"}
		}
		return nil
	}
	return []string{"main_schema must name one of the report's own templates"}
}

// validateRunningObjectShape checks what a report and a data processor share:
// attributes and table parts that exist only while the object runs. They are
// checked like any others - a name is a name and a type is a type whether or
// not the value is ever written down.
func validateRunningObjectShape(attributes []Attribute, parts []TablePart, configuration project.Project) []string {
	// A running object has no standard attributes, so no name is taken: the
	// help lists none for the object, and a row of its table part has only
	// its line number, which is a name inside the part. The configurations
	// being moved name an attribute of a data processor «Ссылка» six times
	// and «НомерСтроки» once.
	// An attribute of the object itself takes any type, a value table or a
	// standard period as much as a string; one of its table parts keeps to
	// what a stored field takes - the configurations being moved never give
	// one anything else, and the help says nothing to the contrary.
	issues := validateAttributesIn("attributes", attributes, configuration, nil, placeRunningObject)
	names := map[string]bool{}
	for _, attribute := range attributes {
		names[strings.ToLower(attribute.Name)] = true
	}
	// A running object's table part is not stored: its rows live as long as the
	// report does. So no width for a line number, and nothing for a part to
	// belong to either.
	issues = append(issues, validateTableParts(parts, names, configuration, nil, tablePartRules{})...)
	issues = append(issues, validateFieldLinks([]fieldGroup{{"attributes", attributes}}, parts, tablePartStandardChoices(parts)...)...)
	return append(issues, validateAttributeUse([]fieldGroup{{"attributes", attributes}}, parts, false, false)...)
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
	value.RunningObjectPresentations = cloneRunningObjectPresentations(value.RunningObjectPresentations)
	return value
}

func cloneDataProcessor(value DataProcessorDefinition) DataProcessorDefinition {
	value.Title = cloneTitle(value.Title)
	value.Attributes = cloneAttributes(value.Attributes)
	value.TableParts = cloneTableParts(value.TableParts)
	value.Forms = cloneFormSet(value.Forms)
	value.Commands = cloneObjectCommands(value.Commands)
	value.Templates = cloneObjectTemplates(value.Templates)
	value.RunningObjectPresentations = cloneRunningObjectPresentations(value.RunningObjectPresentations)
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
