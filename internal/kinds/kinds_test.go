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

// The palette of a catalog has the four tabs of the prototype's, in its
// order - the last captioned «Справка» (the owner's decision of 26.10.2026) -
// «Справка» the one «только важные» takes away, and on
// «Основные» the name, the synonym, the comment and a row of each module of
// the kind, written out from the inventory (docs/block-00/spravochnik.md,
// «Палитра свойств»; snapshots palitra-osnovnye, vazhnye-osnovnye). Every
// row has a caption, a field Studio builds and a line of explanation.
// Defects caught: a tab left out or moved, a module row missing, a row
// without its explanation, a row of a field the palette cannot build.
func TestThePaletteOfACatalogIsThePrototypes(t *testing.T) {
	var tabs, important, main, data, dataImportant, presentation, presentationImportant, help []string
	fields := []string{NameField, LocalizedField, TextField, ModuleField, FlagField, ChoiceField, NumberField, SwitchField, ObjectsField, FieldsField, WindowField, FormField}
	for _, tab := range Must(Catalog).Palette() {
		tabs = append(tabs, tab.Title)
		if tab.Important {
			important = append(important, tab.Title)
		}
		for _, row := range tab.Rows {
			if row.Field == SeparatorField {
				continue
			}
			if row.Title == "" || row.Description == "" || !slices.Contains(fields, row.Field) {
				t.Errorf("%s: row %+v lacks a caption, an explanation or a field Studio builds", tab.Title, row)
			}
			if (row.Field == ChoiceField || row.Field == SwitchField) != (len(row.Choices) > 0) {
				t.Errorf("%s: row %q has choices %v for a field %s", tab.Title, row.Title, row.Choices, row.Field)
			}
			switch tab.Key {
			case "main":
				main = append(main, row.Title)
			case "data":
				data = append(data, row.Title)
				if row.Important {
					dataImportant = append(dataImportant, row.Title)
				}
			case "presentation":
				presentation = append(presentation, row.Title)
				if row.Important {
					presentationImportant = append(presentationImportant, row.Title)
				}
			case "help":
				help = append(help, row.Title)
			}
		}
	}
	// docs/block-00/spravochnik.md, «Палитра свойств», «Данные»; snapshots
	// palitra-dannye-vladelcy-kod0, vazhnye-dannye: «только важные» takes the
	// characteristics and the fields of the data lock away.
	wantData := []string{"Иерархический", "Вид иерархии", "Ограничивать кол-во уровней", "Количество уровней", "Размещать группы сверху",
		"Владельцы", "Использование подчинения", "Длина кода", "Длина наименования", "Тип кода", "Допустимая длина кода", "Серии кодов",
		"Контроль уникальности", "Автонумерация", "Стандартные реквизиты", "Характеристики", "Предопределенные",
		"Обновление предопределенных данных", "Вводится на основании", "Поля блокировки данных", "Режим управления блокировкой данных",
		"Полнотекстовый поиск", "История данных", "Обновлять историю данных сразу после записи",
		"Выполнять обработку после записи версии истории данных", "Дополнительные индексы"}
	if !slices.Equal(data, wantData) {
		t.Errorf("«Данные» %q, want %q", data, wantData)
	}
	// «Представление» and «Справка» (snapshots
	// palitra-predstavlenie-ne-ierarhicheskiy, vazhnye-predstavlenie,
	// palitra-spravochnaya-informaciya).
	if want := []string{"Использовать стандартные команды", "Основное представление", "Быстрый выбор", "Способ выбора", "Ввод по строке",
		"Способ поиска строки при вводе по строке", "Полнотекстовый поиск при вводе по строке",
		"Режим получения данных выбора при вводе по строке", "Создание при вводе", "История выбора при вводе",
		"Основная форма объекта", "Основная форма группы", "Основная форма списка", "Основная форма выбора", "Основная форма выб. группы",
		"Представление объекта", "Расширенное представление объекта", "Представление списка", "Расширенное представление списка",
		"Пояснение"}; !slices.Equal(presentation, want) {
		t.Errorf("«Представление» %q, want %q", presentation, want)
	}
	if want := []string{"Способ поиска строки при вводе по строке", "Полнотекстовый поиск при вводе по строке",
		"Режим получения данных выбора при вводе по строке"}; !slices.Equal(presentationImportant, want) {
		t.Errorf("«Представление» kept by «только важные» %q, want %q", presentationImportant, want)
	}
	if want := []string{"Включать в содержание справки", "Справочная информация"}; !slices.Equal(help, want) {
		t.Errorf("«Справка» %q, want %q", help, want)
	}
	wantImportant := slices.DeleteFunc(slices.Clone(wantData), func(title string) bool {
		return title == "Характеристики" || title == "Поля блокировки данных"
	})
	if !slices.Equal(dataImportant, wantImportant) {
		t.Errorf("«Данные» kept by «только важные» %q, want %q", dataImportant, wantImportant)
	}
	if want := []string{"Основные", "Данные", "Представление", "Справка"}; !slices.Equal(tabs, want) {
		t.Errorf("tabs %q, want %q", tabs, want)
	}
	if want := []string{"Основные", "Данные", "Представление"}; !slices.Equal(important, want) {
		t.Errorf("tabs kept by «только важные» %q, want %q", important, want)
	}
	if want := []string{"Имя", "Синоним", "Комментарий", "Модуль объекта", "Модуль менеджера"}; !slices.Equal(main, want) {
		t.Errorf("«Основные» %q, want %q", main, want)
	}
}

