package kinds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEveryKindHasEveryWord catches a kind added without one of its names: an
// empty word would make the kind unreachable from code in one language, or a
// blank caption in the interface.
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
			"genitive": item.Genitive, "title": item.Title, "titles": item.Titles,
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
