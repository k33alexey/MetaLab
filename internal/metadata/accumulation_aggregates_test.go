package metadata

import (
	"strings"
	"testing"
)

const (
	aggregateRegisterID  = "a6300000-0000-4000-8000-000000000001"
	aggregateGoodID      = "a6300000-0000-4000-8000-000000000010"
	aggregateStoreID     = "a6300000-0000-4000-8000-000000000011"
	aggregateQuantityID  = "a6300000-0000-4000-8000-000000000012"
	aggregateAmountIDVal = "a6300000-0000-4000-8000-000000000013"
)

func aggregateRegisterBody(body string) string {
	return `format: 1
id: ` + aggregateRegisterID + `
name: ПродажиОбороты
title: {ru: Продажи обороты}
kind: turnover
dimensions:
  - {id: ` + aggregateGoodID + `, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: string, length: 50}]}
  - {id: ` + aggregateStoreID + `, name: Склад, title: {ru: Склад}, types: [{kind: string, length: 50}]}
resources:
  - {id: ` + aggregateQuantityID + `, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}
  - {id: ` + aggregateAmountIDVal + `, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}
` + body
}

// The mechanism itself: a subordinate entity with its own dimensions,
// periodicity and use, none of which the register had.
func TestARegisterCarriesItsAggregates(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, AccumulationRegisterKind, aggregateRegisterID, aggregateRegisterBody(`aggregates:
  - periodicity: month
    use: always
    dimensions: [Номенклатура, Склад]
  - periodicity: year
    use: auto
    dimensions: [Склад]
  - periodicity: non-periodic
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, ok := catalog.AccumulationRegisterDefinition("ПродажиОбороты")
	if !ok || len(register.Aggregates) != 3 {
		t.Fatalf("the aggregates were lost: %+v found=%v", register.Aggregates, ok)
	}
	switch first := register.Aggregates[0]; {
	case first.Periodicity != AggregatePeriodicityMonth:
		t.Fatalf("the periodicity was lost: %+v", first)
	case first.Use != AggregateUseAlways:
		t.Fatalf("the use was lost: %+v", first)
	case len(first.Dimensions) != 2 || first.Dimensions[0] != "Номенклатура":
		t.Fatalf("the dimensions were lost: %+v", first)
	}
	// An aggregate over no dimensions at all is the coarsest cut there is, and
	// it is a legitimate one - folded by period and nothing else.
	if last := register.Aggregates[2]; len(last.Dimensions) != 0 || last.Periodicity != AggregatePeriodicityNonPeriodic {
		t.Fatalf("the aggregate over no dimensions was not kept as written: %+v", last)
	}
}

// Seven periodicities and two uses, and the sets are the aggregate's own: the
// periodicity here is not the one an information register has - no week, no
// second - and the use is not the three-valued switch fields carry.
func TestAnAggregateTakesOnlyItsOwnValues(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		body    string
		refusal string
	}{
		"авто":            {"periodicity: auto\n", ""},
		"непериодический": {"periodicity: non-periodic\n", ""},
		"день":            {"periodicity: day\n", ""},
		"месяц":           {"periodicity: month\n", ""},
		"квартал":         {"periodicity: quarter\n", ""},
		"полугодие":       {"periodicity: half-year\n", ""},
		"год":             {"periodicity: year\n", ""},
		"недели у агрегата нет":      {"periodicity: week\n", "periodicity must be auto"},
		"секунды тоже нет":           {"periodicity: second\n", "periodicity must be auto"},
		"использование всегда":       {"use: always\n", ""},
		"использование авто":         {"use: auto\n", ""},
		"третьего использования нет": {"use: dont-use\n", "use must be always or auto"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, AccumulationRegisterKind, aggregateRegisterID,
				aggregateRegisterBody("aggregates:\n  - "+want.body))
			_, err := Load(root)
			switch {
			case want.refusal == "" && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case want.refusal != "":
				if err == nil || !strings.Contains(err.Error(), want.refusal) {
					t.Fatalf("%s: %v", name, err)
				}
			}
		})
	}
}

// Two refusals, and they are refusals for different reasons. A dimension the
// register has not got describes a cut nothing can build. Two aggregates
// holding the same thing are not two answers to one question but one answer
// stored twice - and since an aggregate has no name, nothing tells them apart.
func TestAnAggregateIsRefusedWhenItCannotBeBuiltOrIsACopy(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		body    string
		refusal string
	}{
		"измерения у регистра нет": {`aggregates:
  - {periodicity: month, dimensions: [Контрагент]}
`, "which is not a dimension of this register"},
		"ресурс измерением не станет": {`aggregates:
  - {periodicity: month, dimensions: [Количество]}
`, "which is not a dimension of this register"},
		"измерение повторено внутри агрегата": {`aggregates:
  - {periodicity: month, dimensions: [Склад, Склад]}
`, "repeats Склад"},
		"тот же состав, но другая периодичность — разные разрезы": {`aggregates:
  - {periodicity: month, dimensions: [Номенклатура, Склад]}
  - {periodicity: year, dimensions: [Номенклатура, Склад]}
`, ""},
		"вложенный состав — разные разрезы": {`aggregates:
  - {periodicity: month, dimensions: [Номенклатура, Склад]}
  - {periodicity: month, dimensions: [Склад]}
`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, AccumulationRegisterKind, aggregateRegisterID, aggregateRegisterBody(want.body))
			_, err := Load(root)
			switch {
			case want.refusal == "" && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case want.refusal != "":
				if err == nil || !strings.Contains(err.Error(), want.refusal) {
					t.Fatalf("%s: %v", name, err)
				}
			}
		})
	}
}

// A balance register with aggregates is carried with a note: the
// configurator keeps the aggregates of a turnover register changed to
// balances, hidden, and its check of the configuration does not refuse them
// (block 00, 15.10.2026). Defects caught: such a register refused, as the
// map once asked on the strength of the window alone; carried silently, as
// until 2.338, so nothing said its aggregates act on nothing; and the note
// given to a turnover register, whose aggregates do act.
func TestAggregatesOnABalanceRegisterAreKeptNotRefused(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, AccumulationRegisterKind, aggregateRegisterID, `format: 1
id: `+aggregateRegisterID+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: balance
allow_totals_splitting: true
dimensions:
  - {id: `+aggregateGoodID+`, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: string, length: 50}]}
resources:
  - {id: `+aggregateQuantityID+`, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}]}
aggregates:
  - {periodicity: day, use: auto, dimensions: [Номенклатура]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// Totals splitting beside aggregates is not a contradiction either: which
	// of the two a register runs on is a setting of the working database, and
	// the configuration may declare both.
	register, _ := catalog.AccumulationRegisterDefinition("ОстаткиТоваров")
	if len(register.Aggregates) != 1 || !register.AllowTotalsSplitting {
		t.Fatalf("the declaration was not kept as written: %+v", register.Aggregates)
	}
	notes := catalog.Notes()
	if len(notes) != 1 || notes[0].Kind != NoteAggregatesOfBalance || notes[0].Where != "accumulation-registers ОстаткиТоваров aggregates" {
		t.Fatalf("notes = %+v, expected the aggregates of the balance register", notes)
	}

	turnover := metadataProject(t)
	writeMetadata(t, turnover, AccumulationRegisterKind, aggregateRegisterID,
		aggregateRegisterBody("aggregates:\n  - {periodicity: month, dimensions: [Номенклатура]}\n"))
	catalog, err = Load(turnover)
	if err != nil {
		t.Fatal(err)
	}
	if notes := catalog.Notes(); len(notes) != 0 {
		t.Fatalf("a turnover register with aggregates was noted: %+v", notes)
	}
}
