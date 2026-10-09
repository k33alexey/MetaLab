package metadata

import (
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A button names the command it runs as the prototype writes it (help,
// FormButton.CommandName - a string): by name, with what the command belongs
// to in front of it.
//
//	Form.Command.<name>                         a command of the form (35398 in the exports)
//	Form.StandardCommand.<name>                 a standard command of the form (7618)
//	Form.Item.<element>.StandardCommand.<name>  a standard command of an element of the form (9028)
//	CommonCommand.<name>                        a common command (1954)
//	<kind>.<object>.Command.<name>              a command of an object of the configuration (536)
//	<kind>.<object>.StandardCommand.<name>      a standard command of one (419)
//
// The prototype writes the code of an element of a form in its place as well
// - a number and an identifier (309), or a lone 0 (262) - which is carried as
// written and noted, as no one knows what it points at.
type buttonCommandKind int

const (
	formCommand buttonCommandKind = iota + 1
	formStandardCommand
	elementStandardCommand
	commonCommand
	objectCommand
	objectStandardCommand
	elementCodeCommand
)

type buttonCommand struct {
	kind buttonCommandKind
	// owner is the word the prototype names the kind of an object by, and
	// object its name; element is the element of the form.
	owner, object, element string
	name                   string
}

// commandOwnerKinds are the kinds of object a button may name a command of,
// by the word the prototype writes for each, with the list of the catalog
// they are read from. A common form has only standard commands.
var commandOwnerKinds = map[string]string{
	"Catalog": "Catalogs", "Document": "Documents", "Enum": "Enumerations",
	"ChartOfCharacteristicTypes": "ChartsOfCharacteristicTypes", "ChartOfAccounts": "ChartsOfAccounts",
	"ChartOfCalculationTypes": "ChartsOfCalculationTypes", "BusinessProcess": "BusinessProcesses", "Task": "Tasks",
	"ExchangePlan": "ExchangePlans", "DocumentJournal": "DocumentJournals", "InformationRegister": "InformationRegisters",
	"AccumulationRegister": "AccumulationRegisters", "AccountingRegister": "AccountingRegisters",
	"CalculationRegister": "CalculationRegisters", "Report": "Reports", "DataProcessor": "DataProcessors",
	"FilterCriterion": "FilterCriteria", "CommonForm": "",
}

// commandWritings is every way a command is written, for the message that
// refuses another.
const commandWritings = "Form.Command.<name>, Form.StandardCommand.<name>, " +
	"Form.Item.<element>.StandardCommand.<name>, CommonCommand.<name>, <kind>.<object>.Command.<name>, " +
	"<kind>.<object>.StandardCommand.<name> or the code of an element of a form"

// parseButtonCommand reads the command of a button; false is something no
// command is written as.
func parseButtonCommand(value string) (buttonCommand, bool) {
	if formElementCode.MatchString(value) {
		return buttonCommand{kind: elementCodeCommand}, true
	}
	parts := strings.Split(value, ".")
	for _, part := range parts {
		if !validIdentifier(part) {
			return buttonCommand{}, false
		}
	}
	switch {
	case len(parts) == 3 && parts[0] == "Form" && parts[1] == "Command":
		return buttonCommand{kind: formCommand, name: parts[2]}, true
	case len(parts) == 3 && parts[0] == "Form" && parts[1] == "StandardCommand":
		return buttonCommand{kind: formStandardCommand, name: parts[2]}, true
	case len(parts) == 5 && parts[0] == "Form" && parts[1] == "Item" && parts[3] == "StandardCommand":
		return buttonCommand{kind: elementStandardCommand, element: parts[2], name: parts[4]}, true
	case len(parts) == 2 && parts[0] == "CommonCommand":
		return buttonCommand{kind: commonCommand, name: parts[1]}, true
	case len(parts) == 4 && parts[2] == "StandardCommand":
		if _, known := commandOwnerKinds[parts[0]]; known {
			return buttonCommand{kind: objectStandardCommand, owner: parts[0], object: parts[1], name: parts[3]}, true
		}
	case len(parts) == 4 && parts[2] == "Command" && parts[0] != "CommonForm":
		if _, known := commandOwnerKinds[parts[0]]; known {
			return buttonCommand{kind: objectCommand, owner: parts[0], object: parts[1], name: parts[3]}, true
		}
	}
	return buttonCommand{}, false
}

// FormCommandName is the name of the command of the form a button runs, if
// it runs one.
func (element ManagedFormElement) FormCommandName() (string, bool) {
	command, ok := parseButtonCommand(element.Command)
	if !ok || command.kind != formCommand {
		return "", false
	}
	return command.name, true
}

// FormCommandParameter is the parameter a button passes to its command: the
// types of a value (for creating by a parameter - 6 in the exports) or an
// object of the configuration (for showing in a list - 60), written as the
// prototype writes it, by kind and name ("DocumentJournal.Взаимодействия"),
// or by an identifier that resolves to nothing in the configuration (42).
type FormCommandParameter struct {
	Types  []Type `yaml:"types,omitempty" json:"types,omitempty"`
	Object string `yaml:"object,omitempty" json:"object,omitempty"`
}

func validateFormCommandParameter(path string, parameter *FormCommandParameter) []string {
	if parameter == nil {
		return nil
	}
	switch {
	case len(parameter.Types) != 0 && parameter.Object != "":
		return []string{path + " is either types or an object, not both"}
	case len(parameter.Types) != 0:
		return validateTypesIn(path+".types", parameter.Types, placeFormAttribute)
	case parameter.Object == "":
		return []string{path + " must be types or an object"}
	}
	if _, ok := parseCommandObject(parameter.Object); !ok && !isUUIDText(parameter.Object) {
		return []string{path + ".object must be <kind>.<name> of an object of the configuration or its identifier"}
	}
	return nil
}

// parseCommandObject reads an object of the configuration written by kind and
// name.
func parseCommandObject(value string) (buttonCommand, bool) {
	owner, object, found := strings.Cut(value, ".")
	if _, known := commandOwnerKinds[owner]; !found || !known || !validIdentifier(object) {
		return buttonCommand{}, false
	}
	return buttonCommand{owner: owner, object: object}, true
}

func isUUIDText(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && !id.IsZero()
}

// commandOwner finds the object of the configuration a button names by kind
// and name, with the names of its commands; a common form has none.
func (catalog *Catalog) commandOwner(owner, object string) (map[string]bool, bool) {
	field := commandOwnerKinds[owner]
	if field == "" {
		return nil, catalog.commonFormNames[strings.ToLower(object)]
	}
	list := reflect.ValueOf(catalog).Elem().FieldByName(field)
	for index := range list.Len() {
		definition := list.Index(index)
		if !strings.EqualFold(definition.FieldByName("Name").String(), object) {
			continue
		}
		names := map[string]bool{}
		if commands := definition.FieldByName("Commands"); commands.IsValid() {
			for _, command := range commands.Interface().([]ObjectCommand) {
				names[strings.ToLower(command.Name)] = true
			}
		}
		return names, true
	}
	return nil, false
}

// resolveButtonCommand checks a command of a button against the
// configuration: a common command and an object with its command must be
// there. What is not is a reference to nothing, as a common picture that is
// gone is; the code of an element is noted.
func (catalog *Catalog) resolveButtonCommand(where string, element ManagedFormElement) {
	catalog.resolveCommandText(where+" command", element.Command)
	if parameter := element.CommandParameter; parameter != nil && parameter.Object != "" {
		object, byName := parseCommandObject(parameter.Object)
		if _, ok := catalog.commandOwner(object.owner, object.object); !byName || !ok {
			catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where + " command parameter", Written: parameter.Object})
		}
	}
}

