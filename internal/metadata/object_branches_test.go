package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/icons"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Every kind shows under an object the branches the prototype's tree does, in
// its order (inventory of block 00, «Ветки под объектом», snapshots derevo/
// of 19.10.2026); the expectation is written here from the snapshots, not
// read off partGroups. Defects caught: a kind with parts shown without a
// branch of them, or with one it has not (the fields a criterion of filter
// searches by shown as «Поля»); branches out of the prototype's order (the
// flags of a chart of accounts after its table parts, the addressing of a task
// after its attributes); commands or standard attributes given a branch of
// parts beside the tree's «Команды».
func TestEveryKindShowsTheBranchesOfThePrototypesTree(t *testing.T) {
	t.Parallel()
	ordinary := []PartGroup{AttributesPart, TablePartsPart}
	register := []PartGroup{DimensionsPart, ResourcesPart, AttributesPart}
	want := map[Kind][]PartGroup{
		CatalogKind: ordinary, DocumentKind: ordinary, ChartOfCharacteristicTypesKind: ordinary, ChartOfCalculationTypesKind: ordinary,
		ExchangePlanKind: ordinary, BusinessProcessKind: ordinary, ReportKind: ordinary, DataProcessorKind: ordinary,
		ChartOfAccountsKind:     {AttributesPart, AccountingFlagsPart, ExtDimensionAccountingFlagsPart, TablePartsPart},
		TaskKind:                {AddressingAttributesPart, AttributesPart, TablePartsPart},
		InformationRegisterKind: register, AccumulationRegisterKind: register, AccountingRegisterKind: register, CalculationRegisterKind: register,
		ExternalDataSourceTableKind: {FieldsPart}, ExternalDimensionTableKind: {FieldsPart}, ExternalCubeKind: {DimensionsPart, ResourcesPart},
	}
	for _, item := range kindInfos {
		if got := branchesOf(item.kind); !slices.Equal(got, want[item.kind]) {
			t.Errorf("%s shows the branches %v, want %v", item.kind, got, want[item.kind])
		}
	}
}

// The branches of an object hold what it declares of each sort, a table part
// with its fields, and not a common attribute taken into it; a catalog that
// is not there gives every branch empty. Defects caught: a part put in the
// branch of another sort; the fields of a table part left out or shown as the
// object's own; a common attribute shown as the object's; the tree breaking
// on a project that does not load.
func TestObjectBranchesHoldWhatTheObjectDeclares(t *testing.T) {
	t.Parallel()
	id := func() uuid.UUID { return uuid.MustNew() }
	chart, task, attribute, common, flag, subcontoFlag, part, partField, addressing, taskAttribute := id(), id(), id(), id(), id(), id(), id(), id(), id(), id()
	catalog := &Catalog{
		chartOfAccountsByID: map[uuid.UUID]int{chart: 0},
		ChartsOfAccounts: []ChartOfAccountsDefinition{{ID: chart,
			Attributes:                  []Attribute{{ID: attribute, Name: "Вид"}, {ID: common, Name: "Организация"}},
			AccountingFlags:             []AccountingFlag{{Attribute: Attribute{ID: flag, Name: "Валютный"}}},
			ExtDimensionAccountingFlags: []AccountingFlag{{Attribute: Attribute{ID: subcontoFlag, Name: "Суммовой"}}},
			TableParts:                  []TablePart{{ID: part, Name: "ВидыСубконто", Attributes: []Attribute{{ID: partField, Name: "ВидСубконто"}}}}}},
		taskByID: map[uuid.UUID]int{task: 0},
		Tasks: []TaskDefinition{{ID: task, Attributes: []Attribute{{ID: taskAttribute, Name: "Описание"}},
			AddressingAttributes: []AddressingAttribute{{ID: addressing, Name: "Исполнитель"}}}},
		commonAttributeByID: map[uuid.UUID]int{common: 0},
	}
	type held struct {
		group PartGroup
		names []string
	}
	shown := func(branches []ObjectBranch) []held {
		var result []held
		for _, branch := range branches {
			var names []string
			for _, item := range branch.Parts {
				names = append(names, item.Name)
				for _, inside := range item.Parts {
					names = append(names, item.Name+"."+inside.Name)
				}
			}
			result = append(result, held{branch.Group, names})
		}
		return result
	}
	for _, check := range []struct {
		name string
		got  []held
		want []held
	}{
		{"a chart of accounts", shown(catalog.ObjectBranches(ChartOfAccountsKind, chart)), []held{{AttributesPart, []string{"Вид"}},
			{AccountingFlagsPart, []string{"Валютный"}}, {ExtDimensionAccountingFlagsPart, []string{"Суммовой"}},
			{TablePartsPart, []string{"ВидыСубконто", "ВидыСубконто.ВидСубконто"}}}},
		{"a task", shown(catalog.ObjectBranches(TaskKind, task)), []held{{AddressingAttributesPart, []string{"Исполнитель"}},
			{AttributesPart, []string{"Описание"}}, {TablePartsPart, nil}}},
		{"no catalog", shown((*Catalog)(nil).ObjectBranches(TaskKind, task)), []held{{AddressingAttributesPart, nil}, {AttributesPart, nil}, {TablePartsPart, nil}}},
	} {
		if !slices.EqualFunc(check.got, check.want, func(a, b held) bool { return a.group == b.group && slices.Equal(a.names, b.names) }) {
			t.Errorf("%s shows %v, want %v", check.name, check.got, check.want)
		}
	}
}

// Every branch of the tree is drawn by an icon the set has. Defects caught: a
// sort of part given an icon the set does not draw, which would stand in the
// tree as an empty square, branch and parts alike.
func TestEveryBranchOfTheTreeIsDrawn(t *testing.T) {
	t.Parallel()
	for _, sort := range partGroups {
		if sort.icon != "" && !icons.Has(sort.icon) {
			t.Errorf("%s names icon %q, which the set does not draw", sort.group, sort.icon)
		}
	}
}
