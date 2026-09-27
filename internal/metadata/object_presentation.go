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
// The fields keep the prototype's own names rather than shorter ones. A field
// called List here would be shadowed by the List a catalog already has - its
// page size and search fields - and a field unreachable by its plain name is a
// field someone will one day read as empty.
type Presentations struct {
	// ObjectPresentation is what one of these is called - "Товар", not "Товары".
	// The extended one is the same name where there is room for a longer
	// wording, which is the title of the window that opens one.
	ObjectPresentation         LocalizedText `yaml:"object_presentation,omitempty" json:"objectPresentation,omitempty"`
	ExtendedObjectPresentation LocalizedText `yaml:"extended_object_presentation,omitempty" json:"extendedObjectPresentation,omitempty"`
	// ListPresentation is what the whole of them is called - "Товары" - and the
	// extended one is the longer wording of that.
	ListPresentation         LocalizedText `yaml:"list_presentation,omitempty" json:"listPresentation,omitempty"`
	ExtendedListPresentation LocalizedText `yaml:"extended_list_presentation,omitempty" json:"extendedListPresentation,omitempty"`
	// Explanation is the sentence shown where the object is offered - beside the
	// command that opens it in the panel of actions. It is localized, like every
	// other text here: it is read by a person.
	Explanation LocalizedText `yaml:"explanation,omitempty" json:"explanation,omitempty"`
	// Comment is for the developer and is not localized - nobody but the team
	// reads it. Every metadata object of the prototype has one, including the
	// ones that have nothing else from this group.
	Comment string `yaml:"comment,omitempty" json:"comment,omitempty"`
	// UseStandardCommands decides whether the platform offers its own commands
	// for this object - opening the list, creating one, and the rest.
	UseStandardCommands bool `yaml:"use_standard_commands,omitempty" json:"useStandardCommands,omitempty"`
	// IncludeHelpInContents puts this object's help topic into the table of
	// contents of the configuration's help. The topic itself is not a property:
	// it is content, kept in a file of its own.
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
}

// validatePresentations checks the texts. Only what was written down is
// checked: a presentation left empty is the ordinary case - the prototype writes
// all eight properties on every object and fills a minority of them - and an
// empty localized text would otherwise fail the check that a title has at least
// one translation.
func validatePresentations(presentation Presentations, configuration project.Project) []string {
	var issues []string
	for name, text := range map[string]LocalizedText{
		"object_presentation":          presentation.ObjectPresentation,
		"extended_object_presentation": presentation.ExtendedObjectPresentation,
		"list_presentation":            presentation.ListPresentation,
		"extended_list_presentation":   presentation.ExtendedListPresentation,
		"explanation":                  presentation.Explanation,
	} {
		if len(text) > 0 {
			issues = append(issues, validateTitle(name, text, configuration)...)
		}
	}
	return issues
}

func clonePresentations(presentation Presentations) Presentations {
	presentation.ObjectPresentation = cloneTitle(presentation.ObjectPresentation)
	presentation.ExtendedObjectPresentation = cloneTitle(presentation.ExtendedObjectPresentation)
	presentation.ListPresentation = cloneTitle(presentation.ListPresentation)
	presentation.ExtendedListPresentation = cloneTitle(presentation.ExtendedListPresentation)
	presentation.Explanation = cloneTitle(presentation.Explanation)
	return presentation
}
