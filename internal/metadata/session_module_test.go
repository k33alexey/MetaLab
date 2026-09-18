package metadata

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func sessionModuleRuntime(t *testing.T, source string) *Runtime {
	t.Helper()
	number := func(name string) SessionParameter {
		return SessionParameter{Format: CurrentFormat, ID: uuid.MustNew(), Name: name, Title: LocalizedText{"ru": name},
			Types: []Type{{Kind: NumberType, Precision: 9}}}
	}
	text := func(name string) SessionParameter {
		return SessionParameter{Format: CurrentFormat, ID: uuid.MustNew(), Name: name, Title: LocalizedText{"ru": name},
			Types: []Type{{Kind: StringType, Length: 50}}}
	}
	catalog, err := NewCatalogSnapshotWithSessionParameters(metadataManifest(), nil, nil, nil, nil, nil, nil, nil, nil, nil,
		[]SessionParameter{number("ЧислоВызовов"), text("ДоступныеСклады"), text("ДоступныеОрганизации"), text("Незаданный")})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{catalog: catalog, sessionParameters: make(map[string]bytecode.Value)}
	if source == "" {
		return runtime
	}
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{{
		Name: SessionModuleName, Filename: "session-module.bsl", Source: source, DefaultContext: syntax.ContextServer,
	}})
	if program == nil {
		t.Fatalf("session module did not compile: %v", diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewSessionBSLEvents(machine.NewContextWithMetadata(runtime))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetSessionModuleHandler(handler)
	return runtime
}

// Reading a parameter the solution has not set yet is what calls the handler:
// without it a declarative restriction on "the warehouses of this user" could
// never be given a value, which is the whole reason the session module exists.
func TestSessionModuleSuppliesParameterOnFirstRead(t *testing.T) {
	t.Parallel()
	runtime := sessionModuleRuntime(t, `
Перем Вызовы;

Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	Если Вызовы = Неопределено Тогда
		Вызовы = 0;
	КонецЕсли;
	Вызовы = Вызовы + 1;
	ПараметрыСеанса.ЧислоВызовов = Вызовы;
	ПараметрыСеанса.ДоступныеСклады = "Основной";
КонецПроцедуры
`)
	ctx := context.Background()
	value, err := runtime.GetSessionParameter(ctx, "ДоступныеСклады")
	if err != nil || value.String() != "Основной" {
		t.Fatalf("value = %v, error = %v", value, err)
	}
	calls, err := runtime.GetSessionParameter(ctx, "ЧислоВызовов")
	if err != nil {
		t.Fatal(err)
	}
	if number, _ := calls.AsNumber(); number != 1 {
		t.Fatalf("handler ran %v times, want once", calls)
	}
}

// A real handler initializes a whole group from one read of the data and says
// so; the platform must therefore re-check every requested name after ONE call
// instead of calling the handler again for the parameter next to it.
func TestSessionModuleInitializesAGroupInOneCall(t *testing.T) {
	t.Parallel()
	runtime := sessionModuleRuntime(t, `
Перем Вызовы;

Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	Если Вызовы = Неопределено Тогда
		Вызовы = 0;
	КонецЕсли;
	Вызовы = Вызовы + 1;
	ПараметрыСеанса.ЧислоВызовов = Вызовы;
	ПараметрыСеанса.ДоступныеСклады = "Основной";
	ПараметрыСеанса.ДоступныеОрганизации = "Головная";
КонецПроцедуры
`)
	ctx := context.Background()
	if _, err := runtime.GetSessionParameter(ctx, "ДоступныеСклады"); err != nil {
		t.Fatal(err)
	}
	second, err := runtime.GetSessionParameter(ctx, "ДоступныеОрганизации")
	if err != nil || second.String() != "Головная" {
		t.Fatalf("second parameter = %v, error = %v", second, err)
	}
	calls, _ := runtime.GetSessionParameter(ctx, "ЧислоВызовов")
	if number, _ := calls.AsNumber(); number != 1 {
		t.Fatalf("handler ran %v times, want once for the whole group", calls)
	}
}

// The handler reads session parameters to decide what still needs setting.
// Without a re-entrancy guard that read would call the handler again, and the
// session would hang instead of starting.
func TestSessionModuleReadingAParameterInsideItselfDoesNotRecurse(t *testing.T) {
	t.Parallel()
	runtime := sessionModuleRuntime(t, `
Перем Вызовы;

Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	Если Вызовы = Неопределено Тогда
		Вызовы = 0;
	КонецЕсли;
	Вызовы = Вызовы + 1;
	Если ПараметрыСеанса.Незаданный = Неопределено Тогда
		ПараметрыСеанса.ДоступныеСклады = "Основной";
	Иначе
		ПараметрыСеанса.ДоступныеСклады = "Неожиданно";
	КонецЕсли;
	ПараметрыСеанса.ЧислоВызовов = Вызовы;
КонецПроцедуры
`)
	ctx := context.Background()
	value, err := runtime.GetSessionParameter(ctx, "ДоступныеСклады")
	if err != nil || value.String() != "Основной" {
		t.Fatalf("value = %v, error = %v", value, err)
	}
	calls, _ := runtime.GetSessionParameter(ctx, "ЧислоВызовов")
	if number, _ := calls.AsNumber(); number != 1 {
		t.Fatalf("handler ran %v times, want exactly once", calls)
	}
}

// The names argument is how the handler knows what it was asked for, and the
// session-start form is the same call with nothing named.
func TestSessionModuleReceivesTheRequestedNames(t *testing.T) {
	t.Parallel()
	source := `
Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	Если ИменаПараметровСеанса = Неопределено Тогда
		ПараметрыСеанса.ДоступныеСклады = "начало сеанса";
	Иначе
		ПараметрыСеанса.ДоступныеСклады = "по требованию: " + ИменаПараметровСеанса[0];
	КонецЕсли;
КонецПроцедуры
`
	ctx := context.Background()
	onDemand := sessionModuleRuntime(t, source)
	value, err := onDemand.GetSessionParameter(ctx, "ДоступныеСклады")
	if err != nil || value.String() != "по требованию: ДоступныеСклады" {
		t.Fatalf("on-demand call = %v, error = %v", value, err)
	}
	atStart := sessionModuleRuntime(t, source)
	if err := atStart.InitializeSessionParameters(ctx); err != nil {
		t.Fatal(err)
	}
	if value, _ := atStart.GetSessionParameter(ctx, "ДоступныеСклады"); value.String() != "начало сеанса" {
		t.Fatalf("session-start call = %v", value)
	}
}

// A project with no session module, or one that declares no handler, keeps
// working exactly as before: the declared default answers the read.
func TestSessionParameterWithoutAHandlerKeepsItsDefault(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, runtime := range map[string]*Runtime{
		"no session module": sessionModuleRuntime(t, ""),
		"module without the predefined procedure": sessionModuleRuntime(t, `
Функция ЧтоТоСвоё() Экспорт
	Возврат 1;
КонецФункции
`),
	} {
		value, err := runtime.GetSessionParameter(ctx, "ДоступныеСклады")
		if err != nil || value.Kind() != bytecode.UndefinedKind {
			t.Fatalf("%s: value = %v, error = %v", name, value, err)
		}
	}
}

// A handler that fails must fail the read. Answering with the default would
// hand a restriction the value "everything the default allows" precisely when
// the code that decides who may see what did not run.
func TestSessionModuleFailureIsReportedNotSwallowed(t *testing.T) {
	t.Parallel()
	runtime := sessionModuleRuntime(t, `
Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	ВызватьИсключение "склады недоступны";
КонецПроцедуры
`)
	_, err := runtime.GetSessionParameter(context.Background(), "ДоступныеСклады")
	if err == nil || !strings.Contains(err.Error(), "склады недоступны") {
		t.Fatalf("handler failure = %v", err)
	}
}

// A list is what a real restriction compares against: "the warehouses this user
// may see" is a set, and the В/НЕ В operators exist to consume it.
func TestSessionModuleSuppliesAListOfValues(t *testing.T) {
	t.Parallel()
	runtime := sessionModuleRuntime(t, `
Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	Склады = Новый Массив;
	Склады.Добавить("Основной");
	Склады.Добавить("Розничный");
	ПараметрыСеанса.ДоступныеСклады = Склады;
КонецПроцедуры
`)
	values, ok, err := runtime.SessionParameterValues(context.Background(), "ДоступныеСклады")
	if err != nil || !ok || len(values) != 2 || values[0].Data != "Основной" || values[1].Data != "Розничный" {
		t.Fatalf("list parameter = %+v, ok=%v, error=%v", values, ok, err)
	}
	// The same parameter read from BSL is the same list, not a flattened value.
	value, err := runtime.GetSessionParameter(context.Background(), "ДоступныеСклады")
	if err != nil {
		t.Fatal(err)
	}
	if length, ok := value.ArrayLength(); !ok || length != 2 {
		t.Fatalf("BSL view of the list = %v (length %v, array %v)", value, length, ok)
	}
}

// Every element is checked against the declared type: a list is not a way to
// smuggle in a value the parameter could not hold on its own.
func TestSessionModuleListRejectsAValueOfTheWrongType(t *testing.T) {
	t.Parallel()
	runtime := sessionModuleRuntime(t, `
Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	Склады = Новый Массив;
	Склады.Добавить("Основной");
	Склады.Добавить(42);
	ПараметрыСеанса.ДоступныеСклады = Склады;
КонецПроцедуры
`)
	_, _, err := runtime.SessionParameterValues(context.Background(), "ДоступныеСклады")
	if err == nil {
		t.Fatal("a list element of the wrong type was accepted")
	}
}

// A parameter nobody set and that declares no default stays absent, so the
// restriction naming it refuses the read instead of widening it.
func TestSessionParameterValuesReportsAnUnsetParameterAsAbsent(t *testing.T) {
	t.Parallel()
	runtime := sessionModuleRuntime(t, `
Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)
	ПараметрыСеанса.ДоступныеСклады = "Основной";
КонецПроцедуры
`)
	values, ok, err := runtime.SessionParameterValues(context.Background(), "ДоступныеОрганизации")
	if err != nil || ok || values != nil {
		t.Fatalf("unset parameter = %+v, ok=%v, error=%v", values, ok, err)
	}
}
