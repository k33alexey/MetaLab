package metadata

import (
	"strings"
	"testing"
)

const (
	presentationCatalog  = "c8000000-0000-4000-8000-000000000001"
	presentationPartners = "c8000000-0000-4000-8000-000000000002"
	presentationAmount   = "c8000000-0000-4000-8000-000000000010"
	presentationSecret   = "c8000000-0000-4000-8000-000000000011"
	presentationPartner  = "c8000000-0000-4000-8000-000000000012"
	presentationContract = "c8000000-0000-4000-8000-000000000013"
	presentationPart     = "c8000000-0000-4000-8000-000000000020"
	presentationLine     = "c8000000-0000-4000-8000-000000000021"
	presentationMissing  = "c8000000-0000-4000-8000-0000000000ff"
)

// presentationOrder is a catalog whose fields carry the whole set: a number
// shown one way and entered another, a password, and a field picked from a
// list narrowed by the field above it.
func presentationOrder(attributes string) string {
	return `format: 1
id: ` + presentationCatalog + `
name: Заказы
title: {ru: Заказы}
code: {type: string, length: 9, auto: true}
description_length: 150
` + attributes
}

func TestFieldKeepsHowItIsShownAndPicked(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, presentationPartners, "Партнеры")
	writeMetadata(t, root, CatalogKind, presentationCatalog, presentationOrder(`attributes:
  - id: `+presentationAmount+`
    name: Сумма
    title: {ru: Сумма}
    comment: Сумма заказа с налогами
    types: [{kind: number, precision: 15, scale: 2}]
    presentation:
      format: {ru: "ЧДЦ=2"}
      edit_format: {ru: "ЧДЦ=2; ЧН=0"}
      tooltip: {ru: Сумма заказа}
      mark_negatives: true
      min_value: "0"
      max_value: "1000000"
  - id: `+presentationSecret+`
    name: Пароль
    title: {ru: Пароль}
    types: [{kind: string, length: 100}]
    presentation: {mask: "999-999", password: true, multi_line: false, extended_edit: true}
  - id: `+presentationPartner+`
    name: Партнер
    title: {ru: Партнёр}
    types: [{kind: catalog, reference: `+presentationPartners+`}]
    choice:
      quick_choice: use
      create_on_input: dont-use
      history_on_input: auto
      folders_and_items: items
      parameters:
        - {name: Отбор.Действует, values: [{kind: boolean, data: "true"}]}
  - id: `+presentationContract+`
    name: Договор
    title: {ru: Договор}
    types: [{kind: catalog, reference: `+presentationPartners+`}]
    choice:
      parameter_links:
        - {name: Отбор.Владелец, source: {attribute: `+presentationPartner+`}, change: clear}
      link_by_type: {source: {attribute: `+presentationPartner+`}, item: 0}
table_parts:
  - id: `+presentationPart+`
    name: Строки
    title: {ru: Строки}
    attributes:
      - id: `+presentationLine+`
        name: Номенклатура
        title: {ru: Номенклатура}
        types: [{kind: catalog, reference: `+presentationPartners+`}]
        choice:
          parameter_links:
            - {name: Отбор.Владелец, source: {attribute: `+presentationPartner+`}}
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	orders, ok := catalog.CatalogDefinition("Заказы")
	if !ok || len(orders.Attributes) != 4 {
		t.Fatalf("the catalog did not load: found=%v %+v", ok, orders.Attributes)
	}
	amount, secret, partner, contract := orders.Attributes[0], orders.Attributes[1], orders.Attributes[2], orders.Attributes[3]
	switch {
	case amount.Comment == "":
		t.Fatalf("the comment was lost: %+v", amount)
	case len(amount.Presentation.Format) == 0 || len(amount.Presentation.EditFormat) == 0:
		t.Fatalf("how the value is shown and how it is entered are two settings: %+v", amount.Presentation)
	case len(amount.Presentation.ToolTip) == 0 || !amount.Presentation.MarkNegatives:
		t.Fatalf("the presentation was lost: %+v", amount.Presentation)
	case amount.Presentation.MinValue == nil || amount.Presentation.MaxValue == nil:
		t.Fatalf("the bounds were lost: %+v", amount.Presentation)
	case secret.Presentation.Mask != "999-999" || !secret.Presentation.Password || !secret.Presentation.ExtendedEdit:
		t.Fatalf("how the field is entered was lost: %+v", secret.Presentation)
	case partner.Choice.QuickChoice != UsageUse || partner.Choice.CreateOnInput != UsageDontUse:
		t.Fatalf("the three-valued switches were lost: %+v", partner.Choice)
	case partner.Choice.FoldersAndItems != ChoiceTargetItems:
		t.Fatalf("what a choice may land on was lost: %+v", partner.Choice)
	case len(partner.Choice.Parameters) != 1 || len(partner.Choice.Parameters[0].Values) != 1:
		t.Fatalf("the fixed choice parameter was lost: %+v", partner.Choice)
	case len(contract.Choice.ParameterLinks) != 1 || contract.Choice.ParameterLinks[0].Change != ValueChangeClear:
		t.Fatalf("the link to another field was lost: %+v", contract.Choice)
	case contract.Choice.LinkByType == nil || contract.Choice.LinkByType.Source.Attribute.String() != presentationPartner:
		t.Fatalf("the link by type was lost: %+v", contract.Choice)
	case len(orders.TableParts[0].Attributes[0].Choice.ParameterLinks) != 1:
		t.Fatalf("a field of a table part lost its link: %+v", orders.TableParts[0].Attributes[0])
	}

	*orders.Attributes[0].Presentation.MinValue = "999"
	orders.Attributes[3].Choice.ParameterLinks[0].Name = "Другое"
	again, _ := catalog.CatalogDefinition("Заказы")
	if *again.Attributes[0].Presentation.MinValue != "0" || again.Attributes[3].Choice.ParameterLinks[0].Name != "Отбор.Владелец" {
		t.Fatal("a field's settings were handed out by reference")
	}
}

// A link takes a choice parameter from another field of the same object. One
// pointing at a field that is not there narrows nothing, and the user picks
// out of everything with nothing said about it.
func TestFieldLinksMustPointAtAFieldOfTheObject(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"связь на несуществующий реквизит": {`attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 50}]
    choice:
      parameter_links: [{name: Отбор.Владелец, source: {attribute: ` + presentationMissing + `}}]`,
			"parameter_links[0].source is not a field of this object"},
		"связь на чужую табличную часть": {`attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 50}]
    choice:
      parameter_links:
        - {name: Отбор.Владелец, source: {table_part: ` + presentationMissing + `, attribute: ` + presentationLine + `}}
