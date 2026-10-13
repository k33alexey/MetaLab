package kinds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEveryKindIsLaidOut catches a kind added without a layout, or with one
// that cannot be followed: a kind with no storage is not read from anywhere,
// an object lying inside another with no owner or folder has no path, and a
// kind of the configuration with no branch is missing from the tree and from
// the folders of metadata/.
func TestEveryKindIsLaidOut(t *testing.T) {
	for _, item := range All() {
		layout, ok := layouts[item.Kind]
		if !ok {
			t.Errorf("%s has no layout", item.Kind)
			continue
		}
		if layout.Storage == 0 {
			t.Errorf("%s says nothing of where its objects lie", item.Kind)
		}
		inside := layout.Owner != "" || layout.Folder != ""
		// An object described in its owner's description keeps nothing on
		// disk and needs no folder; every other object inside another does.
		described := layout.Storage == InOwnerDescription
		if inside && (layout.Owner == "" || (layout.Folder == "") != described || layout.Branch != NoBranch) {
			t.Errorf("%s lies inside another object and is laid out as %+v", item.Kind, layout)
		}
		if described && (!inside || len(layout.Modules) > 0 || layout.Forms) {
			t.Errorf("%s is described in its owner and keeps something on disk, or has no owner: %+v", item.Kind, layout)
		}
		if !inside && layout.Branch == NoBranch {
			t.Errorf("%s is an object of the configuration with no branch of the tree", item.Kind)
		}
		if _, ok := Of(layout.Owner); inside && !ok {
			t.Errorf("%s lies inside %q, which is no kind", item.Kind, layout.Owner)
		}
		if (layout.Commands || layout.Templates) && !layout.Forms {
			t.Errorf("%s has commands or templates and no forms", item.Kind)
		}
		for _, module := range layout.Modules {
			if module.Title() == "" {
				t.Errorf("%s keeps %s, which has no caption", item.Kind, module)
			}
		}
	}
	for kind := range layouts {
		if _, ok := Of(kind); !ok {
			t.Errorf("%s is laid out and is no kind", kind)
		}
	}
}

// TestModulesAreThePrototypes catches a kind given a module its objects do
// not have, or denied one they have: the loader refuses the second, and the
// first lets a file nothing runs lie in the object's folder. The expectation
// is written out here from the exports (four configurations checked and
// energy, Ext/*.bsl of every object, forms and commands aside), not read from
// layouts: ValueManagerModule is МодульЗначения, Module of a service
// МодульСервиса.
func TestModulesAreThePrototypes(t *testing.T) {
	expected := map[Kind]string{
		Catalog: "МодульОбъекта МодульМенеджера", Document: "МодульОбъекта МодульМенеджера",
		Report: "МодульОбъекта МодульМенеджера", DataProcessor: "МодульОбъекта МодульМенеджера",
		ChartOfCharacteristicTypes: "МодульОбъекта МодульМенеджера", ChartOfAccounts: "МодульОбъекта МодульМенеджера",
		ChartOfCalculationTypes: "МодульОбъекта МодульМенеджера", ExchangePlan: "МодульОбъекта МодульМенеджера",
		BusinessProcess: "МодульОбъекта МодульМенеджера", Task: "МодульОбъекта МодульМенеджера",
		InformationRegister: "МодульНабораЗаписей МодульМенеджера", AccumulationRegister: "МодульНабораЗаписей МодульМенеджера",
		AccountingRegister: "МодульНабораЗаписей МодульМенеджера", CalculationRegister: "МодульНабораЗаписей МодульМенеджера",
		Enumeration: "МодульМенеджера", DocumentJournal: "МодульМенеджера", FilterCriterion: "МодульМенеджера",
		SettingsStorage: "МодульМенеджера", Sequence: "МодульНабораЗаписей", Recalculation: "МодульНабораЗаписей",
		Constant: "МодульЗначения МодульМенеджера", CommonCommand: "МодульКоманды", CommonForm: "МодульФормы",
		WebService: "МодульСервиса", HTTPService: "МодульСервиса", WebSocketClient: "МодульКлиента",
		// help: ВнешнийИсточникДанныхТаблицаОбъект, ...ТаблицаНаборЗаписей,
		// ...КубНаборЗаписей, ...КубТаблицаИзмеренияОбъект and their managers.
		ExternalDataSourceTable: "МодульОбъекта МодульНабораЗаписей МодульМенеджера",
		ExternalCube:            "МодульНабораЗаписей МодульМенеджера",
		ExternalDimensionTable:  "МодульОбъекта МодульМенеджера",
	}
	for _, item := range All() {
		var words []string
		for _, module := range item.Layout().Modules {
			words = append(words, string(module))
		}
		if got := strings.Join(words, " "); got != expected[item.Kind] {
			t.Errorf("%s keeps modules %q, the prototype %q", item.Kind, got, expected[item.Kind])
		}
	}
}

