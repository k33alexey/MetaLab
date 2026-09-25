package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	commandCatalog       = "cd000000-0000-4000-8000-000000000001"
	commandDocument      = "cd000000-0000-4000-8000-000000000002"
	commandJournal       = "cd000000-0000-4000-8000-000000000003"
	commandInfoRegister  = "cd000000-0000-4000-8000-000000000004"
	commandAccumRegister = "cd000000-0000-4000-8000-000000000005"
	commandCharTypes     = "cd000000-0000-4000-8000-000000000006"
	commandAccounts      = "cd000000-0000-4000-8000-000000000007"
	commandCalcTypes     = "cd000000-0000-4000-8000-000000000008"
	commandProcess       = "cd000000-0000-4000-8000-000000000009"
	commandTask          = "cd000000-0000-4000-8000-00000000000a"
	commandExchangePlan  = "cd000000-0000-4000-8000-00000000000b"
	commandAccountingReg = "cd000000-0000-4000-8000-00000000000c"
	commandCalcRegister  = "cd000000-0000-4000-8000-00000000000d"
	commandReport        = "cd000000-0000-4000-8000-00000000000e"
	commandDataProcessor = "cd000000-0000-4000-8000-00000000000f"

	commandID    = "cd000000-0000-4000-8000-000000000100"
	commandGroup = "cd000000-0000-4000-8000-000000000102"
)

// oneOfEach is what every kind of object gets in the tests below: the same
// command and the same template, described the same way, because the point of
// both iterations is that neither depends on what it hangs off. Only the
// identifiers differ, and they differ because two objects never share a file.
//
// The command names no module: it keeps a folder of its own, and the module
// inside that folder is the command's by where it lies.
func oneOfEach(command, template string) string {
	return `
commands:
  - id: ` + command + `
    name: ОткрытьСписок
    title: {ru: Открыть список}
    group: navigation-panel-ordinary
templates:
  - id: ` + template + `
    name: ПечатнаяФорма
    title: {ru: Печатная форма}
    kind: spreadsheet
`
}

