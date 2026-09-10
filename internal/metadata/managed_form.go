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
	MaxManagedFormCommands = 1_024
	MaxFormDataPathDepth   = 16
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

type FormCommandAction string

const (
	FormCommandCustom      FormCommandAction = "custom"
	FormCommandSave        FormCommandAction = "save"
	FormCommandSaveClose   FormCommandAction = "save-and-close"
	FormCommandClose       FormCommandAction = "close"
	FormCommandRefresh     FormCommandAction = "refresh"
	FormCommandPost        FormCommandAction = "post"
	FormCommandUndoPosting FormCommandAction = "undo-posting"
	FormCommandChoose      FormCommandAction = "choose"
)

// ManagedFormCommand describes one form command. Custom commands call a BSL
// routine from the form module; standard commands are executed by ML App.
type ManagedFormCommand struct {
	ID      uuid.UUID         `yaml:"id" json:"id"`
	Name    string            `yaml:"name" json:"name"`
	Title   LocalizedText     `yaml:"title" json:"title"`
	Action  FormCommandAction `yaml:"action" json:"action"`
	Handler string            `yaml:"handler,omitempty" json:"handler,omitempty"`
}

// ManagedForm is the versioned source model edited by the visual form designer.
type ManagedForm struct {
	Format   int                  `yaml:"format" json:"format"`
	ID       uuid.UUID            `yaml:"id" json:"id"`
	Name     string               `yaml:"name" json:"name"`
	Title    LocalizedText        `yaml:"title" json:"title"`
	Kind     FormKind             `yaml:"kind" json:"kind"`
	Module   *uuid.UUID           `yaml:"module,omitempty" json:"module,omitempty"`
	Commands []ManagedFormCommand `yaml:"commands,omitempty" json:"commands"`
	Items    []ManagedFormElement `yaml:"items,omitempty" json:"items"`
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
	DataPath    string               `yaml:"data_path,omitempty" json:"dataPath,omitempty"`
	Command     *uuid.UUID           `yaml:"command,omitempty" json:"command,omitempty"`
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
	if len(value.Commands) > MaxManagedFormCommands {
		issues = append(issues, fmt.Sprintf("commands must not contain more than %d items", MaxManagedFormCommands))
	}
	commandNames, commandIDs := map[string]bool{}, map[uuid.UUID]bool{}
	for index, command := range value.Commands {
		prefix := fmt.Sprintf("commands[%d]", index)
		if command.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		} else if commandIDs[command.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		commandIDs[command.ID] = true
		if !validIdentifier(command.Name) || utf8.RuneCountInString(command.Name) > 128 {
			issues = append(issues, prefix+".name must be a valid identifier of at most 128 characters")
		}
		folded := strings.ToLower(command.Name)
		if commandNames[folded] {
			issues = append(issues, prefix+".name must be unique within commands")
		}
		commandNames[folded] = true
		issues = append(issues, validateTitle(prefix+".title", command.Title, manifest)...)
		switch command.Action {
		case FormCommandCustom:
			if !validIdentifier(command.Handler) || utf8.RuneCountInString(command.Handler) > 128 {
				issues = append(issues, prefix+".handler must be a valid BSL routine name for a custom command")
			}
		case FormCommandSave, FormCommandSaveClose, FormCommandClose, FormCommandRefresh,
			FormCommandPost, FormCommandUndoPosting, FormCommandChoose:
			if command.Handler != "" {
				issues = append(issues, prefix+".handler is allowed only for a custom command")
			}
		default:
			issues = append(issues, prefix+".action is unsupported")
		}
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
	for id := range commandIDs {
		ids[id] = true
	}
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
		if item.DataPath != "" {
			if item.Kind != FormElementField && item.Kind != FormElementTable {
				issues = append(issues, current.path+".data_path is allowed only for fields and tables")
			}
			issues = append(issues, validateFormDataPath(current.path+".data_path", item.DataPath)...)
		}
		if item.Command != nil {
			if item.Kind != FormElementButton {
				issues = append(issues, current.path+".command is allowed only for buttons")
			}
			if item.Command.IsZero() {
				issues = append(issues, current.path+".command must be a non-zero UUID")
			} else if !commandIDs[*item.Command] {
				issues = append(issues, current.path+".command references an unknown form command")
			}
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
		case FormElementTable:
			if item.Orientation != "" {
				issues = append(issues, current.path+".orientation is allowed only for groups")
			}
			if len(item.Children) > MaxGroupChildren {
				issues = append(issues, fmt.Sprintf("%s.children must not contain more than %d elements", current.path, MaxGroupChildren))
				continue
			}
			for index := len(item.Children) - 1; index >= 0; index-- {
				if item.Children[index].Kind != FormElementField {
					issues = append(issues, fmt.Sprintf("%s.children[%d] must be a field", current.path, index))
				}
				stack = append(stack, pending{element: item.Children[index], path: fmt.Sprintf("%s.children[%d]", current.path, index), depth: current.depth + 1})
			}
		case FormElementField, FormElementLabel, FormElementButton:
			if len(item.Children) != 0 {
				issues = append(issues, current.path+".children are allowed only for groups and tables")
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

func validateFormDataPath(path, value string) []string {
	if strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > 512 {
		return []string{path + " must be a canonical data path of at most 512 characters"}
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 || len(parts) > MaxFormDataPathDepth {
		return []string{fmt.Sprintf("%s must contain 1..%d identifier segments", path, MaxFormDataPathDepth)}
	}
	for _, part := range parts {
		if !validIdentifier(part) || utf8.RuneCountInString(part) > 128 {
			return []string{path + " must contain only valid identifier segments separated by dots"}
		}
	}
	return nil
}