// The palettes of an attribute and of a table part are the prototype's,
// written out from the inventory (docs/block-00/rekvizit.md,
// tablichnaya-chast.md, «Палитра»; snapshots spravochnik/rekvizit-*): the
// rows of an attribute by the kind of its type - a string, a number, a
// reference - and those of every type; the use only at a part of a catalog.
// Defects caught: a row of one kind of type shown at another, a row missing
// or out of its place, the use shown at an attribute of a table part.
func TestThePalettesOfTheParts(t *testing.T) {
	shown := func(part, tab, condition string) []string {
		var result []string
		for _, item := range PartPalette(part) {
			if item.Key != tab {
				continue
			}
			for _, row := range item.Rows {
				if row.Field == SeparatorField {
					continue
				}
				if row.Description == "" {
					t.Errorf("%s/%s: %q has no explanation", part, tab, row.Title)
				}
				if len(row.Shown) == 0 || slices.Contains(row.Shown, condition) {
					result = append(result, row.Title)
				}
			}
		}
		return result
	}
	for name, testCase := range map[string]struct {
		got, want []string
	}{
		"attribute, main, string":         {shown("attribute", "main", "type=string"), []string{"Имя", "Синоним", "Комментарий", "Тип", "Длина", "Допустимая длина", "Неограниченная длина"}},
		"attribute, main, number":         {shown("attribute", "main", "type=number"), []string{"Имя", "Синоним", "Комментарий", "Тип", "Длина", "Точность", "Неотрицательное"}},
		"attribute, main, reference":      {shown("attribute", "main", "type=reference"), []string{"Имя", "Синоним", "Комментарий", "Тип"}},
		"attribute, use, of a catalog":    {shown("attribute", "use", "owner=catalogs"), []string{"Использование", "Индексировать", "Полнотекстовый поиск", "История данных"}},
		"attribute, use, of a table part": {shown("attribute", "use", "owner=tablePart"), []string{"Индексировать", "Полнотекстовый поиск", "История данных"}},
		"attribute, presentation, string": {shown("attribute", "presentation", "type=string"), []string{"Режим пароля", "Подсказка", "Маска",
			"Многострочный режим", "Расширенное редактирование", "Заполнять из данных заполнения", "Значение заполнения", "Проверка заполнения",
			"Быстрый выбор", "Создание при вводе", "История выбора при вводе", "Связь по типу"}},
		"attribute, presentation, number": {shown("attribute", "presentation", "type=number"), []string{"Формат", "Формат редактирования", "Подсказка",
			"Выделять отрицательные", "Минимальное значение", "Максимальное значение", "Заполнять из данных заполнения", "Значение заполнения",
			"Проверка заполнения", "Быстрый выбор", "Создание при вводе", "История выбора при вводе", "Связь по типу"}},
		"attribute, presentation, reference": {shown("attribute", "presentation", "type=reference"), []string{"Подсказка", "Выбор групп и элементов",
			"Связи параметров выбора", "Параметры выбора", "Форма выбора", "Заполнять из данных заполнения", "Значение заполнения", "Проверка заполнения",
			"Быстрый выбор", "Создание при вводе", "История выбора при вводе", "Связь по типу"}},
		"table part, main":         {shown("tablePart", "main", ""), []string{"Имя", "Синоним", "Комментарий"}},
		"table part, data":         {shown("tablePart", "data", ""), []string{"Стандартные реквизиты", "Длина номера строки"}},
		"table part, use":          {shown("tablePart", "use", "owner=catalogs"), []string{"Использование"}},
		"table part, presentation": {shown("tablePart", "presentation", ""), []string{"Подсказка", "Проверка заполнения"}},
	} {
		if !slices.Equal(testCase.got, testCase.want) {
			t.Errorf("%s: %q, want %q", name, testCase.got, testCase.want)
		}
	}
}

