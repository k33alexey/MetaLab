package vm

import (
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

func TestRussianAndEnglishProgramsExecute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		source   string
		function string
	}{
		{
			name: "russian", function: "Расчёт",
			source: "Функция Расчёт(А, Б) Экспорт\nВозврат (А + Б) * 2;\nКонецФункции",
		},
		{
			name: "english", function: "Calculate",
			source: "Function Calculate(A, B) Export\nReturn (A + B) * 2;\nEndFunction",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			machine := compileMachine(t, test.source)
			result, err := machine.Call(test.function, bytecode.Number(20), bytecode.Number(22))
			if err != nil {
				t.Fatal(err)
			}
			if number, ok := result.AsNumber(); !ok || number != 84 {
				t.Fatalf("result = %v, want 84", result)
			}
		})
	}
}

func TestMachineReportsSourcePositionForDivisionByZero(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, "Function Divide(A, B)\nReturn A / B;\nEndFunction")
	_, err := machine.Call("Divide", bytecode.Number(1), bytecode.Number(0))
	if err == nil {
		t.Fatal("expected division error")
	}
	if got := err.Error(); got != "Divide:2:8: division by zero" {
		t.Fatalf("error = %q", got)
	}
}

func TestMachineRejectsWrongArity(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, "Function Identity(Value)\nReturn Value;\nEndFunction")
	if _, err := machine.Call("Identity"); err == nil {
		t.Fatal("Call() accepted wrong arity")
	}
}

func TestMachineRejectsInvalidConstructionAndUnknownRoutine(t *testing.T) {
	t.Parallel()

	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) succeeded")
	}
	if _, err := New(&bytecode.Program{}); err == nil {
		t.Fatal("New() accepted invalid bytecode")
	}
	machine := compileMachine(t, "Function Known()\nReturn 1;\nEndFunction")
	if _, err := machine.Call("Missing"); err == nil {
		t.Fatal("Call() accepted an unknown routine")
	}
}

func TestMachineOwnsImmutableProgramCopy(t *testing.T) {
	t.Parallel()

	program, diagnostics := compiler.CompileSource("test.bsl", "Function Value()\nReturn 42;\nEndFunction")
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := New(program)
	if err != nil {
		t.Fatal(err)
	}
	program.Functions[0].Name = "Changed"
	program.Functions[0].Constants[0] = bytecode.Number(0)
	result, err := machine.Call("Value")
	if err != nil {
		t.Fatal(err)
	}
	if number, _ := result.AsNumber(); number != 42 {
		t.Fatalf("result = %v, want 42", result)
	}
}

func TestMachineConcatenatesStrings(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, "Function Join(A, B)\nReturn A + B;\nEndFunction")
	result, err := machine.Call("Join", bytecode.String("Meta"), bytecode.String("Lab"))
	if err != nil {
		t.Fatal(err)
	}
	if text, ok := result.AsString(); !ok || text != "MetaLab" {
		t.Fatalf("result = %v", result)
	}
}

func TestMachineExecutesNumericOperators(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, "Function Calculate(A, B)\nReturn -(A - B) / 2;\nEndFunction")
	result, err := machine.Call("Calculate", bytecode.Number(10), bytecode.Number(4))
	if err != nil {
		t.Fatal(err)
	}
	if number, ok := result.AsNumber(); !ok || number != -3 {
		t.Fatalf("result = %v, want -3", result)
	}
}

func TestMachineRejectsInvalidOperandTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		args   []bytecode.Value
	}{
		{name: "unary", source: "Function Test(A) Return -A; EndFunction", args: []bytecode.Value{bytecode.String("x")}},
		{name: "binary", source: "Function Test(A, B) Return A * B; EndFunction", args: []bytecode.Value{bytecode.String("x"), bytecode.Number(2)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			machine := compileMachine(t, test.source)
			if _, err := machine.Call("Test", test.args...); err == nil {
				t.Fatal("Call() accepted invalid operand types")
			}
		})
	}
}

func TestMachineIsSafeForConcurrentCalls(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, "Function Add(A, B)\nReturn A + B;\nEndFunction")
	var wait sync.WaitGroup
	errors := make(chan error, 100)
	for index := 0; index < 100; index++ {
		wait.Add(1)
		go func(value int) {
			defer wait.Done()
			result, err := machine.Call("Add", bytecode.Number(float64(value)), bytecode.Number(1))
			if err != nil {
				errors <- err
				return
			}
			if number, _ := result.AsNumber(); number != float64(value+1) {
				errors <- &unexpectedResult{got: number, want: float64(value + 1)}
			}
		}(index)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

type unexpectedResult struct{ got, want float64 }

func (err *unexpectedResult) Error() string { return "unexpected concurrent result" }

func compileMachine(t testing.TB, source string) *Machine {
	t.Helper()
	program, diagnostics := compiler.CompileSource("test.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatalf("compile diagnostics = %v", diagnostics)
	}
	machine, err := New(program)
	if err != nil {
		t.Fatal(err)
	}
	return machine
}

func BenchmarkCompile(b *testing.B) {
	source := "Function Calculate(A, B) Export\nReturn (A + B) * 2;\nEndFunction"
	b.ReportAllocs()
	for b.Loop() {
		program, diagnostics := compiler.CompileSource("bench.bsl", source)
		if len(diagnostics) != 0 || program == nil {
			b.Fatal(diagnostics)
		}
	}
}

func BenchmarkMachineCall(b *testing.B) {
	machine := compileMachine(b, "Function Add(A, B)\nReturn A + B;\nEndFunction")
	left, right := bytecode.Number(20), bytecode.Number(22)
	b.ReportAllocs()
	for b.Loop() {
		result, err := machine.Call("Add", left, right)
		if err != nil {
			b.Fatal(err)
		}
		if number, _ := result.AsNumber(); number != 42 {
			b.Fatal(number)
		}
	}
}