// A command is a subordinate entity of every kind of object at once, not a
// property of one of them. A kind that quietly lacks commands loses them
// silently at import - the loss shows up long after the configuration was
// transferred, so it is checked here for every kind that can hold one.
func TestEveryObjectKindCarriesItsOwnCommands(t *testing.T) {
	t.Parallel()
	root := subordinateProject(t)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string][]ObjectCommand{}
	definition, ok := catalog.CatalogDefinition("Контрагенты")
	if !ok {
		t.Fatal("the catalog did not load")
	}
	kinds["справочник"] = definition.Commands
	document, ok := catalog.DocumentDefinition("РасходТовара")
	if !ok {
		t.Fatal("the document did not load")
	}
	kinds["документ"] = document.Commands
	journal, ok := catalog.DocumentJournal("ОбщийЖурнал")
	if !ok {
		t.Fatal("the document journal did not load")
	}
	kinds["журнал документов"] = journal.Commands
	information, ok := catalog.InformationRegisterDefinition("Курсы")
	if !ok {
		t.Fatal("the information register did not load")
	}
	kinds["регистр сведений"] = information.Commands
	accumulation, ok := catalog.AccumulationRegisterDefinition("ОстаткиТоваров")
	if !ok {
		t.Fatal("the accumulation register did not load")
	}
	kinds["регистр накопления"] = accumulation.Commands
	characteristics, ok := catalog.ChartOfCharacteristicTypes("ВидыСвойств")
	if !ok {
		t.Fatal("the chart of characteristic types did not load")
	}
	kinds["план видов характеристик"] = characteristics.Commands
	accounts, ok := catalog.ChartOfAccounts("Основной")
	if !ok {
		t.Fatal("the chart of accounts did not load")
	}
	kinds["план счетов"] = accounts.Commands
	calculationTypes, ok := catalog.ChartOfCalculationTypes("Начисления")
	if !ok {
		t.Fatal("the chart of calculation types did not load")
	}
	kinds["план видов расчёта"] = calculationTypes.Commands
	process, ok := catalog.BusinessProcess("Задание")
	if !ok {
		t.Fatal("the business process did not load")
	}
	kinds["бизнес-процесс"] = process.Commands
	task, ok := catalog.Task("Исполнение")
	if !ok {
		t.Fatal("the task did not load")
	}
	kinds["задача"] = task.Commands
	plan, ok := catalog.ExchangePlan("Филиалы")
	if !ok {
		t.Fatal("the exchange plan did not load")
	}
	kinds["план обмена"] = plan.Commands
	entries, ok := catalog.AccountingRegister("Хозрасчетный")
	if !ok {
		t.Fatal("the accounting register did not load")
	}
	kinds["регистр бухгалтерии"] = entries.Commands
	calculations, ok := catalog.CalculationRegister("ОсновныеНачисления")
	if !ok {
		t.Fatal("the calculation register did not load")
	}
	kinds["регистр расчёта"] = calculations.Commands
	report, ok := catalog.Report("ОстаткиНаСкладе")
	if !ok {
		t.Fatal("the report did not load")
	}
	kinds["отчёт"] = report.Commands
	processor, ok := catalog.DataProcessor("ЗагрузкаЦен")
	if !ok {
		t.Fatal("the data processor did not load")
	}
	kinds["обработка"] = processor.Commands

	if len(kinds) != len(objectFolderKindsForTest()) {
		t.Fatalf("the test covers %d kinds of object, the project has %d that keep their own commands",
			len(kinds), len(objectFolderKindsForTest()))
	}
	for kind, commands := range kinds {
		if len(commands) != 1 {
			t.Fatalf("%s lost its commands: %+v", kind, commands)
		}
		if commands[0].Name != "ОткрытьСписок" || commands[0].Group != "navigation-panel-ordinary" {
			t.Fatalf("%s carried its command but not what it says: %+v", kind, commands[0])
		}
	}
}

