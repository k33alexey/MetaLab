package metadata

import (
	"testing"
)

const calculationTypesID = "a0000000-0000-4000-8000-000000000001"

// A calculation type without its competition rules computes different numbers
// while looking transferred, so the rules travel with it: what displaces it,
// what leads it, and what its base is gathered from.
func TestLoadChartOfCalculationTypesWithCompetitionRules(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCalculationTypesKind, calculationTypesID, `format: 1
id: `+calculationTypesID+`
name: ОсновныеНачисления
title: {ru: Основные начисления}
code: {type: string, length: 5}
description_length: 100
action_period_use: true
base_dependency: by-action-period
base_charts: [`+calculationTypesID+`]
predefined:
  - id: a0000000-0000-4000-8000-000000000010
    name: ОкладПоДням
    code: "00001"
    description: Оклад по дням
    action_period_is_base: true
    displacing: [ОплатаКомандировки]
  - id: a0000000-0000-4000-8000-000000000011
    name: ОплатаКомандировки
    code: "00002"
    description: Оплата командировки
    action_period_is_base: true
  - id: a0000000-0000-4000-8000-000000000012
    name: Премия
    code: "00003"
    description: Премия
    leading: [ОкладПоДням]
    base: [ОкладПоДням]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	chart, ok := catalog.ChartOfCalculationTypes("ОсновныеНачисления")
	if !ok {
		t.Fatal("the chart did not load")
	}
	if !chart.ActionPeriodUse || chart.BaseDependency != ActionPeriodBase || len(chart.BaseCharts) != 1 {
		t.Fatalf("the chart lost its settings: %+v", chart)
	}
	if len(chart.Predefined) != 3 {
		t.Fatalf("predefined calculation types = %d, want 3", len(chart.Predefined))
	}
	if chart.Predefined[0].Displacing[0] != "ОплатаКомандировки" || !chart.Predefined[0].ActionPeriodIsBase {
		t.Fatalf("the competition rules were lost: %+v", chart.Predefined[0])
	}
	if chart.Predefined[2].Leading[0] != "ОкладПоДням" || chart.Predefined[2].Base[0] != "ОкладПоДням" {
		t.Fatalf("leading and base were lost: %+v", chart.Predefined[2])
	}
}

// All three standard tabular sections are there whatever the settings say, as
// they are in the prototype: every chart of the configurations being moved has
// all three, those without an action period too. So is the flag "action period
// is base", a standard field of every calculation type. A line of each section
// says whether the configuration brought it, which is its standard field
// "Предопределенный".
func TestCompetitionTablesAreThereWhateverTheSettings(t *testing.T) {
	t.Parallel()
	for name, header := range map[string]string{
		"без периода действия и базы": "",
		"только период действия":      "action_period_use: true",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ChartOfCalculationTypesKind, calculationTypesID, `format: 1
id: `+calculationTypesID+`
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 5}
description_length: 100
`+header+`
`)
			catalog, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := catalog.ApplicationSchema()
			if err != nil {
				t.Fatal(err)
			}
			id := catalog.ChartsOfCalculationTypes[0].ID
			own, err := PhysicalCatalogTable(id)
			if err != nil {
				t.Fatal(err)
			}
			columns := map[string]map[string]bool{}
			for _, table := range schema.Tables {
				columns[table.Name] = map[string]bool{}
				for _, column := range table.Columns {
					columns[table.Name][column.Name] = true
				}
			}
			if !columns[own]["action_period_is_base"] {
				t.Fatal("the flag of an action period being the base is a standard field of every calculation type")
			}
			names := catalog.PhysicalNames()
			for _, prefix := range []string{"tl", "tw", "tb"} {
				table := competitionTableName(prefix, id)
				if columns[table] == nil {
					t.Fatalf("the standard table part %s must exist whatever the settings", prefix)
				}
				if !columns[table]["predefined"] {
					t.Fatalf("a line of %s must say whether the configuration brought it", prefix)
				}
				if names[table] == "" {
					t.Fatalf("the table part %s has no name a developer knows", prefix)
				}
			}
		})
	}
}

// A chart that switched a setting off after its lists were filled in keeps
// them, as the prototype does: erp "Удержания" has no action period and still
// writes the flag and the list of displacing types. The calculation is what
// leaves them unread, not the loader that drops the chart.
func TestCompetitionRulesAreKeptWithTheirSettingOff(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCalculationTypesKind, calculationTypesID, `format: 1
id: `+calculationTypesID+`
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 5}
description_length: 100
predefined:
  - id: a0000000-0000-4000-8000-000000000010
    name: Оклад
    code: "00001"
    action_period_is_base: true
    displacing: [Премия]
    base: [Премия]
  - id: a0000000-0000-4000-8000-000000000011
    name: Премия
    code: "00002"
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	chart, _ := catalog.ChartOfCalculationTypes("Начисления")
	salary := chart.Predefined[0]
	if !salary.ActionPeriodIsBase || len(salary.Displacing) != 1 || len(salary.Base) != 1 {
		t.Fatalf("the rules of a chart with its settings off were lost: %+v", salary)
	}
}

// Rules that point at nothing are refused: they would quietly change the
// numbers instead of failing to transfer.
func TestChartOfCalculationTypesRefusesRulesThatPointNowhere(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"вытесняет несуществующий вид": `
action_period_use: true
predefined:
  - id: a0000000-0000-4000-8000-000000000010
    name: Оклад
    code: "00001"
    displacing: [Надбавка]
`,
		"вытесняет сам себя": `
action_period_use: true
predefined:
  - id: a0000000-0000-4000-8000-000000000010
    name: Оклад
    code: "00001"
    displacing: [Оклад]
`,
		"зависимость без базовых планов": `
base_dependency: by-action-period
`,
		"базовые планы без зависимости": `
base_charts: [` + calculationTypesID + `]
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ChartOfCalculationTypesKind, calculationTypesID, `format: 1
id: `+calculationTypesID+`
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 5}
description_length: 100
`+body)
			if _, err := Load(root); err == nil {
				t.Fatal("a rule that points at nothing was accepted")
			}
		})
	}
}