// resolveCommandText checks a command written as a button writes one -
// on a button or in the command interface of a form - against the
// configuration.
func (catalog *Catalog) resolveCommandText(where, text string) {
	command, _ := parseButtonCommand(text)
	switch command.kind {
	case elementCodeCommand:
		catalog.noteElementCode(where, text)
	case commonCommand:
		if !catalog.hasCommonCommandFolded(command.name) {
			catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, Written: text})
		}
	case objectCommand, objectStandardCommand:
		names, ok := catalog.commandOwner(command.owner, command.object)
		if !ok || command.kind == objectCommand && !names[strings.ToLower(command.name)] {
			catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, Written: text})
		}
	}
}

func (catalog *Catalog) hasCommonCommandFolded(name string) bool {
	for _, command := range catalog.CommonCommands {
		if strings.EqualFold(command.Name, name) {
			return true
		}
	}
	return false
}

// noteFormReferences notes every reference of an element the prototype wrote
// as a code or a number instead of a name, and a data path of two joined by
// "~": they are carried as written, and what they point at is not known.
func (catalog *Catalog) noteFormReferences(where string, element ManagedFormElement) {
	asWritten := func(value string) bool {
		trimmed := strings.TrimPrefix(value, "~")
		return value != "" && (formElementCode.MatchString(value) || strings.HasPrefix(value, "~") && strings.Contains(trimmed, "~"))
	}
	for _, reference := range []struct{ name, value string }{
		{"data_path", element.DataPath}, {"title_data_path", element.TitleDataPath},
		{"footer_data_path", element.FooterDataPath}, {"row_picture_data_path", element.RowPictureDataPath},
	} {
		if asWritten(reference.value) || formPathCode.MatchString(reference.value) {
			catalog.notePathCode(where+" "+reference.name, reference.value)
		}
	}
	if asWritten(element.CommandSource) {
		catalog.noteElementCode(where+" command_source", element.CommandSource)
	}
	for _, link := range element.ChoiceParameterLinks {
		if asWritten(link.DataPath) {
			catalog.notePathCode(where+" choice parameter link "+link.Name, link.DataPath)
		}
	}
	if link := element.TypeLink; link != nil && asWritten(link.DataPath) {
		catalog.notePathCode(where+" type link", link.DataPath)
	}
}

