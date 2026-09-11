package mlapp

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
)

// FormFromMetadata converts a validated runtime descriptor into the stable
// browser component contract.
func FormFromMetadata(descriptor metadata.FormDescriptor, custom *metadata.ManagedForm, language string) (Form, error) {
	if descriptor.ObjectID.IsZero() || descriptor.ObjectName == "" || descriptor.Title == "" {
		return Form{}, fmt.Errorf("invalid application form descriptor")
	}
	if descriptor.Generated {
		if custom != nil {
			return Form{}, fmt.Errorf("generated form must not include a custom source")
		}
		return generatedForm(descriptor), nil
	}
	if custom == nil || descriptor.SourceID == nil || custom.ID != *descriptor.SourceID || custom.Kind != descriptor.Kind {
		return Form{}, fmt.Errorf("custom form does not match its descriptor")
	}
	return customForm(descriptor, *custom, language), nil
}

func generatedForm(descriptor metadata.FormDescriptor) Form {
	form := Form{ID: formID(descriptor), Title: descriptor.Title, Commands: generatedCommands(descriptor.Commands)}
	fields := make([]Element, 0, len(descriptor.Fields))
	for _, field := range descriptor.Fields {
		fields = append(fields, formField(field))
	}
	if descriptor.Kind == metadata.ObjectForm {
		form.Items = append(form.Items, Element{ID: "fields", Kind: "group", Orientation: "vertical", Children: fields})
		for _, part := range descriptor.TableParts {
			columns := make([]Element, 0, len(part.Columns))
			for _, column := range part.Columns {
				columns = append(columns, formField(column))
			}
			form.Items = append(form.Items, Element{ID: "table-" + part.Name, Kind: "table", Title: part.Title, Children: columns})
		}
	} else {
		form.Items = []Element{{ID: "list", Kind: "table", Title: descriptor.Title, Children: fields}}
	}
	return form
}

func generatedCommands(commands []metadata.FormCommand) []Command {
	result := make([]Command, len(commands))
	for index, command := range commands {
		kind := "secondary"
		if index == 0 || command.Name == "Post" {
			kind = "primary"
		}
		result[index] = Command{ID: command.Name, Title: command.Title, Kind: kind}
	}
	return result
}

func formField(field metadata.FormField) Element {
	return Element{
		ID: "field-" + field.Name, Kind: "field", Title: field.Title,
		InputType: inputType(field.Types), DataPath: field.Name, ReadOnly: field.ReadOnly,
	}
}

func inputType(types []metadata.Type) string {
	if len(types) != 1 {
		return "text"
	}
	switch types[0].Kind {
	case metadata.NumberType:
		return "number"
	case metadata.DateType:
		return "datetime-local"
	case metadata.BooleanType:
		return "checkbox"
	default:
		return "text"
	}
}

func customForm(descriptor metadata.FormDescriptor, source metadata.ManagedForm, language string) Form {
	title := source.Title.Resolve(language, nil)
	if title == "" {
		title = descriptor.Title
	}
	commands := make([]Command, len(source.Commands))
	commandNames := make(map[string]string, len(source.Commands))
	for index, command := range source.Commands {
		commandTitle := command.Title.Resolve(language, nil)
		if commandTitle == "" {
			commandTitle = command.Name
		}
		commands[index] = Command{ID: command.Name, Title: commandTitle, Kind: commandKind(string(command.Action))}
		commandNames[command.ID.String()] = command.Name
	}
	return Form{ID: formID(descriptor), Title: title, Commands: commands, Items: customElements(source.Items, language, commandNames)}
}

func customElements(source []metadata.ManagedFormElement, language string, commands map[string]string) []Element {
	result := make([]Element, 0, len(source))
	for _, item := range source {
		if item.Hidden {
			continue
		}
		title := item.Title.Resolve(language, nil)
		if title == "" {
			title = item.Name
		}
		value := Element{
			ID: item.ID.String(), Kind: string(item.Kind), Title: title,
			ReadOnly: item.ReadOnly, Disabled: item.Disabled, Orientation: string(item.Orientation),
			DataPath: item.DataPath, Children: customElements(item.Children, language, commands),
		}
		if item.Command != nil {
			value.Command = commands[item.Command.String()]
		}
		result = append(result, value)
	}
	return result
}

func commandKind(action string) string {
	if action == string(metadata.FormCommandSave) || action == string(metadata.FormCommandChoose) || action == string(metadata.FormCommandPost) {
		return "primary"
	}
	return "secondary"
}

func formID(descriptor metadata.FormDescriptor) string {
	return strings.Join([]string{string(descriptor.ObjectKind), descriptor.ObjectID.String(), string(descriptor.Kind)}, ":")
}
