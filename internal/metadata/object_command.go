package metadata

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

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

// ShownAs is how a command or a group is drawn on the screen, which is what it
// asks for unless it asks for a picture it does not have. Then it is drawn as
// text, its title: a button with neither is a place nobody can tell what it
// does. The help does not say what the prototype draws there; whatever it
// draws, a button that says what it does is the one we show (owner,
// 02.10.2026). Every drawing of a command or a group goes through this and not
// through the representation itself.
func (representation CommandRepresentation) ShownAs(picture *PictureReference) CommandRepresentation {
	if !picture.Names() && (representation == CommandPicture || representation == CommandPictureAndText) {
		return CommandText
	}
	return representation
}

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
//
// They are the eleven of the platform's own list (the help, the standard
// groups of commands), and the configurations being moved use exactly these.
// Four more stood here once - an ordinary group of the form's navigation
// panel and of its command bar, «see also» of the command bar and of the
// actions panel - which the prototype does not have: a command placed in one
// could never have come from it.
var standardCommandGroups = map[string]bool{
	"navigation-panel-important": true, "navigation-panel-ordinary": true, "navigation-panel-see-also": true,
	"form-navigation-panel-important": true, "form-navigation-panel-see-also": true, "form-navigation-panel-go-to": true,
	"form-command-bar-important": true, "form-command-bar-create-based-on": true,
	"actions-panel-create": true, "actions-panel-reports": true, "actions-panel-tools": true,
}

// PictureReference names the picture shown beside a command. It is one the
// platform ships, one the configuration keeps among its common pictures, or a
// file of its own - never two of them; a command that names none is shown by
// its text alone.
//
// A file of its own lies in the folder of whoever is shown with it, under the
// name the reference gives, the way the prototype keeps such a picture beside
// its owner and names it in the reference (Abs). The configurations being
// moved give it to elements of forms only - 84, 66, 77, 0 and 14 references,
// each with its file in place - and to no command; the help offers it to a
// command all the same, so it is carried wherever a reference stands: the
// folder of a command, of a common command, the folder a subsystem or a group
// of commands keeps by its identifier, the folder of an item of a route map.
//
// The common picture is held by identifier rather than by name, like every
// other reference in the model, so that renaming the picture does not orphan
// the commands that use it.
//
// LoadTransparent draws the picture with its transparent colour taken out, as
// the prototype offers wherever a picture is named (commands 99 times in erp,
// common commands and groups 14, 15 and 22, subsystems 0, 4 and 6), and
// TransparentPixel says which colour, the way a common picture says it: the
// configurations being moved name the point 0, 3, 9, 0 and 1 times, at common
// commands, command groups and subsystems.
//
// A reference may name no picture and still carry the two: the prototype
// writes a reference to «picture 0» - the picture gone, the settings left - 3,
// 2, 6, 0 and 0 times, 8 of them with settings. Such a reference is carried
// with its settings and drawn as no picture at all (see Names); a reference
// with neither a picture nor a setting is written as no reference.
type PictureReference struct {
	Standard         string        `yaml:"standard,omitempty" json:"standard,omitempty"`
	Common           *uuid.UUID    `yaml:"common,omitempty" json:"common,omitempty"`
	File             string        `yaml:"file,omitempty" json:"file,omitempty"`
	LoadTransparent  bool          `yaml:"load_transparent,omitempty" json:"loadTransparent,omitempty"`
	TransparentPixel *PicturePixel `yaml:"transparent_pixel,omitempty" json:"transparentPixel,omitempty"`
	// Variants make a picture of its own a set of images under different
	// densities and interfaces, the way a common picture is one. File then
	// names a folder rather than a file - an identifier, so never taken for
	// an image - holding the files the variants name. The prototype keeps
	// such a set as Picture.zip, eight densities, the 8.2 image and the
	// description of the set: once in the configurations being moved, a
	// button of lombard1. Only a picture of an element of a form carries one
	// (validateElementPictureReference); no other owner keeps a set there.
	Variants []PictureVariant `yaml:"variants,omitempty" json:"variants,omitempty"`
}

// Names reports whether the reference names a picture to draw. A reference
// left with its settings after the picture was removed names none, and
// whatever it stands beside is drawn as if it had no picture.
func (picture *PictureReference) Names() bool {
	return picture != nil && (picture.Standard != "" || picture.Common != nil || picture.File != "")
}

// fileName is the file of its own the reference draws, or nothing.
func (picture *PictureReference) fileName() string {
	if picture == nil {
		return ""
	}
	return picture.File
}

// drawsFile reports whether a file lying in the owner's folder is the one the
// reference draws. The name is compared without regard to case, the way the
// prototype looks a picture's file up - see PictureVariant. It is asked by the
// owners that keep no set of variants; the elements of a form ask drawsEntry.
func (picture *PictureReference) drawsFile(file string) bool {
	return picture.fileName() != "" && strings.EqualFold(picture.File, file)
}

// drawsEntry reports whether an entry of the owner's folder is what the
// reference draws: its file, or the folder of its set of variants.
func (picture *PictureReference) drawsEntry(entry fs.DirEntry) bool {
	if picture.fileName() == "" || !strings.EqualFold(picture.File, entry.Name()) {
		return false
	}
	if len(picture.Variants) > 0 {
		return entry.IsDir() && entry.Type()&fs.ModeSymlink == 0
	}
	return entry.Type().IsRegular()
}

