package metadata

import (
	"strings"
	"testing"
)

// The defect this catches: a field of an accounting register carried a name, a
// type and indexing and nothing else, because it had a struct of its own
// instead of the common set. The prototype has one metadata object «Измерение»
// and one «Ресурс» for all four kinds of register, and a dimension of an
// accounting register there carries the same twenty-odd properties a catalog
// attribute does. Ours could not be given a format, a tooltip, a choice
// parameter or a filling value at all: the reader refused the words.
func TestAccountingRegisterFieldCarriesTheCommonSet(t *testing.T) {
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
    # Заполнение мы принимаем и храним, но выгрузка не даёт его ни одному
    # ресурсу регистров бухгалтерии, расчёта и накопления - вопрос открыт и
    # заведён пунктом в карте. Тест держит текущее поведение, а не эталон.
    filling: {value: {kind: number, data: "0"}}
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
	dimension := register.Dimensions[0]
	if dimension.Comment == "" || dimension.FillChecking != ShowFillingError || dimension.FullTextSearch != UsageUse {
		t.Fatalf("dimension = %+v", dimension)
	}
	if dimension.Presentation.ToolTip["ru"] != "Организация проводки" {
		t.Fatalf("dimension tooltip = %+v", dimension.Presentation.ToolTip)
	}
	if dimension.Choice.QuickChoice != UsageUse || dimension.Choice.FoldersAndItems != ChoiceTargetItems {
		t.Fatalf("dimension choice = %+v", dimension.Choice)
	}
	if len(dimension.Choice.Parameters) != 1 || dimension.Choice.Parameters[0].Name != "Отбор.Действует" {
		t.Fatalf("dimension choice parameters = %+v", dimension.Choice.Parameters)
	}
	// The dimension keeps what it always had beside the common set.
	if !dimension.Balance || dimension.Indexing != IndexField {
		t.Fatalf("dimension lost its own properties: %+v", dimension)
	}
	resource := register.Resources[0]
	if resource.Presentation.Format["ru"] != "ЧДЦ=2" || !resource.Presentation.MarkNegatives {
		t.Fatalf("resource presentation = %+v", resource.Presentation)
	}
	if resource.Presentation.MinValue == nil || resource.Presentation.MinValue.Data != "0" {
		t.Fatalf("resource min value = %+v", resource.Presentation.MinValue)
	}
	if resource.Filling.Value == nil || resource.Filling.Value.Data != "0" {
		t.Fatalf("resource filling = %+v", resource.Filling)
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
		// «Признак учета субконто» - «используется для объектов метаданных,
		// описывающих ресурсы регистра бухгалтерии». A dimension holds no
		// amount, so there is nothing for it to keep by ext dimension.
		"признак учёта субконто у измерения": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], ext_dimension_accounting_flag: ` + entriesExtFlag + `}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`, "ext_dimension_accounting_flag belongs to a resource"},
		// The reader is strict, and that is what makes the test above mean
		// something: a property it accepts is a property the model really has,
		// not a word it ignored.
		"незнакомое свойство поля": {`dimensions:
  - {id: ` + entriesCompany + `, name: Организация, title: {ru: Организация}, types: [{kind: catalog, reference: ` + entriesCompanies + `}], высота_строки: 3}
resources:
  - {id: ` + entriesSum + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`, "высота_строки"},
		// «Индексирование» of a resource is «для ресурсов регистра сведений»,
		// and this is not one.
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

// The same for a dimension of a calculation register, which had a struct of its
// own for the same reason and lost the same properties by it.
func TestCalculationRegisterDimensionCarriesTheCommonSet(t *testing.T) {
	t.Parallel()
	body := `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: month
schedule: ` + calcSchedule + `
schedule_value: ` + calcScheduleValue + `
schedule_date: ` + calcScheduleDate + `
dimensions:
  - id: ` + calcPerson + `
    name: ФизическоеЛицо
    title: {ru: Физическое лицо}
    types: [{kind: catalog, reference: ` + calcPeople + `}]
    comment: Кому начислено
    base: true
    indexing: index
    schedule_link: ` + calcSchedulePerson + `
    fill_checking: show-error
    presentation:
      tooltip: {ru: Физическое лицо}
    choice:
      quick_choice: use
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`
	catalog, err := Load(calculationProject(t, body))
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.CalculationRegister("ОсновныеНачисления")
	if !ok {
		t.Fatal("the calculation register did not load")
	}
	dimension := register.Dimensions[0]
	if dimension.Comment == "" || dimension.FillChecking != ShowFillingError {
		t.Fatalf("dimension = %+v", dimension)
	}
	if dimension.Presentation.ToolTip["ru"] == "" || dimension.Choice.QuickChoice != UsageUse {
		t.Fatalf("dimension presentation=%+v choice=%+v", dimension.Presentation, dimension.Choice)
	}
	if !dimension.Base || dimension.ScheduleLink == nil || dimension.Indexing != IndexField {
		t.Fatalf("dimension lost its own properties: %+v", dimension)
	}
}

// A resource of a calculation register is found by nothing: «индексирование»
// of a resource is «для ресурсов регистра сведений», and this is not one.
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
