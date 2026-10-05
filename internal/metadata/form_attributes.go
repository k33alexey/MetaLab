package metadata

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FormAttribute is one attribute of a form: the data the form holds besides
// the elements that show it. The configurations being moved have 38625,
// 20698 and 17126 of them, and a form read without them keeps its fields
// pointing at nothing.
//
// What it carries is what the designer's list of attributes gives one (help,
// the form designer): a name and a synonym, the type, whether it is the main
// attribute, the functional options it belongs to, and which of the data
// nested in it is passed to the client. Besides those the description of a
// form keeps whether the data is saved with the object, the check of filling,
// what of it is kept in the form's settings, the rights to view and to edit it
// by role, and the columns of an attribute that is a table.
type FormAttribute struct {
	ID    uuid.UUID     `yaml:"id" json:"id"`
	Name  string        `yaml:"name" json:"name"`
	Title LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	// Types left empty is an arbitrary type, as the designer saves one and as
	// a running object's attribute takes one.
	Types []Type `yaml:"types,omitempty" json:"types,omitempty"`
	// Main says the type of this attribute decides what the form is: a form
	// whose main attribute is a catalog object edits the catalog. A form has
	// one or none.
	Main bool `yaml:"main,omitempty" json:"main,omitempty"`
	// SavedData marks the attribute as data written when the form is saved:
	// changing it marks the form modified and asks before closing (help,
	// FormAttribute.StoredData).
	SavedData    bool      `yaml:"saved_data,omitempty" json:"savedData,omitempty"`
	FillChecking FillCheck `yaml:"fill_checking,omitempty" json:"fillChecking,omitempty"`
	// FunctionalOptions are the options that switch the attribute off.
	FunctionalOptions []uuid.UUID `yaml:"functional_options,omitempty" json:"functionalOptions,omitempty"`
	// UseAlways are the paths of the data nested in the attribute that is
	// passed to the client whether an element shows it or not. Each starts
	// with the attribute's own name, and the prototype writes some of them
	// with a leading «~»; the mark is carried as written.
	UseAlways []string `yaml:"use_always,omitempty" json:"useAlways,omitempty"`
	// SaveInSettings are what of the attribute is kept in the form's settings
	// between sessions: the attribute itself by its name, or a place inside
	// its settings in the prototype's own notation («3/2», «1/0:<id>»),
	// carried as written.
	SaveInSettings []string `yaml:"save_in_settings,omitempty" json:"saveInSettings,omitempty"`
	// View and Edit are the rights to see and to change the attribute; nil is
	// the designer's default, everyone may.
	View    *FormAttributeRight   `yaml:"view,omitempty" json:"view,omitempty"`
	Edit    *FormAttributeRight   `yaml:"edit,omitempty" json:"edit,omitempty"`
	Columns []FormAttributeColumn `yaml:"columns,omitempty" json:"columns,omitempty"`
	// AdditionalColumns are columns the form adds to a table it takes from
	// the attribute's data, a table part of the object for one, named by its
	// path from the attribute.
	AdditionalColumns []FormAdditionalColumns `yaml:"additional_columns,omitempty" json:"additionalColumns,omitempty"`
}

// FormAttributeColumn is a column of an attribute that is a table. The help
// gives it what an attribute has, save being the main one and the use on the
// client.
type FormAttributeColumn struct {
	ID                uuid.UUID           `yaml:"id" json:"id"`
	Name              string              `yaml:"name" json:"name"`
	Title             LocalizedText       `yaml:"title,omitempty" json:"title,omitempty"`
	Types             []Type              `yaml:"types,omitempty" json:"types,omitempty"`
	FunctionalOptions []uuid.UUID         `yaml:"functional_options,omitempty" json:"functionalOptions,omitempty"`
	View              *FormAttributeRight `yaml:"view,omitempty" json:"view,omitempty"`
	Edit              *FormAttributeRight `yaml:"edit,omitempty" json:"edit,omitempty"`
}

// FormAdditionalColumns are the columns a form adds to one table of an
// attribute.
type FormAdditionalColumns struct {
	Table   string                `yaml:"table" json:"table"`
	Columns []FormAttributeColumn `yaml:"columns,omitempty" json:"columns,omitempty"`
}

// FormAttributeRight is a right to an attribute: one answer for every role,
// and the roles that answer otherwise.
type FormAttributeRight struct {
	Common bool            `yaml:"common" json:"common"`
	Roles  []FormRoleRight `yaml:"roles,omitempty" json:"roles,omitempty"`
}

// FormRoleRight is the answer of one role.
type FormRoleRight struct {
	Role  uuid.UUID `yaml:"role" json:"role"`
	Value bool      `yaml:"value" json:"value"`
}

