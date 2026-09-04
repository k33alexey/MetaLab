package compiler

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

func TestCompileSourceProducesDeterministicBytecode(t *testing.T) {
	t.Parallel()

	source := "Функция Сумма(А, Б) Экспорт\nВозврат (А + Б) * 2;\nКонецФункции"
	program, diagnostics := CompileSource("sum.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	function, ok := program.Lookup("Сумма")
	if !ok {
		t.Fatal("compiled function not found")
	}
	want := []bytecode.Opcode{
		bytecode.OpLoadLocal, bytecode.OpLoadLocal, bytecode.OpAdd,
		bytecode.OpConstant, bytecode.OpMultiply, bytecode.OpReturn,
	}
	if len(function.Code) != len(want) {
		t.Fatalf("code length = %d, want %d", len(function.Code), len(want))
	}
	for index, opcode := range want {
		if function.Code[index].Opcode != opcode {
			t.Fatalf("code[%d] = %s, want %s", index, function.Code[index].Opcode, opcode)
		}
	}
	if function.MaxStack != 2 {
		t.Fatalf("max stack = %d, want 2", function.MaxStack)
	}
}

func TestCompileReportsUnknownIdentifier(t *testing.T) {
	t.Parallel()

	_, diagnostics := CompileSource("broken.bsl", "Function Test()\nReturn Missing;\nEndFunction")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3007" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestCompileRejectsDuplicateRoutineIgnoringCase(t *testing.T) {
	t.Parallel()

	source := "Function Test() Return 1; EndFunction\nFunction tEsT() Return 2; EndFunction"
	_, diagnostics := CompileSource("duplicate.bsl", source)
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3001" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestCompileProcedureAddsImplicitUndefinedReturn(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileSource("procedure.bsl", "Procedure Run() Export\nEndProcedure")
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	function, ok := program.Lookup("run")
	if !ok {
		t.Fatal("compiled procedure not found")
	}
	if function.IsFunction || !function.Export {
		t.Fatalf("procedure flags = function:%v export:%v", function.IsFunction, function.Export)
	}
	if len(function.Constants) != 1 || function.Constants[0].Kind() != bytecode.UndefinedKind {
		t.Fatalf("constants = %#v", function.Constants)
	}
}

func TestCompileRejectsDuplicateParameterIgnoringCase(t *testing.T) {
	t.Parallel()

	_, diagnostics := CompileSource("duplicate.bsl", "Function Test(Value, vAlUe)\nReturn Value;\nEndFunction")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3004" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestCompileControlFlowUsesLocalsAndValidatedJumps(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileSource("flow.bsl", `Function Flow(Limit)
Total = 0;
For Index = 1 To Limit Do
    If Index = 2 Then Continue; EndIf;
    Total = Total + Index;
EndDo;
Return Total;
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	function, ok := program.Lookup("Flow")
	if !ok {
		t.Fatal("compiled function not found")
	}
	if function.LocalCount != 4 {
		t.Fatalf("local count = %d, want 4", function.LocalCount)
	}
	var jumps, stores int
	for _, instruction := range function.Code {
		switch instruction.Opcode {
		case bytecode.OpJump, bytecode.OpJumpIfFalse:
			jumps++
		case bytecode.OpStoreLocal:
			stores++
		}
	}
	if jumps < 4 || stores < 4 {
		t.Fatalf("jumps = %d, stores = %d", jumps, stores)
	}
	if err := program.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCompileRejectsLoopControlOutsideLoop(t *testing.T) {
	t.Parallel()

	for _, statement := range []string{"Break;", "Continue;"} {
		_, diagnostics := CompileSource("invalid.bsl", "Procedure Test()\n"+statement+"\nEndProcedure")
		if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3012" {
			t.Fatalf("statement %q diagnostics = %v", statement, diagnostics)
		}
	}
}

func TestCompileKeepsUnreachableStatementsStructurallyValid(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileSource("unreachable.bsl", "Function Test()\nReturn 1;\nReturn 2 + 3;\nEndFunction")
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if err := program.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCompileForEachUsesInternalIteratorLocals(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileSource("foreach.bsl", "Procedure Test(Items)\nFor Each Item In Items Do Break; EndDo;\nEndProcedure")
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	function, ok := program.Lookup("Test")
	if !ok || function.LocalCount != 4 {
		t.Fatalf("function = %+v", function)
	}
}

func FuzzCompileControlFlowNeverProducesInvalidBytecode(f *testing.F) {
	for _, source := range []string{
		"Function Test() Return 1; EndFunction",
		"Function Test(Value) If Value > 0 Then Return Value; Else Return 0; EndIf; EndFunction",
		"Procedure Test() For Index = 1 To 3 Do Continue; EndDo; EndProcedure",
		"Procedure Test(Items) For Each Item In Items Do Break; EndDo; EndProcedure",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		program, diagnostics := CompileSource("fuzz.bsl", source)
		if len(diagnostics) != 0 {
			return
		}
		if program == nil {
			t.Fatal("successful compilation returned a nil program")
		}
		if err := program.Validate(); err != nil {
			t.Fatalf("compiler produced invalid bytecode: %v", err)
		}
	})
}
