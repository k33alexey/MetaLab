package vm

import (
	"context"
	"errors"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

var errUnfinishedTestTransaction = errors.New("unfinished test transaction")

type transactionRuntimeStub struct {
	metadataRuntimeStub
	depth int
}

func (runtime *transactionRuntimeStub) BeginExecution(ctx context.Context) (context.Context, func(error) error, error) {
	return ctx, func(error) error {
		if runtime.depth == 0 {
			return nil
		}
		runtime.depth = 0
		return errUnfinishedTestTransaction
	}, nil
}
func (runtime *transactionRuntimeStub) BeginTransaction(context.Context) error {
	runtime.depth++
	return nil
}
func (runtime *transactionRuntimeStub) CommitTransaction(context.Context) error {
	if runtime.depth == 0 {
		return errors.New("transaction is not active")
	}
	runtime.depth--
	return nil
}
func (runtime *transactionRuntimeStub) RollbackTransaction(context.Context) error {
	if runtime.depth == 0 {
		return errors.New("transaction is not active")
	}
	runtime.depth = 0
	return nil
}
func (runtime *transactionRuntimeStub) TransactionActive(context.Context) (bool, error) {
	return runtime.depth > 0, nil
}

func TestTransactionsExecuteInRussianAndEnglish(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source, routine string
	}{
		{name: "russian", routine: "Проверить", source: `&НаСервере
Функция Проверить()
    НачатьТранзакцию();
    НачатьТранзакцию();
    ЗафиксироватьТранзакцию();
    Активна = ТранзакцияАктивна();
    ОтменитьТранзакцию();
    Возврат Активна И Не ТранзакцияАктивна();
КонецФункции`},
		{name: "english", routine: "Run", source: `&AtServer
Function Run()
    BeginTransaction();
    Active = TransactionActive();
    CommitTransaction();
    Return Active And Not TransactionActive();
EndFunction`},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			program, diagnostics := compiler.CompileSource("transaction.bsl", test.source)
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			machine, err := New(program)
			if err != nil {
				t.Fatal(err)
			}
			runtime := &transactionRuntimeStub{}
			result, err := machine.NewContextWithMetadata(runtime).Call(test.routine)
			if err != nil || result != bytecode.Boolean(true) || runtime.depth != 0 {
				t.Fatalf("result=%v depth=%d error=%v", result, runtime.depth, err)
			}
		})
	}
}

func TestExecutionRuntimeRejectsUnfinishedTransaction(t *testing.T) {
	t.Parallel()
	program, diagnostics := compiler.CompileSource("unfinished.bsl", `&AtServer
Function Run()
    BeginTransaction();
    Return 1;
EndFunction`)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := New(program)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &transactionRuntimeStub{}
	_, err = machine.NewContextWithMetadata(runtime).Call("Run")
	if !errors.Is(err, errUnfinishedTestTransaction) || runtime.depth != 0 {
		t.Fatalf("depth=%d error=%v", runtime.depth, err)
	}
}

func TestCompilerRejectsTransactionsOutsideServer(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`BeginTransaction();`, `Value = New DataLock;`} {
		_, diagnostics := compiler.CompileSource("client.bsl", `&AtClient
Procedure Run()
	`+body+`
EndProcedure`)
		if len(diagnostics) != 1 || diagnostics[0].Code != "BSL3042" {
			t.Fatalf("body=%q diagnostics=%v", body, diagnostics)
		}
	}
}
