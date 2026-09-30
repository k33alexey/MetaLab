package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A resource of an accounting register keeps a structure of its own - it has
// the ext dimension flag no dimension has - and it has to carry the common set
// all the same. The defect this catches is the one it had while a field of
// this register was a struct apart: a name, a type and indexing and nothing
// else, so a resource could not be given a format, a mark for negatives or a
// bound at all. The dimensions of the register are the dimension of every
// register - see TestEveryRegisterDimensionCarriesTheCommonSetAndItsOwn.
func TestAccountingRegisterResourceCarriesTheCommonSet(t *testing.T) {
	t.Parallel()
	fields := `dimensions:
  - id: ` + entriesCompany + `
    name: Организация
    title: {ru: Организация}
    types: [{kind: catalog, reference: ` + entriesCompanies + `}]
    comment: Организация проводки
    balance: true
    indexing: index
    fill_checking: show-error
    full_text_search: use
    presentation:
      tooltip: {ru: Организация проводки}
    choice:
      quick_choice: use
      folders_and_items: items
      parameters:
        - {name: Отбор.Действует, values: [{kind: boolean, data: "true"}]}
resources:
  - id: ` + entriesSum + `
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
    balance: true
    presentation:
      format: {ru: "ЧДЦ=2"}
      mark_negatives: true
      min_value: {kind: number, data: "0"}
  - id: ` + entriesFxSum + `
    name: ВалютнаяСумма
    title: {ru: Валютная сумма}
    types: [{kind: number, precision: 15, scale: 2}]
    ext_dimension_accounting_flag: ` + entriesExtFlag
	catalog, err := Load(entriesProject(t, true, fields))
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.AccountingRegister("Хозрасчетный")
	if !ok {
		t.Fatal("the accounting register did not load")
	}
	resource := register.Resources[0]
	if resource.Presentation.Format["ru"] != "ЧДЦ=2" || !resource.Presentation.MarkNegatives {
		t.Fatalf("resource presentation = %+v", resource.Presentation)
	}
	if resource.Presentation.MinValue == nil || resource.Presentation.MinValue.Data != "0" {
		t.Fatalf("resource min value = %+v", resource.Presentation.MinValue)
	}
	if flag := register.Resources[1].ExtDimensionAccountingFlag; flag == nil {
		t.Fatal("the resource lost its ext dimension flag")
	}

	// A copy shares nothing with the original: the register comes out of the
	// catalog cloned, and a caller that changes what it got must not change
	// what the next caller gets.
	again, _ := catalog.AccountingRegister("Хозрасчетный")
	again.Dimensions[0].Presentation.ToolTip["ru"] = "changed"
	again.Dimensions[0].Choice.Parameters[0].Name = "changed"
	fresh, _ := catalog.AccountingRegister("Хозрасчетный")
	if fresh.Dimensions[0].Presentation.ToolTip["ru"] != "Организация проводки" || fresh.Dimensions[0].Choice.Parameters[0].Name != "Отбор.Действует" {
		t.Fatal("the copy shared its presentation or its choice with the catalog")
	}
}

// Now that a field of a register carries the common set, the settings inside it
// have to be checked like any other field's. Before this they were not looked
// at for dimensions and resources at all, so a bound of another type than the
// field and a choice parameter with no name went in silently - and a bound that
// is compared with nothing rejects nothing, which is a field that accepts what
// it was set up to refuse.
func TestAccountingRegisterFieldRefusesWrongSettings(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ fields, message string }{
		"граница другого типа": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}]}
resources:
  - id: ` + entriesSum + `
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
    presentation: {min_value: {kind: string, data: "0"}}`, "min_value"},
		"параметр выбора без имени": {`dimensions:
  - id: ` + entriesCompany + `
    name: Организация
    title: {ru: Организация}
    types: [{kind: catalog, reference: ` + entriesCompanies + `}]
    choice:
      parameters:
        - {name: "", values: [{kind: boolean, data: "true"}]}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`, "choice"},
		"неизвестный режим проверки заполнения": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], fill_checking: сомневаться}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`, "fill_checking"},
		// ПризнакУчетаСубконто belongs to the resources of an accounting
		// register only. A dimension holds no amount, so there is nothing for
		// it to keep by ext dimension - and since a dimension is now the
		// dimension of every register, which has no such property at all, the
		// strict reader refuses the word before any check sees it.
		"признак учёта субконто у измерения": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], ext_dimension_accounting_flag: ` + entriesExtFlag + `}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`, "field ext_dimension_accounting_flag not found"},
		// The reader is strict, and that is what makes the test above mean
		// something: a property it accepts is a property the model really has,
		// not a word it ignored.
		"незнакомое свойство поля": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], высота_строки: 3}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`, "высота_строки"},
		// Filling and data history belong to a field a person fills in on a new
		// object. A movement is written by posting, from the document, and its
		// versions live in the history of that document - so the export writes
		// neither on any field of these three registers, while writing both on
		// every field of an information register, whose records are entered by
		// hand.
		"значение заполнения у ресурса": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}]}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], filling: {value: {kind: number, data: "0"}}}`, "filling belongs to a field somebody fills in"},
		"история данных у измерения": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], data_history: use}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`, "data_history belongs to an object"},
		// Индексирование of a resource belongs to information registers, and
		// this is not one.
		"индексирование у ресурса": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}]}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], indexing: index}`, "indexing belongs to a dimension"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(entriesProject(t, true, testCase.fields))
			if err == nil {
				t.Fatal("the register was accepted")
			}
			if !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("error = %q, expected it to name %q", err, testCase.message)
			}
		})
	}
}

// A resource of a calculation register is found by nothing: Индексирование of
// a resource belongs to information registers, and this is not one.
func TestCalculationRegisterResourceRefusesIndexing(t *testing.T) {
	t.Parallel()
	body := `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: month
dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}]}
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}], indexing: index}
`
	_, err := Load(calculationProject(t, body))
	if err == nil {
		t.Fatal("the register was accepted")
	}
	if !strings.Contains(err.Error(), "indexing belongs to a dimension") {
		t.Fatalf("error = %q", err)
	}
}

// demoConfiguration is the smallest configuration a register decodes against.
func demoConfiguration() project.Project {
	return project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"},
		DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
}

// A movement of an accumulation register is found by its dimensions and its
// period, never by a quantity, and «ОбъектМетаданных: Ресурс.Индексирование»
// belongs to information registers only. Ours accepted it and the schema even built
// the index - a table nobody queries, on every write of the register.
//
// Refused rather than quietly turned off: a property silently dropped is what
// the import report calls a loss of meaning, and the export shows nothing real
// is refused - not one resource of the accumulation, accounting and calculation
// registers of the demo configuration is indexed.
func TestAccumulationRegisterResourceRefusesIndexing(t *testing.T) {
	t.Parallel()
	registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + registerID.String() + "\nname: ОстаткиТоваров\ntitle: {ru: Остатки товаров}\nkind: balance\n" +
		"dimensions:\n  - {id: " + dimensionID.String() + ", name: Товар, title: {ru: Товар}, types: [{kind: string, length: 100}], indexing: index}\n" +
		"resources:\n  - {id: " + resourceID.String() + ", name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}], indexing: index}\n"
	_, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
	if err == nil {
		t.Fatal("the register was accepted")
	}
	if !strings.Contains(err.Error(), "resources[0].indexing belongs to a dimension") {
		t.Fatalf("error = %q", err)
	}
	if strings.Contains(err.Error(), "dimensions[0]") {
		t.Fatalf("the dimension was refused its index too: %q", err)
	}
}

// The other side of the same rule, and the one that keeps it from spreading: a
// resource of an information register may be indexed, and in the demo
// configuration 52 of 699 are. Records there are read by resource - a price
// list is searched by price - which is why the property is theirs alone.
func TestInformationRegisterResourceKeepsItsIndexing(t *testing.T) {
	t.Parallel()
	registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + registerID.String() + "\nname: Цены\ntitle: {ru: Цены}\nwrite_mode: independent\nperiodicity: none\n" +
		"dimensions:\n  - {id: " + dimensionID.String() + ", name: Товар, title: {ru: Товар}, types: [{kind: string, length: 100}]}\n" +
		"resources:\n  - {id: " + resourceID.String() + ", name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}], indexing: index}\n"
	value, err := DecodeInformationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
	if err != nil {
		t.Fatalf("an indexed resource of an information register was refused: %v", err)
	}
	if value.Resources[0].Indexing != IndexField {
		t.Fatalf("resource indexing = %q", value.Resources[0].Indexing)
	}
}

// TestAccountingRegisterFieldLinksReachDimensions is the defect that came out
// of looking at where the converted fields were not: the list of fields a
// choice link may reach was built from attributes alone, so a link to a
// dimension of the same register was refused as pointing outside the object,
// and a link drawn from a dimension was not looked at at all.
//
// Both halves are here, because the fix has to hold both: the legitimate link
// is accepted, and the one pointing nowhere is refused.
func TestAccountingRegisterFieldLinksReachDimensions(t *testing.T) {
	t.Parallel()
	toDimension := `dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}]}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}
attributes:
  - id: ` + entriesFxSum + `
    name: Договор
    title: {ru: Договор}
    types: [{kind: catalog, reference: ` + entriesCompanies + `}]
    choice:
      parameter_links:
        - {name: Отбор.Владелец, source: {attribute: ` + entriesCompany + `}, change: clear}
