package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FormOrientation is how a group lays out what it holds: a usual group and a
// page as a form does (help, ChildFormItemsGroup), a group of columns its
// columns, side by side, one under another or in one cell (help,
// ColumnsGroup).
type FormOrientation string

const (
	FormVertical             FormOrientation = "vertical"
	FormHorizontal           FormOrientation = "horizontal"
	FormAlwaysHorizontal     FormOrientation = "always-horizontal"
	FormHorizontalIfPossible FormOrientation = "horizontal-if-possible"
	FormInCell               FormOrientation = "in-cell"
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
	// Events are the events of the form handled by procedures of its
	// module, by event.
	Events map[FormEvent]string `yaml:"events,omitempty" json:"events,omitempty"`
	// AutoCommandBar is the command bar of the form - see FormAttachedMenu.
	AutoCommandBar *FormAttachedMenu    `yaml:"auto_command_bar,omitempty" json:"autoCommandBar,omitempty"`
	Items          []ManagedFormElement `yaml:"items,omitempty" json:"items"`
	Attributes     []FormAttribute      `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	// Parameters are the values the form is opened with.
	Parameters []FormParameter `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// ManagedFormElement is one stable node in a managed form tree. Hidden and
// disabled use inverse flags so omitted YAML retains the useful true defaults.
type ManagedFormElement struct {
	ID    uuid.UUID       `yaml:"id" json:"id"`
	Name  string          `yaml:"name" json:"name"`
	Kind  FormElementKind `yaml:"kind" json:"kind"`
	Title LocalizedText   `yaml:"title,omitempty" json:"title"`
	// TitleFormatted reads the title of a decoration as a formatted string,
	// with its marks of bold, colour and links; the prototype writes it on
	// 1166 decorations, 32 of them with no title.
	TitleFormatted bool   `yaml:"title_formatted,omitempty" json:"titleFormatted,omitempty"`
	Hidden         bool   `yaml:"hidden,omitempty" json:"hidden"`
	Disabled       bool   `yaml:"disabled,omitempty" json:"disabled"`
	ReadOnly       bool   `yaml:"read_only,omitempty" json:"readOnly"`
	DataPath       string `yaml:"data_path,omitempty" json:"dataPath,omitempty"`
	// Command is the command a button runs, written as the prototype writes
	// it (form_commands.go), and CommandParameter what it passes to it.
	Command          string                `yaml:"command,omitempty" json:"command,omitempty"`
	CommandParameter *FormCommandParameter `yaml:"command_parameter,omitempty" json:"commandParameter,omitempty"`
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
	// DisplayImportance is how important the element is when the form is
	// fitted to a narrow screen; the prototype writes it on 1582 elements of
	// every kind.
	DisplayImportance FormDisplayImportance `yaml:"display_importance,omitempty" json:"displayImportance,omitempty"`
	// FieldBehavior is what a field has besides what every element has.
	FieldBehavior `yaml:",inline"`
	// FieldLayout is the size of a field and where it stands.
	FieldLayout `yaml:",inline"`
	// FieldLook is how a field and its title are drawn.
	FieldLook `yaml:",inline"`
	// FieldColumn is what a field has as a column of a table.
	FieldColumn `yaml:",inline"`
	// FieldButtons are the buttons of an input field.
	FieldButtons `yaml:",inline"`
	// FieldTextInput is how text is typed into an input field.
	FieldTextInput `yaml:",inline"`
	// FieldFormat is how a field shows a value and bounds a number.
	FieldFormat `yaml:",inline"`
	// InputFieldChoice is how a value of an input field is picked.
	InputFieldChoice `yaml:",inline"`
	// ChoiceList is the list a value of an input or a radio button field is
	// picked from.
	ChoiceList []FormChoiceListItem `yaml:"choice_list,omitempty" json:"choiceList,omitempty"`
	// InputFieldChoiceParameters narrow what an input field offers to pick.
	InputFieldChoiceParameters `yaml:",inline"`
	// FieldValueView is how a label field, a check box and a radio button
	// field show their value.
	FieldValueView `yaml:",inline"`
	// FieldPicture is what a picture field has.
	FieldPicture `yaml:",inline"`
	// FieldDocument is what the fields of documents have.
	FieldDocument `yaml:",inline"`
	// FieldOther is what a calendar, a progress bar and a track bar have.
	FieldOther `yaml:",inline"`
	// GroupProperties is what the groups have of their own.
	GroupProperties `yaml:",inline"`
	// ButtonProperties is what a button has of its own.
	ButtonProperties `yaml:",inline"`
	// TableProperties is what a table has of its own.
	TableProperties `yaml:",inline"`
	// TableAdditions are the search string, view status and search control
	// a table holds.
	TableAdditions `yaml:",inline"`
	// ExtendedTooltip is the extended tooltip of the element: a label of the
	// form (help, ExtendedTooltip - FormDecoration) the prototype writes
	// inside nearly every element - 294504 in the exports, most with nothing
	// but an identifier and a name. Code reaches it by its name, which is
	// kept as written: the configurator does not rename it with its element.
	ExtendedTooltip *ManagedFormElement `yaml:"extended_tooltip,omitempty" json:"extendedTooltip,omitempty"`
	// OnMainServerUnavailable is what a decoration does while the main
	// server is out of reach (help, OnMainServerUnavalableBehavior of a
	// decoration); the prototype writes it once, on an extended tooltip.
	OnMainServerUnavailable FormServerUnavailableBehavior `yaml:"on_main_server_unavailable,omitempty" json:"onMainServerUnavailable,omitempty"`
	// Events are the events of the element handled by procedures of the
	// module of the form, by event.
	Events map[FormEvent]string `yaml:"events,omitempty" json:"events,omitempty"`
	// AdditionSource is the table an addition standing apart from it is of,
	// named by its element. An addition a table holds names none: it is of
	// that table. One standing apart may name none too - the prototype saves
	// a search string and a search control in the command bar of a form with
	// no source (lombard1, one form) - and is then of no table and noted
	// (NoteAdditionOfNoTable). The prototype writes the kind of the addition beside it,
	// always the kind of the addition itself, which is not kept twice.
	AdditionSource string `yaml:"addition_source,omitempty" json:"additionSource,omitempty"`
	// ButtonType is what a button is; only a button has one.
	ButtonType FormButtonType `yaml:"button_type,omitempty" json:"buttonType,omitempty"`
	// Orientation lays out what a usual group, a page or a group of columns
	// holds; empty is vertical. The prototype writes always horizontal 3165
	// times, horizontal if possible 481, and in one cell 2101 on groups of
	// columns.
	Orientation FormOrientation `yaml:"orientation,omitempty" json:"orientation,omitempty"`
	// ContextMenu is the context menu of a field, a decoration, a table or an
	// addition of a table, and AutoCommandBar the command bar of a table.
	ContextMenu    *FormAttachedMenu    `yaml:"context_menu,omitempty" json:"contextMenu,omitempty"`
	AutoCommandBar *FormAttachedMenu    `yaml:"auto_command_bar,omitempty" json:"autoCommandBar,omitempty"`
	Children       []ManagedFormElement `yaml:"children,omitempty" json:"children"`
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
	issues = append(issues, validateFormEvents("form", value.Events, formEvents, "a form")...)
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
		// held is an addition a table holds, and tooltip the extended tooltip
		// of an element.
		held, tooltip bool
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
	// shared holds the names every holder of which may share it: the
	// prototype names an addition of a table and a context menu itself
	// ("Addition", "ContextMenu") and saves the name repeated (3 forms of the
	// exports); the load notes it. Any other name is the form's own and is
	// unique.
	shared := map[string]bool{}
	claim := func(path, name string, shareable bool) {
		folded := foldedName(name)
		if names[folded] && !(shareable && shared[folded]) {
			issues = append(issues, path+".name must be unique within the form")
		}
		shared[folded] = shareable && (shared[folded] || !names[folded])
		names[folded] = true
	}
	// A group names the table whose current row it shows, and the field or
	// table it takes commands from; each is found once the whole form is
	// walked, as it may stand after the group.
	sources, associated := map[string]FormElementKind{}, []pending(nil)
	// A button runs a standard command of an element, which may stand after
	// it too, and an addition standing apart names a table that may.
	var commanded, sourced []pending
	// A table of a dynamic list shows an attribute of the form and names the
	// group its user settings go in, which may stand after it.
	var listed []pending
	for id := range commandIDs {
		ids[id] = true
	}
	// A context menu and a command bar are named and numbered among the
	// elements, and hold buttons that are elements of the form too.
	attach := func(path, name string, menu *FormAttachedMenu, commandBar, ofForm bool) {
		issues = append(issues, validateAttachedMenu(path+"."+name, *menu, commandBar, ofForm)...)
		if !menu.ID.IsZero() && ids[menu.ID] {
			issues = append(issues, path+"."+name+".id must be unique")
		}
		ids[menu.ID] = true
		if menu.Name != "" {
			claim(path+"."+name, menu.Name, !commandBar)
		}
		for index := len(menu.Children) - 1; index >= 0; index-- {
			child, place := menu.Children[index], attachedMenuPlace(path, name, index)
			if _, known := formElementClasses[child.Kind]; known && !menuHolds(commandBar, child.Kind) {
				issues = append(issues, place+": "+strings.ReplaceAll(name, "_", " ")+" cannot hold "+string(child.Kind))
			}
			stack = append(stack, pending{element: child, path: place})
		}
	}
	if value.AutoCommandBar != nil {
		attach("form", "auto_command_bar", value.AutoCommandBar, true, true)
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
		claim(current.path, item.Name, formElementClasses[item.Kind] == formAdditionClass || current.tooltip)
		issues = append(issues, validateTitle(current.path+".title", item.Title, configuration)...)
		if item.TitleFormatted && formElementClasses[item.Kind] != formDecorationClass {
			issues = append(issues, current.path+".title_formatted is allowed only for decorations")
		}
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
		issues = append(issues, validateFormField(current.path, item.FieldBehavior, class, item.Kind, configuration)...)
		issues = append(issues, validateFieldLayout(current.path, item.FieldLayout, class, item.Kind)...)
		issues = append(issues, validateFieldLook(current.path, item.FieldLook, class, item.Kind)...)
		issues = append(issues, validateFieldColumn(current.path, item.FieldColumn, class, item.Kind, configuration)...)
		issues = append(issues, validateFieldButtons(current.path, item.FieldButtons, item.Kind)...)
		issues = append(issues, validateFieldTextInput(current.path, item.FieldTextInput, item.Kind, configuration)...)
		issues = append(issues, validateFieldFormat(current.path, item.FieldFormat, item.Kind, configuration)...)
		issues = append(issues, validateInputFieldChoice(current.path, item.InputFieldChoice, item.Kind)...)
		issues = append(issues, validateChoiceList(current.path, item.ChoiceList, item.Kind, configuration)...)
		issues = append(issues, validateInputFieldChoiceParameters(current.path, item.InputFieldChoiceParameters, item.Kind)...)
		issues = append(issues, validateFieldValueView(current.path, item.FieldValueView, item.Kind)...)
		issues = append(issues, validateFieldPicture(current.path, item.FieldPicture, item.Kind, configuration)...)
		issues = append(issues, validateFieldDocument(current.path, item.FieldDocument, item.Kind)...)
		issues = append(issues, validateFieldOther(current.path, item.FieldOther, item.Kind)...)
		issues = append(issues, validateGroupProperties(current.path, item.GroupProperties, item.Kind, configuration)...)
		issues = append(issues, validateButtonProperties(current.path, item.ButtonProperties, item.Kind)...)
		issues = append(issues, validateTableProperties(current.path, item.TableProperties, item.Kind)...)
		issues = append(issues, validateFormEvents(current.path, item.Events, formElementEvents[item.Kind], "a "+string(item.Kind))...)
		if _, named := item.commandSourceItem(); item.AssociatedTable != "" || named {
			associated = append(associated, pending{element: item, path: current.path})
		}
		if class == formTableClass || class == formFieldClass {
			sources[folded] = item.Kind
		}
		if item.DynamicList != nil && class == formTableClass {
			listed = append(listed, pending{element: item, path: current.path})
		}
		issues = append(issues, validateOwnPictureFiles(current.path, item)...)
		// The help gives every group whether it is read only (FormGroup), and
		// the prototype writes it on usual groups, pages and groups of columns.
		if item.ReadOnly && class != formFieldClass && class != formTableClass && !slices.Contains(formGroupKinds, item.Kind) {
			issues = append(issues, current.path+".read_only is allowed only for fields, tables and groups")
		}
		if item.Command != "" {
			if class != formButtonClass {
				issues = append(issues, current.path+".command is allowed only for buttons")
			}
			switch command, ok := parseButtonCommand(item.Command); {
			case !ok:
				issues = append(issues, current.path+".command must be Form.Command.<name>, Form.StandardCommand.<name>, "+
					"Form.Item.<element>.StandardCommand.<name>, CommonCommand.<name>, <kind>.<object>.Command.<name>, "+
					"<kind>.<object>.StandardCommand.<name> or the code of an element of a form")
			case command.kind == formCommand && !commandNames[strings.ToLower(command.name)]:
				issues = append(issues, current.path+".command names no command of the form")
			case command.kind == elementStandardCommand:
				commanded = append(commanded, pending{element: item, path: current.path})
			}
		}
		if item.CommandParameter != nil && item.Command == "" {
			issues = append(issues, current.path+".command_parameter is allowed only beside a command")
		}
		issues = append(issues, validateFormCommandParameter(current.path+".command_parameter", item.CommandParameter)...)
		issues = append(issues, validateElementCommon(current.path, item, class, configuration)...)
		if item.ButtonType != "" && class != formButtonClass {
			issues = append(issues, current.path+".button_type is allowed only for buttons")
		}
		issues = append(issues, oneOf(current.path+".button_type", item.ButtonType, FormButtonUsual, FormButtonHyperlink, FormButtonCommandBarButton, FormButtonCommandBarHyperlink)...)
		if item.Orientation != "" {
			if class != formAreaClass && class != formColumnsClass {
				issues = append(issues, current.path+".orientation is allowed only for usual groups, pages and groups of columns")
			}
			if class == formColumnsClass {
				issues = append(issues, oneOf(current.path+".orientation", item.Orientation, FormVertical, FormHorizontal, FormInCell)...)
			} else {
				issues = append(issues, oneOf(current.path+".orientation", item.Orientation, FormVertical, FormHorizontal, FormAlwaysHorizontal, FormHorizontalIfPossible)...)
			}
		}
		for index := len(item.Children) - 1; index >= 0; index-- {
			child := item.Children[index]
			place := fmt.Sprintf("%s.children[%d]", current.path, index)
			if _, known := formElementClasses[child.Kind]; known && !item.Kind.Holds(child.Kind) {
				issues = append(issues, place+": "+string(item.Kind)+" cannot hold "+string(child.Kind))
			}
			stack = append(stack, pending{element: child, path: place})
		}
		if item.ContextMenu != nil {
			if !contextMenuHolder(class) {
				issues = append(issues, current.path+".context_menu is allowed only for fields, decorations, tables and additions of a table")
			}
			attach(current.path, "context_menu", item.ContextMenu, false, false)
		}
		if item.AutoCommandBar != nil {
			if class != formTableClass {
				issues = append(issues, current.path+".auto_command_bar is allowed only for tables")
			}
			attach(current.path, "auto_command_bar", item.AutoCommandBar, true, false)
		}
		for _, held := range item.tableAdditions() {
			if held.addition == nil {
				continue
			}
			place := current.path + "." + held.name
			if class != formTableClass {
				issues = append(issues, place+" is allowed only for tables")
			}
			if held.addition.Kind != held.kind {
				issues = append(issues, place+".kind must be "+string(held.kind))
			}
			stack = append(stack, pending{element: *held.addition, path: place, held: true})
		}
		if tooltip := item.ExtendedTooltip; tooltip != nil {
			if current.tooltip {
				issues = append(issues, current.path+".extended_tooltip: an extended tooltip has none of its own")
			}
			if tooltip.Kind != FormElementLabelDecoration {
				issues = append(issues, current.path+".extended_tooltip.kind must be label-decoration")
			}
			stack = append(stack, pending{element: *tooltip, path: current.path + ".extended_tooltip", tooltip: true})
		}
		if current.tooltip && item.ContextMenu != nil {
			issues = append(issues, current.path+".context_menu: an extended tooltip has none")
		}
		if item.OnMainServerUnavailable != "" && class != formDecorationClass {
			issues = append(issues, current.path+".on_main_server_unavailable is allowed only for decorations")
		}
		issues = append(issues, oneOf(current.path+".on_main_server_unavailable", item.OnMainServerUnavailable, FormServerUnavailableAuto,
			FormServerUnavailableDontChange, FormServerUnavailableMakeDisable)...)
		switch {
		case class != formAdditionClass && item.AdditionSource != "":
			issues = append(issues, current.path+".addition_source is allowed only for additions of a table")
		case class != formAdditionClass:
		case item.AdditionSource != "" && (!validIdentifier(item.AdditionSource) || utf8.RuneCountInString(item.AdditionSource) > maxNameLength):
			issues = append(issues, current.path+".addition_source must be the name of an element of the form")
		case item.AdditionSource != "":
			sourced = append(sourced, current)
		}
	}
	for _, group := range associated {
		if table := group.element.AssociatedTable; table != "" && sources[strings.ToLower(table)] != FormElementTable {
			issues = append(issues, group.path+".associated_table names no table of the form")
		}
		if name, named := group.element.commandSourceItem(); named && sources[strings.ToLower(name)] == "" {
			issues = append(issues, group.path+".command_source names no field or table of the form")
		}
	}
	for _, button := range commanded {
		if command, _ := parseButtonCommand(button.element.Command); !names[strings.ToLower(command.element)] {
			issues = append(issues, button.path+".command names no element of the form")
		}
	}
	// An addition standing apart names a table; one a table holds names, if
	// anything, an element of the form - in sb the extended tooltip of
	// another element (NoteHeldAdditionOfAnother).
	for _, addition := range sourced {
		source := addition.element.AdditionSource
		switch {
		case addition.held && !names[foldedName(source)]:
			issues = append(issues, addition.path+".addition_source names no element of the form")
		case !addition.held && sources[strings.ToLower(source)] != FormElementTable:
			issues = append(issues, addition.path+".addition_source names no table of the form")
		}
	}
	for _, table := range listed {
		attribute := slices.IndexFunc(value.Attributes, func(attribute FormAttribute) bool {
			single, ok := SingleType(attribute.Types)
			return ok && single.Kind == DynamicListType && foldedName(attribute.Name) == foldedName(table.element.DataPath)
		})
		if attribute < 0 {
			issues = append(issues, table.path+".dynamic_list belongs to a table showing an attribute of the form that is a dynamic list")
		}
		if group := table.element.DynamicList.UserSettingsGroup; group != "" && !formElementCode.MatchString(group) && !names[foldedName(group)] {
			issues = append(issues, table.path+".dynamic_list.user_settings_group names no element of the form")
		}
	}
	issues = append(issues, validateFormAttributes(value.Attributes, ids, configuration)...)
	issues = append(issues, validateFormParameters(value.Parameters)...)
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
