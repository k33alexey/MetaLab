package metadata

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// CommandInterface is what one section of the application shows and in what
// order: which commands it shows to which roles, where it puts a command, how
// it orders commands within a group, how it orders the groups, and how it
// orders the sections nested in it. The prototype keeps it beside the
// subsystem, 192/84/100 of them in the configurations being moved; the root's
// half - the order and the visibility of the top-level sections - lies on the
// root, see project.Project.
//
// It is carried, not yet acted upon: the command interface is built in block
// 18. Every reference is resolved where it can be; what cannot be is carried
// as written and listed among the unresolved references - see
// UnresolvedReference.
type CommandInterface struct {
	CommandsVisibility []InterfaceCommandVisibility `yaml:"commands_visibility,omitempty" json:"commandsVisibility,omitempty"`
	CommandsPlacement  []InterfaceCommandPlacement  `yaml:"commands_placement,omitempty" json:"commandsPlacement,omitempty"`
	CommandsOrder      []InterfaceCommandOrder      `yaml:"commands_order,omitempty" json:"commandsOrder,omitempty"`
	GroupsOrder        []InterfaceGroup             `yaml:"groups_order,omitempty" json:"groupsOrder,omitempty"`
	// SubsystemsOrder orders the subsystems nested in this one.
	SubsystemsOrder []uuid.UUID `yaml:"subsystems_order,omitempty" json:"subsystemsOrder,omitempty"`
}

// IsEmpty reports whether the subsystem keeps a command interface at all.
func (value CommandInterface) IsEmpty() bool {
	return len(value.CommandsVisibility) == 0 && len(value.CommandsPlacement) == 0 &&
		len(value.CommandsOrder) == 0 && len(value.GroupsOrder) == 0 && len(value.SubsystemsOrder) == 0
}

// InterfaceCommand names one command the way the command interface does: an
// object's own command or a common command by its identifier, or a standard
// command of an object - open its list, open it, create one, create a folder.
//
// Written is the third way, and it is not ours: about a thousand commands of
// the configurations being moved are written in the prototype's internal
// notation - "0:<uuid>", "1:<uuid>", "100:<uuid>", a bare "0" - which names
// nothing the configuration carries. It is kept as written so that nothing is
// lost, and never acted upon.
type InterfaceCommand struct {
	Command  *uuid.UUID      `yaml:"command,omitempty" json:"command,omitempty"`
	Object   *uuid.UUID      `yaml:"object,omitempty" json:"object,omitempty"`
	Standard StandardCommand `yaml:"standard,omitempty" json:"standard,omitempty"`
	Written  string          `yaml:"written,omitempty" json:"written,omitempty"`
}

// StandardCommand is a command the platform gives an object by itself. The set
// is the one the configurations being moved place in their sections.
type StandardCommand string

const (
	StandardOpenList     StandardCommand = "open-list"
	StandardOpen         StandardCommand = "open"
	StandardCreate       StandardCommand = "create"
	StandardCreateFolder StandardCommand = "create-folder"
)

// InterfaceGroup names one group of a section: a group the platform has by
// itself, or a command group of the configuration by its identifier. Written
// is a group the prototype wrote by an identifier that is not a command group -
// the nil identifier among them, ten times - kept as written.
type InterfaceGroup struct {
	Standard InterfaceGroupKind `yaml:"standard,omitempty" json:"standard,omitempty"`
	Group    *uuid.UUID         `yaml:"group,omitempty" json:"group,omitempty"`
	Written  string             `yaml:"written,omitempty" json:"written,omitempty"`
}

// InterfaceGroupKind is a group of a section the platform has by itself: three
// of the navigation panel and three of the actions panel.
type InterfaceGroupKind string

const (
	NavigationPanelOrdinary  InterfaceGroupKind = "navigation-panel-ordinary"
	NavigationPanelImportant InterfaceGroupKind = "navigation-panel-important"
	NavigationPanelSeeAlso   InterfaceGroupKind = "navigation-panel-see-also"
	ActionsPanelCreate       InterfaceGroupKind = "actions-panel-create"
	ActionsPanelReports      InterfaceGroupKind = "actions-panel-reports"
	ActionsPanelTools        InterfaceGroupKind = "actions-panel-tools"
)

// InterfaceCommandVisibility is whether a section shows one command.
type InterfaceCommandVisibility struct {
	InterfaceCommand            `yaml:",inline"`
	project.InterfaceVisibility `yaml:"visibility" json:"visibility"`
}

// InterfaceCommandPlacement moves one command into another group of the
// section.
type InterfaceCommandPlacement struct {
	InterfaceCommand `yaml:",inline"`
	Group            InterfaceGroup   `yaml:"group" json:"group"`
	Placement        CommandPlacement `yaml:"placement,omitempty" json:"placement,omitempty"`
}