`
	if _, err := Load(entriesProject(t, true, toDimension)); err != nil {
		t.Fatalf("a link to a dimension of the same register was refused: %v", err)
	}

	fromDimension := `dimensions:
  - id: ` + entriesCompany + `
    name: Организация
    title: {ru: Организация}
    types: [{kind: catalog, reference: ` + entriesCompanies + `}]
    choice:
      parameter_links:
        - {name: Отбор.Владелец, source: {attribute: ` + entriesKinds + `}, change: clear}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}
`
	err := entriesError(t, fromDimension)
	if err == nil {
		t.Fatal("a link from a dimension to something that is not a field of this register was accepted")
	}
	if !strings.Contains(err.Error(), "dimensions[0].choice.parameter_links[0].source is not a field of this object") {
		t.Fatalf("error = %q", err)
	}
}

// entriesError loads the entries project and returns only the error, which is
// all the refusal cases care about.
func entriesError(t *testing.T, fields string) error {
	t.Helper()
	_, err := Load(entriesProject(t, true, fields))
	return err
}

// Использование belongs to the attributes of catalogs and of charts of
// characteristic types, and a register is neither. It said where a field belongs -
// to items, to folders, to both - and a register has no folders to belong to,
// so the setting could only ever be read by nobody.
//
// The check existed and the converted fields were simply not passed to it.
func TestRegisterFieldRefusesUse(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"измерение регистра бухгалтерии": `dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], use: for-item}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`,
		"ресурс регистра бухгалтерии": `dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}]}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}], use: for-folder-and-item}`,
	}
	for name, fields := range cases {
		t.Run(name, func(t *testing.T) {
			err := entriesError(t, fields)
			if err == nil {
				t.Fatal("the field was accepted with a use of its own")
			}
			if !strings.Contains(err.Error(), ".use belongs to an attribute of a catalog") {
				t.Fatalf("error = %q", err)
			}
		})
	}
}

// The same for a dimension of a calculation register, which was left out of the
// same two checks for the same reason.
func TestCalculationRegisterDimensionRefusesUse(t *testing.T) {
	t.Parallel()
	body := `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: month
dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}], use: for-item}
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`
	_, err := Load(calculationProject(t, body))
	if err == nil {
		t.Fatal("the dimension was accepted with a use of its own")
	}
	if !strings.Contains(err.Error(), "dimensions[0].use belongs to an attribute of a catalog") {
		t.Fatalf("error = %q", err)
	}
}

// The same refusal on the other two registers of movements, and the opposite on
// the information register: its records are entered by hand, so filling and data
// history are exactly what its fields are for. The rule is narrow, and a rule
// this narrow is easy to widen by one kind - in the demonstration configuration
// all 699 fields of its registers carry both.
func TestMovementFieldsRefuseFillingWhileAnInformationRegisterKeepsIt(t *testing.T) {
	t.Parallel()
	t.Run("регистр накопления", func(t *testing.T) {
		registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
		source := "format: 1\nid: " + registerID.String() + "\nname: ОстаткиТоваров\ntitle: {ru: Остатки товаров}\nkind: balance\n" +
			"dimensions:\n  - {id: " + dimensionID.String() + ", name: Товар, title: {ru: Товар}, types: [{kind: string, length: 100}]}\n" +
			"resources:\n  - {id: " + resourceID.String() + ", name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}], data_history: use}\n"
		_, err := DecodeAccumulationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
		if err == nil || !strings.Contains(err.Error(), "data_history belongs to an object") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("регистр расчёта", func(t *testing.T) {
		body := `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: month
dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}], filling: {value: {kind: string, data: ""}, from_filling_value: true}}
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`
		_, err := Load(calculationProject(t, body))
		if err == nil || !strings.Contains(err.Error(), "filling belongs to a field somebody fills in") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("регистр сведений сохраняет и то и другое", func(t *testing.T) {
		registerID, dimensionID, resourceID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
		source := "format: 1\nid: " + registerID.String() + "\nname: Цены\ntitle: {ru: Цены}\nwrite_mode: independent\nperiodicity: none\n" +
			"dimensions:\n  - {id: " + dimensionID.String() + ", name: Товар, title: {ru: Товар}, types: [{kind: string, length: 100}], data_history: use}\n" +
			"resources:\n  - {id: " + resourceID.String() + ", name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}], filling: {value: {kind: number, data: \"0\"}}}\n"
		value, err := DecodeInformationRegister("register.yaml", strings.NewReader(source), demoConfiguration())
		if err != nil {
			t.Fatalf("an information register was refused what its fields are for: %v", err)
		}
		if value.Dimensions[0].DataHistory != UsageUse || value.Resources[0].Filling.Value == nil {
			t.Fatalf("dimension=%+v resource=%+v", value.Dimensions[0], value.Resources[0])
		}
	})
}
