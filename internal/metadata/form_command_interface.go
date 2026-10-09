package metadata

import (
	"fmt"
	"slices"
	"strings"
)

// FormCommandInterface is where the commands that are no element of the form
// stand on it: the navigation panel of the form and its command bar (help,
// "Editing the command interface of a form"; 2216 forms of the exports keep
// one). Each panel is a list of lines, in the order the prototype writes
// them, and a line is either a command the platform puts on the form by
// itself - the commands of the objects subordinate to the form's object,
// say - which the form shows otherwise or elsewhere, or one the form adds.
//
// It is carried, not yet acted upon: the command interface is built in block
// 18.
type FormCommandInterface struct {
	NavigationPanel []FormInterfaceCommand `yaml:"navigation_panel,omitempty" json:"navigationPanel,omitempty"`
	CommandBar      []FormInterfaceCommand `yaml:"command_bar,omitempty" json:"commandBar,omitempty"`
}

// FormInterfaceCommand is one line of a panel of a form.
//
// A line is no unique key: the prototype writes the same command into one
// panel again and again - with another group, another source of its
// parameter, or with nothing different at all (NoteRepeatedInterfaceCommand).
type FormInterfaceCommand struct {
	// Command is written as the command of a button is: CommonCommand.<name>,
	// <kind>.<object>.Command.<name> and the rest (see buttonCommandKind).
	// About 8600 lines of the exports name it by the code of an element of a
	// form - "0", "0:<uuid>", "3:<uuid>" - which is carried as written and
	// noted.
	Command string `yaml:"command" json:"command"`
	// Added is a line the form adds; the others are commands the platform
	// puts there itself (12613 and 4164 of the exports, against 433 and 218
	// added).
	Added bool `yaml:"added,omitempty" json:"added,omitempty"`
	// Group is the group of the panel the command stands in: one the
	// platform gives the panel, a command group of the configuration
	// (CommandGroup.<name>, as the prototype writes it), or the identifier of
	// a command group the configuration does not have - the group of a
	// library left out of it, 165 times - kept as written. None is the
	// platform's choice.
	Group string `yaml:"group,omitempty" json:"group,omitempty"`
	// Index is the place of the command in its group, from 1; none puts it
	// where the platform does (the "auto position" of the editor).
	Index int `yaml:"index,omitempty" json:"index,omitempty"`
	// Visibility is set when the line does not leave the visibility to the
	// platform (the prototype's DefaultVisible false, the "auto visibility"
	// of the editor turned off): whether the command is shown, and to which
	// roles otherwise. The prototype leaves out the visibility when the
	// command is shown to everyone, which is common: true with no roles.
	Visibility *FormAttributeRight `yaml:"visibility,omitempty" json:"visibility,omitempty"`
	// DataPath is where the command takes its parameter from: an attribute
	// of the form ("Объект.Ref") or the current row of a table
	// ("Items.Список.CurrentData.Ref"), written as the data path of an
	// element is.
	DataPath string `yaml:"data_path,omitempty" json:"dataPath,omitempty"`
}

// IsEmpty reports whether the form keeps a command interface at all.
func (value FormCommandInterface) IsEmpty() bool {
	return len(value.NavigationPanel) == 0 && len(value.CommandBar) == 0
}

// The groups the platform gives the panels of a form (help,
// StandardCommandsGroup): three of the navigation panel, two of the command
// bar.
const (
	FormNavigationPanelImportant = "form-navigation-panel-important"
	FormNavigationPanelGoTo      = "form-navigation-panel-go-to"
	FormNavigationPanelSeeAlso   = "form-navigation-panel-see-also"
	FormCommandBarImportant      = "form-command-bar-important"
	FormCommandBarCreateBasedOn  = "form-command-bar-create-based-on"
)

// formInterfacePanel is one panel of a form: its name in the model, the
// groups the platform gives it, and the category a command group of the
// configuration standing in it must have.
type formInterfacePanel struct {
	name     string
	groups   []string
	category CommandCategory
}

var (
	formNavigationPanel = formInterfacePanel{"navigation_panel",
		[]string{FormNavigationPanelImportant, FormNavigationPanelGoTo, FormNavigationPanelSeeAlso}, FormNavigationPanelCategory}
	formCommandBarPanel = formInterfacePanel{"command_bar",
		[]string{FormCommandBarImportant, FormCommandBarCreateBasedOn}, FormCommandBarCategory}
)

func (value FormCommandInterface) panels() []struct {
	panel formInterfacePanel
	lines []FormInterfaceCommand
} {
	return []struct {
		panel formInterfacePanel
		lines []FormInterfaceCommand
	}{{formNavigationPanel, value.NavigationPanel}, {formCommandBarPanel, value.CommandBar}}
}