table_parts:
  - id: ` + presentationPart + `
    name: Строки
    title: {ru: Строки}
    attributes:
      - {id: ` + presentationLine + `, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: string, length: 50}]}`,
			"is not a field of this object"},
		"связь по типу в никуда": {`attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 50}]
    choice:
      link_by_type: {source: {attribute: ` + presentationMissing + `}}`,
			"link_by_type.source is not a field of this object"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(broken.body+"\n")), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The settings of a field are checked where the field is written down.
func TestBrokenFieldSettingsAreRefused(t *testing.T) {
	t.Parallel()
	field := func(settings string) string {
		return `attributes:
  - id: ` + presentationAmount + `
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
` + settings
	}
	for name, broken := range map[string]struct{ body, want string }{
		"переключатель не из трёх": {field(`    choice: {quick_choice: maybe}`),
			"must be auto, use or dont-use"},
		"выбор не из трёх": {field(`    choice: {folders_and_items: anything}`),
			"must be items, folders or folders-and-items"},
		// The prototype has no "auto" here and no "use" in the history of
		// choice: values a reader of the help would not expect, and an import
		// would never produce.
		"авто в выборе групп и элементов": {field(`    choice: {folders_and_items: auto}`),
			"must be items, folders or folders-and-items"},
		"использовать в истории выбора": {field(`    choice: {history_on_input: use}`),
			"history_on_input must be auto or dont-use"},
		"параметр без значения": {field(`    choice: {parameters: [{name: Отбор.Вид, values: []}]}`),
			"values must contain at least one value"},
		"несколько значений молча": {field(`    choice:
      parameters:
        - name: Отбор.Вид
          values: [{kind: number, data: "1"}, {kind: number, data: "2"}]`),
			"must say it is a list"},
		"параметр задан и связан разом": {field(`    choice:
      parameters: [{name: Отбор.Вид, values: [{kind: number, data: "1"}]}]
      parameter_links: [{name: отбор.вид, source: {attribute: ` + presentationAmount + `}}]`),
			"is already set or linked"},
		"связь меняется неизвестно как": {field(`    choice:
      parameter_links: [{name: Отбор.Вид, source: {attribute: ` + presentationAmount + `}, change: erase}]`),
			"must be clear or dont-change"},
		"форма выбора без объекта": {field(`    choice: {form: {kind: catalogs, name: ФормаВыбора}}`),
			"object must be set beside the kind"},
		"форма выбора без вида": {field(`    choice: {form: {object: ` + presentationPartners + `, name: ФормаВыбора}}`),
			"kind must be set beside the object"},
		// A choice form belongs to an object that has forms. Defect caught:
		// any kind the model knew was accepted - a constant, a common module,
		// a role.
		"форма выбора у константы": {field(`    choice: {form: {kind: constants, object: ` + presentationPartners + `, name: ФормаВыбора}}`),
			"kind must be a kind of object that has forms"},
		"форма выбора у общего модуля": {field(`    choice: {form: {kind: common-modules, object: ` + presentationPartners + `, name: ФормаВыбора}}`),
			"kind must be a kind of object that has forms"},
		"форма выбора у роли": {field(`    choice: {form: {kind: roles, object: ` + presentationPartners + `, name: ФормаВыбора}}`),
			"kind must be a kind of object that has forms"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(broken.body+"\n")), metadataConfiguration())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}

	// Every kind with forms of its own may hold a choice form; the
	// configurations being moved use catalogs, documents, charts of accounts
	// and an exchange plan.
	for _, kind := range []Kind{CatalogKind, DocumentKind, EnumerationKind, DocumentJournalKind,
		ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind,
		ExchangePlanKind, BusinessProcessKind, TaskKind,
		InformationRegisterKind, AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind,
		ReportKind, DataProcessorKind, FilterCriterionKind, SettingsStorageKind,
		ExternalDataSourceTableKind, ExternalCubeKind, ExternalDimensionTableKind} {
		if _, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(field(`    choice: {form: {kind: `+string(kind)+`, object: `+presentationPartners+`, name: ФормаВыбора}}`)+"\n")), metadataConfiguration()); err != nil {
			t.Fatalf("a choice form of %s was refused: %v", kind, err)
		}
	}

	// A bound is not checked against a description the configuration may widen:
	// what a set holds is not written down in the field, so nothing here can
	// say whether the bound fits.
	_, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(`attributes:
  - id: `+presentationAmount+`
    name: Ссылка2
    title: {ru: Ссылка}
    types: [{kind: catalog-ref}]
    presentation: {min_value: "0"}