func (picture *PictureReference) clone() *PictureReference {
	if picture == nil {
		return nil
	}
	value := *picture
	value.Common = clonePointer(value.Common)
	value.TransparentPixel = clonePointer(value.TransparentPixel)
	value.Variants = slices.Clone(value.Variants)
	return &value
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
}

// validateObjectCommands checks the commands of one object. Names and
// identifiers are kept apart from the object's own, because a command is not an
// attribute and the two never collide in practice.
//
// The module of a command is not among what is checked here: it has no
// identifier to check. A command keeps a folder of its own named after it, and
// the module inside that folder is the command's by where it lies. That the
// file is there at all is checked against the folder, in validateObjectFiles.
func validateObjectCommands(commands []ObjectCommand, configuration project.Project) []string {
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
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
		issues = append(issues, validateCommandShape(prefix, command, configuration)...)
	}
	return issues
}

// validateCommandShape checks what makes a command a command: where it is
// shown, what it takes, and how it is drawn. It is shared by a command that
// belongs to an object and one that belongs to no object - the two differ in
// where they live, not in what they are.
func validateCommandShape(prefix string, command ObjectCommand, configuration project.Project) []string {
	var issues []string
	issues = append(issues, validateTitle(prefix+".title", command.Title, configuration)...)
	issues = append(issues, validateTitle(prefix+".tooltip", command.Tooltip, configuration)...)
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
		issues = append(issues, validateTypesIn(prefix+".parameter", command.Parameter, placeCommandParameter)...)
	}
	switch command.ParameterUse {
	case "", CommandParameterSingle, CommandParameterMultiple:
	default:
		issues = append(issues, prefix+".parameter_use must be single or multiple")
	}
	// How many objects a command takes is kept even when it takes none: the
	// prototype writes the mode on every command, and the configurations being
	// moved leave «single» without a parameter type 1096 times and «multiple»
	// 35 times. The mode means nothing until a type is given.
	issues = append(issues, validatePictureReference(prefix+".picture", command.Picture)...)
	switch command.Representation {
	case "", CommandAuto, CommandText, CommandPicture, CommandPictureAndText:
	default:
		issues = append(issues, prefix+".representation must be auto, text, picture or picture-and-text")
	}
	// A command may be drawn as a picture and have none: the prototype saves
	// it, and the configurations being moved do it twice. It is kept as
	// written and drawn as text - see ShownAs.
	switch command.OnServerUnavailable {
	case "", ServerUnavailableAuto, ServerUnavailableAvailable, ServerUnavailableNotAvailable:
	default:
		issues = append(issues, prefix+".on_server_unavailable must be auto, available or not-available")
	}
	return issues
}

// validatePictureReference checks that a picture names one source, not two.
// It may name none when it still carries a setting - see PictureReference;
// with neither it is written as no reference at all.
func validatePictureReference(path string, picture *PictureReference) []string {
	if picture != nil && len(picture.Variants) > 0 {
		return []string{path + ".variants belong to a picture of an element of a form only"}
	}
	return validatePictureSource(path, picture)
}

// validateElementPictureReference checks a picture an element of a form is
// drawn with, which alone may be a set of variants of its own.
func validateElementPictureReference(path string, picture *PictureReference) []string {
	issues := validatePictureSource(path, picture)
	if picture != nil {
		for _, issue := range validatePictureImages(PictureImages{Variants: picture.Variants}) {
			issues = append(issues, path+"."+issue)
		}
	}
	return issues
}

func validatePictureSource(path string, picture *PictureReference) []string {
	if picture == nil {
		return nil
	}
	issues := validateTransparentPixel(path+".transparent_pixel", picture.LoadTransparent, picture.TransparentPixel)
	sources := 0
	for _, named := range []bool{picture.Standard != "", picture.Common != nil, picture.File != ""} {
		if named {
			sources++
		}
	}
	switch {
	case sources > 1:
		issues = append(issues, path+" names more than one of a standard picture, a common picture and a file")
	case len(picture.Variants) > 0 && picture.File == "":
		issues = append(issues, path+".variants belong to a picture drawn from a file of its own")
	case len(picture.Variants) > 0 && project.SubordinateName(picture.File) != nil:
		issues = append(issues, path+".file must be the name of the folder of its variants, an identifier")
	case len(picture.Variants) == 0 && picture.File != "" && !PictureFile(picture.File):
		issues = append(issues, path+".file must be the name of an image file")
	case !picture.Names() && !picture.LoadTransparent && picture.TransparentPixel == nil:
		issues = append(issues, path+" must name a standard picture, a common picture or a file, or carry what is left of one")
	case picture.Standard != "" && !validIdentifier(picture.Standard):
		issues = append(issues, path+".standard must be a valid identifier")
	case picture.Common != nil && picture.Common.IsZero():
		issues = append(issues, path+".common must be a non-zero UUID")
	}
	return issues
}

func cloneObjectCommands(commands []ObjectCommand) []ObjectCommand {
	commands = slices.Clone(commands)
	for index := range commands {
		command := &commands[index]
		command.Title = cloneTitle(command.Title)
		command.Tooltip = cloneTitle(command.Tooltip)
		command.Parameter = cloneTypes(command.Parameter)
		command.Picture = command.Picture.clone()
		if command.GroupRef != nil {
			copied := *command.GroupRef
			command.GroupRef = &copied
		}
	}
	return commands
}