// TestFormsCommandsAndTemplatesAreThePrototypes catches a kind given forms,
// commands or templates its objects do not have, or denied them: Studio shows
// a group by them, and the loader and publication accept a folder by them.
// The expectation is written out from the help (8.3.27, ОбъектМетаданных:
// <Вид>.Формы, .Команды, .Макеты); TestFormsCommandsAndTemplatesAreInTheHelp
// checks the same against the help itself when it is here.
func TestFormsCommandsAndTemplatesAreThePrototypes(t *testing.T) {
	all := "Формы Команды Макеты"
	expected := map[Kind]string{
		Catalog: all, Document: all, DocumentJournal: all, Enumeration: all, Report: all, DataProcessor: all,
		ChartOfCharacteristicTypes: all, ChartOfAccounts: all, ChartOfCalculationTypes: all, ExchangePlan: all,
		BusinessProcess: all, Task: all, InformationRegister: all, AccumulationRegister: all,
		AccountingRegister: all, CalculationRegister: all, ExternalDataSourceTable: all, ExternalCube: all,
		ExternalDimensionTable: all, FilterCriterion: "Формы Команды", SettingsStorage: "Формы Макеты",
	}
	for _, item := range All() {
		if got := ownedWords(item.Layout()); got != expected[item.Kind] {
			t.Errorf("%s keeps %q, the prototype %q", item.Kind, got, expected[item.Kind])
		}
	}
}

func ownedWords(layout Layout) string {
	var words []string
	for index, has := range []bool{layout.Forms, layout.Commands, layout.Templates} {
		if has {
			words = append(words, []string{"Формы", "Команды", "Макеты"}[index])
		}
	}
	return strings.Join(words, " ")
}