// shownRows are the captions of a tab a value of the kinds of type given
// shows - one kind, or several of a composite type, which shows the union of
// theirs once each.
func shownRows(tabs []PaletteTab, tab string, conditions ...string) []string {
	var result []string
	for _, item := range tabs {
		if item.Key != tab {
			continue
		}
		for _, row := range item.Rows {
			if row.Field == SeparatorField {
				continue
			}
			if len(row.Shown) == 0 || slices.ContainsFunc(row.Shown, func(condition string) bool { return slices.Contains(conditions, condition) }) {
				result = append(result, row.Title)
			}
		}
	}
	return result
}

// The palette of a constant is the prototype's (docs/block-00/konstanta.md,
// «Палитра свойств»; snapshots konstanta/palitra-*), and the presentation of
// a value by the kind of its type is one rule for a constant and an
// attribute (obshchee.md, «Представление по типу»): a date and a boolean have
// the format and the format of editing, a value storage only the tooltip, a
// composite type the union. Defects caught: a tab or a row missing or out of
// place; «только важные» keeping «Представление»; the tooltip shown twice
// for a composite type, or a row of one kind at another.
func TestThePaletteOfAConstantIsThePrototypes(t *testing.T) {
	palette := Must(Constant).Palette()
	var tabs, important []string
	for _, tab := range palette {
		tabs = append(tabs, tab.Title)
		if tab.Important {
			important = append(important, tab.Title)
		}
	}
	presentationOfAll := []string{"Проверка заполнения", "Быстрый выбор", "История выбора при вводе", "Связь по типу"}
	head := []string{"Использовать стандартные команды", "Основная форма"}
	middle := []string{"Расширенное представление", "Пояснение"}
	for name, testCase := range map[string]struct{ got, want []string }{
		"tabs":                  {tabs, []string{"Основные", "Данные", "Представление"}},
		"only important":        {important, []string{"Основные", "Данные"}},
		"main, date":            {shownRows(palette, "main", "type=date"), []string{"Имя", "Синоним", "Комментарий", "Тип", "Состав даты", "Модуль менеджера", "Модуль менеджера значения"}},
		"data":                  {shownRows(palette, "data"), []string{"Режим управления блокировкой данных", "История данных", "Обновлять историю данных сразу после записи", "Выполнять обработку после записи версии истории данных"}},
		"presentation, string":  {shownRows(palette, "presentation", "type=string"), slices.Concat(head, middle, []string{"Режим пароля", "Подсказка", "Маска", "Многострочный режим", "Расширенное редактирование"}, presentationOfAll)},
		"presentation, date":    {shownRows(palette, "presentation", "type=date"), slices.Concat(head, middle, []string{"Формат", "Формат редактирования", "Подсказка"}, presentationOfAll)},
		"presentation, boolean": {shownRows(palette, "presentation", "type=boolean"), slices.Concat(head, middle, []string{"Формат", "Формат редактирования", "Подсказка"}, presentationOfAll)},
		"presentation, storage": {shownRows(palette, "presentation", "type=value-storage"), slices.Concat(head, middle, []string{"Подсказка"}, presentationOfAll)},
		"presentation, reference": {shownRows(palette, "presentation", "type=reference"), slices.Concat(head,
			[]string{"Выбор групп и элементов", "Связи параметров выбора", "Параметры выбора", "Форма выбора"}, middle, []string{"Подсказка"}, presentationOfAll)},
		"attribute, string and number": {shownRows(PartPalette("attribute"), "presentation", "type=string", "type=number")[:10], []string{"Формат", "Формат редактирования",
			"Режим пароля", "Подсказка", "Маска", "Многострочный режим", "Расширенное редактирование", "Выделять отрицательные", "Минимальное значение", "Максимальное значение"}},
	} {
		if !slices.Equal(testCase.got, testCase.want) {
			t.Errorf("%s: %q, want %q", name, testCase.got, testCase.want)
		}
	}
}

