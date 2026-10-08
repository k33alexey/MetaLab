package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
)

// A command of a form keeps what it has besides its name, title and action
// through YAML and the Studio: its tooltip, picture and how its buttons are
// drawn, its shortcut, whether it modifies the saved data, the current row
// it needs and of which table, the options that switch it off and whom it is
// available to; and a command with no action is kept.
//
// Defect caught: the tooltip (33305 commands), the picture (14750), the
// representation (11433) or the shortcut (2295) of a command lost, so that
// its buttons are drawn and reached differently; a command that modifies the
// saved data (4970) read as one that does not, so that closing the form
// loses the change without asking; a command switched off by a functional
// option (2002) or limited to roles (512) shown to everyone; the 285
// commands of the exports with no action refused.
func TestAFormCommandKeepsWhatItHas(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	source := formAttrHead + "commands:\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990001, name: Удалить, title: {ru: Удалить}, action: custom, handler: Удалить," +
		" tool_tip: {ru: Удалить строку}, picture: {standard: Delete, load_transparent: true}, representation: picture-and-text, shortcut: Shift+F9," +
		" modifies_saved_data: true, current_row_use: use, associated_table: Список, functional_options: [c0de0000-0000-4000-8000-000000990002]," +
		" use: {common: false, roles: [{role: c0de0000-0000-4000-8000-000000990003, value: true}]}}\n" +
		"  - {id: c0de0000-0000-4000-8000-000000990004, name: Пусто, title: {ru: Пусто}, action: custom}\n" +
		"items:\n  - {id: c0de0000-0000-4000-8000-000000990005, name: Список, kind: table}\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, commands []ManagedFormCommand) {
		t.Helper()
		command := commands[0]
		switch {
		case command.ToolTip["ru"] != "Удалить строку" || command.Picture == nil || command.Picture.Standard != "Delete" ||
			command.Representation != CommandPictureAndText || command.Shortcut != "Shift+F9":
			t.Fatalf("%s: how it is drawn: %+v", source, command)
		case !command.ModifiesSavedData || command.CurrentRowUse != FormUseYes || command.AssociatedTable != "Список":
			t.Fatalf("%s: what it needs: %+v", source, command)
		case len(command.FunctionalOptions) != 1 || command.Use == nil || command.Use.Common || len(command.Use.Roles) != 1:
			t.Fatalf("%s: whom it is for: %+v", source, command)
		case commands[1].Handler != "" || commands[1].Action != FormCommandCustom:
			t.Fatalf("%s: a command with no action: %+v", source, commands[1])
		}
	}
	check("read", form.Commands)
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatal(err)
	}
	check("written back", again.Commands)
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatal(err)
	}
	check("carried through the Studio", received.Commands)
	if !reflect.DeepEqual(received.Commands, form.Commands) || !reflect.DeepEqual(again.Commands, form.Commands) {
		t.Fatal("the commands changed on the way")
	}
}

// What is wrong in a command of a form is refused, naming it.
//
// Defect caught: the prototype's spelling of the representation (TextPicture)
// or of the use of the current row taken as written; a table of the current
// row naming nothing in the form; a shortcut with spaces around; a
// functional option or a role with no identifier; a picture naming two; a
// handler that is no name of a procedure; a tooltip in no language.
func TestAFormCommandRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	command := func(rest string) string {
		return formAttrHead + "commands:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Команда, title: {ru: Команда}, action: custom" + rest + "}\n"
	}
	for name, test := range map[string]struct{ source, want string }{
		"отображение прототипа": {command(", representation: TextPicture"), "commands[0].representation must be auto, text, picture or picture-and-text"},
		"строка прототипа":      {command(", current_row_use: DontUse"), "commands[0].current_row_use must be auto, use or dont-use"},
		"таблицы нет":           {command(", associated_table: Список"), "commands[0].associated_table names no element of the form"},
		"таблица с пробелом":    {command(", associated_table: Список товаров"), "commands[0].associated_table must be the name or the code of a table of the form"},
		"сочетание":             {command(", shortcut: ' F5'"), "commands[0].shortcut must be written without surrounding spaces"},
		"опция":                 {command(", functional_options: [00000000-0000-0000-0000-000000000000]"), "commands[0].functional_options[0] must be a non-zero UUID"},
		"роль":                  {command(", use: {common: true, roles: [{role: 00000000-0000-0000-0000-000000000000, value: false}]}"), "commands[0].use.roles[0].role must be a non-zero UUID"},
		"две картинки":          {command(", picture: {standard: Delete, file: Picture.png}"), "commands[0].picture names more than one"},
		"обработчик":            {command(", handler: Удалить строку"), "commands[0].handler must be a valid BSL routine name"},
		"подсказка не на языке": {command(", tool_tip: {\"d=e\": Подсказка}"), "commands[0].tool_tip"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(test.source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// What a command of a form names in the project is resolved at load, and
// the table of its current row noted when it is written as a code or names
// no table.
//
// Defect caught: a command switched off by a functional option or limited
// to a role the project no longer has, or drawn with a common picture that
// is gone, loading clean; the table of the current row naming an input
// field (5 commands of the exports) or written as a code carried silently;
// a note on a command whose table is a table.
func TestWhatAFormCommandNamesIsResolved(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		command string
		gone    string
		note    NoteKind
		written string
	}{
		"всё на месте":   {command: "functional_options: [" + cmpOption + "], use: {common: false, roles: [{role: " + cmpRole + ", value: true}]}, picture: {common: " + cmpCommonPicture + "}, associated_table: Список"},
		"опции нет":      {command: "functional_options: [" + refGone + "]", gone: "command Удалить functional option"},
		"роли нет":       {command: "use: {common: true, roles: [{role: " + refGone + ", value: false}]}", gone: "command Удалить right of role"},
		"картинки нет":   {command: "picture: {common: " + refGone + "}", gone: "command Удалить picture"},
		"таблица — поле": {command: "associated_table: Код", note: NoteAssociatedTableNotTable, written: "Код (input-field)"},
		"таблица — код":  {command: "associated_table: \"3:409b9a53-7f7e-4178-86c1-33176c7c7a7a\"", note: NoteFormReferenceAsWritten, written: "3:409b9a53-7f7e-4178-86c1-33176c7c7a7a"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			added := "commands:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Удалить, title: {ru: Удалить}, action: custom, handler: Удалить, " + test.command + "}\n" +
				"items:\n  - {id: c0de0000-0000-4000-8000-000000990002, name: Список, kind: table}\n  - {id: c0de0000-0000-4000-8000-000000990003, name: Код, kind: input-field}\n"
			writeFile(t, path, strings.Replace(string(content), "attributes:\n", added+"attributes:\n", 1))
			where := "catalog Номенклатура form ФормаЭлемента "
			if test.gone != "" {
				if found := unresolvedOf(t, root); !containsWhere(found, where+test.gone) {
					t.Fatalf("unresolved = %+v", found)
				}
				return
			}
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			var found []Note
			for _, note := range catalog.Notes() {
				if strings.HasPrefix(note.Where, where+"command") {
					found = append(found, note)
				}
			}
			if test.note == "" && len(found) != 0 || test.note != "" && (len(found) != 1 || found[0].Kind != test.note || found[0].Written != test.written) {
				t.Fatalf("notes = %+v, want %s %q", found, test.note, test.written)
			}
		})
	}
}
