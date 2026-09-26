package metadata

import (
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A standard field is the platform's, and what the developer says about it is
// the whole point of the mechanism: the synonym a user reads, the format a
// number is shown in, whether the field is checked before a write. All of it
// has to come back out of the project unchanged, or the description is a file
// nobody reads.
func TestCatalogKeepsWhatWasSaidAboutItsStandardFields(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
standard_attributes:
  - name: Наименование
    title: {ru: Полное имя}
    comment: длиннее обычного
    fill_checking: show-error
    full_text_search: use
    presentation:
      tooltip: {ru: Как товар называется в накладной}
      multi_line: true
  - name: Code
    title: {ru: Артикул}
    type_reduction: transform-values
    presentation:
      format: {ru: "ЧЦ=9"}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("номенклатура")
	if !ok || len(definition.StandardAttributes) != 2 {
		t.Fatalf("standard attributes = %+v, found=%v", definition.StandardAttributes, ok)
	}
	description := definition.StandardAttributes[0]
	if description.Name != "Наименование" || description.Title["ru"] != "Полное имя" ||
		description.Comment != "длиннее обычного" || description.FillChecking != ShowFillingError ||
		description.FullTextSearch != UsageUse || !description.Presentation.MultiLine ||
		description.Presentation.ToolTip["ru"] != "Как товар называется в накладной" {
		t.Fatalf("description of Наименование = %+v", description)
	}
	// The second one is written in English, which is the other name for the
	// same field, and the description of it is the same description.
	if code := definition.StandardAttributes[1]; code.Name != "Code" ||
		code.TypeReduction != TypeReductionTransform || code.Presentation.Format["ru"] != "ЧЦ=9" {
		t.Fatalf("description of Code = %+v", code)
	}
	// A lookup hands out a copy: a caller that edits what it got must not edit
	// the configuration everybody else reads.
	definition.StandardAttributes[0].Title["ru"] = "Изменено"
	again, _ := catalog.CatalogDefinition("Номенклатура")
	if again.StandardAttributes[0].Title["ru"] != "Полное имя" {
		t.Fatal("the catalog handed out its own standard attribute descriptions")
	}
}

// The composition is the platform's. A description of a field this kind of
// object has not got is a setting on nothing, and it is silence that makes it
// dangerous: the developer thinks the format was set and it never was.
func TestStandardFieldOfAnotherKindIsRefused(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]struct {
		kind    Kind
		id      string
		content string
	}{
		"владелец у документа": {DocumentKind, documentID, `format: 1
id: ` + documentID + `
name: РасходнаяНакладная
title: {ru: Расходная накладная}
number: {type: string, length: 9, auto: true, periodicity: year}
standard_attributes:
  - name: Владелец
    title: {ru: Чей}
`},
		"родитель у плана видов расчёта": {ChartOfCalculationTypesKind, calcTypesStandardID, `format: 1
id: ` + calcTypesStandardID + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
code: {type: string, length: 9, auto: true}
description_length: 100
standard_attributes:
  - name: Родитель
    title: {ru: Родитель}
`},
		"наименование у перечисления": {EnumerationKind, enumerationID, `format: 1
id: ` + enumerationID + `
name: СтатусыЗаказов
title: {ru: Статусы заказов}
values:
  - id: ` + enumValueID + `
    name: Новый
    title: {ru: Новый}
standard_attributes:
  - name: Наименование
    title: {ru: Наименование}
`},
		"период у регистра расчёта": {CalculationRegisterKind, calcRegisterStandardID, `format: 1
id: ` + calcRegisterStandardID + `
name: Начисления
title: {ru: Начисления}
periodicity: month
resources:
  - id: ` + calcResourceStandardID + `
    name: Результат
    title: {ru: Результат}
    types: [{kind: number, precision: 15, scale: 2}]
standard_attributes:
  - name: Период
    title: {ru: Период}
`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, body.kind, body.id, body.content)
			_, err := Load(root)
			if err == nil {
				t.Fatal("a description of a field this kind has not got was accepted")
			}
			if !strings.Contains(err.Error(), "not a standard field") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// Одно поле — одно описание. Two of them are two answers to every question the
// description asks, and which one wins is then a matter of order in a file.
func TestOneStandardFieldIsDescribedOnce(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
standard_attributes:
  - name: Наименование
    title: {ru: Одно}
  - name: Description
    title: {ru: Другое}
`)
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "a second time") {
		t.Fatalf("err = %v", err)
	}
}

