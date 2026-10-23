package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// The tree «Состав» of a filter criterion offers the attributes of eight kinds
// (opis of the filter criterion, 14.10.2026, snapshot 10: the charts of
// accounts among them) and of no other. The kinds are written here, not read
// from kindUsesOf; a kind saying nothing of it fails, so a new kind is not
// left out by omission.
//
// Defect caught: a chart of accounts refused in the content, which the
// designer offers - seven kinds, as the first pass of 02.10.2026 had it; a
// register or a report let in.
func TestTheContentOfACriterionTakesThePrototypesKinds(t *testing.T) {
	t.Parallel()
	offered := []Kind{CatalogKind, DocumentKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind,
		ChartOfCalculationTypesKind, BusinessProcessKind, TaskKind, ExchangePlanKind}
	for _, item := range kinds.All() {
		got := usesOf(item.Kind).criterion
		want := absent
		if slices.Contains(offered, item.Kind) {
			want = present
		}
		switch {
		case got == unsaid:
			t.Errorf("%s says nothing of the content of a criterion", item.Kind)
		case got != want:
			t.Errorf("the content of a criterion takes %s: %d, the prototype's %d", item.Kind, got, want)
		}
	}
}

// A criterion searches an attribute of a chart of accounts and an attribute of
// its table part, as it searches those of a document.
//
// Defect caught: the chart of accounts let into the list of kinds and refused
// all the same where the field is resolved.
func TestACriterionSearchesAChartOfAccounts(t *testing.T) {
	t.Parallel()
	root := criterionProject(t)
	chart, attribute, part, partAttribute := "d2000000-0000-4000-8000-000000000030", "d2000000-0000-4000-8000-000000000031",
		"d2000000-0000-4000-8000-000000000032", "d2000000-0000-4000-8000-000000000033"
	writeMetadata(t, root, ChartOfAccountsKind, chart, `format: 1
id: `+chart+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
order_length: 5
attributes:
  - {id: `+attribute+`, name: Договор, title: {ru: Договор}, types: [{kind: catalog, reference: `+criterionContracts+`}]}
table_parts:
  - id: `+part+`
    name: Договоры
    title: {ru: Договоры}
    attributes:
      - {id: `+partAttribute+`, name: Договор, title: {ru: Договор}, types: [{kind: catalog, reference: `+criterionContracts+`}]}
`)
	folder := string(ChartOfAccountsKind)
	writeMetadata(t, root, FilterCriterionKind, criterionID, `format: 1
id: `+criterionID+`
name: ПоДоговору
title: {ru: По договору}
types: [{kind: catalog, reference: `+criterionContracts+`}]
fields:
  - {kind: `+folder+`, object: `+chart+`, attribute: `+attribute+`}
  - {kind: `+folder+`, object: `+chart+`, table_part: `+part+`, attribute: `+partAttribute+`}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	criterion, ok := catalog.FilterCriterion("ПоДоговору")
	if !ok || len(criterion.Fields) != 2 || criterion.Fields[0].Kind != ChartOfAccountsKind {
		t.Fatalf("criterion = %+v", criterion)
	}
}
