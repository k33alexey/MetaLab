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
