package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
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
posting: {allowed: true}
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
	// Each case names the reason it is refused for: a register wrong in two
	// ways is refused for either, and a test that only asks for some error
	// passes whichever check is gone.
	withAction := strings.Replace(base, "periodicity: month\n", "periodicity: month\naction_period: true\n", 1)
	for name, test := range map[string]struct{ body, want string }{
		"значение графика без графика": {base + `schedule_value: ` + calcScheduleValue + `
`, "schedule_value and schedule_date need the schedule they belong to"},
		"график без значения и даты": {base + `schedule: ` + calcSchedule + `
`, "a schedule needs both the resource"},
		"дата графика не из графика": {withAction + `schedule: ` + calcSchedule + `
schedule_value: ` + calcScheduleValue + `
schedule_date: ` + calcResult + `
`, "takes the date of the schedule from " + calcResult + ", which is not a dimension of ГрафикиРаботы"},
		"значение графика не из ресурсов графика": {withAction + `schedule: ` + calcSchedule + `
schedule_value: ` + calcScheduleDate + `
schedule_date: ` + calcScheduleDate + `
`, "takes the value of the schedule from " + calcScheduleDate + ", which is not a resource of ГрафикиРаботы"},
		"связь с графиком без графика": {base + `dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}], schedule_link: ` + calcSchedulePerson + `}
`, "schedule_link needs a schedule"},
		"перерасчёт по чужому измерению": {base + `recalculations:
  - {id: ` + calcRecalc + `, name: Перерасчет, title: {ru: Перерасчёт}, dimensions: [{id: ` + calcRecalcDim + `, name: Лицо, title: {ru: Лицо}, register_dimension: ` + calcScheduleValue + `, leading_data: [` + calcPerson + `]}]}
`, "which is not a dimension of this register"},
		"график без периода действия": {strings.Replace(calcRegisterBody, "action_period: true\n", "", 1), "a schedule needs the period of action"},
		// «Проверка конфигурации»: «Не назначен план видов расчета»
		// (docs/block-00/registr-rascheta.md).
		"без плана видов расчёта": {strings.Replace(calcRegisterBody, "chart_of_calculation_types: "+calcChart+"\n", "", 1),
			"chart_of_calculation_types is required"},
		"неизвестная периодичность": {`format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: fortnight
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
`, "periodicity must be day, month, quarter or year"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := calculationProject(t, test.body)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want it to say %q", err, test.want)
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
posting: {allowed: true}
movements: [`+calcRegister+`]
`)
		return root
	}
	// Each case names its reason: a period of action also needs a schedule
	// (2.497), so without one the register would be refused for that instead.
	for name, test := range map[string]struct{ body, want string }{
		"период действия без конкуренции": {`action_period: true
schedule: ` + calcSchedule + `
schedule_value: ` + calcScheduleValue + `
schedule_date: ` + calcScheduleDate + `
`, "has no competition over it"},
		"базовый период без зависимости от базы": {`base_period: true
`, "takes no base, so the period would be gathered over nothing"},
		"базовое измерение без зависимости от базы": {`dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}], base: true}
`, "is a base dimension, but"},
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
`+test.body)
			if message := loadRefused(t, root, "a register outrunning its chart"); !strings.Contains(message, test.want) {
				t.Fatalf("refused for another reason: %v", message)
			}
		})
	}
}

// The link to the schedule is filled on attributes of a calculation register,
// not on its dimensions, in the configurations being moved - twelve of them -
// and the designer shows it on both (checked by the owner on the platform,
// 01.10.2026). The link points at a dimension of the schedule, like a
// dimension's does.
//
// Defect caught: the link of an attribute refused or dropped at load (twelve
// attributes of erp and acc), and a link of an attribute accepted without a
// schedule or pointing outside it.
func TestCalculationRegisterAttributeLinksToTheSchedule(t *testing.T) {
	t.Parallel()
	attribute := `attributes:
  - {id: ca100000-0000-4000-8000-000000000020, name: ВидГрафика, title: {ru: Вид графика}, types: [{kind: catalog, reference: ` + calcPeople + `}], schedule_link: %s}
`
	root := calculationProject(t, calcRegisterBody+fmt.Sprintf(attribute, calcSchedulePerson))
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("the link of an attribute refused: %v", err)
	}
	register, _ := catalog.CalculationRegister("ОсновныеНачисления")
	if len(register.Attributes) != 1 || register.Attributes[0].ScheduleLink == nil || register.Attributes[0].ScheduleLink.String() != calcSchedulePerson {
		t.Fatalf("the link of an attribute was lost: %+v", register.Attributes)
	}

	// Pointing outside the schedule.
	root = calculationProject(t, calcRegisterBody+fmt.Sprintf(attribute, calcResult))
	if _, err := Load(root); err == nil {
		t.Fatal("an attribute linked to something that is not a dimension of the schedule was accepted")
	}
	// With no schedule at all.
	noSchedule := strings.NewReplacer("schedule: "+calcSchedule+"\n", "", "schedule_value: "+calcScheduleValue+"\n", "",
		"schedule_date: "+calcScheduleDate+"\n", "", "    schedule_link: "+calcSchedulePerson+"\n", "").Replace(calcRegisterBody)
	if _, err := DecodeCalculationRegister("register.yaml", strings.NewReader(noSchedule+fmt.Sprintf(attribute, calcSchedulePerson)), metadataConfiguration()); err == nil {
		t.Fatal("an attribute linked to a schedule the register does not have was accepted")
	}
}

