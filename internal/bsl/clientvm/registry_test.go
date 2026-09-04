package clientvm

import (
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

func TestRegistryLoadsCallsAndReleasesMachine(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	handle := loadSource(t, registry, "Function Add(A, B) Return A + B; EndFunction")
	result, err := registry.Call(handle, "add", bytecode.Number(20), bytecode.Number(22))
	if err != nil {
		t.Fatal(err)
	}
	if number, ok := result.AsNumber(); !ok || number != 42 {
		t.Fatalf("result = %v, want 42", result)
	}
	if !registry.Release(handle) {
		t.Fatal("Release() = false")
	}
	if registry.Release(handle) {
		t.Fatal("second Release() = true")
	}
	if _, err := registry.Call(handle, "Add"); err == nil {
		t.Fatal("Call() succeeded after release")
	}
}

func TestRegistryRejectsInvalidBytecode(t *testing.T) {
	t.Parallel()

	if _, err := NewRegistry().Load([]byte("invalid")); err == nil {
		t.Fatal("Load() accepted invalid bytecode")
	}
}

func TestRegistrySupportsConcurrentCalls(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	handle := loadSource(t, registry, "Function Add(A, B) Return A + B; EndFunction")
	var wait sync.WaitGroup
	for index := 0; index < 100; index++ {
		wait.Add(1)
		go func(value int) {
			defer wait.Done()
			result, err := registry.Call(handle, "Add", bytecode.Number(float64(value)), bytecode.Number(1))
			if err != nil {
				t.Error(err)
				return
			}
			if number, _ := result.AsNumber(); number != float64(value+1) {
				t.Errorf("result = %v", result)
			}
		}(index)
	}
	wait.Wait()
}

func TestRegistryPreservesModuleStatePerHandle(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	source := `Var State;
Procedure Set(Value) State = Value; EndProcedure
Function Get() Return State; EndFunction`
	first := loadSource(t, registry, source)
	second := loadSource(t, registry, source)
	if _, err := registry.Call(first, "Set", bytecode.Number(42)); err != nil {
		t.Fatal(err)
	}
	value, err := registry.Call(first, "Get")
	if err != nil || value.String() != "42" {
		t.Fatalf("first handle state = %v, %v", value, err)
	}
	value, err = registry.Call(second, "Get")
	if err != nil || value.Kind() != bytecode.UndefinedKind {
		t.Fatalf("second handle state = %v, %v", value, err)
	}
}

func TestRegistrySerializesConcurrentStatefulCalls(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	handle := loadSource(t, registry, `Var Counter;
Procedure Set(Value) Counter = Value; EndProcedure
Procedure Increment() Counter = Counter + 1; EndProcedure
Function Get() Return Counter; EndFunction`)
	if _, err := registry.Call(handle, "Set", bytecode.Number(0)); err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := registry.Call(handle, "Increment"); err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	value, err := registry.Call(handle, "Get")
	if err != nil || value.String() != "100" {
		t.Fatalf("state after concurrent calls = %v, %v", value, err)
	}
}

func loadSource(t *testing.T, registry *Registry, source string) uint32 {
	t.Helper()
	program, diagnostics := compiler.CompileSource("test.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	encoded, err := bytecode.MarshalBinary(program)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := registry.Load(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}
