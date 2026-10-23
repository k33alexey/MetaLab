package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestLoadProjectModulesFromDemoProject(t *testing.T) {
	t.Parallel()
	root := demoProjectRoot(t)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 2 {
		t.Fatalf("expected 2 document object modules in the demo project, got %d: %+v", len(modules), modules)
	}
	byName := make(map[string]RuntimeModule, len(modules))
	for _, module := range modules {
		byName[module.Name] = module
	}
	receipt, ok := byName["МодульОбъектаДокумента.ПоступлениеТоваров"]
	if !ok {
		t.Fatalf("missing receipt document module, got names: %v", moduleNames(modules))
	}
	if !strings.Contains(receipt.Source, "ВидДвиженияНакопления.Приход") {
		t.Fatalf("receipt module source missing expected posting logic: %s", receipt.Source)
	}
	sale, ok := byName["МодульОбъектаДокумента.ПродажаТоваров"]
	if !ok {
		t.Fatalf("missing sale document module, got names: %v", moduleNames(modules))
	}
	if !strings.Contains(sale.Source, "ВидДвиженияНакопления.Расход") {
		t.Fatalf("sale module source missing expected posting logic: %s", sale.Source)
	}
	for _, module := range modules {
		if len(module.PredefinedVariables) != 4 {
			t.Fatalf("document object module %s should predefine ЭтотОбъект/ThisObject/Движения/RegisterRecords, got %v", module.Name, module.PredefinedVariables)
		}
	}
	// Building the snapshot and compiling it must succeed end to end.
	snapshot, err := RuntimeSnapshot{Format: CurrentFormat}.WithModules(modules)
	if err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, err := snapshot.CompileModules(); err != nil {
		t.Fatalf("compile demo project BSL: %v (%v)", err, diagnostics)
	}
}

func moduleNames(modules []RuntimeModule) []string {
	names := make([]string, len(modules))
	for index, module := range modules {
		names[index] = module.Name
	}
	return names
}

// demoProjectRoot locates examples/sales-and-warehouse relative to this
// package, walking up from the working directory the same way `go test`
// runs it (package directory), independent of the caller's cwd.
func demoProjectRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// A project publishes whatever number and size of modules it has: erp has
// 10 962 modules and 425 MB of them, against the 10 000 modules and 16 MB in
// all that publication used to take. Ten thousand and one modules, one of them
// of 17 MB, must all be read.
func TestProjectModulesHaveNoCeilingOnCountOrSize(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	directory := filepath.Join(root, "modules")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	const count = 10_001
	large := strings.Repeat("// Строка комментария модуля.\n", 600_000)
	for index := range count {
		content := "Процедура П()\nКонецПроцедуры\n"
		if index == 0 {
			content = large + content
		}
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("m%05d.bsl", index)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatalf("%d modules with %d bytes in one were refused: %v", count, len(large), err)
	}
	read := 0
	for _, module := range modules {
		if strings.HasPrefix(module.Filename, "modules/") {
			read++
		}
	}
	if read != count {
		t.Fatalf("%d modules read, want %d", read, count)
	}
}