// The palette of the root is the prototype's five tabs (docs/block-00/
// konfiguraciya.md, «Палитра»; snapshots konfiguraciya/01 to 07) without
// what ML does not carry (METADATA-OBJECTS.md, «Корень конфигурации») and
// without the rows the prototype's palette does not show (В13 of series
// 2.646): the modules stand after the main roles, where the root puts them;
// the main language has no cross. Defects caught: a tab or a row missing or
// out of place, a row of a mode, of the mobile application, of the logo or
// of a form of data history coming back, the main form of constants or the
// prefix of names shown, the modules standing elsewhere, the main language
// cleared.
func TestThePaletteOfTheRootIsThePrototypes(t *testing.T) {
	palette := RootPalette([]PaletteRow{{Key: "m1", Title: "Модуль приложения", Field: ModuleField}, {Key: "m2", Title: "Модуль сеанса", Field: ModuleField}})
	var tabs []string
	for _, tab := range palette {
		tabs = append(tabs, tab.Title)
	}
	for name, testCase := range map[string]struct{ got, want []string }{
		"tabs": {tabs, []string{"Основные", "Представление", "Разработка", "Справка", "Совместимость"}},
		"main": {shownRows(palette, "main"), []string{"Имя", "Синоним", "Комментарий", "Вариант встроенного языка", "Основные роли",
			"Модуль приложения", "Модуль сеанса", "Дополнительные словари полнотекстового поиска",
			"Хранилище общих настроек", "Хранилище пользовательских настроек отчетов", "Хранилище вариантов отчетов",
			"Хранилище настроек данных форм", "Хранилище пользовательских настроек динамических списков", "Хранилище внешних данных навигационных ссылок"}},
		"presentation": {shownRows(palette, "presentation"), []string{"Рабочая область начальной страницы", "Командный интерфейс основного раздела",
			"Картинка основного раздела", "Интерфейс клиентского приложения", "Основной стиль", "Основной язык", "Краткая информация",
			"Подробная информация", "Авторские права", "Адрес информации о поставщике", "Адрес информации о проекте",
			"Основная форма отчета", "Основная форма настроек отчета", "Основная форма варианта отчета",
			"Основная форма настроек динамического списка", "Основная форма поиска", "Основной макет оформления отчета"}},
		"development":   {shownRows(palette, "development"), []string{"Поставщик", "Версия"}},
		"help":          {shownRows(palette, "help"), []string{"Включать в содержание справки", "Справочная информация"}},
		"compatibility": {shownRows(palette, "compatibility"), []string{"Режим управления блокировкой данных"}},
	} {
		if !slices.Equal(testCase.got, testCase.want) {
			t.Errorf("%s: %q, want %q", name, testCase.got, testCase.want)
		}
	}
	for _, tab := range palette {
		for _, row := range tab.Rows {
			if row.Required != (row.Key == "defaultLanguage") {
				t.Errorf("%q: required %v", row.Title, row.Required)
			}
		}
	}
	if again := RootPalette(nil); len(again[0].Rows) != len(rootPalette[0].Rows)-1 {
		t.Errorf("the modules of one call stayed in the palette of the next: %d rows", len(again[0].Rows))
	}
}