// validateFormCommandInterface checks the command interface of a form
// against the form alone: names is every element of the form, commands every
// command of it, both folded.
func validateFormCommandInterface(value *FormCommandInterface, names, commands map[string]bool) []string {
	if value == nil {
		return nil
	}
	var issues []string
	for _, panel := range value.panels() {
		for index, line := range panel.lines {
			path := fmt.Sprintf("command_interface.%s[%d]", panel.panel.name, index)
			switch command, _, ok := parseInterfaceCommand(line.Command); {
			case !ok:
				issues = append(issues, path+".command must be "+commandWritings)
			case command.kind == formCommand && !commands[foldedName(command.name)]:
				issues = append(issues, path+".command names no command of the form")
			case command.kind == elementStandardCommand && !names[foldedName(command.element)]:
				issues = append(issues, path+".command names no element of the form")
			}
			if group := line.Group; group != "" && !isFormStandardGroup(group) {
				if name, found := strings.CutPrefix(group, "CommandGroup."); !(found && validIdentifier(name)) && !isUUIDText(group) {
					issues = append(issues, path+".group must be a group of the panel, CommandGroup.<name> or the identifier of a command group")
				}
			} else if group != "" && !slices.Contains(panel.panel.groups, group) {
				issues = append(issues, path+".group must be one of "+strings.Join(panel.panel.groups, ", "))
			}
			if line.Index < 0 {
				issues = append(issues, path+".index must be a place in the group, from 1")
			}
			issues = append(issues, validateFormRight(path+".visibility", line.Visibility)...)
			if line.DataPath != "" {
				issues = append(issues, validateElementDataPath(path+".data_path", line.DataPath)...)
			}
		}
	}
	return issues
}

// parseInterfaceCommand reads a command of the command interface of a form.
// It is written as the command of a button, and one way more: the command
// that opens the records of an information register by the value of its
// master dimension,
//
//	InformationRegister.<register>.StandardCommand.OpenByValue.<dimension>
//
// the command of an object subordinate to the form's object that the help
// names ("Editing the command interface of a form"); 360 lines of the
// exports, each naming a master dimension of a register the configuration
// has. A button never writes it.
func parseInterfaceCommand(value string) (command buttonCommand, dimension string, ok bool) {
	parts := strings.Split(value, ".")
	if len(parts) == 5 && parts[0] == "InformationRegister" && parts[2] == "StandardCommand" && parts[3] == "OpenByValue" &&
		validIdentifier(parts[1]) && validIdentifier(parts[4]) {
		return buttonCommand{kind: objectStandardCommand, owner: parts[0], object: parts[1], name: parts[3]}, parts[4], true
	}
	command, ok = parseButtonCommand(value)
	return command, "", ok
}

// resolveOpenByValue checks that the register of the command is there and
// that the dimension it opens by is a master one of it.
func (catalog *Catalog) resolveOpenByValue(where, text string) {
	parts := strings.Split(text, ".")
	if position, ok := catalog.informationRegisterByName[strings.ToLower(parts[1])]; ok {
		for _, dimension := range catalog.InformationRegisters[position].Dimensions {
			if dimension.Master && strings.EqualFold(dimension.Name, parts[4]) {
				return
			}
		}
	}
	catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, Written: text})
}

func isFormStandardGroup(value string) bool {
	return slices.Contains(formNavigationPanel.groups, value) || slices.Contains(formCommandBarPanel.groups, value)
}

// resolveFormCommandInterface resolves what the command interface of one
// form names in the project: the commands, the command groups and the roles
// a command is shown to. A command or a group the project does not have is a
// reference to nothing; a group of another panel is refused, as the
// prototype never writes one (2512 of 2512 stand in their own). What the
// prototype wrote as a code, and a line repeated whole, is noted.
func (catalog *Catalog) resolveFormCommandInterface(form string, value *FormCommandInterface) error {
	if value == nil {
		return nil
	}
	for _, panel := range value.panels() {
		seen := map[string]bool{}
		for index, line := range panel.lines {
			where := fmt.Sprintf("%s command interface %s[%d]", form, panel.panel.name, index)
			if _, dimension, _ := parseInterfaceCommand(line.Command); dimension != "" {
				catalog.resolveOpenByValue(where+" command", line.Command)
			} else {
				catalog.resolveCommandText(where+" command", line.Command)
			}
			switch name, byName := strings.CutPrefix(line.Group, "CommandGroup."); {
			case byName:
				position, ok := catalog.commandGroupByName[strings.ToLower(name)]
				if !ok {
					catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where + " group", Written: line.Group})
				} else if category := catalog.CommandGroups[position].Category; category != panel.panel.category {
					return fmt.Errorf("%s group %s stands in the %s, not in the %s", where, line.Group, category, panel.panel.category)
				}
			case isUUIDText(line.Group):
				catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where + " group", Written: line.Group})
			}
			if err := catalog.resolveFormData(where, nil, nil, line.Visibility); err != nil {
				return err
			}
			// A code of several segments is not in the command interface of
			// any form of the exports and stays a note.
			if strings.HasPrefix(line.DataPath, "~") && strings.Contains(line.DataPath[1:], "~") ||
				formElementCode.MatchString(line.DataPath) || formPathCode.MatchString(line.DataPath) {
				catalog.noteElementCode(where+" data_path", line.DataPath)
			}
			key := fmt.Sprintf("%s\x00%t\x00%s\x00%d\x00%s", foldedName(line.Command), line.Added, line.Group, line.Index, line.DataPath)
			if line.Visibility != nil {
				key = fmt.Sprintf("%s\x00%+v", key, *line.Visibility)
			}
			if seen[key] && !formElementCode.MatchString(line.Command) {
				catalog.noteForm(NoteRepeatedInterfaceCommand, where, line.Command)
			}
			seen[key] = true
		}
	}
	return nil
}
