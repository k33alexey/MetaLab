package bytecode

import (
	"sync"
	"testing"
)

func TestCollectionConstructorsRejectInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arguments []Value
	}{
		{name: "Unknown"},
		{name: "Array", arguments: []Value{Number(-1)}},
		{name: "Structure", arguments: []Value{String("1Invalid")}},
		{name: "Map", arguments: []Value{Number(1)}},
	}
	for _, test := range tests {
		if _, err := ConstructCollection(test.name, test.arguments); err == nil {
			t.Fatalf("ConstructCollection(%q) accepted invalid input", test.name)
		}
	}
	if _, err := ConstructCollectionWithin("Array", []Value{Number(100)}, 128); err == nil {
		t.Fatal("collection constructor ignored memory limit")
	}
}

func TestCollectionReferenceIdentityAndCyclesAreBounded(t *testing.T) {
	t.Parallel()

	array, err := ConstructCollection("Array", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CollectionMethod(array, "Add", []Value{array}); err != nil {
		t.Fatal(err)
	}
	if !SameReference(array, array) || ValuesEqual(array, Array()) {
		t.Fatal("collection reference equality is invalid")
	}
	if size, ok := array.DynamicMemory(1 << 20); !ok || size == 0 {
		t.Fatalf("cyclic collection memory = %d, %v", size, ok)
	}
}

func TestArrayMutationIsSafeForConcurrentCallers(t *testing.T) {
	array := Array()
	const workers = 100
	var wait sync.WaitGroup
	for index := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := CollectionMethod(array, "Add", []Value{Number(float64(index))}); err != nil {
				t.Errorf("Add: %v", err)
			}
		}()
	}
	wait.Wait()
	result, err := CollectionMethod(array, "Count", nil)
	if err != nil || result.String() != "100" {
		t.Fatalf("Count() = %v, %v", result, err)
	}
}

func TestStructureMapAndTableCollectionAPI(t *testing.T) {
	t.Parallel()

	structure, err := ConstructCollection("Structure", []Value{String("Name"), String("MetaLab")})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetCollectionProperty(structure, "Name", String("ML")); err != nil {
		t.Fatal(err)
	}
	if err := SetCollectionProperty(structure, "Missing", Number(1)); err == nil {
		t.Fatal("Structure accepted an unknown property assignment")
	}

	mapping, _ := ConstructCollection("Map", nil)
	if err := SetCollectionIndex(mapping, String("key"), structure); err != nil {
		t.Fatal(err)
	}
	if value, err := CollectionIndex(mapping, String("key")); err != nil || !SameReference(value, structure) {
		t.Fatalf("Map index = %v, %v", value, err)
	}

	table, _ := ConstructCollection("ValueTable", nil)
	columns, _ := CollectionProperty(table, "Columns")
	if _, err := CollectionMethod(columns, "Add", []Value{String("Amount")}); err != nil {
		t.Fatal(err)
	}
	row, err := CollectionMethod(table, "Add", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetCollectionProperty(row, "Amount", Number(42)); err != nil {
		t.Fatal(err)
	}
	if value, err := CollectionProperty(row, "Amount"); err != nil || value.String() != "42" {
		t.Fatalf("row Amount = %v, %v", value, err)
	}
}
