package metadata

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
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
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
	// FormWindow is the window of the form and how it behaves, kept beside
	// the rest of the form.
	FormWindow `yaml:",inline"`
	// FormLayout is how the form lays out what it holds.
	FormLayout `yaml:",inline"`
	// FormExtension is what the form has by the kind of its main attribute.
	FormExtension `yaml:",inline"`
	Commands      []ManagedFormCommand `yaml:"commands,omitempty" json:"commands"`
	Items         []ManagedFormElement `yaml:"items,omitempty" json:"items"`
	Attributes    []FormAttribute      `yaml:"attributes,omitempty" json:"attributes,omitempty"`
}

// ManagedFormElement is one stable node in a managed form tree. Hidden and
// disabled use inverse flags so omitted YAML retains the useful true defaults.
type ManagedFormElement struct {
	ID       uuid.UUID       `yaml:"id" json:"id"`
	Name     string          `yaml:"name" json:"name"`
	Kind     FormElementKind `yaml:"kind" json:"kind"`
	Title    LocalizedText   `yaml:"title,omitempty" json:"title"`
	Hidden   bool            `yaml:"hidden,omitempty" json:"hidden"`
	Disabled bool            `yaml:"disabled,omitempty" json:"disabled"`
	ReadOnly bool            `yaml:"read_only,omitempty" json:"readOnly"`
	DataPath string          `yaml:"data_path,omitempty" json:"dataPath,omitempty"`
	Command  *uuid.UUID      `yaml:"command,omitempty" json:"command,omitempty"`
	// ToolTip is the tooltip of the element; a button shows that of its
	// command. ToolTipRepresentation is how the tooltip shows.
	ToolTip               LocalizedText             `yaml:"tool_tip,omitempty" json:"toolTip,omitempty"`
	ToolTipRepresentation FormToolTipRepresentation `yaml:"tool_tip_representation,omitempty" json:"toolTipRepresentation,omitempty"`
	// UserVisible is whom the element is shown to until the user changes the
	// settings of the form: one answer for every role and the roles that
	// answer otherwise, as a right of an attribute of a form. Nil is shown to
	// everyone; the prototype writes it on 6611 elements, 6590 of them shown
	// to no one until the user turns them on. The help does not name it.
	UserVisible *FormAttributeRight `yaml:"user_visible,omitempty" json:"userVisible,omitempty"`
	// FieldBehavior is what a field has besides what every element has.
	FieldBehavior `yaml:",inline"`
	// ButtonType is what a button is; only a button has one.
	ButtonType FormButtonType `yaml:"button_type,omitempty" json:"buttonType,omitempty"`
	// Orientation lays out what a usual group, a page or a group of columns
	// holds; empty is vertical.
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
	issues = append(issues, validateFormWindow(value.FormWindow)...)
	issues = append(issues, validateFormLayout(value.FormLayout)...)
	issues = append(issues, validateFormExtension(value.FormExtension, value.Attributes)...)
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
		item := value.Items[index]
		// The form lays out what it holds as a usual group does.
		if _, known := formElementClasses[item.Kind]; known && !formClassHolds(formAreaClass, item.Kind) {
			issues = append(issues, fmt.Sprintf("items[%d]: a form cannot hold %s", index, item.Kind))
		}
		stack = append(stack, pending{element: item, path: fmt.Sprintf("items[%d]", index)})
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
		class, known := formElementClasses[item.Kind]
		if !known {
			issues = append(issues, current.path+".kind "+string(item.Kind)+" is not a kind of element of a form")
			continue
		}
		// A button has a data path too (1112 in the exports), which the help
		// does not name: by what it holds, the source of the parameter of
		// its command. It is carried as written.
		if item.DataPath != "" {
			if class != formFieldClass && class != formTableClass && class != formButtonClass {
				issues = append(issues, current.path+".data_path is allowed only for fields, tables and buttons")
			}
			issues = append(issues, validateElementDataPath(current.path+".data_path", item.DataPath)...)
		}
		issues = append(issues, validateFormField(current.path, item.FieldBehavior, class, configuration)...)
		if item.ReadOnly && class != formFieldClass && class != formTableClass {
			issues = append(issues, current.path+".read_only is allowed only for fields and tables")
		}
		if item.Command != nil {
			if class != formButtonClass {
				issues = append(issues, current.path+".command is allowed only for buttons")
			}
			if item.Command.IsZero() {
				issues = append(issues, current.path+".command must be a non-zero UUID")
			} else if !commandIDs[*item.Command] {
				issues = append(issues, current.path+".command references an unknown form command")
			}
		}
		issues = append(issues, validateElementCommon(current.path, item, class, configuration)...)
		if item.ButtonType != "" && class != formButtonClass {
			issues = append(issues, current.path+".button_type is allowed only for buttons")
		}
		issues = append(issues, oneOf(current.path+".button_type", item.ButtonType, FormButtonUsual, FormButtonHyperlink, FormButtonCommandBarButton, FormButtonCommandBarHyperlink)...)
		if item.Orientation != "" {
			if class != formAreaClass && class != formColumnsClass {
				issues = append(issues, current.path+".orientation is allowed only for usual groups, pages and groups of columns")
			}
			issues = append(issues, oneOf(current.path+".orientation", item.Orientation, FormVertical, FormHorizontal)...)
		}
		for index := len(item.Children) - 1; index >= 0; index-- {
			child := item.Children[index]
			place := fmt.Sprintf("%s.children[%d]", current.path, index)
			if _, known := formElementClasses[child.Kind]; known && !item.Kind.Holds(child.Kind) {
				issues = append(issues, place+": "+string(item.Kind)+" cannot hold "+string(child.Kind))
			}
			stack = append(stack, pending{element: child, path: place})
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
