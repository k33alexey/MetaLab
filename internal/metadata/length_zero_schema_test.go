package metadata

import (
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A length of 0 on the description of a chart of accounts, a chart of
// calculation types and a task, and on the code and the description of an
// exchange plan, switches the field off in the database as on a catalog: no
// column, no index by it. With a length the tables are what they were, column
// for column, so a base that already has them migrates nothing.
//
// Defect caught: a column of character varying(0), which PostgreSQL refuses -
// the migration of the whole configuration would stop on it; an index or a
// uniqueness on a code that is not there; and a table laid out anew for a
// length the change never touched.
func TestALengthOfZeroSwitchesTheColumnOffAndLeavesTheRestAlone(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T, length int) *Catalog {
		t.Helper()
		root := metadataProject(t)
		for _, item := range []struct {
			kind Kind
			body string
		}{
			{ChartOfAccountsKind, "code: {type: string, length: 5}\n"},
			{ChartOfCalculationTypesKind, "code: {type: string, length: 5}\n"},
			{TaskKind, "number: {type: string, length: 11, auto: true, periodicity: none}\n"},
			{ExchangePlanKind, "code: {type: string, length: " + map[bool]string{true: "0", false: "9"}[length == 0] + "}\n"},
		} {
			noteChartBody(t, root, item.kind, item.body+"description_length: "+map[bool]string{true: "0", false: "100"}[length == 0]+"\n")
		}
		catalog, err := Load(root)
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		return catalog
	}
	tables := func(t *testing.T, catalog *Catalog) map[string]schemadiff.Table {
		t.Helper()
		schema, err := catalog.ApplicationSchema()
		if err != nil {
			t.Fatal(err)
		}
		result := map[string]schemadiff.Table{}
		for _, table := range schema.Tables {
			result[table.Name] = table
		}
		return result
	}
	columns := func(table schemadiff.Table) []string {
		var names []string
		for _, column := range table.Columns {
			names = append(names, column.Name)
		}
		return names
	}

	off := build(t, 0)
	for name, table := range tables(t, off) {
		for _, column := range table.Columns {
			if strings.Contains(column.Type, "(0)") {
				t.Errorf("table %s column %s is %s", name, column.Name, column.Type)
			}
		}
		for _, index := range table.Indexes {
			if slices.Contains(index.Keys, "description") {
				t.Errorf("table %s indexes a switched-off description: %+v", name, index)
			}
		}
	}
	accounts, _ := PhysicalCatalogTable(off.ChartsOfAccounts[0].ID)
	calculation, _ := PhysicalCatalogTable(off.ChartsOfCalculationTypes[0].ID)
	task, _ := PhysicalCatalogTable(off.Tasks[0].ID)
	plan, _ := PhysicalCatalogTable(off.ExchangePlans[0].ID)
	built := tables(t, off)
	for _, name := range []string{accounts, calculation, task} {
		if slices.Contains(columns(built[name]), "description") {
			t.Errorf("table %s keeps a description of length 0: %v", name, columns(built[name]))
		}
	}
	if got := columns(built[plan]); slices.Contains(got, "code") || slices.Contains(got, "description") {
		t.Errorf("exchange plan keeps a field of length 0: %v", got)
	}
	for _, index := range built[plan].Indexes {
		if slices.Contains(index.Keys, "code") {
			t.Errorf("exchange plan indexes a code of length 0: %+v", index)
		}
	}
	for _, constraint := range built[plan].Constraints {
		if strings.Contains(constraint.Definition, "(code)") {
			t.Errorf("exchange plan keeps a constraint on a code of length 0: %+v", constraint)
		}
	}

	// With a length, the tables have the columns they always had. The schema
	// orders columns by name, so the set is what is compared.
	on := build(t, 1)
	built = tables(t, on)
	accounts, _ = PhysicalCatalogTable(on.ChartsOfAccounts[0].ID)
	calculation, _ = PhysicalCatalogTable(on.ChartsOfCalculationTypes[0].ID)
	task, _ = PhysicalCatalogTable(on.Tasks[0].ID)
	plan, _ = PhysicalCatalogTable(on.ExchangePlans[0].ID)
	for name, want := range map[string][]string{
		accounts:    {"ref", "version", "code", "description", "account_kind", "off_balance", "deletion_mark", "predefined_name"},
		calculation: {"ref", "version", "code", "description", "deletion_mark", "predefined_name", "action_period_is_base"},
		task:        {"ref", "version", "number", "date", "description", "executed", "business_process", "route_point", "deletion_mark"},
		plan:        {"ref", "version", "code", "description", "this_node", "sent_no", "received_no", "deletion_mark"},
	} {
		got := columns(built[name])
		for _, column := range want {
			if !slices.Contains(got, column) {
				t.Errorf("table %s lost column %s: %v", name, column, got)
			}
		}
	}
	if !slices.ContainsFunc(built[plan].Indexes, func(index schemadiff.Index) bool { return slices.Equal(index.Keys, []string{"code"}) }) {
		t.Errorf("exchange plan with a code lost its index by code: %+v", built[plan].Indexes)
	}
}

