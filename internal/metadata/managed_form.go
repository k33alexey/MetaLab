package metadata

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
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

// FormType is how a form is built and shown. The platform builds managed
// forms; an ordinary form is what the prototype's older configurations carry,
// and it is kept so that importing one does not silently turn it into
// something else.
type FormType string

const (
	ManagedFormType  FormType = "managed"
	OrdinaryFormType FormType = "ordinary"
)

// FormPurpose is one kind of application a form is meant for. A form may be
// meant for several, and the set is a property of the form: the reference
// configuration keeps forms offered on the desktop but not on a phone. It is
// the purpose of the configuration root - one type in the prototype, one list
// here.
type FormPurpose = project.UsePurpose

const (
	PlatformApplicationPurpose       = project.PlatformApplicationPurpose
	MobilePlatformApplicationPurpose = project.MobilePlatformApplicationPurpose
)

// ManagedForm is the versioned source model edited by the visual form designer.
//
// Its module is not named here. A form keeps a folder, and the module lies in
// that folder under the name of its role: the file is the declaration, and
// there is no second place to disagree with it.
type ManagedForm struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	Kind   FormKind      `yaml:"kind" json:"kind"`
	// Comment, Explanation and ExtendedPresentation are the three ways a form
	// is described: to the developer reading the tree, to the user hovering
	// over it, and to the user reading a list of forms.
	Comment              string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	Explanation          LocalizedText `yaml:"explanation,omitempty" json:"explanation,omitempty"`
	ExtendedPresentation LocalizedText `yaml:"extended_presentation,omitempty" json:"extendedPresentation,omitempty"`
	// Type is managed unless the form says otherwise.
	Type FormType `yaml:"type,omitempty" json:"type,omitempty"`
	// Purposes says which kinds of application the form is meant for. Empty
	// means the form makes no claim and is offered everywhere.
	Purposes []FormPurpose `yaml:"purposes,omitempty" json:"purposes,omitempty"`
	// UseStandardCommands decides whether the platform offers its own commands
	// on this form beside the ones it declares.
	UseStandardCommands bool `yaml:"use_standard_commands,omitempty" json:"useStandardCommands,omitempty"`
	// IncludeHelpInContents puts this form's help into the table of contents
	// of the configuration's help.
	IncludeHelpInContents bool                 `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
	Commands              []ManagedFormCommand `yaml:"commands,omitempty" json:"commands"`
	Items                 []ManagedFormElement `yaml:"items,omitempty" json:"items"`
	Attributes            []FormAttribute      `yaml:"attributes,omitempty" json:"attributes,omitempty"`
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
func DecodeManagedForm(source string, reader io.Reader, configuration project.Project) (ManagedForm, error) {
	var value ManagedForm
	if err := decodeStrict(source, reader, &value); err != nil {
		return ManagedForm{}, err
	}
	if err := ValidateManagedForm(source, value, configuration); err != nil {
		return ManagedForm{}, err
	}
	return value, nil
}

// ValidateManagedForm validates a form received from either YAML or Studio JSON.
func ValidateManagedForm(source string, value ManagedForm, configuration project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	switch value.Kind {
	case ObjectForm, ListForm, ChoiceForm, CommonForm:
	default:
		issues = append(issues, "kind must be object, list, choice or common")
	}
	switch value.Type {
	case "", ManagedFormType, OrdinaryFormType:
	default:
		issues = append(issues, "type must be managed or ordinary")
	}
	seenPurposes := map[FormPurpose]bool{}
	for index, purpose := range value.Purposes {
		switch purpose {
		case PlatformApplicationPurpose, MobilePlatformApplicationPurpose:
		default:
			issues = append(issues, fmt.Sprintf("purposes[%d] is not a kind of application", index))
			continue
		}
		if seenPurposes[purpose] {
			issues = append(issues, fmt.Sprintf("purposes[%d] is already among the purposes", index))
		}
		seenPurposes[purpose] = true
	}
	// Both are optional: a form that says nothing extra about itself is an
	// ordinary form, and an empty text passes the check as it is.
	for name, text := range map[string]LocalizedText{
		"explanation": value.Explanation, "extended_presentation": value.ExtendedPresentation,
	} {
		issues = append(issues, validateTitle(name, text, configuration)...)
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
		if !validIdentifier(command.Name) || utf8.RuneCountInString(command.Name) > maxNameLength {
			issues = append(issues, prefix+".name must be a valid identifier of at most 255 characters")
		}
		folded := strings.ToLower(command.Name)
		if commandNames[folded] {
			issues = append(issues, prefix+".name must be unique within commands")
		}
		commandNames[folded] = true
		issues = append(issues, validateTitle(prefix+".title", command.Title, configuration)...)
		switch command.Action {
		case FormCommandCustom:
			if !validIdentifier(command.Handler) || utf8.RuneCountInString(command.Handler) > maxNameLength {
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
	}
	stack := make([]pending, 0, len(value.Items))
	for index := len(value.Items) - 1; index >= 0; index-- {
		stack = append(stack, pending{element: value.Items[index], path: fmt.Sprintf("items[%d]", index)})
	}
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for id := range commandIDs {
		ids[id] = true
	}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		item := current.element
		if item.ID.IsZero() {
			issues = append(issues, current.path+".id must be a non-zero UUID")
		} else if ids[item.ID] {
			issues = append(issues, current.path+".id must be unique")
		}
		ids[item.ID] = true
		if !validIdentifier(item.Name) || utf8.RuneCountInString(item.Name) > maxNameLength {
			issues = append(issues, current.path+".name must be a valid identifier of at most 255 characters")
		}
		folded := strings.ToLower(item.Name)
		if names[folded] {
			issues = append(issues, current.path+".name must be unique within the form")
		}
		names[folded] = true
		issues = append(issues, validateTitle(current.path+".title", item.Title, configuration)...)
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
			for index := len(item.Children) - 1; index >= 0; index-- {
				stack = append(stack, pending{element: item.Children[index], path: fmt.Sprintf("%s.children[%d]", current.path, index)})
			}
		case FormElementTable:
			if item.Orientation != "" {
				issues = append(issues, current.path+".orientation is allowed only for groups")
			}
			for index := len(item.Children) - 1; index >= 0; index-- {
				if item.Children[index].Kind != FormElementField {
					issues = append(issues, fmt.Sprintf("%s.children[%d] must be a field", current.path, index))
				}
				stack = append(stack, pending{element: item.Children[index], path: fmt.Sprintf("%s.children[%d]", current.path, index)})
			}
		case FormElementField, FormElementLabel, FormElementButton:
			if len(item.Children) != 0 {
				issues = append(issues, current.path+".children are allowed only for groups and tables")
			}
			if item.Orientation != "" {
				issues = append(issues, current.path+".orientation is allowed only for groups")
			}
			if item.ReadOnly && item.Kind != FormElementField {
				issues = append(issues, current.path+".read_only is allowed only for fields and tables")
			}
		default:
			issues = append(issues, current.path+".kind must be group, field, label, table or button")
		}
	}
	issues = append(issues, validateFormAttributes(value.Attributes, ids, configuration)...)
	return issuesError(source, value.Format, issues)
}

// validateFormDataPath checks a data path segment by segment. Its length and
// its depth are not limited: the prototype names no ceiling on either, and a
// path through a table part and the attributes of what it references runs as
// deep as the configuration does.
func validateFormDataPath(path, value string) []string {
	if strings.TrimSpace(value) != value {
		return []string{path + " must be a canonical data path"}
	}
	for _, part := range strings.Split(value, ".") {
		if !validIdentifier(part) || utf8.RuneCountInString(part) > maxNameLength {
			return []string{path + " must contain only valid identifier segments separated by dots"}
		}
	}
	return nil
}
