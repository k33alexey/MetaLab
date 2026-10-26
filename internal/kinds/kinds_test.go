package kinds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestEveryKindHasEveryWord catches a kind added without one of its names: an
// empty word would make the kind unreachable from code in one language, or a
// blank caption in the interface; and without its icon, an empty square in
// the tree (whether the set draws it - internal/icons).
func TestEveryKindHasEveryWord(t *testing.T) {
	seen := map[Kind]bool{}
	for _, item := range All() {
		if seen[item.Kind] {
			t.Errorf("%s is described twice", item.Kind)
		}
		seen[item.Kind] = true
		for field, value := range map[string]string{
			"name in Russian": item.Name.Russian, "name in English": item.Name.English,
			"collection in Russian": item.Collection.Russian, "collection in English": item.Collection.English,
			"genitive": item.Genitive, "title": item.Title, "titles": item.Titles, "icon": item.Icon,
		} {
			if strings.TrimSpace(value) == "" {
				t.Errorf("%s has no %s", item.Kind, field)
			}
		}
		// The prototype writes no ё in a name of code; a name with it is not
		// the name code uses (ПланВидовРасчёта was written so in the
		// migration dialog).
		for _, word := range []string{item.Name.Russian, item.Collection.Russian, item.Genitive} {
			if strings.ContainsAny(word, "ёЁ") {
				t.Errorf("%s: %q is written with ё, the help writes it without", item.Kind, word)
			}
		}
	}
}

// TestNamesAreThePrototypes catches a name of a kind that differs from the
// prototype's. The expectation is written out here from the help (8.3.27,
// ОбъектМетаданных: <Name>, ОбъектМетаданныхКонфигурация.<Collection>) and
// the export's tags and folders, not read from the table under test.
func TestNamesAreThePrototypes(t *testing.T) {
	expected := map[Kind]string{
		Catalog:                    "Справочник Catalog Справочники Catalogs",
		Document:                   "Документ Document Документы Documents",
		Enumeration:                "Перечисление Enum Перечисления Enums",
		InformationRegister:        "РегистрСведений InformationRegister РегистрыСведений InformationRegisters",
		AccumulationRegister:       "РегистрНакопления AccumulationRegister РегистрыНакопления AccumulationRegisters",
		AccountingRegister:         "РегистрБухгалтерии AccountingRegister РегистрыБухгалтерии AccountingRegisters",
		CalculationRegister:        "РегистрРасчета CalculationRegister РегистрыРасчета CalculationRegisters",
		ChartOfCalculationTypes:    "ПланВидовРасчета ChartOfCalculationTypes ПланыВидовРасчета ChartsOfCalculationTypes",
		ChartOfCharacteristicTypes: "ПланВидовХарактеристик ChartOfCharacteristicTypes ПланыВидовХарактеристик ChartsOfCharacteristicTypes",
		ChartOfAccounts:            "ПланСчетов ChartOfAccounts ПланыСчетов ChartsOfAccounts",
		Numerator:                  "Нумератор DocumentNumerator НумераторыДокументов DocumentNumerators",
		Report:                     "Отчет Report Отчеты Reports",
		Constant:                   "Константа Constant Константы Constants",
		SessionParameter:           "ПараметрСеанса SessionParameter ПараметрыСеанса SessionParameters",
		DefinedType:                "ОпределяемыйТип DefinedType ОпределяемыеТипы DefinedTypes",
		FunctionalOptionParameter:  "ПараметрФункциональныхОпций FunctionalOptionsParameter ПараметрыФункциональныхОпций FunctionalOptionsParameters",
		CommonForm:                 "ОбщаяФорма CommonForm ОбщиеФормы CommonForms",
		ExternalDimensionTable:     "ТаблицаИзмерения DimensionTable ТаблицыИзмерений DimensionTables",
		Recalculation:              "Перерасчет Recalculation Перерасчеты Recalculations",
		ExternalFunction:           "Функция Function Функции Functions",
	}
	for kind, words := range expected {
		item, ok := Of(kind)
		if !ok {
			t.Errorf("%s has no descriptor", kind)
			continue
		}
		got := strings.Join([]string{item.Name.Russian, item.Name.English, item.Collection.Russian, item.Collection.English}, " ")
		if got != words {
			t.Errorf("%s is called %q, the prototype calls it %q", kind, got, words)
		}
	}
}