// Everything a command is described by has to survive the trip to disk and
// back: a property that loads as its zero value is indistinguishable from one
// the developer never set, and the difference is what the command does.
func TestCommandKeepsEveryPropertyItWasGiven(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, commandCatalog, `format: 1
id: `+commandCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
commands:
  - id: `+commandID+`
    name: НачислитьБонусы
    title: {ru: Начислить бонусы}
    comment: Считает бонусы за период
    tooltip: {ru: Начисляет бонусы выбранным контрагентам}
    group_ref: `+commandGroup+`
    parameter: [{kind: catalog, reference: `+commandCatalog+`}]
    parameter_use: multiple
    modifies_data: true
    picture: {standard: Начислить}
    representation: picture-and-text
    shortcut: Ctrl+Shift+B
    on_server_unavailable: not-available
`)
	writeCommandModule(t, root, CatalogKind, "Контрагенты", "НачислитьБонусы")
	// The command is placed in a group of the configuration, so that group has
	// to exist: a command placed nowhere is drawn nowhere.
	writeCommandGroup(t, root, commandGroup, "Бонусы")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("Контрагенты")
	if !ok {
		t.Fatal("the catalog did not load")
	}
	if len(definition.Commands) != 1 {
		t.Fatalf("the command was lost: %+v", definition.Commands)
	}
	command := definition.Commands[0]
	switch {
	case command.Comment != "Считает бонусы за период":
		t.Fatalf("the comment was lost: %+v", command)
	case command.Tooltip["ru"] == "":
		t.Fatalf("the tooltip was lost: %+v", command)
	case command.GroupRef == nil || command.GroupRef.String() != commandGroup:
		t.Fatalf("the group of the configuration was lost: %+v", command)
	case len(command.Parameter) != 1 || command.Parameter[0].Kind != CatalogType:
		t.Fatalf("the parameter type was lost: %+v", command)
	case command.ParameterUse != CommandParameterMultiple:
		t.Fatalf("a command over several objects became a command over one: %+v", command)
	case !command.ModifiesData:
		t.Fatalf("a command that changes data was carried as one that does not: %+v", command)
	case command.Picture == nil || command.Picture.Standard != "Начислить":
		t.Fatalf("the picture was lost: %+v", command)
	case command.Representation != CommandPictureAndText:
		t.Fatalf("the representation was lost: %+v", command)
	case command.Shortcut != "Ctrl+Shift+B":
		t.Fatalf("the shortcut was lost: %+v", command)
	case command.OnServerUnavailable != ServerUnavailableNotAvailable:
		t.Fatalf("what the command does without the main server was lost: %+v", command)
	}

	// The catalog hands out copies: a caller that edits what it was given must
	// not be editing the loaded configuration.
	definition.Commands[0].Name = "Подменено"
	again, _ := catalog.CatalogDefinition("Контрагенты")
	if again.Commands[0].Name != "НачислитьБонусы" {
		t.Fatal("the commands of an object were handed out by reference")
	}
}

// A command keeps a folder of its own beside the object, holding the module
// that runs it. Declaring a command and not writing that module is the same
// mistake as declaring a form and not writing it, and it is caught in the same
// place. A folder for a command nobody declares is the mistake seen from the
// other side: nothing would ever run what is in it.
func TestCommandModuleIsExpectedInTheObjectFolder(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, commandCatalog, `format: 1
id: `+commandCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
commands:
  - id: `+commandID+`
    name: НачислитьБонусы
    title: {ru: Начислить бонусы}
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a command module that does not exist was accepted")
	}
	writeCommandModule(t, root, CatalogKind, "Контрагенты", "НачислитьБонусы")
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	writeCommandModule(t, root, CatalogKind, "Контрагенты", "НачислитьПрочее")
	if _, err := Load(root); err == nil {
		t.Fatal("a folder for a command the object does not declare was accepted")
	}
}

// What a command is checked for is what makes it a command: a name of its own,
// a place to be shown in, and a parameter that means something. The module is
// not among it: a command declares none, so there is nothing here to be wrong.
func TestBrokenCommandsAreRefused(t *testing.T) {
	t.Parallel()
	const second = "cd000000-0000-4000-8000-000000000103"
	// Each case says what it expects to hear back. A test that only demands a
	// refusal passes for the wrong reason the moment a new rule refuses the
	// body earlier than the rule under test does.
	for name, broken := range map[string]struct{ body, want string }{
		"два имени в одном объекте": {`commands:
  - {id: ` + commandID + `, name: Открыть, title: {ru: Открыть}}
  - {id: ` + second + `, name: открыть, title: {ru: Открыть ещё}}`,
			"commands[1].name must be unique"},
		"один идентификатор на две команды": {`commands:
  - {id: ` + commandID + `, name: Открыть, title: {ru: Открыть}}
  - {id: ` + commandID + `, name: Закрыть, title: {ru: Закрыть}}`,
			"commands[1].id must be unique"},
		"имя не идентификатор": {`commands:
  - {id: ` + commandID + `, name: "Открыть список", title: {ru: Открыть}}`,
			"commands[0].name must be a valid identifier"},
		"группы платформы с таким именем нет": {`commands:
  - {id: ` + commandID + `, name: Открыть, title: {ru: Открыть}, group: nowhere}`,
			"commands[0].group is not a standard group of the platform"},
		"место указано дважды": {`commands:
  - {id: ` + commandID + `, name: Открыть, title: {ru: Открыть}, group: actions-panel-tools, group_ref: ` + commandGroup + `}`,
			"names both a standard group and a group of the configuration"},
		"режим параметра без параметра": {`commands:
  - {id: ` + commandID + `, name: Открыть, title: {ru: Открыть}, parameter_use: single}`,
			"commands[0].parameter_use needs a parameter type"},
		"отображение картинкой без картинки": {`commands:
  - {id: ` + commandID + `, name: Открыть, title: {ru: Открыть}, representation: picture}`,
			"commands[0].representation picture needs a picture"},
		"картинка из двух источников": {`commands:
  - {id: ` + commandID + `, name: Открыть, title: {ru: Открыть}, picture: {standard: Открыть, common: ` + commandGroup + `}}`,
			"names both a standard picture and a common picture"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(`format: 1
id: `+commandCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
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

// writeCommandGroup writes one command group - a place the configuration makes
// for its own commands.
func writeCommandGroup(t *testing.T, root, id, name string) {
	t.Helper()
	writeMetadata(t, root, CommandGroupKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
category: actions-panel
`)
}