// The prototype writes Проведен and Предопределенный with е, and a developer
// types either spelling. Refusing one of them would refuse a description of a
// field that plainly exists.
func TestStandardFieldNameIgnoresCaseAndYo(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, DocumentKind, documentID, `format: 1
id: `+documentID+`
name: РасходнаяНакладная
title: {ru: Расходная накладная}
number: {type: string, length: 9, auto: true, periodicity: none}
standard_attributes:
  - name: проведён
    title: {ru: Отгружено}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	document, ok := catalog.DocumentDefinition("расходнаянакладная")
	if !ok || len(document.StandardAttributes) != 1 || document.StandardAttributes[0].Title["ru"] != "Отгружено" {
		t.Fatalf("document = %+v, found=%v", document.StandardAttributes, ok)
	}
}

// Whether a field belongs to folders or to items is the developer's to set on
// an attribute he declared, and not on one the platform gave: the help gives a
// standard field twenty-seven properties and this is not among them.
func TestStandardFieldHasNoFoldersAndItems(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
standard_attributes:
  - name: Наименование
    choice: {folders_and_items: items}
`)
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "folders_and_items") {
		t.Fatalf("err = %v", err)
	}
}

// Indexing a standard field on demand is not the developer's either, and the
// description has nowhere to put it: a file that tries says so at once rather
// than having the setting quietly dropped.
func TestStandardFieldHasNoIndexing(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
standard_attributes:
  - name: Код
    indexing: index
`)
	if _, err := Load(root); err == nil {
		t.Fatal("indexing was accepted on a standard field")
	}
}

// A register of balances tells an increase from a decrease; a register of
// turnovers has only the turnover. The field exists on the one and not on the
// other, and the description follows the register.
func TestKindOfMovementFollowsTheKindOfRegister(t *testing.T) {
	t.Parallel()
	register := func(kind string) string {
		return `format: 1
id: ` + accumulationStandardID + `
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: ` + kind + `
dimensions:
  - id: ` + accumulationDimensionID + `
    name: Номенклатура
    title: {ru: Номенклатура}
    types: [{kind: string, length: 50}]
resources:
  - id: ` + accumulationResourceID + `
    name: Количество
    title: {ru: Количество}
    types: [{kind: number, precision: 15, scale: 3}]
standard_attributes:
  - name: ВидДвижения
    title: {ru: Приход или расход}
`
	}
	t.Run("balance", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, AccumulationRegisterKind, accumulationStandardID, register("balance"))
		catalog, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		definition, ok := catalog.AccumulationRegisterDefinition("остаткитоваров")
		if !ok || len(definition.StandardAttributes) != 1 {
			t.Fatalf("register = %+v, found=%v", definition.StandardAttributes, ok)
		}
	})
	t.Run("turnover", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, AccumulationRegisterKind, accumulationStandardID, register("turnover"))
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "not a standard field") {
			t.Fatalf("err = %v", err)
		}
	})
}

// How many ext dimensions an entry has is the chart's to say, and the chart is
// in another file. A description of one the chart does not allow therefore
// passes the register's own file and has to be caught when the project is read
// whole - which is the only place it can be caught at all.
func TestExtDimensionDescriptionIsBoundedByTheChart(t *testing.T) {
	t.Parallel()
	for name, field := range map[string]struct {
		field   string
		accepts bool
	}{
		"третье субконто чарт разрешает":     {"Субконто3", true},
		"четвёртого субконто у чарта нет":    {"Субконто4", false},
		"вид четвёртого субконто тоже негде": {"ВидСубконто4", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
			writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
ext_dimension_types: `+characteristicsID+`
max_ext_dimension_count: 3
`)
			writeMetadata(t, root, AccountingRegisterKind, accountingStandardID, `format: 1
id: `+accountingStandardID+`
name: Хозрасчетный
title: {ru: Хозрасчётный}
chart_of_accounts: `+accountsID+`
correspondence: true
resources:
  - id: `+accountingResourceID+`
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
standard_attributes:
  - name: `+field.field+`
    title: {ru: Аналитика}
`)
			_, err := Load(root)
			switch {
			case field.accepts && err != nil:
				t.Fatalf("a description of %s the chart allows was refused: %v", field.field, err)
			case !field.accepts && err == nil:
				t.Fatalf("a description of %s the chart does not allow was accepted", field.field)
			}
		})
	}
}

