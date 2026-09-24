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
code: {type: string, length: 5, auto: true}
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

// Each standard tabular section exists only under the setting that gives it
// meaning: displacing without an action period has nothing to displace, and a
// base without a dependency has nothing to gather.
func TestCompetitionTablesFollowTheSettings(t *testing.T) {
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
code: {type: string, length: 5, auto: true}
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
			present := map[string]bool{}
			for _, table := range schema.Tables {
				present[table.Name] = true
			}
			if !present[competitionTableName("tl", id)] {
				t.Fatal("leading calculation types must always have their table")
			}
			if present[competitionTableName("tw", id)] != (header != "") {
				t.Fatal("the displacing table must exist exactly when the action period is used")
			}
			if present[competitionTableName("tb", id)] {
				t.Fatal("a base table without a base dependency is a column nobody may fill")
			}
		})
	}
}

// Rules that point at nothing are refused: they would quietly change the
// numbers instead of failing to transfer.
func TestChartOfCalculationTypesRefusesRulesThatPointNowhere(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"вытеснение без периода действия": `
predefined:
  - id: a0000000-0000-4000-8000-000000000010
    name: Оклад
    code: "00001"
    displacing: [Премия]
  - id: a0000000-0000-4000-8000-000000000011
    name: Премия
    code: "00002"
`,
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
		"база без зависимости": `
predefined:
  - id: a0000000-0000-4000-8000-000000000010
    name: Оклад
    code: "00001"
    base: [Оклад]
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
code: {type: string, length: 5, auto: true}
description_length: 100
`+body)
			if _, err := Load(root); err == nil {
				t.Fatal("a rule that points at nothing was accepted")
			}
		})
	}
}
