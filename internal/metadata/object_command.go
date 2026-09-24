package metadata

import (
	"fmt"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// maxCommandsPerObject is where a list of commands stops being a list.
const maxCommandsPerObject = 128

// CommandParameterUse says whether a command takes one object or several.
type CommandParameterUse string

const (
	CommandParameterSingle   CommandParameterUse = "single"
	CommandParameterMultiple CommandParameterUse = "multiple"
)

// CommandRepresentation is how a command shows itself where it is placed.
type CommandRepresentation string

const (
	CommandAuto           CommandRepresentation = "auto"
	CommandText           CommandRepresentation = "text"
	CommandPicture        CommandRepresentation = "picture"
	CommandPictureAndText CommandRepresentation = "picture-and-text"
)

// ServerUnavailableBehavior is what a command does when the main server cannot
// be reached.
type ServerUnavailableBehavior string

const (
	ServerUnavailableAuto         ServerUnavailableBehavior = "auto"
	ServerUnavailableAvailable    ServerUnavailableBehavior = "available"
	ServerUnavailableNotAvailable ServerUnavailableBehavior = "not-available"
)

// standardCommandGroups are the places the platform itself offers. A command
// either goes into one of them or into a command group the configuration
// declares - one property, two kinds of answer, because that is how a command
// is placed: by naming the place.
var standardCommandGroups = map[string]bool{
	"navigation-panel-important": true, "navigation-panel-ordinary": true, "navigation-panel-see-also": true,
	"form-navigation-panel-important": true, "form-navigation-panel-ordinary": true,
	"form-navigation-panel-see-also": true, "form-navigation-panel-go-to": true,
	"form-command-bar-important": true, "form-command-bar-ordinary": true, "form-command-bar-see-also": true,
	"form-command-bar-create-based-on": true,
	"actions-panel-create":             true, "actions-panel-reports": true, "actions-panel-tools": true,
	"actions-panel-see-also": true,
}

// PictureReference names the picture shown beside a command. It is either one
// the platform ships or one the configuration keeps among its common pictures,
// never both; a command that names neither is shown by its text alone.
//
// The common picture is held by identifier rather than by name, like every
// other reference in the model, so that renaming the picture does not orphan
// the commands that use it.
type PictureReference struct {
	Standard string     `yaml:"standard,omitempty" json:"standard,omitempty"`
	Common   *uuid.UUID `yaml:"common,omitempty" json:"common,omitempty"`
}

// ObjectCommand is an action offered beside an object. It belongs to the object
// and is shown where its group says.
//
// The parameter is the point of it: a command that takes a reference is offered
// where such a reference is at hand, and the platform passes the object the
// user is looking at. A command without a parameter is offered everywhere the
// object is.
type ObjectCommand struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	Tooltip LocalizedText `yaml:"tooltip,omitempty" json:"tooltip,omitempty"`
	// Group is either a standard group of the platform or the identifier of a
	// command group the configuration declares.
	Group        string              `yaml:"group,omitempty" json:"group,omitempty"`
	GroupRef     *uuid.UUID          `yaml:"group_ref,omitempty" json:"groupRef,omitempty"`
	Parameter    []Type              `yaml:"parameter,omitempty" json:"parameter,omitempty"`
	ParameterUse CommandParameterUse `yaml:"parameter_use,omitempty" json:"parameterUse,omitempty"`
	// ModifiesData decides whether the command may be offered where data must
	// not change, so it is not a hint but a permission.
	ModifiesData        bool                      `yaml:"modifies_data,omitempty" json:"modifiesData,omitempty"`
	Picture             *PictureReference         `yaml:"picture,omitempty" json:"picture,omitempty"`
	Representation      CommandRepresentation     `yaml:"representation,omitempty" json:"representation,omitempty"`
	Shortcut            string                    `yaml:"shortcut,omitempty" json:"shortcut,omitempty"`
	OnServerUnavailable ServerUnavailableBehavior `yaml:"on_server_unavailable,omitempty" json:"onServerUnavailable,omitempty"`
	// Module holds the procedure that runs the command, and every command has
	// one: a command without a body is a place in the interface that answers a
	// click with nothing, which is worse than not offering it at all.
	Module uuid.UUID `yaml:"module" json:"module"`
}

