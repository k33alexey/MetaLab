package metadata

import (
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
)

// formSlot is one role of form an object may name: the key the role is written
// under in the description, and the form standing in it.
//
// Every kind has its own set of roles, and the set is what tells the kinds
// apart here: a register opens a list and nothing else, a catalog opens five
// different things. The sets are collected as slots rather than read field by
// field because two very different places need the same list - the description
// is checked against the naming rules, and the object's folder is checked for
// the forms the names point at. Read field by field, the two lists drift, and
// a role then exists in one of them only.
type formSlot struct {
	key  string
	form string
}

// role is what a message about this slot calls it: forms.auxiliary_list is the
// auxiliary list form. Derived rather than written down, so a role added to a
// set is named the same way in both places by construction.
func (slot formSlot) role() string {
	return strings.ReplaceAll(strings.TrimPrefix(slot.key, "forms."), "_", " ") + " form"
}

// ObjectForms are the roles of form every reference object has: the object
// itself, its list, and choosing one out of that list. Each has an auxiliary
// form beside it.
//
// An auxiliary form is not a second main form, and it does not depend on one:
// the platform opens it exactly when the main form is missing or does not fit
// the client it has to be shown in. So an auxiliary form named alone is a
// complete answer, not half of one.
type ObjectForms struct {
	Object string `yaml:"object,omitempty" json:"object,omitempty"`
	List   string `yaml:"list,omitempty" json:"list,omitempty"`
	Choice string `yaml:"choice,omitempty" json:"choice,omitempty"`

	AuxiliaryObject string `yaml:"auxiliary_object,omitempty" json:"auxiliaryObject,omitempty"`
	AuxiliaryList   string `yaml:"auxiliary_list,omitempty" json:"auxiliaryList,omitempty"`
	AuxiliaryChoice string `yaml:"auxiliary_choice,omitempty" json:"auxiliaryChoice,omitempty"`
}

func (forms ObjectForms) slots() []formSlot {
	return []formSlot{
		{"forms.object", forms.Object},
		{"forms.list", forms.List},
		{"forms.choice", forms.Choice},
		{"forms.auxiliary_object", forms.AuxiliaryObject},
		{"forms.auxiliary_list", forms.AuxiliaryList},
		{"forms.auxiliary_choice", forms.AuxiliaryChoice},
	}
}

// HierarchicalObjectForms are the roles of an object whose hierarchy has
// folders. A folder is edited and chosen apart from an item - it holds almost
// none of the attributes an item holds - so it brings two roles of its own,
// each with its auxiliary form.
//
// Only a hierarchy of folders and items brings them. An object with no
// hierarchy, or one whose hierarchy is of items alone, has no folder to open a
// form of: there the roles exist in the type and stay empty, and naming one is
// caught where the two are checked against each other.
type HierarchicalObjectForms struct {
	ObjectForms `yaml:",inline"`

	Folder       string `yaml:"folder,omitempty" json:"folder,omitempty"`
	FolderChoice string `yaml:"folder_choice,omitempty" json:"folderChoice,omitempty"`

	AuxiliaryFolder       string `yaml:"auxiliary_folder,omitempty" json:"auxiliaryFolder,omitempty"`
	AuxiliaryFolderChoice string `yaml:"auxiliary_folder_choice,omitempty" json:"auxiliaryFolderChoice,omitempty"`
}

func (forms HierarchicalObjectForms) slots() []formSlot {
	return append(forms.ObjectForms.slots(),
		formSlot{"forms.folder", forms.Folder},
		formSlot{"forms.folder_choice", forms.FolderChoice},
		formSlot{"forms.auxiliary_folder", forms.AuxiliaryFolder},
		formSlot{"forms.auxiliary_folder_choice", forms.AuxiliaryFolderChoice},
	)
}

// folderSlots are the roles a folder brings, apart from the ones every object
// has. They are the ones a hierarchy without folders makes meaningless.
func (forms HierarchicalObjectForms) folderSlots() []formSlot {
	return []formSlot{
		{"forms.folder", forms.Folder},
		{"forms.folder_choice", forms.FolderChoice},
		{"forms.auxiliary_folder", forms.AuxiliaryFolder},
		{"forms.auxiliary_folder_choice", forms.AuxiliaryFolderChoice},
	}
}

// validateFormSlots checks that every named slot carries something that can be
// a form: a slot holds the name of a form, and a form is a folder named after
// itself. That the form is actually there is checked against the object's own
// folder, where forms live, and not here.
//
// Two slots may well name one form. The prototype's own catalog of users does
// exactly that - its ФормаСписка is both the main list form and the main
// choice form - so refusing it would lose a real configuration at import.
func validateFormSlots(slots []formSlot) []string {
	var issues []string
	for _, slot := range slots {
		if slot.form == "" {
			continue
		}
		if project.SubordinateName(slot.form) != nil {
			issues = append(issues, slot.key+" must be the name of a form")
		}
	}
	slices.Sort(issues)
	return issues
}

// validateFolderForms checks the folder roles against the hierarchy that would
// give them something to open. A folder form on an object with no folders is a
// form nothing ever opens - it reads as working and does nothing, the same way
// a level count nobody limits does.
func validateFolderForms(forms HierarchicalObjectForms, hierarchy Hierarchy) []string {
	if hierarchy.Enabled && hierarchy.Kind == FoldersAndItemsHierarchy {
		return nil
	}
	var issues []string
	for _, slot := range forms.folderSlots() {
		if slot.form != "" {
			issues = append(issues, slot.key+" needs a hierarchy of folders and items")
		}
	}
	slices.Sort(issues)
	return issues
}