// The periodicity of a calculation register is one of four: day, month,
// quarter, year - the syntax assistant's ПериодичностьРегистраРасчета and the
// designer alike (checked by the owner on the platform, 01.10.2026).
//
// Defect caught: ten days and half a year accepted, periodicities the
// prototype does not give this register and nothing could ever import.
func TestCalculationRegisterPeriodicityIsThePrototypes(t *testing.T) {
	t.Parallel()
	for periodicity, want := range map[string]bool{
		"day": true, "month": true, "quarter": true, "year": true, "ten-days": false, "half-year": false,
	} {
		body := strings.Replace(calcRegisterBody, "periodicity: month", "periodicity: "+periodicity, 1)
		_, err := DecodeCalculationRegister("register.yaml", strings.NewReader(body), metadataConfiguration())
		if (err == nil) != want {
			t.Errorf("periodicity %s: err = %v, want accepted = %v", periodicity, err, want)
		}
	}
}

// A recalculation and its dimension carry a comment, and the recalculation its
// record set module, in a folder of its own inside the register's folder.
//
// Defect caught: the comment of a recalculation or of its dimension refused or
// dropped; the record set module of a recalculation having nowhere to lie; the
// module left out of the project's sources, so that publication would lose it;
// a folder for a recalculation the register does not describe, or a file other
// than the record set module in it, accepted.
func TestRecalculationKeepsItsCommentAndItsModule(t *testing.T) {
	t.Parallel()
	body := strings.Replace(calcRegisterBody, "    title: {ru: Перерасчёт основных начислений}\n",
		"    title: {ru: Перерасчёт основных начислений}\n    comment: УДАЛИТЬ\n", 1)
	body = strings.Replace(body, "        register_dimension: "+calcPerson+"\n",
		"        register_dimension: "+calcPerson+"\n        comment: по сотруднику\n", 1)
	root := calculationProject(t, body)
	folder := filepath.Join(root, "metadata", string(CalculationRegisterKind), "ОсновныеНачисления", project.RecalculationsDirectory, "ПерерасчетОсновныхНачислений")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	module := "Процедура ПередЗаписью(Отказ, Замещение)\nКонецПроцедуры\n"
	if err := os.WriteFile(filepath.Join(folder, project.RecordSetModuleFile), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a recalculation with a comment and a module refused: %v", err)
	}
	register, _ := catalog.CalculationRegister("ОсновныеНачисления")
	recalculation := register.Recalculations[0]
	if recalculation.Comment != "УДАЛИТЬ" || recalculation.Dimensions[0].Comment != "по сотруднику" {
		t.Fatalf("comments lost: %q, %q", recalculation.Comment, recalculation.Dimensions[0].Comment)
	}
	sources, err := project.ObjectFolderSourcePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "metadata/calculation-registers/ОсновныеНачисления/recalculations/ПерерасчетОсновныхНачислений/" + project.RecordSetModuleFile
	if !slices.Contains(sources, want) {
		t.Fatalf("the module of the recalculation is not among the project's sources: %v", sources)
	}

	if err := os.WriteFile(filepath.Join(folder, project.ObjectModuleFile), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("a file other than the record set module accepted in a recalculation's folder")
	}
	if err := os.Remove(filepath.Join(folder, project.ObjectModuleFile)); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(filepath.Dir(folder), "Чужой")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("a folder for a recalculation the register does not describe accepted")
	}
}

