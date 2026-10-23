package metadata

import (
	"strings"
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

// A plan depending on a base names a base chart: «Проверка конфигурации»
// refuses one naming none (opis of 15.10.2026, plan-vidov-rascheta.md). Base
// charts under no dependency the designer keeps and does not refuse: taken,
// with no note. The texts are written out here.
//
// Defect caught: a dependency with no base chart loaded, or noted instead of
// refused; base charts under no dependency refused or noted.
func TestABaseDependencyNamesABaseChart(t *testing.T) {
	t.Parallel()
	chart := func(rest string) string {
		return "format: 1\nid: " + calculationTypesID + "\nname: ОсновныеНачисления\ntitle: {ru: Основные}\n" +
			"code: {type: string, length: 5}\ndescription_length: 100\naction_period_use: true\n" + rest
	}
	for _, dependency := range []string{"by-action-period", "by-registration-period"} {
		root := metadataProject(t)
		writeMetadata(t, root, ChartOfCalculationTypesKind, calculationTypesID, chart("base_dependency: "+dependency+"\n"))
		if message := loadRefused(t, root, dependency+" with no base chart"); !strings.Contains(message,
			"base_dependency "+dependency+" takes the base from base charts, and base_charts names none") {
			t.Fatalf("%s with no base chart refused for another reason: %v", dependency, message)
		}
	}
	for name, rest := range map[string]string{
		"a dependency with its base chart": "base_dependency: by-action-period\nbase_charts: [" + calculationTypesID + "]\n",
		"base charts under none":           "base_dependency: none\nbase_charts: [" + calculationTypesID + "]\n",
		"base charts under nothing said":   "base_charts: [" + calculationTypesID + "]\n",
	} {
		root := metadataProject(t)
		writeMetadata(t, root, ChartOfCalculationTypesKind, calculationTypesID, chart(rest))
		catalog, err := Load(root)
		if err != nil {
			t.Fatalf("%s was refused: %v", name, err)
		}
		if notes := catalog.Notes(); len(notes) != 0 {
			t.Fatalf("%s is noted: %+v", name, notes)
		}
	}
}
