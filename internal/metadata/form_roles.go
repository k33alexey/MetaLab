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

// commonFormPrefix marks a role as pointing outside the object, at a common
// form of the configuration. It is the folder such a form lies in, because the
// folder is the only thing that tells the two apart: one report of the
// demonstration configuration keeps its own ФормаОтчета and opens
// common-forms/ФормаОтчета all the same, and by the name alone we would open
// the wrong one and never know.
const commonFormPrefix = "common-forms/"

// formReference is where the form a role names lies: among the forms of the
// object itself, or among the common forms of the configuration. A report is
// the case that made it necessary - twenty-three of the twenty-six reports in
// the demonstration configuration open a common form - but the platform allows
// it in every role of every kind, and so do we.
type formReference struct {
	name   string
	common bool
}

// parseFormReference reads what a role carries. It fails only on something that
// cannot be a form at all: a name no folder can be called, or a folder other
// than the one common forms live in. The second result is empty when the value
// is a reference, and otherwise is the rest of the message about the role,
// which its key is put in front of.
func parseFormReference(value string) (formReference, string) {
	reference := formReference{name: value}
	if after, found := strings.CutPrefix(value, commonFormPrefix); found {
		reference = formReference{name: after, common: true}
	} else if strings.ContainsRune(value, '/') {
		return formReference{}, " must name a form of the object or one under " + commonFormPrefix
	}
	if project.SubordinateName(reference.name) != nil {
		return formReference{}, " must be the name of a form"
	}
	return reference, ""
}

// validateFormSlots checks that every filled role carries something that can
// be a form: the name of a form, which is a folder named after itself, either
// beside the object or among the common forms. That the form is actually there
// is checked against those folders, and not here.
//
// Two roles may well name one form. The prototype's own catalog of users does
// exactly that - its ФормаСписка is both the main list form and the main
// choice form - so refusing it would lose a real configuration at import.
func validateFormSlots(slots []formSlot) []string {
	var issues []string
	for _, slot := range slots {
		if slot.form == "" {
			continue
		}
		if _, problem := parseFormReference(slot.form); problem != "" {
			issues = append(issues, slot.key+problem)
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

// RegisterForms is the role set of a register of totals: it shows a list of its
// records and nothing else. A record of one is not opened on its own - it is
// written by whatever moves the register - so there is no role for it, and the
// list has its auxiliary form beside it like any main form.
type RegisterForms struct {
	List          string `yaml:"list,omitempty" json:"list,omitempty"`
	AuxiliaryList string `yaml:"auxiliary_list,omitempty" json:"auxiliaryList,omitempty"`
}

func (forms RegisterForms) slots() []formSlot {
	return []formSlot{
		{"forms.list", forms.List},
		{"forms.auxiliary_list", forms.AuxiliaryList},
	}
}

// InformationRegisterForms is the role set of a register of information. It has
// one role more than a register of totals: a record of it is edited by hand,
// which no other register allows, so the record has a form of its own.
type InformationRegisterForms struct {
	List   string `yaml:"list,omitempty" json:"list,omitempty"`
	Record string `yaml:"record,omitempty" json:"record,omitempty"`

	AuxiliaryList   string `yaml:"auxiliary_list,omitempty" json:"auxiliaryList,omitempty"`
	AuxiliaryRecord string `yaml:"auxiliary_record,omitempty" json:"auxiliaryRecord,omitempty"`
}

func (forms InformationRegisterForms) slots() []formSlot {
	return []formSlot{
		{"forms.list", forms.List},
		{"forms.record", forms.Record},
		{"forms.auxiliary_list", forms.AuxiliaryList},
		{"forms.auxiliary_record", forms.AuxiliaryRecord},
	}
}

// SingleRoleForms is the role set of a kind that opens one thing and nothing
// else: a document journal opens its list, a filter criterion opens what it
// found, a data processor opens itself. There is no second role to tell the
// first one apart from, so it is just the main form, with its auxiliary beside
// it.
type SingleRoleForms struct {
	Main      string `yaml:"main,omitempty" json:"main,omitempty"`
	Auxiliary string `yaml:"auxiliary,omitempty" json:"auxiliary,omitempty"`
}

func (forms SingleRoleForms) slots() []formSlot {
	return []formSlot{{"forms.main", forms.Main}, {"forms.auxiliary", forms.Auxiliary}}
}
