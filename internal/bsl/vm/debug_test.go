package vm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

func TestDebugSessionStepsStackVariablesAndExpressions(t *testing.T) {
	t.Parallel()
	machine := compileDebugMachine(t, `Перем Глобальное;

Функция Внутренняя(Значение)
    Локальная = Значение + 1;
    Возврат Локальная * 2;
КонецФункции

Функция Запуск(Начало)
    Глобальное = Начало;
    Результат = Внутренняя(Начало);
    Возврат Результат;
КонецФункции`)
	session, err := machine.NewContext().StartDebug(context.Background(), "Запуск", nil, bytecode.Number(40))
	if err != nil {
		t.Fatal(err)
	}
	entry := waitDebug(t, session, 0)
	if entry.State != DebugPaused || entry.Reason != "entry" || len(entry.Frames) != 1 || entry.Frames[0].Location.Line != 9 {
		t.Fatalf("entry = %+v", entry)
	}
	if value, err := session.Evaluate(0, "Начало + 2"); err != nil || value.Value != "42" {
		t.Fatalf("entry expression = %+v, %v", value, err)
	}

	if err := session.Resume(DebugStepOver); err != nil {
		t.Fatal(err)
	}
	call := waitDebug(t, session, entry.Sequence)
	if call.State != DebugPaused || call.Frames[0].Location.Line != 10 {
		t.Fatalf("step over = %+v", call)
	}
	if err := session.Resume(DebugStepInto); err != nil {
		t.Fatal(err)
	}
	inner := waitDebug(t, session, call.Sequence)
	if inner.State != DebugPaused || len(inner.Frames) != 2 || inner.Frames[0].Routine != "Внутренняя" || inner.Frames[0].Location.Line != 4 {
		t.Fatalf("step into = %+v", inner)
	}
	if value, err := session.Evaluate(0, "Значение + 2"); err != nil || value.Value != "42" {
		t.Fatalf("inner expression = %+v, %v", value, err)
	}
	if _, err := session.Evaluate(0, "Новый Массив"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("unsafe expression error = %v", err)
	}

	if err := session.Resume(DebugStepOut); err != nil {
		t.Fatal(err)
	}
	outer := waitDebug(t, session, inner.Sequence)
	if outer.State != DebugPaused || len(outer.Frames) != 1 || outer.Frames[0].Location.Line != 11 {
		t.Fatalf("step out = %+v", outer)
	}
	if value, err := session.Evaluate(0, "Результат"); err != nil || value.Value != "82" {
		t.Fatalf("outer expression = %+v, %v", value, err)
	}
	if err := session.Resume(DebugContinue); err != nil {
		t.Fatal(err)
	}
	completed := waitDebug(t, session, outer.Sequence)
	if completed.State != DebugCompleted || completed.Result == nil || completed.Result.Value != "82" {
		t.Fatalf("completed = %+v", completed)
	}
}

func TestDebugSessionBreakpointsAndExceptions(t *testing.T) {
	t.Parallel()
	machine := compileDebugMachine(t, `Функция Ошибка()
    Значение = 1;
    Возврат Значение / 0;
КонецФункции`)
	session, err := machine.NewContext().StartDebug(context.Background(), "Ошибка", []DebugBreakpoint{{Filename: "modules/debug.bsl", Line: 3}})
	if err != nil {
		t.Fatal(err)
	}
	entry := waitDebug(t, session, 0)
	if err := session.Resume(DebugContinue); err != nil {
		t.Fatal(err)
	}
	breakpoint := waitDebug(t, session, entry.Sequence)
	if breakpoint.Reason != "breakpoint" || breakpoint.Frames[0].Location.Line != 3 {
		t.Fatalf("breakpoint = %+v", breakpoint)
	}
	if err := session.Resume(DebugContinue); err != nil {
		t.Fatal(err)
	}
	exception := waitDebug(t, session, breakpoint.Sequence)
	if exception.Reason != "exception" || !strings.Contains(exception.Message, "division by zero") {
		t.Fatalf("exception = %+v", exception)
	}
	if err := session.Resume(DebugContinue); err != nil {
		t.Fatal(err)
	}
	failed := waitDebug(t, session, exception.Sequence)
	if failed.State != DebugFailed || !strings.Contains(failed.Error, "division by zero") {
		t.Fatalf("failed = %+v", failed)
	}
}

func TestDebugSessionCanStopAndValidatesState(t *testing.T) {
	t.Parallel()
	machine := compileDebugMachine(t, "Процедура Запуск()\nПока Истина Цикл\nКонецЦикла;\nКонецПроцедуры")
	session, err := machine.NewContext().StartDebug(context.Background(), "Запуск", nil)
	if err != nil {
		t.Fatal(err)
	}
	entry := waitDebug(t, session, 0)
	if err := session.Resume(DebugContinue); err != nil {
		t.Fatal(err)
	}
	session.Stop()
	stopped := waitDebug(t, session, entry.Sequence)
	if stopped.State != DebugStopped {
		t.Fatalf("stopped = %+v", stopped)
	}
	if err := session.Resume(DebugContinue); !errors.Is(err, ErrDebugFinished) {
		t.Fatalf("Resume() error = %v", err)
	}
	if _, err := session.Evaluate(0, "1 + 1"); !errors.Is(err, ErrDebugNotPaused) {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if err := session.SetBreakpoints([]DebugBreakpoint{{Filename: "", Line: 0}}); err == nil {
		t.Fatal("SetBreakpoints() accepted an invalid breakpoint")
	}
}

func TestDebugSessionCanPauseRunningCode(t *testing.T) {
	t.Parallel()
	machine := compileDebugMachine(t, "Процедура Запуск()\nПока Истина Цикл\nКонецЦикла;\nКонецПроцедуры")
	session, err := machine.NewContext().StartDebug(context.Background(), "Запуск", nil)
	if err != nil {
		t.Fatal(err)
	}
	entry := waitDebug(t, session, 0)
	if err := session.Resume(DebugContinue); err != nil {
		t.Fatal(err)
	}
	if err := session.Pause(); err != nil {
		t.Fatal(err)
	}
	paused := waitDebug(t, session, entry.Sequence)
	if paused.State != DebugPaused || paused.Reason != "pause" {
		t.Fatalf("paused = %+v", paused)
	}
	session.Stop()
	stopped := waitDebug(t, session, paused.Sequence)
	if stopped.State != DebugStopped {
		t.Fatalf("stopped = %+v", stopped)
	}
}

func TestCompiledProgramRetainsDebuggerLocalNames(t *testing.T) {
	t.Parallel()
	program, diagnostics := compiler.CompileSource("names.bsl", "Функция Запуск(Параметр)\nЯвная = Параметр;\nВозврат Явная;\nКонецФункции")
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	function, ok := program.Lookup("Запуск")
	if !ok || len(function.LocalNames) < 2 || function.LocalNames[0] != "Параметр" || function.LocalNames[1] != "Явная" {
		t.Fatalf("local names = %+v", function)
	}
}

func compileDebugMachine(t *testing.T, source string) *Machine {
	t.Helper()
	program, diagnostics := compiler.CompileSource("modules/debug.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := New(program)
	if err != nil {
		t.Fatal(err)
	}
	return machine
}

func waitDebug(t *testing.T, session *DebugSession, after uint64) DebugSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	snapshot, err := session.Wait(ctx, after)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
