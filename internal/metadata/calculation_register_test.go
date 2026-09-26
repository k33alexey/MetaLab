package metadata

import (
	"testing"
)

const (
	calcChart    = "ca100000-0000-4000-8000-000000000001"
	calcRegister = "ca100000-0000-4000-8000-000000000002"
	calcDocument = "ca100000-0000-4000-8000-000000000003"
	calcSchedule = "ca100000-0000-4000-8000-000000000004"
	calcPeople   = "ca100000-0000-4000-8000-000000000005"

	calcScheduleDate   = "ca100000-0000-4000-8000-000000000010"
	calcSchedulePerson = "ca100000-0000-4000-8000-000000000011"
	calcScheduleValue  = "ca100000-0000-4000-8000-000000000012"
	calcPerson         = "ca100000-0000-4000-8000-000000000013"
	calcResult         = "ca100000-0000-4000-8000-000000000014"
	calcRecalc         = "ca100000-0000-4000-8000-000000000015"
	calcRecalcDim      = "ca100000-0000-4000-8000-000000000016"
)

// A payroll register: it competes for a period of action, takes a base, reads a
// working calendar, and carries a recalculation.
func calculationProject(t *testing.T, register string) string {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, calcPeople, "ФизическиеЛица")
	writeMetadata(t, root, ChartOfCalculationTypesKind, calcChart, `format: 1
id: `+calcChart+`
name: ОсновныеНачисления
title: {ru: Основные начисления}
code: {type: string, length: 9, auto: false}
description_length: 100
action_period_use: true
base_dependency: by-action-period
base_charts: [`+calcChart+`]
`)
	writeMetadata(t, root, DocumentKind, calcDocument, `format: 1
id: `+calcDocument+`
name: НачислениеЗарплаты
title: {ru: Начисление зарплаты}
number: {type: string, length: 9, auto: true, periodicity: none}
posting: true
movements: [`+calcRegister+`]
`)
	writeMetadata(t, root, InformationRegisterKind, calcSchedule, `format: 1
id: `+calcSchedule+`
name: ГрафикиРаботы
title: {ru: Графики работы}
write_mode: independent
periodicity: none
dimensions:
  - {id: `+calcScheduleDate+`, name: Дата, title: {ru: Дата}, types: [{kind: date, date_parts: date}]}
  - {id: `+calcSchedulePerson+`, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: `+calcPeople+`}]}
resources:
  - {id: `+calcScheduleValue+`, name: Значение, title: {ru: Значение}, types: [{kind: number, precision: 5, scale: 2}]}
`)
	writeMetadata(t, root, CalculationRegisterKind, calcRegister, register)
	return root
}

const calcRegisterBody = `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: month
action_period: true
base_period: true
schedule: ` + calcSchedule + `
schedule_value: ` + calcScheduleValue + `
schedule_date: ` + calcScheduleDate + `
dimensions:
  - id: ` + calcPerson + `
    name: ФизическоеЛицо
    title: {ru: Физическое лицо}
    types: [{kind: catalog, reference: ` + calcPeople + `}]
    base: true
    indexing: index
    schedule_link: ` + calcSchedulePerson + `
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
recalculations:
  - id: ` + calcRecalc + `
    name: ПерерасчетОсновныхНачислений
    title: {ru: Перерасчёт основных начислений}
    dimensions:
      - id: ` + calcRecalcDim + `
        name: ФизическоеЛицо
        title: {ru: Физическое лицо}
        register_dimension: ` + calcPerson + `
        leading_data: [` + calcPerson + `]
`