// TestFormsCommandsAndTemplatesAreInTheHelp reads the same from the help:
// every class whose page of Формы, Команды or Макеты the help has. A common
// form is of the class Форма and has none. Without the help the test is
// skipped with the reason.
func TestFormsCommandsAndTemplatesAreInTheHelp(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "materials", "its", "8.3.27.2342", "json", "shcntx_ru.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the help is not here (%v): the materials are kept out of the repository", err)
	}
	var help struct {
		Pages []struct {
			TitleRU string `json:"title_ru"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(content, &help); err != nil {
		t.Fatal(err)
	}
	page := regexp.MustCompile(`^ОбъектМетаданных: ([\p{L}\p{N}_]+)\.(Формы|Команды|Макеты)$`)
	inHelp := map[string]map[string]bool{}
	for _, item := range help.Pages {
		if match := page.FindStringSubmatch(item.TitleRU); match != nil {
			if inHelp[match[1]] == nil {
				inHelp[match[1]] = map[string]bool{}
			}
			inHelp[match[1]][match[2]] = true
		}
	}
	for _, item := range All() {
		var words []string
		for _, word := range []string{"Формы", "Команды", "Макеты"} {
			if inHelp[item.Name.Russian][word] {
				words = append(words, word)
			}
		}
		if item.Kind == CommonForm || item.Kind == CommonTemplate {
			words = nil
		}
		if got, want := ownedWords(item.Layout()), strings.Join(words, " "); got != want {
			t.Errorf("%s keeps %q, the help %q", item.Kind, got, want)
		}
	}
}

// TestBranchesAreTheConfigurators catches a kind moved in the tree or out of
// it: the order is the configurator's - «Общие» first, then the objects -
// with numerators and sequences inside «Документы».
func TestBranchesAreTheConfigurators(t *testing.T) {
	expected := "Common: subsystems common-modules session-parameters roles common-attributes exchange-plans filter-criteria " +
		"event-subscriptions scheduled-jobs functional-options functional-options-parameters defined-types settings-storages " +
		"common-commands command-groups common-forms common-templates common-pictures xdto-packages web-services http-services " +
		"ws-references websocket-clients style-items styles languages | Top: constants catalogs documents | Documents: " +
		"document-numerators sequences | Top: document-journals enumerations reports data-processors charts-of-characteristic-types " +
		"charts-of-accounts charts-of-calculation-types information-registers accumulation-registers accounting-registers " +
		"calculation-registers business-processes tasks external-data-sources"
	names := map[Branch]string{CommonBranch: "Common", TopBranch: "Top", DocumentsBranch: "Documents"}
	var words []string
	last := NoBranch
	for _, item := range Branches() {
		if branch := item.Layout().Branch; branch != last {
			if last != NoBranch {
				words = append(words, "|")
			}
			words = append(words, names[branch]+":")
			last = branch
		}
		words = append(words, string(item.Kind))
	}
	if got := strings.Join(words, " "); got != expected {
		t.Errorf("the tree is\n%s\nthe configurator's is\n%s", got, expected)
	}
}

// TestModuleWordsAreTheHelps catches a module whose code is denied a word the
// prototype gives it - the module does not compile, as 333 common modules
// of acc, erp and sb did not with ЭтотОбъект - or given one it does not have.
// The expectation is written out here from the help (8.3.27), not read from
// moduleWords: ЭтотОбъект is a property of every object, record set and value
// manager of a kind (СправочникОбъект, ПерерасчетНаборЗаписей,
// КонстантаМенеджерЗначения, ...), of ФормаКлиентскогоПриложения, of
// КомандаКомандногоИнтерфейса and of ОбщийМодуль, and of no manager, service
// or client; Движения - of ДокументОбъект only; Элементы - of
// ФормаКлиентскогоПриложения; ЭтаФорма is the exports'.
func TestModuleWordsAreTheHelps(t *testing.T) {
	object, set := "МодульОбъекта=ЭтотОбъект МодульМенеджера=", "МодульНабораЗаписей=ЭтотОбъект МодульМенеджера="
	expected := map[Kind]string{
		Catalog: object, Report: object, DataProcessor: object, ChartOfCharacteristicTypes: object,
		ChartOfAccounts: object, ChartOfCalculationTypes: object, ExchangePlan: object, BusinessProcess: object,
		Task: object, ExternalDimensionTable: object,
		Document:            "МодульОбъекта=ЭтотОбъект,Движения МодульМенеджера=",
		InformationRegister: set, AccumulationRegister: set, AccountingRegister: set, CalculationRegister: set,
		ExternalCube: set, Sequence: "МодульНабораЗаписей=ЭтотОбъект", Recalculation: "МодульНабораЗаписей=ЭтотОбъект",
		ExternalDataSourceTable: "МодульОбъекта=ЭтотОбъект МодульНабораЗаписей=ЭтотОбъект МодульМенеджера=",
		Constant:                "МодульЗначения=ЭтотОбъект МодульМенеджера=",
		Enumeration:             "МодульМенеджера=", DocumentJournal: "МодульМенеджера=", FilterCriterion: "МодульМенеджера=",
		SettingsStorage: "МодульМенеджера=", WebService: "МодульСервиса=", HTTPService: "МодульСервиса=",
		WebSocketClient: "МодульКлиента=", CommonCommand: "МодульКоманды=ЭтотОбъект",
		CommonForm: "МодульФормы=ЭтаФорма,ЭтотОбъект,Элементы",
	}
	russian := func(words []Words) string {
		var result []string
		for _, word := range words {
			result = append(result, word.Russian)
		}
		return strings.Join(result, ",")
	}
	for _, item := range All() {
		var modules []string
		for _, module := range item.Layout().Modules {
			modules = append(modules, string(module)+"="+russian(item.ModuleWords(module)))
		}
		if got := strings.Join(modules, " "); got != expected[item.Kind] {
			t.Errorf("the modules of %s find %q, the help gives %q", item.Kind, got, expected[item.Kind])
		}
	}
	// A form and a command of an object, and a common module, which plays no
	// role in a folder.
	for name, test := range map[string]struct {
		kind   Kind
		module Module
		want   string
	}{
		"a form of a catalog":    {Catalog, FormModule, "ЭтаФорма,ЭтотОбъект,Элементы"},
		"a form of a document":   {Document, FormModule, "ЭтаФорма,ЭтотОбъект,Элементы"},
		"a command of a catalog": {Catalog, CommandModule, "ЭтотОбъект"},
		"a common module":        {CommonModule, "", "ЭтотОбъект"},
	} {
		if got := russian(Must(test.kind).ModuleWords(test.module)); got != test.want {
			t.Errorf("%s finds %q, the help gives %q", name, got, test.want)
		}
	}
	if got := Must(Document).ModuleWords(ObjectModule); got[1].English != "RegisterRecords" || got[0].English != "ThisObject" {
		t.Errorf("a document's object module finds %+v: the help's English names are ThisObject and RegisterRecords", got)
	}
}
