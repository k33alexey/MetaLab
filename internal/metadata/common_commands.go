package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	// CommonCommandKind holds actions that belong to no object: they are
	// offered by the configuration itself.
	CommonCommandKind Kind = "common-commands"
	// CommandGroupKind holds the places a configuration makes for its own
	// commands, beside the places the platform offers.
	CommandGroupKind Kind = "command-groups"
)

// CommandCategory is the part of the interface a command group lives in. The
// platform offers its own places; a group of the configuration says which of
// them it stands among.
type CommandCategory string

const (
	ActionsPanelCategory   CommandCategory = "actions-panel"
	FormCommandBarCategory CommandCategory = "form-command-bar"
)

// CommonCommandDefinition is a command belonging to no object.
//
// It is the same thing as a command of an object - the same placement, the
// same parameter, the same picture - and it carries that sameness by reusing
// the type, so the two cannot drift apart. What it adds is where it lives: a
// folder of its own, holding the module that runs it.
type CommonCommandDefinition struct {
	Format        int `yaml:"format" json:"format"`
	ObjectCommand `yaml:",inline"`
	// IncludeHelpInContents puts this command's help into the table of
	// contents of the configuration's help.
	IncludeHelpInContents bool `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
}

// CommandGroupDefinition is a place the configuration makes for its commands.
// It holds nothing and does nothing: it is a name, a presentation and a
// category saying which part of the interface it stands in.
type CommandGroupDefinition struct {
	Format   int               `yaml:"format" json:"format"`
	ID       uuid.UUID         `yaml:"id" json:"id"`
	Name     string            `yaml:"name" json:"name"`
	Title    LocalizedText     `yaml:"title" json:"title"`
	Comment  string            `yaml:"comment,omitempty" json:"comment,omitempty"`
	Tooltip  LocalizedText     `yaml:"tooltip,omitempty" json:"tooltip,omitempty"`
	Category CommandCategory   `yaml:"category" json:"category"`
	Picture  *PictureReference `yaml:"picture,omitempty" json:"picture,omitempty"`
	// Representation decides whether the group shows its picture, its text or
	// both, the same way a command does.
	Representation CommandRepresentation `yaml:"representation,omitempty" json:"representation,omitempty"`
}

// DecodeCommonCommand reads and validates one common command.
func DecodeCommonCommand(source string, reader io.Reader, manifest project.Project) (CommonCommandDefinition, error) {
	var value CommonCommandDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CommonCommandDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	// The command is checked as any command is, against its own identifier:
	// nothing owns it, so there is no object identifier to keep apart from.
	issues = append(issues, validateCommandShape("command", value.ObjectCommand, value.ID, manifest)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return CommonCommandDefinition{}, err
	}
	return value, nil
}

// DecodeCommandGroup reads and validates one command group.
func DecodeCommandGroup(source string, reader io.Reader, manifest project.Project) (CommandGroupDefinition, error) {
	var value CommandGroupDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CommandGroupDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if len(value.Tooltip) > 0 {
		issues = append(issues, validateTitle("tooltip", value.Tooltip, manifest)...)
	}
	switch value.Category {
	case ActionsPanelCategory, FormCommandBarCategory:
	default:
		issues = append(issues, "category must be actions-panel or form-command-bar")
	}
	issues = append(issues, validatePictureReference("picture", value.Picture)...)
	switch value.Representation {
	case "", CommandAuto, CommandText, CommandPicture, CommandPictureAndText:
	default:
		issues = append(issues, "representation must be auto, text, picture or picture-and-text")
	}
	if value.Representation == CommandPicture && value.Picture == nil {
		issues = append(issues, "representation picture needs a picture")
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return CommandGroupDefinition{}, err
	}
	return value, nil
}

func cloneCommonCommand(value CommonCommandDefinition) CommonCommandDefinition {
	value.ObjectCommand = cloneObjectCommands([]ObjectCommand{value.ObjectCommand})[0]
	return value
}

func cloneCommandGroup(value CommandGroupDefinition) CommandGroupDefinition {
	value.Title = cloneTitle(value.Title)
	value.Tooltip = cloneTitle(value.Tooltip)
	if value.Picture != nil {
		picture := *value.Picture
		if picture.Common != nil {
			id := *picture.Common
			picture.Common = &id
		}
		value.Picture = &picture
	}
	return value
}

// CommonCommand returns one common command by name, folded case.
func (catalog *Catalog) CommonCommand(name string) (CommonCommandDefinition, bool) {
	index, ok := catalog.commonCommandByName[strings.ToLower(name)]
	if !ok {
		return CommonCommandDefinition{}, false
	}
	return cloneCommonCommand(catalog.CommonCommands[index]), true
}

// CommandGroup returns one command group by name, folded case.
func (catalog *Catalog) CommandGroup(name string) (CommandGroupDefinition, bool) {
	index, ok := catalog.commandGroupByName[strings.ToLower(name)]
	if !ok {
		return CommandGroupDefinition{}, false
	}
	return cloneCommandGroup(catalog.CommandGroups[index]), true
}

// validateCommandGroupReferences checks that every command placed in a group
// of the configuration names a group that exists.
//
// Until command groups were objects there was nothing to check this against,
// and a command could name a group that had never existed: the platform would
// then have nowhere to draw it, and the command would simply not appear.
func (catalog *Catalog) validateCommandGroupReferences() error {
	placed := func(owner string, command ObjectCommand) error {
		if command.GroupRef == nil {
			return nil
		}
		if _, ok := catalog.commandGroupByID[*command.GroupRef]; !ok {
			return fmt.Errorf("%s command %s is placed in unknown command group %s",
				owner, command.Name, command.GroupRef)
		}
		return nil
	}
	for _, item := range catalog.CommonCommands {
		if err := placed("common", item.ObjectCommand); err != nil {
			return err
		}
	}
	for _, owned := range catalog.everyObjectCommands() {
		for _, command := range owned.commands {
			if err := placed(owned.owner, command); err != nil {
				return err
			}
		}
	}
	return nil
}

// ownedCommands is one object and the commands it keeps.
type ownedCommands struct {
	owner    string
	commands []ObjectCommand
}

// everyObjectCommands lists the commands of every object that keeps any. It is
// spelled out kind by kind on purpose: a kind left out here is a kind whose
// commands nothing checks, and that is exactly the silence this check exists
// to break.
func (catalog *Catalog) everyObjectCommands() []ownedCommands {
	var result []ownedCommands
	add := func(kind, name string, commands []ObjectCommand) {
		if len(commands) > 0 {
			result = append(result, ownedCommands{owner: kind + " " + name, commands: commands})
		}
	}
	for _, item := range catalog.Catalogs {
		add("catalog", item.Name, item.Commands)
	}
	for _, item := range catalog.Documents {
		add("document", item.Name, item.Commands)
	}
	for _, item := range catalog.DocumentJournals {
		add("document journal", item.Name, item.Commands)
	}
	for _, item := range catalog.Enumerations {
		add("enumeration", item.Name, item.Commands)
	}
	for _, item := range catalog.InformationRegisters {
		add("information register", item.Name, item.Commands)
	}
	for _, item := range catalog.AccumulationRegisters {
		add("accumulation register", item.Name, item.Commands)
	}
	for _, item := range catalog.AccountingRegisters {
		add("accounting register", item.Name, item.Commands)
	}
	for _, item := range catalog.CalculationRegisters {
		add("calculation register", item.Name, item.Commands)
	}
	for _, item := range catalog.ChartsOfCharacteristicTypes {
		add("chart of characteristic types", item.Name, item.Commands)
	}
	for _, item := range catalog.ChartsOfAccounts {
		add("chart of accounts", item.Name, item.Commands)
	}
	for _, item := range catalog.ChartsOfCalculationTypes {
		add("chart of calculation types", item.Name, item.Commands)
	}
	for _, item := range catalog.BusinessProcesses {
		add("business process", item.Name, item.Commands)
	}
	for _, item := range catalog.Tasks {
		add("task", item.Name, item.Commands)
	}
	for _, item := range catalog.ExchangePlans {
		add("exchange plan", item.Name, item.Commands)
	}
	for _, item := range catalog.Reports {
		add("report", item.Name, item.Commands)
	}
	for _, item := range catalog.DataProcessors {
		add("data processor", item.Name, item.Commands)
	}
	for _, item := range catalog.FilterCriteria {
		add("filter criterion", item.Name, item.Commands)
	}
	return result
}

// validateCommonCommandFiles checks the folder of every common command: it
// holds the command's description and the module that runs it, and nothing
// else.
//
// The module is required for the same reason a command of an object needs one:
// a command without a body is a place in the interface that answers a click
// with nothing, which is worse than not offering it at all.
func (catalog *Catalog) validateCommonCommandFiles(root string) error {
	if root == "" {
		return nil
	}
	for _, item := range catalog.CommonCommands {
		found, err := namedFolderContents(root, CommonCommandKind, "common command", item.Name)
		if err != nil {
			return err
		}
		if !found[project.CommandModuleFile] {
			return fmt.Errorf("common command %s has no module", item.Name)
		}
	}
	return nil
}

// namedFolderContents reads the folder of one object of a kind that keeps only
// files there, checks it holds nothing else, and reports which of the allowed
// files are actually present.
//
// Which of them are required is the kind's own business: a command without its
// module answers a click with nothing, while a constant without one is simply a
// constant nobody wrote code for.
func namedFolderContents(root string, kind Kind, what, name string) (map[string]bool, error) {
	allowed, ok := project.NamedFolderFiles(string(kind))
	if !ok {
		return nil, fmt.Errorf("kind %q keeps no folder of its own", kind)
	}
	entries, err := os.ReadDir(filepath.Join(root, "metadata", string(kind), name))
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", what, name, err)
	}
	found := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || !slices.Contains(allowed, entry.Name()) {
			return nil, fmt.Errorf("%s %s keeps %q, and it keeps only %s",
				what, name, entry.Name(), strings.Join(allowed, ", "))
		}
		found[entry.Name()] = true
	}
	return found, nil
}
