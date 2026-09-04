package vm

import (
	"sync"
	"testing"
	"time"

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

func TestMachineExecutesConditionsAndLoops(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		"russian": `Функция Поток(Предел)
Итог = 0;
Для Индекс = 1 По Предел Цикл
    Если Индекс = 2 Тогда
        Продолжить;
    КонецЕсли;
    Итог = Итог + Индекс;
    Если Итог > 10 Тогда
        Прервать;
    КонецЕсли;
КонецЦикла;
Пока Итог < 12 Цикл
    Итог = Итог + 1;
КонецЦикла;
Возврат Итог;
КонецФункции`,
		"english": `Function Flow(Limit)
Total = 0;
For Index = 1 To Limit Do
    If Index = 2 Then
        Continue;
    EndIf;
    Total = Total + Index;
    If Total > 10 Then
        Break;
    EndIf;
EndDo;
While Total < 12 Do
    Total = Total + 1;
EndDo;
Return Total;
EndFunction`,
	}
	for language, source := range sources {
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			machine := compileMachine(t, source)
			function := "Поток"
			if language == "english" {
				function = "Flow"
			}
			for _, test := range []struct {
				limit float64
				want  float64
			}{{limit: 3, want: 12}, {limit: 5, want: 13}} {
				result, err := machine.Call(function, bytecode.Number(test.limit))
				if err != nil {
					t.Fatal(err)
				}
				if number, ok := result.AsNumber(); !ok || number != test.want {
					t.Fatalf("limit %v result = %v, want %v", test.limit, result, test.want)
				}
			}
		})
	}
}

func TestMachineExecutesElsIfAndBooleanOperators(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function Classify(Value)
If Value < 0 Then
    Return -1;
ElsIf Value = 0 Or Not (Value <> 0) Then
    Return 0;
Else
    Return 1;
EndIf;
EndFunction`)
	for _, test := range []struct {
		value float64
		want  float64
	}{{value: -5, want: -1}, {value: 0, want: 0}, {value: 8, want: 1}} {
		result, err := machine.Call("Classify", bytecode.Number(test.value))
		if err != nil {
			t.Fatal(err)
		}
		if number, ok := result.AsNumber(); !ok || number != test.want {
			t.Fatalf("Classify(%v) = %v, want %v", test.value, result, test.want)
		}
	}
}

func TestMachineExecutesForEachWithBreakAndContinue(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function Sum(Items)
Total = 0;
For Each Item In Items Do
    If Item < 0 Then Continue; EndIf;
    Total = Total + Item;
    If Total >= 6 Then Break; EndIf;
EndDo;
Return Total;
EndFunction`)
	result, err := machine.Call("Sum", bytecode.Array(
		bytecode.Number(1), bytecode.Number(-5), bytecode.Number(2), bytecode.Number(3), bytecode.Number(100),
	))
	if err != nil {
		t.Fatal(err)
	}
	if number, ok := result.AsNumber(); !ok || number != 6 {
		t.Fatalf("result = %v, want 6", result)
	}
}

func TestMachineUsesNearestNestedLoopForBreak(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function Nested()
Total = 0;
For I = 1 To 3 Do
    For J = 1 To 3 Do
        If J = 2 Then Break; EndIf;
        Total = Total + 1;
    EndDo;
EndDo;
Return Total;
EndFunction`)
	result, err := machine.Call("Nested")
	if err != nil {
		t.Fatal(err)
	}
	if number, ok := result.AsNumber(); !ok || number != 3 {
		t.Fatalf("result = %v, want 3", result)
	}
}

func TestMachineRejectsNonArrayForEachValue(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, "Function Test(Items)\nFor Each Item In Items Do Break; EndDo;\nReturn 0;\nEndFunction")
	if _, err := machine.Call("Test", bytecode.Number(1)); err == nil || err.Error() != "Test:2:1: For Each requires an array" {
		t.Fatalf("error = %v", err)
	}
}

func TestMachineRejectsNonBooleanCondition(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, "Function Test()\nIf 1 Then Return 1; EndIf;\nReturn 0;\nEndFunction")
	if _, err := machine.Call("Test"); err == nil || err.Error() != "Test:2:1: condition requires a boolean" {
		t.Fatalf("error = %v", err)
	}
}

func TestMachineExecutesPrimitiveLiteralsAndDateArithmetic(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function UndefinedValue() Return Undefined; EndFunction
Function NullValue() Return Null; EndFunction
Function DateValue() Return '20240229010203' + 60; EndFunction
Function DateDifference() Return '20240229010203' - '20240229010103'; EndFunction
Function DateSubtract() Return '20240229010203' - 3.5; EndFunction`)
	undefined, err := machine.Call("UndefinedValue")
	if err != nil || undefined.Kind() != bytecode.UndefinedKind {
		t.Fatalf("UndefinedValue() = %v, %v", undefined, err)
	}
	null, err := machine.Call("NullValue")
	if err != nil || null.Kind() != bytecode.NullKind {
		t.Fatalf("NullValue() = %v, %v", null, err)
	}
	shifted, err := machine.Call("DateValue")
	if err != nil {
		t.Fatal(err)
	}
	wantDate := time.Date(2024, 2, 29, 1, 3, 3, 0, time.UTC)
	if value, ok := shifted.AsDate(); !ok || !value.Equal(wantDate) {
		t.Fatalf("DateValue() = %v, want %v", shifted, wantDate)
	}
	difference, err := machine.Call("DateDifference")
	if err != nil {
		t.Fatal(err)
	}
	if number, ok := difference.AsNumber(); !ok || number != 60 {
		t.Fatalf("DateDifference() = %v, want 60", difference)
	}
	subtracted, err := machine.Call("DateSubtract")
	if err != nil {
		t.Fatal(err)
	}
	wantDate = time.Date(2024, 2, 29, 1, 1, 59, 500_000_000, time.UTC)
	if value, ok := subtracted.AsDate(); !ok || !value.Equal(wantDate) {
		t.Fatalf("DateSubtract() = %v, want %v", subtracted, wantDate)
	}
}