// TestEveryModuleIsNamedByWhereItLies catches a module of a kind, a form, a
// command or an object inside another that is compiled under a name built
// from its path, or without the words the prototype gives its code: before
// one reading of the path named them, six kinds of twenty-six had names and
// ЭтотОбъект, and an HTTP service, a common form, a command, a recalculation
// fell back to Модуль.<path>. The names and the words are written out here,
// not built by ObjectModuleName or read from the kinds package.
func TestEveryModuleIsNamedByWhereItLies(t *testing.T) {
	t.Parallel()
	// A form's module has ЭтаФорма and the 50 properties of the form, each in
	// both languages (kinds.TestFormModuleNamesTheForm holds the list); here
	// the first, two of the properties and the count stand for them.
	const this, form, records = "ЭтотОбъект ThisObject", "form", "ЭтотОбъект ThisObject Движения RegisterRecords"
	formWords := func(words []string) bool {
		joined := " " + strings.Join(words, " ") + " "
		return len(words) == 102 && strings.HasPrefix(joined, " ЭтаФорма ThisForm ") &&
			strings.Contains(joined, " Элементы Items ") && strings.Contains(joined, " Модифицированность Modified ")
	}
	namer := (*Catalog)(nil).ModuleNamer()
	for path, want := range map[string]struct{ name, words string }{
		"metadata/catalogs/Банки/МодульОбъекта.bsl":                          {"МодульОбъектаСправочника.Банки", this},
		"metadata/catalogs/Банки/МодульМенеджера.bsl":                        {"МодульМенеджераСправочника.Банки", ""},
		"metadata/catalogs/Банки/forms/ФормаЭлемента/МодульФормы.bsl":        {"МодульФормыСправочника.Банки.ФормаЭлемента", form},
		"metadata/catalogs/Банки/commands/Открыть/МодульКоманды.bsl":         {"МодульКомандыСправочника.Банки.Открыть", this},
		"metadata/documents/Продажа/МодульОбъекта.bsl":                       {"МодульОбъектаДокумента.Продажа", records},
		"metadata/reports/Остатки/МодульОбъекта.bsl":                         {"МодульОбъектаОтчета.Остатки", this},
		"metadata/data-processors/Загрузка/МодульОбъекта.bsl":                {"МодульОбъектаОбработки.Загрузка", this},
		"metadata/exchange-plans/Полный/МодульОбъекта.bsl":                   {"МодульОбъектаПланаОбмена.Полный", this},
		"metadata/business-processes/Задание/МодульОбъекта.bsl":              {"МодульОбъектаБизнесПроцесса.Задание", this},
		"metadata/tasks/Задача/МодульОбъекта.bsl":                            {"МодульОбъектаЗадачи.Задача", this},
		"metadata/charts-of-accounts/Хозрасчетный/МодульОбъекта.bsl":         {"МодульОбъектаПланаСчетов.Хозрасчетный", this},
		"metadata/accounting-registers/Хозрасчетный/МодульНабораЗаписей.bsl": {"МодульНабораЗаписейРегистраБухгалтерии.Хозрасчетный", this},
		"metadata/calculation-registers/Начисления/МодульНабораЗаписей.bsl":  {"МодульНабораЗаписейРегистраРасчета.Начисления", this},
		"metadata/calculation-registers/Начисления/recalculations/Перерасчет/МодульНабораЗаписей.bsl": {
			"МодульНабораЗаписейПерерасчета.Начисления.Перерасчет", this},
		"metadata/sequences/Партии/МодульНабораЗаписей.bsl":     {"МодульНабораЗаписейПоследовательности.Партии", this},
		"metadata/constants/Валюта/МодульЗначения.bsl":          {"МодульЗначенияКонстанты.Валюта", this},
		"metadata/enumerations/Виды/МодульМенеджера.bsl":        {"МодульМенеджераПеречисления.Виды", ""},
		"metadata/document-journals/Склад/МодульМенеджера.bsl":  {"МодульМенеджераЖурналаДокументов.Склад", ""},
		"metadata/settings-storages/Отчеты/МодульМенеджера.bsl": {"МодульМенеджераХранилищаНастроек.Отчеты", ""},
		"metadata/common-forms/Вопрос/МодульФормы.bsl":          {"МодульФормыОбщейФормы.Вопрос", form},
		"metadata/common-commands/Печать/МодульКоманды.bsl":     {"МодульКомандыОбщейКоманды.Печать", this},
		"metadata/http-services/API/МодульСервиса.bsl":          {"МодульСервисаHTTPСервиса.API", ""},
		"metadata/web-services/Обмен/МодульСервиса.bsl":         {"МодульСервисаWebСервиса.Обмен", ""},
		"metadata/websocket-clients/Чат/МодульКлиента.bsl":      {"МодульКлиентаWebSocketКлиента.Чат", ""},
		"metadata/external-data-sources/Склад/tables/Остатки/МодульНабораЗаписей.bsl": {
			"МодульНабораЗаписейТаблицыВнешнегоИсточника.Склад.Остатки", this},
		// What is no module of an object falls back to its path.
		"metadata/catalogs/Банки/МодульЗначения.bsl":                    {"Модуль.catalogs.Банки.МодульЗначения", ""},
		"metadata/catalogs/Банки/templates/Макет/МодульКоманды.bsl":     {"Модуль.catalogs.Банки.templates.Макет.МодульКоманды", ""},
		"metadata/catalogs/Банки/forms/ФормаЭлемента/МодульОбъекта.bsl": {"Модуль.catalogs.Банки.forms.ФормаЭлемента.МодульОбъекта", ""},
		"metadata/calculation-registers/Начисления/recalculations/Перерасчет/МодульОбъекта.bsl": {
			"Модуль.calculation-registers.Начисления.recalculations.Перерасчет.МодульОбъекта", ""},
		"tests/Проверка.bsl": {"Тест.Проверка", ""},
	} {
		got := namer.Identify(path)
		words := strings.Join(got.Predefined, " ") == want.words
		if want.words == form {
			words = formWords(got.Predefined)
		}
		if got.Name != want.name || !words {
			t.Errorf("%s is compiled as %q with %q, want %q with %q", path, got.Name, got.Predefined, want.name, want.words)
		}
	}
	// A common module lies apart by its identifier and is named out of the
	// catalog; without one it is no common module.
	id := uuid.MustNew()
	catalog := &Catalog{CommonModules: []CommonModuleDefinition{{Name: "ОбщегоНазначенияКлиент", Module: id, Client: true}}}
	path := "modules/" + id.String() + ".bsl"
	got := catalog.ModuleNamer().Identify(path)
	if got.Name != "ОбщегоНазначенияКлиент" || strings.Join(got.Predefined, " ") != this || got.DefaultContext != syntax.ContextClient || !got.Public {
		t.Errorf("a common module is compiled as %+v", got)
	}
	if got := namer.Identify(path); got.Name != "Модуль.modules."+id.String() || got.Public {
		t.Errorf("a module no common module claims is compiled as %+v", got)
	}
}