// CommandPlacement is how a placed command stands in its group. The
// configurations being moved write one value only.
type CommandPlacement string

const PlacementAuto CommandPlacement = "auto"

// InterfaceCommandOrder puts one command in its place within a group: the
// order of the list is the order of the commands.
type InterfaceCommandOrder struct {
	InterfaceCommand `yaml:",inline"`
	Group            InterfaceGroup `yaml:"group" json:"group"`
}

func validateCommandInterface(path string, value CommandInterface) []string {
	var issues []string
	for index, item := range value.CommandsVisibility {
		prefix := fmt.Sprintf("%s.commands_visibility[%d]", path, index)
		issues = append(issues, validateInterfaceCommand(prefix, item.InterfaceCommand)...)
		issues = append(issues, project.ValidateInterfaceVisibility(prefix+".visibility", item.InterfaceVisibility)...)
	}
	for index, item := range value.CommandsPlacement {
		prefix := fmt.Sprintf("%s.commands_placement[%d]", path, index)
		issues = append(issues, validateInterfaceCommand(prefix, item.InterfaceCommand)...)
		issues = append(issues, validateInterfaceGroup(prefix+".group", item.Group)...)
		if item.Placement != "" && item.Placement != PlacementAuto {
			issues = append(issues, prefix+".placement must be auto")
		}
	}
	for index, item := range value.CommandsOrder {
		prefix := fmt.Sprintf("%s.commands_order[%d]", path, index)
		issues = append(issues, validateInterfaceCommand(prefix, item.InterfaceCommand)...)
		issues = append(issues, validateInterfaceGroup(prefix+".group", item.Group)...)
	}
	for index, group := range value.GroupsOrder {
		issues = append(issues, validateInterfaceGroup(fmt.Sprintf("%s.groups_order[%d]", path, index), group)...)
	}
	seen := make(map[uuid.UUID]bool, len(value.SubsystemsOrder))
	for index, id := range value.SubsystemsOrder {
		prefix := fmt.Sprintf("%s.subsystems_order[%d]", path, index)
		if id.IsZero() {
			issues = append(issues, prefix+" must be a non-zero UUID")
		} else if seen[id] {
			issues = append(issues, prefix+" must be unique")
		}
		seen[id] = true
	}
	return issues
}

// validateInterfaceCommand checks that a command is named exactly one way.
func validateInterfaceCommand(path string, value InterfaceCommand) []string {
	ways := 0
	var issues []string
	if value.Command != nil {
		ways++
		if value.Command.IsZero() {
			issues = append(issues, path+".command must be a non-zero UUID")
		}
	}
	if value.Object != nil || value.Standard != "" {
		ways++
		switch {
		case value.Object == nil || value.Object.IsZero():
			issues = append(issues, path+".object must name the object of the standard command")
		case !knownStandardCommand(value.Standard):
			issues = append(issues, path+".standard must be open-list, open, create or create-folder")
		}
	}
	if value.Written != "" {
		ways++
		issues = append(issues, validateWritten(path+".written", value.Written)...)
	}
	if ways != 1 {
		issues = append(issues, path+" must name the command exactly one way: command, object with standard, or written")
	}
	return issues
}

func validateInterfaceGroup(path string, value InterfaceGroup) []string {
	ways := 0
	var issues []string
	if value.Standard != "" {
		ways++
		if !knownInterfaceGroupKind(value.Standard) {
			issues = append(issues, path+".standard must be a group of the navigation or the actions panel")
		}
	}
	if value.Group != nil {
		ways++
		if value.Group.IsZero() {
			issues = append(issues, path+".group must be a non-zero UUID")
		}
	}
	if value.Written != "" {
		ways++
		issues = append(issues, validateWritten(path+".written", value.Written)...)
	}
	if ways != 1 {
		issues = append(issues, path+" must name the group exactly one way: standard, group, or written")
	}
	return issues
}

// validateWritten admits a reference carried as the prototype wrote it: one
// line, nothing invisible in it.
func validateWritten(path, value string) []string {
	if strings.TrimSpace(value) != value {
		return []string{path + " must not start or end with a space"}
	}
	for _, symbol := range value {
		if unicode.IsControl(symbol) {
			return []string{path + " must not contain control characters"}
		}
	}
	return nil
}

func knownStandardCommand(value StandardCommand) bool {
	switch value {
	case StandardOpenList, StandardOpen, StandardCreate, StandardCreateFolder:
		return true
	}
	return false
}

func knownInterfaceGroupKind(value InterfaceGroupKind) bool {
	switch value {
	case NavigationPanelOrdinary, NavigationPanelImportant, NavigationPanelSeeAlso,
		ActionsPanelCreate, ActionsPanelReports, ActionsPanelTools:
		return true
	}
	return false
}