`)), metadataConfiguration())
	if err != nil {
		t.Fatalf("a bound beside an open type description was refused: %v", err)
	}
}

// A bound is carried as written on a field of any type - the configurations
// being moved keep them on booleans, strings, dates and references, and write
// them with a point or a comma - and an empty reference stays the filling of
// a string field it was left on. The defects caught: such a field refused,
// which turns away what the prototype saves, and the text changed on the way.
func TestFieldCarriesBoundsAndFillingTheFieldDoesNotHold(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, presentationPartners, "Партнеры")
	writeMetadata(t, root, CatalogKind, presentationCatalog, presentationOrder(`attributes:
  - id: `+presentationAmount+`
    name: Признак
    title: {ru: Признак}
    types: [{kind: boolean}]
    presentation: {min_value: "0", max_value: "1"}
  - id: `+presentationSecret+`
    name: Комментарий
    title: {ru: Комментарий}
    types: [{kind: string, length: 100}]
    presentation: {min_value: "0,5"}
    filling: {value: {kind: catalog, data: "00000000-0000-0000-0000-000000000000", object: `+presentationPartners+`}}
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a field with a bound or a filling of another type was refused: %v", err)
	}
	orders, _ := catalog.CatalogDefinition("Заказы")
	flag, text := orders.Attributes[0], orders.Attributes[1]
	if flag.Presentation.MinValue == nil || *flag.Presentation.MinValue != "0" || *flag.Presentation.MaxValue != "1" {
		t.Fatalf("the bounds of a boolean were not kept as written: %+v", flag.Presentation)
	}
	if text.Presentation.MinValue == nil || *text.Presentation.MinValue != "0,5" {
		t.Fatalf("the bound of a string lost its comma: %+v", text.Presentation)
	}
	if text.Filling.Value == nil || text.Filling.Value.Kind != CatalogType {
		t.Fatalf("the filling the field does not hold was not kept: %+v", text.Filling)
	}
}

