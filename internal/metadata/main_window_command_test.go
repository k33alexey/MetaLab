package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A command placed in a panel of the main window - a standard group of it or a
// group of the configuration of its two categories - takes no parameter; in a
// form and with no group it may (opis of the common command, 14.10.2026). The
// mode and «Изменяет данные» are carried there all the same: the prototype
// keeps them though they are unavailable. The groups are written here, not
// read from the model.
//
// Defect caught: a command of the main window given a parameter, which the
// designer cannot do and nothing would ever pass; a command of a form refused
// its parameter; the mode or the flag the exports keep refused.
func TestACommandOfTheMainWindowTakesNoParameter(t *testing.T) {
	t.Parallel()
	configuration := metadataConfiguration()
	parameter := []Type{{Kind: StringType, Length: 10}}
	for group, refused := range map[string]bool{
		"navigation-panel-important": true, "navigation-panel-ordinary": true, "navigation-panel-see-also": true,
		"actions-panel-create": true, "actions-panel-reports": true, "actions-panel-tools": true,
		"form-navigation-panel-important": false, "form-navigation-panel-see-also": false, "form-navigation-panel-go-to": false,
		"form-command-bar-important": false, "form-command-bar-create-based-on": false, "": false,
	} {
		command := ObjectCommand{ID: uuid.MustNew(), Name: "Команда", Title: LocalizedText{"ru": "Команда"}, Group: group, Parameter: parameter, ParameterUse: CommandParameterSingle}
		issues := validateCommandShape("command", command, configuration)
		if refused != (len(issues) == 1 && strings.Contains(issues[0], "command.parameter is not allowed: the command stands in a panel of the main window")) {
			t.Errorf("a parameter in %q: %v", group, issues)
		}
		command.Parameter, command.ModifiesData = nil, true
		if issues := validateCommandShape("command", command, configuration); len(issues) != 0 {
			t.Errorf("no parameter, a mode and «Изменяет данные» in %q: %v", group, issues)
		}
	}
	for category, refused := range map[string]bool{
		"actions-panel": true, "navigation-panel": true, "form-command-bar": false, "form-navigation-panel": false,
	} {
		root := metadataProject(t)
		group := uuid.MustNew().String()
		writeMetadata(t, root, CommandGroupKind, group, "format: 1\nid: "+group+"\nname: Группа\ntitle: {ru: Группа}\ncategory: "+category+"\n")
		writeCommonCommand(t, root, "Команда", "format: 1\nid: "+uuid.MustNew().String()+"\nname: Команда\ntitle: {ru: Команда}\n"+
			"group_ref: "+group+"\nparameter: [{kind: string, length: 10}]\nparameter_use: single\n", true)
		_, err := Load(root)
		switch {
		case refused && (err == nil || !strings.Contains(err.Error(), "common command Команда.parameter is not allowed: the command stands in a panel of the main window ("+category+")") || !strings.Contains(err.Error(), "of group Группа")):
			t.Errorf("a parameter in a group of %s: %v", category, err)
		case !refused && err != nil:
			t.Errorf("a parameter in a group of %s refused: %v", category, err)
		}
	}
}
