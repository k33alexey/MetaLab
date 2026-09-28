package metadata

import "github.com/k33alexey/MetaLab/internal/project"

// How an object is named to the person using it, and what the platform offers
// for it. Eight properties, and they are the object's own rather than a form's:
// a list opened from three different places is called the same thing in all
// three, because the name belongs to the kind.
//
// The set is not the same for every kind, and the plan for this point said it
// was. The syntax assistant's property list for each metadata object says
// otherwise: a presentation of the object exists only where a row is a thing a
// person opens on its own - a catalog item, a document, a task, a business
// process, an account, a kind of characteristic, a kind of calculation, a node
// of an exchange plan. A register has a list and no object; an information
// register has a record instead. That is why only those eight kinds carry the
// whole of what is below.
//
// The set is built out of pieces rather than carried whole, so that a kind gets
// the fields the prototype gives it and no others. A register with a field for
// the presentation of an object would let a configuration name something the
// platform never shows, and the strictest refusal available is the one that
// happens when the file is read - which is what a missing field gives.
//
// The fields keep the prototype's own names rather than shorter ones. A field
// called List here would be shadowed by the List a catalog already has - its
// page size and search fields - and a field unreachable by its plain name is a
// field someone will one day read as empty.

// ObjectExplanation is the two texts every kind in this group has, whatever else
// it has: the sentence the object is offered by, and the developer's own note.
type ObjectExplanation struct {
	// Explanation is the sentence shown where the object is offered - beside the
	// command that opens it in the panel of actions. It is localized, like every
	// other text here: it is read by a person.
	Explanation LocalizedText `yaml:"explanation,omitempty" json:"explanation,omitempty"`
	// Comment is for the developer and is not localized - nobody but the team
	// reads it. Every metadata object of the prototype has one, including the
	// ones that have nothing else from this group.
	Comment string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

// ListPresentations is what a kind with a list carries. Seven kinds have exactly
// this: a document journal, the four registers, an enumeration and a filter
// criterion - although the first five add the help flag below and the last two
// pointedly do not have it.
type ListPresentations struct {
	// ListPresentation is what the whole of them is called - "Товары" - and the
	// extended one is the longer wording of that.
	ListPresentation         LocalizedText `yaml:"list_presentation,omitempty" json:"listPresentation,omitempty"`
	ExtendedListPresentation LocalizedText `yaml:"extended_list_presentation,omitempty" json:"extendedListPresentation,omitempty"`
	ObjectExplanation        `yaml:",inline" json:",inline"`
	// UseStandardCommands decides whether the platform offers its own commands
	// for this object - opening the list, creating one, and the rest.
	UseStandardCommands bool `yaml:"use_standard_commands,omitempty" json:"useStandardCommands,omitempty"`
}

// RunningObjectPresentations is what a kind with nothing to list carries: a
// report and a data processor show their own result, not rows of their own. A
// subsystem has the same minus the standard commands, which it has none of.
type RunningObjectPresentations struct {
	ObjectExplanation `yaml:",inline" json:",inline"`
	// ExtendedPresentation is the longer name of the report or the data
	// processor, shown where there is room for it. The eight object kinds have
	// two of these - one for the object, one for the list; a report shows its
	// own result and has one.
	ExtendedPresentation LocalizedText `yaml:"extended_presentation,omitempty" json:"extendedPresentation,omitempty"`
	// UseStandardCommands offers the platform's own command to open this report
	// or run this data processor.
	UseStandardCommands bool `yaml:"use_standard_commands,omitempty" json:"useStandardCommands,omitempty"`
	// IncludeHelpInContents puts this object's help topic into the table of
	// contents of the configuration's help.
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
}

// RecordPresentations is what an information register adds and no other kind
// has: its row is not an object a person opens but a record, so the record is
// what gets a name.
type RecordPresentations struct {
	RecordPresentation         LocalizedText `yaml:"record_presentation,omitempty" json:"recordPresentation,omitempty"`
	ExtendedRecordPresentation LocalizedText `yaml:"extended_record_presentation,omitempty" json:"extendedRecordPresentation,omitempty"`
}

// Presentations is the whole of it, for the eight kinds whose row is a thing a
// person opens on its own.
type Presentations struct {
	// ObjectPresentation is what one of these is called - "Товар", not "Товары".
	// The extended one is the same name where there is room for a longer
	// wording, which is the title of the window that opens one.
	ObjectPresentation         LocalizedText `yaml:"object_presentation,omitempty" json:"objectPresentation,omitempty"`
	ExtendedObjectPresentation LocalizedText `yaml:"extended_object_presentation,omitempty" json:"extendedObjectPresentation,omitempty"`
	ListPresentations          `yaml:",inline" json:",inline"`
	// IncludeHelpInContents puts this object's help topic into the table of
	// contents of the configuration's help. The topic itself is not a property:
	// it is content, kept in a file of its own.
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
}

func validatePresentations(presentation Presentations, configuration project.Project) []string {
	issues := validateLocalizedTexts(configuration, map[string]LocalizedText{
		"object_presentation":          presentation.ObjectPresentation,
		"extended_object_presentation": presentation.ExtendedObjectPresentation,
	})
	return append(issues, validateListPresentations(presentation.ListPresentations, configuration)...)
}

func validateListPresentations(presentation ListPresentations, configuration project.Project) []string {
	return validateLocalizedTexts(configuration, map[string]LocalizedText{
		"list_presentation":          presentation.ListPresentation,
		"extended_list_presentation": presentation.ExtendedListPresentation,
		"explanation":                presentation.Explanation,
	})
}

func validateRunningObjectPresentations(presentation RunningObjectPresentations, configuration project.Project) []string {
	return validateLocalizedTexts(configuration, map[string]LocalizedText{
		"explanation":           presentation.Explanation,
		"extended_presentation": presentation.ExtendedPresentation,
	})
}

func validateRecordPresentations(presentation RecordPresentations, configuration project.Project) []string {
	return validateLocalizedTexts(configuration, map[string]LocalizedText{
		"record_presentation":          presentation.RecordPresentation,
		"extended_record_presentation": presentation.ExtendedRecordPresentation,
	})
}

// validateLocalizedTexts checks the ones that were written down and leaves the
// rest alone. An empty presentation is the ordinary case - the prototype writes
// every property on every object and fills a minority of them - and it must not
// fail the check that a text carries at least one translation.
func validateLocalizedTexts(configuration project.Project, texts map[string]LocalizedText) []string {
	var issues []string
	for name, text := range texts {
		if len(text) > 0 {
			issues = append(issues, validateTitle(name, text, configuration)...)
		}
	}
	return issues
}

func clonePresentations(presentation Presentations) Presentations {
	presentation.ObjectPresentation = cloneTitle(presentation.ObjectPresentation)
	presentation.ExtendedObjectPresentation = cloneTitle(presentation.ExtendedObjectPresentation)
	presentation.ListPresentations = cloneListPresentations(presentation.ListPresentations)
	return presentation
}

func cloneListPresentations(presentation ListPresentations) ListPresentations {
	presentation.ListPresentation = cloneTitle(presentation.ListPresentation)
	presentation.ExtendedListPresentation = cloneTitle(presentation.ExtendedListPresentation)
	presentation.Explanation = cloneTitle(presentation.Explanation)
	return presentation
}

func cloneRunningObjectPresentations(presentation RunningObjectPresentations) RunningObjectPresentations {
	presentation.Explanation = cloneTitle(presentation.Explanation)
	return presentation
}

func cloneRecordPresentations(presentation RecordPresentations) RecordPresentations {
	presentation.RecordPresentation = cloneTitle(presentation.RecordPresentation)
	presentation.ExtendedRecordPresentation = cloneTitle(presentation.ExtendedRecordPresentation)
	return presentation
}