// A bound bounds a field of one number only, read with a point or a comma; a
// filling value fills only a field that holds it. Each case is the answer the
// running application gets, so a rule that bounds a string, misreads a comma,
// bounds a composite field or fills a string with a reference is caught.
func TestOnlyANumberIsBoundedAndOnlyWhatTheFieldHoldsFillsIt(t *testing.T) {
	t.Parallel()
	text := func(value string) *string { return &value }
	number := []Type{{Kind: NumberType}}
	for _, test := range []struct {
		name  string
		bound *string
		types []Type
		want  string
		ok    bool
	}{
		{"целое у числа", text("0"), number, "0", true},
		{"запятая у числа", text("0,5"), number, "0.5", true},
		{"точка у числа", text("-12.75"), number, "-12.75", true},
		{"не число у числа", text("abc"), number, "", false},
		{"у строки", text("0"), []Type{{Kind: StringType}}, "", false},
		{"у булева", text("1"), []Type{{Kind: BooleanType}}, "", false},
		{"у составного", text("0"), []Type{{Kind: NumberType}, {Kind: StringType}}, "", false},
		{"нет границы", nil, number, "", false},
	} {
		got, ok := NumberBound(test.bound, test.types)
		if got != test.want || ok != test.ok {
			t.Errorf("%s: NumberBound = %q %v, want %q %v", test.name, got, ok, test.want, test.ok)
		}
	}
	reference := &Value{Kind: CatalogType, Data: "00000000-0000-0000-0000-000000000000"}
	if EffectiveFillingValue(reference, []Type{{Kind: StringType}}) != nil {
		t.Error("a reference fills a string field")
	}
	if filled := EffectiveFillingValue(reference, []Type{{Kind: CatalogType}}); filled == nil || filled.Kind != CatalogType {
		t.Error("a reference does not fill a reference field")
	}
	if EffectiveFillingValue(nil, []Type{{Kind: StringType}}) != nil {
		t.Error("nothing fills a field with something")
	}
}

// A link takes its value from a standard field by name: the date of a
// document, the reference or the owner of a catalog item, the line number of
// a row. The configurations being moved do it 419 times. The defects caught on
// the accepting side are such a link refused - a standard field has no
// identifier to write; on the refusing side, a name the object does not have
// (a catalog has no date), a row's field other than its line number, and a
// path that names a field two ways or none.
func TestLinkTakesItsValueFromAStandardField(t *testing.T) {
	t.Parallel()
	catalogLink := func(source string) string {
		return presentationOrder(`attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 10}]
    choice:
      parameter_links: [{name: Отбор.Владелец, source: ` + source + `}]
table_parts:
  - id: ` + presentationPart + `
    name: Товары
    title: {ru: Товары}
    attributes:
      - id: ` + presentationLine + `
        name: Номер
        title: {ru: Номер}
        types: [{kind: string, length: 10}]
        choice: {link_by_type: {source: ` + source + `}}
`)
	}
	documentLink := func(source string) string {
		return `format: 1
id: ` + presentationCatalog + `
name: Заказ
title: {ru: Заказ}
number: {type: string, length: 9, auto: true, periodicity: none}
attributes:
  - id: ` + presentationContract + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 10}]
    choice: {parameter_links: [{name: Отбор.Дата, source: ` + source + `}]}