// Under double entry an entry names two accounts and the side is told by which
// of them is filled; without it the entry touches one account and says which
// way by a field of its own. The field is there in the second case only.
func TestKindOfEntryFollowsCorrespondence(t *testing.T) {
	t.Parallel()
	for name, correspondence := range map[string]bool{"без корреспонденции": false, "с корреспонденцией": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
			writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
`)
			writeMetadata(t, root, AccountingRegisterKind, accountingStandardID, `format: 1
id: `+accountingStandardID+`
name: Хозрасчетный
title: {ru: Хозрасчётный}
chart_of_accounts: `+accountsID+`
correspondence: `+boolText(correspondence)+`
resources:
  - id: `+accountingResourceID+`
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
standard_attributes:
  - name: ВидДвижения
    title: {ru: Дебет или кредит}
`)
			_, err := Load(root)
			switch {
			case correspondence && err == nil:
				t.Fatal("the kind of entry was described on a register that tells the side by the accounts")
			case !correspondence && err != nil:
				t.Fatalf("the kind of entry was refused on a register that has one: %v", err)
			}
		})
	}
}

// A standard table part is the platform's too, and it carries descriptions of
// its own line's fields. A chart of accounts has one - the analytics of an
// account - and every one of its four fields may be described.
func TestChartOfAccountsKeepsItsStandardTablePart(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
	writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
ext_dimension_types: `+characteristicsID+`
max_ext_dimension_count: 3
standard_table_parts:
  - name: ВидыСубконто
    title: {ru: Аналитика}
    tooltip: {ru: "Разрезы, по которым ведётся учёт"}
    fill_checking: show-error
    standard_attributes:
      - name: ВидСубконто
        title: {ru: Разрез}
        fill_checking: show-error
      - name: TurnoversOnly
        title: {ru: Только обороты}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	chart, ok := catalog.ChartOfAccounts("основной")
	if !ok || len(chart.StandardTableParts) != 1 {
		t.Fatalf("chart = %+v, found=%v", chart.StandardTableParts, ok)
	}
	part := chart.StandardTableParts[0]
	if part.Title["ru"] != "Аналитика" || part.FillChecking != ShowFillingError ||
		part.ToolTip["ru"] != "Разрезы, по которым ведётся учёт" || len(part.StandardAttributes) != 2 ||
		part.StandardAttributes[1].Title["ru"] != "Только обороты" {
		t.Fatalf("standard table part = %+v", part)
	}
	// And the descriptions inside it are copies as well.
	chart.StandardTableParts[0].StandardAttributes[0].Title["ru"] = "Изменено"
	again, _ := catalog.ChartOfAccounts("Основной")
	if again.StandardTableParts[0].StandardAttributes[0].Title["ru"] != "Разрез" {
		t.Fatal("the chart handed out its own standard table part descriptions")
	}
}

// The three lists of competition are the chart of calculation types' own
// standard parts, and the field inside them is the calculation type they name -
// not, say, the kind of analytics, which belongs to another chart entirely.
func TestChartOfCalculationTypesKeepsItsStandardTableParts(t *testing.T) {
	t.Parallel()
	body := func(part, field string) string {
		return `format: 1
id: ` + calcTypesStandardID + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
code: {type: string, length: 9, auto: true}
description_length: 100
action_period_use: true
standard_table_parts:
  - name: ` + part + `
    title: {ru: Конкуренция}
    standard_attributes:
      - name: ` + field + `
        title: {ru: Вид}
`
	}
	for name, content := range map[string]struct {
		body    string
		accepts bool
	}{
		"вытесняющие по-русски":         {body("ВытесняющиеВидыРасчета", "ВидРасчета"), true},
		"базовые по-английски":          {body("BaseCalculationTypes", "CalculationType"), true},
		"вид субконто здесь ни при чём": {body("ВедущиеВидыРасчета", "ВидСубконто"), false},
		"такой части у плана нет":       {body("ВидыСубконто", "ВидСубконто"), false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ChartOfCalculationTypesKind, calcTypesStandardID, content.body)
			_, err := Load(root)
			switch {
			case content.accepts && err != nil:
				t.Fatalf("a standard part the chart has was refused: %v", err)
			case !content.accepts && err == nil:
				t.Fatal("a standard part the chart has not got was accepted")
			}
		})
	}
}

