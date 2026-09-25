package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	commonCommandID    = "d8000000-0000-4000-8000-000000000001"
	commandGroupID     = "d8000000-0000-4000-8000-000000000002"
	commonCommandOther = "d8000000-0000-4000-8000-000000000003"
)

// writeCommonCommand writes one common command into the folder named after it,
// with the module that runs it beside its description.
func writeCommonCommand(t *testing.T, root, name, body string, withModule bool) {
	t.Helper()
	directory := filepath.Join(root, "metadata", string(CommonCommandKind), name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, project.ObjectMetadataFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if !withModule {
		return
	}
	source := "&НаКлиенте\nПроцедура ОбработкаКоманды(ПараметрКоманды, ПараметрыВыполненияКоманды)\nКонецПроцедуры\n"
	if err := os.WriteFile(filepath.Join(directory, project.CommandModuleFile), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A command that belongs to no object is described by exactly what a command
// of an object is described by, plus where its help goes. None of it was in
// the model, so a configuration's own commands were lost whole at import.
func TestCommonCommandKeepsEveryPropertyItWasGiven(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommandGroup(t, root, commandGroupID, "Сервис")
	writeCommonCommand(t, root, "ОткрытьВнешнийОтчет", `format: 1
id: `+commonCommandID+`
name: ОткрытьВнешнийОтчет
title: {ru: Открыть внешний отчёт}
comment: Для целей тестирования
tooltip: {ru: Открывает внешний отчёт или обработку}
group_ref: `+commandGroupID+`
parameter: [{kind: string, length: 100}]
parameter_use: single
modifies_data: true
picture: {standard: Открыть}
representation: picture-and-text
shortcut: Ctrl+Shift+O
on_server_unavailable: not-available
include_help_in_contents: true
`, true)

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	command, ok := catalog.CommonCommand("ОткрытьВнешнийОтчет")
	if !ok {
		t.Fatal("the common command did not load")
	}
	switch {
	case command.Comment == "" || len(command.Tooltip) == 0:
		t.Fatalf("the comment or the tooltip was lost: %+v", command)
	case command.GroupRef == nil || command.GroupRef.String() != commandGroupID:
		t.Fatalf("where the command is shown was lost: %+v", command)
	case len(command.Parameter) != 1 || command.ParameterUse != CommandParameterSingle:
		t.Fatalf("the parameter was lost: %+v", command)
	case !command.ModifiesData:
		t.Fatalf("a command that changes data was carried as one that does not: %+v", command)
	case command.Picture == nil || command.Representation != CommandPictureAndText:
		t.Fatalf("how the command is drawn was lost: %+v", command)
	case command.Shortcut != "Ctrl+Shift+O":
		t.Fatalf("the shortcut was lost: %+v", command)
	case command.OnServerUnavailable != ServerUnavailableNotAvailable:
		t.Fatalf("what the command does without the main server was lost: %+v", command)
	case !command.IncludeHelpInContents:
		t.Fatalf("where the help goes was lost: %+v", command)
	}

	// The catalog hands out copies here too.
	command.Name = "Подменено"
	again, _ := catalog.CommonCommand("ОткрытьВнешнийОтчет")
	if again.Name != "ОткрытьВнешнийОтчет" {
		t.Fatal("a common command was handed out by reference")
	}
}

// A command group is a place, and everything it has is what makes the place
// recognisable: what it is called, what it shows, and which part of the
// interface it stands in.
func TestCommandGroupKeepsWhatMakesItAPlace(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CommandGroupKind, commandGroupID, `format: 1
id: `+commandGroupID+`
name: Синхронизация
title: {ru: Синхронизация}
comment: Всё про обмен данными
tooltip: {ru: Настроить синхронизацию данных}
category: form-command-bar
picture: {standard: Обмен}
representation: picture-and-text
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	group, ok := catalog.CommandGroup("Синхронизация")
	if !ok {
		t.Fatal("the command group did not load")
	}
	switch {
	case group.Comment == "" || len(group.Tooltip) == 0:
		t.Fatalf("the comment or the tooltip was lost: %+v", group)
	case group.Category != FormCommandBarCategory:
		t.Fatalf("which part of the interface the group stands in was lost: %+v", group)
	case group.Picture == nil || group.Representation != CommandPictureAndText:
		t.Fatalf("how the group is drawn was lost: %+v", group)
	}
	group.Title["ru"] = "Подменено"
	again, _ := catalog.CommandGroup("Синхронизация")
	if again.Title["ru"] != "Синхронизация" {
		t.Fatal("a command group was handed out by reference")
	}
}

// A command placed in a group of the configuration has to name a group that
// exists. Until groups were objects there was nothing to check this against,
// and a command placed nowhere is simply drawn nowhere - with nobody told.
func TestCommandPlacedInAGroupThatDoesNotExistIsRefused(t *testing.T) {
	t.Parallel()
	for name, write := range map[string]func(t *testing.T, root string){
		"общая команда": func(t *testing.T, root string) {
			writeCommonCommand(t, root, "Открыть", `format: 1
id: `+commonCommandID+`
name: Открыть
title: {ru: Открыть}
group_ref: `+commandGroupID+`
`, true)
		},
		"команда объекта": func(t *testing.T, root string) {
			writeMetadata(t, root, CatalogKind, commonCommandOther, `format: 1
id: `+commonCommandOther+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
commands:
  - {id: `+commonCommandID+`, name: Открыть, title: {ru: Открыть}, group_ref: `+commandGroupID+`}
`)
			writeCommandModule(t, root, CatalogKind, "Контрагенты", "Открыть")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			write(t, root)
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), "unknown command group") {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// A common command keeps a folder, because it keeps the module that runs it.
func TestCommonCommandFolderHoldsItsDescriptionAndItsModule(t *testing.T) {
	t.Parallel()
	const body = `format: 1
id: ` + commonCommandID + `
name: Открыть
title: {ru: Открыть}
`
	t.Run("без модуля", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeCommonCommand(t, root, "Открыть", body, false)
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "has no module") {
			t.Fatalf("a common command with no module: %v", err)
		}
	})
	t.Run("лишний файл", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeCommonCommand(t, root, "Открыть", body, true)
		stray := filepath.Join(root, "metadata", string(CommonCommandKind), "Открыть", "МодульФормы.bsl")
		if err := os.WriteFile(stray, []byte("// стороннее\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "it keeps only object.yaml") {
			t.Fatalf("a common command keeping something else: %v", err)
		}
	})
	t.Run("имя расходится с папкой", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeCommonCommand(t, root, "ДругоеИмя", body, true)
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "lies in a folder called") {
			t.Fatalf("a common command calling itself otherwise: %v", err)
		}
	})
}

// What a command group is checked for is what would make it unplaceable.
func TestBrokenCommandGroupsAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"категории не существует": {"category: где-нибудь", "category must be actions-panel or form-command-bar"},
		"категория не названа":    {"", "category must be actions-panel or form-command-bar"},
		"отображение картинкой без картинки": {"category: actions-panel\nrepresentation: picture",
			"representation picture needs a picture"},
		"картинка из двух источников": {"category: actions-panel\npicture: {standard: Обмен, common: " + commonCommandID + "}",
			"names both a standard picture and a common picture"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCommandGroup("object.yaml", strings.NewReader(`format: 1
id: `+commandGroupID+`
name: Синхронизация
title: {ru: Синхронизация}
`+broken.body+`
`), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}