// validateFormAttributes checks the attributes of one form against the form
// alone. What they reference - roles, options, the objects in their types -
// is not resolved here: the form is checked where it is edited, with no
// configuration at hand.
func validateFormAttributes(attributes []FormAttribute, ids map[uuid.UUID]bool, configuration project.Project) []string {
	var issues []string
	names := map[string]bool{}
	main := -1
	for index, attribute := range attributes {
		path := fmt.Sprintf("attributes[%d]", index)
		issues = append(issues, validateFormDataName(path, attribute.ID, attribute.Name, ids, names, "the attributes")...)
		issues = append(issues, validateTitle(path+".title", attribute.Title, configuration)...)
		issues = append(issues, validateTypesIn(path+".types", attribute.Types, placeFormAttribute)...)
		if attribute.Main {
			if main >= 0 {
				issues = append(issues, fmt.Sprintf("%s.main: attributes[%d] is the main attribute already, and a form has one", path, main))
			} else {
				main = index
			}
		}
		switch attribute.FillChecking {
		case "", DontCheckFilling, ShowFillingError:
		default:
			issues = append(issues, path+".fill_checking must be dont-check or show-error")
		}
		issues = append(issues, validateFormOptions(path+".functional_options", attribute.FunctionalOptions)...)
		for position, field := range attribute.UseAlways {
			place := fmt.Sprintf("%s.use_always[%d]", path, position)
			trimmed := strings.TrimPrefix(field, "~")
			if problems := validateFormDataPath(place, trimmed); len(problems) != 0 {
				issues = append(issues, problems...)
				continue
			}
			if first, _, _ := strings.Cut(trimmed, "."); !strings.EqualFold(first, attribute.Name) {
				issues = append(issues, place+" must start with the attribute's own name")
			}
		}
		for position, field := range attribute.SaveInSettings {
			if field == "" || strings.TrimSpace(field) != field {
				issues = append(issues, fmt.Sprintf("%s.save_in_settings[%d] must be a non-empty path without surrounding spaces", path, position))
			}
		}
		issues = append(issues, validateFormRight(path+".view", attribute.View)...)
		issues = append(issues, validateFormRight(path+".edit", attribute.Edit)...)
		issues = append(issues, validateFormColumns(path+".columns", attribute.Columns, ids, configuration)...)
		tables := map[string]bool{}
		for position, additional := range attribute.AdditionalColumns {
			place := fmt.Sprintf("%s.additional_columns[%d]", path, position)
			if problems := validateFormDataPath(place+".table", additional.Table); len(problems) != 0 {
				issues = append(issues, problems...)
			} else if first, _, _ := strings.Cut(additional.Table, "."); !strings.EqualFold(first, attribute.Name) {
				issues = append(issues, place+".table must start with the attribute's own name")
			}
			folded := strings.ToLower(additional.Table)
			if tables[folded] {
				issues = append(issues, place+".table already has its additional columns")
			}
			tables[folded] = true
			issues = append(issues, validateFormColumns(place+".columns", additional.Columns, ids, configuration)...)
		}
	}
	return issues
}

// validateFormColumns checks one set of columns, whose names are unique among
// themselves.
func validateFormColumns(path string, columns []FormAttributeColumn, ids map[uuid.UUID]bool, configuration project.Project) []string {
	var issues []string
	names := map[string]bool{}
	for index, column := range columns {
		place := fmt.Sprintf("%s[%d]", path, index)
		issues = append(issues, validateFormDataName(place, column.ID, column.Name, ids, names, "the columns")...)
		issues = append(issues, validateTitle(place+".title", column.Title, configuration)...)
		issues = append(issues, validateTypesIn(place+".types", column.Types, placeFormAttribute)...)
		issues = append(issues, validateFormOptions(place+".functional_options", column.FunctionalOptions)...)
		issues = append(issues, validateFormRight(place+".view", column.View)...)
		issues = append(issues, validateFormRight(place+".edit", column.Edit)...)
	}
	return issues
}

// validateFormDataName checks the identity of an attribute or a column: an
// identifier no other part of the form has, and a name unique in its set.
func validateFormDataName(path string, id uuid.UUID, name string, ids map[uuid.UUID]bool, names map[string]bool, set string) []string {
	var issues []string
	if id.IsZero() {
		issues = append(issues, path+".id must be a non-zero UUID")
	} else if ids[id] {
		issues = append(issues, path+".id must be unique within the form")
	}
	ids[id] = true
	if !validIdentifier(name) || utf8.RuneCountInString(name) > maxNameLength {
		issues = append(issues, path+".name must be a valid identifier of at most 255 characters")
	}
	folded := strings.ToLower(name)
	if names[folded] {
		issues = append(issues, path+".name must be unique within "+set)
	}
	names[folded] = true
	return issues
}

func validateFormOptions(path string, options []uuid.UUID) []string {
	var issues []string
	seen := map[uuid.UUID]bool{}
	for index, option := range options {
		if option.IsZero() {
			issues = append(issues, fmt.Sprintf("%s[%d] must be a non-zero UUID", path, index))
		} else if seen[option] {
			issues = append(issues, fmt.Sprintf("%s[%d] is already among the options", path, index))
		}
		seen[option] = true
	}
	return issues
}

func validateFormRight(path string, right *FormAttributeRight) []string {
	if right == nil {
		return nil
	}
	var issues []string
	seen := map[uuid.UUID]bool{}
	for index, role := range right.Roles {
		if role.Role.IsZero() {
			issues = append(issues, fmt.Sprintf("%s.roles[%d].role must be a non-zero UUID", path, index))
		} else if seen[role.Role] {
			issues = append(issues, fmt.Sprintf("%s.roles[%d].role already has its answer", path, index))
		}
		seen[role.Role] = true
	}
	return issues
}