`
	}
	decodeCatalog := func(body string) error {
		_, err := DecodeCatalog("object.yaml", strings.NewReader(body), metadataConfiguration())
		return err
	}
	decodeDocument := func(body string) error {
		_, err := DecodeDocument("object.yaml", strings.NewReader(body), metadataConfiguration())
		return err
	}
	part := "table_part: " + presentationPart + ", "
	for name, test := range map[string]struct {
		err  error
		want string
	}{
		"ссылка справочника":            {decodeCatalog(catalogLink(`{standard: Ссылка}`)), ""},
		"ссылка по-английски":           {decodeCatalog(catalogLink(`{standard: Ref}`)), ""},
		"пометка удаления":              {decodeCatalog(catalogLink(`{standard: ПометкаУдаления}`)), ""},
		"номер строки табличной части":  {decodeCatalog(catalogLink(`{` + part + `standard: НомерСтроки}`)), ""},
		"дата документа":                {decodeDocument(documentLink(`{standard: Дата}`)), ""},
		"у справочника нет даты":        {decodeCatalog(catalogLink(`{standard: Дата}`)), "is not a field of this object"},
		"незнакомое имя":                {decodeDocument(documentLink(`{standard: Ниоткуда}`)), "is not a field of this object"},
		"у строки нет ссылки":           {decodeCatalog(catalogLink(`{` + part + `standard: Ссылка}`)), "is not a field of this object"},
		"и реквизит, и стандартный":     {decodeCatalog(catalogLink(`{attribute: ` + presentationLine + `, standard: Ссылка}`)), "names both an attribute and a standard field"},
		"ни реквизита, ни стандартного": {decodeCatalog(catalogLink(`{}`)), "must be a non-zero UUID, or source.standard"},
	} {
		switch {
		case test.want == "" && test.err != nil:
			t.Errorf("%s: refused: %v", name, test.err)
		case test.want != "" && test.err == nil:
			t.Errorf("%s: accepted", name)
		case test.want != "" && !strings.Contains(test.err.Error(), test.want):
			t.Errorf("%s: refused for another reason: %v", name, test.err)
		}
	}
}

// A link the prototype saves leading out of the object - a field of another
// object by its identifier, or a bare number - is carried word for word and
// filters nothing. The configurations being moved have 1468 and 202 of them,
// and every one was refused, which refuses the whole configuration. The
// defects caught: such a path refused, its text changed on the way, and a
// path that is not one the prototype writes, or that names a field beside
// the kept text, let through.
func TestLinkLeadingOutOfTheObjectIsCarriedAsWritten(t *testing.T) {
	t.Parallel()
	const foreignPart, foreignField = "7b526229-3286-418a-920a-ff15a9b3ec48", "da473af6-1960-4683-b941-b8e3988be4fb"
	body := func(source string) string {
		return presentationOrder(`attributes:
  - id: ` + presentationContract + `
    name: Номенклатура
    title: {ru: Номенклатура}
    types: [{kind: string, length: 10}]
    choice:
      parameter_links: [{name: Отбор.Владелец, source: ` + source + `}]
      link_by_type: {source: ` + source + `}
`)
	}
	for name, test := range map[string]struct {
		path, want string
	}{
		"поле чужого объекта":                 {`"0:` + foreignField + `"`, ""},
		"поле табличной части чужого объекта": {`"0:` + foreignPart + `/0:` + foreignField + `"`, ""},
		"ноль":                    {`"0"`, ""},
		"отрицательное число":     {`"-5"`, ""},
		"ноль через косую черту":  {`"0/0"`, ""},
		"табличная часть и число": {`"0:` + foreignPart + `/0"`, ""},
		"не путь прототипа":       {`"Документ.Заказ"`, "must be a path the prototype writes"},
		"не идентификатор":        {`"0:abc"`, "must be a path the prototype writes"},
		"пустой шаг":              {`"0//0"`, "must be a path the prototype writes"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value, err := DecodeCatalog("object.yaml", strings.NewReader(body(`{unresolved: `+test.path+`}`)), metadataConfiguration())
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("want a refusal saying %q, got %v", test.want, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("a path the prototype writes was refused: %v", err)
			}
			written := strings.Trim(test.path, `"`)
			choice := value.Attributes[0].Choice
			if choice.ParameterLinks[0].Source.Unresolved != written || choice.LinkByType.Source.Unresolved != written {
				t.Fatalf("the path did not come back word for word: %+v %+v", choice.ParameterLinks[0].Source, choice.LinkByType.Source)
			}
		})
	}
	t.Run("путь и поле рядом", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeCatalog("object.yaml", strings.NewReader(body(`{unresolved: "0", attribute: `+presentationContract+`}`)), metadataConfiguration())
		if err == nil || !strings.Contains(err.Error(), "names no field beside it") {
			t.Fatalf("a kept path with a field beside it: %v", err)
		}
	})
}