// notePathCode carries a data path of an element, or the path of a link of a
// field, the prototype wrote as a code. A code of several segments is what
// the configurator leaves when the path no longer leads anywhere, as a single
// segment is for an element: a live path is written by its names
// ("Items.Товары.CurrentData.Номенклатура", "Объект.Партнер"). In the exports
// every one of the 36 paths of a link of several segments leads to nothing -
// a table that was deleted (5), a column the table part of a table does not
// have (11: an attribute deleted, or one no object has), a field the object
// of an attribute of the form does not have (20: the form copied from
// another object with its links). So does every one of the 139 data paths,
// all of lombard1: a field the object does not have (98), a column the table
// part does not have (36, 15 of them of a footer), a standard attribute the
// catalog does not have (2: a code of length 0, an owner of none), a column
// a value table does not have (2), a column of another table part than the
// table shows (2), a field of a catalog through an attribute of a composite
// type (2) - the prototype names no path through an attribute of a composite
// type, 111 of 111 through an attribute of one. It is a remnant of what was
// deleted (2.203), as the same path of a link of an attribute of an object is
// (FieldPath.Unresolved). The path starts at an attribute of the form, by its
// number, or at an element, by its code; one that starts with another
// identifier is not in the exports and stays a note.
func (catalog *Catalog) notePathCode(where, written string) {
	if first, _, compound := strings.Cut(written, "/"); compound && formPathCode.MatchString(written) &&
		(allDigits(first) || deletedFormElement.MatchString(first)) {
		catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, Written: written})
		return
	}
	catalog.noteElementCode(where, written)
}

// noteElementCode carries a reference of a form the prototype wrote as a
// code: the code of a deleted element is a remnant of what was deleted
// (deletedFormElement), any other is noted as written.
func (catalog *Catalog) noteElementCode(where, written string) {
	if deletedFormElement.MatchString(written) {
		catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, Written: written})
		return
	}
	catalog.noteForm(NoteFormReferenceAsWritten, where, written)
}

// validateFormCommandProperties checks what a command of a form has besides
// its name, title and action.
func validateFormCommandProperties(path string, command ManagedFormCommand, configuration project.Project) []string {
	var issues []string
	issues = append(issues, validateTitle(path+".tool_tip", command.ToolTip, configuration)...)
	issues = append(issues, validatePictureReference(path+".picture", command.Picture)...)
	issues = append(issues, oneOf(path+".representation", command.Representation, CommandAuto, CommandText, CommandPicture, CommandPictureAndText)...)
	if strings.TrimSpace(command.Shortcut) != command.Shortcut {
		issues = append(issues, path+".shortcut must be written without surrounding spaces")
	}
	issues = append(issues, oneOf(path+".current_row_use", command.CurrentRowUse, FormUseAuto, FormUseYes, FormUseDontUse)...)
	if table := command.AssociatedTable; table != "" && !formElementCode.MatchString(table) &&
		(!validIdentifier(table) || utf8.RuneCountInString(table) > maxNameLength) {
		issues = append(issues, path+".associated_table must be the name or the code of a table of the form")
	}
	issues = append(issues, validateFormOptions(path+".functional_options", command.FunctionalOptions)...)
	issues = append(issues, validateFormRight(path+".use", command.Use)...)
	return issues
}

// resolveFormCommands resolves what the commands of one form name in the
// project: the functional options that switch them off, the roles they are
// available to, the common picture they are drawn with; and notes the table
// a command names by the code of an element, or by an element that is no
// table.
func (catalog *Catalog) resolveFormCommands(form string, value ManagedForm) error {
	kinds := map[string]FormElementKind{}
	var walk func(items []ManagedFormElement)
	walk = func(items []ManagedFormElement) {
		for _, item := range items {
			if _, seen := kinds[foldedName(item.Name)]; !seen {
				kinds[foldedName(item.Name)] = item.Kind
			}
			walk(item.Nested())
		}
	}
	walk(value.FormItems())
	for _, command := range value.Commands {
		where := form + " command " + command.Name
		if err := catalog.resolveFormData(where, nil, command.FunctionalOptions, command.Use); err != nil {
			return err
		}
		if picture := command.Picture; picture != nil && picture.Common != nil {
			if _, ok := catalog.commonPictureByID[*picture.Common]; !ok {
				catalog.noteUnresolved(where+" picture", *picture.Common)
			}
		}
		switch table := command.AssociatedTable; {
		case formElementCode.MatchString(table):
			catalog.noteElementCode(where+" associated_table", table)
		case table != "" && kinds[foldedName(table)] != FormElementTable:
			kind := string(kinds[foldedName(table)])
			if kind == "" {
				kind = "context menu or command bar"
			}
			catalog.noteForm(NoteAssociatedTableNotTable, where+" associated_table", table+" ("+kind+")")
		}
	}
	return nil
}