// The schedule is what the syntax assistant describes: a register of
// information that is not periodic, its date a dimension of the type Date, its
// value a resource of the type Number.
//
// Defect caught: a periodic register, a date that is not a date, or a value
// that is not a number accepted as the schedule - the register would then
// spread records over a calendar it cannot read.
func TestScheduleIsWhatTheSyntaxAssistantDescribes(t *testing.T) {
	t.Parallel()
	for name, change := range map[string][2]string{
		"периодический график": {"periodicity: none", "periodicity: day"},
		"дата не дата":         {"name: Дата, title: {ru: Дата}, types: [{kind: date, date_parts: date}]", "name: Дата, title: {ru: Дата}, types: [{kind: string, length: 10}]"},
		"значение не число":    {"name: Значение, title: {ru: Значение}, types: [{kind: number, precision: 5, scale: 2}]", "name: Значение, title: {ru: Значение}, types: [{kind: string, length: 10}]"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := calculationProject(t, calcRegisterBody)
			path := filepath.Join(root, "metadata", string(InformationRegisterKind), "ГрафикиРаботы", project.ObjectMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), change[0]) {
				t.Fatalf("fixture changed: %q not found", change[0])
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(content), change[0], change[1], 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(root); err == nil {
				t.Fatal("accepted as a schedule")
			}
		})
	}
}

// A recalculation with no dimensions is saved by the designer - the test
// configuration of mdclasses keeps one, with no child objects at all.
//
// Defect caught: such a recalculation refused at load, with nothing in the
// prototype to refuse it for.
func TestRecalculationWithNoDimensionsIsKept(t *testing.T) {
	t.Parallel()
	body := strings.Replace(calcRegisterBody, `    dimensions:
      - id: `+calcRecalcDim+`
        name: ФизическоеЛицо
        title: {ru: Физическое лицо}
        register_dimension: `+calcPerson+`
        leading_data: [`+calcPerson+`]
`, "", 1)
	if body == calcRegisterBody {
		t.Fatal("fixture changed: the recalculation's dimensions were not found")
	}
	catalog, err := Load(calculationProject(t, body))
	if err != nil {
		t.Fatalf("a recalculation with no dimensions refused: %v", err)
	}
	register, _ := catalog.CalculationRegister("ОсновныеНачисления")
	if len(register.Recalculations) != 1 || len(register.Recalculations[0].Dimensions) != 0 {
		t.Fatalf("recalculations = %+v", register.Recalculations)
	}
}

// A resource of a calculation or an accounting register is a number or a
// defined type that stands for a number (checked by the owner on the platform,
// 01.10.2026) - the rule a register of accumulation already kept.
//
// Defect caught: a string, a date or a reference accepted as the amount of a
// payroll record or of an entry, and a defined type standing for something
// other than a number accepted where an amount is kept.
func TestCalculationAndAccountingResourcesAreNumbers(t *testing.T) {
	t.Parallel()
	textual := strings.Replace(calcRegisterBody, "name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]",
		"name: Результат, title: {ru: Результат}, types: [{kind: string, length: 10}]", 1)
	if _, err := DecodeCalculationRegister("register.yaml", strings.NewReader(textual), metadataConfiguration()); err == nil {
		t.Fatal("a calculation register with a string resource accepted")
	}
	twice := strings.Replace(calcRegisterBody, "types: [{kind: number, precision: 15, scale: 2}]}\nrecalculations",
		"types: [{kind: number, precision: 15, scale: 2}, {kind: date}]}\nrecalculations", 1)
	if _, err := DecodeCalculationRegister("register.yaml", strings.NewReader(twice), metadataConfiguration()); err == nil {
		t.Fatal("a calculation register with a composite resource accepted")
	}

	root := entriesProject(t, true, `resources:
  - {id: `+entriesSum+`, name: Сумма, title: {ru: Сумма}, types: [{kind: string, length: 10}]}`)
	if _, err := Load(root); err == nil {
		t.Fatal("an accounting register with a string resource accepted")
	}
	// Where the file alone is read - an editor checking one object - the
	// accounting register refuses it too, without the rest of the project.
	if _, err := DecodeAccountingRegister("register.yaml", strings.NewReader(`format: 1
id: `+entriesRegister+`
name: Хозрасчетный
title: {ru: Хозрасчётный}
resources:
  - {id: `+entriesSum+`, name: Сумма, title: {ru: Сумма}, types: [{kind: string, length: 10}]}
`), metadataConfiguration()); err == nil {
		t.Fatal("an accounting register with a string resource accepted where its file is read")
	}

	// A defined type is resolved where the defined types are known.
	for name, value := range map[string]string{"число": "[{kind: number, precision: 15, scale: 2}]", "строка": "[{kind: string, length: 10}]"} {
		root := entriesProject(t, true, `resources:
  - {id: `+entriesSum+`, name: Сумма, title: {ru: Сумма}, types: [{kind: defined-type, reference: ca100000-0000-4000-8000-0000000000dd}]}`)
		writeMetadata(t, root, DefinedTypeKind, "ca100000-0000-4000-8000-0000000000dd", `format: 1
id: ca100000-0000-4000-8000-0000000000dd
name: ДенежнаяСумма
title: {ru: Денежная сумма}
types: `+value+`
`)
		_, err := Load(root)
		if (err == nil) != (name == "число") {
			t.Errorf("defined type of %s: err = %v", name, err)
		}
	}
}

