package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	moduleCatalog  = "d4000000-0000-4000-8000-000000000001"
	moduleRegister = "d4000000-0000-4000-8000-000000000002"
	moduleJournal  = "d4000000-0000-4000-8000-000000000003"
	moduleDocument = "d4000000-0000-4000-8000-000000000004"
	moduleOwner    = "d4000000-0000-4000-8000-000000000005"
	moduleRate     = "d4000000-0000-4000-8000-000000000006"
	moduleCommand  = "d4000000-0000-4000-8000-000000000007"
)

// A module is no longer declared anywhere. It lies in the folder of what owns
// it, under the name of the role it plays, and that is the whole of what says
// which module it is - so a configuration that names no module at all still
// gets its modules, and they are compiled under the names the runtime expects.
func TestModuleIsTheFileAndNothingDeclaresIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, moduleCatalog, "Товары")
	writeObjectModule(t, root, CatalogKind, "Товары", project.ObjectModuleFile)
	writeObjectModule(t, root, CatalogKind, "Товары", project.ManagerModuleFile)
	writeMetadata(t, root, InformationRegisterKind, moduleRegister, `format: 1
id: `+moduleRegister+`
name: Курсы
title: {ru: Курсы}
write_mode: independent
periodicity: day
dimensions: [{id: `+moduleOwner+`, name: Валюта, title: {ru: Валюта}, types: [{kind: string, length: 3}]}]
resources: [{id: `+moduleJournal+`, name: Курс, title: {ru: Курс}, types: [{kind: number, precision: 15, scale: 4}]}]
`)
	writeObjectModule(t, root, InformationRegisterKind, "Курсы", project.RecordSetModuleFile)

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, module := range modules {
		names[module.Filename] = module.Name
	}
	for path, want := range map[string]string{
		"metadata/catalogs/Товары/" + project.ObjectModuleFile:                "МодульОбъектаСправочника.Товары",
		"metadata/catalogs/Товары/" + project.ManagerModuleFile:               "МодульМенеджераСправочника.Товары",
		"metadata/information-registers/Курсы/" + project.RecordSetModuleFile: "МодульНабораЗаписейРегистраСведений.Курсы",
	} {
		if names[path] != want {
			t.Fatalf("%s was compiled as %q, not as %q", path, names[path], want)
		}
	}
}

// Which modules an object may keep is decided by its kind: a catalog keeps an
// object, a register keeps a record set, and a document journal - which nobody
// writes to, because the platform fills it - keeps neither. A module in a role
// the kind does not have would never be called, and nothing would say why.
func TestModuleInARoleTheKindDoesNotHaveIsRefused(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		kind       Kind
		object     string
		role, want string
	}{
		"набор записей у справочника": {CatalogKind, "Товары", project.RecordSetModuleFile,
			"has no module in that role"},
		"модуль объекта у регистра сведений": {InformationRegisterKind, "Курсы", project.ObjectModuleFile,
			"has no module in that role"},
		"модуль объекта у журнала документов": {DocumentJournalKind, "Общий", project.ObjectModuleFile,
			"has no module in that role"},
		"модуль набора записей у журнала документов": {DocumentJournalKind, "Общий", project.RecordSetModuleFile,
			"has no module in that role"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := moduleProject(t)
			writeObjectModule(t, root, test.kind, test.object, test.role)
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The other side of the same rule: every role a kind does have is accepted
// where it lies. A document journal is the case worth naming - nobody writes
// to one, so it has no object and no record set, but the platform still calls
// its manager, and the prototype's own export keeps that module.
func TestEachKindKeepsTheModulesItHas(t *testing.T) {
	t.Parallel()
	root := moduleProject(t)
	writeObjectModule(t, root, CatalogKind, "Товары", project.ObjectModuleFile)
	writeObjectModule(t, root, CatalogKind, "Товары", project.ManagerModuleFile)
	writeObjectModule(t, root, InformationRegisterKind, "Курсы", project.RecordSetModuleFile)
	writeObjectModule(t, root, InformationRegisterKind, "Курсы", project.ManagerModuleFile)
	writeObjectModule(t, root, DocumentJournalKind, "Общий", project.ManagerModuleFile)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}

// The folder of an object holds its description, its modules and the folders
// of its forms, commands and templates - and nothing else. A file nobody can
// name is either a module misspelled, which would silently never run, or
// something that does not belong in the configuration at all.
func TestObjectFolderHoldsOnlyWhatAnObjectKeeps(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		write func(t *testing.T, root string)
		want  string
	}{
		"модуль с опечаткой в имени": {func(t *testing.T, root string) {
			writeObjectModule(t, root, CatalogKind, "Товары", "МодульОбьекта.bsl")
		}, "neither its description nor a module of its own"},
		"посторонний файл": {func(t *testing.T, root string) {
			writeObjectModule(t, root, CatalogKind, "Товары", "заметки.txt")
		}, "neither its description nor a module of its own"},
		"посторонняя папка": {func(t *testing.T, root string) {
			directory := filepath.Join(root, "metadata", string(CatalogKind), "Товары", "прочее")
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
		}, "which is not forms, commands or templates"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := moduleProject(t)
			test.write(t, root)
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// A command keeps a folder of its own, and that folder keeps its module and
// nothing else. Anything else there is a file that will never be run.
func TestCommandFolderHoldsOnlyItsModule(t *testing.T) {
	t.Parallel()
	root := moduleProject(t)
	writeMetadata(t, root, CatalogKind, moduleCatalog, `format: 1
id: `+moduleCatalog+`
name: Товары
title: {ru: Товары}
code: {type: string, length: 9, auto: true}
description_length: 150
commands:
  - {id: `+moduleCommand+`, name: Пересчитать, title: {ru: Пересчитать}}
`)
	writeCommandModule(t, root, CatalogKind, "Товары", "Пересчитать")
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(root, "metadata", string(CatalogKind), "Товары", "commands", "Пересчитать", "МодульФормы.bsl")
	if err := os.WriteFile(stray, []byte("// модуль\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("a file that is not the command's module was accepted")
	}
	if !strings.Contains(err.Error(), "a command keeps only its module") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// moduleProject writes one object of each kind the tests above put a module
// into: a catalog, an information register and a document journal.
func moduleProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, moduleCatalog, "Товары")
	writeMetadata(t, root, InformationRegisterKind, moduleRegister, `format: 1
id: `+moduleRegister+`
name: Курсы
title: {ru: Курсы}
write_mode: independent
periodicity: day
dimensions: [{id: `+moduleOwner+`, name: Валюта, title: {ru: Валюта}, types: [{kind: string, length: 3}]}]
resources: [{id: `+moduleRate+`, name: Курс, title: {ru: Курс}, types: [{kind: number, precision: 15, scale: 4}]}]
`)
	writeMetadata(t, root, DocumentKind, moduleDocument, `format: 1
id: `+moduleDocument+`
name: Приход
title: {ru: Приход}
number: {type: string, length: 11, auto: true, unique: true, periodicity: year}
`)
	writeMetadata(t, root, DocumentJournalKind, moduleJournal, `format: 1
id: `+moduleJournal+`
name: Общий
title: {ru: Общий}
documents: [`+moduleDocument+`]
`)
	return root
}