// TestNamesAreInTheHelp catches a name no class or collection of the help
// has, for every kind at once. The help lies outside the repository; without
// it the test is skipped with the reason.
func TestNamesAreInTheHelp(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "materials", "its", "8.3.27.2342", "json", "shcntx_ru.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the help is not here (%v): the materials are kept out of the repository", err)
	}
	var help struct {
		Pages []struct {
			TitleRU string `json:"title_ru"`
			TitleEN string `json:"title_en"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(content, &help); err != nil {
		t.Fatal(err)
	}
	class := regexp.MustCompile(`^ОбъектМетаданных: ([\p{L}\p{N}_]+)$`)
	collection := regexp.MustCompile(`^ОбъектМетаданных(?:Конфигурация|: [\p{L}\p{N}_]+)\.([\p{L}\p{N}_]+)$`)
	classes, collections := map[string]string{}, map[string]string{}
	for _, page := range help.Pages {
		if match := class.FindStringSubmatch(page.TitleRU); match != nil {
			classes[match[1]] = strings.TrimPrefix(page.TitleEN, "MetadataObject: ")
		}
		if match := collection.FindStringSubmatch(page.TitleRU); match != nil {
			collections[match[1]] = page.TitleEN[strings.LastIndex(page.TitleEN, ".")+1:]
		}
	}
	// A common form and a common template are of the classes Форма and Макет,
	// which an object's own forms and templates share; their full names
	// (ОбщаяФорма.Имя, help: ПолучитьФорму) and the export's tags name them.
	commonByFullName := map[Kind]bool{CommonForm: true, CommonTemplate: true}
	for _, item := range All() {
		if english, ok := classes[item.Name.Russian]; !commonByFullName[item.Kind] && (!ok || english != item.Name.English) {
			t.Errorf("%s: the help has no class %s (%s)", item.Kind, item.Name.Russian, item.Name.English)
		}
		if english, ok := collections[item.Collection.Russian]; !ok || english != item.Collection.English {
			t.Errorf("%s: the help has no collection %s (%s)", item.Kind, item.Collection.Russian, item.Collection.English)
		}
	}
}

// TestReferencesAreThePrototypes catches a reference type missing, misnamed
// or given to a kind whose objects have no reference. The expectation is
// written out here from the help (8.3.27: every page <Name>Ссылка.<Имя ...>),
// not read from the table under test; TestReferencesAreInTheHelp checks the
// same against the help itself when it is here.
func TestReferencesAreThePrototypes(t *testing.T) {
	expected := map[Kind]string{
		Catalog:                    "СправочникСсылка CatalogRef",
		Document:                   "ДокументСсылка DocumentRef",
		Enumeration:                "ПеречислениеСсылка EnumRef",
		ChartOfCharacteristicTypes: "ПланВидовХарактеристикСсылка ChartOfCharacteristicTypesRef",
		ChartOfAccounts:            "ПланСчетовСсылка ChartOfAccountsRef",
		ChartOfCalculationTypes:    "ПланВидовРасчетаСсылка ChartOfCalculationTypesRef",
		BusinessProcess:            "БизнесПроцессСсылка BusinessProcessRef ТочкаМаршрутаБизнесПроцессаСсылка BusinessProcessRoutePointRef",
		Task:                       "ЗадачаСсылка TaskRef",
		ExchangePlan:               "ПланОбменаСсылка ExchangePlanRef",
		ExternalDataSourceTable:    "ВнешнийИсточникДанныхТаблицаСсылка ExternalDataSourceTableRef",
		ExternalDimensionTable:     "ВнешнийИсточникДанныхКубТаблицаИзмеренияСсылка ExternalDataSourceCubeDimensionTableRef",
	}
	for _, item := range All() {
		var words []string
		for _, reference := range item.References() {
			words = append(words, reference.Russian, reference.English)
		}
		if got := strings.Join(words, " "); got != expected[item.Kind] {
			t.Errorf("%s has references %q, the prototype %q", item.Kind, got, expected[item.Kind])
		}
	}
}

// TestReferencesAreInTheHelp catches the help having a reference type no
// kind names, or a kind naming one the help has not: every page titled
// <Type>Ссылка.<...> of the help is a reference type. Without the help the
// test is skipped with the reason.
func TestReferencesAreInTheHelp(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "materials", "its", "8.3.27.2342", "json", "shcntx_ru.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the help is not here (%v): the materials are kept out of the repository", err)
	}
	var help struct {
		Pages []struct {
			TitleRU string `json:"title_ru"`
			TitleEN string `json:"title_en"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(content, &help); err != nil {
		t.Fatal(err)
	}
	page := regexp.MustCompile(`^([\p{L}\p{N}_]+Ссылка)\.<[^>]+>(?:\.<[^>]+>)*$`)
	inHelp := map[string]string{}
	for _, item := range help.Pages {
		if match := page.FindStringSubmatch(item.TitleRU); match != nil && item.TitleEN != "" {
			inHelp[match[1]] = item.TitleEN[:strings.IndexByte(item.TitleEN, '.')]
		}
	}
	named := map[string]string{}
	for _, item := range All() {
		for _, reference := range item.References() {
			named[reference.Russian] = reference.English
		}
	}
	for russian, english := range inHelp {
		if named[russian] != english {
			t.Errorf("the help has the reference type %s (%s), the kinds name it %q", russian, english, named[russian])
		}
	}
	for russian := range named {
		if _, ok := inHelp[russian]; !ok {
			t.Errorf("%s is named a reference type, the help has no such", russian)
		}
	}
}

// TestIsComparesAsTheLanguage catches a comparison of a name that minds the
// case, or that takes a word of one kind for another.
func TestIsComparesAsTheLanguage(t *testing.T) {
	catalog := Must(Catalog).Name
	for _, word := range []string{"Справочник", "СПРАВОЧНИК", "справочник", "Catalog", "CATALOG"} {
		if !catalog.Is(word) {
			t.Errorf("%q does not name a catalog", word)
		}
	}
	for _, word := range []string{"Справочники", "Catalogs", "Документ", "", "Справочник "} {
		if catalog.Is(word) {
			t.Errorf("%q names a catalog", word)
		}
	}
	if _, ok := Of("catalog"); ok {
		t.Error("a kind is found by a folder no kind has")
	}
}

// TestACollectionNamesItsKindAndNoOther catches ByCollection missing the
// first kind it knows - subsystems stand first - or answering a word that is
// no collection: Справочники in any case is the catalogs, Подсистемы the
// subsystems, Нечто nothing (2.432, a mutant lived on both).
func TestACollectionNamesItsKindAndNoOther(t *testing.T) {
	for word, want := range map[string]Kind{"Подсистемы": Subsystem, "справочники": Catalog, "Catalogs": Catalog} {
		if got, ok := ByCollection(word); !ok || got != want {
			t.Errorf("ByCollection(%q) = %q, %v; want %q", word, got, ok, want)
		}
	}
	if got, ok := ByCollection("Нечто"); ok {
		t.Errorf("ByCollection(Нечто) = %q, want nothing", got)
	}
}

// TestTheLayoutSaysWhereAnObjectLies catches the layout answering wrongly
// what the tree and the project's files are built from (2.432, no test ran
// these): a subsystem lies in one file and a recalculation in its register,
// a catalog, a constant and a common form keep a folder; numerators and
// sequences stand inside the documents' branch (inventory of block 00,
// the configuration tree), a catalog in its own; opening the module of a
// command is the command's own action, of an object it is not; a module's
// file is read back as the module, any other name as none; and no kind is
// left with no way to lie at all.
func TestTheLayoutSaysWhereAnObjectLies(t *testing.T) {
	for kind, want := range map[Kind]bool{Subsystem: false, Recalculation: false, Catalog: true, Constant: true, CommonForm: true} {
		if got := Must(kind).Layout().KeepsFolder(); got != want {
			t.Errorf("%s keeps a folder: %v, want %v", kind, got, want)
		}
	}
	for kind, want := range map[Kind]Kind{Numerator: Document, Sequence: Document, Catalog: ""} {
		if got := Must(kind).Layout().Within(); got != want {
			t.Errorf("%s stands within %q, want %q", kind, got, want)
		}
	}
	if !CommandModule.OwnAction() || ObjectModule.OwnAction() {
		t.Errorf("own action: command module %v, object module %v; want the first alone", CommandModule.OwnAction(), ObjectModule.OwnAction())
	}
	if module, ok := ModuleOfFile("МодульОбъекта.bsl"); !ok || module != ObjectModule {
		t.Errorf("МодульОбъекта.bsl is read as %q, %v", module, ok)
	}
	if module, ok := ModuleOfFile("Нечто.bsl"); ok {
		t.Errorf("Нечто.bsl is read as %q", module)
	}
	var unset Storage
	for _, item := range All() {
		if item.Layout().Storage == unset {
			t.Errorf("%s has no way to lie in the project", item.Kind)
		}
	}
}

// The window of a catalog lists the fifteen pages of the prototype's, in its
// order, written out from the inventory (docs/block-00/spravochnik.md, «Окно
// редактирования»; snapshots spravochnik/okno-01 … okno-15). Defect caught:
// a page left out, renamed or moved, which the window would then show so.
func TestTheWindowOfACatalogHasThePrototypesPages(t *testing.T) {
	want := []string{"Основные", "Подсистемы", "Функциональные опции", "Иерархия", "Владельцы", "Данные", "Нумерация",
		"Формы", "Поле ввода", "Команды", "Макеты", "Ввод на основании", "Права", "Обмен данными", "Прочее"}
	var got []string
	keys := map[string]bool{}
	for _, page := range Must(Catalog).WindowPages() {
		got = append(got, page.Title)
		if page.Key == "" || keys[page.Key] {
			t.Errorf("the page %q has the key %q, empty or repeated", page.Title, page.Key)
		}
		keys[page.Key] = true
	}
	if !slices.Equal(got, want) {
		t.Errorf("the window of a catalog lists %q, want %q", got, want)
	}
	if pages := Must(Constant).WindowPages(); len(pages) != 0 {
		t.Errorf("a constant has no window, yet lists %v", pages)
	}
}
