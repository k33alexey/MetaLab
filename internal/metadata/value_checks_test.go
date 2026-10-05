package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A type description reaches past a defined type and a chart of
// characteristic types to every type that follows them in the list - and so
// does the question whether the description is open to the configuration.
//
// Defect caught: the walk stopping after the first defined type or chart it
// expanded, dropping the types after it.
func TestTypeExpansionGoesOnPastADefinedTypeAndAChart(t *testing.T) {
	t.Parallel()
	defined, chart := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		DefinedTypes:                   []DefinedTypeObject{{ID: defined, Types: []Type{{Kind: StringType, Length: 10}}}},
		ChartsOfCharacteristicTypes:    []ChartOfCharacteristicTypesDefinition{{ID: chart, ValueType: []Type{{Kind: BooleanType}}}},
		definedTypeByID:                map[uuid.UUID]int{defined: 0},
		chartOfCharacteristicTypesByID: map[uuid.UUID]int{chart: 0},
	}
	for name, first := range map[string]Type{
		"defined type": {Kind: DefinedType, Reference: &defined},
		"chart":        {Kind: CharacteristicSet, Reference: &chart},
	} {
		expanded, err := catalog.expandTypes([]Type{first, {Kind: NumberType, Precision: 5}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(expanded) != 2 || expanded[1].Kind != NumberType {
			t.Fatalf("%s: expanded to %+v, the number after it lost", name, expanded)
		}
	}
	open, err := catalog.typesOpenToConfiguration([]Type{{Kind: DefinedType, Reference: &defined}, {Kind: CatalogSet}}, nil)
	if err != nil || !open {
		t.Fatalf("a set after a closed defined type: open = %v, %v", open, err)
	}
}

// A reference is allowed when the description names that very object.
//
// Defect caught: a reference with its object named refused.
func TestAReferenceToTheNamedObjectIsAllowed(t *testing.T) {
	t.Parallel()
	object, other := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{}
	types := []Type{{Kind: CatalogType, Reference: &object}}
	if allowed, err := catalog.allowsObjectReference(types, CatalogType, object); err != nil || !allowed {
		t.Fatalf("the named catalog: %v, %v", allowed, err)
	}
	if allowed, _ := catalog.allowsObjectReference(types, CatalogType, other); allowed {
		t.Fatal("another catalog allowed")
	}
}

// Values at the edges of their types: a string of exactly its length, the
// two words of a boolean, the first and the last year a date may hold.
//
// Defect caught: the longest string refused, a boolean accepted from any
// word, the empty date or the last day of 3999 refused.
func TestValuesAtTheEdgesOfTheirTypes(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{}
	for _, test := range []struct {
		value  Value
		allow  Type
		fits   bool
		reason string
	}{
		{Value{Kind: StringType, Data: "абв"}, Type{Kind: StringType, Length: 3}, true, ""},
		{Value{Kind: StringType, Data: "абвг"}, Type{Kind: StringType, Length: 3}, false, "string exceeds 3 characters"},
		{Value{Kind: BooleanType, Data: "true"}, Type{Kind: BooleanType}, true, ""},
		{Value{Kind: BooleanType, Data: "false"}, Type{Kind: BooleanType}, true, ""},
		{Value{Kind: BooleanType, Data: "да"}, Type{Kind: BooleanType}, false, "boolean must be true or false"},
		{Value{Kind: DateType, Data: "0001-01-01T00:00:00Z"}, Type{Kind: DateType}, true, ""},
		{Value{Kind: DateType, Data: "3999-12-31T23:59:59Z"}, Type{Kind: DateType}, true, ""},
		{Value{Kind: DateType, Data: "4000-01-01T00:00:00Z"}, Type{Kind: DateType}, false, "date must be RFC3339 within years 1..3999"},
	} {
		_, ok, reason := catalog.normalizeAs(test.value, test.allow)
		if ok != test.fits || !strings.Contains(reason, test.reason) {
			t.Fatalf("%s %q: fits = %v (%q), want %v (%q)", test.value.Kind, test.value.Data, ok, reason, test.fits, test.reason)
		}
	}
}