// Неопределено and a reference to a type nothing has are values the prototype
// writes at design time - a choice parameter set to Неопределено 145 times,
// a filling value naming a vanished type 212 times - and each has one way to
// be written. The defects caught: either refused; a value with no kind let
// through, which a reader cannot tell from Неопределено; Неопределено with
// data; and a kept reference with its text lost. And what neither fills: a
// field is filled by neither, whatever its types.
func TestUndefinedAndUnresolvedReferenceAreValuesOfTheirOwn(t *testing.T) {
	t.Parallel()
	const unresolved = "466cbe70-c94c-4cdc-a0fb-f9f9084bdef2.00000000-0000-0000-0000-000000000000"
	body := func(parameter, filling string) string {
		return presentationOrder(`attributes:
  - id: ` + presentationContract + `
    name: Счет
    title: {ru: Счёт}
    types: [{kind: string, length: 10}]
    choice:
      parameters: [{name: ВыборСчетовГоловнойОрганизации, values: [` + parameter + `]}]
    filling: {value: ` + filling + `}
`)
	}
	for name, test := range map[string]struct {
		parameter, filling, want string
	}{
		"неопределено и неразрешённая ссылка": {`{kind: undefined, data: ""}`, `{kind: unresolved-reference, data: "` + unresolved + `"}`, ""},
		"вид не назван":          {`{kind: "", data: ""}`, `{kind: string, data: ""}`, "Неопределено is kind undefined"},
		"неопределено с данными": {`{kind: undefined, data: "x"}`, `{kind: string, data: ""}`, "is undefined and carries nothing"},
		"ссылка без текста":      {`{kind: undefined, data: ""}`, `{kind: unresolved-reference, data: ""}`, "must keep the reference as the prototype wrote it"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value, err := DecodeCatalog("object.yaml", strings.NewReader(body(test.parameter, test.filling)), metadataConfiguration())
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("want a refusal saying %q, got %v", test.want, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("a value the prototype writes was refused: %v", err)
			}
			field := value.Attributes[0]
			if got := field.Choice.Parameters[0].Values[0]; got.Kind != UndefinedValue {
				t.Fatalf("Неопределено came back as %+v", got)
			}
			if got := field.Filling.Value; got == nil || got.Kind != UnresolvedReferenceValue || got.Data != unresolved {
				t.Fatalf("the reference did not come back word for word: %+v", got)
			}
		})
	}
	for _, kind := range []TypeKind{UndefinedValue, UnresolvedReferenceValue} {
		for _, types := range [][]Type{{{Kind: StringType}}, {{Kind: CatalogType}, {Kind: StringType}}} {
			if EffectiveFillingValue(&Value{Kind: kind, Data: "x"}, types) != nil {
				t.Errorf("%s fills a field of %v", kind, types)
			}
		}
	}
}

// The name of a choice parameter has no ceiling in the help; the 256 once
// refused here was nobody's. The values the prototype does write - history
// auto or dont-use, a choice of folders, items or both - all load, on a field
// and on the object.
func TestChoiceSettingsTakeWhatThePrototypeWrites(t *testing.T) {
	t.Parallel()
	long := "Отбор." + strings.Repeat("Реквизит", 40)
	for name, choice := range map[string]string{
		"длинное имя параметра":   `{parameters: [{name: ` + long + `, values: [{kind: boolean, data: "true"}]}]}`,
		"история авто":            `{history_on_input: auto}`,
		"история не использовать": `{history_on_input: dont-use}`,
		"группы":            `{folders_and_items: folders}`,
		"группы и элементы": `{folders_and_items: folders-and-items}`,
		"элементы":          `{folders_and_items: items}`,
	} {
		_, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder(`attributes:
  - id: `+presentationContract+`
    name: Договор
    title: {ru: Договор}
    types: [{kind: string, length: 10}]
    choice: `+choice+`
`)), metadataConfiguration())
		if err != nil {
			t.Errorf("%s: refused: %v", name, err)
		}
	}
	if _, err := DecodeCatalog("object.yaml", strings.NewReader(presentationOrder("choice_history_on_input: use\n")), metadataConfiguration()); err == nil {
		t.Error("an object's history of choice took a value the prototype does not have")
	}
}
