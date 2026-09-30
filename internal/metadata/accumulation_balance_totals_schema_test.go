package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Which dimension of a balance register gets an index of the period and
// itself, and which does not. The defect on each side is different: missing
// the index leaves a filter on that dimension walking the whole period, and an
// index for the first dimension, for an unmarked one or for one of unbounded
// size is a cost on every write that nothing reads - or, for the unbounded one,
// a posting refused when the value outgrows an index entry.
func TestBalanceTotalsIndexAnIndexedDimensionFromTheSecondOn(t *testing.T) {
	t.Parallel()
	first, marked, unbounded, unmarked, resource := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	definition := AccumulationRegisterDefinition{
		ID: uuid.MustNew(), Name: "Остатки", Kind: AccumulationRegisterBalance,
		Dimensions: []RegisterDimension{
			{Attribute: Attribute{ID: first, Name: "Первое", Indexing: IndexField, Types: []Type{{Kind: StringType, Length: 20}}}},
			{Attribute: Attribute{ID: marked, Name: "Отмеченное", Indexing: IndexWithAdditionalOrder, Types: []Type{{Kind: StringType, Length: 50}}}},
			{Attribute: Attribute{ID: unbounded, Name: "Безграничное", Indexing: IndexField, Types: []Type{{Kind: StringType}}}},
			{Attribute: Attribute{ID: unmarked, Name: "Неотмеченное", Types: []Type{{Kind: NumberType, Precision: 10}}}},
		},
		Resources: []Attribute{{ID: resource, Name: "Количество", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}},
	}
	catalog := &Catalog{}
	_, totals, err := catalog.accumulationRegisterTables(definition)
	if err != nil {
		t.Fatal(err)
	}
	markedColumn, _ := PhysicalAttributeColumn(marked)
	var dimensionIndexes [][]string
	for _, index := range totals.Indexes {
		if index.Name == physicalObjectName("itd", marked) && !slices.Equal(index.Keys, []string{"total_period", markedColumn}) {
			t.Fatalf("the index of the marked dimension is %v", index.Keys)
		}
		for _, id := range []uuid.UUID{first, marked, unbounded, unmarked} {
			if index.Name == physicalObjectName("itd", id) {
				dimensionIndexes = append(dimensionIndexes, index.Keys)
			}
		}
	}
	if len(dimensionIndexes) != 1 {
		t.Fatalf("dimension indexes of the balances = %v, want only the marked second dimension", dimensionIndexes)
	}

	// A turnover register's totals are not read by anything yet, so its
	// marked dimensions get nothing here - that belongs to the reading of
	// turnovers from totals, which is its own point.
	definition.Kind = AccumulationRegisterTurnover
	_, totals, err = catalog.accumulationRegisterTables(definition)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range totals.Indexes {
		if index.Name == physicalObjectName("itd", marked) {
			t.Fatalf("a turnover register got an index of the balances: %v", index.Keys)
		}
	}
}