// noteChartBody writes an object of the kind given with the body given.
func noteChartBody(t *testing.T, root string, kind Kind, body string) {
	t.Helper()
	id := uuid.MustNew().String()
	writeMetadata(t, root, kind, id, "format: 1\nid: "+id+"\nname: Объект"+string(kind[:3])+"\ntitle: {ru: Объект}\n"+body)
}

// A chart of accounts that allows analytics and names no chart of kinds to
// take them from has none: its accounting register has no ext dimension
// columns and no standard fields Субконто1..N, and a role names no code or
// description an object of length 0 does not have.
//
// Defect caught: the register offering Субконто1..3 as standard fields over
// columns the schema never built, and a role granting rights to a field of
// length 0 - a right over nothing.
func TestHalfSetAnalyticsAndSwitchedOffFieldsOfferNothing(t *testing.T) {
	t.Parallel()
	chart := ChartOfAccountsDefinition{ID: uuid.MustNew(), Name: "Хозрасчетный", Code: CatalogCode{Type: StringType, Length: 5}, DescriptionLength: 0, MaxExtDimensionCount: 3}
	register := AccountingRegisterDefinition{ID: uuid.MustNew(), Name: "Журнал", ChartOfAccounts: chart.ID}
	plan := ExchangePlanDefinition{ID: uuid.MustNew(), Name: "Филиалы", Code: CatalogCode{Type: StringType, Length: 0}, DescriptionLength: 0}
	task := TaskDefinition{ID: uuid.MustNew(), Name: "Задача", Number: DocumentNumber{Type: StringType, Length: 11}, DescriptionLength: 0}
	types := ChartOfCalculationTypesDefinition{ID: uuid.MustNew(), Name: "Начисления", Code: CatalogCode{Type: StringType, Length: 5}, DescriptionLength: 0}
	catalog := &Catalog{
		ChartsOfAccounts: []ChartOfAccountsDefinition{chart}, chartOfAccountsByID: map[uuid.UUID]int{chart.ID: 0},
		AccountingRegisters: []AccountingRegisterDefinition{register}, accountingRegisterByID: map[uuid.UUID]int{register.ID: 0},
		ExchangePlans: []ExchangePlanDefinition{plan}, exchangePlanByID: map[uuid.UUID]int{plan.ID: 0},
		Tasks: []TaskDefinition{task}, taskByID: map[uuid.UUID]int{task.ID: 0},
		ChartsOfCalculationTypes: []ChartOfCalculationTypesDefinition{types}, chartOfCalculationTypesByID: map[uuid.UUID]int{types.ID: 0},
	}
	if chart.EffectiveMaxExtDimensionCount() != 0 {
		t.Fatal("a chart with no chart of kinds has analytics")
	}
	for _, field := range catalog.standardAttributeFields(AccountingRegisterKind, register.ID) {
		if strings.HasPrefix(field.ru, "Субконто") {
			t.Fatalf("the register offers %s over a chart that has no analytics", field.ru)
		}
	}
	table := schemadiff.Table{}
	catalog.appendExtDimensionColumns(&table, register)
	if len(table.Columns) != 0 {
		t.Fatalf("the register lays out analytics: %+v", table.Columns)
	}
	for name, id := range map[string]uuid.UUID{"chart of accounts": chart.ID, "exchange plan": plan.ID, "task": task.ID, "chart of calculation types": types.ID} {
		target, ok := catalog.permissionTarget(id)
		if !ok {
			t.Fatalf("%s: no permission target", name)
		}
		if _, has := target.fields["description"]; has {
			t.Errorf("%s: a role may be granted the description of length 0", name)
		}
	}
	if target, _ := catalog.permissionTarget(plan.ID); func() bool { _, has := target.fields["code"]; return has }() {
		t.Error("exchange plan: a role may be granted the code of length 0")
	}
}