// validateObjectCommands checks the commands of one object. Names and
// identifiers are kept apart from the object's own, because a command is not an
// attribute and the two never collide in practice.
//
// owned are the modules the object itself already has - its object, manager or
// record-set module. A command module is a file in the same folder, named by
// the same kind of identifier, so a command that reuses one of them would quietly
// share a body with the object instead of having one of its own.
func validateObjectCommands(commands []ObjectCommand, self uuid.UUID, manifest project.Project, owned ...*uuid.UUID) []string {
	if len(commands) > maxCommandsPerObject {
		return []string{fmt.Sprintf("commands must not contain more than %d items", maxCommandsPerObject)}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	modules := map[uuid.UUID]bool{}
	for _, module := range owned {
		if module != nil {
			modules[*module] = true
		}
	}
	for index, command := range commands {
		prefix := fmt.Sprintf("commands[%d]", index)
		if command.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		if ids[command.ID] {
			issues = append(issues, prefix+".id must be unique")
		}
		ids[command.ID] = true
		if !validIdentifier(command.Name) {
			issues = append(issues, prefix+".name must be a valid identifier")
		}
		folded := strings.ToLower(command.Name)
		if names[folded] {
			issues = append(issues, prefix+".name must be unique")
		}
		names[folded] = true
		issues = append(issues, validateTitle(prefix+".title", command.Title, manifest)...)
		if len(command.Tooltip) > 0 {
			issues = append(issues, validateTitle(prefix+".tooltip", command.Tooltip, manifest)...)
		}
		// A command is placed in one place, not in two.
		if command.Group != "" && command.GroupRef != nil {
			issues = append(issues, prefix+" names both a standard group and a group of the configuration")
		}
		if command.Group != "" && !standardCommandGroups[command.Group] {
			issues = append(issues, prefix+".group is not a standard group of the platform")
		}
		if command.GroupRef != nil && command.GroupRef.IsZero() {
			issues = append(issues, prefix+".group_ref must be a non-zero UUID")
		}
		if len(command.Parameter) > 0 {
			issues = append(issues, validateTypes(prefix+".parameter", command.Parameter, self)...)
		}
		switch command.ParameterUse {
		case "", CommandParameterSingle, CommandParameterMultiple:
		default:
			issues = append(issues, prefix+".parameter_use must be single or multiple")
		}
		// Saying how many objects a command takes, when it takes none, is an
		// answer to a question nobody asked.
		if command.ParameterUse != "" && len(command.Parameter) == 0 {
			issues = append(issues, prefix+".parameter_use needs a parameter type")
		}
		issues = append(issues, validatePictureReference(prefix+".picture", command.Picture)...)
		switch command.Representation {
		case "", CommandAuto, CommandText, CommandPicture, CommandPictureAndText:
		default:
			issues = append(issues, prefix+".representation must be auto, text, picture or picture-and-text")
		}
		// A command drawn as a picture needs one, and asking for a picture the
		// command does not have leaves an empty place in the interface.
		if command.Representation == CommandPicture && command.Picture == nil {
			issues = append(issues, prefix+".representation picture needs a picture")
		}
		switch command.OnServerUnavailable {
		case "", ServerUnavailableAuto, ServerUnavailableAvailable, ServerUnavailableNotAvailable:
		default:
			issues = append(issues, prefix+".on_server_unavailable must be auto, available or not-available")
		}
		switch {
		case command.Module.IsZero():
			issues = append(issues, prefix+".module is required: a command runs a procedure of its own")
		case modules[command.Module]:
			issues = append(issues, prefix+".module is already a module of this object")
		default:
			modules[command.Module] = true
		}
	}
	return issues
}

// validatePictureReference checks that a picture names one source, not two and
// not none: an empty reference is written as no reference at all.
func validatePictureReference(path string, picture *PictureReference) []string {
	if picture == nil {
		return nil
	}
	switch {
	case picture.Standard != "" && picture.Common != nil:
		return []string{path + " names both a standard picture and a common picture"}
	case picture.Standard == "" && picture.Common == nil:
		return []string{path + " must name a standard picture or a common picture"}
	case picture.Standard != "" && !validIdentifier(picture.Standard):
		return []string{path + ".standard must be a valid identifier"}
	case picture.Common != nil && picture.Common.IsZero():
		return []string{path + ".common must be a non-zero UUID"}
	}
	return nil
}

func cloneObjectCommands(commands []ObjectCommand) []ObjectCommand {
	commands = slices.Clone(commands)
	for index := range commands {
		command := &commands[index]
		command.Title = cloneTitle(command.Title)
		command.Tooltip = cloneTitle(command.Tooltip)
		command.Parameter = cloneTypes(command.Parameter)
		if command.Picture != nil {
			picture := *command.Picture
			if picture.Common != nil {
				copied := *picture.Common
				picture.Common = &copied
			}
			command.Picture = &picture
		}
		if command.GroupRef != nil {
			copied := *command.GroupRef
			command.GroupRef = &copied
		}
	}
	return commands
}