// A catalog has no standard table part at all, and the key that would describe
// one is refused rather than read and ignored.
func TestCatalogHasNoStandardTablePart(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
standard_table_parts:
  - name: ВидыСубконто
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a standard table part was accepted on a catalog")
	}
}

// A line of an ordinary table part is given one field, its number, and the
// prototype describes that one on a hundred and sixty parts. Anything else
// named there is a field the line has not got.
func TestTablePartLineNumberIsTheOnlyStandardFieldOfAPart(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]struct {
		field   string
		accepts bool
	}{
		"номер строки":        {"НомерСтроки", true},
		"он же по-английски":  {"LineNumber", true},
		"ссылки у строки нет": {"Ссылка", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, catalogID, `format: 1
id: `+catalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
table_parts:
  - id: `+tablePartID+`
    name: Состав
    title: {ru: Состав}
    attributes:
      - id: `+partFieldID+`
        name: Материал
        title: {ru: Материал}
        types: [{kind: string, length: 50}]
    standard_attributes:
      - name: `+content.field+`
        title: {ru: Номер}
`)
			_, err := Load(root)
			switch {
			case content.accepts && err != nil:
				t.Fatalf("the number of a line was refused: %v", err)
			case !content.accepts && err == nil:
				t.Fatalf("%s was accepted as a standard field of a line", content.field)
			}
		})
	}
}

// The choice of a standard field may be narrowed by another field of the same
// object, and a link to a field that is not there narrows nothing: the user
// picks out of everything with no sign that anything was meant to narrow it.
func TestStandardFieldChoiceLinkResolvesWithinTheObject(t *testing.T) {
	t.Parallel()
	body := func(source string) string {
		return `format: 1
id: ` + catalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: ` + attributeID + `
    name: Поставщик
    title: {ru: Поставщик}
    types: [{kind: string, length: 50}]
standard_attributes:
  - name: Наименование
    choice:
      parameter_links:
        - name: Отбор.Поставщик
          source: {attribute: ` + source + `}
`
	}
	t.Run("на поле этого объекта", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, catalogID, body(attributeID))
		if _, err := Load(root); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("на поле, которого нет", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, catalogID, body("40000000-0000-4000-8000-0000000000ff"))
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "not a field of this object") {
			t.Fatalf("err = %v", err)
		}
	})
}

// Every standard field is a name the platform has taken. An application field
// allowed to take the same name would be a second thing answering to it, and
// the description of the standard one would then be a setting on whichever of
// the two the reader happened to mean. The composition and the reserved names
// are therefore one answer, and this is what says so.
func TestEveryStandardFieldNameIsReservedForItsKind(t *testing.T) {
	t.Parallel()
	for kind, reserved := range map[Kind]func(string) bool{
		CatalogKind:                    reservedCatalogObjectName,
		ChartOfCharacteristicTypesKind: reservedChartOfCharacteristicTypesName,
		ChartOfAccountsKind:            reservedChartOfAccountsName,
		ChartOfCalculationTypesKind:    reservedCalculationTypeName,
		ExchangePlanKind:               reservedExchangePlanName,
		DocumentKind:                   reservedDocumentObjectName,
		BusinessProcessKind:            reservedBusinessProcessName,
		TaskKind:                       reservedTaskName,
		InformationRegisterKind:        reservedInformationRegisterName,
		AccumulationRegisterKind:       reservedAccumulationRegisterName,
		AccountingRegisterKind:         reservedAccountingRegisterName,
		CalculationRegisterKind:        reservedCalculationRegisterName,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			fields := standardFieldsOfKind(kind)
			if len(fields) == 0 {
				t.Fatal("a kind with no standard fields at all is not a kind we know")
			}
			for _, field := range fields {
				for _, name := range []string{field.ru, field.en, strings.ToUpper(field.ru), strings.ToLower(field.en)} {
					if !reserved(name) {
						t.Errorf("%s is a standard field of a %s and an application field may still take the name", name, kind)
					}
				}
			}
		})
	}
}

