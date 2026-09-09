package metadata

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	MaxManagedFormElements = 10_000
	MaxManagedFormDepth    = 64
	MaxGroupChildren       = 1_000
)

type FormElementKind string

const (
	FormElementGroup  FormElementKind = "group"
	FormElementField  FormElementKind = "field"
	FormElementLabel  FormElementKind = "label"
	FormElementTable  FormElementKind = "table"
	FormElementButton FormElementKind = "button"
)

type FormOrientation string

const (
	FormVertical   FormOrientation = "vertical"
	FormHorizontal FormOrientation = "horizontal"
)

// ManagedForm is the versioned source model edited by the visual form designer.
// Data bindings and commands are added by the next form-designer iteration.
type ManagedForm struct {
	Format int                  `yaml:"format" json:"format"`
	ID     uuid.UUID            `yaml:"id" json:"id"`
	Name   string               `yaml:"name" json:"name"`
	Title  LocalizedText        `yaml:"title" json:"title"`
	Kind   FormKind             `yaml:"kind" json:"kind"`
	Module *uuid.UUID           `yaml:"module,omitempty" json:"module,omitempty"`
	Items  []ManagedFormElement `yaml:"items,omitempty" json:"items"`
}

// ManagedFormElement is one stable node in a managed form tree. Hidden and
// disabled use inverse flags so omitted YAML retains the useful true defaults.
type ManagedFormElement struct {
	ID          uuid.UUID            `yaml:"id" json:"id"`
	Name        string               `yaml:"name" json:"name"`
	Kind        FormElementKind      `yaml:"kind" json:"kind"`
	Title       LocalizedText        `yaml:"title,omitempty" json:"title"`
	Hidden      bool                 `yaml:"hidden,omitempty" json:"hidden"`
	Disabled    bool                 `yaml:"disabled,omitempty" json:"disabled"`
	ReadOnly    bool                 `yaml:"read_only,omitempty" json:"readOnly"`
	Orientation FormOrientation      `yaml:"orientation,omitempty" json:"orientation,omitempty"`
	Children    []ManagedFormElement `yaml:"children,omitempty" json:"children"`
}

// DecodeManagedForm reads and validates one strict managed-form YAML document.
func DecodeManagedForm(source string, reader io.Reader, manifest project.Project) (ManagedForm, error) {
	var value ManagedForm
	if err := decodeStrict(source, reader, &value); err != nil {
		return ManagedForm{}, err
	}
	if err := ValidateManagedForm(source, value, manifest); err != nil {
		return ManagedForm{}, err
	}
	return value, nil
}

// ValidateManagedForm validates a form received from either YAML or Studio JSON.
func ValidateManagedForm(source string, value ManagedForm, manifest project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	switch value.Kind {
	case ObjectForm, ListForm, ChoiceForm, CommonForm:
	default:
		issues = append(issues, "kind must be object, list, choice or common")
	}
	if value.Module != nil && value.Module.IsZero() {
		issues = append(issues, "module must be a non-zero UUID")
	}

	type pending struct {
		element ManagedFormElement
		path    string
		depth   int
	}
	stack := make([]pending, 0, len(value.Items))
	for index := len(value.Items) - 1; index >= 0; index-- {
		stack = append(stack, pending{element: value.Items[index], path: fmt.Sprintf("items[%d]", index), depth: 1})
	}
	names, ids, count := map[string]bool{}, map[uuid.UUID]bool{}, 0
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if count > MaxManagedFormElements {
			issues = append(issues, fmt.Sprintf("items must not contain more than %d elements", MaxManagedFormElements))
			break
		}
		if current.depth > MaxManagedFormDepth {
			issues = append(issues, fmt.Sprintf("%s exceeds maximum nesting depth %d", current.path, MaxManagedFormDepth))
			continue
		}
		item := current.element
		if item.ID.IsZero() {
			issues = append(issues, current.path+".id must be a non-zero UUID")
		} else if ids[item.ID] {
			issues = append(issues, current.path+".id must be unique")
		}
		ids[item.ID] = true
		if !validIdentifier(item.Name) || utf8.RuneCountInString(item.Name) > 128 {
			issues = append(issues, current.path+".name must be a valid identifier of at most 128 characters")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, current.path+".name must be unique within the form")
		}
		names[folded] = true
		if len(item.Title) != 0 {
			issues = append(issues, validateTitle(current.path+".title", item.Title, manifest)...)
		}
		switch item.Kind {
		case FormElementGroup:
			if item.ReadOnly {
				issues = append(issues, current.path+".read_only is not allowed for a group")
			}
			if item.Orientation != FormVertical && item.Orientation != FormHorizontal {
				issues = append(issues, current.path+".orientation must be vertical or horizontal")
			}
			if len(item.Children) > MaxGroupChildren {
				issues = append(issues, fmt.Sprintf("%s.children must not contain more than %d elements", current.path, MaxGroupChildren))
				continue
			}
			for index := len(item.Children) - 1; index >= 0; index-- {
				stack = append(stack, pending{element: item.Children[index], path: fmt.Sprintf("%s.children[%d]", current.path, index), depth: current.depth + 1})
			}
		case FormElementField, FormElementLabel, FormElementTable, FormElementButton:
			if len(item.Children) != 0 {
				issues = append(issues, current.path+".children are allowed only for groups")
			}
			if item.Orientation != "" {
				issues = append(issues, current.path+".orientation is allowed only for groups")
			}
			if item.ReadOnly && item.Kind != FormElementField && item.Kind != FormElementTable {
				issues = append(issues, current.path+".read_only is allowed only for fields and tables")
			}
		default:
			issues = append(issues, current.path+".kind must be group, field, label, table or button")
		}
	}
	return issuesError(source, value.Format, issues)
}