func TestLoadCalculationRegisterWithRecalculation(t *testing.T) {
	t.Parallel()
	root := calculationProject(t, calcRegisterBody)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.CalculationRegister("ОсновныеНачисления")
	if !ok {
		t.Fatal("the calculation register did not load")
	}
	if register.Periodicity != CalculationPeriodMonth {
		t.Fatalf("the registration period was lost: %+v", register.Periodicity)
	}
	if !register.ActionPeriod || !register.BasePeriod {
		t.Fatalf("the two periods were lost: %+v", register)
	}
	if register.Schedule == nil || register.ScheduleValue == nil || register.ScheduleDate == nil {
		t.Fatalf("the schedule was lost: %+v", register)
	}
	if !register.Dimensions[0].Base || register.Dimensions[0].ScheduleLink == nil {
		t.Fatalf("a dimension lost what ties it to its base and to the schedule: %+v", register.Dimensions[0])
	}
	if len(register.Recalculations) != 1 || len(register.Recalculations[0].Dimensions) != 1 {
		t.Fatalf("the recalculation was lost: %+v", register.Recalculations)
	}
	if len(register.Recalculations[0].Dimensions[0].LeadingData) != 1 {
		t.Fatal("a recalculation dimension without leading data is never set off")
	}

	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	records, err := PhysicalCalculationRegisterTable(register.ID)
	if err != nil {
		t.Fatal(err)
	}
	recalculated, err := PhysicalRecalculationTable(register.Recalculations[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]map[string]bool{}
	for _, table := range schema.Tables {
		if table.Name == records || table.Name == recalculated {
			columns := map[string]bool{}
			for _, column := range table.Columns {
				columns[column.Name] = true
			}
			found[table.Name] = columns
		}
	}
	if len(found) != 2 {
		t.Fatalf("a register and its recalculation are two tables, got %d", len(found))
	}
	// The period a record is registered in and the period it acts over are
	// different columns, and the second is an interval.
	for _, column := range []string{"period", "calculation_type", "reversing",
		"action_period_start", "action_period_end", "base_period_start", "base_period_end"} {
		if !found[records][column] {
			t.Fatalf("the record has no %s", column)
		}
	}
	for _, column := range []string{"recorder_type", "recorder_ref", "calculation_type"} {
		if !found[recalculated][column] {
			t.Fatalf("the recalculation has no %s", column)
		}
	}
}

// A register without the two periods keeps neither, and nothing else changes.
func TestCalculationRegisterWithoutPeriodsKeepsNone(t *testing.T) {
	t.Parallel()
	root := calculationProject(t, `format: 1
id: `+calcRegister+`
name: Разовые
title: {ru: Разовые начисления}
chart_of_calculation_types: `+calcChart+`
periodicity: day
resources:
  - {id: `+calcResult+`, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, _ := catalog.CalculationRegister("Разовые")
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	table, err := PhysicalCalculationRegisterTable(register.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range schema.Tables {
		if item.Name != table {
			continue
		}
		for _, column := range item.Columns {
			if column.Name == "action_period_start" || column.Name == "base_period_end" {
				t.Fatalf("a register that keeps no periods must not carry %s", column.Name)
			}
		}
	}
}

// Each of these is a link whose absence shows up as a wrong number rather than
// as an error - so it is refused where it is written.
func TestCalculationRegisterRefusesLinksThatComputeNothing(t *testing.T) {
	t.Parallel()
	base := `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: month
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`
	for name, body := range map[string]string{
		"значение графика без графика": base + `schedule_value: ` + calcScheduleValue + `
`,
		"график без значения и даты": base + `schedule: ` + calcSchedule + `
`,
		"дата графика не из графика": base + `schedule: ` + calcSchedule + `
schedule_value: ` + calcScheduleValue + `
schedule_date: ` + calcResult + `
`,
		"связь с графиком без графика": base + `dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}], schedule_link: ` + calcSchedulePerson + `}
`,
		"перерасчёт по чужому измерению": base + `recalculations:
  - {id: ` + calcRecalc + `, name: Перерасчет, title: {ru: Перерасчёт}, dimensions: [{id: ` + calcRecalcDim + `, name: Лицо, title: {ru: Лицо}, register_dimension: ` + calcScheduleValue + `, leading_data: [` + calcPerson + `]}]}
`,
		"перерасчёт без ведущих данных": base + `dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}]}
recalculations:
  - {id: ` + calcRecalc + `, name: Перерасчет, title: {ru: Перерасчёт}, dimensions: [{id: ` + calcRecalcDim + `, name: Лицо, title: {ru: Лицо}, register_dimension: ` + calcPerson + `, leading_data: []}]}
`,
		"перерасчёт без измерений": base + `recalculations:
  - {id: ` + calcRecalc + `, name: Перерасчет, title: {ru: Перерасчёт}, dimensions: []}
`,
		"неизвестная периодичность": `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: fortnight
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := calculationProject(t, body)
			if _, err := Load(root); err == nil {
				t.Fatal("a link that computes nothing was accepted")
			}
		})
	}
}

// A period or a base dimension whose chart supports neither would be a column
// nobody can fill with meaning.
func TestCalculationRegisterNeedsItsChartToAgree(t *testing.T) {
	t.Parallel()
	plainChart := func(t *testing.T) string {
		t.Helper()
		root := metadataProject(t)
		catalogNamed(t, root, calcPeople, "ФизическиеЛица")
		writeMetadata(t, root, ChartOfCalculationTypesKind, calcChart, `format: 1
id: `+calcChart+`
name: Простые
title: {ru: Простые начисления}
code: {type: string, length: 9, auto: false}
description_length: 100
`)
		writeMetadata(t, root, DocumentKind, calcDocument, `format: 1
id: `+calcDocument+`
name: НачислениеЗарплаты
title: {ru: Начисление зарплаты}
number: {type: string, length: 9, auto: true, periodicity: none}
posting: true
movements: [`+calcRegister+`]
`)
		return root
	}
	for name, body := range map[string]string{
		"период действия без конкуренции": `action_period: true
`,
		"базовый период без зависимости от базы": `base_period: true
`,
		"базовое измерение без зависимости от базы": `dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}], base: true}
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := plainChart(t)
			writeMetadata(t, root, CalculationRegisterKind, calcRegister, `format: 1
id: `+calcRegister+`
name: Простые
title: {ru: Простые начисления}
chart_of_calculation_types: `+calcChart+`
periodicity: month
resources:
  - {id: `+calcResult+`, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`+body)
			if _, err := Load(root); err == nil {
				t.Fatal("a register outrunning its chart was accepted")
			}
		})
	}
}
