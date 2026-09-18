package vm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

// panickingRuntime is a host object that crashes instead of returning an error -
// the defect class this barrier exists for. The engine itself does not panic;
// what reaches it comes from the metadata layer underneath.
type panickingRuntime struct{ calls int }

func (runtime *panickingRuntime) GetConstant(context.Context, string) (bytecode.Value, error) {
	runtime.calls++
	panic("host object is broken")
}
func (runtime *panickingRuntime) SetConstant(context.Context, string, bytecode.Value) error {
	return nil
}
func (runtime *panickingRuntime) GetSessionParameter(context.Context, string) (bytecode.Value, error) {
	runtime.calls++
	panic("host object is broken")
}
func (runtime *panickingRuntime) SetSessionParameter(context.Context, string, bytecode.Value) error {
	return nil
}
func (runtime *panickingRuntime) GetEnumerationValue(context.Context, string, string) (bytecode.Value, error) {
	return bytecode.Undefined(), nil
}
func (runtime *panickingRuntime) GetDefinedType(context.Context, string) (bytecode.Value, error) {
	return bytecode.Undefined(), nil
}

func panickingMachine(t *testing.T) (*Machine, *panickingRuntime) {
	t.Helper()
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: "Тест", Filename: "test.bsl", DefaultContext: syntax.ContextServer,
		Source: "Функция Прочитать() Экспорт\n\tВозврат ПараметрыСеанса.Склад;\nКонецФункции\n",
	}})
	if program == nil {
		t.Fatalf("compile: %v", diagnostics)
	}
	machine, err := New(program)
	if err != nil {
		t.Fatal(err)
	}
	return machine, &panickingRuntime{}
}

// A crash in application code must come back as an error. Letting it unwind
// would end the process - and with it every session of every database, not just
// the request that triggered it.
func TestPanicInHostObjectBecomesAnErrorNotACrash(t *testing.T) {
	t.Parallel()
	machine, runtime := panickingMachine(t)
	machineContext := machine.NewContextWithMetadata(runtime)
	value, err := machineContext.CallContext(context.Background(), "Тест.Прочитать")
	var failure *PanicError
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want a PanicError", err)
	}
	if failure.Routine != "Тест.Прочитать" || !strings.Contains(failure.Error(), "host object is broken") {
		t.Fatalf("panic error = %+v", failure)
	}
	if !strings.Contains(failure.Stack, "panic") {
		t.Fatalf("panic error carries no stack: %q", failure.Stack)
	}
	if value.Kind() != bytecode.UndefinedKind {
		t.Fatalf("value after a crash = %v, want Неопределено", value)
	}
	// The machine stays usable: one broken call is not a broken process.
	if _, err := machineContext.CallContext(context.Background(), "Тест.Прочитать"); !errors.As(err, &failure) {
		t.Fatalf("second call error = %v", err)
	}
	if runtime.calls != 2 {
		t.Fatalf("host calls = %d, want both attempts to have run", runtime.calls)
	}
}

func TestPanicIsCaughtOnEveryExecutionEntryPoint(t *testing.T) {
	t.Parallel()
	machine, runtime := panickingMachine(t)
	var failure *PanicError
	if _, err := machine.CallContext(context.Background(), "Тест.Прочитать"); err == nil {
		t.Fatal("Machine.CallContext without metadata should fail, not succeed")
	}
	machineContext := machine.NewContextWithMetadata(runtime)
	if _, _, err := machineContext.CallContextMutable(context.Background(), "Тест.Прочитать"); !errors.As(err, &failure) {
		t.Fatalf("CallContextMutable error = %v, want a PanicError", err)
	}
	if _, err := machineContext.CallExported("Тест", "Прочитать"); !errors.As(err, &failure) {
		t.Fatalf("CallExported error = %v, want a PanicError", err)
	}
}

// Nothing may swallow a crash quietly: an incident that leaves no trace is
// indistinguishable from one that was ignored, and the platform would keep
// answering requests while something inside it is broken.
func TestPanicIsRecordedAsAnIncident(t *testing.T) {
	machine, runtime := panickingMachine(t)
	var recorded bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&recorded, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	if _, err := machine.NewContextWithMetadata(runtime).CallContext(context.Background(), "Тест.Прочитать"); err == nil {
		t.Fatal("a crashing call reported success")
	}
	logged := recorded.String()
	for _, fragment := range []string{"BSL execution panicked", "Тест.Прочитать", "host object is broken", "stack"} {
		if !strings.Contains(logged, fragment) {
			t.Fatalf("incident record is missing %q: %s", fragment, logged)
		}
	}
}
