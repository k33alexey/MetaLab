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

func TestCompileExecutionContextsAndRPCRoutes(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileSource("contexts.bsl", `&AtServer
Function ServerValue() Return 1; EndFunction
&AtServerNoContext
Function IsolatedValue() Return 2; EndFunction
&AtClient
Function ClientValue() Return ServerValue() + IsolatedValue(); EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	server, _ := program.Lookup("ServerValue")
	isolated, _ := program.Lookup("IsolatedValue")
	client, _ := program.Lookup("ClientValue")
	if server.Context != bytecode.ContextServer || isolated.Context != bytecode.ContextServerNoContext || client.Context != bytecode.ContextClient {
		t.Fatalf("contexts = %d, %d, %d", server.Context, isolated.Context, client.Context)
	}
	if len(client.CallSites) != 2 || client.CallSites[0].Route != bytecode.CallServer || client.CallSites[1].Route != bytecode.CallServerNoContext {
		t.Fatalf("call sites = %+v", client.CallSites)
	}
}

func TestCompileRejectsServerToClientCall(t *testing.T) {
	t.Parallel()

	callers := []string{"&AtServer", "&AtServerNoContext", "&AtClientAtServer", "&AtClientAtServerNoContext", ""}
	for _, caller := range callers {
		_, diagnostics := CompileSource("contexts.bsl", `&AtClient
Procedure Notify() EndProcedure
`+caller+`
Procedure Run() Notify(); EndProcedure`)
		if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3039" {
			t.Fatalf("caller %q diagnostics = %v", caller, diagnostics)
		}
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

func TestCompileLinksCallsParametersAndModuleVariables(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileModules([]ModuleSource{
		{Name: "Состояние", Filename: "state.bsl", Source: `Перем Счётчик Экспорт;
Процедура Увеличить(Знач Шаг = 1) Экспорт Счётчик = Счётчик + Шаг; КонецПроцедуры
Функция Получить() Экспорт Возврат Счётчик; КонецФункции`},
		{Name: "Приложение", Filename: "app.bsl", Source: `Функция Запустить() Экспорт
Состояние.Счётчик = 0;
Состояние.Увеличить();
Возврат Состояние.Получить();
КонецФункции`},
	})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if len(program.Modules) != 2 || len(program.Functions) != 3 {
		t.Fatalf("program shape = %d modules, %d routines", len(program.Modules), len(program.Functions))
	}
	run, ok := program.Lookup("Приложение.Запустить")
	if !ok || len(run.CallSites) != 2 || len(run.ModuleVars) != 1 {
		t.Fatalf("linked function = %+v", run)
	}
	increment, ok := program.Lookup("Состояние.Увеличить")
	if !ok || len(increment.Parameters) != 1 || !increment.Parameters[0].ByValue || !increment.Parameters[0].HasDefault {
		t.Fatalf("parameter metadata = %+v", increment)
	}
	if err := program.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCompileRejectsInvalidRoutineContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		code   string
	}{
		{name: "required after optional", source: "Function Test(A = 1, B) Return B; EndFunction", code: "BSL3016"},
		{name: "nonconstant default", source: "Function Test(A = Missing) Return A; EndFunction", code: "BSL3017"},
		{name: "procedure as value", source: "Procedure Run() EndProcedure Function Test() Return Run(); EndFunction", code: "BSL3029"},
		{name: "procedure return value", source: "Procedure Test() Return 1; EndProcedure", code: "BSL3025"},
		{name: "missing argument", source: "Function Add(A) Return A; EndFunction Function Test() Return Add(); EndFunction", code: "BSL3031"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, diagnostics := CompileSource("invalid.bsl", test.source)
			if len(diagnostics) == 0 || diagnostics[0].Code != test.code {
				t.Fatalf("diagnostics = %v, want %s", diagnostics, test.code)
			}
		})
	}
}

func TestCompileEnforcesModuleExports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		callee string
		caller string
		code   string
	}{
		{name: "routine", callee: "Function Hidden() Return 1; EndFunction", caller: "Function Test() Return Private.Hidden(); EndFunction", code: "BSL3034"},
		{name: "variable", callee: "Var Hidden;", caller: "Function Test() Return Private.Hidden; EndFunction", code: "BSL3027"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, diagnostics := CompileModules([]ModuleSource{
				{Name: "Private", Filename: "private.bsl", Source: test.callee},
				{Name: "Caller", Filename: "caller.bsl", Source: test.caller},
			})
			if len(diagnostics) != 1 || diagnostics[0].Code != test.code || diagnostics[0].Filename != "caller.bsl" {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestCompileKeepsSameRoutineNameInSeparateModuleNamespaces(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileModules([]ModuleSource{
		{Name: "First", Filename: "first.bsl", Source: "Function Value() Export Return 1; EndFunction"},
		{Name: "Second", Filename: "second.bsl", Source: "Function Value() Export Return 2; EndFunction"},
	})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if _, ok := program.Lookup("Value"); ok {
		t.Fatal("ambiguous unqualified lookup unexpectedly succeeded")
	}
	if first, ok := program.Lookup("first.value"); !ok || first.Module != 0 {
		t.Fatalf("case-insensitive qualified lookup = %+v, %v", first, ok)
	}
}

func TestCompileUsesRoutineWideLocalScopeAndExplicitShadowing(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileSource("scope.bsl", `Var Value;
Function BeforeAssignment() Return Local; Local = 1; EndFunction
Function Shadow() Var Value; Return Value; EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	before, _ := program.Lookup("BeforeAssignment")
	shadow, _ := program.Lookup("Shadow")
	if before.LocalCount != 1 || len(before.ModuleVars) != 0 {
		t.Fatalf("implicit local scope = %+v", before)
	}
	if shadow.LocalCount != 1 || len(shadow.ModuleVars) != 0 {
		t.Fatalf("explicit local shadow = %+v", shadow)
	}
}

func TestCompileDeclaresExplicitLocalFromNestedBlockForWholeRoutine(t *testing.T) {
	program, diagnostics := CompileSource("scope.bsl", `Function Test()
If True Then
    Var Nested;
EndIf;
Return Nested;
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	function, ok := program.Lookup("Test")
	if !ok {
		t.Fatal("compiled function not found")
	}
	if function.LocalCount != 1 {
		t.Fatalf("local count = %d, want 1", function.LocalCount)
	}
}

func TestCompileExceptionHandlersAndSourceIdentity(t *testing.T) {
	t.Parallel()

	program, diagnostics := CompileSource("errors/main.bsl", `Function Recover() Export
Try
    Raise "boom";
Except
    Result = 42;
EndTry;
Return Result;
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if len(program.Modules) != 1 || program.Modules[0].Source != "errors/main.bsl" {
		t.Fatalf("module source = %+v", program.Modules)
	}
	function, ok := program.Lookup("Recover")
	if !ok || len(function.Exceptions) != 1 {
		t.Fatalf("compiled function = %+v", function)
	}
	handler := function.Exceptions[0]
	if handler.Start >= handler.End || handler.Target < handler.End {
		t.Fatalf("exception handler = %+v", handler)
	}
	if err := program.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCompileRejectsReraiseOutsideExcept(t *testing.T) {
	t.Parallel()

	_, diagnostics := CompileSource("invalid.bsl", "Procedure Test() Raise; EndProcedure")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3035" {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestCompileValidatesErrorDescriptionContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		code   string
	}{
		{source: "Function Test() Return ErrorDescription(); EndFunction", code: "BSL3038"},
		{source: "Function Test() Try Raise \"x\"; Except Return ErrorDescription(1); EndTry; EndFunction", code: "BSL3037"},
	}
	for _, test := range tests {
		_, diagnostics := CompileSource("invalid.bsl", test.source)
		if len(diagnostics) == 0 || diagnostics[0].Code != test.code {
			t.Fatalf("diagnostics = %+v, want %s", diagnostics, test.code)
		}
	}
}

func FuzzCompileControlFlowNeverProducesInvalidBytecode(f *testing.F) {
	for _, source := range []string{
		"Function Test() Return 1; EndFunction",
		"Function Test(Value) If Value > 0 Then Return Value; Else Return 0; EndIf; EndFunction",
		"Procedure Test() For Index = 1 To 3 Do Continue; EndDo; EndProcedure",
		"Procedure Test(Items) For Each Item In Items Do Break; EndDo; EndProcedure",
		"Function Test() Try Raise \"boom\"; Except Return 42; EndTry; EndFunction",
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
