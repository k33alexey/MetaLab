package metadata

import (
	"context"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestAccumulationDimensionKeyIsStableAndOrdered(t *testing.T) {
	t.Parallel()
	firstID, secondID := uuid.MustNew(), uuid.MustNew()
	definition := AccumulationRegisterDefinition{ID: uuid.MustNew(), Dimensions: []Attribute{{ID: firstID}, {ID: secondID}}}
	values := map[uuid.UUID]Value{firstID: {Kind: StringType, Data: "A"}, secondID: {Kind: NumberType, Data: "1"}}
	first := accumulationDimensionKey(definition, values)
	second := accumulationDimensionKey(definition, map[uuid.UUID]Value{secondID: values[secondID], firstID: values[firstID]})
	if first != second || len(first) != 64 {
		t.Fatalf("keys %q and %q", first, second)
	}
	definition.Dimensions[0], definition.Dimensions[1] = definition.Dimensions[1], definition.Dimensions[0]
	if accumulationDimensionKey(definition, values) == first {
		t.Fatal("metadata dimension order was ignored")
	}
}

func TestNormalizeAccumulationPeriod(t *testing.T) {
	t.Parallel()
	value := time.Date(2026, 9, 7, 12, 30, 0, 123456789, time.FixedZone("test", 2*60*60))
	got, err := normalizeAccumulationPeriod(value)
	if err != nil || got.Location() != time.UTC || got.Nanosecond()%100000 != 0 {
		t.Fatalf("period=%s error=%v", got, err)
	}
}

func TestMergeAccumulationAggregates(t *testing.T) {
	t.Parallel()
	dimensionID, resourceID := uuid.MustNew(), uuid.MustNew()
	definition := AccumulationRegisterDefinition{ID: uuid.MustNew(), Dimensions: []Attribute{{ID: dimensionID}}, Resources: []Attribute{{ID: resourceID}}}
	dimensions := map[uuid.UUID]Value{dimensionID: {Kind: StringType, Data: "A"}}
	result, err := mergeAccumulationAggregates(definition,
		[]AccumulationRegisterAggregate{{Dimensions: dimensions, Turnover: map[uuid.UUID]Value{resourceID: {Kind: NumberType, Data: "10"}}}},
		[]AccumulationRegisterAggregate{{Dimensions: dimensions, Turnover: map[uuid.UUID]Value{resourceID: {Kind: NumberType, Data: "-3"}}, Receipt: map[uuid.UUID]Value{resourceID: {Kind: NumberType, Data: "2"}}, Expense: map[uuid.UUID]Value{resourceID: {Kind: NumberType, Data: "5"}}}},
	)
	if err != nil || len(result) != 1 || result[0].Closing[resourceID].Data != "7" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestAccumulationRegisterEventCancellation(t *testing.T) {
	t.Parallel()
	err := dispatchAccumulationRegisterEvent(t.Context(), AccumulationRegisterEventHandlerFunc(func(context.Context, AccumulationRegisterEvent, *AccumulationRegisterRecordSet, bool) (bool, error) {
		return true, nil
	}), AccumulationRegisterEventBeforeWrite, &AccumulationRegisterRecordSet{}, true)
	if err != ErrAccumulationRegisterWriteCancelled {
		t.Fatalf("error=%v", err)
	}
}
