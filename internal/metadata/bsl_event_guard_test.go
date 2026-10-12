package metadata

import (
	"context"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/kinds"
)

func TestBSLEventGuardRejectsCyclesAndAllowsNestedHandlers(t *testing.T) {
	t.Parallel()
	first, second := new(int), new(int)
	ctx, err := enterBSLEvent(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = enterBSLEvent(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enterBSLEvent(ctx, first); err == nil {
		t.Fatal("recursive BSL event chain was accepted")
	}
}

// TestModuleWordsHoldTheirValueForOneCall catches a module that keeps the
// object of an event after the event - a later call into the module would
// read ЭтотОбъект of a record already written - and a failure half way that
// leaves the first language bound: a module compiled with ЭтотОбъект and no
// ThisObject cannot take the second, and must not keep the first.
func TestModuleWordsHoldTheirValueForOneCall(t *testing.T) {
	t.Parallel()
	const source = "Функция Что() Экспорт\n Возврат ЭтотОбъект;\nКонецФункции\n"
	read := func(t *testing.T, context *vm.Context, module string) bytecode.Value {
		t.Helper()
		value, err := context.Call(module + ".Что")
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	machineOf := func(t *testing.T, compile func() (*bytecode.Program, []syntax.Diagnostic)) *vm.Context {
		t.Helper()
		program, diagnostics := compile()
		if len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		machine, err := vm.New(program)
		if err != nil {
			t.Fatal(err)
		}
		return machine.NewContext()
	}
	t.Run("one call", func(t *testing.T) {
		t.Parallel()
		module := ObjectModuleName(CatalogKind, "Банки", string(kinds.ObjectModule))
		context := machineOf(t, func() (*bytecode.Program, []syntax.Diagnostic) {
			return compileObjectModule(CatalogKind, "Банки", kinds.ObjectModule, "Банки.bsl", source)
		})
		unbind, err := bindModuleWords(context, module, boundWord{kinds.ThisObject, bytecode.String("запись")})
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := read(t, context, module).AsString(); got != "запись" {
			t.Fatalf("during the call ЭтотОбъект is %q", got)
		}
		unbind()
		if got := read(t, context, module); got.Kind() != bytecode.UndefinedKind {
			t.Fatalf("after the call the module still holds %v", got)
		}
	})
	t.Run("a failure half way", func(t *testing.T) {
		t.Parallel()
		context := machineOf(t, func() (*bytecode.Program, []syntax.Diagnostic) {
			return compiler.CompileModules([]compiler.ModuleSource{{Name: "М", Filename: "М.bsl", Source: source,
				PredefinedVariables: []string{kinds.ThisObject.Russian}}})
		})
		if _, err := bindModuleWords(context, "М", boundWord{kinds.ThisObject, bytecode.String("запись")}); err == nil {
			t.Fatal("a module with no ThisObject took it")
		}
		if got := read(t, context, "М"); got.Kind() != bytecode.UndefinedKind {
			t.Fatalf("a failed binding left ЭтотОбъект holding %v", got)
		}
	})
}