// writeTemplateContent writes one file of a template's content into the folder
// that template keeps beside its object. The folder is named after the
// template; the file inside is still named after the kind of template.
func writeTemplateContent(t *testing.T, root string, kind Kind, objectName, template, file, content string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", string(kind), objectName, "templates", template)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeCommandModule writes the body of one command into the folder that
// command keeps beside its object. The folder is named after the command, and
// the file after the role it plays - a command has no identifier on disk.
func writeCommandModule(t *testing.T, root string, kind Kind, objectName, command string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", string(kind), objectName, "commands", command)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, project.CommandModuleFile)
	if err := os.WriteFile(path, []byte("&НаКлиенте\nПроцедура ОбработкаКоманды(ПараметрКоманды, ПараметрыВыполненияКоманды)\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeObjectModule writes one of an object's own modules under the name of
// the role it plays, which is the whole of what says which module it is.
func writeObjectModule(t *testing.T, root string, kind Kind, objectName, role string) {
	t.Helper()
	path := filepath.Join(root, "metadata", string(kind), objectName, role)
	if err := os.WriteFile(path, []byte("// модуль\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// objectName reads the name out of a body about to be written, the way the
// folder holding it is named.
func objectName(body string) string {
	return objectFolderName(body, "")
}

func objectFolderKindsForTest() []string {
	return []string{"catalogs", "documents", "document-journals", "information-registers", "accumulation-registers",
		"charts-of-characteristic-types", "charts-of-accounts", "charts-of-calculation-types",
		"business-processes", "tasks", "exchange-plans", "accounting-registers", "calculation-registers",
		"reports", "data-processors"}
}

// subordinateProject writes one object of every kind that keeps subordinate
// entities of its own, each with the same single command and the same single
// template.
func subordinateProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	// Identifiers of the commands and of the templates, handed out in the
	// order the objects are written below. A module needs none: it is named
	// after the role it plays, inside the folder of what owns it.
	next := 0
	ids := func() (string, string) {
		next++
		return fmt.Sprintf("cd000000-0000-4000-8000-0000000003%02x", next),
			fmt.Sprintf("cd000000-0000-4000-8000-0000000005%02x", next)
	}
	withCommand := func(kind Kind, objectID, body string) {
		t.Helper()
		command, template := ids()
		writeMetadata(t, root, kind, objectID, body+oneOfEach(command, template))
		writeCommandModule(t, root, kind, objectName(body), "ОткрытьСписок")
		writeTemplateContent(t, root, kind, objectName(body), "ПечатнаяФорма", "content.yaml", "format: 1\n")
	}
	withCommand(CatalogKind, commandCatalog, `format: 1
id: `+commandCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
`)
	withCommand(DocumentKind, commandDocument, `format: 1
id: `+commandDocument+`
name: РасходТовара
title: {ru: Расход товара}
number: {type: string, length: 9, auto: true, periodicity: none}
posting: true
`)
	withCommand(DocumentJournalKind, commandJournal, `format: 1
id: `+commandJournal+`
name: ОбщийЖурнал
title: {ru: Общий журнал}
documents: [`+commandDocument+`]
`)
	withCommand(InformationRegisterKind, commandInfoRegister, `format: 1
id: `+commandInfoRegister+`
name: Курсы
title: {ru: Курсы}
write_mode: independent
periodicity: day
dimensions:
  - {id: cd000000-0000-4000-8000-000000000201, name: Контрагент, title: {ru: Контрагент}, types: [{kind: catalog, reference: `+commandCatalog+`}]}
resources:
  - {id: cd000000-0000-4000-8000-000000000202, name: Курс, title: {ru: Курс}, types: [{kind: number, precision: 15, scale: 4}]}
`)
	withCommand(AccumulationRegisterKind, commandAccumRegister, `format: 1
id: `+commandAccumRegister+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: balance
recorders: [`+commandDocument+`]
dimensions:
  - {id: cd000000-0000-4000-8000-000000000203, name: Контрагент, title: {ru: Контрагент}, types: [{kind: catalog, reference: `+commandCatalog+`}]}
resources:
  - {id: cd000000-0000-4000-8000-000000000204, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}
`)
	withCommand(ChartOfCharacteristicTypesKind, commandCharTypes, `format: 1
id: `+commandCharTypes+`
name: ВидыСвойств
title: {ru: Виды свойств}
code: {type: string, length: 9, auto: true}
description_length: 150
value_type: [{kind: string, length: 100}]
`)
	withCommand(ChartOfAccountsKind, commandAccounts, `format: 1
id: `+commandAccounts+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 9, auto: false}
description_length: 150
`)
	withCommand(ChartOfCalculationTypesKind, commandCalcTypes, `format: 1
id: `+commandCalcTypes+`
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 9, auto: true}
description_length: 150
`)
	withCommand(TaskKind, commandTask, `format: 1
id: `+commandTask+`
name: Исполнение
title: {ru: Исполнение}
number: {type: string, length: 14, auto: true, unique: true, periodicity: none}
description_length: 150
number_prefix: business-process-number
addressing: `+commandInfoRegister+`
main_addressing_attribute: Контрагент
addressing_attributes:
  - id: cd000000-0000-4000-8000-000000000205
    name: Контрагент
    title: {ru: Контрагент}
    types: [{kind: catalog, reference: `+commandCatalog+`}]
    dimension: cd000000-0000-4000-8000-000000000201
`)
	withCommand(BusinessProcessKind, commandProcess, `format: 1
id: `+commandProcess+`
name: Задание
title: {ru: Задание}
number: {type: string, length: 11, auto: true, unique: true, periodicity: none}
task: `+commandTask+`
route:
  points:
    - {id: cd000000-0000-4000-8000-000000000206, name: Старт, kind: start}
    - {id: cd000000-0000-4000-8000-000000000207, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}
`)
	withCommand(ExchangePlanKind, commandExchangePlan, `format: 1
id: `+commandExchangePlan+`
name: Филиалы
title: {ru: Филиалы}
code: {type: string, length: 36, auto: false}
description_length: 150
content:
  - {kind: catalogs, object: `+commandCatalog+`, auto_record: allow}
`)
	withCommand(AccountingRegisterKind, commandAccountingReg, `format: 1
id: `+commandAccountingReg+`
name: Хозрасчетный
title: {ru: Хозрасчётный}
chart_of_accounts: `+commandAccounts+`
correspondence: true
recorders: [`+commandDocument+`]
resources:
  - {id: cd000000-0000-4000-8000-000000000208, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], balance: false}
`)
	withCommand(CalculationRegisterKind, commandCalcRegister, `format: 1
id: `+commandCalcRegister+`
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: `+commandCalcTypes+`
periodicity: month
recorders: [`+commandDocument+`]
resources:
  - {id: cd000000-0000-4000-8000-000000000209, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	withCommand(ReportKind, commandReport, `format: 1
id: `+commandReport+`
name: ОстаткиНаСкладе
title: {ru: Остатки на складе}
`)
	withCommand(DataProcessorKind, commandDataProcessor, `format: 1
id: `+commandDataProcessor+`
name: ЗагрузкаЦен
title: {ru: Загрузка цен}
`)
	return root
}
