package testsuite

import (
	"context"
	"errors"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

type runtimeStub struct {
	begun      int
	finished   int
	configured int
}

func (*runtimeStub) GetConstant(context.Context, string) (bytecode.Value, error) {
	return bytecode.Undefined(), nil
}
func (*runtimeStub) SetConstant(context.Context, string, bytecode.Value) error { return nil }
func (*runtimeStub) GetEnumerationValue(context.Context, string, string) (bytecode.Value, error) {
	return bytecode.Undefined(), nil
}
func (*runtimeStub) GetDefinedType(context.Context, string) (bytecode.Value, error) {
	return bytecode.Undefined(), nil
}
func (runtime *runtimeStub) BeginTestExecution(ctx context.Context) (context.Context, func(error) error, error) {
	runtime.begun++
	return ctx, func(error) error { runtime.finished++; return nil }, nil
}
func (runtime *runtimeStub) ConfigureProgramEvents(*bytecode.Program) error {
	runtime.configured++
	return nil
}

func TestDiscoverAndRunIsolatesExportedProcedures(t *testing.T) {
	program, diagnostics := compiler.CompileModules([]compiler.ModuleSource{
		{Name: "Помощник", Filename: "modules/helper.bsl", Source: "Функция Ответ() Экспорт\nВозврат 42;\nКонецФункции"},
		{Name: "Проверки", Filename: "tests/11111111-1111-4111-8111-111111111111.bsl", Source: `
Перем Состояние;
Процедура Успешный() Экспорт
    Состояние = 1;
    Если Помощник.Ответ() <> 42 Тогда ВызватьИсключение "ошибка"; КонецЕсли;
КонецПроцедуры
Процедура Ошибка() Экспорт
    Если Состояние <> Неопределено Тогда ВызватьИсключение "состояние протекло"; КонецЕсли;
    ВызватьИсключение "ожидаемый сбой";
КонецПроцедуры
Функция НеТест() Экспорт
    Возврат Истина;
КонецФункции
Процедура Скрытый()
КонецПроцедуры`},
	})
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	cases := Discover(program)
	if len(cases) != 2 || cases[0].Routine != "Успешный" || cases[1].Routine != "Ошибка" {
		t.Fatalf("cases=%+v", cases)
	}
	runtime := &runtimeStub{}
	report, err := Run(context.Background(), program, runtime, Selection{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed != 1 || report.Failed != 1 || runtime.begun != 2 || runtime.finished != 2 || runtime.configured != 2 {
		t.Fatalf("report=%+v runtime=%+v", report, runtime)
	}
	if report.Coverage.Total == 0 || report.Coverage.Covered == 0 || report.Coverage.Percent <= 0 {
		t.Fatalf("coverage=%+v", report.Coverage)
	}
	if report.Results[1].Error == "" || len(report.Results[1].Stack) == 0 || report.Results[1].Stack[0].Path != cases[1].Path {
		t.Fatalf("failed result=%+v", report.Results[1])
	}
}

func TestRunSelectionAndValidation(t *testing.T) {
	program, diagnostics := compiler.CompileSource("tests/11111111-1111-4111-8111-111111111111.bsl", "Процедура Проверить() Экспорт\nКонецПроцедуры")
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	runtime := &runtimeStub{}
	report, err := Run(context.Background(), program, runtime, Selection{Path: program.Modules[0].Source, Routine: "проверить"})
	if err != nil || report.Passed != 1 {
		t.Fatalf("report=%+v error=%v", report, err)
	}
	if _, err := Run(context.Background(), program, runtime, Selection{Routine: "Нет"}); err == nil {
		t.Fatal("missing selection was accepted")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(canceled, program, runtime, Selection{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Run error=%v", err)
	}
}