// resolveCommandInterfaces notes every reference of the command interface
// that points at nothing, the root's half included. Nothing is refused: a
// command hidden from a section, or a section ordered, that the configuration
// no longer has is a line the prototype keeps and draws nothing from.
//
// It runs against a project on disk only: a catalog assembled from a snapshot
// holds only some kinds, and a command of a kind it does not hold would look
// stale when it is merely not read.
func (catalog *Catalog) resolveCommandInterfaces(root string) {
	if root == "" {
		return
	}
	commands := map[uuid.UUID]bool{}
	for _, item := range catalog.CommonCommands {
		commands[item.ID] = true
	}
	for _, owned := range catalog.everyObjectCommands() {
		for _, command := range owned.commands {
			commands[command.ID] = true
		}
	}
	role := func(where string, visibility project.InterfaceVisibility) {
		if !catalog.rolesLoaded {
			return
		}
		for _, item := range visibility.Roles {
			if _, ok := catalog.roleByID[item.Role]; !ok {
				catalog.noteUnresolved(where+" role", item.Role)
			}
		}
	}
	command := func(where string, value InterfaceCommand) {
		switch {
		case value.Command != nil && !commands[*value.Command]:
			catalog.noteUnresolved(where+" command", *value.Command)
		case value.Object != nil && !catalog.knownMetadataObject(*value.Object):
			catalog.noteUnresolved(where+" object", *value.Object)
		}
	}
	group := func(where string, value InterfaceGroup) {
		if value.Group != nil {
			if _, ok := catalog.commandGroupByID[*value.Group]; !ok {
				catalog.noteUnresolved(where+" group", *value.Group)
			}
		}
	}
	subsystem := func(where string, id uuid.UUID) {
		if _, ok := catalog.subsystemByID[id]; !ok {
			catalog.noteUnresolved(where, id)
		}
	}
	for _, item := range catalog.Subsystems {
		where := "subsystem " + item.Name + " command interface"
		value := item.CommandInterface
		for _, entry := range value.CommandsVisibility {
			command(where, entry.InterfaceCommand)
			role(where, entry.InterfaceVisibility)
		}
		for _, entry := range value.CommandsPlacement {
			command(where, entry.InterfaceCommand)
			group(where, entry.Group)
		}
		for _, entry := range value.CommandsOrder {
			command(where, entry.InterfaceCommand)
			group(where, entry.Group)
		}
		for _, entry := range value.GroupsOrder {
			group(where, entry)
		}
		for _, id := range value.SubsystemsOrder {
			subsystem(where+" subsystem", id)
		}
	}
	for _, id := range catalog.Project.SubsystemsOrder {
		subsystem("the configuration's order of sections", id)
	}
	for _, item := range catalog.Project.SubsystemsVisibility {
		subsystem("the configuration's visibility of sections", item.Subsystem)
		role("the configuration's visibility of sections", item.InterfaceVisibility)
	}
}

// cloneCommandInterface copies everything a caller could change through the
// value it was handed. The identifiers inside are values, so copying the
// lists copies them too, except the pointers, which are copied one by one.
func cloneCommandInterface(value CommandInterface) CommandInterface {
	command := func(item InterfaceCommand) InterfaceCommand {
		item.Command, item.Object = cloneUUIDPointer(item.Command), cloneUUIDPointer(item.Object)
		return item
	}
	group := func(item InterfaceGroup) InterfaceGroup {
		item.Group = cloneUUIDPointer(item.Group)
		return item
	}
	visibility := func(item project.InterfaceVisibility) project.InterfaceVisibility {
		item.Roles = append([]project.RoleVisibility(nil), item.Roles...)
		return item
	}
	result := CommandInterface{SubsystemsOrder: append([]uuid.UUID(nil), value.SubsystemsOrder...)}
	for _, item := range value.CommandsVisibility {
		result.CommandsVisibility = append(result.CommandsVisibility,
			InterfaceCommandVisibility{InterfaceCommand: command(item.InterfaceCommand), InterfaceVisibility: visibility(item.InterfaceVisibility)})
	}
	for _, item := range value.CommandsPlacement {
		result.CommandsPlacement = append(result.CommandsPlacement,
			InterfaceCommandPlacement{InterfaceCommand: command(item.InterfaceCommand), Group: group(item.Group), Placement: item.Placement})
	}
	for _, item := range value.CommandsOrder {
		result.CommandsOrder = append(result.CommandsOrder,
			InterfaceCommandOrder{InterfaceCommand: command(item.InterfaceCommand), Group: group(item.Group)})
	}
	for _, item := range value.GroupsOrder {
		result.GroupsOrder = append(result.GroupsOrder, group(item))
	}
	return result
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