// Every row of a page of a window built of the palette names a row the
// palette of its kind has, so the field, its values and its availability
// are the palette's and are not written twice; the pages «Поле ввода» and
// «Прочее» of a catalog hold the inventory's rows (docs/block-00/
// spravochnik.md, «Окно редактирования»). Defects caught: a window row
// naming a property the palette lacks, which would stand empty; a row of
// the inventory missing from a page.
func TestTheRowsOfAWindowAreThePalettes(t *testing.T) {
	for _, descriptor := range All() {
		keys := map[string]bool{}
		for _, tab := range descriptor.Palette() {
			for _, row := range tab.Rows {
				keys[row.Key] = true
			}
		}
		for page, rows := range descriptor.WindowRows() {
			for _, row := range rows {
				if !keys[row.Key] {
					t.Errorf("%s, page %s: %q names %q, which the palette has not", descriptor.Kind, page, row.Title, row.Key)
				}
			}
		}
	}
	titles := func(page string) []string {
		var result []string
		for _, row := range Must(Catalog).WindowRows()[page] {
			result = append(result, row.Title)
		}
		return result
	}
	if want := []string{"Элемента:", "Группы:", "Списка:", "Выбора:", "Выбора группы:"}; !slices.Equal(titles("forms"), want) {
		t.Errorf("«Формы» %q, want %q", titles("forms"), want)
	}
	if want := []string{"Использовать стандартные команды"}; !slices.Equal(titles("commands"), want) {
		t.Errorf("«Команды» %q, want %q", titles("commands"), want)
	}
	if want := []string{"Быстрый выбор", "Создание при вводе:", "Ввод по строке:", "Способ поиска строки:", "Полнотекстовый поиск:",
		"Режим получения данных выбора:", "История выбора при вводе:"}; !slices.Equal(titles("input"), want) {
		t.Errorf("«Поле ввода» %q, want %q", titles("input"), want)
	}
	if want := []string{"Модуль объекта", "Модуль менеджера", "Предопределенные", "Обновление предопределенных данных:", "Поля блокировки данных:",
		"Полнотекстовый поиск:", "История данных:", "Обновлять историю данных сразу после записи",
		"Выполнять обработку после записи версии истории данных", "Справочная информация", "Включать в содержание справки"}; !slices.Equal(titles("other"), want) {
		t.Errorf("«Прочее» %q, want %q", titles("other"), want)
	}
}

// The palette of a role is the prototype's (docs/block-00/rol.md, «Палитра»;
// snapshot rol/02-novaya-rol): one tab «Основные» - the name, the synonym,
// the comment and, apart, «Права» opening the window of the rights - every
// row important. Defects caught: the palette of a role empty, the name and
// the synonym left to its window; «Права» missing or not a window.
func TestThePaletteOfARoleIsThePrototypes(t *testing.T) {
	palette := Must(Role).Palette()
	if len(palette) != 1 || palette[0].Title != "Основные" || !palette[0].Important {
		t.Fatalf("the palette of a role is %+v, want the one tab «Основные»", palette)
	}
	var rows []string
	for _, row := range palette[0].Rows {
		rows = append(rows, row.Title+" "+row.Field)
		if !row.Important {
			t.Errorf("the row %q of a role is not important", row.Title)
		}
	}
	if want := []string{"Имя name", "Синоним localized", "Комментарий text", " separator", "Права window"}; !slices.Equal(rows, want) {
		t.Errorf("the rows of the palette of a role are %q, want %q", rows, want)
	}
}

// The captions of the pages «Подсистемы» and «Функциональные опции» name
// the object by the word the prototype puts there (obshchee.md, «Окно
// объекта»; series-2.584): the word of the kind in small letters, «вид
// расчета» for a chart of calculation types, a word of Latin letters as
// written. Defects caught: «план видов расчета»; «http-сервис».
func TestTheWordOfTheCaptionsOfAWindow(t *testing.T) {
	for kind, want := range map[Kind]string{Catalog: "справочник", InformationRegister: "регистр сведений",
		ChartOfCalculationTypes: "вид расчета", WebService: "Web-сервис", HTTPService: "HTTP-сервис", Subsystem: "подсистема"} {
		if got := Must(kind).MemberWord(); got != want {
			t.Errorf("%s: %q, want %q", kind, got, want)
		}
	}
}
