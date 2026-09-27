package metadata

import (
	"strings"
	"testing"
)

const (
	registerPropsID        = "60000000-0000-4000-8000-000000000051"
	registerPropsDimension = "60000000-0000-4000-8000-000000000052"
	registerPropsResource  = "60000000-0000-4000-8000-000000000053"
)

func informationRegisterYAML(body string) string {
	return `format: 1
id: ` + registerPropsID + `
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: day
dimensions:
  - id: ` + registerPropsDimension + `
    name: Товар
    title: {ru: Товар}
    types: [{kind: string, length: 50}]
resources:
  - id: ` + registerPropsResource + `
    name: Цена
    title: {ru: Цена}
    types: [{kind: number, precision: 15, scale: 2}]
` + body
}

// Four properties belong to the information register alone, and all four were
// read and dropped before this point.
func TestInformationRegisterKeepsItsOwnProperties(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, InformationRegisterKind, registerPropsID, informationRegisterYAML(`edit_type: both-ways
main_filter_on_period: true
totals:
  slice_first: true
  slice_last: true
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.InformationRegisterDefinition("цены")
	if !ok {
		t.Fatal("the register was not read")
	}
	if register.EditType != EditBothWays || !register.MainFilterOnPeriod ||
		!register.Totals.SliceFirst || !register.Totals.SliceLast {
		t.Fatalf("register = edit=%q filter=%v totals=%+v",
			register.EditType, register.MainFilterOnPeriod, register.Totals)
	}
}

// The way of editing has the three values the prototype's enumeration has. It is
// one enumeration shared across many kinds of object, so getting its values right
// here is worth more than one register.
func TestEditTypeTakesOnlyItsThreeValues(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		value   string
		accepts bool
	}{
		"в диалоге":        {"in-dialog", true},
		"в списке":         {"in-list", true},
		"обоими способами": {"both-ways", true},
		"четвёртого нет":   {"in-form", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, InformationRegisterKind, registerPropsID,
				informationRegisterYAML("edit_type: "+want.value+"\n"))
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("refused: %v", err)
			case !want.accepts && (err == nil || !strings.Contains(err.Error(), "edit_type must be")):
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// A non-periodic register may still say these things. In the demonstration
// configuration every register that sets one of them is periodic and all 234
// non-periodic ones leave all three off - but the help states no such rule, and
// the export proves nothing by silence. Refusing a configuration the prototype
// accepts is a worse failure than carrying a setting that says little, so this
// test pins the permissiveness rather than leaving it to chance.
func TestSlicesAndPeriodFilterDoNotRequirePeriodicity(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, InformationRegisterKind, registerPropsID, `format: 1
id: `+registerPropsID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: none
dimensions:
  - id: `+registerPropsDimension+`
    name: Товар
    title: {ru: Товар}
    types: [{kind: string, length: 50}]
resources:
  - id: `+registerPropsResource+`
    name: Цена
    title: {ru: Цена}
    types: [{kind: number, precision: 15, scale: 2}]
main_filter_on_period: true
totals: {slice_first: true, slice_last: true}
`)
	if _, err := Load(root); err != nil {
		t.Fatalf("a rule the help does not state refused a configuration: %v", err)
	}
}

// The refinement of a period is an integer from 0 to 3 - the help says so
// outright. Zero means the register does not support refinement, which is what
// both accounting registers of the demonstration configuration say.
func TestPeriodAdjustmentLengthIsBoundedByThePrototype(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		length  string
		accepts bool
	}{
		"ноль — нет уточнения": {"0", true},
		"три — максимум":       {"3", true},
		"четыре — уже нет":     {"4", false},
		"отрицательного нет":   {"-1", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ChartOfCharacteristicTypesKind, characteristicsID, accountsChartYAML)
			writeMetadata(t, root, ChartOfAccountsKind, accountsID, `format: 1
id: `+accountsID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
`)
			writeMetadata(t, root, AccountingRegisterKind, accountingStandardID, `format: 1
id: `+accountingStandardID+`
name: Хозрасчетный
title: {ru: Хозрасчётный}
chart_of_accounts: `+accountsID+`
period_adjustment_length: `+want.length+`
resources:
  - id: `+accountingResourceID+`
    name: Сумма
    title: {ru: Сумма}
    types: [{kind: number, precision: 15, scale: 2}]
`)
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("refused: %v", err)
			case !want.accepts && (err == nil || !strings.Contains(err.Error(), "period_adjustment_length must be 0..3")):
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// Splitting the totals belongs to the accumulation and the accounting register,
// and to neither of the other two. The accounting one has carried it all along;
// this is the accumulation register finally able to say it.
func TestTotalsSplittingIsCarriedByTheTwoRegistersThatHaveIt(t *testing.T) {
	t.Parallel()
	t.Run("регистр накопления", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, AccumulationRegisterKind, accumulationStandardID, `format: 1
id: `+accumulationStandardID+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: balance
totals_splitting: true
dimensions:
  - id: `+accumulationDimensionID+`
    name: Товар
    title: {ru: Товар}
    types: [{kind: string, length: 50}]
resources:
  - id: `+accumulationResourceID+`
    name: Количество
    title: {ru: Количество}
    types: [{kind: number, precision: 15, scale: 3}]
`)
		catalog, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		register, ok := catalog.AccumulationRegisterDefinition("остаткитоваров")
		if !ok || !register.TotalsSplitting {
			t.Fatalf("register = %+v, found=%v", register.TotalsSplitting, ok)
		}
	})
	// A register of calculations has no totals at all, so the key is refused
	// where the file is read rather than by a check of its own.
	t.Run("у регистра расчёта его нет", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, CalculationRegisterKind, calcRegisterStandardID, `format: 1
id: `+calcRegisterStandardID+`
name: Начисления
title: {ru: Начисления}
periodicity: month
totals_splitting: true
resources:
  - id: `+calcResourceStandardID+`
    name: Результат
    title: {ru: Результат}
    types: [{kind: number, precision: 15, scale: 2}]
`)
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "totals_splitting") {
			t.Fatalf("err = %v", err)
		}
	})
}
