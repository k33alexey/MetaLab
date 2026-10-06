package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The checks of a form, each on the case that tells it apart: a name of 255
// characters is taken and one of 256 is not (the designer takes 255), wherever
// a name stands; a custom command keeps its handler; a data path belongs to a
// field and is not limited in length or depth; a group's first child is
// checked like the rest; a label is not read-only.
func TestManagedFormChecksEachCaseThatTellsItApart(t *testing.T) {
	t.Parallel()
	longest, tooLong := strings.Repeat("Я", 255), strings.Repeat("Я", 256)
	form := func(commands []ManagedFormCommand, items ...ManagedFormElement) ManagedForm {
		return ManagedForm{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Форма", Title: LocalizedText{"ru": "Форма"}, Kind: ObjectForm,
			Commands: commands, Items: items}
	}
	command := func(name, handler string) ManagedFormCommand {
		return ManagedFormCommand{ID: uuid.MustNew(), Name: name, Title: LocalizedText{"ru": "Команда"}, Action: FormCommandCustom, Handler: handler}
	}
	element := func(kind FormElementKind, name string) ManagedFormElement {
		item := ManagedFormElement{ID: uuid.MustNew(), Name: name, Kind: kind}
		if kind == FormElementUsualGroup {
			item.Orientation = FormVertical
		}
		return item
	}
	field := func(name, path string) ManagedFormElement {
		item := element(FormElementInputField, name)
		item.DataPath = path
		return item
	}
	deep := strings.TrimSuffix(strings.Repeat("Реквизит.", 40), ".")
	label := element(FormElementLabelDecoration, "Надпись")
	label.DataPath = "Объект.Наименование"
	readOnlyLabel := element(FormElementLabelDecoration, "Надпись")
	readOnlyLabel.ReadOnly = true
	group := element(FormElementUsualGroup, "Группа")
	group.Children = []ManagedFormElement{element("неизвестный", "Первый"), element(FormElementInputField, "Второй")}

	tests := []struct {
		name string
		form ManagedForm
		want string // empty: the form is taken
	}{
		{"a command named by 255 characters", form([]ManagedFormCommand{command(longest, "Выполнить")}), ""},
		{"a command named by 256 characters", form([]ManagedFormCommand{command(tooLong, "Выполнить")}), "commands[0].name must be a valid identifier"},
		{"a handler of 255 characters", form([]ManagedFormCommand{command("Выполнить", longest)}), ""},
		{"a handler of 256 characters", form([]ManagedFormCommand{command("Выполнить", tooLong)}), "commands[0].handler must be a valid BSL routine name"},
		{"an element named by 255 characters", form(nil, element(FormElementLabelDecoration, longest)), ""},
		{"an element named by 256 characters", form(nil, element(FormElementLabelDecoration, tooLong)), "items[0].name must be a valid identifier"},
		{"a path segment of 255 characters", form(nil, field("Поле", "Объект."+longest)), ""},
		{"a path segment of 256 characters", form(nil, field("Поле", "Объект."+tooLong)), "items[0].data_path must contain only valid identifier segments"},
		{"a path forty segments deep", form(nil, field("Поле", deep)), ""},
		{"a data path on a label", form(nil, label), "items[0].data_path is allowed only for fields and tables"},
		{"a read-only label", form(nil, readOnlyLabel), "items[0].read_only is allowed only for fields and tables"},
		{"a group whose first child is wrong", form(nil, group), "items[0].children[0].kind неизвестный is not a kind of element of a form"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateManagedForm("form.yaml", test.form, managedFormConfiguration())
			switch {
			case test.want == "" && err != nil:
				t.Fatalf("the form was refused: %v", err)
			case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
				t.Fatalf("error = %v, want it to say %q", err, test.want)
			}
		})
	}
}