// A register with a period of action has a schedule: the designer refuses one
// without - «Не назначен график регистру с периодом действия» - and keeps a
// register with no resource, which its check of the configuration does not
// name (opis of 15.10.2026, registr-rascheta.md). The latter is carried and
// noted: it calculates nothing.
//
// Defect caught: a period of action with no schedule loaded; a register with
// no resource refused, as the rule this replaced did, or taken silently.
func TestACalculationRegisterNeedsAScheduleAndNoResource(t *testing.T) {
	t.Parallel()
	root := calculationProject(t, strings.Replace(calcRegisterBody,
		"schedule: "+calcSchedule+"\nschedule_value: "+calcScheduleValue+"\nschedule_date: "+calcScheduleDate+"\n", "", 1))
	if message := loadRefused(t, root, "a period of action with no schedule"); !strings.Contains(message,
		"action_period needs a schedule: the designer refuses a register with a period of action and no schedule") {
		t.Fatalf("refused for another reason: %v", message)
	}
	root = calculationProject(t, `format: 1
id: `+calcRegister+`
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: `+calcChart+`
periodicity: month
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a register with no resource was refused: %v", err)
	}
	var noted []Note
	for _, note := range catalog.Notes() {
		if note.Kind == NoteRegisterWithoutFields {
			noted = append(noted, note)
		}
	}
	if len(noted) != 1 || noted[0].Where != "calculation-registers ОсновныеНачисления" || noted[0].Written != "no resources" {
		t.Fatalf("notes = %+v, want the register with no resources", noted)
	}
	if _, err := catalog.ApplicationSchema(); err != nil {
		t.Fatalf("the schema of a register with no resource: %v", err)
	}
}

// A dimension of a recalculation names the dimension of the register it finds
// its records by: «Проверка конфигурации» refuses one naming none - «Не задано
// измерение основного регистра» (opis of 15.10.2026, pereraschet.md). With no
// leading data it is taken, and with no note: the check says nothing of it.
// The text is written out here.
//
// Defect caught: a dimension naming no dimension of the register loaded, or
// taken with a note; one with no leading data refused or noted.
func TestARecalculationDimensionNamesADimensionOfTheRegister(t *testing.T) {
	t.Parallel()
	register := func(dimension string) string {
		return `format: 1
id: ` + calcRegister + `
name: ОсновныеНачисления
title: {ru: Основные начисления}
chart_of_calculation_types: ` + calcChart + `
periodicity: month
dimensions:
  - {id: ` + calcPerson + `, name: ФизическоеЛицо, title: {ru: Физическое лицо}, types: [{kind: catalog, reference: ` + calcPeople + `}]}
resources:
  - {id: ` + calcResult + `, name: Результат, title: {ru: Результат}, types: [{kind: number, precision: 15, scale: 2}]}
recalculations:
  - {id: ` + calcRecalc + `, name: Перерасчет, title: {ru: Перерасчёт}, dimensions: [{id: ` + calcRecalcDim + `, name: Лицо, title: {ru: Лицо}` + dimension + `}]}
`
	}
	root := calculationProject(t, register(", leading_data: ["+calcPerson+"]"))
	if message := loadRefused(t, root, "a dimension naming no dimension of the register"); !strings.Contains(message,
		"recalculations[0].dimensions[0].register_dimension must name a dimension of the register: the designer refuses a dimension of a recalculation naming none") {
		t.Fatalf("refused for another reason: %v", message)
	}
	root = calculationProject(t, register(", register_dimension: "+calcPerson))
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a dimension with no leading data was refused: %v", err)
	}
	if notes := catalog.Notes(); len(notes) != 0 {
		t.Fatalf("a dimension with no leading data is noted: %+v", notes)
	}
}