func TestMachineKeepsDecimalArithmeticExact(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function ExactSum() Return 0.1 + 0.2; EndFunction
Function Maximum() Return 99999999999999999999999999999999999999; EndFunction`)
	sum, err := machine.Call("ExactSum")
	if err != nil {
		t.Fatal(err)
	}
	if text, ok := sum.NumberText(); !ok || text != "0.3" {
		t.Fatalf("ExactSum() = %v, want 0.3", sum)
	}
	maximum, err := machine.Call("Maximum")
	if err != nil {
		t.Fatal(err)
	}
	if text, ok := maximum.NumberText(); !ok || text != "99999999999999999999999999999999999999" {
		t.Fatalf("Maximum() = %v", maximum)
	}
}

func TestMachineComparesPrimitiveTypesAndConcatenatesString(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function Ordered()
Return Null < Undefined And Undefined < False And False < 0 And 0 < '20240101' And '20240101' < "A";
EndFunction
Function DistinctEmptyValues() Return Null <> Undefined; EndFunction
Function CaseSensitive() Return "A" <> "a"; EndFunction
Function Presentation() Return "Value=" + 42; EndFunction`)
	for _, function := range []string{"Ordered", "DistinctEmptyValues", "CaseSensitive"} {
		result, err := machine.Call(function)
		if err != nil {
			t.Fatal(err)
		}
		if value, ok := result.AsBoolean(); !ok || !value {
			t.Fatalf("%s() = %v, want True", function, result)
		}
	}
	result, err := machine.Call("Presentation")
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := result.AsString(); !ok || value != "Value=42" {
		t.Fatalf("Presentation() = %v", result)
	}
}

func TestMachineShortCircuitsLogicalExpressions(t *testing.T) {
	t.Parallel()

	machine := compileMachine(t, `Function AndValue() Return False And (1 / 0 = 0); EndFunction
Function OrValue() Return True Or (1 / 0 = 0); EndFunction`)
	andValue, err := machine.Call("AndValue")
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := andValue.AsBoolean(); !ok || value {
		t.Fatalf("AndValue() = %v", andValue)
	}
	orValue, err := machine.Call("OrValue")
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := orValue.AsBoolean(); !ok || !value {
		t.Fatalf("OrValue() = %v", orValue)
	}
}

func TestMachineValidatesEvaluatedLogicalOperand(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		"Function Test() Return True And 1; EndFunction",
		"Function Test() Return False Or 1; EndFunction",
	} {
		machine := compileMachine(t, source)
		if _, err := machine.Call("Test"); err == nil {
			t.Fatalf("Call() accepted non-boolean operand in %q", source)
		}
	}
}

func TestCompileRejectsInvalidCalendarDate(t *testing.T) {
	t.Parallel()

	for _, literal := range []string{"''", "'20230229'", "'20240101240000'", "'40000101'"} {
		_, diagnostics := compiler.CompileSource("date.bsl", "Function Test() Return "+literal+"; EndFunction")
		if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3014" {
			t.Fatalf("literal %s diagnostics = %v", literal, diagnostics)
		}
	}
}

func TestCompileRejectsNumberOutsidePrecision(t *testing.T) {
	t.Parallel()

	_, diagnostics := compiler.CompileSource("number.bsl", "Function Test() Return 123456789012345678901234567890123456789; EndFunction")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3006" {
		t.Fatalf("diagnostics = %v", diagnostics)
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

func TestMachineCommonControlFlowDoesNotAllocatePerCall(t *testing.T) {
	machine := compileMachine(t, `Function Sum(Limit)
Total = 0;
For Index = 1 To Limit Do Total = Total + Index; EndDo;
Return Total;
EndFunction`)
	limit := bytecode.Number(10)
	allocations := testing.AllocsPerRun(1000, func() {
		result, err := machine.Call("Sum", limit)
		if err != nil {
			panic(err)
		}
		if number, _ := result.AsNumber(); number != 55 {
			panic("unexpected result")
		}
	})
	if allocations != 0 {
		t.Fatalf("allocations per call = %v, want 0", allocations)
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

func BenchmarkMachineControlFlow(b *testing.B) {
	machine := compileMachine(b, `Function Sum(Limit)
Total = 0;
For Index = 1 To Limit Do
    If Index % 2 = 0 Then Continue; EndIf;
    Total = Total + Index;
EndDo;
Return Total;
EndFunction`)
	limit := bytecode.Number(100)
	b.ReportAllocs()
	for b.Loop() {
		result, err := machine.Call("Sum", limit)
		if err != nil {
			b.Fatal(err)
		}
		if number, _ := result.AsNumber(); number != 2500 {
			b.Fatal(number)
		}
	}
}