// StandardAttributeNames is how everything outside this package asks what
// fields an object has without declaring them - the importer of a real
// configuration, and the editor offering them in a form. The answer depends on
// the object where the prototype makes it depend on the object, and that is
// what a caller cannot work out for itself.
func TestStandardAttributeNamesAnswerForTheObject(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
	writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
ext_dimension_types: `+characteristicsID+`
max_ext_dimension_count: 2
`)
	writeMetadata(t, root, AccountingRegisterKind, accountingStandardID, `format: 1
id: `+accountingStandardID+`
name: Хозрасчетный
title: {ru: Хозрасчётный}
chart_of_accounts: `+accountsID+`
correspondence: true
resources:
  - id: `+accountingResourceID+`
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
`)
	writeMetadata(t, root, AccumulationRegisterKind, accumulationStandardID, `format: 1
id: `+accumulationStandardID+`
name: ОборотыТоваров
title: {ru: Обороты товаров}
kind: turnover
dimensions:
  - id: `+accumulationDimensionID+`
    name: Номенклатура
    title: {ru: Номенклатура}
    types: [{kind: string, length: 50}]
resources:
  - id: `+accumulationResourceID+`
    name: Количество
    title: {ru: Количество}
    types: [{kind: number, precision: 15, scale: 3}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.AccountingRegister("хозрасчетный")
	if !ok {
		t.Fatal("the accounting register was not read")
	}
	// Two ext dimensions because the chart allows two, and no kind of entry
	// because the side is told by the two accounts.
	if names := catalog.StandardAttributeNames(AccountingRegisterKind, register.ID); !slices.Equal(names, []string{
		"Период", "Регистратор", "НомерСтроки", "Активность", "Счет",
		"Субконто1", "ВидСубконто1", "Субконто2", "ВидСубконто2",
	}) {
		t.Fatalf("accounting register fields = %v", names)
	}
	turnovers, ok := catalog.AccumulationRegisterDefinition("оборотытоваров")
	if !ok {
		t.Fatal("the accumulation register was not read")
	}
	if names := catalog.StandardAttributeNames(AccumulationRegisterKind, turnovers.ID); !slices.Equal(names,
		[]string{"Период", "Регистратор", "НомерСтроки", "Активность"}) {
		t.Fatalf("register of turnovers fields = %v", names)
	}
	// A kind whose set does not depend on the object answers without one.
	if names := catalog.StandardAttributeNames(EnumerationKind, uuid.UUID{}); !slices.Equal(names, []string{"Ссылка", "Порядок"}) {
		t.Fatalf("enumeration fields = %v", names)
	}
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

const (
	calcTypesStandardID     = "90000000-0000-4000-8000-000000000101"
	calcRegisterStandardID  = "90000000-0000-4000-8000-000000000102"
	calcResourceStandardID  = "90000000-0000-4000-8000-000000000103"
	accumulationStandardID  = "90000000-0000-4000-8000-000000000104"
	accumulationDimensionID = "90000000-0000-4000-8000-000000000105"
	accumulationResourceID  = "90000000-0000-4000-8000-000000000106"
	accountingStandardID    = "90000000-0000-4000-8000-000000000107"
	accountingResourceID    = "90000000-0000-4000-8000-000000000108"
)
